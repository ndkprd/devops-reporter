package main

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"time"
)

//go:embed templates/tenable-was/template.html
var tenableWasTemplate string

func init() {
	RegisterSource("tenable-was", &ReportSource{
		DefaultTitle: "Tenable WAS Scan Report",
		Template:     tenableWasTemplate,
		FuncMap: template.FuncMap{
			"wasRiskClass": wasRiskClass,
			"wasRiskLabel": wasRiskLabel,
		},
		Parse: func(data []byte, title string) (any, error) {
			var report WasReport
			if err := json.Unmarshal(data, &report); err != nil {
				return nil, err
			}
			return BuildWasReportData(report, title), nil
		},
	})
}

// ── Input types ──────────────────────────────────────────────────

type WasFinding struct {
	PluginID    int      `json:"plugin_id"`
	Name        string   `json:"name"`
	Family      string   `json:"family"`
	Synopsis    string   `json:"synopsis"`
	Description string   `json:"description"`
	Solution    string   `json:"solution"`
	RiskFactor  string   `json:"risk_factor"`
	URI         string   `json:"uri"`
	CVSSv3      *float64 `json:"cvssv3"`
	CVSSv4      *float64 `json:"cvssv4"`
	CVEs        []string `json:"cves"`
	CWE         []int    `json:"cwe"`
	OWASP       []string `json:"owasp"`
	Output      string   `json:"output"`
	Proof       string   `json:"proof"`
	SeeAlso     []string `json:"see_also"`
}

type WasScan struct {
	ScanID      string `json:"scan_id"`
	Status      string `json:"status"`
	StartedAt   string `json:"started_at"`
	FinalizedAt string `json:"finalized_at"`
	Target      string `json:"target"`
}

type WasReport struct {
	Report struct {
		Version   string `json:"version"`
		CreatedAt string `json:"created_at"`
	} `json:"report"`
	Config struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ScannerType string `json:"scanner_type"`
	} `json:"config"`
	Template struct {
		Name string `json:"name"`
	} `json:"template"`
	Scan     WasScan      `json:"scan"`
	Findings []WasFinding `json:"findings"`
}

// ── Report types ─────────────────────────────────────────────────

type WasSeverityGroup struct {
	Severity string
	Findings []WasFinding
}

type WasSummary struct {
	Critical int
	High     int
	Medium   int
	Low      int
	Info     int
	Total    int
}

type WasReportData struct {
	Title       string
	GeneratedAt string
	ScanTarget  string
	ScanID      string
	ScanStatus  string
	ScanName    string
	Template    string
	StartedAt   string
	FinalizedAt string
	Duration    string
	Summary     WasSummary
	Groups      []WasSeverityGroup
	HasIssues   bool
}

var wasSeverityOrder = []string{"critical", "high", "medium", "low", "info"}

func BuildWasReportData(r WasReport, title string) WasReportData {
	groupMap := make(map[string][]WasFinding)
	for _, f := range r.Findings {
		groupMap[f.RiskFactor] = append(groupMap[f.RiskFactor], f)
	}

	summary := WasSummary{Total: len(r.Findings)}
	summary.Critical = len(groupMap["critical"])
	summary.High = len(groupMap["high"])
	summary.Medium = len(groupMap["medium"])
	summary.Low = len(groupMap["low"])
	summary.Info = len(groupMap["info"])

	groups := make([]WasSeverityGroup, 0, len(wasSeverityOrder))
	for _, sev := range wasSeverityOrder {
		if findings, ok := groupMap[sev]; ok {
			groups = append(groups, WasSeverityGroup{Severity: sev, Findings: findings})
		}
	}

	startedAt, finishedAt, duration := wasDuration(r.Scan.StartedAt, r.Scan.FinalizedAt)

	return WasReportData{
		Title:       title,
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		ScanTarget:  r.Scan.Target,
		ScanID:      r.Scan.ScanID,
		ScanStatus:  r.Scan.Status,
		ScanName:    r.Config.Name,
		Template:    r.Template.Name,
		StartedAt:   startedAt,
		FinalizedAt: finishedAt,
		Duration:    duration,
		Summary:     summary,
		Groups:      groups,
		HasIssues:   summary.Critical+summary.High+summary.Medium > 0,
	}
}

func wasDuration(startStr, endStr string) (string, string, string) {
	const layout = "2006-01-02T15:04:05.999999-07:00"
	start, err1 := time.Parse(layout, startStr)
	end, err2 := time.Parse(layout, endStr)
	if err1 != nil || err2 != nil {
		return startStr, endStr, ""
	}
	d := end.Sub(start).Round(time.Second)
	return start.UTC().Format("2006-01-02 15:04:05 UTC"),
		end.UTC().Format("2006-01-02 15:04:05 UTC"),
		d.String()
}

func wasRiskClass(risk string) string {
	switch risk {
	case "critical":
		return "risk-critical"
	case "high":
		return "risk-high"
	case "medium":
		return "risk-medium"
	case "low":
		return "risk-low"
	default:
		return "risk-info"
	}
}

func wasRiskLabel(risk string) string {
	switch risk {
	case "critical":
		return "Critical"
	case "high":
		return "High"
	case "medium":
		return "Medium"
	case "low":
		return "Low"
	default:
		return "Info"
	}
}
