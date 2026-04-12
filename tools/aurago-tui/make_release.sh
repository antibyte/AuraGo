#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "==========================================="
echo " Building aurago-tui release binaries"
echo "==========================================="
echo ""

if ! command -v cargo &> /dev/null; then
    echo "ERROR: cargo not found."
    echo "Install Rust from https://rustup.rs/"
    exit 1
fi

TARGET="${1:-$(rustc -vV | sed -n 's|host: ||p')}"

echo "Building for $TARGET ..."
cargo build --release --target "$TARGET"

SRC="target/$TARGET/release/aurago-tui"
DEST="../../bin/aurago-tui-${TARGET}"

mkdir -p ../../bin
cp "$SRC" "$DEST"

echo ""
echo "SUCCESS: Binary copied to $DEST"
echo ""
