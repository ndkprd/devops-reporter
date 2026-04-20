---
sidebar_position: 3
---

# Custom Templates

devops-reporter supports custom HTML templates via the `-template` flag. The built-in templates live in `cmd/templates/<source>/template.html` and serve as the reference implementation.

## Usage

```bash
argocd app get my-app -o json | devops-reporter \
  -source argocd \
  -template /path/to/my-template.html
```

## Writing Your Own Template

Templates use Go's [`html/template`](https://pkg.go.dev/html/template) syntax. The data available in the template depends on the source.

### ArgoCD template data

| Field | Type | Description |
|---|---|---|
| `.Title` | string | Report title |
| `.GeneratedAt` | string | UTC timestamp |
| `.AppName` | string | Application name |
| `.AppNamespace` | string | Application namespace |
| `.Project` | string | ArgoCD project |
| `.RepoURL` | string | Source repository URL |
| `.Path` | string | Source path |
| `.TargetRevision` | string | Target revision |
| `.Revision` | string | Actual deployed revision |
| `.DestServer` | string | Destination server |
| `.DestNamespace` | string | Destination namespace |
| `.SyncStatus` | string | Sync status (`Synced`, `OutOfSync`) |
| `.HealthStatus` | string | Health status (`Healthy`, `Degraded`, …) |
| `.OperationPhase` | string | Last operation phase |
| `.OperationMsg` | string | Last operation message |
| `.HasIssues` | bool | `true` when not synced or not healthy |
| `.Summary.Total` | int | Total resource count |
| `.Summary.Synced` | int | Synced count |
| `.Summary.OutOfSync` | int | Out-of-sync count |
| `.Summary.Healthy` | int | Healthy count |
| `.Summary.Degraded` | int | Degraded count |
| `.Summary.Missing` | int | Missing count |
| `.Summary.Unknown` | int | Unknown count |
| `.Groups` | []KindGroup | Resources grouped by kind |
| `.SyncResults` | []SyncResultGroup | Sync op results grouped by kind |
| `.ExternalURLs` | []string | External URLs from app summary |
| `.Images` | []string | Container images from app summary |
| `.SourceType` | string | Source type (e.g. `Kustomize`) |

**Template functions (ArgoCD):**

| Function | Description |
|---|---|
| `syncClass .Status` | Returns a CSS class for sync status |
| `healthClass .Status` | Returns a CSS class for health status |
| `opClass .Phase` | Returns a CSS class for operation phase |
| `shortRev .Revision` | Truncates a git SHA to 7 characters |

### Kubeconform template data

| Field | Type | Description |
|---|---|---|
| `.Title` | string | Report title |
| `.GeneratedAt` | string | UTC timestamp |
| `.TotalCount` | int | Total resources inspected |
| `.HasIssues` | bool | `true` when invalid or errors > 0 |
| `.Summary.Valid` | int | Valid count |
| `.Summary.Invalid` | int | Invalid count |
| `.Summary.Errors` | int | Error count |
| `.Summary.Skipped` | int | Skipped count |
| `.Groups` | []KcKindGroup | Resources grouped by kind |

**Template functions (Kubeconform):**

| Function | Description |
|---|---|
| `statusLabel .Status` | Returns a human-readable label for a status string |
| `statusClass .Status` | Returns a CSS class for a status string |

### Tenable WAS template data

| Field | Type | Description |
|---|---|---|
| `.Title` | string | Report title |
| `.GeneratedAt` | string | UTC timestamp |
| `.ScanTarget` | string | Target URL of the scan |
| `.ScanID` | string | Tenable scan UUID |
| `.ScanStatus` | string | Scan status (e.g. `completed`) |
| `.ScanName` | string | Scan configuration name |
| `.Template` | string | Scan template name (e.g. `basic`) |
| `.StartedAt` | string | Scan start time (UTC) |
| `.FinalizedAt` | string | Scan finish time (UTC) |
| `.Duration` | string | Scan duration (e.g. `3m44s`) |
| `.HasIssues` | bool | `true` when critical/high/medium findings > 0 |
| `.Summary.Critical` | int | Critical finding count |
| `.Summary.High` | int | High finding count |
| `.Summary.Medium` | int | Medium finding count |
| `.Summary.Low` | int | Low finding count |
| `.Summary.Info` | int | Info finding count |
| `.Summary.Total` | int | Total finding count |
| `.Groups` | []WasSeverityGroup | Findings grouped by severity (critical → high → medium → low → info) |

Each item in `.Groups` has:
- `.Severity` — severity string (`critical`, `high`, `medium`, `low`, `info`)
- `.Findings` — slice of findings for that severity

Each finding has `.PluginID`, `.Name`, `.Family`, `.Synopsis`, `.Description`, `.Solution`, `.RiskFactor`, `.URI`, `.CVEs`, `.CWE`, `.OWASP`, `.Output`, `.Proof`, `.SeeAlso`.

**Template functions (Tenable WAS):**

| Function | Description |
|---|---|
| `wasRiskClass .Severity` | Returns a CSS class for a severity string (e.g. `risk-critical`) |
| `wasRiskLabel .Severity` | Returns a human-readable label (e.g. `Critical`) |
