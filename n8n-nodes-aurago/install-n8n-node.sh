#!/usr/bin/env bash
# install-n8n-node.sh — Installs n8n-nodes-aurago on a self-hosted n8n server
# Usage: bash install-n8n-node.sh [--n8n-datadir /path/to/.n8n] [--source /path/to/n8n-nodes-aurago]
set -euo pipefail

# ── Defaults ─────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
N8N_DATADIR="${N8N_DATADIR:-$HOME/.n8n}"
SOURCE_DIR="${SOURCE_DIR:-$SCRIPT_DIR}"

# ── Argument parsing ──────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --n8n-datadir) N8N_DATADIR="$2"; shift 2 ;;
    --source)      SOURCE_DIR="$2";  shift 2 ;;
    --help|-h)
      echo "Usage: bash install-n8n-node.sh [--n8n-datadir DIR] [--source DIR]"
      echo "  --n8n-datadir  n8n data directory (default: \$HOME/.n8n)"
      echo "  --source       Path to n8n-nodes-aurago source (default: same dir as this script)"
      exit 0 ;;
    *) echo "Unknown argument: $1"; exit 1 ;;
  esac
done

echo "━━━ n8n-nodes-aurago Installer ━━━"
echo "  Source dir : $SOURCE_DIR"
echo "  n8n data   : $N8N_DATADIR"
echo ""

# ── Prereq checks ─────────────────────────────────────────────────────────────
command -v node >/dev/null 2>&1 || { echo "[ERROR] node not found in PATH"; exit 1; }
command -v npm  >/dev/null 2>&1 || { echo "[ERROR] npm not found in PATH";  exit 1; }

NODE_VER=$(node -e "process.stdout.write(process.version)")
echo "[1/4] Node: $NODE_VER"

# ── Build ─────────────────────────────────────────────────────────────────────
echo "[2/4] Building n8n-nodes-aurago ..."
cd "$SOURCE_DIR"

if [[ ! -f package.json ]]; then
  echo "[ERROR] package.json not found in $SOURCE_DIR"
  exit 1
fi

npm install --prefer-offline 2>/dev/null || npm install
npm run build

# Create tarball
TARBALL=$(npm pack 2>/dev/null | tail -1)
echo "  → packed: $SOURCE_DIR/$TARBALL"

# ── Install into n8n data dir ─────────────────────────────────────────────────
echo "[3/4] Installing into n8n data directory: $N8N_DATADIR"
mkdir -p "$N8N_DATADIR"

if [[ ! -f "$N8N_DATADIR/package.json" ]]; then
  # Bootstrap a minimal package.json in the data dir so npm install works
  echo '{"name":"n8n-custom-nodes","private":true}' > "$N8N_DATADIR/package.json"
fi

cd "$N8N_DATADIR"
npm install "$SOURCE_DIR/$TARBALL"
echo "  → installed"

# ── Restart n8n ───────────────────────────────────────────────────────────────
echo "[4/4] Restarting n8n ..."
if systemctl is-active --quiet n8n 2>/dev/null; then
  systemctl restart n8n
  sleep 3
  if systemctl is-active --quiet n8n; then
    echo "  → n8n restarted successfully"
  else
    echo "  [WARN] n8n may not have started cleanly — check: journalctl -u n8n -n 50"
  fi
elif command -v pm2 >/dev/null 2>&1 && pm2 list | grep -q n8n; then
  pm2 restart n8n
  echo "  → n8n restarted via pm2"
else
  echo "  [INFO] No running n8n service detected."
  echo "         Restart n8n manually or with: systemctl restart n8n"
fi

echo ""
echo "━━━ Done ━━━"
echo "Open n8n → Settings → Community Nodes to confirm the node is loaded."
echo "If the node doesn't appear, set this in your n8n environment config:"
echo "  N8N_CUSTOM_EXTENSIONS=$N8N_DATADIR/node_modules/n8n-nodes-aurago"
