package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"
)

//go:embed templates/dep-check.html
var depCheckTemplate string

func init() {
	RegisterSource("dependency-check", &ReportSource{
		DefaultTitle: "Dependency-Check Report",
		Template:     depCheckTemplate,
		FuncMap: template.FuncMap{
			"depSevClass":    depSevClass,
			"depSevLabel":    depSevLabel,
			"depMarkdown":    depMarkdownToHTML,
			"depCVSSScore":   depCVSSScore,
		},
		Parse: func(data []byte, title string) (any, error) {
			var report DepReport
			if err := json.Unmarshal(data, &report); err != nil {
				return nil, err
			}
			return BuildDepReportData(report, title), nil
		},
	})
}

// ── Input types ──────────────────────────────────────────────────

type DepDataSource struct {
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
}

type DepEvidence struct {
	Type       string `json:"type"`
	Confidence string `json:"confidence"`
	Source     string `json:"source"`
	Name       string `json:"name"`
	Value      string `json:"value"`
}

type DepPackage struct {
	ID         string `json:"id"`
	Confidence string `json:"confidence"`
	URL        string `json:"url"`
}

type DepCVSSv3 struct {
	BaseScore    float64 `json:"baseScore"`
	BaseSeverity string  `json:"baseSeverity"`
	AttackVector string  `json:"attackVector"`
}

type DepCVSSv2 struct {
	Score    float64 `json:"score"`
	Severity string  `json:"severity"`
}

type DepReference struct {
	Source string `json:"source"`
	URL    string `json:"url"`
	Name   string `json:"name"`
}

type DepVulnerability struct {
	Source      string         `json:"source"`
	Name        string         `json:"name"`
	Severity    string         `json:"severity"`
	CVSSv3      *DepCVSSv3     `json:"cvssv3"`
	CVSSv2      *DepCVSSv2     `json:"cvssv2"`
	CWEs        []string       `json:"cwes"`
	Description string         `json:"description"`
	Notes       string         `json:"notes"`
	References  []DepReference `json:"references"`
}

type DepDependency struct {
	IsVirtual         bool         `json:"isVirtual"`
	FileName          string       `json:"fileName"`
	FilePath          string       `json:"filePath"`
	MD5               string       `json:"md5"`
	SHA1              string       `json:"sha1"`
	SHA256            string       `json:"sha256"`
	Packages          []DepPackage `json:"packages"`
	EvidenceCollected struct {
		VendorEvidence  []DepEvidence `json:"vendorEvidence"`
		ProductEvidence []DepEvidence `json:"productEvidence"`
		VersionEvidence []DepEvidence `json:"versionEvidence"`
	} `json:"evidenceCollected"`
	Vulnerabilities []DepVulnerability `json:"vulnerabilities"`
}

type DepReport struct {
	ReportSchema string `json:"reportSchema"`
	ScanInfo     struct {
		EngineVersion string          `json:"engineVersion"`
		DataSource    []DepDataSource `json:"dataSource"`
	} `json:"scanInfo"`
	ProjectInfo struct {
		Name       string            `json:"name"`
		ReportDate string            `json:"reportDate"`
		Credits    map[string]string `json:"credits"`
	} `json:"projectInfo"`
	Dependencies []DepDependency `json:"dependencies"`
}

// ── Report types ─────────────────────────────────────────────────

type DepCredit struct {
	Source string
	Text   string
}

type DepSummary struct {
	Total      int
	Vulnerable int
	Clean      int
	Critical   int
	High       int
	Medium     int
	Low        int
	Info       int
}

type DepVulnEntry struct {
	DepName  string
	CVEName  string
	Severity string
	CVSS     string
}

type DepVulnGroup struct {
	Severity string
	Entries  []DepVulnEntry
}

type DepReportData struct {
	Title          string
	GeneratedAt    string
	ProjectName    string
	ReportDate     string
	EngineVersion  string
	DataSources    []DepDataSource
	Summary        DepSummary
	VulnGroups     []DepVulnGroup
	VulnerableDeps []DepDependency
	CleanDeps      []DepDependency
	Credits        []DepCredit
	HasIssues      bool
}

var depSeverityRank = map[string]int{
	"critical":      0,
	"high":          1,
	"medium":        2,
	"moderate":      2,
	"low":           3,
	"info":          4,
	"informational": 4,
	"negligible":    4,
}

func depNormSeverity(s string) string {
	switch strings.ToLower(s) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium", "moderate":
		return "medium"
	case "low":
		return "low"
	default:
		return "info"
	}
}

func depWorstSeverity(dep DepDependency) string {
	best := 999
	for _, v := range dep.Vulnerabilities {
		if r, ok := depSeverityRank[strings.ToLower(v.Severity)]; ok && r < best {
			best = r
		}
	}
	switch best {
	case 0:
		return "critical"
	case 1:
		return "high"
	case 2:
		return "medium"
	case 3:
		return "low"
	default:
		return "info"
	}
}

func BuildDepReportData(r DepReport, title string) DepReportData {
	summary := DepSummary{Total: len(r.Dependencies)}

	var vulnDeps, cleanDeps []DepDependency

	for _, d := range r.Dependencies {
		if len(d.Vulnerabilities) == 0 {
			cleanDeps = append(cleanDeps, d)
			summary.Clean++
			continue
		}
		summary.Vulnerable++
		// sort vulns within each dep by severity
		sort.Slice(d.Vulnerabilities, func(i, j int) bool {
			ri := depSeverityRank[strings.ToLower(d.Vulnerabilities[i].Severity)]
			rj := depSeverityRank[strings.ToLower(d.Vulnerabilities[j].Severity)]
			return ri < rj
		})
		for _, v := range d.Vulnerabilities {
			switch depNormSeverity(v.Severity) {
			case "critical":
				summary.Critical++
			case "high":
				summary.High++
			case "medium":
				summary.Medium++
			case "low":
				summary.Low++
			default:
				summary.Info++
			}
		}
		vulnDeps = append(vulnDeps, d)
	}

	// sort vulnerable deps by worst severity
	sort.Slice(vulnDeps, func(i, j int) bool {
		ri := depSeverityRank[depWorstSeverity(vulnDeps[i])]
		rj := depSeverityRank[depWorstSeverity(vulnDeps[j])]
		if ri != rj {
			return ri < rj
		}
		return vulnDeps[i].FileName < vulnDeps[j].FileName
	})

	// Build VulnGroups: severity-ordered groups for the summary table
	sevOrder := []string{"critical", "high", "medium", "low", "info"}
	groupMap := make(map[string][]DepVulnEntry)
	for _, d := range vulnDeps {
		for _, v := range d.Vulnerabilities {
			norm := depNormSeverity(v.Severity)
			groupMap[norm] = append(groupMap[norm], DepVulnEntry{
				DepName:  d.FileName,
				CVEName:  v.Name,
				Severity: norm,
				CVSS:     depCVSSScore(v),
			})
		}
	}
	vulnGroups := make([]DepVulnGroup, 0, len(sevOrder))
	for _, sev := range sevOrder {
		if entries, ok := groupMap[sev]; ok {
			vulnGroups = append(vulnGroups, DepVulnGroup{Severity: sev, Entries: entries})
		}
	}

	reportDate := depFormatTimestamp(r.ProjectInfo.ReportDate)

	creditKeys := make([]string, 0, len(r.ProjectInfo.Credits))
	for k := range r.ProjectInfo.Credits {
		creditKeys = append(creditKeys, k)
	}
	sort.Strings(creditKeys)
	credits := make([]DepCredit, 0, len(creditKeys))
	for _, k := range creditKeys {
		credits = append(credits, DepCredit{Source: k, Text: r.ProjectInfo.Credits[k]})
	}

	return DepReportData{
		Title:          title,
		GeneratedAt:    time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		ProjectName:    r.ProjectInfo.Name,
		ReportDate:     reportDate,
		EngineVersion:  r.ScanInfo.EngineVersion,
		DataSources:    r.ScanInfo.DataSource,
		Summary:        summary,
		VulnGroups:     vulnGroups,
		VulnerableDeps: vulnDeps,
		CleanDeps:      cleanDeps,
		Credits:        credits,
		HasIssues:      summary.Vulnerable > 0,
	}
}

func depFormatTimestamp(s string) string {
	layouts := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC().Format("2006-01-02 15:04:05 UTC")
		}
	}
	return s
}

var (
	reMDBold = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	reMDCode = regexp.MustCompile("`([^`\n]+)`")
	reMDLink = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\n]+)\)`)
)

func depInline(s string) string {
	s = html.EscapeString(s)
	s = reMDBold.ReplaceAllString(s, `<strong>$1</strong>`)
	s = reMDCode.ReplaceAllString(s, `<code>$1</code>`)
	s = reMDLink.ReplaceAllString(s, `<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>`)
	return s
}

func depMarkdownToHTML(s string) template.HTML {
	if s == "" {
		return ""
	}

	var sb strings.Builder
	var paraLines []string
	var listItems []string
	inCode := false
	var codeLines []string

	flushPara := func() {
		if len(paraLines) == 0 {
			return
		}
		sb.WriteString("<p>")
		for i, l := range paraLines {
			if i > 0 {
				sb.WriteString("<br>")
			}
			sb.WriteString(depInline(l))
		}
		sb.WriteString("</p>")
		paraLines = paraLines[:0]
	}
	flushList := func() {
		if len(listItems) == 0 {
			return
		}
		sb.WriteString("<ul>")
		for _, item := range listItems {
			sb.WriteString("<li>")
			sb.WriteString(depInline(item))
			sb.WriteString("</li>")
		}
		sb.WriteString("</ul>")
		listItems = listItems[:0]
	}
	flushCode := func() {
		if len(codeLines) == 0 {
			return
		}
		sb.WriteString("<pre><code>")
		sb.WriteString(html.EscapeString(strings.Join(codeLines, "\n")))
		sb.WriteString("</code></pre>")
		codeLines = codeLines[:0]
	}

	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "```") {
			if inCode {
				inCode = false
				flushCode()
			} else {
				flushPara()
				flushList()
				inCode = true
			}
			continue
		}
		if inCode {
			codeLines = append(codeLines, line)
			continue
		}
		switch {
		case strings.HasPrefix(line, "### "):
			flushPara()
			flushList()
			sb.WriteString("<h5>")
			sb.WriteString(depInline(strings.TrimPrefix(line, "### ")))
			sb.WriteString("</h5>")
		case strings.HasPrefix(line, "## "):
			flushPara()
			flushList()
			sb.WriteString("<h4>")
			sb.WriteString(depInline(strings.TrimPrefix(line, "## ")))
			sb.WriteString("</h4>")
		case strings.HasPrefix(line, "# "):
			flushPara()
			flushList()
			sb.WriteString("<h3>")
			sb.WriteString(depInline(strings.TrimPrefix(line, "# ")))
			sb.WriteString("</h3>")
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			flushPara()
			listItems = append(listItems, line[2:])
		case strings.TrimSpace(line) == "":
			flushPara()
			flushList()
		default:
			flushList()
			paraLines = append(paraLines, line)
		}
	}
	flushPara()
	flushList()

	return template.HTML(sb.String())
}

func depCVSSScore(v DepVulnerability) string {
	if v.CVSSv3 != nil {
		return fmt.Sprintf("%.1f v3", v.CVSSv3.BaseScore)
	}
	if v.CVSSv2 != nil {
		return fmt.Sprintf("%.1f v2", v.CVSSv2.Score)
	}
	return "—"
}

func depSevClass(severity string) string {
	switch depNormSeverity(severity) {
	case "critical":
		return "sev-critical"
	case "high":
		return "sev-high"
	case "medium":
		return "sev-medium"
	case "low":
		return "sev-low"
	default:
		return "sev-info"
	}
}

func depSevLabel(severity string) string {
	switch depNormSeverity(severity) {
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
