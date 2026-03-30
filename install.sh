#!/usr/bin/env bash
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  AuraGo Bootstrap Installer  (Linux x86_64 + arm64)
#
#  Usage:
#    curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
#
#  This is a minimal bootstrap script that:
#   1. Detects architecture
#   2. Asks: build from source or download pre-built binaries?
#   3. Obtains all binaries (build or download)
#   4. Hands off to  ./bin/agocli --setup  for the real setup logic
#       (dependencies, master key, HTTPS, systemd, password, etc.)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
set -euo pipefail

GITHUB_REPO="antibyte/AuraGo"
REPO="https://github.com/${GITHUB_REPO}.git"
INSTALL_DIR="${AURAGO_DIR:-$HOME/aurago}"
GO_VERSION="1.26.0"
GO_INSTALL_DIR="/usr/local"

# ── UI helpers ────────────────────────────────────────────────────────────
RED='\033[38;5;196m'; GREEN='\033[38;5;114m'; CYAN='\033[38;5;86m'
YELLOW='\033[38;5;220m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'
info() { echo -e "${CYAN}* AuraGo${NC} -> $*"; }
ok()   { echo -e "${GREEN}OK${NC}        -> $*"; }
warn() { echo -e "${YELLOW}!! WARN${NC}  -> $*"; }
die()  { echo -e "${RED}ERR ERROR${NC} -> $*"; exit 1; }

echo ""
echo -e " ${CYAN}+--------------------------------------+${NC}"
echo -e " ${CYAN}|${NC} ${BOLD}AuraGo Bootstrap Installer${NC}           ${CYAN}|${NC}"
echo -e " ${DIM} | AI Agent Framework for Linux          |${NC}"
echo -e " ${CYAN}+--------------------------------------+${NC}"
echo ""

# ── Architecture detection ────────────────────────────────────────────────
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)        GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    *)             die "Unsupported architecture: $ARCH_RAW" ;;
esac
ok "Architecture: $ARCH_RAW → $GOARCH"

SUDO=""
[ "$(id -u)" -ne 0 ] && SUDO="sudo"

# ── Download helper ───────────────────────────────────────────────────────
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || {
    info "Installing curl..."
    if   command -v apt-get >/dev/null 2>&1; then $SUDO apt-get install -y curl
    elif command -v dnf     >/dev/null 2>&1; then $SUDO dnf install -y curl
    elif command -v pacman  >/dev/null 2>&1; then $SUDO pacman -Sy --noconfirm curl
    elif command -v apk     >/dev/null 2>&1; then $SUDO apk add curl
    fi
}

_download() {
    local url="$1" dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$dest"
    else
        wget -q "$url" -O "$dest"
    fi
}

# ══════════════════════════════════════════════════════════════════════════
#  Choose installation mode
# ══════════════════════════════════════════════════════════════════════════
for _godir in /usr/local/go/bin "$HOME/go/bin" /usr/local/bin; do
    [ -d "$_godir" ] && [[ ":$PATH:" != *":$_godir:"* ]] && export PATH="$_godir:$PATH"
done

_go_version_ok() {
    local installed
    installed=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
    [ -n "$installed" ] || return 1
    [ "$(printf '%s\n%s' "$GO_VERSION" "$installed" | sort -V | head -n1)" = "$GO_VERSION" ]
}

BUILD_FROM_SOURCE=false

if _go_version_ok; then
    ok "Go $(go version | awk '{print $3}') found."
    BUILD_FROM_SOURCE=true
else
    echo ""
    echo "  Choose installation mode:"
    echo "    1) Binary install — download pre-built binaries (fast, no Go needed)"
    echo "    2) Source build   — install Go $GO_VERSION, clone repo, build from source"
    echo ""
    read -r -p "Install mode [1/2, default=1]: " MODE_REPLY < /dev/tty || true
    if [[ "${MODE_REPLY:-1}" == "2" ]]; then
        info "Installing Go $GO_VERSION for $GOARCH..."
        GO_TAR="go${GO_VERSION}.linux-${GOARCH}.tar.gz"
        TMP_GO=$(mktemp -d)
        _download "https://go.dev/dl/${GO_TAR}" "$TMP_GO/$GO_TAR"
        $SUDO rm -rf "$GO_INSTALL_DIR/go"
        $SUDO tar -C "$GO_INSTALL_DIR" -xzf "$TMP_GO/$GO_TAR"
        rm -rf "$TMP_GO"
        export PATH="$GO_INSTALL_DIR/go/bin:$PATH"
        $SUDO tee /etc/profile.d/go.sh > /dev/null <<'GOPATH'
export PATH="/usr/local/go/bin:$PATH"
GOPATH
        ok "Go $GO_VERSION installed."
        BUILD_FROM_SOURCE=true
    fi
fi

# ══════════════════════════════════════════════════════════════════════════
#  MODE A: Source build
# ══════════════════════════════════════════════════════════════════════════
if $BUILD_FROM_SOURCE; then
    command -v git >/dev/null 2>&1 || {
        info "Installing git..."
        if   command -v apt-get >/dev/null 2>&1; then $SUDO apt-get install -y git
        elif command -v dnf     >/dev/null 2>&1; then $SUDO dnf install -y git
        elif command -v pacman  >/dev/null 2>&1; then $SUDO pacman -Sy --noconfirm git
        elif command -v apk     >/dev/null 2>&1; then $SUDO apk add git
        fi
    }

    if [ -d "$INSTALL_DIR/.git" ]; then
        info "Existing repo at $INSTALL_DIR — pulling latest..."
        git -C "$INSTALL_DIR" pull --ff-only
    else
        info "Cloning into $INSTALL_DIR ..."
        git clone "$REPO" "$INSTALL_DIR"
    fi

    cd "$INSTALL_DIR"
    mkdir -p bin

    LDFLAGS="-s -w"
    for target in \
        "aurago_linux:./cmd/aurago" \
        "lifeboat_linux:./cmd/lifeboat" \
        "config-merger_linux:./cmd/config-merger" \
        "aurago-remote_linux:./cmd/remote" \
        "agocli_linux:./cmd/agocli"; do
        OUT="${target%%:*}"; PKG="${target##*:}"
        info "Building $OUT ..."
        CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
            go build -trimpath -ldflags="$LDFLAGS" -o "bin/$OUT" "$PKG"
    done
    ok "All binaries built."

# ══════════════════════════════════════════════════════════════════════════
#  MODE B: Binary install
# ══════════════════════════════════════════════════════════════════════════
else
    RELEASE_TAG=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
                  | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
    [ -z "$RELEASE_TAG" ] && die "Could not determine latest release tag."
    info "Latest release: $RELEASE_TAG"

    BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    mkdir -p "$INSTALL_DIR/bin"
    cd "$INSTALL_DIR"

    # Download binaries
    SUFFIX=""
    [ "$GOARCH" = "arm64" ] && SUFFIX="_arm64"

    for bin in aurago_linux lifeboat_linux config-merger_linux aurago-remote_linux agocli_linux; do
        ASSET="${bin}${SUFFIX}"
        info "Downloading $ASSET ..."
        _download "${BASE}/${ASSET}" "bin/${bin}" 2>/dev/null || warn "$ASSET not in release — skipping."
    done

    # Download resources.dat
    info "Downloading resources.dat ..."
    _download "${BASE}/resources.dat" "resources.dat" 2>/dev/null || warn "resources.dat not in release."

    printf '%s' "$RELEASE_TAG" > ".version"
    ok "Binaries downloaded."
fi

chmod +x bin/* 2>/dev/null || true

# ══════════════════════════════════════════════════════════════════════════
#  Hand off to agocli --setup
# ══════════════════════════════════════════════════════════════════════════
echo ""
info "Starting AuraGo Setup Wizard..."
echo ""
exec "$INSTALL_DIR/bin/agocli_linux" --setup
