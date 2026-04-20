---
slug: /
sidebar_position: 1
---

# Introduction

**devops-reporter** is a CLI tool that reads JSON output from various DevOps tools and generates self-contained, static HTML reports — no external dependencies, no server required.

## Features

- Zero runtime dependencies — single statically linked binary
- Plugin-based architecture: each report source is isolated in its own file
- Built-in templates for each source, with support for custom templates via `--template`
- Fully self-contained HTML output (all CSS inlined)
- Print/PDF-friendly layouts

## Supported Sources

| Source | Input | Description |
|---|---|---|
| `argocd` | `argocd app get -o json` | ArgoCD Application deployment status reports |
| `kubeconform` | `kubeconform -output json` | Kubernetes manifest validation reports |
| `tenable-was` | Tenable WAS JSON export | Web application security scan reports |

## Quick Start

```bash
# ArgoCD deployment report
argocd app get my-app -o json | devops-reporter -source argocd -o report.html

# Kubeconform validation report
kubeconform -output json ./manifests/ | devops-reporter -source kubeconform -o report.html

# Tenable WAS security scan report
cat scan-report.json | devops-reporter -source tenable-was -o was-report.html
```

## Disclaimer

This project is independently developed and is not affiliated with, endorsed by, or officially connected to any of the tools it supports. All referenced tools are the work of their respective contributors.
