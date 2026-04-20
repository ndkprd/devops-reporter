---
sidebar_position: 3
---

# Tenable WAS

The `tenable-was` source reads [Tenable Web Application Scanning](https://www.tenable.com/products/tenable-was) JSON export and generates a web application security scan report.

## Input

The input must be a Tenable WAS JSON export. You can export it from the Tenable platform or retrieve it via the Tenable WAS API:

```bash
# Via Tenable WAS API
curl -s -X GET "https://cloud.tenable.com/was/v2/scans/<scan-id>/report" \
  -H "X-ApiKeys: accessKey=<access-key>;secretKey=<secret-key>" \
  -o scan-report.json
```

## Usage

```bash
cat scan-report.json | devops-reporter -source tenable-was
```

```bash
cat scan-report.json | devops-reporter -source tenable-was \
  -o was-report.html \
  -title "WAS Scan — my-app (production)"
```

## Report Sections

| Section | Description |
|---|---|
| Header | Report title and scan target URL |
| Scan metadata | Scan name, status, template, start time, finish time, duration |
| Status banner | Pass/fail based on presence of critical/high/medium findings |
| Summary grid | Finding counts by severity: Critical, High, Medium, Low, Info |
| Findings by severity | All findings grouped by severity, each with description, solution, CVE/CWE/OWASP references, plugin output, and proof |

## Severity Levels

| Severity | Meaning |
|---|---|
| Critical | Immediate action required — actively exploitable vulnerability |
| High | Significant risk — should be remediated promptly |
| Medium | Moderate risk — remediation recommended |
| Low | Minor risk — remediate as time allows |
| Info | Informational finding — no direct vulnerability |

`HasIssues` is `true` when there is at least one Critical, High, or Medium finding.

## In GitLab CI/CD

```yaml
was-report:
  stage: security
  image: alpine:latest
  variables:
    DEVOPS_REPORTER_VERSION: v0.2.0
    TENABLE_SCAN_ID: "<your-scan-id>"
  before_script:
    - apk add --no-cache curl
    - |
      curl -sSL -o /usr/local/bin/devops-reporter \
        https://github.com/ndkprd/devops-reporter/releases/download/${DEVOPS_REPORTER_VERSION}/devops-reporter_linux_amd64
      chmod +x /usr/local/bin/devops-reporter
  script:
    - |
      curl -s -X GET "https://cloud.tenable.com/was/v2/scans/${TENABLE_SCAN_ID}/report" \
        -H "X-ApiKeys: accessKey=${TENABLE_ACCESS_KEY};secretKey=${TENABLE_SECRET_KEY}" \
        | devops-reporter -source tenable-was \
            -o was-report.html \
            -title "WAS Scan — ${CI_PROJECT_NAME}"
  artifacts:
    when: always
    paths:
      - was-report.html
    expire_in: 1 week
```
