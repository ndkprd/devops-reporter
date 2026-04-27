# Agent Guidelines for devops-reporter

## Project Overview

CLI tool that reads JSON output from DevSecOps tools (ArgoCD, Kubeconform, Tenable WAS, CycloneDX SBOM, OWASP Dependency-Check, SonarQube, Trivy) and generates self-contained static HTML reports.

## Build Commands

```bash
# Build binary
go build -o devops-reporter ./cmd/

# Build Docker image
docker build -t devops-reporter .

# Run tests (none currently exist)
go test ./...

# Format code
go fmt ./...

# Vet code
go vet ./...

# Tidy dependencies
go mod tidy
```

## Architecture

The tool uses a single unified HTML template (`cmd/templates/report.html`) with an embedded CSS theme (`cmd/templates/themes/paper.css`). All seven sources share the same template and rendering pipeline.

### Data flow

1. `main.go` reads stdin JSON, selects the registered `ReportSource`, calls its `Parse` func.
2. Each source's `Build*ReportData()` adapter maps the source-specific JSON structs into a generic `ReportData` value.
3. `main.go` injects the chosen CSS into `ReportData.CSS` and executes the unified template.

### Generic data contract (`cmd/report.go`)

Every source adapter returns `ReportData`:

```go
type ReportData struct {
    Title, OrgName, Eyebrow, Subtitle, GeneratedAt string
    Status     string         // "issues" | "clean"
    StatusLine string
    Meta        []KV
    ExtraPanels []template.HTML // identity panels shown even with --summary-only
    Summary     []StatCard
    Sections    []Section
    SummaryOnly bool           // injected by main after parse
    Footer      FooterInfo
    CSS         template.CSS   // injected by main after parse
}
```

`OrgName` is injected by `main` from the `--org` flag after parse (same pattern as `SummaryOnly`). When non-empty it is rendered in the `header-stamp` block (above the "Generated" timestamp) and as `footer-org` in the report footer.

`Section` has a `Kind` discriminator (`"table"` | `"cards"` | `"raw"`) that selects which named sub-template renders it. `SectionGroup` holds either `Rows [][]template.HTML` (table) or `Cards []Card` (cards).

All cell-level formatting (badges, links, mono code) is done in Go via `BadgeHTML`, `LinkHTML`, `MonoHTML` helpers — the template never touches source-specific vocabulary.

### Canonical CSS class vocabulary

Both `paper.css` and any custom CSS must target these class names:

- **State**: `state-pass`, `state-fail`, `state-warn`, `state-neutral`
- **Severity**: `sev-critical`, `sev-high`, `sev-medium`, `sev-low`, `sev-info`, `sev-blocker`, `sev-unknown`
- **Badges**: `badge` + one state/severity modifier (e.g. `badge sev-critical`, `badge state-pass`)
- **Stat cards**: `stat-card`, `stat-<variant>` where variant matches `StatCard.Variant`
- **Group heading**: `group-heading`, `group-name`, `group-count` — the severity/state class is applied to **both** the `group-heading` div and the inner `group-name` span, so themes can colour the left border, background tint, and name text from a single class rule
- **Layout**: `report-header`, `header-issues`, `header-clean`, `header-stamp`, `stamp-org`, `scan-meta`, `status-banner`, `summary-grid`, `section`, `section-title`, `group-heading`, `report-table`, `card`, `extra-panel`, `report-footer`, `footer-org`

Source-specific status strings (Synced/OutOfSync/Healthy/CRITICAL/Blocker/etc.) are mapped to canonical classes inside each source's Go adapter — never in templates or CSS.

## Code Style Guidelines

### Go Version
- Use Go 1.26

### Structure
Each source handler is a separate file under `cmd/` with:
1. `init()` registers the source via `RegisterSource(name, &ReportSource{DefaultTitle, Parse})`
2. Input types (JSON binding structs — keep as-is)
3. `Build*ReportData(input, title string) ReportData` adapter function
4. Unexported helper functions (canonical class mappers, cell renderers, etc.)

No per-source HTML templates. No `FuncMap` on `ReportSource`. All rendering logic lives in Go.

### Adding a new source

1. Create `cmd/<source>.go`
2. Define JSON input structs
3. Write `Build<Source>ReportData` returning `ReportData` with appropriate `Sections`
4. Register in `init()` with `RegisterSource("<name>", &ReportSource{...})`
5. Add a sample JSON input to `tests/`

### Section shapes

| Kind | Use when | Key fields |
|---|---|---|
| `"table"` | Tabular rows grouped by a dimension | `Groups[].Columns`, `Groups[].Rows [][]template.HTML` |
| `"cards"` | Rich findings with long-form body text | `Groups[].Cards []Card` |
| `"raw"` | Arbitrary pre-rendered HTML blocks | `Section.HTML template.HTML` |

### Section Headers
Use `// ── Section Name ──` comment separators:
```go
// ── Input types ──────────────────────────────────────────────────

// ── Adapter ──────────────────────────────────────────────────────

// ── Helpers ──────────────────────────────────────────────────────
```

### Imports
Standard library only. Use single import block (no subgroups):
```go
import (
    "encoding/json"
    "fmt"
    "html/template"
    "sort"
    "strings"
    "time"
)
```

### Docker
- Multi-stage builds
- Builder stage: `golang:1.26-alpine`
- Runtime stage: `alpine:3.23`
- Binary placed at `/usr/local/bin/devops-reporter`

## File Structure

```
cmd/
├── main.go              # Entry point, CLI flags, CSS/template resolution
├── report.go            # Generic ReportData types + shared helpers
├── argocd.go
├── kubeconform.go
├── tenable-was.go
├── sbom-cdx.go
├── dep-check.go
├── sonarqube.go
├── trivy.go
└── templates/
    ├── report.html          # Single unified HTML template (embedded)
    └── themes/
        └── paper.css        # Default embedded theme
themes/
└── dracula.css              # Dracula dark theme — #282a36 bg, purple/cyan/pink palette, JetBrains Mono
tests/
├── input.*.json             # Sample input files for each source
├── test.sh                  # Runs all sub-scripts then serves outputs on :8182
├── test.default.sh          # Generates output.*.default.html with built-in paper theme
└── test.dracula.sh          # Generates output.*.dracula.html with themes/dracula.css
```
