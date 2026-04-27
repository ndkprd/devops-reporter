package main

import (
	"fmt"
	"html/template"
	"strings"
)

// ── Generic report types ──────────────────────────────────────────

type ReportData struct {
	Title       string
	OrgName     string
	Eyebrow     string
	Subtitle    string
	GeneratedAt string

	// Status drives header border + status banner: "issues" | "clean"
	Status     string
	StatusLine string

	Meta        []KV
	ExtraPanels []template.HTML // identity panels, shown even with --summary-only

	Summary []StatCard

	Sections    []Section
	SummaryOnly bool

	Footer FooterInfo

	CSS template.CSS // injected by main after parse
}

type KV struct {
	Label string
	Value string
}

type StatCard struct {
	Number  string
	Label   string
	Variant string // CSS suffix appended as "stat-<Variant>" e.g. "primary", "critical"
}

type FooterInfo struct {
	Total string
	Brand string
}

// Section represents a detail block in the report.
// Kind selects which named template renders it: "table" | "cards" | "raw".
type Section struct {
	Kind   string
	Title  string
	Groups []SectionGroup
	HTML   template.HTML // only populated when Kind == "raw"
	Empty  string        // message shown when Groups is empty
}

type SectionGroup struct {
	Name    string // group heading label
	Count   string // e.g. "12 issues"
	Class   string // CSS class applied to group-name for colour
	Columns []string
	Rows    [][]template.HTML // pre-rendered cells (Kind == "table")
	Cards   []Card            // (Kind == "cards")
}

type Card struct {
	Header template.HTML
	Body   template.HTML
	Tags   []string
}

// ── ReportSource contract ─────────────────────────────────────────

type ReportSource struct {
	DefaultTitle string
	Parse        func(data []byte, title string) (ReportData, error)
}

var sources = map[string]*ReportSource{}

func RegisterSource(name string, s *ReportSource) {
	sources[name] = s
}

// ── Pre-render helpers ────────────────────────────────────────────

func BadgeHTML(class, label string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<span class="badge %s">%s</span>`,
		template.HTMLEscapeString(class),
		template.HTMLEscapeString(label),
	))
}

func LinkHTML(href, label string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`,
		template.HTMLEscapeString(href),
		template.HTMLEscapeString(label),
	))
}

func MonoHTML(s string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<code class="mono">%s</code>`,
		template.HTMLEscapeString(s),
	))
}

// ShortHash trims a sha256: prefix and truncates to 12 chars.
func ShortHash(s string) string {
	s = strings.TrimPrefix(s, "sha256:")
	if len(s) <= 12 {
		return s
	}
	return s[:12] + "…"
}

// ShortRev returns the first 7 chars of a git revision.
func ShortRev(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func pluralise(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func statusFromBool(hasIssues bool) string {
	if hasIssues {
		return "issues"
	}
	return "clean"
}
