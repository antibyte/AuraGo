#!/usr/bin/env bash
# Copies the Bluetooth kit block from scripts/aurago-bluetooth.sh into
# install.sh, update.sh and install_service_linux.sh (all must stay
# single-file). Run after editing the kit:
#
#     bash scripts/sync-bluetooth-kit.sh
#
# internal/audit fails when the embedded copies differ from the source block.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

SRC="scripts/aurago-bluetooth.sh"
TARGETS=(install.sh update.sh install_service_linux.sh)

block_file="$(mktemp)"
trap 'rm -f "$block_file"' EXIT

awk '/^# >>> AURAGO-BLUETOOTH-KIT/ {p=1} p {print} /^# <<< AURAGO-BLUETOOTH-KIT/ {exit}' "$SRC" > "$block_file"
if [ "$(grep -c '^# >>> AURAGO-BLUETOOTH-KIT' "$block_file")" -ne 1 ] || [ "$(grep -c '^# <<< AURAGO-BLUETOOTH-KIT' "$block_file")" -ne 1 ]; then
    echo "sync-bluetooth-kit: $SRC has no complete KIT marker pair" >&2
    exit 1
fi

for target in "${TARGETS[@]}"; do
    if [ "$(grep -c '^# >>> AURAGO-BLUETOOTH-KIT' "$target")" -ne 1 ] || [ "$(grep -c '^# <<< AURAGO-BLUETOOTH-KIT' "$target")" -ne 1 ]; then
        echo "sync-bluetooth-kit: $target needs exactly one KIT marker pair" >&2
        exit 1
    fi
    out="$(mktemp)"
    awk -v blockfile="$block_file" '
        /^# >>> AURAGO-BLUETOOTH-KIT/ { while ((getline line < blockfile) > 0) print line; close(blockfile); skipping = 1; next }
        skipping && /^# <<< AURAGO-BLUETOOTH-KIT/ { skipping = 0; next }
        !skipping { print }
    ' "$target" > "$out"
    cat "$out" > "$target"   # keeps the target's mode bits
    rm -f "$out"
    bash -n "$target"
    echo "synced $target"
done
