#!/usr/bin/env bash
# AuraGo Systemd Service Installer (Linux)
# This script sets up AuraGo as a system-wide service.

set -euo pipefail

# Configuration
SERVICE_NAME="aurago"
INSTALL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
BINARY_PATH="${INSTALL_DIR}/bin/aurago_linux"
CONFIG_PATH="${INSTALL_DIR}/config.yaml"
ENV_FILE="${INSTALL_DIR}/.env"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info() { echo -e "${CYAN}[AuraGo]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
ok() { echo -e "${GREEN}[OK]${NC} $*"; }

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

if [[ ! -f "$CONFIG_PATH" ]]; then
    warn "config.yaml not found at ${CONFIG_PATH}. Using default might fail."
fi

# 3. Handle Environment Variables (AURAGO_MASTER_KEY)
if [[ -f "$ENV_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$ENV_FILE"
fi

if [[ -z "${AURAGO_MASTER_KEY:-}" ]]; then
    warn "AURAGO_MASTER_KEY not found in ${ENV_FILE} or environment."
    read -rp "Enter AURAGO_MASTER_KEY (64 hex characters) or press Enter to generate one: " USER_KEY
    if [[ -z "$USER_KEY" ]]; then
        info "Generating random AURAGO_MASTER_KEY..."
        AURAGO_MASTER_KEY=$(openssl rand -hex 32 2>/dev/null || python3 -c "import secrets; print(secrets.token_hex(32))" 2>/dev/null || echo "failed")
        if [[ "$AURAGO_MASTER_KEY" == "failed" ]]; then
            error "Failed to generate a secure random key. Please provide one manually."
        fi
        echo "AURAGO_MASTER_KEY=${AURAGO_MASTER_KEY}" >> "$ENV_FILE"
        chmod 600 "$ENV_FILE"
        ok "Generated key saved to ${ENV_FILE}"
    else
        AURAGO_MASTER_KEY="$USER_KEY"
        echo "AURAGO_MASTER_KEY=${AURAGO_MASTER_KEY}" >> "$ENV_FILE"
        chmod 600 "$ENV_FILE"
        ok "Entered key saved to ${ENV_FILE}"
    fi
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
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BINARY_PATH} --config ${CONFIG_PATH}
Restart=always
RestartSec=5
Environment="AURAGO_MASTER_KEY=${AURAGO_MASTER_KEY}"
StandardOutput=append:${INSTALL_DIR}/log/aurago.log
StandardError=append:${INSTALL_DIR}/log/aurago.err

# Security hardening
# NoNewPrivileges=true
# ProtectSystem=full
# ProtectHome=read-only

[Install]
WantedBy=multi-user.target
EOF

# Ensure log directory exists
mkdir -p "${INSTALL_DIR}/log"
chown -R "${SUDO_USER:-root}:${SUDO_USER:-root}" "${INSTALL_DIR}/log"

# 5. Reload systemd and enable service
info "Reloading systemd daemon..."
systemctl daemon-reload

info "Enabling ${SERVICE_NAME} service..."
systemctl enable "${SERVICE_NAME}"

ok "AuraGo service has been installed and enabled."
info "To start the service:   sudo systemctl start ${SERVICE_NAME}"
info "To check status:        sudo systemctl status ${SERVICE_NAME}"
info "To view logs:           tail -f ${INSTALL_DIR}/log/aurago.log"
