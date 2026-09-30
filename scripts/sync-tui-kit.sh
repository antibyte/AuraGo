#!/usr/bin/env bash
# Copies the TUI kit block from scripts/aurago-tui.sh into install.sh and
# update.sh (both must stay single-file). Run after editing the kit:
#
#     bash scripts/sync-tui-kit.sh
#
# internal/audit fails when the embedded copies differ from the source block.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

SRC="scripts/aurago-tui.sh"
TARGETS=(install.sh update.sh)

block_file="$(mktemp)"
trap 'rm -f "$block_file"' EXIT

awk '/^# >>> AURAGO-TUI-KIT/ {p=1} p {print} /^# <<< AURAGO-TUI-KIT/ {exit}' "$SRC" > "$block_file"
if [ "$(grep -c '^# >>> AURAGO-TUI-KIT' "$block_file")" -ne 1 ] || [ "$(grep -c '^# <<< AURAGO-TUI-KIT' "$block_file")" -ne 1 ]; then
    echo "sync-tui-kit: $SRC has no complete KIT marker pair" >&2
    exit 1
fi

for target in "${TARGETS[@]}"; do
    if [ "$(grep -c '^# >>> AURAGO-TUI-KIT' "$target")" -ne 1 ] || [ "$(grep -c '^# <<< AURAGO-TUI-KIT' "$target")" -ne 1 ]; then
        echo "sync-tui-kit: $target needs exactly one KIT marker pair" >&2
        exit 1
    fi
    out="$(mktemp)"
    awk -v blockfile="$block_file" '
        /^# >>> AURAGO-TUI-KIT/ { while ((getline line < blockfile) > 0) print line; close(blockfile); skipping = 1; next }
        skipping && /^# <<< AURAGO-TUI-KIT/ { skipping = 0; next }
        !skipping { print }
    ' "$target" > "$out"
    cat "$out" > "$target"   # keeps the target's mode bits
    rm -f "$out"
    bash -n "$target"
    echo "synced $target"
done
