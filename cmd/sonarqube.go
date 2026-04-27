package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"
)

func init() {
	RegisterSource("sonarqube", &ReportSource{
		DefaultTitle: "SonarQube Analysis Report",
		Parse: func(data []byte, title string) (ReportData, error) {
			var report SonarQubeReport
			if err := json.Unmarshal(data, &report); err != nil {
				return ReportData{}, err
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

// ── Adapter ──────────────────────────────────────────────────────

var sqSeverityOrder = []string{"BLOCKER", "CRITICAL", "MAJOR", "MINOR", "INFO"}

func BuildSonarQubeReportData(report SonarQubeReport, title string) ReportData {
	total := len(report.Issues)
	hasIssues := total > 0

	type counts struct{ blocker, critical, major, minor, info int }
	var cnt counts
	sevMap := make(map[string][]SonarQubeIssue)

	projectKey := ""
	for _, issue := range report.Issues {
		sev := strings.ToUpper(issue.Severity)
		sevMap[sev] = append(sevMap[sev], issue)
		switch sev {
		case "BLOCKER":
			cnt.blocker++
		case "CRITICAL":
			cnt.critical++
		case "MAJOR":
			cnt.major++
		case "MINOR":
			cnt.minor++
		default:
			cnt.info++
		}
		if projectKey == "" {
			parts := strings.SplitN(issue.Component, ":", 2)
			if len(parts) > 1 {
				projectKey = parts[0]
			}
		}
	}

	statusLine := "No issues found — code is clean"
	if hasIssues {
		statusLine = fmt.Sprintf("%s found · %d blocker, %d critical, %d major", pluralise(total, "issue", "issues"), cnt.blocker, cnt.critical, cnt.major)
	}

	cols := []string{"Rule", "Type", "File", "Line", "Message"}
	var groups []SectionGroup
	for _, sev := range sqSeverityOrder {
		issues, ok := sevMap[sev]
		if !ok {
			continue
		}
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].Component < issues[j].Component
		})
		cls, lbl := sqSevCanonical(sev)
		rows := make([][]template.HTML, 0, len(issues))
		for _, issue := range issues {
			file := sqFileFromComp(issue.Component)
			line := ""
			if issue.Line > 0 {
				line = fmt.Sprintf("%d", issue.Line)
			}
			rows = append(rows, []template.HTML{
				MonoHTML(sqRuleLang(issue.Rule) + ":" + sqRuleID(issue.Rule)),
				template.HTML(fmt.Sprintf(`<span class="badge state-neutral">%s</span>`, template.HTMLEscapeString(sqTypeLabel(issue.Type)))),
				template.HTML(fmt.Sprintf(`<span class="td-file">%s</span>`, template.HTMLEscapeString(file))),
				MonoHTML(line),
				template.HTML(template.HTMLEscapeString(issue.Message)),
			})
		}
		groups = append(groups, SectionGroup{
			Name:    lbl,
			Count:   pluralise(len(issues), "issue", "issues"),
			Class:   cls,
			Columns: cols,
			Rows:    rows,
		})
	}

	meta := []KV{
		{Label: "Project Key", Value: projectKey},
	}

	return ReportData{
		Title:       title,
		Eyebrow:     "Static Code Analysis",
		Subtitle:    fmt.Sprintf("Project: %s · %s", projectKey, pluralise(total, "issue", "issues")),
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Status:      statusFromBool(hasIssues),
		StatusLine:  statusLine,
		Meta:        meta,
		Summary: []StatCard{
			{Number: fmt.Sprintf("%d", total), Label: "Total", Variant: "primary"},
			{Number: fmt.Sprintf("%d", cnt.blocker), Label: "Blocker", Variant: "blocker"},
			{Number: fmt.Sprintf("%d", cnt.critical), Label: "Critical", Variant: "critical"},
			{Number: fmt.Sprintf("%d", cnt.major), Label: "Major", Variant: "medium"},
			{Number: fmt.Sprintf("%d", cnt.minor), Label: "Minor", Variant: "low"},
			{Number: fmt.Sprintf("%d", cnt.info), Label: "Info", Variant: "info"},
		},
		Sections: []Section{
			{Kind: "table", Title: "Issues by Severity", Groups: groups, Empty: "No issues in report."},
		},
		Footer: FooterInfo{
			Total: pluralise(total, "issue", "issues"),
			Brand: "devops-reporter · sonarqube",
		},
	}
}

// ── Helpers ──────────────────────────────────────────────────────

func sqSevCanonical(sev string) (cssClass, label string) {
	switch strings.ToUpper(sev) {
	case "BLOCKER":
		return "sev-blocker", "Blocker"
	case "CRITICAL":
		return "sev-critical", "Critical"
	case "MAJOR":
		return "sev-medium", "Major"
	case "MINOR":
		return "sev-low", "Minor"
	default:
		return "sev-info", "Info"
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

func sqRuleID(rule string) string {
	parts := strings.SplitN(rule, ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return rule
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
