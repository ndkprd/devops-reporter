---
sidebar_position: 4
---

# CycloneDX SBOM

The `cyclonedx` source reads [CycloneDX](https://cyclonedx.org/) JSON Software Bill of Materials (SBOM) files and generates a component inventory report. CycloneDX is an OWASP standard format supported by tools like [cdxgen](https://github.com/CycloneDX/cdxgen), [Syft](https://github.com/anchore/syft), and [Trivy](https://github.com/aquasecurity/trivy).

## Input

The input must be a CycloneDX JSON SBOM (`bomFormat: "CycloneDX"`). You can generate one with any of the following tools:

```bash
# cdxgen (Node.js, Python, Go, Java, and many more)
cdxgen -t npm -o sbom.json .

# Syft
syft . -o cyclonedx-json > sbom.json

# Trivy
trivy fs --format cyclonedx --output sbom.json .
```

## Usage

```bash
cat sbom.json | devops-reporter -source cyclonedx
```

```bash
cat sbom.json | devops-reporter -source cyclonedx \
  -o sbom-report.html \
  -title "SBOM — my-app (v1.0.0)"
```

## Report Sections

| Section | Description |
|---|---|
| Header | Report title and main application name/version |
| BOM metadata | BOM format, spec version, creation time, lifecycle phase, generator tool, serial number |
| Described application | Name, group, version, description, license, and PURL of the main component |
| Status banner | Pass/fail based on whether any components lack license information |
| Summary grid | Total, Libraries, Applications, Unlicensed, Unique Licenses, Ecosystems |
| Components by ecosystem | All dependency components grouped by package ecosystem (npm, pypi, maven, etc.), each with name, version, type, license, PURL, and hash |

## HasIssues Flag

`HasIssues` is `true` when at least one component in the SBOM lacks license information. This drives the status banner and the header accent color.

## In GitLab CI/CD

```yaml
sbom-report:
  stage: build
  image: node:20-alpine
  variables:
    DEVOPS_REPORTER_VERSION: v0.2.0
  before_script:
    - npm install -g @cyclonedx/cdxgen
    - |
      wget -qO /usr/local/bin/devops-reporter \
        https://github.com/ndkprd/devops-reporter/releases/download/${DEVOPS_REPORTER_VERSION}/devops-reporter_linux_amd64
      chmod +x /usr/local/bin/devops-reporter
  script:
    - cdxgen -t npm -o sbom.json .
    - |
      cat sbom.json | devops-reporter -source cyclonedx \
        -o sbom-report.html \
        -title "SBOM — ${CI_PROJECT_NAME} (${CI_COMMIT_SHORT_SHA})"
  artifacts:
    when: always
    paths:
      - sbom.json
      - sbom-report.html
    expire_in: 1 month
```
