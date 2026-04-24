package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"
	"strings"
)

// ReportSource defines a pluggable report source.
type ReportSource struct {
	DefaultTitle string
	Template     string
	FuncMap      template.FuncMap
	Parse        func(data []byte, title string) (any, error)
}

var sources = map[string]*ReportSource{}

func RegisterSource(name string, s *ReportSource) {
	sources[name] = s
}

var version = "dev"

func main() {
	sourceNames := make([]string, 0, len(sources))
	for name := range sources {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)

	var outputFile, source, title, templateFile string
	var showVersion bool

	flag.StringVar(&outputFile, "output", "report.html", "output HTML file path")
	flag.StringVar(&outputFile, "o", "report.html", "shorthand for --output")
	flag.StringVar(&source, "source", "", "report source: "+strings.Join(sourceNames, ", "))
	flag.StringVar(&source, "s", "", "shorthand for --source")
	flag.StringVar(&title, "title", "", "report title (defaults to source-specific title)")
	flag.StringVar(&title, "t", "", "shorthand for --title")
	flag.StringVar(&templateFile, "template", "", "path to a custom HTML template file (uses built-in template if not set)")
	flag.StringVar(&templateFile, "T", "", "shorthand for --template")
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
		fmt.Fprintf(os.Stdout, "  kubeconform -output json ./manifests/ | devops-reporter --source kubeconform\n")
		fmt.Fprintf(os.Stdout, "  argocd app get my-app -o json | devops-reporter --source argocd --template custom.template.html\n")
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

	tmplContent := src.Template
	if templateFile != "" {
		fileBytes, err := os.ReadFile(templateFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading template file: %v\n", err)
			os.Exit(1)
		}
		tmplContent = string(fileBytes)
	}

	tmpl, err := template.New("report").Funcs(src.FuncMap).Parse(tmplContent)
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
