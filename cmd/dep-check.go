package main

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"
)

func init() {
	RegisterSource("dependency-check", &ReportSource{
		DefaultTitle: "Dependency-Check Report",
		Parse: func(data []byte, title string) (ReportData, error) {
			var report DepReport
			if err := json.Unmarshal(data, &report); err != nil {
				return ReportData{}, err
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

// ── Adapter ──────────────────────────────────────────────────────

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

func BuildDepReportData(r DepReport, title string) ReportData {
	var totalVulns, critical, high, medium, low, info int
	var vulnDeps, cleanDeps []DepDependency

	for _, d := range r.Dependencies {
		if len(d.Vulnerabilities) == 0 {
			cleanDeps = append(cleanDeps, d)
			continue
		}
		sort.Slice(d.Vulnerabilities, func(i, j int) bool {
			return depSeverityRank[strings.ToLower(d.Vulnerabilities[i].Severity)] <
				depSeverityRank[strings.ToLower(d.Vulnerabilities[j].Severity)]
		})
		for _, v := range d.Vulnerabilities {
			totalVulns++
			switch depNormSeverity(v.Severity) {
			case "critical":
				critical++
			case "high":
				high++
			case "medium":
				medium++
			case "low":
				low++
			default:
				info++
			}
		}
		vulnDeps = append(vulnDeps, d)
	}

	sort.Slice(vulnDeps, func(i, j int) bool {
		ri := depSeverityRank[depWorstSeverity(vulnDeps[i])]
		rj := depSeverityRank[depWorstSeverity(vulnDeps[j])]
		if ri != rj {
			return ri < rj
		}
		return vulnDeps[i].FileName < vulnDeps[j].FileName
	})

	totalDeps := len(r.Dependencies)
	vulnerableDeps := len(vulnDeps)
	hasIssues := vulnerableDeps > 0

	statusLine := "No vulnerable dependencies found"
	if hasIssues {
		statusLine = fmt.Sprintf("%s · %d critical, %d high, %d medium",
			pluralise(vulnerableDeps, "vulnerable dependency", "vulnerable dependencies"), critical, high, medium)
	}

	// ── Vulnerability summary table (by severity) ─────────────────
	sevOrder := []string{"critical", "high", "medium", "low", "info"}
	type vulnEntry struct{ dep, cve, cvss string }
	groupMap := make(map[string][]vulnEntry)
	for _, d := range vulnDeps {
		for _, v := range d.Vulnerabilities {
			norm := depNormSeverity(v.Severity)
			groupMap[norm] = append(groupMap[norm], vulnEntry{
				dep:  d.FileName,
				cve:  v.Name,
				cvss: depCVSSScore(v),
			})
		}
	}

	vulnCols := []string{"Dependency", "CVE / ID", "CVSS"}
	var vulnGroups []SectionGroup
	for _, sev := range sevOrder {
		entries, ok := groupMap[sev]
		if !ok {
			continue
		}
		cls, lbl := depSevCanonical(sev)
		rows := make([][]template.HTML, 0, len(entries))
		for _, e := range entries {
			rows = append(rows, []template.HTML{
				template.HTML(fmt.Sprintf(`<span class="td-file">%s</span>`, template.HTMLEscapeString(e.dep))),
				template.HTML(fmt.Sprintf(`<strong>%s</strong>`, template.HTMLEscapeString(e.cve))),
				MonoHTML(e.cvss),
			})
		}
		vulnGroups = append(vulnGroups, SectionGroup{
			Name:    lbl,
			Count:   pluralise(len(entries), "vulnerability", "vulnerabilities"),
			Class:   cls,
			Columns: vulnCols,
			Rows:    rows,
		})
	}

	// ── Vulnerable dependencies detail (one card per dep) ───────────
	depCards := make([]Card, 0, len(vulnDeps))
	for _, d := range vulnDeps {
		header := template.HTML(fmt.Sprintf(`<strong class="dep-card-name">%s</strong> <span style="opacity:.6;font-size:.85em">%s</span>`,
			template.HTMLEscapeString(d.FileName),
			pluralise(len(d.Vulnerabilities), "vulnerability", "vulnerabilities"),
		))

		var bodyB strings.Builder
		for i, v := range d.Vulnerabilities {
			if i > 0 {
				bodyB.WriteString(`<hr class="vuln-sep">`)
			}
			cls, lbl := depSevCanonical(depNormSeverity(v.Severity))
			bodyB.WriteString(fmt.Sprintf(`<div class="vuln-entry-head">%s <strong class="vuln-name">%s</strong> <span class="vuln-cvss">%s</span></div>`,
				BadgeHTML("badge "+cls, lbl),
				template.HTMLEscapeString(v.Name),
				template.HTMLEscapeString(depCVSSScore(v)),
			))
			if v.Description != "" {
				bodyB.WriteString(`<div class="vuln-desc">`)
				bodyB.WriteString(string(depMarkdownToHTML(v.Description)))
				bodyB.WriteString(`</div>`)
			}
			if len(v.References) > 0 {
				bodyB.WriteString(`<p class="dep-refs">`)
				for j, ref := range v.References {
					if j > 0 {
						bodyB.WriteString(" · ")
					}
					if ref.URL != "" {
						bodyB.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
							template.HTMLEscapeString(ref.URL),
							template.HTMLEscapeString(depRefLabel(ref)),
						))
					} else {
						bodyB.WriteString(template.HTMLEscapeString(depRefLabel(ref)))
					}
				}
				bodyB.WriteString(`</p>`)
			}
		}

		var tags []string
		for _, v := range d.Vulnerabilities {
			tags = append(tags, v.CWEs...)
		}
		depCards = append(depCards, Card{
			Header: header,
			Body:   template.HTML(bodyB.String()),
			Tags:   tags,
		})
	}
	var depCardGroups []SectionGroup
	if len(depCards) > 0 {
		depCardGroups = []SectionGroup{{Cards: depCards}}
	}

	// ── Credits raw section ────────────────────────────────────────
	var creditsHTML strings.Builder
	if len(r.ProjectInfo.Credits) > 0 {
		creditKeys := make([]string, 0, len(r.ProjectInfo.Credits))
		for k := range r.ProjectInfo.Credits {
			creditKeys = append(creditKeys, k)
		}
		sort.Strings(creditKeys)
		creditsHTML.WriteString(`<ul class="dep-credits">`)
		for _, k := range creditKeys {
			creditsHTML.WriteString(fmt.Sprintf(`<li><strong>%s</strong>: %s</li>`,
				template.HTMLEscapeString(k),
				template.HTMLEscapeString(r.ProjectInfo.Credits[k]),
			))
		}
		creditsHTML.WriteString(`</ul>`)
	}

	reportDate := depFormatTimestamp(r.ProjectInfo.ReportDate)

	sections := []Section{
		{Kind: "table", Title: "Vulnerabilities by Severity", Groups: vulnGroups, Empty: "No vulnerabilities found."},
		{Kind: "cards", Title: "Vulnerable Dependencies", Groups: depCardGroups, Empty: "No vulnerable dependencies."},
	}
	if creditsHTML.Len() > 0 {
		sections = append(sections, Section{
			Kind:  "raw",
			Title: "Data Sources & Credits",
			HTML:  template.HTML(creditsHTML.String()),
		})
	}

	return ReportData{
		Title:       title,
		Eyebrow:     "Dependency Vulnerability Scan",
		Subtitle:    r.ProjectInfo.Name,
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Status:      statusFromBool(hasIssues),
		StatusLine:  statusLine,
		Meta: []KV{
			{Label: "Project", Value: r.ProjectInfo.Name},
			{Label: "Report Date", Value: reportDate},
			{Label: "Engine Version", Value: r.ScanInfo.EngineVersion},
			{Label: "Total Dependencies", Value: fmt.Sprintf("%d", totalDeps)},
		},
		Summary: []StatCard{
			{Number: fmt.Sprintf("%d", totalVulns), Label: "Total Vulns", Variant: "primary"},
			{Number: fmt.Sprintf("%d", critical), Label: "Critical", Variant: "critical"},
			{Number: fmt.Sprintf("%d", high), Label: "High", Variant: "high"},
			{Number: fmt.Sprintf("%d", medium), Label: "Medium", Variant: "medium"},
			{Number: fmt.Sprintf("%d", low), Label: "Low", Variant: "low"},
			{Number: fmt.Sprintf("%d", vulnerableDeps), Label: "Affected Deps", Variant: "warn"},
		},
		Sections: sections,
		Footer: FooterInfo{
			Total: pluralise(vulnerableDeps, "vulnerable dependency", "vulnerable dependencies") +
				" · " + pluralise(totalDeps, "dependency", "dependencies"),
			Brand: "devops-reporter · dependency-check",
		},
	}
}

// ── Helpers ──────────────────────────────────────────────────────

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

func depSevCanonical(sev string) (cssClass, label string) {
	switch depNormSeverity(sev) {
	case "critical":
		return "sev-critical", "Critical"
	case "high":
		return "sev-high", "High"
	case "medium":
		return "sev-medium", "Medium"
	case "low":
		return "sev-low", "Low"
	default:
		return "sev-info", "Info"
	}
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

func depRefLabel(ref DepReference) string {
	if ref.Name != "" {
		return ref.Name
	}
	if ref.Source != "" {
		return ref.Source
	}
	return ref.URL
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
