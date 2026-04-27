# devops-reporter

One CLI. Seven tools. One consistent report.

`devops-reporter` reads JSON output from popular DevSecOps tools and generates **self-contained static HTML reports** — every report looks the same, carries the same structure, and can be themed with a single flag. No per-tool templates to maintain, no custom parsers to write.

## Disclaimer

This project is independently developed and is not affiliated with, endorsed by, or officially connected to any of the tools it supports. All referenced tools are the work of their respective contributors.

---

## Supported Sources

| Source | Tool |
|---|---|
| `argocd` | [ArgoCD](https://argoproj.github.io/cd/) — Application deployment state |
| `cyclonedx` | [CycloneDX](https://cyclonedx.org/) — Software Bill of Materials |
| `dependency-check` | [OWASP Dependency-Check](https://owasp.org/www-project-dependency-check/) — Dependency vulnerabilities |
| `kubeconform` | [Kubeconform](https://github.com/yannh/kubeconform) — Kubernetes manifest validation |
| `sonarqube` | [SonarQube](https://www.sonarsource.com/products/sonarqube/) — Static code analysis |
| `tenable-was` | [Tenable WAS](https://www.tenable.com/products/web-app-scanning) — Web application security scan |
| `trivy` | [Trivy](https://trivy.dev/) — Container image vulnerabilities and SBOM |

---

## Why unified reporting matters

DevSecOps pipelines typically span half a dozen tools — each with its own JSON schema, its own terminology, and no HTML output at all. Getting a readable, shareable report out of each one means writing and maintaining seven separate scripts or templates.

`devops-reporter` solves this in one step:

- **Consistent structure** — every report has the same header, status banner, summary grid, and detail sections regardless of source. Stakeholders always know where to look.
- **Self-contained HTML** — CSS is inlined into the output file, so artifacts can be shared, emailed, or archived without breaking styles.
- **One flag to rebrand** — `--css my-theme.css` swaps the entire visual identity. No code changes, no template edits. Works for all seven sources at once.
- **CI-native** — pipe directly from any tool: `trivy image -f json my-image | devops-reporter --source trivy`.

---

## Installation

### Build from source

```bash
git clone https://github.com/ndkprd/devops-reporter.git
cd devops-reporter
go build -o devops-reporter ./cmd/
```

### Docker

```bash
docker build -t devops-reporter .
```

---

## Flags

| Flag | Default | Description |
|---|---|---|
| `--source` / `-s` | | **(required)** Report source — see supported sources above |
| `--output` / `-o` | `report.html` | Output HTML file path |
| `--title` / `-t` | *(source default)* | Title shown in the report header |
| `--org` | | Organization name shown in the header and footer; omit to leave blank |
| `--summary-only` | `false` | Render only the header, meta strip, status banner, and summary grid — skip detail sections |
| `--css` | *(built-in paper theme)* | Path or URL to a CSS file that replaces the built-in theme |
| `--template` / `-T` | *(built-in)* | Path to a custom HTML template (overrides `--css` when set) |
| `--version` / `-v` | | Print version and exit |

**Styling precedence:** `--template` (full override) > `--css` (CSS-only swap) > built-in paper theme

---

## Usage

All sources read from stdin. Pipe tool output directly or redirect from a file.

### ArgoCD

```bash
argocd app get my-app -o json | devops-reporter --source argocd

argocd app get my-app -o json | devops-reporter --source argocd \
  --output deploy-report.html \
  --title "Deploy Report — Service A" \
  --org "My Organisation"
```

### Kubeconform

```bash
kubeconform -output json ./manifests/ | devops-reporter --source kubeconform

kubeconform -output json ./manifests/ | devops-reporter --source kubeconform \
  --output validation-report.html
```

### Tenable WAS

```bash
cat scan-report.json | devops-reporter --source tenable-was

cat scan-report.json | devops-reporter --source tenable-was \
  --output was-report.html \
  --title "WAS Scan — Service A"
```

### CycloneDX SBOM

```bash
cat sbom.json | devops-reporter --source cyclonedx

cat sbom.json | devops-reporter --source cyclonedx \
  --output sbom-report.html \
  --title "SBOM — my-app v1.0.0"
```

### OWASP Dependency-Check

```bash
cat dependency-check-report.json | devops-reporter --source dependency-check

cat dependency-check-report.json | devops-reporter --source dependency-check \
  --output dep-report.html \
  --title "Dependency Scan — my-app"
```

### SonarQube

```bash
cat sonarqube-issues.json | devops-reporter --source sonarqube

cat sonarqube-issues.json | devops-reporter --source sonarqube \
  --output sonarqube-report.html \
  --title "Code Analysis — my-app (main)"
```

### Trivy

```bash
trivy image -f json my-image:tag | devops-reporter --source trivy

# Summary only — useful as a pipeline gate artifact
trivy image -f json my-image:tag | devops-reporter --source trivy \
  --summary-only \
  --output trivy-summary.html
```

---

## Themes and branding

All report styles are driven by a single CSS file. Swap it with `--css` and every source gets the new look instantly — no template edits, no per-source configuration.

### Built-in: Paper (default)

A clean print-inspired theme — Georgia serif body, Courier mono labels, ink-on-paper palette. Embedded in the binary; no external files needed.

```bash
cat input.json | devops-reporter --source trivy
```

### Included: Dracula dark theme

A full dark-mode theme based on the [Dracula colour specification](https://draculatheme.com/contribute) — JetBrains Mono throughout, `#282a36` card backgrounds, and the iconic pink → purple → cyan gradient. Ideal for developer tooling and terminal-adjacent contexts.

```bash
cat input.json | devops-reporter --source trivy --css themes/dracula.css
```

### Custom CSS

Any CSS file — local or remote — can be used as a theme. The file is inlined into the output HTML, so the report remains fully self-contained.

```bash
# Local file
cat input.json | devops-reporter --source trivy --css themes/my-theme.css

# Remote URL (fetched at report generation time, then inlined)
cat input.json | devops-reporter --source trivy --css https://example.com/my-theme.css
```

See `themes/dracula.css` for a complete example of what a theme must cover. The canonical CSS class vocabulary is documented in `AGENTS.md`.

### Organization name

Stamp your organization's name into the report header and footer with `--org`:

```bash
cat input.json | devops-reporter --source trivy \
  --css themes/dracula.css \
  --org "My Organisation"
```

---

## GitLab CI/CD

```yaml
generate-deploy-report:
  stage: deploy
  image: quay.io/argoproj/argocd:latest
  variables:
    DEVOPS_REPORTER_VERSION: v0.2.0
  before_script:
    - |
      curl -sSL -o /usr/local/bin/devops-reporter \
        https://github.com/ndkprd/devops-reporter/releases/download/${DEVOPS_REPORTER_VERSION}/devops-reporter_linux_amd64 \
        && chmod +x /usr/local/bin/devops-reporter
  script:
    - argocd app get ${ARGOCD_APP_NAME} -o json | devops-reporter \
        --source argocd \
        --output report.html \
        --title "Deploy Report — ${CI_PROJECT_NAME} (${CI_ENVIRONMENT_NAME})" \
        --org "${CI_SERVER_HOST}"
  artifacts:
    when: always
    paths:
      - report.html
    expire_in: 1 week
```

---

## License

[MIT](./LICENSE)
