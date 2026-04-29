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

ICO_INFO="*"
ICO_OK="OK"
ICO_WARN="!!"
ICO_ERR="ERR"

info()    { echo -e "${CYAN}${ICO_INFO} UPDATE${NC} -> $*"; }
ok()      { echo -e "${GREEN}${ICO_OK}${NC}        -> $*"; }
warn()    { echo -e "${YELLOW}${ICO_WARN} WARN${NC}  -> $*"; }
die()     { echo -e "${RED}${ICO_ERR} ERROR${NC} -> $*" >&2; exit 1; }
section() { echo -e "\n${BOLD}${BLUE}--- $* ---${NC}"; }

# ── CLI flags ──────────────────────────────────────────────────────────
AUTO_YES=false
NO_RESTART=false
_AU_ESCAPED=""
for arg in "$@"; do
    case "$arg" in
        --yes)        AUTO_YES=true ;;
        --no-restart) NO_RESTART=true ;;
        --escaped)    _AU_ESCAPED=1 ;;   # internal: already running in an independent scope
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
    printf '%s [y/N]: ' "$msg" >/dev/tty
    read -r REPLY </dev/tty
    [[ "${REPLY:-n}" =~ ^[Yy]$ ]]
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

ensure_private_update_runtime_dir() {
    local dir="/tmp/aurago-update-$(id -u)"
    if [ -e "$dir" ] && [ ! -d "$dir" ]; then
        die "Unsafe update runtime path exists and is not a directory: $dir"
    fi
    mkdir -p "$dir"
    chmod 700 "$dir" 2>/dev/null || true
    if [ -L "$dir" ]; then
        die "Unsafe update runtime path is a symlink: $dir"
    fi
    printf '%s\n' "$dir"
}

remove_regular_file_if_present() {
    local path="$1"
    if [ -L "$path" ]; then
        warn "Refusing to remove symlink lock file: $path"
        return 1
    fi
    if [ -f "$path" ]; then
        rm -f -- "$path"
        return 0
    fi
    return 1
}

# ── Find installation directory ────────────────────────────────────────
# _AU_ORIG_DIR is exported when re-execing from a temp copy (see below).
# In that case BASH_SOURCE[0] points to /tmp/... so we must use the saved path.
if [ -n "${_AU_ORIG_DIR:-}" ]; then
    DIR="$_AU_ORIG_DIR"
else
    DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fi
cd "$DIR"

if [ ! -f "$DIR/go.mod" ] && [ ! -f "$DIR/bin/aurago_linux" ]; then
    die "Could not find AuraGo installation at $DIR. Is update.sh in the right place?"
fi

# ── Single-instance guard ──────────────────────────────────────────────
# Prevents re-entrant execution caused by:
#   • bash lazy re-reads of a script replaced on disk by git pull
#   • git hooks or other subprocesses that inherit the environment
# Any invocation that finds this lock and the owning process alive exits silently.
_AU_RUNTIME_DIR="$(ensure_private_update_runtime_dir)"
_AU_LOCK="${_AU_RUNTIME_DIR}/update.lock"
if [ -f "$_AU_LOCK" ]; then
    _AU_LOCK_PID=$(cat "$_AU_LOCK" 2>/dev/null || echo 0)
    if [ "${_AU_LOCK_PID:-0}" -gt 0 ] && kill -0 "$_AU_LOCK_PID" 2>/dev/null; then
        exit 0  # Another update is already running — silently bail
    fi
    remove_regular_file_if_present "$_AU_LOCK" >/dev/null || true  # Stale lock from a dead process
fi

# ── Architecture detection ─────────────────────────────────────────────
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64)        GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    armv7l)        GOARCH="arm"; GOARM="7" ;;
    armv6l)        GOARCH="arm"; GOARM="6" ;;
    *)             GOARCH="amd64"; warn "Unknown architecture $ARCH_RAW — assuming amd64" ;;
esac
ok "Architecture: $ARCH_RAW → Go target: $GOARCH"

# ── Sudo strategy ─────────────────────────────────────────────────────
# When no TTY is attached (triggered from web UI / nohup), use sudo -n
# so the command fails immediately instead of hanging on a password prompt.
# When a TTY is available (manual terminal run), use plain sudo so the
# user can enter a password interactively.
if [ -t 0 ]; then
    SUDO="sudo"
else
    SUDO="sudo -n"
fi

# ── Detect install mode ───────────────────────────────────────────────────
# Binary-only installs (no .git directory) are fully supported.
BINARY_ONLY=false
if [ ! -d "$DIR/.git" ]; then
    BINARY_ONLY=true
fi

GITHUB_REPO="antibyte/AuraGo"
RELEASE_BASE=""  # set in "Checking for updates" for binary mode

fetch_url_to_file() {
    local url="$1"
    local out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$out"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$out"
    else
        return 1
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
    RELEASE_CHECKSUMS_FILE="$(mktemp "/tmp/aurago-sha256.XXXXXX")"
    if ! fetch_url_to_file "${RELEASE_BASE}/SHA256SUMS" "$RELEASE_CHECKSUMS_FILE"; then
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
    local url="${RELEASE_BASE}/${asset}"
    fetch_url_to_file "$url" "$dest"
    verify_release_asset "$asset" "$dest"
}

fetch_url_stdout() {
    local url="$1"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url"
    else
        return 1
    fi
}

latest_release_tag() {
    fetch_url_stdout "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" \
        | grep -o '"tag_name": *"[^"]*"' \
        | head -1 \
        | cut -d'"' -f4
}

read_master_key_from_env() {
    local env_file="$1"
    local raw
    raw=$(grep -E '^AURAGO_MASTER_KEY=' "$env_file" | head -1 || true)
    raw="${raw#AURAGO_MASTER_KEY=}"
    raw="${raw%$'\r'}"
    # Remove surrounding quotes if present
    if [[ "$raw" == \"*\" ]]; then
        raw="${raw:1:-1}"
    elif [[ "$raw" == \'*\' ]]; then
        raw="${raw:1:-1}"
    fi
    printf '%s' "$raw"
}

safe_restore_file() {
    local src="$1"
    local dst="$2"

    [ -f "$src" ] || return 0

    # First try a normal forced overwrite with metadata preserved.
    if cp -fp "$src" "$dst" 2>/dev/null; then
        return 0
    fi

    # If destination exists but is not writable, remove then copy.
    if rm -f "$dst" 2>/dev/null && cp -fp "$src" "$dst" 2>/dev/null; then
        return 0
    fi

    # Last resort: sudo copy (for legacy root-owned files in user installs).
    if command -v sudo >/dev/null 2>&1; then
        if $SUDO cp -fp "$src" "$dst" 2>/dev/null; then
            $SUDO chown "$(id -u):$(id -g)" "$dst" 2>/dev/null || true
            return 0
        fi
    fi

    return 1
}

copy_tree_merge() {
    local src="$1"
    local dst="$2"

    # Ensure destination exists and is writable by the current user when possible.
    mkdir -p "$dst" 2>/dev/null || true
    if [ ! -w "$dst" ] && command -v sudo >/dev/null 2>&1; then
        $SUDO chown -R "$(id -u):$(id -g)" "$dst" 2>/dev/null || true
        $SUDO chmod -R u+rwX "$dst" 2>/dev/null || true
    fi

    if command -v rsync >/dev/null 2>&1; then
        # Avoid owner/group preservation to prevent non-fatal permission errors
        # on systems where destination files may be root-owned.
        # Also avoid timestamp preservation (-t) to prevent "failed to set times" warnings.
        rsync -rl --omit-dir-times --quiet --no-owner --no-group "$src" "$dst"
    else
        cp -r "$src" "$dst"
    fi
}

repair_worktree_permissions() {
    # Make changed tracked files and parent directories writable so git can
    # overwrite/unlink them during update.
    local changed
    changed="$(git -C "$DIR" status --porcelain --untracked-files=no 2>/dev/null | awk '{print $2}')"
    [ -n "$changed" ] || return 0

    while IFS= read -r rel; do
        [ -n "$rel" ] || continue
        local abs="$DIR/$rel"
        local parent
        parent="$(dirname "$abs")"

        if [ -e "$abs" ]; then
            chmod u+rw "$abs" 2>/dev/null || true
            if [ ! -w "$abs" ] && command -v sudo >/dev/null 2>&1; then
                $SUDO chown "$(id -u):$(id -g)" "$abs" 2>/dev/null || true
                $SUDO chmod u+rw "$abs" 2>/dev/null || true
            fi
        fi

        chmod u+rwx "$parent" 2>/dev/null || true
        if [ ! -w "$parent" ] && command -v sudo >/dev/null 2>&1; then
            $SUDO chown "$(id -u):$(id -g)" "$parent" 2>/dev/null || true
            $SUDO chmod u+rwx "$parent" 2>/dev/null || true
        fi
    done <<< "$changed"
}

clean_tracked_changes() {
    # Reset only tracked changes; user data/custom files are restored from backup.
    repair_worktree_permissions

    git -C "$DIR" restore --source=HEAD --staged --worktree . 2>/dev/null || true
    git -C "$DIR" checkout -- . 2>/dev/null || true
    git -C "$DIR" reset --quiet HEAD 2>/dev/null || true

    # Return success if tracked changes are gone.
    git -C "$DIR" diff --quiet && git -C "$DIR" diff --cached --quiet
}

# ── Files & directories that must NEVER be touched ─────────────────────
# These are backed up before git operations and restored afterwards.
PROTECTED_FILES=(
    ".env"
    "config.yaml"
    "config_debug.yaml"
)
# Directories to back up fully (must be small — they go to /tmp).
# data/vectordb, data/tts, data/vectordb_backup are intentionally excluded:
# they are gitignored (git never touches them) and can be very large.
# agent_workspace/workdir and agent_workspace/github are also excluded
# (ephemeral working state, gitignored, safe).
PROTECTED_DIRS=(
    "agent_workspace/tools"
    "agent_workspace/skills"
)
# Critical data files backed up individually (avoids copying large binary dirs)
DATA_FILES=(
    "data/character_journal.md"
    "data/chat_history.json"
    "data/crontab.json"
    "data/current_plan.md"
    "data/graph.json"
    "data/state.json"
    "data/media_registry.db"
    "data/homepage_registry.db"
    "data/cheatsheets.db"
    "data/inventory.db"
    "data/contacts.db"
    "data/knowledge_graph.db"
    "data/skills.db"
    "data/invasion.db"
    "data/image_gallery.db"
    "data/push.db"
    "data/remote_control.db"
    "data/sql_connections.db"
    "data/short_term.db"
)
# Prompt directories: protect all custom *.md files that are NOT tracked by git
PROMPTS_DIR="$DIR/prompts"

# Escape to a separate systemd scope only when actually running inside
# the aurago service cgroup. Manual shell runs do not need this path.
IN_AURAGO_CGROUP=false
if [ -r "/proc/$$/cgroup" ] && grep -qE 'aurago\.service' "/proc/$$/cgroup"; then
    IN_AURAGO_CGROUP=true
fi

# ── Escape systemd service cgroup ─────────────────────────────────────
# When triggered from the AuraGo web UI, this script runs inside the
# aurago systemd service cgroup.  By default (KillMode=control-group),
# systemd sends SIGTERM to *all* processes in that cgroup — including
# this script — the moment aurago's main process is stopped below.
# To survive that cleanup we try to re-exec ourselves in an independent
# transient scope before we touch any processes.
if $IN_AURAGO_CGROUP && [ -z "${_AU_ESCAPED:-}" ]; then
    if command -v systemd-run >/dev/null 2>&1; then
        # Prefer a user scope (no root required, needs active user session).
        # Pass --escaped as a CLI argument — this is 100% reliable regardless
        # of environment variable inheritance or file replacement mid-execution.
        # env-variable guards (export _AU_ESCAPED=1) can be lost when
        # systemd-run --scope uses the logind session environment instead of
        # the calling process's exported vars, or when git stash pop replaces
        # the running script on disk and bash re-reads the new content.
        if systemd-run --user --scope --quiet -- /bin/bash "$0" "--escaped" "$@" 2>/dev/null; then
            exit 0
        fi
        # Fall back to a system scope via sudo (password-less sudo only).
        if command -v sudo >/dev/null 2>&1; then
            if $SUDO systemd-run --scope --quiet -- /bin/bash "$0" "--escaped" "$@" 2>/dev/null; then
                exit 0
            fi
        fi
    fi
    # No escape possible — continue in the same cgroup.
    # Non-systemd installs are unaffected; systemd installs without sudo
    # may be interrupted by cgroup cleanup.  Use `sudo systemctl stop
    # aurago` + `sudo /path/to/update.sh --yes` for a guaranteed update.
fi

# ── Copy to temp to prevent mid-run file replacement ─────────────────
# bash reads scripts lazily in chunks from disk. git pull replaces this
# file during execution; subsequent reads start at the wrong byte offset
# in the new version, causing re-execution from near the top of the file.
# Running from a temp copy ensures git pull cannot affect our execution.
if [ -z "${_AU_TMPRUN:-}" ]; then
    _TMPS=$(mktemp "${_AU_RUNTIME_DIR}/script.XXXXXX")
    cp -- "$0" "$_TMPS"
    chmod +x "$_TMPS"
    export _AU_TMPRUN=1
    export _AU_ORIG_DIR="$DIR"
    exec /bin/bash "$_TMPS" "$@"
fi
# Running from temp copy: claim the single-instance lock and schedule cleanup.
_AU_RUNTIME_DIR="$(ensure_private_update_runtime_dir)"
_AU_LOCK="${_AU_RUNTIME_DIR}/update.lock"
echo $$ > "$_AU_LOCK"
trap 'remove_regular_file_if_present "$_AU_LOCK" >/dev/null || true; remove_regular_file_if_present "${BASH_SOURCE[0]}" >/dev/null || true; [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && remove_regular_file_if_present "$RELEASE_CHECKSUMS_FILE" >/dev/null || true' EXIT

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

GIT_UP_TO_DATE=false   # set to true when local git is already at latest commit

if $BINARY_ONLY; then
    RELEASE_TAG=$(latest_release_tag || true)
    [ -z "$RELEASE_TAG" ] && die "Could not determine latest release tag from GitHub."
    info "Latest release available: $RELEASE_TAG"
    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    fetch_release_checksums || die "Could not download SHA256SUMS for release ${RELEASE_TAG}."
    echo ""
    confirm "Proceed with update to $RELEASE_TAG?" || { info "Update cancelled."; exit 0; }
else
    git fetch origin main --quiet

    LOCAL_HASH=$(git rev-parse HEAD)
    REMOTE_HASH=$(git rev-parse origin/main)
    GIT_UP_TO_DATE=false

    if [ "$LOCAL_HASH" = "$REMOTE_HASH" ]; then
        ok "Code is already at the latest version ($(git log --format='%h %s' -1))"
        GIT_UP_TO_DATE=true
    else
        AHEAD_COUNT=$(git rev-list HEAD..origin/main --count)
        info "Local:  $(git log --format='%h  %s  (%cd)' --date=short -1)"
        info "Remote: $(git log --format='%h  %s  (%cd)' --date=short -1 origin/main)"
        echo ""
        info "$AHEAD_COUNT commit(s) available to pull."
        echo ""

        if [ "$AHEAD_COUNT" -gt 0 ]; then
            section "Changelog"
            git log HEAD..origin/main --oneline --no-decorate -n 20
            echo ""
        fi
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
    local pids=""
    local pat pid exe cwd cmd expected

    # Only stop processes that belong to this AuraGo installation. Matching on
    # broad names such as "bin/aurago_linux" can otherwise kill unrelated test
    # instances in other directories.
    for pat in "${patterns[@]}"; do
        expected="$DIR/$pat"
        while IFS= read -r pid; do
            [ -n "$pid" ] || continue
            [ "$pid" = "$$" ] && continue

            exe="$(readlink -f "/proc/$pid/exe" 2>/dev/null || true)"
            cwd="$(readlink -f "/proc/$pid/cwd" 2>/dev/null || true)"
            cmd="$(tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null || true)"

            if [ "$exe" = "$expected" ] ||
               [[ "$cmd" == *"$expected"* ]] ||
               { [[ "$cwd" == "$DIR" || "$cwd" == "$DIR"/* ]] && [[ "$cmd" == *"$pat"* ]]; }; then
                case " $pids " in
                    *" $pid "*) ;;
                    *) pids="$pids $pid" ;;
                esac
            fi
        done < <(pgrep -f "$pat" 2>/dev/null || true)
    done

    [ -n "${pids// /}" ] || { info "$label: not running"; return 0; }

    info "Stopping $label (SIGTERM)..."
    for pid in $pids; do
        kill -TERM "$pid" 2>/dev/null || true
    done

    # Wait up to 8 seconds for clean exit
    local waited=0
    while true; do
        local still_up=false
        for pid in $pids; do
            kill -0 "$pid" 2>/dev/null && { still_up=true; break; }
        done
        $still_up || break
        sleep 1; waited=$((waited + 1))
        [ $waited -ge 8 ] && break
    done

    # SIGKILL if still alive
    local killed=false
    for pid in $pids; do
        if kill -0 "$pid" 2>/dev/null; then
            warn "$label still alive after ${waited}s — sending SIGKILL"
            kill -KILL "$pid" 2>/dev/null || true
            killed=true
        fi
    done

    # Final wait after SIGKILL
    if $killed; then
        sleep 2
        for pid in $pids; do
            if kill -0 "$pid" 2>/dev/null; then
                warn "Could not kill $label process $pid — update may fail"
            fi
        done
    fi

    ok "$label stopped"
}

# Try to stop via systemd (needs sudo). If sudo fails, fall through to manual kill.
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet aurago 2>/dev/null; then
    info "Stopping aurago systemd service..."
    $SUDO systemctl stop aurago 2>/dev/null && sleep 2 && ok "systemd service stopped" || warn "sudo not available — falling back to manual kill"
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
        if remove_regular_file_if_present "$lockfile"; then
            ok "Removed: $(basename "$lockfile")"
        fi
    fi
done

ok "AuraGo-owned instances stopped"

# ── Backup protected user data ─────────────────────────────────────────
section "Backing up user data"
BACKUP_DIR="$(mktemp -d /tmp/aurago-backup-XXXXXX)"
info "Backup location: $BACKUP_DIR"

backup_current_aurago_binary() {
    mkdir -p "$BACKUP_DIR/bin"
    for _bin in aurago_linux aurago; do
        if [ -f "$DIR/bin/$_bin" ]; then
            cp -p "$DIR/bin/$_bin" "$BACKUP_DIR/bin/$_bin"
            ok "Backed up binary: bin/$_bin"
        fi
    done
}

restore_previous_aurago_binary() {
    local restored=false
    for _bin in aurago_linux aurago; do
        if [ -f "$BACKUP_DIR/bin/$_bin" ]; then
            cp -p "$BACKUP_DIR/bin/$_bin" "$DIR/bin/$_bin"
            mark_executable_if_present "$DIR/bin/$_bin"
            restored=true
        fi
    done
    if $restored; then
        apply_aurago_setcap_if_available
        warn "Restored previous AuraGo binary after failed restart."
        return 0
    fi
    warn "No previous AuraGo binary was available for rollback."
    return 1
}

backup_current_aurago_binary

for f in "${PROTECTED_FILES[@]}"; do
    if [ -f "$DIR/$f" ]; then
        cp -p "$DIR/$f" "$BACKUP_DIR/$(basename "$f")"
        ok "Backed up: $f"
    fi
done

# If config.yaml is missing (e.g. re-execution after git deleted it during a
# tracked→untracked transition), recover it from the most recent prior backup.
if [ ! -f "$BACKUP_DIR/config.yaml" ]; then
    _prev_cfg=$(find /tmp -maxdepth 2 -name "config.yaml" \
        -path "*/aurago-backup-*" ! -path "$BACKUP_DIR/*" \
        2>/dev/null | xargs ls -t 2>/dev/null | head -1)
    if [ -n "$_prev_cfg" ]; then
        cp -p "$_prev_cfg" "$BACKUP_DIR/config.yaml"
        ok "Recovered config.yaml from previous backup (re-execution safety net)."
    fi
fi

# Back up individual critical data files
mkdir -p "$BACKUP_DIR/data"
for f in "${DATA_FILES[@]}"; do
    if [ -f "$DIR/$f" ]; then
        if ! safe_restore_file "$DIR/$f" "$BACKUP_DIR/data/$(basename "$f")"; then
            warn "Could not back up $f (permission denied — try running with sudo)"
        fi
    fi
done
ok "Backed up: data/ (critical files)"

for d in "${PROTECTED_DIRS[@]}"; do
    if [ -d "$DIR/$d" ]; then
        local_name="${d//\//__}"      # replace / with __ for flat backup name
        copy_tree_merge "$DIR/$d/" "$BACKUP_DIR/$local_name/" || warn "Could not fully back up $d/."
        ok "Backed up: $d/"
    fi
done

# Backup custom prompt files
if [ -d "$PROMPTS_DIR" ]; then
    CUSTOM_PROMPTS="$BACKUP_DIR/prompts__custom"
    mkdir -p "$CUSTOM_PROMPTS"
    if $BINARY_ONLY; then
        # Binary install: back up all prompt files (they are always overwritten by update)
        copy_tree_merge "$PROMPTS_DIR/" "$CUSTOM_PROMPTS/" || warn "Could not fully back up prompts/."
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
if $BINARY_ONLY; then
    # Binary-only: download resources.dat and extract
    info "Downloading resources.dat ..."
    TMPRES=$(mktemp)
    if ! download_release_asset "resources.dat" "$TMPRES"; then
        die "Failed to download or verify resources.dat from the release."
    fi
    TMPEXT=$(mktemp -d)
    tar -xzf "$TMPRES" -C "$TMPEXT"
    rm -f "$TMPRES"

    # Always overwrite code assets (prompts, ui, agent_workspace).
    # Use -r (no -p) so we don't try to preserve timestamps/ownership from the
    # tar archive — non-root users cannot change timestamps on files they don't
    # own, which produces spurious "Operation not permitted" warnings with cp -a.
    [ -d "$TMPEXT/prompts" ]           && cp -r "$TMPEXT/prompts"           "$DIR/"
    [ -d "$TMPEXT/agent_workspace" ]   && cp -r "$TMPEXT/agent_workspace"   "$DIR/"
    [ -d "$TMPEXT/ui" ]                && cp -r "$TMPEXT/ui"                "$DIR/" 2>/dev/null || true

    # Treat the extracted config.yaml as the new template for the merger below
    if [ -f "$TMPEXT/config.yaml" ]; then
        cp "$TMPEXT/config.yaml" "$DIR/config.yaml.new_template"
    fi

    if download_release_asset "update.sh" "$DIR/update.sh"; then
        chmod +x "$DIR/update.sh"
        ok "update.sh refreshed"
    else
        warn "Could not refresh update.sh from verified release asset."
    fi

    rm -rf "$TMPEXT"
    printf '%s' "$RELEASE_TAG" > "$DIR/.version"
    ok "Resources updated from release $RELEASE_TAG"
else
    # Git-based update.
    if ! $GIT_UP_TO_DATE; then
        if ! git diff --quiet || ! git diff --cached --quiet; then
            info "Cleaning local tracked changes before update..."
            if ! clean_tracked_changes; then
                warn "Automatic cleanup of tracked changes failed."
                warn "Changed files still present:"
                git -C "$DIR" status --porcelain --untracked-files=no | head -20 || true
                die "Cannot continue update while tracked files are locked/unwritable. Fix permissions or run with sudo."
            fi
        fi

        if ! git fetch origin main --quiet; then
            die "Failed to fetch updates from GitHub (network/connectivity issue)."
        fi

        if ! git merge --ff-only origin/main; then
            warn "Fast-forward merge failed — retrying after tracked-change cleanup..."
            clean_tracked_changes || true
            if ! git merge --ff-only origin/main; then
                # Check if branches have diverged (force-push scenario)
                LOCAL=$(git rev-parse HEAD)
                REMOTE=$(git rev-parse origin/main)
                BASE=$(git merge-base HEAD origin/main)
                if [ "$LOCAL" != "$BASE" ] && [ "$REMOTE" != "$BASE" ]; then
                    warn "Branches have diverged (force-push detected). Performing hard reset..."
                    git reset --hard origin/main
                    ok "Hard reset complete."
                else
                    warn "Could not fast-forward automatically."
                    warn "Please ensure repository files are writable and no manual merge is required."
                    die "Update aborted safely (no hard reset performed)."
                fi
            fi
        fi
        ok "Code updated to $(git log --format='%h  %s' -1)"
        GIT_VER=$(git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo 'git')
        printf '%s' "$GIT_VER" > "$DIR/.version"
    fi

    # Restore user's config.yaml — git must never win over user's config.
    if [ -f "$BACKUP_DIR/config.yaml" ]; then
        safe_restore_file "$BACKUP_DIR/config.yaml" "$DIR/config.yaml" \
            || die "Could not restore config.yaml (permission denied)."
    fi
fi

# ── Migrate old prompts location (agent_workspace/prompts → prompts/) ─
# In binary-only mode the custom prompt backup covers all files; re-apply
# it now so user customisations are not wiped by the resources.dat extract.
if $BINARY_ONLY && [ -d "$BACKUP_DIR/prompts__custom" ] && [ "$(ls -A "$BACKUP_DIR/prompts__custom")" ]; then
    copy_tree_merge "$BACKUP_DIR/prompts__custom/" "$DIR/prompts/" || warn "Could not fully restore custom prompt files."
    ok "Custom prompt files restored"
fi

OLD_PROMPTS="$DIR/agent_workspace/prompts"
if [ -d "$OLD_PROMPTS" ]; then
    section "Migrating prompts directory"
    info "Old location detected: agent_workspace/prompts/ — migrating custom files ..."
    # Copy any files that don't yet exist at the new location (don't overwrite)
    if command -v rsync >/dev/null 2>&1; then
        rsync -rl --quiet --no-owner --no-group --ignore-existing "$OLD_PROMPTS/" "$DIR/prompts/" || warn "Could not fully migrate old prompts directory."
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

# No stash to re-apply — we used git checkout instead.
# User data (config.yaml, custom prompts) is restored from backup below.

# ── Restore user data ──────────────────────────────────────────────────
section "Restoring user data"

for f in "${PROTECTED_FILES[@]}"; do
    bak="$BACKUP_DIR/$(basename "$f")"
    if [ -f "$bak" ]; then
        if [ "$f" = "config.yaml" ]; then
            # config.yaml is already restored after git operations.
            # The merger below will handle it. Skip here.
            continue
        fi
        if safe_restore_file "$bak" "$DIR/$f"; then
            ok "Restored: $f"
        else
            warn "Could not restore $f (permission denied)."
        fi
    fi
done

for d in "${PROTECTED_DIRS[@]}"; do
    local_name="${d//\//__}"
    bak="$BACKUP_DIR/$local_name"
    if [ -d "$bak" ]; then
        # Use rsync if available for smart merge; fall back to cp
        copy_tree_merge "$bak/" "$DIR/$d/" || warn "Could not fully restore $d/."
        ok "Restored: $d/"
    fi
done

# Restore critical data files (these are gitignored so git can't touch them,
# but restore from backup for completeness in case of any edge case)
if [ -d "$BACKUP_DIR/data" ]; then
    mkdir -p "$DIR/data"
    for f in "${DATA_FILES[@]}"; do
        bak="$BACKUP_DIR/data/$(basename "$f")"
        if [ -f "$bak" ]; then
            safe_restore_file "$bak" "$DIR/$f" || warn "Could not restore $f (permission denied)."
        fi
    done
fi

# Restore custom prompt files
CUSTOM_PROMPTS="$BACKUP_DIR/prompts__custom"
if [ -d "$CUSTOM_PROMPTS" ] && [ "$(ls -A "$CUSTOM_PROMPTS")" ]; then
    copy_tree_merge "$CUSTOM_PROMPTS/" "$PROMPTS_DIR/" || warn "Could not fully restore custom prompt files."
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
            AURAGO_MASTER_KEY="$(read_master_key_from_env "$ENV_FILE")"
            if [ -z "${AURAGO_MASTER_KEY:-}" ]; then
                warn "Could not read AURAGO_MASTER_KEY from .env — skipping migration."
            else
                $SUDO mkdir -p "$CREDENTIAL_DIR"
                $SUDO chmod 700 "$CREDENTIAL_DIR"
                printf "AURAGO_MASTER_KEY=%s\n" "$AURAGO_MASTER_KEY" | $SUDO tee "$CREDENTIAL_FILE" > /dev/null
                $SUDO chmod 600 "$CREDENTIAL_FILE"
                $SUDO chown root:root "$CREDENTIAL_DIR" "$CREDENTIAL_FILE"
                rm -f "$ENV_FILE"
                ok "Master key moved to $CREDENTIAL_FILE (root-only, mode 0600)."
                ok "Removed $ENV_FILE."

                # Update systemd unit if it exists and still references .env
                SVC_FILE="/etc/systemd/system/aurago.service"
                if [ -f "$SVC_FILE" ]; then
                    if grep -q "EnvironmentFile=.*\.env" "$SVC_FILE" || grep -q "Environment=.*AURAGO_MASTER_KEY" "$SVC_FILE"; then
                        info "Updating systemd unit to use $CREDENTIAL_FILE ..."
                        # Replace EnvironmentFile pointing to .env
                        $SUDO sed -i "s|EnvironmentFile=.*\.env|EnvironmentFile=${CREDENTIAL_FILE}|g" "$SVC_FILE"
                        # Replace inline Environment= with EnvironmentFile=
                        $SUDO sed -i "s|Environment=\"AURAGO_MASTER_KEY=.*\"|EnvironmentFile=${CREDENTIAL_FILE}|g" "$SVC_FILE"
                        # Remove dash prefix (fail-silent) if present
                        $SUDO sed -i "s|EnvironmentFile=-|EnvironmentFile=|g" "$SVC_FILE"
                        # Add security hardening if not already present
                        if ! grep -q "NoNewPrivileges" "$SVC_FILE"; then
                            $SUDO sed -i "/^\[Install\]/i\\
# Security hardening\\
NoNewPrivileges=true\\
ProtectSystem=strict\\
ReadWritePaths=${DIR} ${CREDENTIAL_DIR}\\
ProtectHome=read-only\\
PrivateTmp=true" "$SVC_FILE"
                        fi
                        $SUDO systemctl daemon-reload
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

# ── Merge config.yaml ──────────────────────────────────────────────────
section "Merging configuration"

# Source:   backup of config.yaml taken before any git/file operations.
# Template: config_template.yaml in the repo (the authoritative template).
#           Binary-only mode: newly extracted config.yaml.new_template.
#           Fallback: git show HEAD:config.yaml.
# Output:   $DIR/config.yaml  (always the final result).
#
# If config.yaml didn't exist before (fresh install): copy template directly.

USER_CONFIG_BAK="$BACKUP_DIR/config.yaml"

if [ ! -f "$USER_CONFIG_BAK" ] && [ ! -f "$DIR/config.yaml" ]; then
    # Fresh install: no prior config at all — create from template.
    if [ -f "$DIR/config_template.yaml" ]; then
        cp "$DIR/config_template.yaml" "$DIR/config.yaml"
        ok "Created config.yaml from template."
    fi
else
    # Existing install: merge user settings with any new template fields.
    if $BINARY_ONLY && [ -f "$DIR/config.yaml.new_template" ]; then
        CURRENT_TEMPLATE="$DIR/config.yaml.new_template"
    elif [ -f "$DIR/config_template.yaml" ]; then
        CURRENT_TEMPLATE="$DIR/config_template.yaml"
    else
        # Fallback: extract template from git history.
        _TMPL=$(mktemp "/tmp/aurago-config-tmpl.XXXXXX")
        if git show HEAD:config_template.yaml > "$_TMPL" 2>/dev/null && [ -s "$_TMPL" ]; then
            CURRENT_TEMPLATE="$_TMPL"
        elif git show HEAD:config.yaml > "$_TMPL" 2>/dev/null && [ -s "$_TMPL" ]; then
            CURRENT_TEMPLATE="$_TMPL"
        else
            CURRENT_TEMPLATE=""
        fi
    fi

    if [ -n "${CURRENT_TEMPLATE:-}" ] && [ -f "$USER_CONFIG_BAK" ]; then
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
            if "$MERGER_BIN" -source "$USER_CONFIG_BAK" -template "$CURRENT_TEMPLATE" -output "$DIR/config.yaml"; then
                ok "Your settings have been merged into the new config.yaml."
            else
                warn "config-merger failed. Restoring your old config.yaml exactly."
                safe_restore_file "$USER_CONFIG_BAK" "$DIR/config.yaml" \
                    || die "Could not restore config.yaml after failed merge (permission denied)."
            fi
        else
            warn "config-merger not found. Keeping your existing config.yaml."
            # User's config is already on disk (restored after git ops). Nothing to do.
        fi
    else
        warn "No template found. Keeping your existing config.yaml."
    fi

    [ -n "${_TMPL:-}" ] && rm -f "$_TMPL"
    [ -f "$DIR/config.yaml.new_template" ] && rm -f "$DIR/config.yaml.new_template"
fi

# ── Update binary ───────────────────────────────────────────────────────
section "Updating binaries"

# Ensure bin directory exists (e.g. if user manually deleted it)
mkdir -p "$DIR/bin"

# Binaries are now distributed via GitHub Releases (no longer tracked in git)
GITHUB_REPO="antibyte/AuraGo"

# Resolve the latest release tag dynamically
RELEASE_TAG=$(latest_release_tag || true)
if [ -z "$RELEASE_TAG" ]; then
    warn "Could not determine latest release tag — trying 'latest' as fallback."
    RELEASE_TAG="latest"
else
    info "Latest release: $RELEASE_TAG"
fi
RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
fetch_release_checksums || die "Could not download SHA256SUMS for release ${RELEASE_TAG}."

_download_release_bin() {
    local name="$1"
    download_release_asset "$name" "$DIR/bin/$name"
}

# Add common Go install locations to PATH (in case the shell was not re-sourced after install)
for _godir in /usr/local/go/bin "$HOME/go/bin" /usr/local/bin; do
    [ -d "$_godir" ] && [[ ":$PATH:" != *":$_godir:"* ]] && export PATH="$_godir:$PATH"
done
unset _godir

GO_FOUND=false
if command -v go >/dev/null 2>&1; then
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    GO_FOUND=true
fi

if $GO_FOUND; then
    # ── Source build (Go available) ───────────────────────────────────────
    info "Go $GO_VERSION found — building from source..."

    if [ "$GOARCH" = "arm" ] && [ -n "${GOARM:-}" ]; then
        export GOARM
    fi

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

    info "Building aurago-remote_linux ($GOARCH)..."
    if CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags='-s -w' -o bin/aurago-remote_linux ./cmd/remote; then
        ok "bin/aurago-remote_linux built from source"
        mkdir -p "$DIR/deploy"
        cp "$DIR/bin/aurago-remote_linux" "$DIR/deploy/aurago-remote_linux_${GOARCH}"
    fi

    # Cross-compile aurago-remote for all client platforms so the
    # /api/remote/download/{os}/{arch} endpoint can serve them.
    info "Cross-compiling aurago-remote client binaries..."
    mkdir -p "$DIR/deploy"
    for _target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
        _os="${_target%/*}"
        _arch="${_target#*/}"
        _ext=""
        [ "$_os" = "windows" ] && _ext=".exe"
        _out="$DIR/deploy/aurago-remote_${_os}_${_arch}${_ext}"
        # Skip if we already built this exact combo above
        if [ "$_os" = "linux" ] && [ "$_arch" = "$GOARCH" ] && [ -f "$_out" ]; then
            continue
        fi
        if CGO_ENABLED=0 GOOS="$_os" GOARCH="$_arch" go build -trimpath -ldflags='-s -w' -o "$_out" ./cmd/remote; then
            ok "  $_out"
        else
            warn "  cross-compile failed: $_os/$_arch"
        fi
    done


else
    # ── Download binaries from GitHub Releases (no Go available) ─────────
    warn "Go is not installed — downloading pre-built binaries from GitHub Releases."

    # Pick arch-appropriate binary names
    if [ "$GOARCH" = "arm64" ]; then
        BINS=("aurago_linux_arm64" "lifeboat_linux_arm64" "config-merger_linux_arm64" "aurago-remote_linux_arm64")
    elif [ "$GOARCH" = "amd64" ]; then
        BINS=("aurago_linux" "lifeboat_linux" "config-merger_linux" "aurago-remote_linux")
    else
        die "No prebuilt release binaries for architecture ${ARCH_RAW}. Install Go 1.26+ to build from source."
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
        [ -f "$DIR/bin/aurago_linux_arm64" ]             && cp -p "$DIR/bin/aurago_linux_arm64"             "$DIR/bin/aurago_linux"
        [ -f "$DIR/bin/lifeboat_linux_arm64" ]           && cp -p "$DIR/bin/lifeboat_linux_arm64"           "$DIR/bin/lifeboat_linux"
        [ -f "$DIR/bin/config-merger_linux_arm64" ]      && cp -p "$DIR/bin/config-merger_linux_arm64"      "$DIR/bin/config-merger_linux"
        [ -f "$DIR/bin/aurago-remote_linux_arm64" ]      && cp -p "$DIR/bin/aurago-remote_linux_arm64"      "$DIR/bin/aurago-remote_linux"
    fi

    # Download aurago-remote client binaries for all platforms so the
    # /api/remote/download/{os}/{arch} endpoint can serve them.
    mkdir -p "$DIR/deploy"
    info "Downloading aurago-remote client binaries for all platforms..."
    for _t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
        _ros="${_t%/*}"; _rarch="${_t#*/}"; _rext=""
        [ "$_ros" = "windows" ] && _rext=".exe"
        _rname="aurago-remote_${_ros}_${_rarch}${_rext}"
        if download_release_asset "${_rname}" "$DIR/deploy/${_rname}"; then
            ok "  deploy/${_rname}"
        else
            warn "  Could not download deploy/${_rname} — skipping."
        fi
    done
    chmod +x "$DIR/deploy/aurago-remote_linux"* 2>/dev/null || true

    [ -f "$DIR/bin/aurago_linux" ] || die "Failed to obtain aurago_linux binary. Cannot continue."

    # Create lifeboat symlink
    [ -f "$DIR/bin/lifeboat_linux" ] && cp -p "$DIR/bin/lifeboat_linux" "$DIR/bin/lifeboat" 2>/dev/null || true
fi

# Ensure known binaries and helper scripts are executable. Keep this list
# explicit so updates never make arbitrary dropped files executable.
mark_executable_if_present() {
    local path="$1"
    [ -f "$path" ] || return 0
    chmod +x "$path" 2>/dev/null || $SUDO chmod +x "$path" 2>/dev/null || true
}

apply_aurago_setcap_if_available() {
    local binary="$DIR/bin/aurago_linux"
    [ -f "$binary" ] || binary="$DIR/bin/aurago"
    [ -f "$binary" ] || return 0
    command -v setcap >/dev/null 2>&1 || return 0
    setcap cap_net_bind_service=+ep "$binary" 2>/dev/null || \
        $SUDO setcap cap_net_bind_service=+ep "$binary" 2>/dev/null || \
        warn "setcap failed on ${binary} — run manually if you need HTTPS on privileged ports."
}

for _exe in \
    "$DIR/bin/aurago_linux" \
    "$DIR/bin/aurago_linux_amd64" \
    "$DIR/bin/aurago_linux_arm64" \
    "$DIR/bin/lifeboat" \
    "$DIR/bin/lifeboat_linux" \
    "$DIR/bin/lifeboat_linux_amd64" \
    "$DIR/bin/lifeboat_linux_arm64" \
    "$DIR/bin/config-merger_linux" \
    "$DIR/bin/config-merger_linux_amd64" \
    "$DIR/bin/config-merger_linux_arm64" \
    "$DIR/bin/aurago-remote_linux" \
    "$DIR/bin/aurago-remote_linux_amd64" \
    "$DIR/bin/aurago-remote_linux_arm64" \
    "$DIR/start.sh" \
    "$DIR/update.sh" \
    "$DIR/install_service_linux.sh" \
    "$DIR/make_deploy.sh"; do
    mark_executable_if_present "$_exe"
done
apply_aurago_setcap_if_available

# ── Patch service file: ensure User= / Group= are set (migration for root-installs) ──
SVC_FILE="/etc/systemd/system/aurago.service"
if [ -f "$SVC_FILE" ] && ! grep -q '^User=' "$SVC_FILE"; then
    # Detect the right user: prefer install directory owner, then SUDO_USER
    _svc_user=""
    _dir_owner="$(stat_owner "$DIR" 2>/dev/null || echo '')"
    if [ -n "$_dir_owner" ] && [ "$_dir_owner" != "root" ]; then
        _svc_user="$_dir_owner"
    elif [ -n "${SUDO_USER:-}" ]; then
        _svc_user="$SUDO_USER"
    fi

    if [ -n "$_svc_user" ]; then
        _svc_group=$(id -gn "$_svc_user" 2>/dev/null || echo "$_svc_user")
        warn "Service file missing User= — was running as root. Patching to User=${_svc_user}..."
        # Insert User=/Group= after Type= line
        $SUDO sed -i "/^Type=/a User=${_svc_user}\nGroup=${_svc_group}" "$SVC_FILE"
        # Fix ownership of data and bin so the new user can write them
        $SUDO chown -R "${_svc_user}:${_svc_group}" "${DIR}/data" "${DIR}/bin" "${DIR}/agent_workspace" 2>/dev/null || true
        $SUDO systemctl daemon-reload
        ok "Service patched: now runs as ${_svc_user}:${_svc_group}. Data directory re-owned."
    else
        warn "Service file has no User= and could not determine a non-root user."
        warn "Consider adding 'User=<youruser>' to $SVC_FILE manually."
    fi
fi

# ── Service restart ────────────────────────────────────────────────────
section "Restart"

if $NO_RESTART; then
    warn "Skipping restart (--no-restart flag set). Start manually:"
    echo "   sudo systemctl restart aurago   OR   ./start.sh"
elif command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files aurago.service >/dev/null 2>&1; then
    info "Starting aurago systemd service..."
    # Try sudo first; if it fails (no password), fall back to nohup start
    if $SUDO systemctl start aurago 2>/dev/null; then
        sleep 2
        if systemctl is-active --quiet aurago 2>/dev/null; then
            ok "Service started successfully via systemd."
        else
            warn "systemd start failed health check — rolling back to previous binary."
            if restore_previous_aurago_binary; then
                $SUDO systemctl start aurago 2>/dev/null || true
            fi
            if systemctl is-active --quiet aurago 2>/dev/null; then
                ok "Service recovered successfully after rollback."
            else
                warn "Rollback did not recover the service — check: sudo journalctl -u aurago -n 50"
            fi
        fi
    else
        warn "sudo not available — starting aurago directly (systemd will adopt on next boot)"
        LAUNCH_BIN="$DIR/bin/aurago_linux"
        [ ! -f "$LAUNCH_BIN" ] && LAUNCH_BIN="$DIR/bin/aurago"
        if [ -f "$LAUNCH_BIN" ]; then
            mkdir -p "$DIR/log"
            # Ensure the vault key is available even if env inheritance broke.
            if [ -z "${AURAGO_MASTER_KEY:-}" ] && [ -f "$DIR/.env" ]; then
                AURAGO_MASTER_KEY="$(read_master_key_from_env "$DIR/.env")"
                export AURAGO_MASTER_KEY
            fi
            nohup "$LAUNCH_BIN" --config "$DIR/config.yaml" >>"${DIR}/log/aurago.log" 2>&1 &
            LAUNCH_PID=$!
            info "AuraGo starting (PID=$LAUNCH_PID)..."
            sleep 3
            if kill -0 "$LAUNCH_PID" 2>/dev/null; then
                ok "AuraGo running (PID=$LAUNCH_PID). Logs: ${DIR}/log/aurago.log"
            else
                warn "direct start failed health check — rolling back to previous binary."
                if restore_previous_aurago_binary; then
                    nohup "$LAUNCH_BIN" --config "$DIR/config.yaml" >>"${DIR}/log/aurago.log" 2>&1 &
                    ok "Started previous AuraGo binary after rollback (PID=$!)."
                else
                    warn "AuraGo may have exited immediately — check: tail -n 50 ${DIR}/log/aurago.log"
                fi
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
        # Ensure the vault key is available even if env inheritance broke.
        if [ -z "${AURAGO_MASTER_KEY:-}" ] && [ -f "$DIR/.env" ]; then
            AURAGO_MASTER_KEY="$(read_master_key_from_env "$DIR/.env")"
            export AURAGO_MASTER_KEY
        fi
        nohup "$LAUNCH_BIN" --config "$DIR/config.yaml" >>"${DIR}/log/aurago.log" 2>&1 &
        LAUNCH_PID=$!
        info "AuraGo starting (PID=$LAUNCH_PID)..."
        sleep 3
        if kill -0 "$LAUNCH_PID" 2>/dev/null; then
            ok "AuraGo running (PID=$LAUNCH_PID). Logs: ${DIR}/log/aurago.log"
        else
            warn "direct start failed health check — rolling back to previous binary."
            if restore_previous_aurago_binary; then
                nohup "$LAUNCH_BIN" --config "$DIR/config.yaml" >>"${DIR}/log/aurago.log" 2>&1 &
                ok "Started previous AuraGo binary after rollback (PID=$!)."
            else
                warn "AuraGo may have exited immediately — check: tail -n 50 ${DIR}/log/aurago.log"
            fi
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
