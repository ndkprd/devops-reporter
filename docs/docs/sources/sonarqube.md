---
sidebar_position: 6
---

# SonarQube

The `sonarqube` source reads [SonarQube](https://www.sonarsource.com/products/sonarqube/) issues API JSON exports and generates a static code analysis report grouped by severity.

## Input

The input must be a SonarQube issues API JSON response. Export it using the REST API:

```bash
# Single page (up to 100 issues)
curl -u "$SONAR_TOKEN:" \
  "$SONAR_URL/api/issues/search?projectKeys=my-project&statuses=OPEN,CONFIRMED,REOPENED" \
  > sonarqube-issues.json

# All issues (paginated, requires jq)
curl -u "$SONAR_TOKEN:" \
  "$SONAR_URL/api/issues/search?projectKeys=my-project&ps=500&p=1" \
  > sonarqube-issues.json
```

## Usage

```bash
cat sonarqube-issues.json | devops-reporter -source sonarqube
```

```bash
cat sonarqube-issues.json | devops-reporter -source sonarqube \
  -o sonarqube-report.html \
  -title "Code Analysis — my-app (main)"
```

## Report Sections

| Section | Description |
|---|---|
| Header | Report title and project key |
| Scan metadata | Total issues, counts by severity and type |
| Status banner | Pass/fail based on presence of any issues |
| Summary grid | Total, Blocker, Critical, Major, Minor counts |
| Issues by severity | Per-severity groups with a table of Rule, Type, Status, File, Line, Message |
| Footer | Total issue count broken down by type |

## HasIssues Flag

`HasIssues` is `true` when the report contains at least one issue. This drives the status banner and the header accent color.

## Severity Levels

| Severity | Description |
|---|---|
| Blocker | Must be fixed immediately — blocks deployment |
| Critical | High impact code flaw |
| Major | Significant quality issue |
| Minor | Minor quality issue |
| Info | Informational — no immediate impact |

## Issue Status Values

| Status | Description |
|---|---|
| `OPEN` | Newly detected issue |
| `CONFIRMED` | Issue acknowledged as valid |
| `REOPENED` | Issue re-detected after being resolved |

## In GitLab CI/CD

```yaml
sonarqube-report:
  stage: test
  image: curlimages/curl:latest
  variables:
    DEVOPS_REPORTER_VERSION: v0.2.0
  before_script:
    - |
      curl -sSL -o /usr/local/bin/devops-reporter \
        https://github.com/ndkprd/devops-reporter/releases/download/${DEVOPS_REPORTER_VERSION}/devops-reporter_linux_amd64
      chmod +x /usr/local/bin/devops-reporter
  script:
    - |
      curl -u "${SONAR_TOKEN}:" \
        "${SONAR_URL}/api/issues/search?projectKeys=${CI_PROJECT_NAME}&ps=500&statuses=OPEN,CONFIRMED,REOPENED" \
        > sonarqube-issues.json
    - |
      cat sonarqube-issues.json | devops-reporter -source sonarqube \
        -o sonarqube-report.html \
        -title "Code Analysis — ${CI_PROJECT_NAME} (${CI_COMMIT_REF_NAME})"
  artifacts:
    when: always
    paths:
      - sonarqube-issues.json
      - sonarqube-report.html
    expire_in: 1 week
```
