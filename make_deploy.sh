#!/usr/bin/env bash
#
# SECURITY: This script handles sensitive data. Do not echo secrets.
#
# make_deploy.sh — Build AuraGo deployment artifacts for Linux, macOS, Windows.
#
# Output in bin/ (Linux release binaries, committed to git):
#   aurago_linux            config-merger_linux
#   aurago_linux_arm64      config-merger_linux_arm64
#
# Output in deploy/ (cross-platform artifacts):
#   aurago_darwin_amd64         aurago_windows_amd64.exe
#   aurago_darwin_arm64         aurago_windows_arm64.exe
#   aurago-remote_linux_amd64   aurago-remote_darwin_amd64  aurago-remote_windows_amd64.exe
#   aurago-remote_linux_arm64   aurago-remote_darwin_arm64  aurago-remote_windows_arm64.exe
#   resources.dat               (shared across all platforms)
#   install.sh                  (one-liner bootstrap script)
#   update.sh                   (updater script)
#   SHA256SUMS                  (release checksum manifest)
#
set -euo pipefail
cd "$(dirname "$0")"

DEPLOY_DIR="./deploy"
RESOURCES="resources.dat"
PUBLISH_RELEASE=true

while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-publish)
      PUBLISH_RELEASE=false
      ;;
    *)
      echo "Usage: $0 [--no-publish]" >&2
      exit 2
      ;;
  esac
  shift
done

if [ "${AURAGO_TARGETS+x}" = "x" ]; then
  read -ra TARGETS <<< "$AURAGO_TARGETS"
else
  read -ra TARGETS <<< "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"
fi
if [ "${AURAGO_REMOTE_TARGETS+x}" = "x" ]; then
  read -ra REMOTE_TARGETS <<< "$AURAGO_REMOTE_TARGETS"
else
  read -ra REMOTE_TARGETS <<< "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"
fi
GO_DIST_TARGETS="$(go tool dist list)"

normalize_targets() {
  local target_set="$1"
  shift
  local target seen=" "
  NORMALIZED_TARGETS=()
  if [ "$#" -eq 0 ]; then
    echo "$target_set must contain at least one os/arch target." >&2
    exit 2
  fi
  for target in "$@"; do
    if [[ ! "$target" =~ ^[^/[:space:]]+/[^/[:space:]]+$ ]]; then
      echo "Invalid $target_set target $target; expected os/arch." >&2
      exit 2
    fi
    if [[ "$seen" == *" $target "* ]]; then
      continue
    fi
    if ! printf '%s\n' "$GO_DIST_TARGETS" | grep -Fqx -- "$target"; then
      echo "Unsupported $target_set target: $target." >&2
      exit 2
    fi
    seen+="$target "
    NORMALIZED_TARGETS+=("$target")
  done
  if [ "${#NORMALIZED_TARGETS[@]}" -eq 0 ]; then
    echo "$target_set must contain at least one os/arch target." >&2
    exit 2
  fi
}

normalize_targets "AURAGO_TARGETS" "${TARGETS[@]}"
TARGETS=("${NORMALIZED_TARGETS[@]}")
normalize_targets "AURAGO_REMOTE_TARGETS" "${REMOTE_TARGETS[@]}"
REMOTE_TARGETS=("${NORMALIZED_TARGETS[@]}")

derive_release_artifact_lists() {
  RELEASE_DEPLOY_ASSETS=("$RESOURCES" install.sh update.sh)
  RELEASE_BIN_ASSETS=()

  for target in "${TARGETS[@]}"; do
    OS="${target%/*}"
    ARCH="${target#*/}"
    EXT=""
    if [ "$OS" = "windows" ]; then EXT=".exe"; fi
    if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
      RELEASE_BIN_ASSETS+=(aurago_linux config-merger_linux)
    elif [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
      RELEASE_BIN_ASSETS+=(aurago_linux_arm64 config-merger_linux_arm64)
    else
      RELEASE_DEPLOY_ASSETS+=("aurago_${OS}_${ARCH}${EXT}")
    fi
  done

  for target in "${REMOTE_TARGETS[@]}"; do
    OS="${target%/*}"
    ARCH="${target#*/}"
    EXT=""
    if [ "$OS" = "windows" ]; then EXT=".exe"; fi
    RELEASE_DEPLOY_ASSETS+=("aurago-remote_${OS}_${ARCH}${EXT}")
    if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
      RELEASE_BIN_ASSETS+=(aurago-remote_linux)
    elif [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
      RELEASE_BIN_ASSETS+=(aurago-remote_linux_arm64)
    fi
  done
}

derive_cleanup_artifact_lists() {
  CLEANUP_DEPLOY_ASSETS=("$RESOURCES" install.sh update.sh SHA256SUMS SHA256SUMS.sig SHA256SUMS.pem)
  CLEANUP_BIN_ASSETS=()

  while IFS= read -r target; do
    OS="${target%/*}"
    ARCH="${target#*/}"
    EXT=""
    if [ "$OS" = "windows" ]; then EXT=".exe"; fi
    if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
      CLEANUP_BIN_ASSETS+=(aurago_linux config-merger_linux aurago-remote_linux)
    elif [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
      CLEANUP_BIN_ASSETS+=(aurago_linux_arm64 config-merger_linux_arm64 aurago-remote_linux_arm64)
    else
      CLEANUP_DEPLOY_ASSETS+=("aurago_${OS}_${ARCH}${EXT}")
    fi
    CLEANUP_DEPLOY_ASSETS+=("aurago-remote_${OS}_${ARCH}${EXT}")
  done <<< "$GO_DIST_TARGETS"
}

derive_release_artifact_lists
derive_cleanup_artifact_lists

echo "━━━ AuraGo Deployment Builder ━━━"
echo ""

if command -v node >/dev/null 2>&1; then
  echo "[0/5] Building UI assets ..."
  if [ ! -d node_modules ] && command -v npm >/dev/null 2>&1; then
    npm ci --no-audit --no-fund
  fi
  node scripts/build-codemirror.js
  if command -v npm >/dev/null 2>&1; then
    npm run build:ui
  else
    node scripts/build-ui-bundles.js
  fi
else
  echo "[0/5] Node.js not found; using committed UI assets."
fi

# ── Clean generated release artifacts while preserving versioned deploy sources ──
mkdir -p "$DEPLOY_DIR"
for asset in "${CLEANUP_DEPLOY_ASSETS[@]}"; do
  rm -f "$DEPLOY_DIR/$asset"
done
for asset in "${CLEANUP_BIN_ASSETS[@]}"; do
  rm -f "bin/$asset"
done

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
mkdir -p "$TMPDIR_RES/data/vectordb" "$TMPDIR_RES/data/embeddings"
mkdir -p "$TMPDIR_RES/log"

# Bundle sample assets for first-start seeding
mkdir -p "$TMPDIR_RES/assets/media_samples"
cp -r assets/media_samples/. "$TMPDIR_RES/assets/media_samples/"
mkdir -p "$TMPDIR_RES/assets/mission_samples"
cp -r assets/mission_samples/. "$TMPDIR_RES/assets/mission_samples/"
mkdir -p "$TMPDIR_RES/assets/cheatsheet_samples"
cp -r assets/cheatsheet_samples/. "$TMPDIR_RES/assets/cheatsheet_samples/"
mkdir -p "$TMPDIR_RES/assets/skill_samples"
cp -r assets/skill_samples/. "$TMPDIR_RES/assets/skill_samples/"

# Pack
tar -czf "$DEPLOY_DIR/$RESOURCES" -C "$TMPDIR_RES" .
echo "    → resources.dat ($(du -h "$DEPLOY_DIR/$RESOURCES" | cut -f1))"

# ── Step 2: Cross-compile binaries ───────────────────────────────────────
echo "[2/5] Compiling AuraGo binaries ..."

for target in "${TARGETS[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"
  
  if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
    # Standard Linux release: put binaries in bin/ for GitHub updates
    mkdir -p bin
    
    OUT_AURAGO="bin/aurago_linux"
    echo "    → $OUT_AURAGO"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT_AURAGO" ./cmd/aurago/

    OUT_MERGER="bin/config-merger_linux"
    echo "    → $OUT_MERGER"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT_MERGER" ./cmd/config-merger/
  elif [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
    # Linux arm64: keep binaries in bin/ for consistency with make_release.bat
    mkdir -p bin

    echo "    → bin/aurago_linux_arm64"
    CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "bin/aurago_linux_arm64" ./cmd/aurago/

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

for target in "${REMOTE_TARGETS[@]}"; do
  OS="${target%/*}"
  ARCH="${target#*/}"
  EXT=""
  if [ "$OS" = "windows" ]; then EXT=".exe"; fi

  OUT="$DEPLOY_DIR/aurago-remote_${OS}_${ARCH}${EXT}"
  echo "    → $OUT"
  CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/remote/
  # Linux remotes also live in bin/ for update.sh / install.sh compatibility.
  if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then
    mkdir -p bin
    cp "$OUT" "bin/aurago-remote_linux"
  elif [ "$OS" = "linux" ] && [ "$ARCH" = "arm64" ]; then
    mkdir -p bin
    cp "$OUT" "bin/aurago-remote_linux_arm64"
  fi
done

# ── Step 4: Copy release scripts + checksums ─────────────────────────────
echo "[4/5] Copying release scripts and generating checksums ..."
cp install.sh "$DEPLOY_DIR/install.sh"
cp update.sh "$DEPLOY_DIR/update.sh" 2>/dev/null || true

write_checksum() {
  local path="$1"
  local asset="$2"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk -v asset="$asset" '{print $1 "  " asset}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk -v asset="$asset" '{print $1 "  " asset}'
  else
    echo "No SHA256 tool found (sha256sum/shasum)." >&2
    exit 1
  fi
}

: > "$DEPLOY_DIR/SHA256SUMS"
checksum_seen=" "
for asset in "${RELEASE_DEPLOY_ASSETS[@]}"; do
  if [ -f "$DEPLOY_DIR/$asset" ]; then
    write_checksum "$DEPLOY_DIR/$asset" "$asset" >> "$DEPLOY_DIR/SHA256SUMS"
    checksum_seen+="$asset "
  fi
done
for asset in "${RELEASE_BIN_ASSETS[@]}"; do
  if [ -f "bin/$asset" ]; then
    if [[ "$checksum_seen" == *" $asset "* ]]; then
      continue
    fi
    write_checksum "bin/$asset" "$asset" >> "$DEPLOY_DIR/SHA256SUMS"
    checksum_seen+="$asset "
  fi
done

if [ "${GITHUB_ACTIONS:-}" = "true" ] && command -v cosign >/dev/null 2>&1; then
  echo "    → signing SHA256SUMS with cosign keyless"
  (
    cd "$DEPLOY_DIR"
    cosign sign-blob --yes \
      --output-signature SHA256SUMS.sig \
      --output-certificate SHA256SUMS.pem \
      SHA256SUMS
  )
else
  echo "    → cosign signing skipped (requires GitHub Actions OIDC and cosign)"
fi

echo "━━━ Done! Artifacts in $DEPLOY_DIR/ ━━━"
ls -lh "$DEPLOY_DIR/"

# ── Step 5: Auto Commit & Push ───────────────────────────────────────────
echo ""
echo "[5/5] Committing and pushing to GitHub ..."
if [ "$PUBLISH_RELEASE" = true ]; then
  git add .
  if git diff-index --quiet HEAD; then
      echo "    No changes to commit."
  else
      git commit -m "build: auto-deploy artifacts and code updates [skip actions]" >/dev/null
      git push origin main
      echo "    Code pushed to GitHub successfully."
  fi
else
  echo "    Skipping publish (--no-publish)."
fi
