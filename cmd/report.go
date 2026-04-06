package main

import (
	"sort"
	"time"
)

type HealthStatus struct {
	Status string `json:"status"`
}

type Resource struct {
	Group     string        `json:"group"`
	Version   string        `json:"version"`
	Kind      string        `json:"kind"`
	Namespace string        `json:"namespace"`
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Health    *HealthStatus `json:"health"`
}

type SyncResult struct {
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

type Source struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	TargetRevision string `json:"targetRevision"`
}

type OperationState struct {
	Phase      string `json:"phase"`
	Message    string `json:"message"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	SyncResult struct {
		Resources []SyncResult `json:"resources"`
		Revision  string       `json:"revision"`
		Source    Source       `json:"source"`
	} `json:"syncResult"`
}

type ArgoApplication struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Source      Source `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
		Project string `json:"project"`
	} `json:"spec"`
	Status struct {
		Resources []Resource `json:"resources"`
		Sync      struct {
			Status   string `json:"status"`
			Revision string `json:"revision"`
		} `json:"sync"`
		Health         HealthStatus    `json:"health"`
		OperationState *OperationState `json:"operationState"`
		Summary        struct {
			ExternalURLs []string `json:"externalURLs"`
			Images       []string `json:"images"`
		} `json:"summary"`
		SourceType string `json:"sourceType"`
	} `json:"status"`
}

type ResourceSummary struct {
	Synced    int
	OutOfSync int
	Healthy   int
	Degraded  int
	Missing   int
	Unknown   int
	Total     int
}

type KindGroup struct {
	Kind      string
	Resources []Resource
}

type SyncResultGroup struct {
	Kind      string
	Resources []SyncResult
}

type ReportData struct {
	Title          string
	GeneratedAt    string
	AppName        string
	AppNamespace   string
	Project        string
	RepoURL        string
	Path           string
	TargetRevision string
	DestServer     string
	DestNamespace  string
	SyncStatus     string
	HealthStatus   string
	OperationPhase string
	OperationMsg   string
	Revision       string
	Summary        ResourceSummary
	Groups         []KindGroup
	SyncResults    []SyncResultGroup
	ExternalURLs   []string
	Images         []string
	SourceType     string
	HasIssues      bool
}

func BuildReportData(app ArgoApplication, title string) ReportData {
	// Build resource summary
	summary := ResourceSummary{Total: len(app.Status.Resources)}
	for _, r := range app.Status.Resources {
		switch r.Status {
		case "Synced":
			summary.Synced++
		case "OutOfSync":
			summary.OutOfSync++
		}
		if r.Health != nil {
			switch r.Health.Status {
			case "Healthy":
				summary.Healthy++
			case "Degraded":
				summary.Degraded++
			case "Missing":
				summary.Missing++
			default:
				summary.Unknown++
			}
		}
	}

	// Group resources by Kind
	kindMap := make(map[string][]Resource)
	for _, r := range app.Status.Resources {
		kindMap[r.Kind] = append(kindMap[r.Kind], r)
	}
	kinds := make([]string, 0, len(kindMap))
	for k := range kindMap {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	groups := make([]KindGroup, 0, len(kinds))
	for _, kind := range kinds {
		resources := kindMap[kind]
		sort.Slice(resources, func(i, j int) bool {
			return resources[i].Name < resources[j].Name
		})
		groups = append(groups, KindGroup{Kind: kind, Resources: resources})
	}

	// Group sync results by Kind
	var syncResultGroups []SyncResultGroup
	var revision string
	var opPhase, opMsg string

	if app.Status.OperationState != nil {
		op := app.Status.OperationState
		opPhase = op.Phase
		opMsg = op.Message
		revision = op.SyncResult.Revision

		syncKindMap := make(map[string][]SyncResult)
		for _, r := range op.SyncResult.Resources {
			syncKindMap[r.Kind] = append(syncKindMap[r.Kind], r)
		}
		syncKinds := make([]string, 0, len(syncKindMap))
		for k := range syncKindMap {
			syncKinds = append(syncKinds, k)
		}
		sort.Strings(syncKinds)

		for _, kind := range syncKinds {
			results := syncKindMap[kind]
			sort.Slice(results, func(i, j int) bool {
				return results[i].Name < results[j].Name
			})
			syncResultGroups = append(syncResultGroups, SyncResultGroup{Kind: kind, Resources: results})
		}
	}

	if revision == "" {
		revision = app.Status.Sync.Revision
	}

	hasIssues := app.Status.Sync.Status != "Synced" || app.Status.Health.Status != "Healthy"

	return ReportData{
		Title:          title,
		GeneratedAt:    time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		AppName:        app.Metadata.Name,
		AppNamespace:   app.Metadata.Namespace,
		Project:        app.Spec.Project,
		RepoURL:        app.Spec.Source.RepoURL,
		Path:           app.Spec.Source.Path,
		TargetRevision: app.Spec.Source.TargetRevision,
		DestServer:     app.Spec.Destination.Server,
		DestNamespace:  app.Spec.Destination.Namespace,
		SyncStatus:     app.Status.Sync.Status,
		HealthStatus:   app.Status.Health.Status,
		OperationPhase: opPhase,
		OperationMsg:   opMsg,
		Revision:       revision,
		Summary:        summary,
		Groups:         groups,
		SyncResults:    syncResultGroups,
		ExternalURLs:   app.Status.Summary.ExternalURLs,
		Images:         app.Status.Summary.Images,
		SourceType:     app.Status.SourceType,
		HasIssues:      hasIssues,
	}
}

func syncClass(status string) string {
	switch status {
	case "Synced":
		return "sync-synced"
	case "OutOfSync":
		return "sync-outofsync"
	default:
		return "sync-unknown"
	}
}

func healthClass(status string) string {
	switch status {
	case "Healthy":
		return "health-healthy"
	case "Degraded":
		return "health-degraded"
	case "Progressing":
		return "health-progressing"
	case "Suspended":
		return "health-suspended"
	case "Missing":
		return "health-missing"
	default:
		return "health-unknown"
	}
}

func opClass(phase string) string {
	switch phase {
	case "Succeeded":
		return "op-succeeded"
	case "Failed":
		return "op-failed"
	case "Running":
		return "op-running"
	case "Error":
		return "op-error"
	default:
		return "op-unknown"
	}
}

func shortRev(revision string) string {
	if len(revision) > 7 {
		return revision[:7]
	}
	return revision
}
