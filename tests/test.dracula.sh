#!/usr/bin/env bash

set -euo pipefail

REPORTER="${REPORTER:-./devops-reporter}"
CSS="themes/dracula.css"
OUT_DIR="${OUT_DIR:-tests}"

run() {
  local source="$1"
  local input="$2"
  local output="${OUT_DIR}/output.${source}.dracula.html"
  echo "  $source -> $output"
  "$REPORTER" --source "$source" --css "$CSS" --output "$output" < "$input"
}

echo "Building..."
go build -o devops-reporter ./cmd/

echo "Generating reports (css: $CSS)..."
run argocd            tests/input.argocd.json
run kubeconform       tests/input.kubeconform.json
run tenable-was       tests/input.tenable-was.json
run cyclonedx         tests/input.cdx.json
run dependency-check  tests/input.depcheck.json
run sonarqube         tests/input.sonarqube.json
run trivy             tests/input.trivy.json

echo "Done."
