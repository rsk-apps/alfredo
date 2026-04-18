#!/usr/bin/env bash
set -euo pipefail

BRUNO_DIR="bruno"
INTEGRATION_DIR="tests/integration"
SKIP_PATH="/api/v1/health"
MISSING=()

while IFS= read -r bru_file; do
  # Extract HTTP method from Bruno file (e.g. "post" from "post {")
  method=$(grep -E '^\s*(get|post|put|patch|delete|options|head)\s*\{' "$bru_file" | head -1 | awk '{print toupper($1)}' | tr -d '{')

  # Extract URL path from Bruno file (e.g. "/api/v1/pets" from "url: {{baseUrl}}/api/v1/pets")
  path=$(grep -E '^\s*url:\s*' "$bru_file" | head -1 | sed -E 's|.*\{\{baseUrl\}\}([^}]*)\}.*|\1|' | grep -oE '^/[a-z0-9/:_-]*' || true)

  [[ -z "$method" || -z "$path" ]] && continue
  [[ "$path" == "$SKIP_PATH" ]] && continue

  # Search integration tests for a reference to this path
  if ! grep -r --include="*.go" -q "$path" "$INTEGRATION_DIR"; then
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
