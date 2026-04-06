# argocd-report

A CLI tool that reads [ArgoCD](https://argoproj.github.io/cd/) Application JSON output and generates a self-contained static HTML deployment report.

## Disclaimer

This project is independently developed and is not affiliated with, endorsed by, or officially connected to the Argo Project or ArgoCD in any way. ArgoCD is the work of its respective contributors.

## Installation

### Build from source

```bash
git clone https://github.com/ndkprd/argocd-report.git
cd argocd-report
go build -o argocd-report ./cmd/
```

### Docker

```bash
docker build -t argocd-report .
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-o` | `report.html` | Output file path for the generated HTML report |
| `-title` | `ArgoCD Application Report` | Title displayed in the report header |
| `-version` | | Print version and exit |

## Usage

The input must be ArgoCD Application JSON, as returned by `argocd app get -o json` or `argocd app wait -o json`.

### Basic

```bash
argocd app get my-app -o json | argocd-report
# writes report.html in the current directory
```

### Custom output path and title

```bash
argocd app get my-app -o json | argocd-report -o reports/deploy.html -title "Deployment Report - Service A (Staging)"
```

### From a file

```bash
cat tests/input.json | argocd-report -o report.html
```

### In GitLab CI/CD

```yaml
generate-deploy-report:
  stage: deploy
  image: docker.io/ndkprd/argocd-report:latest
  script:
    - argocd app get ${ARGOCD_APP_NAME} -o json | argocd-report -o report.html -title "Deploy Report for ${CI_PROJECT_NAME} (${CI_ENVIRONMENT_NAME})"
  artifacts:
    when: always
    paths:
      - report.html
    expire_in: 1 week
```

## License

[MIT](./LICENSE)
