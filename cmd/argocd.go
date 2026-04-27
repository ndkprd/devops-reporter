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
	RegisterSource("argocd", &ReportSource{
		DefaultTitle: "ArgoCD Application Report",
		Parse: func(data []byte, title string) (ReportData, error) {
			var app ArgoApplication
			if err := json.Unmarshal(data, &app); err != nil {
				return ReportData{}, err
			}
			return BuildArgoReportData(app, title), nil
		},
	})
}

// ── Input types ──────────────────────────────────────────────────

type ArgoHealthStatus struct {
	Status string `json:"status"`
}

type ArgoResource struct {
	Group     string            `json:"group"`
	Version   string            `json:"version"`
	Kind      string            `json:"kind"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	Health    *ArgoHealthStatus `json:"health"`
}

type ArgoSyncResult struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	HookPhase string `json:"hookPhase"`
	SyncPhase string `json:"syncPhase"`
}

type ArgoSource struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	TargetRevision string `json:"targetRevision"`
}

type ArgoOperationState struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	SyncResult struct {
		Resources []ArgoSyncResult `json:"resources"`
		Revision  string           `json:"revision"`
		Source    ArgoSource       `json:"source"`
	} `json:"syncResult"`
}

type ArgoApplication struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Source      ArgoSource `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
		Project string `json:"project"`
	} `json:"spec"`
	Status struct {
		Resources []ArgoResource `json:"resources"`
		Sync      struct {
			Status   string `json:"status"`
			Revision string `json:"revision"`
		} `json:"sync"`
		Health         ArgoHealthStatus    `json:"health"`
		OperationState *ArgoOperationState `json:"operationState"`
		Summary        struct {
			ExternalURLs []string `json:"externalURLs"`
			Images       []string `json:"images"`
		} `json:"summary"`
		SourceType string `json:"sourceType"`
	} `json:"status"`
}

// ── Adapter ──────────────────────────────────────────────────────

func BuildArgoReportData(app ArgoApplication, title string) ReportData {
	total := len(app.Status.Resources)
	var synced, outOfSync, healthy, degraded, missing, unknown int
	for _, r := range app.Status.Resources {
		switch r.Status {
		case "Synced":
			synced++
		case "OutOfSync":
			outOfSync++
		}
		if r.Health != nil {
			switch r.Health.Status {
			case "Healthy":
				healthy++
			case "Degraded":
				degraded++
			case "Missing":
				missing++
			default:
				unknown++
			}
		}
	}

	// Group live resources by kind
	kindMap := make(map[string][]ArgoResource)
	for _, r := range app.Status.Resources {
		kindMap[r.Kind] = append(kindMap[r.Kind], r)
	}
	kinds := make([]string, 0, len(kindMap))
	for k := range kindMap {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	resourceCols := []string{"Name", "Namespace", "Sync", "Health"}
	var resourceGroups []SectionGroup
	for _, kind := range kinds {
		resources := kindMap[kind]
		sort.Slice(resources, func(i, j int) bool { return resources[i].Name < resources[j].Name })
		rows := make([][]template.HTML, 0, len(resources))
		for _, r := range resources {
			syncCls, syncLbl := argoSyncCanonical(r.Status)
			healthCell := template.HTML(`<em style="opacity:.4">—</em>`)
			if r.Health != nil {
				hCls, hLbl := argoHealthCanonical(r.Health.Status)
				healthCell = BadgeHTML("badge "+hCls, hLbl)
			}
			rows = append(rows, []template.HTML{
				template.HTML(fmt.Sprintf(`<strong>%s</strong>`, template.HTMLEscapeString(r.Name))),
				MonoHTML(r.Namespace),
				BadgeHTML("badge "+syncCls, syncLbl),
				healthCell,
			})
		}
		resourceGroups = append(resourceGroups, SectionGroup{
			Name:    kind,
			Count:   pluralise(len(resources), "resource", "resources"),
			Columns: resourceCols,
			Rows:    rows,
		})
	}

	// Sync result groups (from last operation)
	var syncResultGroups []SectionGroup
	var revision, opPhase, opMsg string

	if app.Status.OperationState != nil {
		op := app.Status.OperationState
		opPhase = op.Phase
		opMsg = op.Message
		revision = op.SyncResult.Revision

		syncKindMap := make(map[string][]ArgoSyncResult)
		for _, r := range op.SyncResult.Resources {
			syncKindMap[r.Kind] = append(syncKindMap[r.Kind], r)
		}
		syncKinds := make([]string, 0, len(syncKindMap))
		for k := range syncKindMap {
			syncKinds = append(syncKinds, k)
		}
		sort.Strings(syncKinds)

		syncCols := []string{"Name", "Namespace", "Status", "Message"}
		for _, kind := range syncKinds {
			results := syncKindMap[kind]
			sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
			rows := make([][]template.HTML, 0, len(results))
			for _, r := range results {
				cls, lbl := argoSyncResultCanonical(r.Status)
				rows = append(rows, []template.HTML{
					template.HTML(fmt.Sprintf(`<strong>%s</strong>`, template.HTMLEscapeString(r.Name))),
					MonoHTML(r.Namespace),
					BadgeHTML("badge "+cls, lbl),
					template.HTML(template.HTMLEscapeString(r.Message)),
				})
			}
			syncResultGroups = append(syncResultGroups, SectionGroup{
				Name:    kind,
				Count:   pluralise(len(results), "resource", "resources"),
				Columns: syncCols,
				Rows:    rows,
			})
		}
	}

	if revision == "" {
		revision = app.Status.Sync.Revision
	}

	hasIssues := app.Status.Sync.Status != "Synced" || app.Status.Health.Status != "Healthy"

	// Operation banner panel (shown when an op phase exists)
	var extraPanels []template.HTML
	if opPhase != "" {
		extraPanels = append(extraPanels, renderArgoOpPanel(opPhase, opMsg,
			app.Status.OperationState.StartedAt, app.Status.OperationState.FinishedAt))
	}
	// External URLs / images info panel
	if len(app.Status.Summary.ExternalURLs) > 0 || len(app.Status.Summary.Images) > 0 {
		extraPanels = append(extraPanels, renderArgoSummaryPanel(
			app.Status.Summary.ExternalURLs, app.Status.Summary.Images))
	}

	syncCls, syncLbl := argoSyncCanonical(app.Status.Sync.Status)
	healthCls, healthLbl := argoHealthCanonical(app.Status.Health.Status)

	statusLine := fmt.Sprintf("Sync: %s · Health: %s", syncLbl, healthLbl)

	sections := []Section{
		{Kind: "table", Title: "Resources by Kind", Groups: resourceGroups, Empty: "No resources reported."},
	}
	if len(syncResultGroups) > 0 {
		sections = append(sections, Section{
			Kind:   "table",
			Title:  "Last Sync Results",
			Groups: syncResultGroups,
			Empty:  "No sync result data.",
		})
	}

	_ = syncCls // used in stat card variant below
	_ = healthCls

	return ReportData{
		Title:       title,
		Eyebrow:     "ArgoCD Application",
		Subtitle:    fmt.Sprintf("%s/%s", app.Metadata.Namespace, app.Metadata.Name),
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		Status:      statusFromBool(hasIssues),
		StatusLine:  statusLine,
		Meta: []KV{
			{Label: "Application", Value: app.Metadata.Name},
			{Label: "Namespace", Value: app.Metadata.Namespace},
			{Label: "Project", Value: app.Spec.Project},
			{Label: "Repo URL", Value: app.Spec.Source.RepoURL},
			{Label: "Path", Value: app.Spec.Source.Path},
			{Label: "Target Revision", Value: app.Spec.Source.TargetRevision},
			{Label: "Revision", Value: ShortRev(revision)},
			{Label: "Dest Server", Value: app.Spec.Destination.Server},
			{Label: "Dest Namespace", Value: app.Spec.Destination.Namespace},
			{Label: "Source Type", Value: app.Status.SourceType},
		},
		ExtraPanels: extraPanels,
		Summary: []StatCard{
			{Number: fmt.Sprintf("%d", total), Label: "Resources", Variant: "primary"},
			{Number: fmt.Sprintf("%d", synced), Label: "Synced", Variant: "pass"},
			{Number: fmt.Sprintf("%d", outOfSync), Label: "Out of Sync", Variant: "warn"},
			{Number: fmt.Sprintf("%d", healthy), Label: "Healthy", Variant: "pass"},
			{Number: fmt.Sprintf("%d", degraded), Label: "Degraded", Variant: "fail"},
			{Number: fmt.Sprintf("%d", missing), Label: "Missing", Variant: "warn"},
		},
		Sections: sections,
		Footer: FooterInfo{
			Total: pluralise(total, "resource", "resources"),
			Brand: "devops-reporter · argocd",
		},
	}
}

func renderArgoOpPanel(phase, msg, startedAt, finishedAt string) template.HTML {
	cls, lbl := argoOpCanonical(phase)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<div class="extra-panel">
  <div class="extra-panel-eyebrow">Last Operation</div>
  <div class="extra-panel-name">%s %s</div>
  <div class="extra-panel-meta">`,
		BadgeHTML("badge "+cls, lbl),
		template.HTMLEscapeString(msg),
	))
	if startedAt != "" {
		sb.WriteString(fmt.Sprintf(`<span><strong>Started</strong> %s</span>`, template.HTMLEscapeString(startedAt)))
	}
	if finishedAt != "" {
		sb.WriteString(fmt.Sprintf(`<span><strong>Finished</strong> %s</span>`, template.HTMLEscapeString(finishedAt)))
	}
	sb.WriteString(`</div></div>`)
	return template.HTML(sb.String())
}

func renderArgoSummaryPanel(urls, images []string) template.HTML {
	var sb strings.Builder
	sb.WriteString(`<div class="extra-panel">
  <div class="extra-panel-eyebrow">Application Summary</div>
  <div class="extra-panel-meta">`)
	for _, u := range urls {
		sb.WriteString(fmt.Sprintf(`<span class="full"><strong>URL</strong> <a href="%s" target="_blank" rel="noopener noreferrer">%s</a></span>`,
			template.HTMLEscapeString(u), template.HTMLEscapeString(u)))
	}
	for _, img := range images {
		sb.WriteString(fmt.Sprintf(`<span class="full"><strong>Image</strong> %s</span>`,
			template.HTMLEscapeString(img)))
	}
	sb.WriteString(`</div></div>`)
	return template.HTML(sb.String())
}

// ── Helpers ──────────────────────────────────────────────────────

func argoSyncCanonical(status string) (cssClass, label string) {
	switch status {
	case "Synced":
		return "state-pass", "Synced"
	case "OutOfSync":
		return "state-warn", "OutOfSync"
	default:
		return "state-neutral", status
	}
}

func argoHealthCanonical(status string) (cssClass, label string) {
	switch status {
	case "Healthy":
		return "state-pass", "Healthy"
	case "Degraded":
		return "state-fail", "Degraded"
	case "Progressing":
		return "state-warn", "Progressing"
	case "Suspended":
		return "state-neutral", "Suspended"
	case "Missing":
		return "state-warn", "Missing"
	default:
		return "state-neutral", status
	}
}

func argoSyncResultCanonical(status string) (cssClass, label string) {
	switch status {
	case "Synced":
		return "state-pass", "Synced"
	case "SyncFailed":
		return "state-fail", "Failed"
	case "Pruned":
		return "state-neutral", "Pruned"
	default:
		if status == "" {
			return "state-neutral", "—"
		}
		return "state-neutral", status
	}
}

func argoOpCanonical(phase string) (cssClass, label string) {
	switch phase {
	case "Succeeded":
		return "state-pass", "Succeeded"
	case "Failed":
		return "state-fail", "Failed"
	case "Running":
		return "state-warn", "Running"
	case "Error":
		return "state-fail", "Error"
	default:
		return "state-neutral", phase
	}
}
