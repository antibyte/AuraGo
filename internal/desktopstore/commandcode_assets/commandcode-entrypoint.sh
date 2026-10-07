#!/usr/bin/env bash
set -euo pipefail

# User memory is loaded for every project, including existing workspaces.
# Import the image-owned guide so updates never replace user/project instructions.
mkdir -p "$HOME/.commandcode"
memory_file="$HOME/.commandcode/AGENTS.md"
preview_memory='@/usr/local/share/aurago/commandcode-preview.md'
if ! grep -Fqx -- "$preview_memory" "$memory_file" 2>/dev/null; then
  printf '\n\n%s\n' "$preview_memory" >> "$memory_file"
fi

target_file="${COMMANDCODE_PREVIEW_TARGET_FILE:-/tmp/commandcode-preview-target}"
if [ ! -s "$target_file" ]; then
  printf '%s\n' "${COMMANDCODE_PREVIEW_TARGET:-http://127.0.0.1:5173}" > "$target_file"
fi

node /usr/local/bin/commandcode-preview.js &
preview_pid="$!"
cmd_pid=""

cleanup() {
  if [ -n "$cmd_pid" ]; then
    kill "$cmd_pid" 2>/dev/null || true
  fi
  kill "$preview_pid" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

"$@" &
cmd_pid="$!"
wait "$cmd_pid"
