package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"time"
)

func init() {
	RegisterSource("kubeconform", &ReportSource{
		DefaultTitle: "Kubeconform Validation Report",
		Parse: func(data []byte, title string) (ReportData, error) {
			var output KubeconformOutput
			if err := json.Unmarshal(data, &output); err != nil {
				return ReportData{}, err
			}
			return BuildKubeconformReportData(output, title), nil
		},
	})
}

// ── Input types ──────────────────────────────────────────────────

type KcResource struct {
	Filename string `json:"filename"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	Status   string `json:"status"`
	Msg      string `json:"msg"`
}

type KcSummary struct {
	Valid   int `json:"valid"`
	Invalid int `json:"invalid"`
	Errors  int `json:"errors"`
	Skipped int `json:"skipped"`
}

type KubeconformOutput struct {
	Resources []KcResource `json:"resources"`
	Summary   KcSummary    `json:"summary"`
}

// ── Adapter ──────────────────────────────────────────────────────

func BuildKubeconformReportData(output KubeconformOutput, title string) ReportData {
	hasIssues := output.Summary.Invalid+output.Summary.Errors > 0
	total := len(output.Resources)

	statusLine := "All resources valid"
	if hasIssues {
		statusLine = fmt.Sprintf(
			"%s detected",
			pluralise(output.Summary.Invalid+output.Summary.Errors, "invalid resource", "invalid resources"),
		)
	}

	// Group resources by kind, preserving order within each kind
	kindOrder := []string{}
	kindMap := make(map[string][]KcResource)
	for _, r := range output.Resources {
		if _, seen := kindMap[r.Kind]; !seen {
			kindOrder = append(kindOrder, r.Kind)
		}
		kindMap[r.Kind] = append(kindMap[r.Kind], r)
	}

	cols := []string{"Name", "File", "Status", "Message"}
	var groups []SectionGroup
	for _, kind := range kindOrder {
		resources := kindMap[kind]
		rows := make([][]template.HTML, 0, len(resources))
		for _, r := range resources {
			cls, lbl := kcCanonical(r.Status)
			rows = append(rows, []template.HTML{
				template.HTML(template.HTMLEscapeString(r.Name)),
				MonoHTML(r.Filename),
				BadgeHTML("badge "+cls, lbl),
				template.HTML(template.HTMLEscapeString(r.Msg)),
			})
		}
		groups = append(groups, SectionGroup{
			Name:    kind,
			Count:   pluralise(len(resources), "resource", "resources"),
			Columns: cols,
			Rows:    rows,
		})
	}

	return ReportData{
		Title:       title,
		Eyebrow:     "Kubernetes Schema Validation",
		Subtitle:    fmt.Sprintf("%s · %d invalid · %d errors", pluralise(total, "resource", "resources"), output.Summary.Invalid, output.Summary.Errors),
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Status:      statusFromBool(hasIssues),
		StatusLine:  statusLine,
		Meta: nil,
		Summary: []StatCard{
			{Number: fmt.Sprintf("%d", total), Label: "Total", Variant: "primary"},
			{Number: fmt.Sprintf("%d", output.Summary.Valid), Label: "Valid", Variant: "pass"},
			{Number: fmt.Sprintf("%d", output.Summary.Invalid), Label: "Invalid", Variant: "fail"},
			{Number: fmt.Sprintf("%d", output.Summary.Errors), Label: "Errors", Variant: "warn"},
			{Number: fmt.Sprintf("%d", output.Summary.Skipped), Label: "Skipped", Variant: "info"},
		},
		Sections: []Section{
			{Kind: "table", Title: "Resources by Kind", Groups: groups, Empty: "No resources in report."},
		},
		Footer: FooterInfo{
			Total: pluralise(total, "resource", "resources"),
			Brand: "devops-reporter · kubeconform",
		},
	}
}

func kcCanonical(status string) (cssClass, label string) {
	switch status {
	case "statusValid":
		return "state-pass", "Valid"
	case "statusInvalid":
		return "state-fail", "Invalid"
	case "statusError":
		return "state-fail", "Error"
	case "statusSkipped":
		return "state-neutral", "Skipped"
	case "statusEmpty":
		return "state-neutral", "Empty"
	default:
		return "state-neutral", status
	}
}
