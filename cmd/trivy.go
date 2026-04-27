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
	RegisterSource("trivy", &ReportSource{
		DefaultTitle: "Trivy Vulnerability Report",
		Parse: func(data []byte, title string) (ReportData, error) {
			var report TrivyReport
			if err := json.Unmarshal(data, &report); err != nil {
				return ReportData{}, err
			}
			return BuildTrivyReportData(report, title), nil
		},
	})
}

// ── Input types ──────────────────────────────────────────────────

type TrivyVulnerability struct {
	VulnerabilityID  string   `json:"VulnerabilityID"`
	PkgID            string   `json:"PkgID"`
	PkgName          string   `json:"PkgName"`
	InstalledVersion string   `json:"InstalledVersion"`
	FixedVersion     string   `json:"FixedVersion"`
	Status           string   `json:"Status"`
	PrimaryURL       string   `json:"PrimaryURL"`
	Description      string   `json:"Description"`
	Severity         string   `json:"Severity"`
	References       []string `json:"References"`
}

type TrivyPackage struct {
	ID         string   `json:"ID"`
	Name       string   `json:"Name"`
	Version    string   `json:"Version"`
	Arch       string   `json:"Arch"`
	SrcName    string   `json:"SrcName"`
	SrcVersion string   `json:"SrcVersion"`
	Licenses   []string `json:"Licenses"`
	Maintainer string   `json:"Maintainer"`
	Digest     string   `json:"Digest"`
	Layer      struct {
		DiffID string `json:"DiffID"`
	} `json:"Layer"`
}

type TrivyLayer struct {
	Size   int64  `json:"Size"`
	DiffID string `json:"DiffID"`
}

type TrivyResult struct {
	Target          string               `json:"Target"`
	Class           string               `json:"Class"`
	Type            string               `json:"Type"`
	Packages        []TrivyPackage       `json:"Packages"`
	Vulnerabilities []TrivyVulnerability `json:"Vulnerabilities"`
}

type TrivyReport struct {
	SchemaVersion int    `json:"SchemaVersion"`
	ReportID      string `json:"ReportID"`
	CreatedAt     string `json:"CreatedAt"`
	ArtifactID    string `json:"ArtifactID"`
	ArtifactName  string `json:"ArtifactName"`
	ArtifactType  string `json:"ArtifactType"`
	Trivy         struct {
		Version string `json:"Version"`
	} `json:"Trivy"`
	Metadata struct {
		Size int64 `json:"Size"`
		OS   struct {
			Family string `json:"Family"`
			Name   string `json:"Name"`
		} `json:"OS"`
		ImageID     string       `json:"ImageID"`
		RepoTags    []string     `json:"RepoTags"`
		RepoDigests []string     `json:"RepoDigests"`
		Layers      []TrivyLayer `json:"Layers"`
		ImageConfig struct {
			Architecture string `json:"architecture"`
			Created      string `json:"created"`
			OS           string `json:"os"`
			Config       struct {
				Labels map[string]string `json:"Labels"`
			} `json:"config"`
		} `json:"ImageConfig"`
	} `json:"Metadata"`
	Results []TrivyResult `json:"Results"`
}

// ── Adapter ──────────────────────────────────────────────────────

var trivySeverityOrder = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"}

func BuildTrivyReportData(report TrivyReport, title string) ReportData {
	sevMap := make(map[string][]TrivyVulnerability)
	var totalVulns, critical, high, medium, low, unknown, fixable, totalPkgs int

	type pkgGroup struct {
		target string
		typ    string
		pkgs   []TrivyPackage
	}
	var pkgGroups []pkgGroup

	for _, result := range report.Results {
		for _, v := range result.Vulnerabilities {
			sev := strings.ToUpper(v.Severity)
			sevMap[sev] = append(sevMap[sev], v)
			totalVulns++
			switch sev {
			case "CRITICAL":
				critical++
			case "HIGH":
				high++
			case "MEDIUM":
				medium++
			case "LOW":
				low++
			default:
				unknown++
			}
			if v.FixedVersion != "" {
				fixable++
			}
		}
		if len(result.Packages) > 0 {
			pkgs := make([]TrivyPackage, len(result.Packages))
			copy(pkgs, result.Packages)
			sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
			totalPkgs += len(pkgs)
			pkgGroups = append(pkgGroups, pkgGroup{target: result.Target, typ: result.Type, pkgs: pkgs})
		}
	}

	hasIssues := totalVulns > 0
	statusLine := "No vulnerabilities found"
	if hasIssues {
		statusLine = fmt.Sprintf("%s · %d critical, %d high, %d fixable", pluralise(totalVulns, "vulnerability", "vulnerabilities"), critical, high, fixable)
	}

	os := ""
	if report.Metadata.OS.Family != "" {
		os = report.Metadata.OS.Family
		if report.Metadata.OS.Name != "" {
			os += " " + report.Metadata.OS.Name
		}
	}

	labels := report.Metadata.ImageConfig.Config.Labels
	revision := labels["org.opencontainers.image.revision"]
	if len(revision) > 7 {
		revision = revision[:7]
	}
	repoDigest := ""
	if len(report.Metadata.RepoDigests) > 0 {
		repoDigest = report.Metadata.RepoDigests[0]
	}

	// Image details extra panel
	imagePanel := renderTrivyImagePanel(
		report.ArtifactName, report.Metadata.ImageID,
		trivyFormatSize(report.Metadata.Size),
		report.Metadata.ImageConfig.Architecture,
		trivyFormatTimestamp(report.Metadata.ImageConfig.Created),
		os, repoDigest, labels["maintainer"],
		labels["org.opencontainers.image.source"],
		revision, len(report.Metadata.Layers),
	)

	// Vulnerability sections
	vulnCols := []string{"CVE", "Package", "Installed", "Fixed", "Status", "Description"}
	var vulnGroups []SectionGroup
	for _, sev := range trivySeverityOrder {
		vulns, ok := sevMap[sev]
		if !ok {
			continue
		}
		sort.Slice(vulns, func(i, j int) bool { return vulns[i].PkgName < vulns[j].PkgName })
		cls, lbl := trivySevCanonical(sev)
		rows := make([][]template.HTML, 0, len(vulns))
		for _, v := range vulns {
			cveCell := template.HTML(template.HTMLEscapeString(v.VulnerabilityID))
			if v.PrimaryURL != "" {
				cveCell = LinkHTML(v.PrimaryURL, v.VulnerabilityID)
			}
			fixCell := template.HTML(template.HTMLEscapeString(v.FixedVersion))
			if v.FixedVersion == "" {
				fixCell = template.HTML(`<em style="opacity:.5">—</em>`)
			}
			rows = append(rows, []template.HTML{
				template.HTML(fmt.Sprintf(`<span class="td-cve">%s</span>`, cveCell)),
				template.HTML(fmt.Sprintf(`<strong>%s</strong>`, template.HTMLEscapeString(v.PkgName))),
				MonoHTML(v.InstalledVersion),
				fixCell,
				MonoHTML(v.Status),
				template.HTML(template.HTMLEscapeString(v.Description)),
			})
		}
		vulnGroups = append(vulnGroups, SectionGroup{
			Name:    lbl,
			Count:   pluralise(len(vulns), "vulnerability", "vulnerabilities"),
			Class:   cls,
			Columns: vulnCols,
			Rows:    rows,
		})
	}

	// Package sections
	pkgCols := []string{"Package", "Version", "Arch", "Licenses", "Layer"}
	var pkgSectionGroups []SectionGroup
	for _, pg := range pkgGroups {
		rows := make([][]template.HTML, 0, len(pg.pkgs))
		for _, p := range pg.pkgs {
			layer := ShortHash(p.Layer.DiffID)
			rows = append(rows, []template.HTML{
				template.HTML(fmt.Sprintf(`<strong>%s</strong>`, template.HTMLEscapeString(p.Name))),
				MonoHTML(p.Version),
				template.HTML(template.HTMLEscapeString(p.Arch)),
				template.HTML(template.HTMLEscapeString(strings.Join(p.Licenses, ", "))),
				MonoHTML(layer),
			})
		}
		label := pg.target
		if pg.typ != "" {
			label += " (" + pg.typ + ")"
		}
		pkgSectionGroups = append(pkgSectionGroups, SectionGroup{
			Name:    label,
			Count:   pluralise(len(pg.pkgs), "package", "packages"),
			Columns: pkgCols,
			Rows:    rows,
		})
	}

	sections := []Section{
		{Kind: "table", Title: "Vulnerabilities by Severity", Groups: vulnGroups, Empty: "No vulnerabilities found."},
	}
	if len(pkgSectionGroups) > 0 {
		sections = append(sections, Section{
			Kind:   "table",
			Title:  "Installed Packages",
			Groups: pkgSectionGroups,
			Empty:  "No package data.",
		})
	}

	return ReportData{
		Title:       title,
		Eyebrow:     "Vulnerability Scan",
		Subtitle:    report.ArtifactName,
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Status:      statusFromBool(hasIssues),
		StatusLine:  statusLine,
		Meta: []KV{
			{Label: "Artifact Type", Value: report.ArtifactType},
			{Label: "OS", Value: os},
			{Label: "Scan Date", Value: trivyFormatTimestamp(report.CreatedAt)},
			{Label: "Trivy Version", Value: report.Trivy.Version},
		},
		ExtraPanels: []template.HTML{imagePanel},
		Summary: []StatCard{
			{Number: fmt.Sprintf("%d", totalVulns), Label: "Total", Variant: "primary"},
			{Number: fmt.Sprintf("%d", critical), Label: "Critical", Variant: "critical"},
			{Number: fmt.Sprintf("%d", high), Label: "High", Variant: "high"},
			{Number: fmt.Sprintf("%d", medium), Label: "Medium", Variant: "medium"},
			{Number: fmt.Sprintf("%d", low), Label: "Low", Variant: "low"},
			{Number: fmt.Sprintf("%d", fixable), Label: "Fixable", Variant: "fixable"},
		},
		Sections: sections,
		Footer: FooterInfo{
			Total: pluralise(totalVulns, "vulnerability", "vulnerabilities") + " · " + pluralise(totalPkgs, "package", "packages"),
			Brand: "devops-reporter · trivy",
		},
	}
}

func renderTrivyImagePanel(name, imageID, size, arch, builtAt, osStr, repoDigest, maintainer, source, revision string, layerCount int) template.HTML {
	var meta strings.Builder
	writeMeta := func(label, val string) {
		if val == "" {
			return
		}
		meta.WriteString(fmt.Sprintf(`<span><strong>%s</strong> %s</span>`,
			template.HTMLEscapeString(label), template.HTMLEscapeString(val)))
	}
	writeMeta("Size", size)
	writeMeta("Arch", arch)
	writeMeta("Built", builtAt)
	writeMeta("OS", osStr)
	writeMeta("ID", ShortHash(imageID))
	writeMeta("Layers", fmt.Sprintf("%d", layerCount))
	writeMeta("Revision", revision)
	writeMeta("Maintainer", maintainer)
	if repoDigest != "" {
		meta.WriteString(fmt.Sprintf(`<span class="full"><strong>Digest</strong> %s</span>`, template.HTMLEscapeString(repoDigest)))
	}
	if source != "" {
		meta.WriteString(fmt.Sprintf(`<span class="full"><strong>Source</strong> %s</span>`, template.HTMLEscapeString(source)))
	}

	return template.HTML(fmt.Sprintf(`<div class="extra-panel">
  <div class="extra-panel-eyebrow">Image Details</div>
  <div class="extra-panel-name">%s</div>
  <div class="extra-panel-meta">%s</div>
</div>`, template.HTMLEscapeString(name), meta.String()))
}

// ── Helpers ──────────────────────────────────────────────────────

func trivySevCanonical(sev string) (cssClass, label string) {
	switch strings.ToUpper(sev) {
	case "CRITICAL":
		return "sev-critical", "Critical"
	case "HIGH":
		return "sev-high", "High"
	case "MEDIUM":
		return "sev-medium", "Medium"
	case "LOW":
		return "sev-low", "Low"
	default:
		return "sev-unknown", "Unknown"
	}
}

func trivyFormatTimestamp(s string) string {
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

func trivyFormatSize(bytes int64) string {
	if bytes == 0 {
		return ""
	}
	const (
		mb = 1024 * 1024
		gb = 1024 * mb
	)
	if bytes >= gb {
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
}
