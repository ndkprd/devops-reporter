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
	RegisterSource("cyclonedx", &ReportSource{
		DefaultTitle: "Software Bill of Materials",
		Parse: func(data []byte, title string) (ReportData, error) {
			var sbom CdxSBOM
			if err := json.Unmarshal(data, &sbom); err != nil {
				return ReportData{}, err
			}
			return BuildCdxReportData(sbom, title), nil
		},
	})
}

// ── Input types ──────────────────────────────────────────────────

type CdxLicenseEntry struct {
	License struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"license"`
	Expression string `json:"expression"`
}

type CdxHash struct {
	Alg     string `json:"alg"`
	Content string `json:"content"`
}

type CdxComponent struct {
	Group       string            `json:"group"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	PURL        string            `json:"purl"`
	Type        string            `json:"type"`
	BOMRef      string            `json:"bom-ref"`
	Licenses    []CdxLicenseEntry `json:"licenses"`
	Hashes      []CdxHash         `json:"hashes"`
}

type CdxTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Group   string `json:"group"`
}

type CdxSBOM struct {
	BOMFormat    string `json:"bomFormat"`
	SpecVersion  string `json:"specVersion"`
	SerialNumber string `json:"serialNumber"`
	Version      int    `json:"version"`
	Metadata     struct {
		Timestamp string `json:"timestamp"`
		Tools     struct {
			Components []CdxTool `json:"components"`
		} `json:"tools"`
		Component  CdxComponent `json:"component"`
		Lifecycles []struct {
			Phase string `json:"phase"`
		} `json:"lifecycles"`
	} `json:"metadata"`
	Components []CdxComponent `json:"components"`
}

// ── Adapter ──────────────────────────────────────────────────────

var cdxEcosystemOrder = []string{"npm", "pypi", "maven", "gem", "cargo", "golang", "nuget", "composer", "other"}

func BuildCdxReportData(sbom CdxSBOM, title string) ReportData {
	groupMap := make(map[string][]CdxComponent)
	licenseSet := make(map[string]bool)

	total := len(sbom.Components)
	var libraries, applications, unlicensed int

	for _, c := range sbom.Components {
		eco := cdxEcosystemFromPURL(c.PURL)
		groupMap[eco] = append(groupMap[eco], c)
		switch c.Type {
		case "library":
			libraries++
		case "application":
			applications++
		}
		lic := cdxLicenseString(c.Licenses)
		if lic == "" {
			unlicensed++
		} else {
			licenseSet[lic] = true
		}
	}

	hasIssues := unlicensed > 0
	statusLine := "All components licensed"
	if hasIssues {
		statusLine = fmt.Sprintf("%s without license information", pluralise(unlicensed, "component", "components"))
	}

	tool := ""
	if len(sbom.Metadata.Tools.Components) > 0 {
		t := sbom.Metadata.Tools.Components[0]
		if t.Version != "" {
			tool = t.Name + " " + t.Version
		} else {
			tool = t.Name
		}
	}

	lifecycles := make([]string, 0, len(sbom.Metadata.Lifecycles))
	for _, l := range sbom.Metadata.Lifecycles {
		if l.Phase != "" {
			lifecycles = append(lifecycles, l.Phase)
		}
	}

	main := sbom.Metadata.Component
	mainLicense := cdxLicenseString(main.Licenses)

	// Extra panel: main component metadata
	mainPanelHTML := renderCdxMainComponent(main, mainLicense)

	// Build ecosystem groups
	cols := []string{"Component", "Version", "Type", "License", "PURL"}
	var groups []SectionGroup

	seen := make(map[string]bool)
	for _, eco := range cdxEcosystemOrder {
		comps, ok := groupMap[eco]
		if !ok {
			continue
		}
		seen[eco] = true
		sort.Slice(comps, func(i, j int) bool { return comps[i].Name < comps[j].Name })
		groups = append(groups, cdxEcosystemGroup(eco, comps, cols))
	}
	// remaining ecosystems not in the canonical order
	remaining := make([]string, 0)
	for eco := range groupMap {
		if !seen[eco] {
			remaining = append(remaining, eco)
		}
	}
	sort.Strings(remaining)
	for _, eco := range remaining {
		comps := groupMap[eco]
		sort.Slice(comps, func(i, j int) bool { return comps[i].Name < comps[j].Name })
		groups = append(groups, cdxEcosystemGroup(eco, comps, cols))
	}

	createdAt := cdxFormatTimestamp(sbom.Metadata.Timestamp)

	return ReportData{
		Title:       title,
		Eyebrow:     "SBOM Report",
		Subtitle:    fmt.Sprintf("%s · %d ecosystems · %d unique licenses", pluralise(total, "component", "components"), len(groupMap), len(licenseSet)),
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Status:      statusFromBool(hasIssues),
		StatusLine:  statusLine,
		Meta: []KV{
			{Label: "BOM Format", Value: sbom.BOMFormat + " " + sbom.SpecVersion},
			{Label: "Created At", Value: createdAt},
			{Label: "Lifecycle", Value: strings.Join(lifecycles, ", ")},
			{Label: "Generator", Value: tool},
			{Label: "BOM Version", Value: fmt.Sprintf("%d", sbom.Version)},
			{Label: "Serial Number", Value: sbom.SerialNumber},
		},
		ExtraPanels: []template.HTML{mainPanelHTML},
		Summary: []StatCard{
			{Number: fmt.Sprintf("%d", total), Label: "Total", Variant: "primary"},
			{Number: fmt.Sprintf("%d", libraries), Label: "Libraries", Variant: "info"},
			{Number: fmt.Sprintf("%d", applications), Label: "Apps", Variant: "info"},
			{Number: fmt.Sprintf("%d", len(licenseSet)), Label: "Licenses", Variant: "pass"},
			{Number: fmt.Sprintf("%d", unlicensed), Label: "Unlicensed", Variant: "warn"},
			{Number: fmt.Sprintf("%d", len(groupMap)), Label: "Ecosystems", Variant: "primary"},
		},
		Sections: []Section{
			{Kind: "table", Title: "Components by Ecosystem", Groups: groups, Empty: "No components in BOM."},
		},
		Footer: FooterInfo{
			Total: pluralise(total, "component", "components"),
			Brand: "devops-reporter · cyclonedx",
		},
	}
}

func cdxEcosystemGroup(eco string, comps []CdxComponent, cols []string) SectionGroup {
	rows := make([][]template.HTML, 0, len(comps))
	for _, c := range comps {
		lic := cdxLicenseString(c.Licenses)
		licCell := template.HTML(template.HTMLEscapeString(lic))
		if lic == "" {
			licCell = template.HTML(`<em style="opacity:.5">—</em>`)
		}
		name := c.Name
		if c.Group != "" {
			name = c.Group + "/" + c.Name
		}
		rows = append(rows, []template.HTML{
			template.HTML(fmt.Sprintf(`<strong>%s</strong>`, template.HTMLEscapeString(name))),
			MonoHTML(c.Version),
			template.HTML(template.HTMLEscapeString(c.Type)),
			licCell,
			MonoHTML(cdxShortPurl(c.PURL)),
		})
	}
	return SectionGroup{
		Name:    eco,
		Count:   pluralise(len(comps), "component", "components"),
		Columns: cols,
		Rows:    rows,
	}
}

func renderCdxMainComponent(c CdxComponent, license string) template.HTML {
	name := c.Name
	if c.Group != "" {
		name = c.Group + "/" + c.Name
	}
	ver := ""
	if c.Version != "" {
		ver = fmt.Sprintf(` <span style="font-weight:400;opacity:.7">v%s</span>`, template.HTMLEscapeString(c.Version))
	}

	var meta strings.Builder
	if c.Type != "" {
		meta.WriteString(fmt.Sprintf(`<span><strong>Type</strong> %s</span>`, template.HTMLEscapeString(c.Type)))
	}
	if license != "" {
		meta.WriteString(fmt.Sprintf(`<span><strong>License</strong> %s</span>`, template.HTMLEscapeString(license)))
	}
	if c.PURL != "" {
		meta.WriteString(fmt.Sprintf(`<span><strong>PURL</strong> %s</span>`, template.HTMLEscapeString(cdxShortPurl(c.PURL))))
	}

	desc := ""
	if c.Description != "" {
		desc = fmt.Sprintf(`<div class="main-comp-desc">%s</div>`, template.HTMLEscapeString(c.Description))
	}

	return template.HTML(fmt.Sprintf(`<div class="extra-panel">
  <div class="extra-panel-eyebrow">Described Application</div>
  <div class="main-comp-name">%s%s</div>
  %s
  <div class="main-comp-meta">%s</div>
</div>`, template.HTMLEscapeString(name), ver, desc, meta.String()))
}

// ── Helpers ──────────────────────────────────────────────────────

func cdxFormatTimestamp(s string) string {
	layouts := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.999999-07:00",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC().Format("2006-01-02 15:04:05 UTC")
		}
	}
	return s
}

func cdxEcosystemFromPURL(purl string) string {
	if !strings.HasPrefix(purl, "pkg:") {
		return "other"
	}
	rest := purl[4:]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "other"
	}
	return rest[:slash]
}

func cdxLicenseString(licenses []CdxLicenseEntry) string {
	for _, l := range licenses {
		if l.License.ID != "" {
			return l.License.ID
		}
		if l.License.Name != "" {
			return l.License.Name
		}
		if l.Expression != "" {
			return l.Expression
		}
	}
	return ""
}

func cdxShortPurl(purl string) string {
	if !strings.HasPrefix(purl, "pkg:") {
		return purl
	}
	rest := purl[4:]
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return purl
	}
	return rest[slash+1:]
}
