#!/usr/bin/env bash
#
# make_deploy.sh — Build AuraGo deployment artifacts for Linux, macOS, Windows.
#
# Output in bin/ (Linux release binaries, committed to git):
#   aurago_linux            lifeboat_linux            config-merger_linux
#   aurago_linux_arm64      lifeboat_linux_arm64      config-merger_linux_arm64
#   agocli_linux            (AuraGo CLI tool)
#   agocli_linux_arm64
#
# Output in deploy/ (cross-platform artifacts):
#   aurago_darwin_amd64         aurago_windows_amd64.exe
#   aurago_darwin_arm64         aurago_windows_arm64.exe
#   aurago-remote_linux_amd64   aurago-remote_darwin_amd64  aurago-remote_windows_amd64.exe
#   aurago-remote_linux_arm64   aurago-remote_darwin_arm64  aurago-remote_windows_arm64.exe
#   agocli_darwin_amd64         agocli_windows_amd64.exe
#   agocli_darwin_arm64         agocli_windows_arm64.exe
#   resources.dat               (shared across all platforms)
#   install.sh                  (one-liner bootstrap script)
#
set -euo pipefail
cd "$(dirname "$0")"

DEPLOY_DIR="./deploy"
RESOURCES="resources.dat"

echo "━━━ AuraGo Deployment Builder ━━━"
echo ""

# ── Clean ─────────────────────────────────────────────────────────────────
rm -rf "$DEPLOY_DIR"
mkdir -p "$DEPLOY_DIR"

# ── Step 1: Build resources.dat (tar.gz of runtime resources) ─────────────
echo "[1/5] Packing resources.dat ..."

TMPDIR_RES=$(mktemp -d)
trap "rm -rf '$TMPDIR_RES'" EXIT

# Copy resource directories
mkdir -p "$TMPDIR_RES/agent_workspace"
cp -r prompts              "$TMPDIR_RES/"
cp -r agent_workspace/skills   "$TMPDIR_RES/agent_workspace/"
# Remove credential files that must never be deployed
rm -f "$TMPDIR_RES/agent_workspace/skills/client_secret.json"
rm -f "$TMPDIR_RES/agent_workspace/skills/client_secrets.json"
rm -f "$TMPDIR_RES/agent_workspace/skills/token.json"
mkdir -p "$TMPDIR_RES/agent_workspace/tools"
mkdir -p "$TMPDIR_RES/agent_workspace/workdir/attachments"

# Copy template config (strip sensitive values)
sed \
  -e 's/api_key: "sk-[^"]*"/api_key: ""/' \
  -e 's/bot_token: "[^"]*"/bot_token: ""/' \
  -e 's/access_token: "[^"]*"/access_token: ""/' \
  config_template.yaml > "$TMPDIR_RES/config.yaml"

# Create empty data structure
mkdir -p "$TMPDIR_RES/data/vectordb"
mkdir -p "$TMPDIR_RES/log"

# Bundle sample media assets for first-start seeding
mkdir -p "$TMPDIR_RES/assets/media_samples"
cp -r assets/media_samples/. "$TMPDIR_RES/assets/media_samples/"

# Pack
tar -czf "$DEPLOY_DIR/$RESOURCES" -C "$TMPDIR_RES" .
echo "    → resources.dat ($(du -h "$DEPLOY_DIR/$RESOURCES" | cut -f1))"

# ── Step 2: Cross-compile binaries ───────────────────────────────────────
echo "[2/5] Compiling AuraGo binaries ..."

TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

for target in "${TARGETS[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"
  
  if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
    # Standard Linux release: put binaries in bin/ for GitHub updates
    mkdir -p bin
    
    OUT_AURAGO="bin/aurago_linux"
    echo "    → $OUT_AURAGO"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT_AURAGO" ./cmd/aurago/
    
    OUT_LIFEBOAT="bin/lifeboat_linux"
    echo "    → $OUT_LIFEBOAT"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT_LIFEBOAT" ./cmd/lifeboat/

    OUT_MERGER="bin/config-merger_linux"
    echo "    → $OUT_MERGER"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT_MERGER" ./cmd/config-merger/
  elif [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
    # Linux arm64: keep binaries in bin/ for consistency with make_release.bat
    mkdir -p bin

    echo "    → bin/aurago_linux_arm64"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "bin/aurago_linux_arm64" ./cmd/aurago/

    echo "    → bin/lifeboat_linux_arm64"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "bin/lifeboat_linux_arm64" ./cmd/lifeboat/

    echo "    → bin/config-merger_linux_arm64"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "bin/config-merger_linux_arm64" ./cmd/config-merger/
  else
    # Other targets go to deploy/
    EXT=""
    if [ "$OS" = "windows" ]; then EXT=".exe"; fi
    
    OUT="$DEPLOY_DIR/aurago_${OS}_${ARCH}${EXT}"
    echo "    → $OUT"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/aurago/
  fi
done

# ── Step 3: Cross-compile AuraGo Remote binaries ────────────────────────
echo "[3/5] Compiling AuraGo Remote binaries ..."

REMOTE_TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

for target in "${REMOTE_TARGETS[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"
  EXT=""
  if [ "$OS" = "windows" ]; then EXT=".exe"; fi

  OUT="$DEPLOY_DIR/aurago-remote_${OS}_${ARCH}${EXT}"
  echo "    → $OUT"
  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/remote/
  # For Linux/amd64 also keep a copy in bin/ (update.sh / install.sh compatibility)
  if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
    mkdir -p bin
    cp "$OUT" "bin/aurago-remote_linux"
  fi
done

# ── Step 3b: Cross-compile agocli (AuraGo CLI) ─────────────────────────────
echo "[3b/5] Compiling agocli binaries ..."

AGOCLI_TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
)

for target in "${AGOCLI_TARGETS[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"
  EXT=""
  if [ "$OS" = "windows" ]; then EXT=".exe"; fi

  OUT="$DEPLOY_DIR/agocli_${OS}_${ARCH}${EXT}"
  echo "    → $OUT"
  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/agocli/
  # For Linux/amd64 also keep a copy in bin/ AND a plain "agocli" in root
  if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
    mkdir -p bin
    cp "$OUT" "bin/agocli_linux"
    cp "$OUT" "agocli"
  fi
done

# ── Step 4: Copy install script ──────────────────────────────────────────
echo "[4/5] Copying install script ..."
cp deploy/install.sh "$DEPLOY_DIR/install.sh" 2>/dev/null || cp install.sh "$DEPLOY_DIR/" 2>/dev/null || true

echo "━━━ Done! Artifacts in $DEPLOY_DIR/ ━━━"
ls -lh "$DEPLOY_DIR/"

# ── Step 5: Auto Commit & Push ───────────────────────────────────────────
echo ""
echo "[5/5] Committing and pushing to GitHub ..."
git add .
if git diff-index --quiet HEAD; then
    echo "    No changes to commit."
else
    git commit -m "build: auto-deploy artifacts and code updates [skip actions]" >/dev/null
    git push origin main
    echo "    Code pushed to GitHub successfully."
fi
