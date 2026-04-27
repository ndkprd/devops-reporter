package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

//go:embed templates/report.html
var reportTemplate string

//go:embed templates/themes/paper.css
var paperCSS string

var version = "dev"

func main() {
	sourceNames := make([]string, 0, len(sources))
	for name := range sources {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)

	var outputFile, source, title, orgName, templateFile, cssFile string
	var showVersion, summaryOnly bool

	flag.StringVar(&outputFile, "output", "report.html", "output HTML file path")
	flag.StringVar(&outputFile, "o", "report.html", "shorthand for --output")
	flag.StringVar(&source, "source", "", "report source: "+strings.Join(sourceNames, ", "))
	flag.StringVar(&source, "s", "", "shorthand for --source")
	flag.StringVar(&title, "title", "", "report title (defaults to source-specific title)")
	flag.StringVar(&title, "t", "", "shorthand for --title")
	flag.StringVar(&templateFile, "template", "", "path to a custom HTML template file (uses built-in template if not set)")
	flag.StringVar(&templateFile, "T", "", "shorthand for --template")
	flag.StringVar(&cssFile, "css", "", "path or URL to a custom CSS file; replaces the built-in paper theme\n    (e.g. --css themes/dracula.css or --css https://example.com/my.css)\n    Note: if --template is set this flag is ignored")
	flag.StringVar(&orgName, "org", "", "organization name shown in the report header and footer")
	flag.BoolVar(&summaryOnly, "summary-only", false, "render only the summary section; omit detail tables and cards")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "shorthand for --version")

	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: <json-source> | devops-reporter --source <source> [flags]\n\n")
		fmt.Fprintf(os.Stdout, "Reads JSON from stdin and generates a static HTML report.\n\n")
		fmt.Fprintf(os.Stdout, "Supported sources: %s\n\n", strings.Join(sourceNames, ", "))
		fmt.Fprintf(os.Stdout, "Flags:\n")
		flag.CommandLine.SetOutput(os.Stdout)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stdout, "\nExamples:\n")
		fmt.Fprintf(os.Stdout, "  argocd app get my-app -o json | devops-reporter --source argocd\n")
		fmt.Fprintf(os.Stdout, "  trivy image --format json myimage | devops-reporter --source trivy --summary-only\n")
		fmt.Fprintf(os.Stdout, "  trivy image --format json myimage | devops-reporter --source trivy --css themes/dracula.css\n")
		fmt.Fprintf(os.Stdout, "  trivy image --format json myimage | devops-reporter --source trivy --css https://example.com/my-theme.css\n")
	}
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if source == "" {
		fmt.Fprintf(os.Stderr, "error: --source flag is required (supported: %s)\n", strings.Join(sourceNames, ", "))
		os.Exit(1)
	}

	src, ok := sources[source]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown source %q (supported: %s)\n", source, strings.Join(sourceNames, ", "))
		os.Exit(1)
	}

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		flag.Usage()
		os.Exit(0)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
		os.Exit(1)
	}

	if len(data) == 0 {
		fmt.Fprintf(os.Stderr, "error: no input received on stdin\n")
		os.Exit(1)
	}

	if !json.Valid(data) {
		fmt.Fprintf(os.Stderr, "error: input is not valid JSON\n")
		os.Exit(1)
	}

	reportTitle := title
	if reportTitle == "" {
		reportTitle = src.DefaultTitle
	}

	reportData, err := src.Parse(data, reportTitle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing input: %v\n", err)
		os.Exit(1)
	}

	reportData.SummaryOnly = summaryOnly
	reportData.OrgName = orgName

	// Resolve template content: --template flag > embedded report.html
	tmplContent := reportTemplate
	if templateFile != "" {
		fileBytes, err := os.ReadFile(templateFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading template file: %v\n", err)
			os.Exit(1)
		}
		tmplContent = string(fileBytes)
		// When a custom template is used, CSS injection is the user's responsibility.
		reportData.CSS = ""
	} else {
		// Resolve CSS: --css flag > built-in paper theme
		css := paperCSS
		if cssFile != "" {
			loaded, err := loadCSS(cssFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error loading CSS from %q: %v\n", cssFile, err)
				os.Exit(1)
			}
			css = loaded
		}
		reportData.CSS = template.CSS(css)
	}

	tmpl, err := template.New("report").Parse(tmplContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing template: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := tmpl.Execute(f, reportData); err != nil {
		fmt.Fprintf(os.Stderr, "error rendering template: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "report written to %s\n", outputFile)
}

// loadCSS loads a stylesheet from a local file path or an https:// URL.
func loadCSS(ref string) (string, error) {
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(ref)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := os.ReadFile(ref)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
