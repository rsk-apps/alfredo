#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

get_field() {
  local key="$1"
  local file="$2"
  awk '/^---$/{if(f)exit; f=1; next} f' "$file" | grep -E "^${key}:" | head -1 | sed "s/^${key}:[[:space:]]*//" | tr -d "'\"" || true
}

story_ids=()
for path in "$@"; do
  case "$path" in
    docs/stories/done/STORY-*.md)
      story_ids+=("$(basename "$path" .md | cut -d- -f1-2)")
      ;;
    docs/reviews/execution/EVR-*.md)
      if [[ -f "$REPO_ROOT/$path" ]]; then
        story_ref="$(get_field "story_ref" "$REPO_ROOT/$path")"
        if [[ -n "$story_ref" ]]; then
          story_ids+=("$story_ref")
        fi
      fi
      ;;
  esac
done

if [[ ${#story_ids[@]} -eq 0 ]]; then
  echo "Verifier gate: no done-story or execution-review changes detected."
  exit 0
fi

failures=0
fail() {
  echo "FAIL: $1" >&2
  failures=$((failures + 1))
}

while IFS= read -r story_id; do
  [[ -z "$story_id" ]] && continue
  story_file=$(find "$REPO_ROOT/docs/stories" -name "${story_id}-*.md" -not -path "*/templates/*" -not -path "*/examples/*" | head -1 || true)
  if [[ -z "$story_file" ]]; then
    fail "cannot find story file for ${story_id}"
    continue
  fi

  if [[ "$(get_field "status" "$story_file")" != "done" ]]; then
    echo "Verifier gate: ${story_id} is not done; skipping final verifier checks."
    continue
  fi

  "$REPO_ROOT/scripts/validate-artifacts" "$story_file"

  latest_review=""
  while IFS= read -r evr; do
    if grep -q "story_ref: ${story_id}" "$evr"; then
      latest_review="$evr"
    fi
  done < <(find "$REPO_ROOT/docs/reviews/execution" -name 'EVR-*.md' -print | sort)

  if [[ -z "$latest_review" ]]; then
    fail "no Execution Review found for ${story_id}"
    continue
  fi

  "$REPO_ROOT/scripts/validate-artifacts" "$latest_review"

  owner_role="$(get_field "owner_role" "$latest_review")"
  verdict="$(printf '%s' "$(get_field "verdict" "$latest_review")" | tr '[:upper:]' '[:lower:]')"
  strategy_ref="$(get_field "strategy_ref" "$latest_review")"
  story_strategy_ref="$(get_field "current_strategy_ref" "$story_file")"

  if [[ "$owner_role" != "verifier" ]]; then
    fail "${latest_review#$REPO_ROOT/} owner_role is '${owner_role}' (expected 'verifier')"
  fi
  if [[ "$verdict" != "approved" ]]; then
    fail "${latest_review#$REPO_ROOT/} verdict is '${verdict}' (expected 'approved')"
  fi
  if [[ -n "$story_strategy_ref" && "$strategy_ref" != "$story_strategy_ref" ]]; then
    fail "${latest_review#$REPO_ROOT/} strategy_ref '${strategy_ref}' does not match ${story_file#$REPO_ROOT/} current_strategy_ref '${story_strategy_ref}'"
  fi
done < <(printf '%s\n' "${story_ids[@]}" | sort -u)

if [[ $failures -ne 0 ]]; then
  echo "Verifier gate failed with ${failures} issue(s)." >&2
  exit 1
fi

echo "Verifier gate passed."
