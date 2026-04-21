#!/usr/bin/env bash
set -euo pipefail

BRUNO_DIR="bruno"
INTEGRATION_DIR="tests/integration"
SKIP_PATH="/api/v1/health"
MISSING=()

while IFS= read -r bru_file; do
  # Extract HTTP method from Bruno file (e.g. "post" from "post {")
  method=$(grep -E '^\s*(get|post|put|patch|delete|options|head)\s*\{' "$bru_file" | head -1 | awk '{print toupper($1)}' | tr -d '{')

  # Extract the Bruno URL path and reduce it to a stable prefix that can be found in
  # integration tests even when path parameters are concatenated dynamically in Go.
  raw_url=$(sed -n -E 's/^[[:space:]]*url:[[:space:]]*//p' "$bru_file" | head -1)
  path="${raw_url#\{\{baseUrl\}\}}"
  path="${path%%\?*}"
  path="${path%%[[:space:]]*}"
  path=$(printf '%s\n' "$path" | awk '{ sub(/\/(:[A-Za-z_][A-Za-z0-9_]*|\{\{[^}]+\}\}).*$/, "/", $0); print }')

  [[ -z "$method" || -z "$path" ]] && continue
  [[ "$path" == "$SKIP_PATH" ]] && continue

  # Search integration tests for a reference to this path
  if ! grep -r --include="*.go" -F -q "$path" "$INTEGRATION_DIR"; then
    MISSING+=("$method $path (from $(basename "$bru_file"))")
  fi
done < <(find "$BRUNO_DIR" -name "*.bru" ! -path "*/environments/*")

if [[ ${#MISSING[@]} -gt 0 ]]; then
  echo "ERROR: The following routes have no integration test coverage:"
  for route in "${MISSING[@]}"; do
    echo "  - $route"
  done
  exit 1
fi

echo "All routes covered by integration tests."
