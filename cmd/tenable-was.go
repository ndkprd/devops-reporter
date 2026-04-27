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
	RegisterSource("tenable-was", &ReportSource{
		DefaultTitle: "Tenable WAS Scan Report",
		Parse: func(data []byte, title string) (ReportData, error) {
			var report WasReport
			if err := json.Unmarshal(data, &report); err != nil {
				return ReportData{}, err
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

// ── Adapter ──────────────────────────────────────────────────────

var wasSeverityOrder = []string{"critical", "high", "medium", "low", "info"}

func BuildWasReportData(r WasReport, title string) ReportData {
	groupMap := make(map[string][]WasFinding)
	for _, f := range r.Findings {
		risk := strings.ToLower(f.RiskFactor)
		if risk == "" {
			risk = "info"
		}
		groupMap[risk] = append(groupMap[risk], f)
	}

	total := len(r.Findings)
	critical := len(groupMap["critical"])
	high := len(groupMap["high"])
	medium := len(groupMap["medium"])
	low := len(groupMap["low"])
	info := len(groupMap["info"])

	hasIssues := critical+high+medium > 0
	statusLine := "No significant findings"
	if hasIssues {
		statusLine = fmt.Sprintf("%s · %d critical, %d high, %d medium",
			pluralise(total, "finding", "findings"), critical, high, medium)
	}

	startedAt, finishedAt, duration := wasDuration(r.Scan.StartedAt, r.Scan.FinalizedAt)

	// ── Summary overview table ─────────────────────────────────────
	overviewCols := []string{"Risk", "Name", "Family", "URI"}
	var overviewGroups []SectionGroup
	for _, sev := range wasSeverityOrder {
		findings, ok := groupMap[sev]
		if !ok {
			continue
		}
		sort.Slice(findings, func(i, j int) bool { return findings[i].Name < findings[j].Name })
		cls, lbl := wasRiskCanonical(sev)
		rows := make([][]template.HTML, 0, len(findings))
		for _, f := range findings {
			uriCell := template.HTML(template.HTMLEscapeString(f.URI))
			if f.URI != "" {
				uriCell = LinkHTML(f.URI, f.URI)
			}
			rows = append(rows, []template.HTML{
				BadgeHTML("badge "+cls, lbl),
				template.HTML(fmt.Sprintf(`<strong>%s</strong>`, template.HTMLEscapeString(f.Name))),
				template.HTML(template.HTMLEscapeString(f.Family)),
				uriCell,
			})
		}
		overviewGroups = append(overviewGroups, SectionGroup{
			Name:    lbl,
			Count:   pluralise(len(findings), "finding", "findings"),
			Class:   cls,
			Columns: overviewCols,
			Rows:    rows,
		})
	}

	// ── Detail cards (one group per severity, one card per finding) ─
	var cardGroups []SectionGroup
	for _, sev := range wasSeverityOrder {
		findings, ok := groupMap[sev]
		if !ok {
			continue
		}
		cls, lbl := wasRiskCanonical(sev)
		cards := make([]Card, 0, len(findings))
		for _, f := range findings {
			header := wasFindingHeader(f, cls, lbl)
			body := wasFindingBody(f)
			tags := wasFindingTags(f)
			cards = append(cards, Card{Header: header, Body: body, Tags: tags})
		}
		cardGroups = append(cardGroups, SectionGroup{
			Name:  lbl,
			Count: pluralise(len(findings), "finding", "findings"),
			Class: cls,
			Cards: cards,
		})
	}

	return ReportData{
		Title:       title,
		Eyebrow:     "Web Application Scan",
		Subtitle:    r.Scan.Target,
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Status:      statusFromBool(hasIssues),
		StatusLine:  statusLine,
		Meta: []KV{
			{Label: "Scan Target", Value: r.Scan.Target},
			{Label: "Scan Name", Value: r.Config.Name},
			{Label: "Template", Value: r.Template.Name},
			{Label: "Scan ID", Value: r.Scan.ScanID},
			{Label: "Status", Value: r.Scan.Status},
			{Label: "Started", Value: startedAt},
			{Label: "Finished", Value: finishedAt},
			{Label: "Duration", Value: duration},
		},
		Summary: []StatCard{
			{Number: fmt.Sprintf("%d", total), Label: "Total", Variant: "primary"},
			{Number: fmt.Sprintf("%d", critical), Label: "Critical", Variant: "critical"},
			{Number: fmt.Sprintf("%d", high), Label: "High", Variant: "high"},
			{Number: fmt.Sprintf("%d", medium), Label: "Medium", Variant: "medium"},
			{Number: fmt.Sprintf("%d", low), Label: "Low", Variant: "low"},
			{Number: fmt.Sprintf("%d", info), Label: "Info", Variant: "info"},
		},
		Sections: []Section{
			{Kind: "table", Title: "Findings Overview", Groups: overviewGroups, Empty: "No findings."},
			{Kind: "cards", Title: "Finding Details", Groups: cardGroups, Empty: "No findings."},
		},
		Footer: FooterInfo{
			Total: pluralise(total, "finding", "findings"),
			Brand: "devops-reporter · tenable-was",
		},
	}
}

func wasFindingHeader(f WasFinding, cls, lbl string) template.HTML {
	cvss := ""
	if f.CVSSv4 != nil {
		cvss = fmt.Sprintf(`<span style="opacity:.6;font-size:.85em"> CVSSv4 %.1f</span>`, *f.CVSSv4)
	} else if f.CVSSv3 != nil {
		cvss = fmt.Sprintf(`<span style="opacity:.6;font-size:.85em"> CVSSv3 %.1f</span>`, *f.CVSSv3)
	}
	uri := ""
	if f.URI != "" {
		uri = fmt.Sprintf(` <span class="td-file">%s</span>`, template.HTMLEscapeString(f.URI))
	}
	return template.HTML(fmt.Sprintf(`%s <strong>%s</strong>%s%s`,
		BadgeHTML("badge "+cls, lbl),
		template.HTMLEscapeString(f.Name),
		cvss,
		uri,
	))
}

func wasFindingBody(f WasFinding) template.HTML {
	var sb strings.Builder
	writeSection := func(label, text string, isCode bool) {
		if text == "" {
			return
		}
		if label != "" {
			sb.WriteString(fmt.Sprintf(`<p class="finding-label"><strong>%s</strong></p>`, label))
		}
		if isCode {
			sb.WriteString(fmt.Sprintf(`<pre class="finding-code">%s</pre>`, template.HTMLEscapeString(text)))
		} else {
			sb.WriteString(fmt.Sprintf(`<p>%s</p>`, template.HTMLEscapeString(text)))
		}
	}
	writeSection("", f.Synopsis, false)
	writeSection("Description", f.Description, false)
	writeSection("Solution", f.Solution, false)
	writeSection("Output", f.Output, true)
	writeSection("Proof", f.Proof, true)
	if len(f.SeeAlso) > 0 {
		sb.WriteString(`<p class="finding-label"><strong>See Also</strong></p><ul class="finding-refs">`)
		for _, url := range f.SeeAlso {
			sb.WriteString(fmt.Sprintf(`<li><a href="%s" target="_blank" rel="noopener noreferrer">%s</a></li>`,
				template.HTMLEscapeString(url), template.HTMLEscapeString(url)))
		}
		sb.WriteString(`</ul>`)
	}
	return template.HTML(sb.String())
}

func wasFindingTags(f WasFinding) []string {
	tags := make([]string, 0, len(f.CVEs)+len(f.CWE)+len(f.OWASP))
	tags = append(tags, f.CVEs...)
	for _, cwe := range f.CWE {
		tags = append(tags, fmt.Sprintf("CWE-%d", cwe))
	}
	tags = append(tags, f.OWASP...)
	return tags
}

// ── Helpers ──────────────────────────────────────────────────────

func wasRiskCanonical(risk string) (cssClass, label string) {
	switch strings.ToLower(risk) {
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
