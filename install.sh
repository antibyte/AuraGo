#!/usr/bin/env bash
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  AuraGo Quick Installer  (Linux x86_64 + arm64)
#
#  Usage:
#    curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
#
#  Two installation modes:
#    A) Source build  — clones repo, requires Go 1.26+, builds from source
#    B) Binary install — downloads pre-built binaries + resources from
#       GitHub Releases. No git clone, no Go required.
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
set -euo pipefail

GITHUB_REPO="antibyte/AuraGo"
REPO="https://github.com/${GITHUB_REPO}.git"
INSTALL_DIR="${AURAGO_DIR:-$HOME/aurago}"
SYSTEMD_SERVICE="aurago"
GO_VERSION="1.26.0"
GO_INSTALL_DIR="/usr/local"

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info() { echo -e "${CYAN}[AuraGo]${NC} $*"; }
ok()   { echo -e "${GREEN}[  OK  ]${NC} $*"; }
warn() { echo -e "${YELLOW}[ WARN ]${NC} $*"; }
die()  { echo -e "${RED}[ERROR ]${NC} $*"; exit 1; }

echo ""
echo -e "${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║         AuraGo Installer             ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
echo ""

# ── Architecture detection ──────────────────────────────────────────────
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)        GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l|armv6l) GOARCH="armv6l" ;;
    *)             die "Unsupported architecture: $ARCH_RAW" ;;
esac
ok "Architecture: $ARCH_RAW → target: $GOARCH"

SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO="sudo"

# ── Package manager detection ────────────────────────────────────────────
_detect_pkg_manager() {
    if   command -v apt-get >/dev/null 2>&1; then echo "apt"
    elif command -v dnf     >/dev/null 2>&1; then echo "dnf"
    elif command -v yum     >/dev/null 2>&1; then echo "yum"
    elif command -v pacman  >/dev/null 2>&1; then echo "pacman"
    elif command -v apk     >/dev/null 2>&1; then echo "apk"
    elif command -v zypper  >/dev/null 2>&1; then echo "zypper"
    else echo "unknown"
    fi
}
PKG_MGR=$(_detect_pkg_manager)

_pkg_install() {
    case "$PKG_MGR" in
        apt)    $SUDO apt-get install -y "$@" ;;
        dnf)    $SUDO dnf install -y "$@" ;;
        yum)    $SUDO yum install -y "$@" ;;
        pacman) $SUDO pacman -Sy --noconfirm "$@" ;;
        apk)    $SUDO apk add --no-cache "$@" ;;
        zypper) $SUDO zypper install -y "$@" ;;
        *)      warn "Cannot auto-install packages (unknown package manager). Please install manually: $*" ;;
    esac
}

# ── Ensure curl or wget ──────────────────────────────────────────────────
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || \
    { info "Installing curl..."; _pkg_install curl; }

_download() {
    local url="$1" dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$dest"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$dest"
    else
        die "Neither curl nor wget available."
    fi
}

# ── Optional system dependencies ─────────────────────────────────────────
info "Checking system dependencies..."

# ffmpeg (needed for Telegram voice conversion)
if ! command -v ffmpeg >/dev/null 2>&1; then
    warn "ffmpeg not found."
    read -r -p "Install ffmpeg? [Y/n]: " FF_REPLY < /dev/tty || true
    if [[ "${FF_REPLY:-y}" =~ ^[Yy]$ ]]; then
        case "$PKG_MGR" in
            apt)    _pkg_install ffmpeg ;;
            dnf)    $SUDO dnf install -y ffmpeg --allowerasing 2>/dev/null || \
                    { $SUDO dnf install -y https://download1.rpmfusion.org/free/fedora/rpmfusion-free-release-$(rpm -E %fedora).noarch.rpm 2>/dev/null; $SUDO dnf install -y ffmpeg; } ;;
            *)      _pkg_install ffmpeg ;;
        esac
        ok "ffmpeg installed."
    else
        warn "Skipping ffmpeg. Telegram voice messages will not work."
    fi
else
    ok "ffmpeg found."
fi

# Python 3 + pip (needed for Python skills)
PYTHON_MISSING=false
if ! command -v python3 >/dev/null 2>&1 || ! python3 -m pip --version >/dev/null 2>&1; then
    warn "Python 3 / pip not found."
    read -r -p "Install Python 3, pip and venv? [Y/n]: " PY_REPLY < /dev/tty || true
    if [[ "${PY_REPLY:-y}" =~ ^[Yy]$ ]]; then
        case "$PKG_MGR" in
            apt)    _pkg_install python3 python3-pip python3-venv ;;
            dnf)    _pkg_install python3 python3-pip ;;
            pacman) _pkg_install python python-pip ;;
            apk)    _pkg_install python3 py3-pip ;;
            *)      _pkg_install python3 python3-pip ;;
        esac
        ok "Python 3 + pip installed."
    else
        warn "Skipping Python. Python-based skills will not work."
        PYTHON_MISSING=true
    fi
else
    ok "Python 3 + pip found."
fi

# ══════════════════════════════════════════════════════════════════════════
#  Decide installation mode: SOURCE BUILD vs BINARY INSTALL
# ══════════════════════════════════════════════════════════════════════════
BUILD_FROM_SOURCE=false

_go_version_ok() {
    local installed
    installed=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
    [ -n "$installed" ] || return 1
    [ "$(printf '%s\n%s' "$GO_VERSION" "$installed" | sort -V | head -n1)" = "$GO_VERSION" ]
}

if _go_version_ok; then
    export PATH="$GO_INSTALL_DIR/go/bin:$PATH"
    ok "Go $(go version | awk '{print $3}') found — will build from source."
    BUILD_FROM_SOURCE=true
else
    info "Go $GO_VERSION+ not found."
    echo ""
    echo "  Choose installation mode:"
    echo "    1) Binary install — download pre-built binaries (no Go needed, fast)"
    echo "    2) Source build   — install Go $GO_VERSION, clone repo, build from source"
    echo ""
    read -r -p "Install mode [1/2, default=1]: " MODE_REPLY < /dev/tty || true
    if [[ "${MODE_REPLY:-1}" == "2" ]]; then
        info "Installing Go $GO_VERSION for $GOARCH..."
        GO_TAR="go${GO_VERSION}.linux-${GOARCH}.tar.gz"
        GO_URL="https://go.dev/dl/${GO_TAR}"
        TMP_GO=$(mktemp -d)
        trap 'rm -rf "$TMP_GO"' EXIT

        _download "$GO_URL" "$TMP_GO/$GO_TAR"
        $SUDO rm -rf "$GO_INSTALL_DIR/go"
        $SUDO tar -C "$GO_INSTALL_DIR" -xzf "$TMP_GO/$GO_TAR"
        rm -rf "$TMP_GO"
        trap - EXIT

        export PATH="$GO_INSTALL_DIR/go/bin:$PATH"
        $SUDO tee /etc/profile.d/go.sh > /dev/null <<'GOPATH'
export PATH="/usr/local/go/bin:$PATH"
GOPATH
        ok "Go $GO_VERSION installed to $GO_INSTALL_DIR/go"
        BUILD_FROM_SOURCE=true
    else
        ok "Binary install selected — no Go required."
    fi
fi

# ══════════════════════════════════════════════════════════════════════════
#  MODE A: Source build — clone repo & compile
# ══════════════════════════════════════════════════════════════════════════
if $BUILD_FROM_SOURCE; then
    command -v git >/dev/null 2>&1 || { info "Installing git..."; _pkg_install git; }

    if [ -d "$INSTALL_DIR/.git" ]; then
        info "Existing installation found at $INSTALL_DIR — updating..."
        git -C "$INSTALL_DIR" pull --ff-only
        ok "Updated to latest."
    else
        info "Cloning into $INSTALL_DIR ..."
        git clone "$REPO" "$INSTALL_DIR"
        ok "Cloned."
    fi

    cd "$INSTALL_DIR"
    mkdir -p bin data agent_workspace/workdir agent_workspace/tools log

    info "Building AuraGo from source (GOOS=linux GOARCH=$GOARCH)..."
    CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w" -o bin/aurago_linux ./cmd/aurago
    ok "bin/aurago_linux built."

    CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w" -o bin/lifeboat_linux ./cmd/lifeboat
    ok "bin/lifeboat_linux built."

    CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w" -o bin/config-merger_linux ./cmd/config-merger
    ok "bin/config-merger_linux built."

# ══════════════════════════════════════════════════════════════════════════
#  MODE B: Binary install — download from GitHub Releases (no clone)
# ══════════════════════════════════════════════════════════════════════════
else
    info "Binary install — downloading from GitHub Releases..."

    # Resolve the latest release tag
    RELEASE_TAG=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
                  | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
    [ -z "$RELEASE_TAG" ] && die "Could not determine latest release tag from GitHub."
    info "Latest release: $RELEASE_TAG"

    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"

    # Create install directory + subdirectories
    mkdir -p "$INSTALL_DIR/bin" "$INSTALL_DIR/data" "$INSTALL_DIR/log"
    mkdir -p "$INSTALL_DIR/agent_workspace/workdir" "$INSTALL_DIR/agent_workspace/tools"
    cd "$INSTALL_DIR"

    # Download resources.dat and extract (contains prompts, skills, config template, UI)
    info "Downloading resources.dat ..."
    _download "${RELEASE_BASE}/resources.dat" "$INSTALL_DIR/resources.dat"
    # Extract to a temp dir so we can selectively merge (never clobber existing config)
    TMPEXT=$(mktemp -d)
    tar -xzf "$INSTALL_DIR/resources.dat" -C "$TMPEXT"
    rm -f "$INSTALL_DIR/resources.dat"

    # Copy prompts, skills, and other resource dirs (always overwrite — they are code)
    [ -d "$TMPEXT/prompts" ]           && cp -a "$TMPEXT/prompts"           "$INSTALL_DIR/"
    [ -d "$TMPEXT/agent_workspace" ]   && cp -a "$TMPEXT/agent_workspace"   "$INSTALL_DIR/"
    [ -d "$TMPEXT/ui" ]               && cp -a "$TMPEXT/ui"               "$INSTALL_DIR/" 2>/dev/null || true

    # Only install config.yaml if none exists (preserve user config on re-install)
    if [ ! -f "$INSTALL_DIR/config.yaml" ]; then
        [ -f "$TMPEXT/config.yaml" ] && cp "$TMPEXT/config.yaml" "$INSTALL_DIR/config.yaml"
        ok "config.yaml installed (clean template — Setup Wizard will run)."
    else
        ok "Existing config.yaml preserved."
    fi
    rm -rf "$TMPEXT"
    ok "Resources extracted."

    # Download binaries
    if [ "$GOARCH" = "arm64" ]; then
        info "Downloading arm64 binaries..."
        _download "${RELEASE_BASE}/aurago_linux_arm64"          "bin/aurago_linux_arm64"
        _download "${RELEASE_BASE}/lifeboat_linux_arm64"        "bin/lifeboat_linux_arm64"        2>/dev/null || warn "lifeboat_linux_arm64 not in release."
        _download "${RELEASE_BASE}/config-merger_linux_arm64"   "bin/config-merger_linux_arm64"   2>/dev/null || warn "config-merger_linux_arm64 not in release."
        cp bin/aurago_linux_arm64           bin/aurago_linux
        cp bin/lifeboat_linux_arm64         bin/lifeboat_linux         2>/dev/null || true
        cp bin/config-merger_linux_arm64    bin/config-merger_linux     2>/dev/null || true
    else
        info "Downloading amd64 binaries..."
        _download "${RELEASE_BASE}/aurago_linux"                "bin/aurago_linux"
        _download "${RELEASE_BASE}/lifeboat_linux"              "bin/lifeboat_linux"              2>/dev/null || warn "lifeboat_linux not in release."
        _download "${RELEASE_BASE}/config-merger_linux"         "bin/config-merger_linux"         2>/dev/null || warn "config-merger_linux not in release."
    fi
    ok "Binaries downloaded."
fi

chmod +x bin/aurago_linux bin/lifeboat_linux bin/config-merger_linux 2>/dev/null || true
ok "Binaries ready."

# ── Master key ────────────────────────────────────────────────────────────
ENV_FILE="$INSTALL_DIR/.env"
if [ -f "$ENV_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$ENV_FILE"; then
    warn ".env already has AURAGO_MASTER_KEY — keeping existing key."
else
    MASTER_KEY=$(openssl rand -hex 32 2>/dev/null || python3 -c "import secrets; print(secrets.token_hex(32))")
    printf "AURAGO_MASTER_KEY=%s\n" "$MASTER_KEY" > "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    ok "Master key generated → $ENV_FILE"
    warn "Keep .env safe! Losing it means losing access to your encrypted vault."
fi

# ── start.sh ─────────────────────────────────────────────────────────────
cat > "$INSTALL_DIR/start.sh" <<'STARTSH'
#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"
[ -f "$DIR/.env" ] && source "$DIR/.env"

if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
    echo "ERROR: AURAGO_MASTER_KEY is not set."
    echo "  Run: source .env   or   export AURAGO_MASTER_KEY=your_64_hex_chars"
    exit 1
fi

echo "Starting AuraGo..."
./bin/aurago_linux > log/aurago.log 2>&1 &
echo "Started (PID=$!). Web UI: http://localhost:8088"
echo "Follow logs: tail -f $DIR/log/aurago.log"
STARTSH
chmod +x "$INSTALL_DIR/start.sh"

# ── update.sh ─────────────────────────────────────────────────────────────
info "Downloading update.sh ..."
UPDATE_SH_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main/update.sh"
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$UPDATE_SH_URL" -o "$INSTALL_DIR/update.sh" 2>/dev/null || warn "Could not download update.sh — download manually later."
elif command -v wget >/dev/null 2>&1; then
    wget -q "$UPDATE_SH_URL" -O "$INSTALL_DIR/update.sh" 2>/dev/null || warn "Could not download update.sh — download manually later."
fi
[ -f "$INSTALL_DIR/update.sh" ] && chmod +x "$INSTALL_DIR/update.sh" && ok "update.sh installed."

# ── Network binding ───────────────────────────────────────────────────────
echo ""
echo -e "${YELLOW}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║  ⚠  SECURITY WARNING                                        ║${NC}"
echo -e "${YELLOW}║  NEVER enable outside access on an internet-facing server!  ║${NC}"
echo -e "${YELLOW}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "  Only allow network access if AuraGo runs on a trusted local LAN"
echo "  (e.g. a home server / Proxmox container) — never expose it directly"
echo "  to the internet."
echo ""
read -r -p "Enable access from outside localhost? [y/N]: " NET_REPLY < /dev/tty || true

if [[ "${NET_REPLY:-n}" =~ ^[Yy]$ ]]; then
    SERVER_HOST="0.0.0.0"
    warn "Web UI will listen on ALL interfaces (0.0.0.0:8088)."
else
    SERVER_HOST="127.0.0.1"
    ok "Web UI will only be reachable locally (127.0.0.1:8088). ✅"
fi

CONFIG_FILE="$INSTALL_DIR/config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    awk -v host="$SERVER_HOST" '
        /^server:/ { in_server=1 }
        /^[a-z]/ && !/^server:/ { in_server=0 }
        in_server && /^[[:space:]]+host:/ { sub(/host:.*/, "host: " host) }
        { print }
    ' "$CONFIG_FILE" > "$CONFIG_FILE.tmp" && mv "$CONFIG_FILE.tmp" "$CONFIG_FILE"
    ok "config.yaml → server.host set to $SERVER_HOST"
else
    warn "config.yaml not found — skipping host configuration."
fi

# ── Optional systemd service ──────────────────────────────────────────────
SERVICE_INSTALLED=false
if command -v systemctl >/dev/null 2>&1; then
    echo ""
    read -r -p "Install as systemd service (auto-start on boot)? [Y/n]: " SVC_REPLY < /dev/tty || true
    if [[ "${SVC_REPLY:-y}" =~ ^[Yy]$ ]]; then
        $SUDO tee /etc/systemd/system/${SYSTEMD_SERVICE}.service > /dev/null <<EOF
[Unit]
Description=AuraGo AI Agent
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/bin/aurago_linux --config ${INSTALL_DIR}/config.yaml
Restart=on-failure
RestartSec=5
EnvironmentFile=-${INSTALL_DIR}/.env

[Install]
WantedBy=multi-user.target
EOF
        $SUDO systemctl daemon-reload
        $SUDO systemctl enable "$SYSTEMD_SERVICE"
        SERVICE_INSTALLED=true
        ok "Systemd service installed and enabled."
    fi
fi

# ── Summary ───────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  AuraGo installed at: $INSTALL_DIR${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ "$SERVICE_INSTALLED" = "true" ]; then
    echo "  Next steps:"
    echo "  1. Edit config:  nano $CONFIG_FILE"
    echo "     Set at minimum: llm.api_key"
    echo "  2. Start:        sudo systemctl start $SYSTEMD_SERVICE"
    echo ""
    echo -e "  ${CYAN}Service status:${NC}  sudo systemctl status $SYSTEMD_SERVICE"
    echo -e "  ${CYAN}Logs:           ${NC}  sudo journalctl -u $SYSTEMD_SERVICE -f"
else
    echo "  Next steps:"
    echo "  1. Edit config:  nano $CONFIG_FILE"
    echo "     Set at minimum: llm.api_key"
    echo "  2. Start:        cd $INSTALL_DIR && source .env && ./start.sh"
    echo "  3. Open UI:      http://localhost:8088"
    echo ""
    echo -e "  ${CYAN}Logs:${NC}  tail -f $INSTALL_DIR/log/aurago.log"
fi
echo ""
if $BUILD_FROM_SOURCE; then
    echo -e "  ${CYAN}Update later:${NC}  cd $INSTALL_DIR && bash update.sh"
    echo    "               (or rebuild: go build -o bin/aurago_linux ./cmd/aurago)"
else
    echo -e "  ${CYAN}Update later:${NC}  cd $INSTALL_DIR && bash update.sh"
    echo    "               (downloads latest release and merges your config automatically)"
fi
echo ""
echo -e "${GREEN}Setup complete! Finish configuration in the Web UI.${NC}"
echo -e "Go to the ${BOLD}CONFIG${NC} section to set up your LLM provider and API keys."
echo ""

if [ "$PYTHON_MISSING" = "true" ]; then
    echo -e "${YELLOW}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${YELLOW}║  ⚠  Python not installed — Python-based skills will not work.   ║${NC}"
    echo -e "${YELLOW}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
fi
set -euo pipefail
