package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
)

//go:embed template.html
var templateContent string

var version = "dev"

func main() {
	outputFile := flag.String("o", "report.html", "output HTML file path")
	title := flag.String("title", "ArgoCD Application Report", "report title")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: argocd app get <name> -o json | argocd-report [flags]\n\n")
		fmt.Fprintf(os.Stdout, "Reads ArgoCD Application JSON from stdin and generates a static HTML report.\n\n")
		fmt.Fprintf(os.Stdout, "Flags:\n")
		flag.CommandLine.SetOutput(os.Stdout)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stdout, "\nExamples:\n")
		fmt.Fprintf(os.Stdout, "  argocd app get my-app -o json | argocd-report -o report.html\n")
		fmt.Fprintf(os.Stdout, "  kubectl get application my-app -o json | argocd-report -o report.html\n")
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
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

	var app ArgoApplication
	if err := json.Unmarshal(data, &app); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing JSON input: %v\n", err)
		os.Exit(1)
	}

	reportData := BuildReportData(app, *title)

	funcMap := template.FuncMap{
		"syncClass":   syncClass,
		"healthClass": healthClass,
		"opClass":     opClass,
		"shortRev":    shortRev,
	}
	tmpl, err := template.New("report").Funcs(funcMap).Parse(templateContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing template: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create(*outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := tmpl.Execute(f, reportData); err != nil {
		fmt.Fprintf(os.Stderr, "error rendering template: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "report written to %s\n", *outputFile)
}
