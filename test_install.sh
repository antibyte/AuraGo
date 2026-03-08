#!/usr/bin/env bash
set -euo pipefail
# Quick test: simulate binary-only install from GitHub Releases
rm -rf /home/aurago/aurago
GITHUB_REPO="antibyte/AuraGo"
RELEASE_TAG=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
echo "Release: $RELEASE_TAG"
RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
INSTALL_DIR="/home/aurago/aurago"
mkdir -p "$INSTALL_DIR/bin" "$INSTALL_DIR/data" "$INSTALL_DIR/log"
mkdir -p "$INSTALL_DIR/agent_workspace/workdir" "$INSTALL_DIR/agent_workspace/tools"
cd "$INSTALL_DIR"

echo "--- Downloading resources.dat ---"
curl -fsSL "${RELEASE_BASE}/resources.dat" -o resources.dat
TMPEXT=$(mktemp -d)
tar -xzf resources.dat -C "$TMPEXT"
rm resources.dat
echo "Extracted to $TMPEXT:"
ls "$TMPEXT/"

# Check config.yaml providers
echo "--- config providers in template ---"
grep -A2 '^providers:' "$TMPEXT/config.yaml" || echo "(no providers section)"

echo "--- config llm.provider ---"
grep 'provider:' "$TMPEXT/config.yaml" | head -3

# Only copy config if none exists
if [ ! -f "$INSTALL_DIR/config.yaml" ]; then
    cp "$TMPEXT/config.yaml" "$INSTALL_DIR/config.yaml"
    echo "config.yaml installed (clean template)"
fi
[ -d "$TMPEXT/prompts" ] && cp -a "$TMPEXT/prompts" "$INSTALL_DIR/"
[ -d "$TMPEXT/agent_workspace" ] && cp -a "$TMPEXT/agent_workspace" "$INSTALL_DIR/"
rm -rf "$TMPEXT"

# Download binary
echo "--- Downloading aurago_linux ---"
curl -fsSL "${RELEASE_BASE}/aurago_linux" -o bin/aurago_linux
curl -fsSL "${RELEASE_BASE}/lifeboat_linux" -o bin/lifeboat_linux
curl -fsSL "${RELEASE_BASE}/config-merger_linux" -o bin/config-merger_linux
chmod +x bin/*

# Generate master key
MASTER_KEY=$(openssl rand -hex 32)
echo "AURAGO_MASTER_KEY=$MASTER_KEY" > .env
chmod 600 .env

# Start
echo "--- Starting aurago ---"
source .env
timeout 8s ./bin/aurago_linux --config ./config.yaml 2>&1 | tail -20
echo "--- DONE ---"
