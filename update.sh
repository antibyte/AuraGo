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

# ── Colours ────────────────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info()    { echo -e "${CYAN}[UPDATE]${NC} $*"; }
ok()      { echo -e "${GREEN}[  OK  ]${NC} $*"; }
warn()    { echo -e "${YELLOW}[ WARN ]${NC} $*"; }
die()     { echo -e "${RED}[ERROR ]${NC} $*" >&2; exit 1; }
section() { echo -e "\n${BOLD}${CYAN}━━━  $*  ━━━${NC}"; }

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

if [ ! -f "$DIR/go.mod" ] || ! grep -q "aurago" "$DIR/go.mod" 2>/dev/null; then
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

# ── Verify git repo ────────────────────────────────────────────────────
if [ ! -d "$DIR/.git" ]; then
    die "This directory is not a git repository. Clone from GitHub first:\n  git clone https://github.com/antibyte/AuraGo.git"
fi

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
echo ""
echo -e "${CYAN}╔══════════════════════════════════════╗${NC}"
echo -e "${CYAN}║       AuraGo Updater  v2             ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════╝${NC}"
echo ""
info "Installation: $DIR"
info "Remote:       $(git remote get-url origin 2>/dev/null || echo 'unknown')"
echo ""

# ── Check current vs available version ────────────────────────────────
section "Checking for updates"
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

LOCAL_DATE=$(git log -1 --format='%cd' --date=short)
REMOTE_DATE=$(git log -1 --format='%cd' --date=short origin/main)
AHEAD_COUNT=$(git rev-list HEAD..origin/main --count)

info "Local:  $(git log --format='%h  %s  (%cd)' --date=short -1)"
info "Remote: $(git log --format='%h  %s  (%cd)' --date=short -1 origin/main)"
echo ""
info "$AHEAD_COUNT commit(s) available to pull."
echo ""

# Show changelog
if [ "$AHEAD_COUNT" -gt 0 ]; then
    section "Changelog"
    git log HEAD..origin/main --oneline --no-decorate | head -20
    echo ""
fi

confirm "Proceed with update?" || { info "Update cancelled."; exit 0; }

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

# Backup custom prompt files (only untracked / locally modified ones)
if [ -d "$PROMPTS_DIR" ]; then
    CUSTOM_PROMPTS="$BACKUP_DIR/prompts__custom"
    mkdir -p "$CUSTOM_PROMPTS"
    # git ls-files --others = untracked (user-created); --modified = locally changed
    git -C "$DIR" ls-files --others --modified -- "prompts/" | while read -r fp; do
        rel="${fp#prompts/}"
        dest_dir="$CUSTOM_PROMPTS/$(dirname "$rel")"
        mkdir -p "$dest_dir"
        cp -p "$DIR/$fp" "$dest_dir/"
    done
    CUSTOM_COUNT=$(git -C "$DIR" ls-files --others --modified -- "prompts/" | wc -l)
    ok "Backed up $CUSTOM_COUNT custom/modified prompt file(s)"
fi

# ── Apply update via git ───────────────────────────────────────────────
# Stash any local changes to tracked files (avoids merge conflicts)
STASH_NEEDED=false
if ! git diff --quiet || ! git diff --cached --quiet; then
    warn "Local changes detected in tracked files."
    # Special handling for config.yaml: we always reset it to upstream before pull
    # because we merge it ourselves from backup later.
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
            if ! git stash push --quiet -m "aurago-update-stash-$(date +%s)"; then
                 warn "Stash still failing. Forcing standard backup path..."
            fi
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

# ── Migrate old prompts location (agent_workspace/prompts → prompts/) ─
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

# Re-apply stash if we stashed
if $STASH_NEEDED; then
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

# ── Merge config.yaml (Safety First) ──────────────────────────────────
section "Merging configuration"

USER_CONFIG_BAK="$BACKUP_DIR/config.yaml.user"
# The current config.yaml in $DIR is the new template from GitHub
CURRENT_TEMPLATE="$DIR/config.yaml"

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

# Force restore any tracked binaries that might have been locally deleted or clobbered by a stash pop
git checkout HEAD -- \
    bin/aurago_linux bin/lifeboat_linux bin/config-merger_linux \
    bin/aurago_linux_arm64 bin/lifeboat_linux_arm64 bin/config-merger_linux_arm64 \
    2>/dev/null || true

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
    # ── Pre-built binaries (no Go or < 1.26) ─────────────────────────────
    if command -v go >/dev/null 2>&1; then
        warn "Go ($GO_VERSION) is too old (min $GO_MIN_VERSION) — using pre-built binaries."
    else
        warn "Go is not installed — using pre-built binaries from the repository."
    fi
    info "These are the binaries included in the latest git pull."

    # Pick arch-appropriate pre-built binary name
    PREBUILT_SUFFIX=""
    if [ "$GOARCH" = "arm64" ]; then
        PREBUILT_SUFFIX="_arm64"
    elif [ "$GOARCH" != "amd64" ]; then
        warn "No pre-built binary for $ARCH_RAW. Install Go $GO_MIN_VERSION+ and re-run update.sh."
        warn "https://go.dev/dl/"
    fi

    _use_prebuilt() {
        local base="$1" target="$2"
        local src="$DIR/bin/${base}${PREBUILT_SUFFIX}"
        if [ -f "$src" ]; then
            # Copy to standard name (what systemd and scripts expect)
            [ "$src" != "$DIR/bin/$base" ] && cp -p "$src" "$DIR/bin/$base"
            ok "$base  (pre-built $ARCH_RAW, $(du -sh "$src" 2>/dev/null | cut -f1))"
        else
            echo "$target" ; return 1
        fi
    }

    if ! _use_prebuilt "aurago_linux" "bin/aurago_linux not found after git pull. Cannot continue."; then
        die "bin/aurago_linux${PREBUILT_SUFFIX} not found after git pull. Cannot continue."
    fi
    _use_prebuilt "lifeboat_linux" "" || warn "bin/lifeboat_linux${PREBUILT_SUFFIX} not found — maintenance features may not work."
    _use_prebuilt "config-merger_linux" "" || warn "bin/config-merger_linux${PREBUILT_SUFFIX} not found — config merging disabled."

    if [ -f "$DIR/bin/lifeboat_linux" ]; then
        ok "bin/lifeboat_linux  (pre-built, $(du -sh "$DIR/bin/lifeboat_linux" 2>/dev/null | cut -f1))"
        cp -p "$DIR/bin/lifeboat_linux" "$DIR/bin/lifeboat" 2>/dev/null || true
    else
        warn "bin/lifeboat_linux not found — maintenance features may not work."
    fi
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
echo -e "${GREEN}╔══════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   AuraGo updated successfully! 🚀                ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════╝${NC}"
echo ""
info "Backup of your data kept at: $BACKUP_DIR"
info "To remove backup:            rm -rf $BACKUP_DIR"
info "Version:                     $(git log --format='%h  %s  (%cd)' --date=short -1)"
echo ""
