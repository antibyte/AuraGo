#!/usr/bin/env bash
set -euo pipefail

# Fails if inline style attributes are found in baseline-migrated UI templates.
# Allowlist can be extended via INLINE_STYLE_ALLOWLIST regex if needed.

ALLOWLIST_REGEX="${INLINE_STYLE_ALLOWLIST:-^$}"

TARGET_FILES=(
  ui/index.html
  ui/config.html
  ui/dashboard.html
  ui/setup.html
  ui/missions_v2.html
  ui/invasion_control.html
)

matches=$(rg -n 'style="' "${TARGET_FILES[@]}" || true)

if [[ -z "$matches" ]]; then
  echo "OK: no inline style attributes found in baseline target templates"
  exit 0
fi

if [[ "$ALLOWLIST_REGEX" != "^$" ]]; then
  filtered=$(echo "$matches" | rg -v "$ALLOWLIST_REGEX" || true)
else
  filtered="$matches"
fi

if [[ -z "$filtered" ]]; then
  echo "OK: only allowlisted inline styles found"
  exit 0
fi

echo "ERROR: inline style attributes detected:" >&2
echo "$filtered" >&2
exit 1
