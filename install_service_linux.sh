#!/usr/bin/env bash
# AuraGo Systemd Service Installer (Linux)
# This script sets up AuraGo as a system-wide service.
# The vault master key is stored in /etc/aurago/master.key (root-only, mode 0600)
# and injected via systemd EnvironmentFile — it never appears in the unit file.

set -euo pipefail

# Configuration
SERVICE_NAME="aurago"
INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
BINARY_PATH="${INSTALL_DIR}/bin/aurago_linux"
CONFIG_PATH="${INSTALL_DIR}/config.yaml"
ENV_FILE="${INSTALL_DIR}/.env"
CREDENTIAL_DIR="/etc/aurago"
CREDENTIAL_FILE="${CREDENTIAL_DIR}/master.key"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

info() { echo -e "${CYAN}[AuraGo]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
ok() { echo -e "${GREEN}[OK]${NC} $*"; }

is_valid_master_key() {
    printf '%s' "${1:-}" | grep -Eq '^[0-9a-fA-F]{64}$'
}

read_env_value() {
    local env_file="$1"
    local env_key="$2"
    [[ -f "$env_file" ]] || return 1
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

generate_master_key() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 32 2>/dev/null && return 0
    fi
    if command -v python3 >/dev/null 2>&1; then
        python3 -c "import secrets; print(secrets.token_hex(32))" 2>/dev/null && return 0
    fi
    return 1
}

# 1. Check if running as root
if [[ $EUID -ne 0 ]]; then
   error "This script must be run as root (use sudo)."
fi

# 2. Check if AuraGo is already installed
info "Installation directory: ${INSTALL_DIR}"
if [[ ! -f "$BINARY_PATH" ]]; then
    error "AuraGo binary not found at ${BINARY_PATH}. Please run make_deploy.sh first (if building from source) or check your installation."
fi

# Ensure binaries are executable (Windows git push often loses the +x bit)
chmod +x "$BINARY_PATH" "${INSTALL_DIR}/bin/lifeboat_linux" 2>/dev/null || true
ok "Binary permissions verified."

# Grant CAP_NET_BIND_SERVICE so AuraGo can bind ports 80/443 as a non-root user.
# This is required when HTTPS is enabled with standard ports.
if ! command -v setcap >/dev/null 2>&1; then
    info "Installing libcap2-bin for setcap..."
    apt-get install -y libcap2-bin 2>/dev/null || \
    dnf install -y libcap 2>/dev/null || \
    yum install -y libcap 2>/dev/null || \
    warn "setcap not available. After installation run: sudo setcap cap_net_bind_service=+ep ${BINARY_PATH}"
fi
if command -v setcap >/dev/null 2>&1; then
    setcap cap_net_bind_service=+ep "$BINARY_PATH" && \
        ok "CAP_NET_BIND_SERVICE set on binary (allows binding port 443)." || \
        warn "setcap failed on ${BINARY_PATH} — run manually if you need HTTPS on port 443."
fi

if [[ ! -f "$CONFIG_PATH" ]]; then
    warn "config.yaml not found at ${CONFIG_PATH}. Using default might fail."
fi

# 3. Handle Environment Variables (AURAGO_MASTER_KEY)
# Priority: existing /etc/aurago/master.key → local .env → user input → generate
if [[ -f "$CREDENTIAL_FILE" ]] && grep -q "AURAGO_MASTER_KEY" "$CREDENTIAL_FILE"; then
    warn "$CREDENTIAL_FILE already exists — keeping existing key."
    AURAGO_MASTER_KEY="$(read_env_value "$CREDENTIAL_FILE" "AURAGO_MASTER_KEY" || true)"
elif [[ -f "$ENV_FILE" ]]; then
    AURAGO_MASTER_KEY="$(read_env_value "$ENV_FILE" "AURAGO_MASTER_KEY" || true)"
fi

if [[ -z "${AURAGO_MASTER_KEY:-}" ]]; then
    warn "AURAGO_MASTER_KEY not found in ${CREDENTIAL_FILE}, ${ENV_FILE}, or environment."
    read -rp "Enter AURAGO_MASTER_KEY (64 hex characters) or press Enter to generate one: " USER_KEY
    if [[ -z "$USER_KEY" ]]; then
        info "Generating random AURAGO_MASTER_KEY..."
        AURAGO_MASTER_KEY="$(generate_master_key || true)"
        is_valid_master_key "$AURAGO_MASTER_KEY" || error "Failed to generate a secure random key. Please provide one manually."
        ok "Generated new master key."
    else
        AURAGO_MASTER_KEY="$USER_KEY"
        ok "Using user-provided key."
    fi
fi

is_valid_master_key "$AURAGO_MASTER_KEY" || error "AURAGO_MASTER_KEY must be exactly 64 hexadecimal characters."

# 3b. Store the key in /etc/aurago/master.key (root-only)
if ! [[ -f "$CREDENTIAL_FILE" ]] || ! grep -q "AURAGO_MASTER_KEY" "$CREDENTIAL_FILE"; then
    mkdir -p "$CREDENTIAL_DIR"
    chmod 700 "$CREDENTIAL_DIR"
    write_master_key_file "$CREDENTIAL_FILE" "$AURAGO_MASTER_KEY" || error "Failed to write ${CREDENTIAL_FILE} securely."
    chown root:root "$CREDENTIAL_DIR" "$CREDENTIAL_FILE"
    ok "Master key stored at ${CREDENTIAL_FILE} (root-only, mode 0600)."
fi

# Remove the plaintext .env from the install directory (no longer needed)
if [[ -f "$ENV_FILE" ]]; then
    rm -f "$ENV_FILE"
    ok "Removed ${ENV_FILE} — key is now in ${CREDENTIAL_FILE}."
fi

# 4. Create Systemd Service File
info "Creating systemd service file at ${SERVICE_FILE}..."
cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=AuraGo AI Agent
Documentation=https://github.com/antibyte/AuraGo
After=network.target
# Allow unlimited restart attempts — prevents systemd from blocking restarts
# after rapid sequences (e.g. deploy + web-UI restart in quick succession).
StartLimitIntervalSec=0

[Service]
Type=simple
User=$(id -un "${SUDO_USER:-root}")
Group=$(id -gn "${SUDO_USER:-root}")
WorkingDirectory="${INSTALL_DIR}"
ExecStart="${BINARY_PATH}" --config "${CONFIG_PATH}"
Restart=always
RestartSec=5
EnvironmentFile="${CREDENTIAL_FILE}"
StandardOutput=append:${INSTALL_DIR}/log/aurago.log
StandardError=append:${INSTALL_DIR}/log/aurago.err

# Allow binding privileged ports (80, 443) without root.
# Compatible with NoNewPrivileges — systemd sets the capability before the prctl call.
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Security hardening
# NOTE: NoNewPrivileges=true blocks sudo. If you enable agent.sudo_enabled in
# config.yaml (Danger Zone), comment out this line in the service file and run:
#   sudo systemctl daemon-reload && sudo systemctl restart aurago
# NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=${INSTALL_DIR} ${CREDENTIAL_DIR}
ProtectHome=read-only
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

# Ensure log directory exists
mkdir -p "${INSTALL_DIR}/log"
chown -R "${SUDO_USER:-root}:$(id -gn "${SUDO_USER:-root}")" "${INSTALL_DIR}/log"

# 5. Reload systemd and enable service
info "Reloading systemd daemon..."
systemctl daemon-reload

info "Enabling ${SERVICE_NAME} service..."
systemctl enable "${SERVICE_NAME}"

ok "AuraGo service has been installed and enabled."
echo ""
echo -e " ${GREEN}╭──────────────────────────────────────────────────────────────╮${NC}"
echo -e " ${GREEN}│${NC}  ${BOLD}🔐 MASTER KEY SECURED${NC}                                      ${GREEN}│${NC}"
echo -e " ${GREEN}│${NC}  Location: ${BOLD}/etc/aurago/master.key${NC} (root-only, mode 0600)    ${GREEN}│${NC}"
echo -e " ${GREEN}│${NC}  The key is injected into AuraGo via systemd.                ${GREEN}│${NC}"
echo -e " ${GREEN}│${NC}  ${YELLOW}Back up this file! Losing it = losing your vault.${NC}          ${GREEN}│${NC}"
echo -e " ${GREEN}╰──────────────────────────────────────────────────────────────╯${NC}"
echo ""
info "To start the service:   sudo systemctl start ${SERVICE_NAME}"
info "To check status:        sudo systemctl status ${SERVICE_NAME}"
info "To view logs:           tail -f ${INSTALL_DIR}/log/aurago.log"
info "Master key location:    ${CREDENTIAL_FILE}"
