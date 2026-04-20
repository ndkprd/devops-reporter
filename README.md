# devops-reporter

A CLI tool that reads JSON output from various DevSecOps tools and generates self-contained static HTML reports.

## Disclaimer

This project is independently developed and is not affiliated with, endorsed by, or officially connected to any of the tools it supports. All referenced tools are the work of their respective contributors.

## Supported Sources

| Source | Description |
|---|---|
| `argocd` | [ArgoCD](https://argoproj.github.io/cd/) Application deployment reports |
| `kubeconform` | [Kubeconform](https://github.com/yannh/kubeconform) manifest validation reports |
| `tenable-was` | [Tenable WAS](https://www.tenable.com/products/web-app-scanning) web application security scan reports |

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

## Flags

| Flag | Default | Description |
|---|---|---|
| `-source` | | **(required)** Report source: `argocd`, `kubeconform`, `tenable-was` |
| `-o` | `report.html` | Output file path for the generated HTML report |
| `-title` | *(source default)* | Title displayed in the report header |
| `-template` | *(built-in)* | Path to a custom HTML template file |
| `-version` | | Print version and exit |

## Usage

### ArgoCD

```bash
argocd app get my-app -o json | devops-reporter -source argocd
```

```bash
argocd app get my-app -o json | devops-reporter -source argocd -o deploy-report.html -title "Deploy Report - Service A"
```

With a custom template:

```bash
argocd app get my-app -o json | devops-reporter -source argocd -template cmd/templates/argocd/asdp.template.html
```

### Kubeconform

```bash
kubeconform -output json ./manifests/ | devops-reporter -source kubeconform
```

```bash
kubeconform -output json ./manifests/ | devops-reporter -source kubeconform -o validation-report.html
```

### Tenable WAS

```bash
cat scan-report.json | devops-reporter -source tenable-was
```

```bash
cat scan-report.json | devops-reporter -source tenable-was -o was-report.html -title "WAS Scan — Service A"
```

### From a file

```bash
cat tests/argocd/input.json | devops-reporter -source argocd -o report.html
cat tests/kubeconform/input.json | devops-reporter -source kubeconform -o report.html
cat tests/tenable-was/tenable-was-sample.json | devops-reporter -source tenable-was -o report.html
```

### In GitLab CI/CD

```yaml
generate-deploy-report:
  stage: deploy
  image: quay.io/argoproj/argocd:latest
  variables:
    DEVSECOPS_REPORTER_VERSION: v0.2.0
  before_script:
    - |
      curl -sSL -o devops-reporter-linux-amd64 \
        https://github.com/ndkprd/devops-reporter/releases/download/${DEVSECOPS_REPORTER_VERSION}/devops-reporter_linux_amd64 && \
        mv devops-reporter-linux-amd64 /usr/local/bin/devops-reporter && \
        chmod +x /usr/local/bin/devops-reporter
  script:
    - argocd app get ${ARGOCD_APP_NAME} -o json | devops-reporter -source argocd -o report.html -title "Deploy Report for ${CI_PROJECT_NAME} (${CI_ENVIRONMENT_NAME})"
  artifacts:
    when: always
    paths:
      - report.html
    expire_in: 1 week
```

## Custom Templates

You can provide your own HTML template via the `-template` flag. The template uses Go's `html/template` syntax and receives a source-specific data structure. See the built-in templates in `cmd/templates/` for reference.

## License

[MIT](./LICENSE)
