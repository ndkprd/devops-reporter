package main

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"sort"
	"strings"
	"time"
)

//go:embed templates/sonarqube.html
var sonarqubeTemplate string

func init() {
	RegisterSource("sonarqube", &ReportSource{
		DefaultTitle: "SonarQube Analysis Report",
		Template:     sonarqubeTemplate,
		FuncMap: template.FuncMap{
			"sqSeverityClass": sqSeverityClass,
			"sqSeverityLabel": sqSeverityLabel,
			"sqTypeClass":     sqTypeClass,
			"sqTypeLabel":     sqTypeLabel,
			"sqFileFromComp":  sqFileFromComp,
			"sqRuleLang":      sqRuleLang,
		},
		Parse: func(data []byte, title string) (any, error) {
			var report SonarQubeReport
			if err := json.Unmarshal(data, &report); err != nil {
				return nil, err
			}
			return BuildSonarQubeReportData(report, title), nil
		},
	})
}

// ── Input types ──────────────────────────────────────────────────

type SonarQubePaging struct {
	PageIndex int `json:"pageIndex"`
	PageSize  int `json:"pageSize"`
	Total     int `json:"total"`
}

type SonarQubeIssue struct {
	Key          string   `json:"key"`
	Rule         string   `json:"rule"`
	Severity     string   `json:"severity"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	Message      string   `json:"message"`
	Component    string   `json:"component"`
	Line         int      `json:"line"`
	Effort       string   `json:"effort"`
	CreationDate string   `json:"creationDate"`
	UpdateDate   string   `json:"updateDate"`
	Tags         []string `json:"tags"`
}

type SonarQubeReport struct {
	Total  int              `json:"total"`
	Paging SonarQubePaging  `json:"paging"`
	Issues []SonarQubeIssue `json:"issues"`
}

// ── Report types ─────────────────────────────────────────────────

type SqSeverityGroup struct {
	Severity string
	Issues   []SonarQubeIssue
}

type SqTypeGroup struct {
	Type   string
	Issues []SonarQubeIssue
}

type SqFileGroup struct {
	File   string
	Issues []SonarQubeIssue
}

type SqSummary struct {
	Total      int
	BySeverity struct {
		Blocker  int
		Critical int
		Major    int
		Minor    int
		Info     int
	}
	ByType struct {
		Bug             int
		CodeSmell       int
		Vulnerability   int
		SecurityHotspot int
	}
	Open        int
	Confirmed   int
	Reopened    int
	TotalEffort string
}

type SqReportData struct {
	Title          string
	GeneratedAt    string
	ProjectKey     string
	TotalIssues    int
	TotalPages     int
	Summary        SqSummary
	SeverityGroups []SqSeverityGroup
	TypeGroups     []SqTypeGroup
	FileGroups     []SqFileGroup
	HasIssues      bool
}

// ── Helpers ──────────────────────────────────────────────────────

var sqSeverityOrder = []string{"BLOCKER", "CRITICAL", "MAJOR", "MINOR", "INFO"}

var sqTypeOrder = []string{"BUG", "VULNERABILITY", "CODE_SMELL", "SECURITY_HOTSPOT"}

func sqSeverityClass(severity string) string {
	switch strings.ToUpper(severity) {
	case "BLOCKER":
		return "sev-blocker"
	case "CRITICAL":
		return "sev-critical"
	case "MAJOR":
		return "sev-major"
	case "MINOR":
		return "sev-minor"
	default:
		return "sev-info"
	}
}

func sqSeverityLabel(severity string) string {
	switch strings.ToUpper(severity) {
	case "BLOCKER":
		return "Blocker"
	case "CRITICAL":
		return "Critical"
	case "MAJOR":
		return "Major"
	case "MINOR":
		return "Minor"
	default:
		return "Info"
	}
}

func sqTypeClass(issueType string) string {
	switch strings.ToUpper(issueType) {
	case "BUG":
		return "type-bug"
	case "VULNERABILITY":
		return "type-vuln"
	case "CODE_SMELL":
		return "type-smell"
	case "SECURITY_HOTSPOT":
		return "type-hotspot"
	default:
		return "type-other"
	}
}

func sqTypeLabel(issueType string) string {
	switch strings.ToUpper(issueType) {
	case "BUG":
		return "Bug"
	case "VULNERABILITY":
		return "Vulnerability"
	case "CODE_SMELL":
		return "Code Smell"
	case "SECURITY_HOTSPOT":
		return "Security Hotspot"
	default:
		return issueType
	}
}

func sqFileFromComp(component string) string {
	parts := strings.SplitN(component, ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return component
}

func sqRuleLang(rule string) string {
	parts := strings.SplitN(rule, ":", 2)
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

func sqParseEffort(effort string) time.Duration {
	if effort == "" {
		return 0
	}
	d, err := time.ParseDuration(effort)
	if err != nil {
		return 0
	}
	return d
}

func sqFormatEffort(d time.Duration) string {
	if d == 0 {
		return "—"
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	return d.Round(time.Minute).String()
}

// ── Report Builder ───────────────────────────────────────────────

func BuildSonarQubeReportData(report SonarQubeReport, title string) SqReportData {
	summary := SqSummary{Total: len(report.Issues)}

	sevMap := make(map[string][]SonarQubeIssue)
	typeMap := make(map[string][]SonarQubeIssue)
	fileMap := make(map[string][]SonarQubeIssue)
	var totalEffort time.Duration

	for _, issue := range report.Issues {
		sev := strings.ToUpper(issue.Severity)
		typ := strings.ToUpper(issue.Type)

		sevMap[sev] = append(sevMap[sev], issue)
		typeMap[typ] = append(typeMap[typ], issue)

		file := sqFileFromComp(issue.Component)
		fileMap[file] = append(fileMap[file], issue)

		if issue.Effort != "" {
			totalEffort += sqParseEffort(issue.Effort)
		}

		switch sev {
		case "BLOCKER":
			summary.BySeverity.Blocker++
		case "CRITICAL":
			summary.BySeverity.Critical++
		case "MAJOR":
			summary.BySeverity.Major++
		case "MINOR":
			summary.BySeverity.Minor++
		default:
			summary.BySeverity.Info++
		}

		switch typ {
		case "BUG":
			summary.ByType.Bug++
		case "VULNERABILITY":
			summary.ByType.Vulnerability++
		case "CODE_SMELL":
			summary.ByType.CodeSmell++
		case "SECURITY_HOTSPOT":
			summary.ByType.SecurityHotspot++
		}

		switch issue.Status {
		case "OPEN":
			summary.Open++
		case "CONFIRMED":
			summary.Confirmed++
		case "REOPENED":
			summary.Reopened++
		}
	}

	severityGroups := make([]SqSeverityGroup, 0, len(sqSeverityOrder))
	for _, sev := range sqSeverityOrder {
		if issues, ok := sevMap[sev]; ok {
			sort.Slice(issues, func(i, j int) bool {
				return issues[i].Component < issues[j].Component
			})
			severityGroups = append(severityGroups, SqSeverityGroup{Severity: sev, Issues: issues})
		}
	}

	typeGroups := make([]SqTypeGroup, 0, len(sqTypeOrder))
	for _, typ := range sqTypeOrder {
		if issues, ok := typeMap[typ]; ok {
			sort.Slice(issues, func(i, j int) bool {
				return sqSeverityRank(issues[i].Severity) < sqSeverityRank(issues[j].Severity)
			})
			typeGroups = append(typeGroups, SqTypeGroup{Type: typ, Issues: issues})
		}
	}

	fileKeys := make([]string, 0, len(fileMap))
	for f := range fileMap {
		fileKeys = append(fileKeys, f)
	}
	sort.Strings(fileKeys)
	fileGroups := make([]SqFileGroup, 0, len(fileKeys))
	for _, f := range fileKeys {
		fileGroups = append(fileGroups, SqFileGroup{File: f, Issues: fileMap[f]})
	}

	summary.TotalEffort = sqFormatEffort(totalEffort)

	projectKey := ""
	if len(report.Issues) > 0 {
		parts := strings.SplitN(report.Issues[0].Component, ":", 2)
		if len(parts) > 1 {
			projectKey = parts[0]
		}
	}

	return SqReportData{
		Title:          title,
		GeneratedAt:    time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		ProjectKey:     projectKey,
		TotalIssues:    report.Total,
		TotalPages:     report.Paging.Total,
		Summary:        summary,
		SeverityGroups: severityGroups,
		TypeGroups:     typeGroups,
		FileGroups:     fileGroups,
		HasIssues:      summary.Total > 0,
	}
}

func sqSeverityRank(severity string) int {
	switch strings.ToUpper(severity) {
	case "BLOCKER":
		return 0
	case "CRITICAL":
		return 1
	case "MAJOR":
		return 2
	case "MINOR":
		return 3
	default:
		return 4
	}
}
