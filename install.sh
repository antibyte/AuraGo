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

INSTALL_SUCCESS=0
SERVICE_FILE_CREATED=0
CREDENTIAL_FILE_CREATED=0
SERVICE_ENABLED=0
RELEASE_CHECKSUMS_FILE=""

GITHUB_REPO="antibyte/AuraGo"
REPO="https://github.com/${GITHUB_REPO}.git"
INSTALL_DIR="${AURAGO_DIR:-$HOME/aurago}"
SYSTEMD_SERVICE="aurago"
GO_VERSION="1.26.2"
GO_INSTALL_DIR="/usr/local"

# ── UI & Typography ──────────────────────────────────────────────────────
RED='\033[38;5;196m'
YELLOW='\033[38;5;220m'
GREEN='\033[38;5;114m'
CYAN='\033[38;5;86m'
BLUE='\033[38;5;39m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

ICO_INFO="*"
ICO_OK="OK"
ICO_WARN="!!"
ICO_ERR="ERR"

info() { echo -e "${CYAN}${ICO_INFO} AuraGo${NC} -> $*"; }
ok()   { echo -e "${GREEN}${ICO_OK}${NC}        -> $*"; }
warn() { echo -e "${YELLOW}${ICO_WARN} WARN${NC}  -> $*"; }
die()  { echo -e "${RED}${ICO_ERR} ERROR${NC} -> $*"; exit 1; }

cleanup_install_failure() {
    local exit_code=$?
    if [ "$exit_code" -eq 0 ] || [ "$INSTALL_SUCCESS" -eq 1 ]; then
        [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && rm -f "$RELEASE_CHECKSUMS_FILE"
        return 0
    fi

    warn "Installation aborted — cleaning up partially created service artifacts."
    if [ "$SERVICE_ENABLED" -eq 1 ] && command -v systemctl >/dev/null 2>&1; then
        ${SUDO:-} systemctl disable --now "$SYSTEMD_SERVICE" >/dev/null 2>&1 || true
    fi
    if [ "$SERVICE_FILE_CREATED" -eq 1 ] && [ -f "/etc/systemd/system/${SYSTEMD_SERVICE}.service" ]; then
        ${SUDO:-} rm -f "/etc/systemd/system/${SYSTEMD_SERVICE}.service" >/dev/null 2>&1 || true
        command -v systemctl >/dev/null 2>&1 && ${SUDO:-} systemctl daemon-reload >/dev/null 2>&1 || true
    fi
    if [ "$CREDENTIAL_FILE_CREATED" -eq 1 ] && [ -f "/etc/aurago/master.key" ]; then
        ${SUDO:-} rm -f "/etc/aurago/master.key" >/dev/null 2>&1 || true
    fi
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && rm -f "$RELEASE_CHECKSUMS_FILE"
}

trap cleanup_install_failure EXIT

is_valid_master_key() {
    printf '%s' "${1:-}" | grep -Eq '^[0-9a-fA-F]{64}$'
}

read_env_value() {
    local env_file="$1"
    local env_key="$2"
    [ -f "$env_file" ] || return 1
    awk -F= -v key="$env_key" '
        $1 == key {
            sub(/^[^=]*=/, "", $0)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
            gsub(/^["'"'"']|["'"'"']$/, "", $0)
            print $0
            exit
        }
    ' "$env_file"
}

write_master_key_file() {
    local target="$1"
    local key="$2"
    local tmp
    is_valid_master_key "$key" || return 1
    tmp="${target}.tmp.$$"
    (umask 077 && printf 'AURAGO_MASTER_KEY=%s\n' "$key" > "$tmp") || return 1
    mv -f "$tmp" "$target"
}

write_secret_text_file() {
    local target="$1"
    local value="$2"
    local tmp
    tmp="${target}.tmp.$$"
    (umask 077 && printf '%s\n' "$value" > "$tmp") || return 1
    mv -f "$tmp" "$target"
}

generate_master_key() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 32 2>/dev/null && return 0
    fi
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import secrets; print(secrets.token_hex(32))" 2>/dev/null && return 0
    fi
    return 1
}

version_ge() {
    local lhs="${1#v}" rhs="${2#v}" i
    local IFS=.
    local lhs_parts=() rhs_parts=()
    read -r -a lhs_parts <<< "$lhs"
    read -r -a rhs_parts <<< "$rhs"
    local max_len="${#lhs_parts[@]}"
    if [ "${#rhs_parts[@]}" -gt "$max_len" ]; then
        max_len="${#rhs_parts[@]}"
    fi
    for ((i=0; i<max_len; i++)); do
        local l="${lhs_parts[i]:-0}"
        local r="${rhs_parts[i]:-0}"
        l="${l%%[^0-9]*}"
        r="${r%%[^0-9]*}"
        l="${l:-0}"
        r="${r:-0}"
        if ((10#$l > 10#$r)); then
            return 0
        fi
        if ((10#$l < 10#$r)); then
            return 1
        fi
    done
    return 0
}

stat_owner() {
    local path="$1"
    if stat -c '%U' "$path" >/dev/null 2>&1; then
        stat -c '%U' "$path"
    elif stat -f '%Su' "$path" >/dev/null 2>&1; then
        stat -f '%Su' "$path"
    else
        return 1
    fi
}

latest_release_tag_via_redirect() {
    local latest_url="https://github.com/${GITHUB_REPO}/releases/latest"
    local effective_url=""
    if command -v curl >/dev/null 2>&1; then
        effective_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$latest_url" 2>/dev/null || true)"
    elif command -v wget >/dev/null 2>&1; then
        effective_url="$(wget -S --max-redirect=10 --spider "$latest_url" 2>&1 | awk '/^  Location: / {print $2}' | tail -n1 | tr -d '\r' || true)"
    fi
    [ -n "$effective_url" ] || return 1
    basename "$effective_url"
}

install_ffmpeg() {
    case "$PKG_MGR" in
        apt)
            _pkg_install ffmpeg
            ;;
        dnf)
            if $SUDO dnf install -y ffmpeg --allowerasing 2>/dev/null; then
                return 0
            fi
            if [ -f /etc/fedora-release ] && command -v rpm >/dev/null 2>&1; then
                local fedora_release
                fedora_release="$(rpm -E %fedora 2>/dev/null || true)"
                if printf '%s' "$fedora_release" | grep -Eq '^[0-9]+$'; then
                    $SUDO dnf install -y "https://download1.rpmfusion.org/free/fedora/rpmfusion-free-release-${fedora_release}.noarch.rpm" 2>/dev/null || return 1
                    $SUDO dnf install -y ffmpeg
                    return $?
                fi
            fi
            return 1
            ;;
        *)
            _pkg_install ffmpeg
            ;;
    esac
}

install_docker_engine() {
    case "$PKG_MGR" in
        apt)
            $SUDO apt-get update
            $SUDO apt-get install -y docker.io
            ;;
        pacman)
            _pkg_install docker
            ;;
        apk)
            _pkg_install docker docker-cli containerd
            ;;
        zypper)
            _pkg_install docker
            ;;
        dnf|yum)
            warn "Automatic Docker installation for ${PKG_MGR} is disabled here to avoid piping a remote script into sh."
            warn "Install Docker manually via your distro's documented repository setup, then rerun this installer."
            return 1
            ;;
        *)
            warn "Cannot install Docker automatically on this system."
            return 1
            ;;
    esac

    if command -v systemctl >/dev/null 2>&1; then
        $SUDO systemctl enable --now docker >/dev/null 2>&1 || warn "Could not enable/start docker.service automatically."
    fi
}

update_source_checkout() {
    local branch upstream
    if ! git -C "$INSTALL_DIR" diff --quiet --ignore-submodules -- || ! git -C "$INSTALL_DIR" diff --cached --quiet --ignore-submodules --; then
        warn "Local tracked changes detected in $INSTALL_DIR — skipping automatic git fast-forward."
        warn "Commit/stash your changes and rerun the installer if you want the source checkout updated."
        return 0
    fi
    git -C "$INSTALL_DIR" fetch --tags --prune origin || return 1
    branch="$(git -C "$INSTALL_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo main)"
    upstream="origin/${branch}"
    if ! git -C "$INSTALL_DIR" rev-parse --verify "$upstream" >/dev/null 2>&1; then
        warn "Upstream branch ${upstream} not found — keeping current checkout unchanged."
        return 0
    fi
    if git -C "$INSTALL_DIR" merge --ff-only "$upstream"; then
        ok "Updated source checkout to latest ${upstream}."
    else
        warn "Fast-forward merge failed for ${upstream} — keeping current checkout unchanged."
    fi
}

warn_if_systemd_hardening_conflicts() {
    local config_path="$1"
    [ -f "$config_path" ] || return 0
    if grep -Eq '^[[:space:]]+sudo_enabled:[[:space:]]*true([[:space:]]|$)' "$config_path" || \
       grep -Eq '^[[:space:]]+sudo_unrestricted:[[:space:]]*true([[:space:]]|$)' "$config_path"; then
        warn "config.yaml enables sudo features. The generated systemd unit keeps ProtectSystem=strict and may block unrestricted sudo writes."
        warn "If you intentionally need unrestricted sudo, adjust the unit after installation and reload systemd."
    fi
}

G1='\033[38;5;39m'
G2='\033[38;5;38m'
G3='\033[38;5;37m'
G4='\033[38;5;36m'

echo ""
echo -e " ${G1}+--------------------------------------+${NC}"
echo -e " ${G2}|${NC} ${BOLD}AuraGo Quick Installer${NC}               ${G2}|${NC}"
echo -e " ${G3}|${NC} ${DIM}AI Agent Framework for Linux${NC}          ${G3}|${NC}"
echo -e " ${G4}+--------------------------------------+${NC}"
echo ""

# ── Architecture detection ──────────────────────────────────────────────
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)        GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l|armv6l) die "ARMv6/v7 is not supported by the current installer artifacts yet. Please use a supported release or build manually." ;;
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

fetch_url_stdout() {
    local url="$1"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url"
    else
        die "Neither curl nor wget available."
    fi
}

sha256_file() {
    local path="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$path" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$path" | awk '{print $1}'
    elif command -v openssl >/dev/null 2>&1; then
        openssl dgst -sha256 "$path" | awk '{print $NF}'
    else
        return 1
    fi
}

fetch_release_checksums() {
    [ -n "${RELEASE_BASE:-}" ] || die "RELEASE_BASE is not set."
    if [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "${RELEASE_CHECKSUMS_FILE:-}" ]; then
        return 0
    fi
    RELEASE_CHECKSUMS_FILE="$(mktemp)"
    if ! _download "${RELEASE_BASE}/SHA256SUMS" "$RELEASE_CHECKSUMS_FILE"; then
        rm -f "$RELEASE_CHECKSUMS_FILE"
        RELEASE_CHECKSUMS_FILE=""
        return 1
    fi
}

verify_release_asset() {
    local asset="$1"
    local path="$2"
    local expected actual
    [ -f "$path" ] || die "Cannot verify missing file: $path"
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "$RELEASE_CHECKSUMS_FILE" ] || die "Release checksums are not available."
    expected="$(awk -v target="$asset" '$2 == target {print $1; exit}' "$RELEASE_CHECKSUMS_FILE")"
    [ -n "$expected" ] || die "Missing checksum entry for ${asset} in release manifest."
    actual="$(sha256_file "$path" || true)"
    [ -n "$actual" ] || die "No SHA256 tool available to verify ${asset}."
    [ "$actual" = "$expected" ] || die "Checksum verification failed for ${asset}."
}

download_release_asset() {
    local asset="$1"
    local dest="$2"
    _download "${RELEASE_BASE}/${asset}" "$dest"
    verify_release_asset "$asset" "$dest"
}

latest_release_tag() {
    local tag
    tag="$(fetch_url_stdout "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
        | grep -o '"tag_name": *"[^"]*"' \
        | head -1 \
        | cut -d'"' -f4)"
    if [ -n "$tag" ]; then
        printf '%s\n' "$tag"
        return 0
    fi
    latest_release_tag_via_redirect
}

# ── Optional system dependencies ─────────────────────────────────────────
info "Checking system dependencies..."

# ffmpeg (needed for Telegram voice conversion)
if ! command -v ffmpeg >/dev/null 2>&1; then
    warn "ffmpeg not found."
    read -r -p "Install ffmpeg? [Y/n]: " FF_REPLY < /dev/tty || true
    if [[ "${FF_REPLY:-y}" =~ ^[Yy]$ ]]; then
        if install_ffmpeg; then
            ok "ffmpeg installed."
        else
            warn "ffmpeg installation failed. Install it manually for your distro and rerun the installer if you need voice conversion."
        fi
    else
        warn "Skipping ffmpeg. Telegram voice messages will not work."
    fi
else
    ok "ffmpeg found."
fi

# ImageMagick (needed for image format conversion)
if ! command -v magick >/dev/null 2>&1 && ! command -v convert >/dev/null 2>&1; then
    warn "ImageMagick not found."
    read -r -p "Install ImageMagick for image conversion? [Y/n]: " IM_REPLY < /dev/tty || true
    if [[ "${IM_REPLY:-y}" =~ ^[Yy]$ ]]; then
        case "$PKG_MGR" in
            dnf)    _pkg_install ImageMagick ;;
            *)      _pkg_install imagemagick ;;
        esac
        ok "ImageMagick installed."
    else
        warn "Skipping ImageMagick. Image format conversion will not work."
    fi
else
    ok "ImageMagick found."
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

# Docker (needed for many useful features)
if ! command -v docker >/dev/null 2>&1; then
    echo ""
    echo -e " ${G1}+--------------------------------------------------------------+${NC}"
    echo -e " ${G2}|${NC}  ${BOLD}Docker not found${NC}                                            ${G2}|${NC}"
    echo -e " ${G3}|${NC}  Many useful AuraGo features (Sandbox, tools) need Docker.   ${G3}|${NC}"
    echo -e " ${G4}+--------------------------------------------------------------+${NC}"
    echo ""
    read -r -p "Install Docker now? (Recommended) [Y/n]: " DKR_REPLY < /dev/tty || true
    if [[ "${DKR_REPLY:-y}" =~ ^[Yy]$ ]]; then
        info "Installing Docker via the local package manager..."
        if install_docker_engine; then
            $SUDO usermod -aG docker "$USER" || warn "Failed to add $USER to docker group. You may need to do this manually."
            ok "Docker installed."
        else
            warn "Docker was not installed automatically."
        fi
    else
        warn "Skipping Docker installation."
    fi
else
    ok "Docker found."
fi

# ══════════════════════════════════════════════════════════════════════════
#  Decide installation mode: SOURCE BUILD vs BINARY INSTALL
# ══════════════════════════════════════════════════════════════════════════
# Add common Go install locations to PATH (in case Go was already installed but not in PATH)
for _godir in /usr/local/go/bin "$HOME/go/bin" /usr/local/bin; do
    [ -d "$_godir" ] && [[ ":$PATH:" != *":$_godir:"* ]] && export PATH="$_godir:$PATH"
done
unset _godir

BUILD_FROM_SOURCE=false

_go_version_ok() {
    local installed
    installed=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//')
    [ -n "$installed" ] || return 1
    version_ge "$installed" "$GO_VERSION"
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
        update_source_checkout || die "Failed to update source checkout."
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

    CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" \
        go build -trimpath -ldflags="-s -w" -o bin/aurago-remote_linux ./cmd/remote
    ok "bin/aurago-remote_linux built."

    # Install config.yaml from template if none exists (source build has config_template.yaml in repo)
    if [ ! -f "$INSTALL_DIR/config.yaml" ] && [ -f "$INSTALL_DIR/config_template.yaml" ]; then
        cp "$INSTALL_DIR/config_template.yaml" "$INSTALL_DIR/config.yaml"
        ok "config.yaml created from template."
    fi

# ══════════════════════════════════════════════════════════════════════════
#  MODE B: Binary install — download from GitHub Releases (no clone)
# ══════════════════════════════════════════════════════════════════════════
else
    info "Binary install — downloading from GitHub Releases..."

    # Resolve the latest release tag
    RELEASE_TAG="$(latest_release_tag || true)"
    [ -z "$RELEASE_TAG" ] && die "Could not determine latest release tag from GitHub."
    info "Latest release: $RELEASE_TAG"

    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    fetch_release_checksums || die "Could not download SHA256SUMS for release ${RELEASE_TAG}."

    # Create install directory + subdirectories
    mkdir -p "$INSTALL_DIR/bin" "$INSTALL_DIR/data" "$INSTALL_DIR/log"
    mkdir -p "$INSTALL_DIR/agent_workspace/workdir" "$INSTALL_DIR/agent_workspace/tools"
    cd "$INSTALL_DIR"

    EXISTING_CONFIG_BAK=""
    if [ -f "$INSTALL_DIR/config.yaml" ]; then
        EXISTING_CONFIG_BAK="$(mktemp)"
        cp -p "$INSTALL_DIR/config.yaml" "$EXISTING_CONFIG_BAK"
    fi

    # Download resources.dat and extract (contains prompts, skills, config template, UI)
    info "Downloading resources.dat ..."
    download_release_asset "resources.dat" "$INSTALL_DIR/resources.dat"
    # Extract to a temp dir so we can selectively merge (never clobber existing config)
    TMPEXT=$(mktemp -d)
    tar -xzf "$INSTALL_DIR/resources.dat" -C "$TMPEXT"
    rm -f "$INSTALL_DIR/resources.dat"

    # Copy prompts, skills, and other resource dirs (always overwrite — they are code)
    [ -d "$TMPEXT/prompts" ]           && cp -a "$TMPEXT/prompts"           "$INSTALL_DIR/"
    [ -d "$TMPEXT/agent_workspace" ]   && cp -a "$TMPEXT/agent_workspace"   "$INSTALL_DIR/"
    [ -d "$TMPEXT/ui" ]               && cp -a "$TMPEXT/ui"               "$INSTALL_DIR/" 2>/dev/null || true
    [ -d "$TMPEXT/assets" ]           && cp -a "$TMPEXT/assets"           "$INSTALL_DIR/"

    # Save the freshly shipped template for merge/copy after config-merger is available.
    if [ -f "$TMPEXT/config.yaml" ]; then
        cp "$TMPEXT/config.yaml" "$INSTALL_DIR/config.yaml.new_template"
    fi
    rm -rf "$TMPEXT"
    ok "Resources extracted."

    # Download binaries
    if [ "$GOARCH" = "arm64" ]; then
        info "Downloading arm64 binaries..."
        download_release_asset "aurago_linux_arm64"                "bin/aurago_linux_arm64"
        download_release_asset "lifeboat_linux_arm64"              "bin/lifeboat_linux_arm64"              2>/dev/null || warn "lifeboat_linux_arm64 not in release."
        download_release_asset "config-merger_linux_arm64"         "bin/config-merger_linux_arm64"         2>/dev/null || warn "config-merger_linux_arm64 not in release."
        download_release_asset "aurago-remote_linux_arm64"         "bin/aurago-remote_linux_arm64"         2>/dev/null || warn "aurago-remote_linux_arm64 not in release."
        cp bin/aurago_linux_arm64           bin/aurago_linux
        cp bin/lifeboat_linux_arm64         bin/lifeboat_linux             2>/dev/null || true
        cp bin/config-merger_linux_arm64    bin/config-merger_linux         2>/dev/null || true
        cp bin/aurago-remote_linux_arm64    bin/aurago-remote_linux         2>/dev/null || true
    else
        info "Downloading amd64 binaries..."
        download_release_asset "aurago_linux"                      "bin/aurago_linux"
        download_release_asset "lifeboat_linux"                    "bin/lifeboat_linux"                    2>/dev/null || warn "lifeboat_linux not in release."
        download_release_asset "config-merger_linux"               "bin/config-merger_linux"               2>/dev/null || warn "config-merger_linux not in release."
        download_release_asset "aurago-remote_linux"               "bin/aurago-remote_linux"               2>/dev/null || warn "aurago-remote_linux not in release."
    fi
    # Record installed version for update checks
    printf '%s' "$RELEASE_TAG" > "$INSTALL_DIR/.version"
    ok "Binaries downloaded."
fi

chmod +x bin/aurago_linux bin/lifeboat_linux bin/config-merger_linux bin/aurago-remote_linux 2>/dev/null || true
ok "Binaries ready."

if ! $BUILD_FROM_SOURCE; then
    if [ -f "$INSTALL_DIR/config.yaml.new_template" ]; then
        if [ -n "${EXISTING_CONFIG_BAK:-}" ] && [ -f "${EXISTING_CONFIG_BAK:-}" ]; then
            if [ -x "$INSTALL_DIR/bin/config-merger_linux" ]; then
                info "Merging existing config.yaml with new template defaults ..."
                if "$INSTALL_DIR/bin/config-merger_linux" -source "$EXISTING_CONFIG_BAK" -template "$INSTALL_DIR/config.yaml.new_template" -output "$INSTALL_DIR/config.yaml"; then
                    ok "config.yaml merged with latest template defaults."
                else
                    warn "config-merger failed. Restoring previous config.yaml."
                    cp -p "$EXISTING_CONFIG_BAK" "$INSTALL_DIR/config.yaml"
                fi
            else
                warn "config-merger not available. Keeping existing config.yaml unchanged."
            fi
        elif [ ! -f "$INSTALL_DIR/config.yaml" ]; then
            cp "$INSTALL_DIR/config.yaml.new_template" "$INSTALL_DIR/config.yaml"
            ok "config.yaml installed from release template."
        fi
        rm -f "$INSTALL_DIR/config.yaml.new_template"
    fi
    [ -n "${EXISTING_CONFIG_BAK:-}" ] && rm -f "$EXISTING_CONFIG_BAK"

    info "Downloading update.sh ..."
    if download_release_asset "update.sh" "$INSTALL_DIR/update.sh"; then
        chmod +x "$INSTALL_DIR/update.sh"
        ok "update.sh installed."
    else
        warn "Could not download verified update.sh — install it manually later."
    fi

    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && rm -f "$RELEASE_CHECKSUMS_FILE"
fi

# ── Master key ────────────────────────────────────────────────────────────
ENV_FILE="$INSTALL_DIR/.env"
if [ -f "$ENV_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$ENV_FILE"; then
    EXISTING_MASTER_KEY="$(read_env_value "$ENV_FILE" "AURAGO_MASTER_KEY" || true)"
    is_valid_master_key "$EXISTING_MASTER_KEY" || die "Existing $ENV_FILE contains an invalid AURAGO_MASTER_KEY."
    warn ".env already has AURAGO_MASTER_KEY — keeping existing key."
else
    MASTER_KEY="$(generate_master_key || true)"
    is_valid_master_key "$MASTER_KEY" || die "Failed to generate a valid AURAGO_MASTER_KEY. Please install openssl or python3."
    write_master_key_file "$ENV_FILE" "$MASTER_KEY" || die "Failed to write $ENV_FILE securely."
    ok "Master key generated → $ENV_FILE"
    warn "Keep .env safe! Losing it means losing access to your encrypted vault."
fi

# ── start.sh ─────────────────────────────────────────────────────────────
cat > "$INSTALL_DIR/start.sh" <<'STARTSH'
#!/usr/bin/env bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

read_env_value() {
    local env_file="$1"
    local env_key="$2"
    [ -f "$env_file" ] || return 1
    awk -F= -v key="$env_key" '
        $1 == key {
            sub(/^[^=]*=/, "", $0)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0)
            gsub(/^["'"'"']|["'"'"']$/, "", $0)
            print $0
            exit
        }
    ' "$env_file"
}

# Load master key: prefer system-wide credential, fall back to local .env
if [ -f /etc/aurago/master.key ]; then
    AURAGO_MASTER_KEY="$(read_env_value /etc/aurago/master.key AURAGO_MASTER_KEY)"
elif [ -f "$DIR/.env" ]; then
    AURAGO_MASTER_KEY="$(read_env_value "$DIR/.env" AURAGO_MASTER_KEY)"
fi
export AURAGO_MASTER_KEY

if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
    echo "ERROR: AURAGO_MASTER_KEY is not set."
    echo "  Expected at: /etc/aurago/master.key  or  $DIR/.env"
    exit 1
fi

echo "Starting AuraGo..."
./bin/aurago_linux > log/aurago.log 2>&1 &
echo "Started (PID=$!). Web UI: http://localhost:8088"
echo "Follow logs: tail -f $DIR/log/aurago.log"
STARTSH
chmod +x "$INSTALL_DIR/start.sh"

# ── Network binding & HTTPS ───────────────────────────────────────────────
echo ""
echo -e " ${YELLOW}+--------------------------------------------------------------+${NC}"
echo -e " ${YELLOW}|${NC}  ${BOLD}!! NETWORK SETUP${NC}                                             ${YELLOW}|${NC}"
echo -e " ${YELLOW}|${NC}  Configure external access and HTTPS for this installation.  ${YELLOW}|${NC}"
echo -e " ${YELLOW}+--------------------------------------------------------------+${NC}"
echo ""

read -r -p "Is this an internet-facing server and do you want to enable HTTPS (Let's Encrypt)? [y/N]: " HTTPS_REPLY < /dev/tty || true

SERVER_HOST="127.0.0.1"
HTTPS_ENABLED="false"

if [[ "${HTTPS_REPLY:-n}" =~ ^[Yy]$ ]]; then
    SERVER_HOST="0.0.0.0"
    HTTPS_ENABLED="true"
    read -r -p "Enter your domain (e.g., aurago.example.com): " HTTPS_DOMAIN < /dev/tty || true
    read -r -p "Enter your email for Let's Encrypt: " HTTPS_EMAIL < /dev/tty || true
    ok "Web UI will listen on ALL interfaces (0.0.0.0:443) with HTTPS."
else
    echo ""
    echo "  Only allow network access if AuraGo runs on a trusted local LAN"
    echo "  (e.g. a home server / Proxmox container) — never expose it directly"
    echo "  to the internet without HTTPS / reverse proxy."
    echo ""
    read -r -p "Enable HTTP access from outside localhost (LAN)? [y/N]: " NET_REPLY < /dev/tty || true
    if [[ "${NET_REPLY:-n}" =~ ^[Yy]$ ]]; then
        SERVER_HOST="0.0.0.0"
        warn "Web UI will listen on ALL interfaces (0.0.0.0:8088) without HTTPS."
    else
        ok "Web UI will only be reachable locally (127.0.0.1:8088)."
    fi
fi

# ── CAP_NET_BIND_SERVICE for HTTPS on standard ports ─────────────────────
# Ports 80 and 443 require root or this capability. Set it so AuraGo
# can bind them as a normal user — prevents a hard startup failure.
if [ "$HTTPS_ENABLED" = "true" ]; then
    if ! command -v setcap >/dev/null 2>&1; then
        info "Installing libcap2-bin (needed for setcap)..."
        _pkg_install libcap2-bin 2>/dev/null || \
        _pkg_install libcap 2>/dev/null || \
        warn "setcap not available. Install libcap2-bin and run manually: sudo setcap cap_net_bind_service=+ep $INSTALL_DIR/bin/aurago_linux"
    fi
    if command -v setcap >/dev/null 2>&1; then
        $SUDO setcap cap_net_bind_service=+ep "$INSTALL_DIR/bin/aurago_linux" 2>/dev/null && \
            ok "CAP_NET_BIND_SERVICE granted — AuraGo can bind port 443 without root." || \
            warn "setcap failed. Run manually: sudo setcap cap_net_bind_service=+ep $INSTALL_DIR/bin/aurago_linux"
    fi
fi

CONFIG_FILE="$INSTALL_DIR/config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    INFO_PASSWORD=$(openssl rand -base64 12 2>/dev/null || python3 -c "import secrets; print(secrets.token_urlsafe(12))")

    # Save first-use password (owner-readable only)
    write_secret_text_file "$INSTALL_DIR/firstpassword.txt" "$INFO_PASSWORD" || die "Failed to write firstpassword.txt securely."

    if [ "$HTTPS_ENABLED" = "true" ]; then
        ./bin/aurago_linux --config "$CONFIG_FILE" --init-only -password "$INFO_PASSWORD" -https -domain "$HTTPS_DOMAIN" -email "$HTTPS_EMAIL"
        ok "config.yaml → HTTPS enabled for $HTTPS_DOMAIN"
    else
        ./bin/aurago_linux --config "$CONFIG_FILE" --init-only -password "$INFO_PASSWORD"
        awk -v host="$SERVER_HOST" '
            /^server:/ { in_server=1 }
            /^[a-z]/ && !/^server:/ { in_server=0 }
            in_server && /^[[:space:]]+host:/ { sub(/host:.*/, "host: " host) }
            { print }
        ' "$CONFIG_FILE" > "$CONFIG_FILE.tmp" && mv "$CONFIG_FILE.tmp" "$CONFIG_FILE"
        ok "config.yaml → server.host set to $SERVER_HOST"
    fi
else
    # config.yaml still missing — create from template as last resort
    TEMPLATE_FILE="$INSTALL_DIR/config_template.yaml"
    if [ -f "$TEMPLATE_FILE" ]; then
        cp "$TEMPLATE_FILE" "$CONFIG_FILE"
        # Apply chosen host binding directly via awk
        awk -v host="$SERVER_HOST" '
            /^server:/ { in_server=1 }
            /^[a-z]/ && !/^server:/ { in_server=0 }
            in_server && /^[[:space:]]+host:/ { sub(/host:.*/, "host: " host) }
            { print }
        ' "$CONFIG_FILE" > "$CONFIG_FILE.tmp" && mv "$CONFIG_FILE.tmp" "$CONFIG_FILE"
        ok "config.yaml created from template (server.host=$SERVER_HOST)."
    else
        warn "config.yaml not found and no template available — skipping host configuration."
    fi
fi


# ── Optional systemd service ──────────────────────────────────────────────
SERVICE_INSTALLED=false
if command -v systemctl >/dev/null 2>&1; then
    echo ""
    read -r -p "Install as systemd service (auto-start on boot)? [Y/n]: " SVC_REPLY < /dev/tty || true
    if [[ "${SVC_REPLY:-y}" =~ ^[Yy]$ ]]; then

        # ── Move master key to /etc/aurago/master.key (root-only) ─────────
        CREDENTIAL_DIR="/etc/aurago"
        CREDENTIAL_FILE="${CREDENTIAL_DIR}/master.key"
        if [ -f "$CREDENTIAL_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$CREDENTIAL_FILE"; then
            warn "$CREDENTIAL_FILE already exists — keeping existing key."
        else
            AURAGO_MASTER_KEY="$(read_env_value "$ENV_FILE" "AURAGO_MASTER_KEY" || true)"
            is_valid_master_key "$AURAGO_MASTER_KEY" || die "Cannot migrate master key — AURAGO_MASTER_KEY is missing or invalid."
            $SUDO mkdir -p "$CREDENTIAL_DIR"
            $SUDO chmod 700 "$CREDENTIAL_DIR"
            printf "AURAGO_MASTER_KEY=%s\n" "$AURAGO_MASTER_KEY" | $SUDO tee "$CREDENTIAL_FILE" > /dev/null
            $SUDO chmod 600 "$CREDENTIAL_FILE"
            $SUDO chown root:root "$CREDENTIAL_DIR" "$CREDENTIAL_FILE"
            CREDENTIAL_FILE_CREATED=1
            ok "Master key moved to $CREDENTIAL_FILE (root-only, mode 0600)."
        fi

        # Remove the plaintext .env from the install directory
        if [ -f "$ENV_FILE" ]; then
            rm -f "$ENV_FILE"
            ok "Removed $ENV_FILE (no longer needed — key is in $CREDENTIAL_FILE)."
        fi

        # ── Determine service user ─────────────────────────────────────────
        # When invoked via 'sudo ./install.sh', SUDO_USER is the real user.
        # When run directly as a non-root user, use the current user.
        # Avoid running the service as root if at all possible.
        if [ -n "${SUDO_USER:-}" ]; then
            SERVICE_USER="$SUDO_USER"
            SERVICE_GROUP="$(id -gn "$SUDO_USER")"
        elif [ "$(id -u)" -ne 0 ]; then
            SERVICE_USER="$(id -un)"
            SERVICE_GROUP="$(id -gn)"
        else
            # Running directly as root — derive user from install directory owner
            _dir_owner="$(stat_owner "$INSTALL_DIR" 2>/dev/null || echo '')"
            if [ -n "$_dir_owner" ] && [ "$_dir_owner" != "root" ]; then
                SERVICE_USER="$_dir_owner"
                SERVICE_GROUP=$(id -gn "$_dir_owner" 2>/dev/null || echo "$_dir_owner")
            else
                SERVICE_USER="root"
                SERVICE_GROUP="root"
                warn "Could not determine a non-root service user. Service will run as root."
                warn "For better security, create a dedicated user: useradd -r -s /bin/false aurago"
            fi
        fi
        ok "Service will run as: ${SERVICE_USER}:${SERVICE_GROUP}"

        warn_if_systemd_hardening_conflicts "$CONFIG_FILE"

        # Ensure only AuraGo-managed writable paths are owned by the service user.
        CHOWN_TARGETS=()
        for _path in \
            "$INSTALL_DIR/bin" \
            "$INSTALL_DIR/data" \
            "$INSTALL_DIR/log" \
            "$INSTALL_DIR/agent_workspace" \
            "$INSTALL_DIR/config.yaml" \
            "$INSTALL_DIR/config_template.yaml" \
            "$INSTALL_DIR/start.sh" \
            "$INSTALL_DIR/update.sh" \
            "$INSTALL_DIR/firstpassword.txt" \
            "$INSTALL_DIR/.version"; do
            [ -e "$_path" ] && CHOWN_TARGETS+=("$_path")
        done
        if [ "${#CHOWN_TARGETS[@]}" -gt 0 ]; then
            $SUDO chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${CHOWN_TARGETS[@]}" 2>/dev/null || true
        fi

        # Grant CAP_NET_BIND_SERVICE in the systemd unit so the service can bind
        # ports 80/443 as a non-root user without a setcap dependency on the binary.
        # AmbientCapabilities is compatible with NoNewPrivileges=true (systemd sets
        # the capability before the prctl call).
        AMBIENT_CAPS_LINE=""
        if [ "$HTTPS_ENABLED" = "true" ]; then
            AMBIENT_CAPS_LINE="AmbientCapabilities=CAP_NET_BIND_SERVICE"
        fi

        # ── Create systemd unit ──────────────────────────────────────────
        $SUDO tee /etc/systemd/system/${SYSTEMD_SERVICE}.service > /dev/null <<EOF
[Unit]
Description=AuraGo AI Agent
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory="${INSTALL_DIR}"
ExecStart="${INSTALL_DIR}/bin/aurago_linux" --config "${INSTALL_DIR}/config.yaml"
Restart=on-failure
RestartSec=5
EnvironmentFile="${CREDENTIAL_FILE}"
${AMBIENT_CAPS_LINE}
# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=${INSTALL_DIR} ${CREDENTIAL_DIR}
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
        SERVICE_FILE_CREATED=1
        $SUDO systemctl daemon-reload
        $SUDO systemctl enable "$SYSTEMD_SERVICE"
        SERVICE_ENABLED=1
        $SUDO systemctl start "$SYSTEMD_SERVICE"
        SERVICE_INSTALLED=true
        ok "Systemd service installed, enabled and started."

        echo ""
        echo -e " ${GREEN}+--------------------------------------------------------------+${NC}"
        echo -e " ${GREEN}|${NC}  ${BOLD}MASTER KEY SECURED${NC}                                         ${GREEN}|${NC}"
        echo -e " ${GREEN}|${NC}  Location: ${BOLD}/etc/aurago/master.key${NC} (root-only, mode 0600)    ${GREEN}|${NC}"
        echo -e " ${GREEN}|${NC}  The key is injected into AuraGo via systemd.                ${GREEN}|${NC}"
        echo -e " ${GREEN}|${NC}  ${YELLOW}Back up this file! Losing it = losing your vault.${NC}          ${GREEN}|${NC}"
        echo -e " ${GREEN}+--------------------------------------------------------------+${NC}"
    fi
fi

# ── Summary ───────────────────────────────────────────────────────────────
echo ""
echo -e " ${GREEN}+---------------------------------------------------------------+${NC}"
echo -e " ${GREEN}|${NC}  ${BOLD}AuraGo successfully installed!${NC}"
echo -e " ${GREEN}|${NC}  ${DIM}Location:${NC} $INSTALL_DIR"
echo -e " ${GREEN}+---------------------------------------------------------------+${NC}"
echo ""

if [ -n "${INFO_PASSWORD:-}" ]; then
    echo ""
    echo -e " ${GREEN}+--------------------------------------------------------------+${NC}"
    echo -e " ${GREEN}|${NC}  ${BOLD}FIRST-USE PASSWORD${NC}                                        ${GREEN}|${NC}"
    echo -e " ${GREEN}|${NC}  Password: ${BOLD}${INFO_PASSWORD}${NC}"
    echo -e " ${GREEN}|${NC}  Use this to log in to the Web UI for the first time.        ${GREEN}|${NC}"
    echo -e " ${GREEN}|${NC}  ${YELLOW}Change it immediately via Settings -> Login Guard.${NC}           ${GREEN}|${NC}"
    echo -e " ${GREEN}|${NC}  ${DIM}Also saved to: firstpassword.txt (delete after first login)${NC}  ${GREEN}|${NC}"
    echo -e " ${GREEN}+--------------------------------------------------------------+${NC}"
    echo ""
fi

if [ "$SERVICE_INSTALLED" = "true" ]; then
    echo "  Next steps:"
    echo "  1. Edit config:  nano $CONFIG_FILE"
    echo "     Set at minimum: llm.api_key"
    echo "  2. Restart after config change: sudo systemctl restart $SYSTEMD_SERVICE"
    echo ""
    echo -e "  ${CYAN}Service status:${NC}  sudo systemctl status $SYSTEMD_SERVICE"
    echo -e "  ${CYAN}Logs:           ${NC}  sudo journalctl -u $SYSTEMD_SERVICE -f"
    echo -e "  ${CYAN}Master key:    ${NC}  /etc/aurago/master.key (root-only)"
else
    echo "  Next steps:"
    echo "  1. Edit config:  nano $CONFIG_FILE"
    echo "     Set at minimum: llm.api_key"
    echo "  2. Restart after config change: cd $INSTALL_DIR && ./start.sh"
    echo "  3. Open UI:      http://localhost:8088"
    echo ""
    echo -e "  ${CYAN}Logs:${NC}  tail -f $INSTALL_DIR/log/aurago.log"

    # Start AuraGo now
    cd "$INSTALL_DIR"
    bash start.sh
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
INSTALL_SUCCESS=1
echo ""

if [ "$PYTHON_MISSING" = "true" ]; then
    echo -e " ${YELLOW}╭──────────────────────────────────────────────────────────────────╮${NC}"
    echo -e " ${YELLOW}│${NC}  ${BOLD}⚠  Python not installed${NC} — Python-based skills will not work.   ${YELLOW}│${NC}"
    echo -e " ${YELLOW}╰──────────────────────────────────────────────────────────────────╯${NC}"
    echo ""
fi
