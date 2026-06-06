#!/usr/bin/env bash
set -euo pipefail

port="${1:-}"
if [ -z "$port" ]; then
  echo "usage: preview-port <port>" >&2
  exit 2
fi
case "$port" in
  *[!0-9]*)
    echo "preview-port expects a numeric port" >&2
    exit 2
    ;;
esac

target_file="${COMMANDCODE_PREVIEW_TARGET_FILE:-/tmp/commandcode-preview-target}"
printf 'http://127.0.0.1:%s\n' "$port" > "$target_file"
echo "Preview target set to http://127.0.0.1:$port"
