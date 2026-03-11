#!/usr/bin/env bash
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  AuraGo Update Script (Linux)
#
#  Usage:  ./update.sh [--yes] [--no-restart]
#
#  What it does:
#    1. Fetches the latest commit from GitHub (no clobber of user data)
#    2. Preserves ALL user-specific files:
#         .env, config.yaml, config_debug.yaml,
#         data/*, log/*, agent_workspace/tools/*, agent_workspace/skills/*,
#         agent_workspace/workdir/*, agent_workspace/prompts/* (custom only)
#    3. Applies only code / binary / UI / documentation changes
#    4. Optionally restarts the systemd service or background process
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
set -euo pipefail

# ── UI & Typography ──────────────────────────────────────────────────────
RED='\033[38;5;196m'
YELLOW='\033[38;5;220m'
GREEN='\033[38;5;114m'
CYAN='\033[38;5;86m'
BLUE='\033[38;5;39m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

ICO_INFO="ℹ"
ICO_OK="✔"
ICO_WARN="⚠"
ICO_ERR="✖"

info()    { echo -e "${CYAN}${ICO_INFO} UPDATE${NC} ➜ $*"; }
ok()      { echo -e "${GREEN}${ICO_OK} OK${NC}     ➜ $*"; }
warn()    { echo -e "${YELLOW}${ICO_WARN} WARN${NC}   ➜ $*"; }
die()     { echo -e "${RED}${ICO_ERR} ERROR${NC}  ➜ $*" >&2; exit 1; }
section() { echo -e "\n${BOLD}${BLUE}═══ $* ═══${NC}"; }

# ── CLI flags ──────────────────────────────────────────────────────────
AUTO_YES=false
NO_RESTART=false
for arg in "$@"; do
    case "$arg" in
        --yes)        AUTO_YES=true ;;
        --no-restart) NO_RESTART=true ;;
        --help|-h)
            echo "Usage: $0 [--yes] [--no-restart]"
            echo "  --yes         Skip confirmation prompts"
            echo "  --no-restart  Do not restart the service after update"
            exit 0 ;;
        *) warn "Unknown argument: $arg" ;;
    esac
done

confirm() {
    local msg="$1"
    if $AUTO_YES; then return 0; fi
    read -r -p "$msg [y/N]: " REPLY
    [[ "${REPLY:-n}" =~ ^[Yy]$ ]]
}

# ── Find installation directory ────────────────────────────────────────
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR"

if [ ! -f "$DIR/go.mod" ] && [ ! -f "$DIR/bin/aurago_linux" ]; then
    die "Could not find AuraGo installation at $DIR. Is update.sh in the right place?"
fi

# ── Architecture detection ─────────────────────────────────────────────
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)        GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l|armv6l) GOARCH="armv6l" ;;
    *)             GOARCH="amd64"; warn "Unknown architecture $ARCH_RAW — assuming amd64" ;;
esac
ok "Architecture: $ARCH_RAW → Go target: $GOARCH"

# ── Detect install mode ───────────────────────────────────────────────────
# Binary-only installs (no .git directory) are fully supported.
BINARY_ONLY=false
if [ ! -d "$DIR/.git" ]; then
    BINARY_ONLY=true
fi

GITHUB_REPO="antibyte/AuraGo"
RELEASE_BASE=""  # set in "Checking for updates" for binary mode

# ── Files & directories that must NEVER be touched ─────────────────────
# These are backed up before git operations and restored afterwards.
PROTECTED_FILES=(
    ".env"
    "config.yaml"
    "config_debug.yaml"
)
PROTECTED_DIRS=(
    # Runtime directories like data/, log/, workdir/ are in .gitignore
    # and will be preserved by git pull automatically. No need to move them.
)
# Prompt directories: protect all custom *.md files that are NOT tracked by git
PROMPTS_DIR="$DIR/prompts"

# ── Banner ─────────────────────────────────────────────────────────────
G1='\033[38;5;39m'
G2='\033[38;5;38m'
G3='\033[38;5;37m'
G4='\033[38;5;36m'

echo ""
echo -e " ${G1}╭──────────────────────────────────────╮${NC}"
echo -e " ${G2}│${NC} ${BOLD}✨ AuraGo Updater v2${NC}                   ${G2}│${NC}"
echo -e " ${G3}│${NC} ${DIM}Keeping your AI Agent up to date${NC}       ${G3}│${NC}"
echo -e " ${G4}╰──────────────────────────────────────╯${NC}"
echo ""
info "Installation: $DIR"
if $BINARY_ONLY; then
    info "Mode:         Binary-only (no git)"
else
    info "Remote:       $(git remote get-url origin 2>/dev/null || echo 'unknown')"
fi
echo ""

# ── Check current vs available version ────────────────────────────────
section "Checking for updates"

if $BINARY_ONLY; then
    RELEASE_TAG=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
                  | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
    [ -z "$RELEASE_TAG" ] && die "Could not determine latest release tag from GitHub."
    info "Latest release available: $RELEASE_TAG"
    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    echo ""
    confirm "Proceed with update to $RELEASE_TAG?" || { info "Update cancelled."; exit 0; }
else
    git fetch origin main --quiet

    LOCAL_HASH=$(git rev-parse HEAD)
    REMOTE_HASH=$(git rev-parse origin/main)

    if [ "$LOCAL_HASH" = "$REMOTE_HASH" ]; then
        ok "Already up to date! ($(git log --format='%h %s' -1))"
        echo ""
        if ! confirm "Force update anyway?"; then
            info "Nothing to do."
            exit 0
        fi
    fi

    AHEAD_COUNT=$(git rev-list HEAD..origin/main --count)
    info "Local:  $(git log --format='%h  %s  (%cd)' --date=short -1)"
    info "Remote: $(git log --format='%h  %s  (%cd)' --date=short -1 origin/main)"
    echo ""
    info "$AHEAD_COUNT commit(s) available to pull."
    echo ""

    if [ "$AHEAD_COUNT" -gt 0 ]; then
        section "Changelog"
        git log HEAD..origin/main --oneline --no-decorate | head -20
        echo ""
    fi

    confirm "Proceed with update?" || { info "Update cancelled."; exit 0; }
fi

# ── Stop running instances BEFORE any file changes ────────────────────
# This must happen early so the binary file is not locked and lock files
# are cleaned up before git-pull/build overwrites anything.
section "Stopping running instances"

_kill_proc() {
    local label="$1"; shift
    local patterns=("$@")
    local found=false

    for pat in "${patterns[@]}"; do
        if pgrep -f "$pat" >/dev/null 2>&1; then
            found=true; break
        fi
    done
    $found || { info "$label: not running"; return 0; }

    info "Stopping $label (SIGTERM)..."
    for pat in "${patterns[@]}"; do
        pkill -TERM -f "$pat" 2>/dev/null || true
    done

    # Wait up to 8 seconds for clean exit
    local waited=0
    while true; do
        local still_up=false
        for pat in "${patterns[@]}"; do
            pgrep -f "$pat" >/dev/null 2>&1 && { still_up=true; break; }
        done
        $still_up || break
        sleep 1; waited=$((waited + 1))
        [ $waited -ge 8 ] && break
    done

    # SIGKILL if still alive
    local killed=false
    for pat in "${patterns[@]}"; do
        if pgrep -f "$pat" >/dev/null 2>&1; then
            warn "$label still alive after ${waited}s — sending SIGKILL"
            pkill -KILL -f "$pat" 2>/dev/null || true
            killed=true
        fi
    done

    # Final wait after SIGKILL
    if $killed; then
        sleep 2
        for pat in "${patterns[@]}"; do
            if pgrep -f "$pat" >/dev/null 2>&1; then
                warn "Could not kill $label (pattern: $pat) — update may fail"
            fi
        done
    fi

    ok "$label stopped"
}

# Try to stop via systemd (needs sudo). If sudo fails, fall through to manual kill.
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet aurago 2>/dev/null; then
    info "Stopping aurago systemd service..."
    sudo systemctl stop aurago 2>/dev/null && sleep 2 && ok "systemd service stopped" || warn "sudo not available — falling back to manual kill"
fi
# Always kill any remaining instances (covers manual starts, systemd restarts, etc.)
_kill_proc "aurago"   "bin/aurago_linux" "bin/aurago"
_kill_proc "lifeboat" "bin/lifeboat_linux" "bin/lifeboat"

# ── Remove lock files left by killed processes ─────────────────────────
info "Removing stale lock files..."
for lockfile in \
    "$DIR/data/aurago.lock" \
    "$DIR/data/maintenance.lock" \
    "$DIR/.git/index.lock"
do
    if [ -f "$lockfile" ]; then
        rm -f "$lockfile"
        ok "Removed: $(basename "$lockfile")"
    fi
done

# ── Release any bound ports ────────────────────────────────────────────
for PORT in 8080 8088 8089 8090 8091 8099; do
    if command -v fuser >/dev/null 2>&1; then
        fuser -k ${PORT}/tcp 2>/dev/null || true
    elif command -v lsof >/dev/null 2>&1; then
        lsof -ti tcp:${PORT} | xargs -r kill -9 2>/dev/null || true
    fi
done

ok "All instances stopped, ports released"

# ── Backup protected user data ─────────────────────────────────────────
section "Backing up user data"
BACKUP_DIR="$(mktemp -d /tmp/aurago-backup-XXXXXX)"
info "Backup location: $BACKUP_DIR"

for f in "${PROTECTED_FILES[@]}"; do
    if [ -f "$DIR/$f" ]; then
        cp -p "$DIR/$f" "$BACKUP_DIR/$(basename "$f")"
        ok "Backed up: $f"
    fi
done

for d in "${PROTECTED_DIRS[@]}"; do
    if [ -d "$DIR/$d" ]; then
        local_name="${d//\//__}"      # replace / with __ for flat backup name
        cp -rp "$DIR/$d" "$BACKUP_DIR/$local_name"
        ok "Backed up: $d/"
    fi
done

# Backup custom prompt files
if [ -d "$PROMPTS_DIR" ]; then
    CUSTOM_PROMPTS="$BACKUP_DIR/prompts__custom"
    mkdir -p "$CUSTOM_PROMPTS"
    if $BINARY_ONLY; then
        # Binary install: back up all prompt files (they are always overwritten by update)
        if command -v rsync >/dev/null 2>&1; then
            rsync -a "$PROMPTS_DIR/" "$CUSTOM_PROMPTS/"
        else
            cp -rp "$PROMPTS_DIR/." "$CUSTOM_PROMPTS/"
        fi
        CUSTOM_COUNT=$(find "$PROMPTS_DIR" -type f | wc -l)
    else
        # Git install: back up only untracked/locally modified files
        git -C "$DIR" ls-files --others --modified -- "prompts/" | while read -r fp; do
            rel="${fp#prompts/}"
            dest_dir="$CUSTOM_PROMPTS/$(dirname "$rel")"
            mkdir -p "$dest_dir"
            cp -p "$DIR/$fp" "$dest_dir/"
        done
        CUSTOM_COUNT=$(git -C "$DIR" ls-files --others --modified -- "prompts/" | wc -l)
    fi
    ok "Backed up $CUSTOM_COUNT prompt file(s)"
fi

# ── Apply update ───────────────────────────────────────────────────────
STASH_NEEDED=false
if $BINARY_ONLY; then
    # Binary-only: download resources.dat and extract
    info "Downloading resources.dat ..."
    TMPRES=$(mktemp)
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${RELEASE_BASE}/resources.dat" -o "$TMPRES"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "${RELEASE_BASE}/resources.dat" -O "$TMPRES"
    else
        die "Neither curl nor wget found. Cannot download update."
    fi
    TMPEXT=$(mktemp -d)
    tar -xzf "$TMPRES" -C "$TMPEXT"
    rm -f "$TMPRES"

    # Always overwrite code assets (prompts, ui, agent_workspace)
    [ -d "$TMPEXT/prompts" ]           && cp -a "$TMPEXT/prompts"           "$DIR/"
    [ -d "$TMPEXT/agent_workspace" ]   && cp -a "$TMPEXT/agent_workspace"   "$DIR/"
    [ -d "$TMPEXT/ui" ]                && cp -a "$TMPEXT/ui"                "$DIR/" 2>/dev/null || true

    # Treat the extracted config.yaml as the new template for the merger below
    if [ -f "$TMPEXT/config.yaml" ]; then
        cp "$TMPEXT/config.yaml" "$DIR/config.yaml.new_template"
    fi

    # Update update.sh itself
    if [ -f "$TMPEXT/update.sh" ]; then
        cp "$TMPEXT/update.sh" "$DIR/update.sh"
        chmod +x "$DIR/update.sh"
        ok "update.sh refreshed"
    fi

    rm -rf "$TMPEXT"
    printf '%s' "$RELEASE_TAG" > "$DIR/.version"
    ok "Resources updated from release $RELEASE_TAG"
else
    # Git-based: stash, pull, restore
    if ! git diff --quiet || ! git diff --cached --quiet; then
        warn "Local changes detected in tracked files."
        if git diff --name-only | grep -q "config.yaml"; then
            info "Resetting config.yaml to upstream state before pull (will merge from backup later)..."
            git checkout config.yaml
        fi
        if ! git diff --quiet || ! git diff --cached --quiet; then
            warn "Stashing other changes temporarily..."
            if ! git stash push --quiet -m "aurago-update-stash-$(date +%s)"; then
                warn "Git stash failed (index lock?). Attempting index cleanup..."
                rm -f "$DIR/.git/index.lock" 2>/dev/null || true
                git reset --mixed HEAD || true
                git stash push --quiet -m "aurago-update-stash-$(date +%s)" || warn "Stash still failing."
            fi
            STASH_NEEDED=true
        fi
    fi

    git pull origin main --ff-only || {
        warn "Fast-forward failed. Attempting reset to origin/main..."
        if confirm "Reset local repo to origin/main? (Your user data is backed up and will be restored)"; then
            git reset --hard origin/main
        else
            die "Update aborted."
        fi
    }
    ok "Code updated to $(git log --format='%h  %s' -1)"
    # Write version tag for the Web UI update check
    GIT_VER=$(git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo 'git')
    printf '%s' "$GIT_VER" > "$DIR/.version"
fi

# ── Migrate old prompts location (agent_workspace/prompts → prompts/) ─
# In binary-only mode the custom prompt backup covers all files; re-apply
# it now so user customisations are not wiped by the resources.dat extract.
if $BINARY_ONLY && [ -d "$BACKUP_DIR/prompts__custom" ] && [ "$(ls -A "$BACKUP_DIR/prompts__custom")" ]; then
    if command -v rsync >/dev/null 2>&1; then
        rsync -a --quiet "$BACKUP_DIR/prompts__custom/" "$DIR/prompts/"
    else
        cp -rp "$BACKUP_DIR/prompts__custom/." "$DIR/prompts/"
    fi
    ok "Custom prompt files restored"
fi

OLD_PROMPTS="$DIR/agent_workspace/prompts"
if [ -d "$OLD_PROMPTS" ]; then
    section "Migrating prompts directory"
    info "Old location detected: agent_workspace/prompts/ — migrating custom files ..."
    # Copy any files that don't yet exist at the new location (don't overwrite)
    if command -v rsync >/dev/null 2>&1; then
        rsync -a --ignore-existing "$OLD_PROMPTS/" "$DIR/prompts/"
    else
        find "$OLD_PROMPTS" -type f | while read -r f; do
            rel="${f#$OLD_PROMPTS/}"
            dest="$DIR/prompts/$rel"
            if [ ! -f "$dest" ]; then
                mkdir -p "$(dirname "$dest")"
                cp -p "$f" "$dest"
            fi
        done
    fi
    rm -rf "$OLD_PROMPTS"
    ok "Migrated and removed agent_workspace/prompts/"
fi

# Re-apply stash if we stashed (git mode only)
if ! $BINARY_ONLY && $STASH_NEEDED; then
    info "Re-applying stashed local changes..."
    git stash pop --quiet 2>/dev/null || warn "Could not re-apply stash automatically — check 'git stash list'"
fi

# ── Restore user data ──────────────────────────────────────────────────
section "Restoring user data"

for f in "${PROTECTED_FILES[@]}"; do
    bak="$BACKUP_DIR/$(basename "$f")"
    if [ -f "$bak" ]; then
        if [ "$f" = "config.yaml" ]; then
            # We don't just overwrite config.yaml here anymore.
            # We keep it as is (which is the new template after git pull)
            # and let the merging section below handle it.
            cp -p "$bak" "$BACKUP_DIR/config.yaml.user"
            continue
        fi
        cp -p "$bak" "$DIR/$f"
        ok "Restored: $f"
    fi
done

for d in "${PROTECTED_DIRS[@]}"; do
    local_name="${d//\//__}"
    bak="$BACKUP_DIR/$local_name"
    if [ -d "$bak" ]; then
        # Use rsync if available for smart merge; fall back to cp
        if command -v rsync >/dev/null 2>&1; then
            rsync -a --quiet "$bak/" "$DIR/$d/"
        else
            cp -rp "$bak/." "$DIR/$d/"
        fi
        ok "Restored: $d/"
    fi
done

# Restore custom prompt files
CUSTOM_PROMPTS="$BACKUP_DIR/prompts__custom"
if [ -d "$CUSTOM_PROMPTS" ] && [ "$(ls -A "$CUSTOM_PROMPTS")" ]; then
    if command -v rsync >/dev/null 2>&1; then
        rsync -a --quiet "$CUSTOM_PROMPTS/" "$PROMPTS_DIR/"
    else
        cp -rp "$CUSTOM_PROMPTS/." "$PROMPTS_DIR/"
    fi
    ok "Restored custom prompt files"
fi

ok "All user data preserved."

# ── Offer to migrate .env → /etc/aurago/master.key ─────────────────────
# If .env is still in the install directory, offer to move the key to a
# root-owned credential file outside the application directory.
# This is the same mechanism used by install.sh for new systemd installs.
ENV_FILE="$DIR/.env"
CREDENTIAL_DIR="/etc/aurago"
CREDENTIAL_FILE="${CREDENTIAL_DIR}/master.key"

if [ -f "$ENV_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$ENV_FILE"; then
    # Only offer if not already migrated
    if [ -f "$CREDENTIAL_FILE" ] && grep -q "AURAGO_MASTER_KEY" "$CREDENTIAL_FILE"; then
        info "Master key already exists at $CREDENTIAL_FILE."
        info "Removing leftover $ENV_FILE ..."
        rm -f "$ENV_FILE"
        ok "Removed $ENV_FILE (key is in $CREDENTIAL_FILE)."
    else
        echo ""
        echo -e " ${YELLOW}╭──────────────────────────────────────────────────────────────────╮${NC}"
        echo -e " ${YELLOW}│${NC}  ${BOLD}⚠  SECURITY RECOMMENDATION${NC}                                      ${YELLOW}│${NC}"
        echo -e " ${YELLOW}│${NC}  Your vault master key is stored in ${BOLD}.env${NC} inside the AuraGo     ${YELLOW}│${NC}"
        echo -e " ${YELLOW}│${NC}  directory. This file is readable by your user account.          ${YELLOW}│${NC}"
        echo -e " ${YELLOW}│${NC}                                                                  ${YELLOW}│${NC}"
        echo -e " ${YELLOW}│${NC}  It is ${BOLD}strongly recommended${NC} to move it to a root-protected     ${YELLOW}│${NC}"
        echo -e " ${YELLOW}│${NC}  location at ${BOLD}/etc/aurago/master.key${NC} (mode 0600, root:root).    ${YELLOW}│${NC}"
        echo -e " ${YELLOW}│${NC}  systemd will inject it automatically — no manual sourcing.      ${YELLOW}│${NC}"
        echo -e " ${YELLOW}╰──────────────────────────────────────────────────────────────────╯${NC}"
        echo ""

        if confirm "Move master key to /etc/aurago/master.key? (strongly recommended)"; then
            # shellcheck disable=SC1090
            source "$ENV_FILE"
            if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
                warn "Could not read AURAGO_MASTER_KEY from .env — skipping migration."
            else
                sudo mkdir -p "$CREDENTIAL_DIR"
                sudo chmod 700 "$CREDENTIAL_DIR"
                printf "AURAGO_MASTER_KEY=%s\n" "$AURAGO_MASTER_KEY" | sudo tee "$CREDENTIAL_FILE" > /dev/null
                sudo chmod 600 "$CREDENTIAL_FILE"
                sudo chown root:root "$CREDENTIAL_DIR" "$CREDENTIAL_FILE"
                rm -f "$ENV_FILE"
                ok "Master key moved to $CREDENTIAL_FILE (root-only, mode 0600)."
                ok "Removed $ENV_FILE."

                # Update systemd unit if it exists and still references .env
                SVC_FILE="/etc/systemd/system/aurago.service"
                if [ -f "$SVC_FILE" ]; then
                    if grep -q "EnvironmentFile=.*\.env" "$SVC_FILE" || grep -q "Environment=.*AURAGO_MASTER_KEY" "$SVC_FILE"; then
                        info "Updating systemd unit to use $CREDENTIAL_FILE ..."
                        # Replace EnvironmentFile pointing to .env
                        sudo sed -i "s|EnvironmentFile=.*\.env|EnvironmentFile=${CREDENTIAL_FILE}|g" "$SVC_FILE"
                        # Replace inline Environment= with EnvironmentFile=
                        sudo sed -i "s|Environment=\"AURAGO_MASTER_KEY=.*\"|EnvironmentFile=${CREDENTIAL_FILE}|g" "$SVC_FILE"
                        # Remove dash prefix (fail-silent) if present
                        sudo sed -i "s|EnvironmentFile=-|EnvironmentFile=|g" "$SVC_FILE"
                        # Add security hardening if not already present
                        if ! grep -q "NoNewPrivileges" "$SVC_FILE"; then
                            sudo sed -i "/^\[Install\]/i\\
# Security hardening\\
NoNewPrivileges=true\\
ProtectSystem=strict\\
ReadWritePaths=${DIR} ${CREDENTIAL_DIR}\\
ProtectHome=read-only\\
PrivateTmp=true" "$SVC_FILE"
                        fi
                        sudo systemctl daemon-reload
                        ok "systemd unit updated and reloaded."
                    fi
                fi

                echo ""
                echo -e " ${GREEN}╭──────────────────────────────────────────────────────────────╮${NC}"
                echo -e " ${GREEN}│${NC}  ${BOLD}🔐 MASTER KEY SECURED${NC}                                      ${GREEN}│${NC}"
                echo -e " ${GREEN}│${NC}  Location: ${BOLD}/etc/aurago/master.key${NC} (root-only, mode 0600)    ${GREEN}│${NC}"
                echo -e " ${GREEN}│${NC}  The key is injected into AuraGo via systemd.                ${GREEN}│${NC}"
                echo -e " ${GREEN}│${NC}  ${YELLOW}Back up this file! Losing it = losing your vault.${NC}          ${GREEN}│${NC}"
                echo -e " ${GREEN}╰──────────────────────────────────────────────────────────────╯${NC}"
            fi
        else
            warn "Keeping .env in place. You can migrate later by re-running this update."
        fi
    fi
fi

# ── Merge config.yaml (Safety First) ──────────────────────────────────
section "Merging configuration"

USER_CONFIG_BAK="$BACKUP_DIR/config.yaml.user"
# The current config.yaml in $DIR is the new template from GitHub
CURRENT_TEMPLATE="$DIR/config.yaml"

# In binary-only mode the new template was stored as config.yaml.new_template;
# switch it in so the merger works against it.
if $BINARY_ONLY && [ -f "$DIR/config.yaml.new_template" ]; then
    cp "$DIR/config.yaml.new_template" "$CURRENT_TEMPLATE"
    rm -f "$DIR/config.yaml.new_template"
fi

if [ -f "$USER_CONFIG_BAK" ] && [ -f "$CURRENT_TEMPLATE" ]; then
    # Try multiple binary locations for config-merger
    MERGER_BIN=""
    if [ -f "$DIR/bin/config-merger_linux" ]; then
        MERGER_BIN="$DIR/bin/config-merger_linux"
    elif [ -f "$DIR/bin/config-merger" ]; then
        MERGER_BIN="$DIR/bin/config-merger"
    elif [ -f "$DIR/cmd/config-merger/config-merger" ]; then
        MERGER_BIN="$DIR/cmd/config-merger/config-merger"
    fi

    if [ -n "$MERGER_BIN" ]; then
        info "Running config-merger to integrate your settings..."
        # Merger: source=user_bak, template=new_template -> result saved to new_template (config.yaml)
        if "$MERGER_BIN" -source "$USER_CONFIG_BAK" -template "$CURRENT_TEMPLATE" -output "$CURRENT_TEMPLATE"; then
            ok "Your settings have been merged into the new config.yaml."
        else
            warn "config-merger failed. Restoring your old config.yaml exactly."
            cp -p "$USER_CONFIG_BAK" "$CURRENT_TEMPLATE"
        fi
    else
        warn "config-merger tool not found. Restoring your exact old config.yaml."
        warn "You may be missing new configuration options (budget, webdav, etc)."
        cp -p "$USER_CONFIG_BAK" "$CURRENT_TEMPLATE"
        
        NEW_KEYS=$(comm -23 \
            <(grep -E '^[a-z_]+:' "$CURRENT_TEMPLATE" | sort) \
            <(grep -E '^[a-z_]+:' "$USER_CONFIG_BAK" | sort) 2>/dev/null || true)
        if [ -n "$NEW_KEYS" ]; then
            warn "Please add these missing sections manually if needed:"
            echo "$NEW_KEYS" | while read -r key; do echo "    +  $key"; done
        fi
    fi
fi

# ── Update binary ───────────────────────────────────────────────────────
section "Updating binaries"

# Ensure bin directory exists (e.g. if user manually deleted it)
mkdir -p "$DIR/bin"

# Binaries are now distributed via GitHub Releases (no longer tracked in git)
GITHUB_REPO="antibyte/AuraGo"

# Resolve the latest release tag dynamically
RELEASE_TAG=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -z "$RELEASE_TAG" ]; then
    warn "Could not determine latest release tag — trying 'latest' as fallback."
    RELEASE_TAG="latest"
else
    info "Latest release: $RELEASE_TAG"
fi

_download_release_bin() {
    local name="$1"
    local url="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}/${name}"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$DIR/bin/$name"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$DIR/bin/$name"
    else
        return 1
    fi
}

GO_MIN_VERSION="1.26"
GO_FOUND=false
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    # Simple version comparison for 1.26+
    if [ "$(printf '%s\n%s' "$GO_MIN_VERSION" "$GO_VERSION" | sort -V | head -n1)" = "$GO_MIN_VERSION" ]; then
        GO_FOUND=true
    fi
fi

if $GO_FOUND; then
    # ── Source build (Go 1.26+ available) ────────────────────────────────
    info "Go $GO_VERSION found — building from source..."

    info "Building aurago_linux ($GOARCH)..."
    if CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags='-s -w' -o bin/aurago_linux ./cmd/aurago; then
        ok "bin/aurago_linux built from source"
    else
        warn "Build failed! Falling back to pre-built binary included in the repository."
    fi

    info "Building lifeboat_linux ($GOARCH)..."
    if CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags='-s -w' -o bin/lifeboat_linux ./cmd/lifeboat; then
        ok "bin/lifeboat_linux built from source"
    else
        warn "lifeboat build failed — using pre-built binary."
    fi

    info "Building config-merger_linux ($GOARCH)..."
    if CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags='-s -w' -o bin/config-merger_linux ./cmd/config-merger; then
        ok "bin/config-merger_linux built from source"
    fi
else
    # ── Download binaries from GitHub Releases (no Go or < 1.26) ──────────
    if command -v go >/dev/null 2>&1; then
        warn "Go ($GO_VERSION) is too old (min $GO_MIN_VERSION) — downloading pre-built binaries from GitHub Releases."
    else
        warn "Go is not installed — downloading pre-built binaries from GitHub Releases."
    fi

    # Pick arch-appropriate binary names
    if [ "$GOARCH" = "arm64" ]; then
        BINS=("aurago_linux_arm64" "lifeboat_linux_arm64" "config-merger_linux_arm64")
    else
        BINS=("aurago_linux" "lifeboat_linux" "config-merger_linux")
    fi

    for BIN_NAME in "${BINS[@]}"; do
        info "Downloading $BIN_NAME from GitHub Releases..."
        if _download_release_bin "$BIN_NAME"; then
            ok "$BIN_NAME downloaded."
        else
            warn "$BIN_NAME download failed."
        fi
    done

    # Ensure standard names exist (for arm64 → copy to non-suffixed names)
    if [ "$GOARCH" = "arm64" ]; then
        [ -f "$DIR/bin/aurago_linux_arm64" ]        && cp -p "$DIR/bin/aurago_linux_arm64"        "$DIR/bin/aurago_linux"
        [ -f "$DIR/bin/lifeboat_linux_arm64" ]      && cp -p "$DIR/bin/lifeboat_linux_arm64"      "$DIR/bin/lifeboat_linux"
        [ -f "$DIR/bin/config-merger_linux_arm64" ]  && cp -p "$DIR/bin/config-merger_linux_arm64"  "$DIR/bin/config-merger_linux"
    fi

    [ -f "$DIR/bin/aurago_linux" ] || die "Failed to obtain aurago_linux binary. Cannot continue."

    # Create lifeboat symlink
    [ -f "$DIR/bin/lifeboat_linux" ] && cp -p "$DIR/bin/lifeboat_linux" "$DIR/bin/lifeboat" 2>/dev/null || true
fi

# Ensure all binaries are executable. Try with sudo if needed.
chmod +x "$DIR/bin/"* 2>/dev/null || sudo chmod +x "$DIR/bin/"* 2>/dev/null || true
chmod +x "$DIR/"*.sh 2>/dev/null || sudo chmod +x "$DIR/"*.sh 2>/dev/null || true

# ── Service restart ────────────────────────────────────────────────────
section "Restart"

if $NO_RESTART; then
    warn "Skipping restart (--no-restart flag set). Start manually:"
    echo "   sudo systemctl restart aurago   OR   ./start.sh"
elif command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files aurago.service >/dev/null 2>&1; then
    info "Starting aurago systemd service..."
    # Try sudo first; if it fails (no password), fall back to nohup start
    if sudo systemctl start aurago 2>/dev/null; then
        sleep 2
        if systemctl is-active --quiet aurago 2>/dev/null; then
            ok "Service started successfully via systemd."
        else
            warn "systemd start reported OK but service not active — check: sudo journalctl -u aurago -n 50"
        fi
    else
        warn "sudo not available — starting aurago directly (systemd will adopt on next boot)"
        LAUNCH_BIN="$DIR/bin/aurago_linux"
        [ ! -f "$LAUNCH_BIN" ] && LAUNCH_BIN="$DIR/bin/aurago"
        if [ -f "$LAUNCH_BIN" ]; then
            mkdir -p "$DIR/log"
            nohup "$LAUNCH_BIN" --config "$DIR/config.yaml" >>"${DIR}/log/aurago.log" 2>&1 &
            LAUNCH_PID=$!
            info "AuraGo starting (PID=$LAUNCH_PID)..."
            sleep 3
            if kill -0 "$LAUNCH_PID" 2>/dev/null; then
                ok "AuraGo running (PID=$LAUNCH_PID). Logs: ${DIR}/log/aurago.log"
            else
                warn "AuraGo may have exited immediately — check: tail -n 50 ${DIR}/log/aurago.log"
            fi
        else
            warn "No aurago binary found — start manually with: ./start.sh"
        fi
    fi
else
    LAUNCH_BIN="$DIR/bin/aurago_linux"
    [ ! -f "$LAUNCH_BIN" ] && LAUNCH_BIN="$DIR/bin/aurago"
    if [ ! -f "$LAUNCH_BIN" ]; then
        warn "No aurago binary found — start manually with: ./start.sh"
    else
        mkdir -p "$DIR/log"
        nohup "$LAUNCH_BIN" --config "$DIR/config.yaml" >>"${DIR}/log/aurago.log" 2>&1 &
        LAUNCH_PID=$!
        info "AuraGo starting (PID=$LAUNCH_PID)..."
        sleep 3
        if kill -0 "$LAUNCH_PID" 2>/dev/null; then
            ok "AuraGo running (PID=$LAUNCH_PID). Logs: ${DIR}/log/aurago.log"
        else
            warn "AuraGo may have exited immediately — check: tail -n 50 ${DIR}/log/aurago.log"
        fi
    fi
fi

# ── Summary ────────────────────────────────────────────────────────────
echo ""
echo -e " ${GREEN}╭──────────────────────────────────────────────────╮${NC}"
echo -e " ${GREEN}│${NC}   ${BOLD}AuraGo updated successfully! 🚀${NC}                ${GREEN}│${NC}"
echo -e " ${GREEN}╰──────────────────────────────────────────────────╯${NC}"
echo ""
info "Backup of your data kept at: $BACKUP_DIR"
info "To remove backup:            rm -rf $BACKUP_DIR"
if $BINARY_ONLY; then
    info "Version:                     $RELEASE_TAG"
else
    info "Version:                     $(git log --format='%h  %s  (%cd)' --date=short -1)"
fi
echo ""
