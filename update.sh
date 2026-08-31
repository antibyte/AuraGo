#!/usr/bin/env bash
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
#  AuraGo Update Script (Linux)
#
#  Usage:  ./update.sh [--yes] [--no-restart] [--force-reset] [--rebuild]
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
FORCE_RESET=false
REBUILD=false
_AU_ESCAPED=""
for arg in "$@"; do
    case "$arg" in
        --yes)        AUTO_YES=true ;;
        --no-restart) NO_RESTART=true ;;
        --force-reset) FORCE_RESET=true ;;
        --rebuild)     REBUILD=true ;;
        --escaped)    _AU_ESCAPED=1 ;;   # internal: already running in an independent scope
        --help|-h)
            echo "Usage: $0 [--yes] [--no-restart] [--force-reset] [--rebuild]"
            echo "  --yes          Skip confirmation prompts"
            echo "  --no-restart   Do not restart the service after update"
            echo "  --force-reset  Reset a diverged git install to origin/main"
            echo "  --rebuild      Rebuild/reinstall even when the version is unchanged"
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

# Fetches must never stop an unattended updater to ask for Git credentials.
# Cached non-interactive credentials remain usable for private forks, while
# terminal and Git Credential Manager prompts are disabled explicitly.
git_fetch_origin_main() {
    local attempt
    local retry_delay
    for attempt in 1 2 3; do
        if GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=never \
            git -c credential.interactive=never -C "$DIR" fetch origin main --quiet; then
            return 0
        fi
        if [ "$attempt" -lt 3 ]; then
            retry_delay=$((attempt * 3))
            warn "Git fetch failed; retrying without interactive authentication in ${retry_delay}s..." >&2
            sleep "$retry_delay"
        fi
    done
    return 1
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

system_group_exists() {
    local group_name="$1"
    if command -v getent >/dev/null 2>&1; then
        getent group "$group_name" >/dev/null 2>&1
        return
    fi
    grep -q "^${group_name}:" /etc/group 2>/dev/null
}

system_group_id() {
    local group_name="$1"
    local group_record=""
    local group_id=""
    if command -v getent >/dev/null 2>&1; then
        group_record="$(getent group "$group_name" 2>/dev/null | head -n 1 || true)"
    else
        group_record="$(grep -m 1 "^${group_name}:" /etc/group 2>/dev/null || true)"
    fi
    group_id="$(printf '%s\n' "$group_record" | awk -F: '{print $3}')"
    case "$group_id" in
        ""|*[!0-9]*) return 1 ;;
    esac
    [ "$group_id" -gt 0 ] || return 1
    printf '%s' "$group_id"
}

system_gpu_group_ids() {
    local ids=()
    local group_name
    local group_id
    local existing
    local duplicate
    for group_name in render video; do
        group_id="$(system_group_id "$group_name" || true)"
        [ -n "$group_id" ] || continue
        duplicate=false
        for existing in "${ids[@]}"; do
            if [ "$existing" = "$group_id" ]; then
                duplicate=true
                break
            fi
        done
        $duplicate || ids+=("$group_id")
    done
    local IFS=,
    printf '%s' "${ids[*]}"
}

systemd_gpu_groups_line() {
    local groups=()
    local group_name
    for group_name in render video; do
        if system_group_exists "$group_name"; then
            groups+=("$group_name")
        fi
    done
    if [ "${#groups[@]}" -gt 0 ]; then
        local joined
        joined="${groups[*]}"
        printf 'SupplementaryGroups=%s' "$joined"
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
# When no interactive terminal is attached (triggered from web UI / nohup),
# use sudo -n so the command fails immediately instead of hanging on a
# password prompt.  stdin may be redirected by a wrapper while the process
# still has a controlling terminal, so probe the readable and writable
# /dev/tty instead of checking stdin alone.  Plain sudo reads the password
# from that controlling terminal and therefore remains usable in that case.
has_interactive_tty() {
    [ -r /dev/tty ] && [ -w /dev/tty ] && { : </dev/tty; } 2>/dev/null
}

if has_interactive_tty; then
    SUDO="sudo"
else
    SUDO="sudo -n"
fi

# ── Detect install mode ───────────────────────────────────────────────────
# Binary-only installs (no .git directory) are fully supported.
BINARY_ONLY=false
PRE_UPDATE_REF=""
GIT_VER=""
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

fetch_optional_url_to_file() {
    local url="$1"
    local out="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$out" 2>/dev/null
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$out" 2>/dev/null
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

strict_release_verify_enabled() {
    case "${AURAGO_STRICT_RELEASE_VERIFY:-}" in
        1|true|TRUE|yes|YES) return 0 ;;
        *) return 1 ;;
    esac
}

verify_release_checksums_signature() {
    [ -n "${RELEASE_BASE:-}" ] || die "RELEASE_BASE is not set."
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "$RELEASE_CHECKSUMS_FILE" ] || die "Release checksums are not available."

    local sig_file cert_file
    sig_file="$(mktemp "/tmp/aurago-sha256-sig.XXXXXX")"
    cert_file="$(mktemp "/tmp/aurago-sha256-cert.XXXXXX")"

    if ! fetch_optional_url_to_file "${RELEASE_BASE}/SHA256SUMS.sig" "$sig_file" || ! fetch_optional_url_to_file "${RELEASE_BASE}/SHA256SUMS.pem" "$cert_file"; then
        rm -f "$sig_file" "$cert_file"
        if strict_release_verify_enabled; then
            die "Release signature files are missing and AURAGO_STRICT_RELEASE_VERIFY=1 is set."
        fi
        warn "Release signature files not found; continuing with SHA256 manifest verification only."
        return 0
    fi

    if ! command -v cosign >/dev/null 2>&1; then
        rm -f "$sig_file" "$cert_file"
        if strict_release_verify_enabled; then
            die "cosign is required for strict release signature verification."
        fi
        warn "cosign not found; continuing with SHA256 manifest verification only."
        return 0
    fi

    if cosign verify-blob \
        --certificate "$cert_file" \
        --signature "$sig_file" \
        --certificate-identity-regexp "https://github.com/${GITHUB_REPO}/.*" \
        --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
        "$RELEASE_CHECKSUMS_FILE" >/dev/null 2>&1; then
        ok "Release checksum signature verified."
    else
        rm -f "$sig_file" "$cert_file"
        if strict_release_verify_enabled; then
            die "Release checksum signature verification failed."
        fi
        warn "Release checksum signature verification failed; continuing with SHA256 manifest verification only."
        return 0
    fi

    rm -f "$sig_file" "$cert_file"
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
    verify_release_checksums_signature
}

verify_release_asset() {
    local asset="$1"
    local path="$2"
    local expected actual
    [ -f "$path" ] || { warn "Cannot verify missing file: $path"; return 1; }
    [ -n "${RELEASE_CHECKSUMS_FILE:-}" ] && [ -f "$RELEASE_CHECKSUMS_FILE" ] || { warn "Release checksums are not available."; return 1; }
    expected="$(awk -v target="$asset" '{ sub(/\r$/, "", $2); if ($2 == target) { print $1; exit } }' "$RELEASE_CHECKSUMS_FILE")"
    [ -n "$expected" ] || { warn "Missing checksum entry for ${asset} in release manifest."; return 1; }
    actual="$(sha256_file "$path" || true)"
    [ -n "$actual" ] || { warn "No SHA256 tool available to verify ${asset}."; return 1; }
    [ "$actual" = "$expected" ] || { warn "Checksum verification failed for ${asset}."; return 1; }
}

download_release_asset() {
    local asset="$1"
    local dest="$2"
    local url="${RELEASE_BASE}/${asset}"
    fetch_url_to_file "$url" "$dest"
    verify_release_asset "$asset" "$dest"
}

_download_release_bin() {
    local name="$1"
    local dest="${2:-$DIR/bin/$name}"
    mkdir -p "$(dirname "$dest")"
    download_release_asset "$name" "$dest"
}

select_release_bins_for_arch() {
    if [ "$GOARCH" = "arm64" ]; then
        REQUIRED_BINS=("aurago_linux_arm64" "config-merger_linux_arm64")
        OPTIONAL_BINS=("aurago-remote_linux_arm64")
    elif [ "$GOARCH" = "amd64" ]; then
        REQUIRED_BINS=("aurago_linux" "config-merger_linux")
        OPTIONAL_BINS=("aurago-remote_linux")
    else
        die "No prebuilt release binaries for architecture ${ARCH_RAW}. Install Go 1.26.6+ to build from source."
    fi
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

prepare_untracked_merge_collisions() {
    # Git refuses to fast-forward when an untracked file would become tracked.
    # This commonly happens when a runtime asset was deployed manually before
    # the same asset was added to the repository. Remove only byte-identical
    # regular files; preserve a copy in the update backup for audit/rollback.
    local collision_backup="$BACKUP_DIR/untracked_merge_collisions"
    local rel abs backup_path
    local cleared=0
    declare -A untracked_paths=()

    while IFS= read -r -d '' rel; do
        untracked_paths["$rel"]=1
    done < <(git -C "$DIR" ls-files -z --others --exclude-standard)

    while IFS= read -r -d '' rel; do
        [ -n "${untracked_paths[$rel]+present}" ] || continue
        abs="$DIR/$rel"
        if [ ! -f "$abs" ] || [ -L "$abs" ]; then
            warn "Untracked path would be replaced by the update but is not a regular file: $rel"
            return 1
        fi
        if ! git -C "$DIR" show "origin/main:$rel" | cmp -s -- "$abs" -; then
            warn "Untracked file differs from the incoming tracked file: $rel"
            warn "Move or rename it, then run the update again; AuraGo will not overwrite it."
            return 1
        fi

        backup_path="$collision_backup/$rel"
        mkdir -p "$(dirname "$backup_path")"
        cp -p -- "$abs" "$backup_path" || return 1
        rm -f -- "$abs" || return 1
        cleared=$((cleared + 1))
        ok "Prepared byte-identical untracked file for repository update: $rel"
    done < <(git -C "$DIR" diff --name-only -z --diff-filter=A --no-renames HEAD..origin/main --)

    if [ "$cleared" -gt 0 ]; then
        info "Saved $cleared replaced untracked file(s) under $collision_backup"
    fi
}

restore_untracked_merge_collisions_after_failure() {
    local collision_backup="$BACKUP_DIR/untracked_merge_collisions"
    local backup_path rel destination
    [ -d "$collision_backup" ] || return 0

    while IFS= read -r -d '' backup_path; do
        rel="${backup_path#"$collision_backup/"}"
        destination="$DIR/$rel"
        if [ -e "$destination" ] || [ -L "$destination" ]; then
            warn "Rollback kept existing path instead of overwriting it: $rel"
            continue
        fi
        mkdir -p "$(dirname "$destination")"
        cp -p -- "$backup_path" "$destination" || warn "Could not restore untracked merge collision: $rel"
    done < <(find "$collision_backup" -type f -print0)
}

# ── Files & directories that must NEVER be touched ─────────────────────
# These are backed up before git operations and restored afterwards.
PROTECTED_FILES=(
    ".env"
    "config.yaml"
    "config_debug.yaml"
)
# Directories to back up fully (must be small — they go to /tmp).
# data/vectordb, data/embeddings, data/tts, data/vectordb_backup are intentionally excluded:
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
    INSTALLED_RELEASE=""
    if [ -f "$DIR/.version" ]; then
        INSTALLED_RELEASE="$(tr -d '\r\n' < "$DIR/.version")"
    fi
    if [ "$INSTALLED_RELEASE" = "$RELEASE_TAG" ] && ! $REBUILD; then
        ok "AuraGo is already at ${RELEASE_TAG}; no files or services were changed."
        exit 0
    fi
    RELEASE_BASE="https://github.com/${GITHUB_REPO}/releases/download/${RELEASE_TAG}"
    fetch_release_checksums || die "Could not download SHA256SUMS for release ${RELEASE_TAG}."
    echo ""
    confirm "Proceed with update to $RELEASE_TAG?" || { info "Update cancelled."; exit 0; }
else
    if ! git_fetch_origin_main; then
        die "Failed to fetch updates from GitHub without interactive authentication. Verify network access and the origin URL, then retry."
    fi

    LOCAL_HASH=$(git rev-parse HEAD)
    REMOTE_HASH=$(git rev-parse origin/main)
    GIT_UP_TO_DATE=false

    if [ "$LOCAL_HASH" = "$REMOTE_HASH" ]; then
        ok "Code is already at the latest version ($(git log --format='%h %s' -1))"
        GIT_UP_TO_DATE=true
        if ! $REBUILD; then
            ok "No rebuild requested; no files or services were changed."
            exit 0
        fi
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

# Capture the pre-update readiness contract before stopping anything. Older
# binaries that do not expose the read-only healthcheck remain "unknown".
CURRENT_AURAGO_BIN="$DIR/bin/aurago_linux"
[ -x "$CURRENT_AURAGO_BIN" ] || CURRENT_AURAGO_BIN="$DIR/bin/aurago"
CORE_WAS_READY="unknown"
TSNET_WAS_READY="unknown"
TSNET_STATE_DIR=""

binary_supports_option() {
    local binary="$1"
    local option="$2"
    [ -x "$binary" ] || return 1
    "$binary" --help 2>&1 | grep -q -- "$option"
}

tsnet_failure_guidance() {
    local health_output="$1"
    case "$health_output" in
        *TSNET_TIMEOUT*)
            printf '%s\n' "The tsnet startup retry timed out. Check outbound network, DNS, system time, and 'journalctl -u aurago'; do not reauthenticate unless /api/tsnet/status reports a login or node-key error."
            ;;
        *TSNET_LOGIN_REQUIRED*|*TSNET_NODE_KEY_EXPIRED*|*TSNET_AUTH_KEY_MISSING*|*TSNET_AUTH_KEY_REJECTED*)
            printf '%s\n' "Review /api/tsnet/status and use node-specific reauthentication."
            ;;
        *TSNET_STATE_CORRUPT*)
            printf '%s\n' "The persisted tsnet state could not be loaded. Keep the updater backup and review /api/tsnet/status before replacing state."
            ;;
        *)
            printf '%s\n' "Review /api/tsnet/status and 'journalctl -u aurago' for the node-specific failure code."
            ;;
    esac
}

configured_tsnet_state_dir_fallback() {
    local config_file="$DIR/config.yaml"
    local value=""
    if [ -f "$config_file" ]; then
        value="$(awk '
            /^tailscale:[[:space:]]*($|#)/ { in_tailscale=1; in_tsnet=0; next }
            in_tailscale {
                raw=$0
                line=raw
                sub(/^[[:space:]]+/, "", line)
                indent=length(raw)-length(line)
                if (indent == 0 && line !~ /^($|#)/) {
                    in_tailscale=0
                    in_tsnet=0
                    next
                }
                if (!in_tsnet && line ~ /^tsnet:[[:space:]]*($|#)/) {
                    in_tsnet=1
                    tsnet_indent=indent
                    next
                }
                if (in_tsnet && indent <= tsnet_indent && line !~ /^($|#)/) {
                    in_tsnet=0
                }
            }
            in_tsnet && line ~ /^state_dir:[[:space:]]*/ {
                sub(/^state_dir:[[:space:]]*/, "", line)
                sub(/[[:space:]]*#.*/, "", line)
                gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
                if ((substr(line,1,1) == "\"" && substr(line,length(line),1) == "\"") ||
                    (substr(line,1,1) == "\047" && substr(line,length(line),1) == "\047")) {
                    line=substr(line,2,length(line)-2)
                }
                print line
                exit
            }
        ' "$config_file")"
    fi
    [ -n "$value" ] || value="$DIR/data/tsnet"
    case "$value" in
        /*) printf '%s\n' "$value" ;;
        *) printf '%s\n' "$DIR/$value" ;;
    esac
}

if binary_supports_option "$CURRENT_AURAGO_BIN" "healthcheck"; then
    if "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 5s >/dev/null 2>&1; then
        CORE_WAS_READY="ready"
        if "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 5s --healthcheck-require-tsnet >/dev/null 2>&1; then
            TSNET_WAS_READY="ready"
        else
            TSNET_WAS_READY="not_ready"
        fi
    else
        CORE_WAS_READY="not_ready"
        TSNET_WAS_READY="not_ready"
    fi
fi
if binary_supports_option "$CURRENT_AURAGO_BIN" "print-tsnet-state-dir"; then
    TSNET_STATE_DIR="$("$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --print-tsnet-state-dir 2>/dev/null | tail -n 1 || true)"
fi
[ -n "$TSNET_STATE_DIR" ] || TSNET_STATE_DIR="$(configured_tsnet_state_dir_fallback)"
info "Pre-update readiness: core=${CORE_WAS_READY}, tsnet=${TSNET_WAS_READY}"

# ── Stop running instances BEFORE any file changes ────────────────────
# This must happen early so the binary file is not locked and lock files
# are cleaned up before git-pull/build overwrites anything.
section "Stopping running instances"

_find_install_pids() {
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
    printf '%s\n' "$pids"
}

_kill_proc() {
    local label="$1"; shift
    local pids
    pids="$(_find_install_pids "$@")"
    [ -n "${pids// /}" ] || { info "$label: not running"; return 0; }

    info "Stopping $label (SIGTERM)..."
    for pid in $pids; do
        kill -TERM "$pid" 2>/dev/null || true
    done

    # Wait up to 60 seconds for AuraGo's graceful shutdown contract.
    local waited=0
    while true; do
        local still_up=false
        for pid in $pids; do
            kill -0 "$pid" 2>/dev/null && { still_up=true; break; }
        done
        $still_up || break
        sleep 1; waited=$((waited + 1))
        [ $waited -ge 60 ] && break
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
        return 2
    fi

    ok "$label stopped"
    return 0
}

PRE_START_MODE="stopped"
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet aurago 2>/dev/null; then
    PRE_START_MODE="systemd"
elif [ -n "$(_find_install_pids "bin/aurago_linux" "bin/aurago" | tr -d '[:space:]')" ]; then
    PRE_START_MODE="direct"
fi
info "Pre-update start mode: ${PRE_START_MODE}"

restart_unchanged_after_failed_stop() {
    local launch_pid=""
    case "$PRE_START_MODE" in
        stopped)
            return 0
            ;;
        systemd)
            command -v systemctl >/dev/null 2>&1 || return 1
            $SUDO systemctl start aurago >/dev/null 2>&1 || return 1
            local waited=0
            while ! systemctl is-active --quiet aurago 2>/dev/null && [ "$waited" -lt 20 ]; do
                sleep 1
                waited=$((waited + 1))
            done
            systemctl is-active --quiet aurago 2>/dev/null || return 1
            ;;
        direct)
            [ -x "$CURRENT_AURAGO_BIN" ] || return 1
            mkdir -p "$DIR/log"
            if [ -z "${AURAGO_MASTER_KEY:-}" ] && [ -f "$DIR/.env" ]; then
                AURAGO_MASTER_KEY="$(read_master_key_from_env "$DIR/.env")"
                export AURAGO_MASTER_KEY
            fi
            nohup "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" >>"$DIR/log/aurago.log" 2>&1 &
            launch_pid=$!
            sleep 3
            kill -0 "$launch_pid" 2>/dev/null || return 1
            ;;
        *)
            return 1
            ;;
    esac

    if [ "$CORE_WAS_READY" = "ready" ] && binary_supports_option "$CURRENT_AURAGO_BIN" "healthcheck"; then
        "$CURRENT_AURAGO_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 60s >/dev/null 2>&1 || return 1
    elif [ "$PRE_START_MODE" = "systemd" ]; then
        systemctl is-active --quiet aurago 2>/dev/null || return 1
    elif [ "$PRE_START_MODE" = "direct" ]; then
        kill -0 "$launch_pid" 2>/dev/null || return 1
    fi
    return 0
}

abort_before_file_changes() {
    local reason="$1"
    if restart_unchanged_after_failed_stop; then
        if [ "$PRE_START_MODE" = "stopped" ]; then
            die "${reason} AuraGo was not running before the update; no restart was needed and no update files were touched."
        fi
        die "${reason} The unchanged installation was restarted and verified; no update files were touched."
    fi
    die "${reason} AuraGo could not be restarted automatically and is currently stopped. Start it manually with 'sudo systemctl start aurago' or './start.sh'. No update files were touched."
}

# Stop systemd first. An authorization failure is not bypassed with a manual
# kill because Restart=always/on-failure could race the updater.
SYSTEMD_STOP_FORCED=false
if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet aurago 2>/dev/null; then
    info "Stopping aurago systemd service..."
    if command -v timeout >/dev/null 2>&1; then
        # GNU timeout normally places the command in a separate process
        # group.  sudo then receives SIGTTIN while reading its password from
        # the controlling terminal and appears to hang.  --foreground keeps
        # the command in the terminal's foreground process group.
        if timeout --help 2>&1 | grep -q -- '--foreground'; then
            if ! timeout --foreground 60s $SUDO systemctl stop aurago; then
                abort_before_file_changes "systemd did not stop AuraGo cleanly within 60 seconds."
            fi
        elif has_interactive_tty && [ "$SUDO" = "sudo" ]; then
            # BusyBox timeout has no --foreground.  Authenticate before
            # entering its process group, then run the timed command without
            # another terminal read.
            if ! $SUDO -v || ! timeout 60s sudo -n systemctl stop aurago; then
                abort_before_file_changes "systemd did not stop AuraGo cleanly within 60 seconds."
            fi
        elif ! timeout 60s $SUDO systemctl stop aurago; then
            abort_before_file_changes "systemd did not stop AuraGo cleanly within 60 seconds."
        fi
    elif ! $SUDO systemctl stop aurago; then
        abort_before_file_changes "systemd could not stop AuraGo."
    fi
    if systemctl is-active --quiet aurago 2>/dev/null; then
        abort_before_file_changes "AuraGo is still active after systemd stop."
    fi
    _systemd_result="$(systemctl show aurago.service -p Result --value 2>/dev/null || true)"
    if [ "$_systemd_result" = "timeout" ]; then
        SYSTEMD_STOP_FORCED=true
    fi
    ok "systemd service stopped"
fi
# Always kill any remaining instances (covers manual starts, systemd restarts, etc.)
_stop_rc=0
_kill_proc "aurago" "bin/aurago_linux" "bin/aurago" || _stop_rc=$?
if [ "$_stop_rc" -ne 0 ]; then
    if [ "$_stop_rc" -eq 2 ]; then
        abort_before_file_changes "AuraGo required SIGKILL after the 60-second shutdown deadline."
    fi
    abort_before_file_changes "AuraGo could not be stopped safely."
fi
if $SYSTEMD_STOP_FORCED; then
    abort_before_file_changes "systemd reported a forced timeout stop."
fi

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
SYSTEMD_DROPIN_DIR="/etc/systemd/system/aurago.service.d"
SYSTEMD_STOP_TIMEOUT_DROPIN="${SYSTEMD_DROPIN_DIR}/20-aurago-stop-timeout.conf"
SYSTEMD_DROPIN_BACKUP="${BACKUP_DIR}/20-aurago-stop-timeout.conf"
SYSTEMD_DROPIN_EXISTED=false
SYSTEMD_DROPIN_CHANGED=false

if [ -L "$SYSTEMD_DROPIN_DIR" ] || [ -L "$SYSTEMD_STOP_TIMEOUT_DROPIN" ]; then
    abort_before_file_changes "Refusing to update an unsafe symlinked systemd drop-in path."
fi
if [ -f "$SYSTEMD_STOP_TIMEOUT_DROPIN" ]; then
    if cp -p "$SYSTEMD_STOP_TIMEOUT_DROPIN" "$SYSTEMD_DROPIN_BACKUP" 2>/dev/null || \
       $SUDO cp -p "$SYSTEMD_STOP_TIMEOUT_DROPIN" "$SYSTEMD_DROPIN_BACKUP"; then
        SYSTEMD_DROPIN_EXISTED=true
        ok "Backed up the systemd stop-timeout drop-in."
    else
        abort_before_file_changes "Could not back up the existing systemd stop-timeout drop-in."
    fi
fi

validate_tsnet_state_dir() {
    local path="${1%/}"
    [ -n "$path" ] || return 1
    case "$path" in
        /*) ;;
        *) path="$DIR/$path" ;;
    esac
    local current="/"
    local part
    local parts=()
    IFS='/' read -r -a parts <<< "${path#/}"
    for part in "${parts[@]}"; do
        [ -n "$part" ] || continue
        current="${current%/}/$part"
        [ ! -L "$current" ] || return 1
    done
    [ -d "$path" ] || return 1
    local resolved
    resolved="$(cd "$path" 2>/dev/null && pwd -P)" || return 1
    case "$resolved" in
        "/"|"${HOME%/}"|"${DIR%/}") return 1 ;;
    esac
    printf '%s\n' "$resolved"
}

backup_tsnet_state() {
    local resolved candidate
    candidate="${TSNET_STATE_DIR%/}"
    case "$candidate" in
        /*) ;;
        *) candidate="$DIR/$candidate" ;;
    esac
    if [ ! -e "$candidate" ]; then
        if [ "$TSNET_WAS_READY" = "ready" ]; then
            abort_before_file_changes "The previously working tsnet state directory disappeared before backup."
        fi
        warn "No persisted tsnet state directory exists yet; there is nothing to back up."
        return 0
    fi
    resolved="$(validate_tsnet_state_dir "$TSNET_STATE_DIR" || true)"
    if [ -z "$resolved" ]; then
        abort_before_file_changes "Refusing to update because the configured tsnet state path is unsafe."
    fi
    printf '%s\n' "$resolved" > "$BACKUP_DIR/tsnet-state.path"
    if cp -a -- "$resolved" "$BACKUP_DIR/tsnet-state" 2>/dev/null || $SUDO cp -a -- "$resolved" "$BACKUP_DIR/tsnet-state"; then
        ok "Backed up tsnet state with ownership and mode."
        return 0
    fi
    abort_before_file_changes "Could not back up the configured tsnet state directory safely."
}

restore_tsnet_state_backup() {
    [ -d "$BACKUP_DIR/tsnet-state" ] || return 0
    [ -f "$BACKUP_DIR/tsnet-state.path" ] || return 1
    local original resolved failed_copy
    original="$(head -n 1 "$BACKUP_DIR/tsnet-state.path")"
    resolved="$(validate_tsnet_state_dir "$original" || true)"
    if [ -z "$resolved" ]; then
        # The failed runtime may have removed the directory. Validate its parent
        # and original lexical target before recreating it.
        [ ! -L "$original" ] || return 1
        local original_parent resolved_parent
        original_parent="$(dirname "$original")"
        resolved_parent="$(cd "$original_parent" 2>/dev/null && pwd -P)" || return 1
        [ "$resolved_parent" = "$original_parent" ] || return 1
        resolved="$resolved_parent/$(basename "$original")"
    fi
    case "${resolved%/}" in
        ""|"/"|"${HOME%/}"|"${DIR%/}") return 1 ;;
    esac
    failed_copy="$BACKUP_DIR/tsnet-state.failed"
    if [ -e "$resolved" ]; then
        mv -- "$resolved" "$failed_copy" 2>/dev/null || $SUDO mv -- "$resolved" "$failed_copy" || return 1
    fi
    mkdir -p "$(dirname "$resolved")" 2>/dev/null || $SUDO mkdir -p "$(dirname "$resolved")"
    if cp -a -- "$BACKUP_DIR/tsnet-state" "$resolved" 2>/dev/null || $SUDO cp -a -- "$BACKUP_DIR/tsnet-state" "$resolved"; then
        ok "Restored pre-update tsnet state."
        return 0
    fi
    if [ -e "$failed_copy" ]; then
        mv -- "$failed_copy" "$resolved" 2>/dev/null || $SUDO mv -- "$failed_copy" "$resolved" || true
    fi
    return 1
}

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

restore_critical_user_data_after_failure() {
    for f in "${PROTECTED_FILES[@]}"; do
        local bak="$BACKUP_DIR/$(basename "$f")"
        [ -f "$bak" ] || continue
        safe_restore_file "$bak" "$DIR/$f" || warn "Could not restore $f during rollback."
    done
    if [ -d "$BACKUP_DIR/data" ]; then
        mkdir -p "$DIR/data"
        for f in "${DATA_FILES[@]}"; do
            local bak="$BACKUP_DIR/data/$(basename "$f")"
            [ -f "$bak" ] || continue
            safe_restore_file "$bak" "$DIR/$f" || warn "Could not restore $f during rollback."
        done
    fi
}

backup_binary_update_resources() {
    $BINARY_ONLY || return 0
    BINARY_RESOURCE_BACKUP_DIR="$BACKUP_DIR/binary_update_resources"
    mkdir -p "$BINARY_RESOURCE_BACKUP_DIR"
    : > "$BINARY_RESOURCE_BACKUP_DIR/missing.txt"
    for rel in prompts agent_workspace assets ui update.sh config.yaml.new_template; do
        if [ -e "$DIR/$rel" ]; then
            mkdir -p "$BINARY_RESOURCE_BACKUP_DIR/$(dirname "$rel")"
            cp -a "$DIR/$rel" "$BINARY_RESOURCE_BACKUP_DIR/$rel" 2>/dev/null || \
                copy_tree_merge "$DIR/$rel/" "$BINARY_RESOURCE_BACKUP_DIR/$rel/" || \
                warn "Could not fully back up $rel for binary rollback."
        else
            printf '%s\n' "$rel" >> "$BINARY_RESOURCE_BACKUP_DIR/missing.txt"
        fi
    done
}

restore_binary_update_resources_after_failure() {
    $BINARY_ONLY || return 0
    [ -n "${BINARY_RESOURCE_BACKUP_DIR:-}" ] && [ -d "$BINARY_RESOURCE_BACKUP_DIR" ] || return 0
    for rel in prompts agent_workspace assets ui update.sh config.yaml.new_template; do
        if grep -Fxq "$rel" "$BINARY_RESOURCE_BACKUP_DIR/missing.txt" 2>/dev/null; then
            rm -rf "$DIR/$rel"
            continue
        fi
        [ -e "$BINARY_RESOURCE_BACKUP_DIR/$rel" ] || continue
        rm -rf "$DIR/$rel"
        mkdir -p "$DIR/$(dirname "$rel")"
        cp -a "$BINARY_RESOURCE_BACKUP_DIR/$rel" "$DIR/$rel" 2>/dev/null || \
            copy_tree_merge "$BINARY_RESOURCE_BACKUP_DIR/$rel/" "$DIR/$rel/" || \
            warn "Could not fully restore $rel during binary rollback."
    done
}

restart_previous_after_rollback() {
    $NO_RESTART && return 0
    CURRENT_AURAGO_BIN="$DIR/bin/aurago_linux"
    [ -x "$CURRENT_AURAGO_BIN" ] || CURRENT_AURAGO_BIN="$DIR/bin/aurago"
    if restart_unchanged_after_failed_stop; then
        if [ "$PRE_START_MODE" = "stopped" ]; then
            info "AuraGo was stopped before the update; rollback left it stopped."
        else
            ok "Previous AuraGo start mode restored and verified after rollback."
        fi
        return 0
    fi
    warn "The previous AuraGo installation was restored but could not be restarted automatically."
    warn "Start it manually with 'sudo systemctl start aurago' or './start.sh'."
    return 1
}

restore_service_stop_timeout_dropin() {
    $SYSTEMD_DROPIN_CHANGED || return 0
    if $SYSTEMD_DROPIN_EXISTED; then
        [ -f "$SYSTEMD_DROPIN_BACKUP" ] || return 1
        $SUDO mkdir -p "$SYSTEMD_DROPIN_DIR" || return 1
        $SUDO install -o root -g root -m 0644 "$SYSTEMD_DROPIN_BACKUP" "$SYSTEMD_STOP_TIMEOUT_DROPIN" || return 1
    else
        $SUDO rm -f -- "$SYSTEMD_STOP_TIMEOUT_DROPIN" || return 1
        $SUDO rmdir "$SYSTEMD_DROPIN_DIR" >/dev/null 2>&1 || true
    fi
    $SUDO systemctl daemon-reload >/dev/null 2>&1 || return 1
    SYSTEMD_DROPIN_CHANGED=false
    return 0
}

abort_update() {
    local msg="$1"
    warn "Update failed after shutdown; rolling back to the previous working state."
    if command -v systemctl >/dev/null 2>&1; then
        timeout 60s $SUDO systemctl stop aurago >/dev/null 2>&1 || true
    fi
    _kill_proc "updated AuraGo" "bin/aurago_linux" "bin/aurago" >/dev/null 2>&1 || true
    restore_previous_aurago_binary || true
    if [ -n "${PRE_UPDATE_REF:-}" ] && [ -d "$DIR/.git" ]; then
        git -C "$DIR" reset --hard "$PRE_UPDATE_REF" >/dev/null 2>&1 || warn "Could not reset git checkout to $PRE_UPDATE_REF during rollback."
    fi
    restore_untracked_merge_collisions_after_failure
    restore_binary_update_resources_after_failure
    restore_critical_user_data_after_failure
    restore_tsnet_state_backup || warn "Could not restore the pre-update tsnet state automatically."
    restore_service_stop_timeout_dropin || warn "Could not restore the previous systemd stop-timeout drop-in automatically."
    if ! restart_previous_after_rollback; then
        die "${msg} The previous files were restored, but AuraGo is stopped; start it manually with 'sudo systemctl start aurago' or './start.sh'."
    fi
    die "$msg"
}

backup_current_aurago_binary
backup_tsnet_state

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
        CUSTOM_COUNT=0
        while IFS= read -r -d '' fp; do
            if [ ! -f "$DIR/$fp" ]; then
                warn "Skipping missing prompt file during backup: $fp"
                continue
            fi
            rel="${fp#prompts/}"
            dest_dir="$CUSTOM_PROMPTS/$(dirname "$rel")"
            mkdir -p "$dest_dir"
            if cp -p "$DIR/$fp" "$dest_dir/"; then
                CUSTOM_COUNT=$((CUSTOM_COUNT + 1))
            else
                warn "Could not back up prompt file: $fp"
            fi
        done < <(git -C "$DIR" ls-files -z --others --modified -- "prompts/")
    fi
    ok "Backed up $CUSTOM_COUNT prompt file(s)"
fi

backup_binary_update_resources

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

STAGED_RELEASE_DIR=""
REQUIRED_BINS=()
OPTIONAL_BINS=()
if $BINARY_ONLY && ! $GO_FOUND; then
    select_release_bins_for_arch
    STAGED_RELEASE_DIR="$(mktemp -d "${_AU_RUNTIME_DIR}/release.XXXXXX")"
    for BIN_NAME in "${REQUIRED_BINS[@]}"; do
        info "Staging required $BIN_NAME from GitHub Releases..."
        if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
            ok "$BIN_NAME downloaded and verified."
        else
            abort_update "Required release artifact $BIN_NAME could not be downloaded or verified."
        fi
    done
    for BIN_NAME in "${OPTIONAL_BINS[@]}"; do
        info "Staging optional $BIN_NAME from GitHub Releases..."
        if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
            ok "$BIN_NAME downloaded and verified."
        else
            warn "$BIN_NAME download failed; continuing without optional remote client artifact."
        fi
    done
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
    [ -d "$TMPEXT/assets" ]            && cp -r "$TMPEXT/assets"            "$DIR/"
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
    ok "Resources updated from release $RELEASE_TAG"
else
    # Git-based update.
    if ! $GIT_UP_TO_DATE; then
        PRE_UPDATE_REF="$(git -C "$DIR" rev-parse HEAD 2>/dev/null || true)"
        if ! prepare_untracked_merge_collisions; then
            abort_update "Update aborted because an untracked file conflicts with incoming repository content."
        fi
        if ! git diff --quiet || ! git diff --cached --quiet; then
            info "Cleaning local tracked changes before update..."
            if ! clean_tracked_changes; then
                warn "Automatic cleanup of tracked changes failed."
                warn "Changed files still present:"
                git -C "$DIR" status --porcelain --untracked-files=no | head -20 || true
                abort_update "Cannot continue update while tracked files are locked/unwritable. Fix permissions or run with sudo."
            fi
        fi

        if ! git_fetch_origin_main; then
            abort_update "Failed to fetch updates from GitHub without interactive authentication. Verify network access and the origin URL."
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
                    if $FORCE_RESET; then
                        warn "Branches have diverged. --force-reset was supplied; resetting tracked files to origin/main."
                        git reset --hard origin/main
                        ok "Hard reset complete."
                    else
                        warn "Branches have diverged. AuraGo will not discard local commits automatically."
                        warn "Review local commits, merge/rebase manually, or rerun with --force-reset if you intentionally want origin/main to replace this checkout."
                        abort_update "Update aborted safely (no hard reset performed)."
                    fi
                else
                    warn "Could not fast-forward automatically."
                    warn "Please ensure repository files are writable and no manual merge is required."
                    abort_update "Update aborted safely (no hard reset performed)."
                fi
            fi
        fi
        ok "Code updated to $(git log --format='%h  %s' -1)"
        GIT_VER=$(git describe --tags --always 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo 'git')
    fi

    # Restore user's config.yaml — git must never win over user's config.
    if [ -f "$BACKUP_DIR/config.yaml" ]; then
        safe_restore_file "$BACKUP_DIR/config.yaml" "$DIR/config.yaml" \
            || abort_update "Could not restore config.yaml (permission denied)."
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
        abort_update "Failed to build required bin/aurago_linux. Update aborted."
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

    [ -f "$DIR/bin/aurago_linux" ] || abort_update "Required AuraGo binary missing after source build."
    ok "Main binary built successfully"
    if $BINARY_ONLY; then
        printf '%s' "$RELEASE_TAG" > "$DIR/.version"
    elif [ -n "${GIT_VER:-}" ]; then
        printf '%s' "$GIT_VER" > "$DIR/.version"
    fi

else
    # ── Download binaries from GitHub Releases (no Go available) ─────────
    warn "Go is not installed — downloading pre-built binaries from GitHub Releases."

    select_release_bins_for_arch

    if [ -z "${STAGED_RELEASE_DIR:-}" ]; then
        STAGED_RELEASE_DIR="$(mktemp -d "${_AU_RUNTIME_DIR}/release.XXXXXX")"
        for BIN_NAME in "${REQUIRED_BINS[@]}"; do
            info "Downloading required $BIN_NAME from GitHub Releases..."
            if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
                ok "$BIN_NAME downloaded and verified."
            else
                abort_update "Required release artifact $BIN_NAME could not be downloaded or verified."
            fi
        done
        for BIN_NAME in "${OPTIONAL_BINS[@]}"; do
            info "Downloading optional $BIN_NAME from GitHub Releases..."
            if _download_release_bin "$BIN_NAME" "$STAGED_RELEASE_DIR/$BIN_NAME"; then
                ok "$BIN_NAME downloaded and verified."
            else
                warn "$BIN_NAME download failed; continuing without optional remote client artifact."
            fi
        done
    fi

    mkdir -p "$DIR/bin"
    for BIN_NAME in "${REQUIRED_BINS[@]}" "${OPTIONAL_BINS[@]}"; do
        [ -f "$STAGED_RELEASE_DIR/$BIN_NAME" ] || continue
        cp -p "$STAGED_RELEASE_DIR/$BIN_NAME" "$DIR/bin/$BIN_NAME" || abort_update "Could not install verified release artifact $BIN_NAME."
    done

    # Ensure standard names exist (for arm64 → copy to non-suffixed names)
    if [ "$GOARCH" = "arm64" ]; then
        [ -f "$DIR/bin/aurago_linux_arm64" ]             && cp -p "$DIR/bin/aurago_linux_arm64"             "$DIR/bin/aurago_linux"
        [ -f "$DIR/bin/config-merger_linux_arm64" ]      && cp -p "$DIR/bin/config-merger_linux_arm64"      "$DIR/bin/config-merger_linux"
        [ -f "$DIR/bin/aurago-remote_linux_arm64" ]      && cp -p "$DIR/bin/aurago-remote_linux_arm64"      "$DIR/bin/aurago-remote_linux"
    fi

    # Download aurago-remote client binaries for all platforms so the
    # /api/remote/download/{os}/{arch} endpoint can serve them.
    mkdir -p "$DIR/deploy"
    STAGED_DEPLOY_DIR="$(mktemp -d "${_AU_RUNTIME_DIR}/deploy.XXXXXX")"
    info "Downloading aurago-remote client binaries for all platforms..."
    for _t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
        _ros="${_t%/*}"; _rarch="${_t#*/}"; _rext=""
        [ "$_ros" = "windows" ] && _rext=".exe"
        _rname="aurago-remote_${_ros}_${_rarch}${_rext}"
        if download_release_asset "${_rname}" "$STAGED_DEPLOY_DIR/${_rname}"; then
            cp -p "$STAGED_DEPLOY_DIR/${_rname}" "$DIR/deploy/${_rname}"
            ok "  deploy/${_rname}"
        else
            warn "  Could not download deploy/${_rname} — skipping."
        fi
    done
    mark_executable_if_present "$DIR/deploy/aurago-remote_linux_amd64"
    mark_executable_if_present "$DIR/deploy/aurago-remote_linux_arm64"
    mark_executable_if_present "$DIR/deploy/aurago-remote_linux"

    [ -f "$DIR/bin/aurago_linux" ] || abort_update "Required AuraGo binary missing after update."
    ok "Main binary built successfully"
    printf '%s' "$RELEASE_TAG" > "$DIR/.version"
fi

# Ensure known binaries and helper scripts are executable. Keep this list
# explicit so updates never make arbitrary dropped files executable.
for _exe in \
    "$DIR/bin/aurago_linux" \
    "$DIR/bin/aurago_linux_amd64" \
    "$DIR/bin/aurago_linux_arm64" \
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

# Grant existing systemd installations access to GPU render devices without
# requiring permanent changes to the service user's account memberships.
_gpu_groups_line="$(systemd_gpu_groups_line)"
if [ -f "$SVC_FILE" ] && [ -n "$_gpu_groups_line" ]; then
    _current_gpu_groups_line="$(grep '^SupplementaryGroups=' "$SVC_FILE" | head -n 1 || true)"
    if [ "$_current_gpu_groups_line" != "$_gpu_groups_line" ]; then
        if [ -n "$_current_gpu_groups_line" ]; then
            $SUDO sed -i "s/^SupplementaryGroups=.*/${_gpu_groups_line}/" "$SVC_FILE"
        elif grep -q '^Group=' "$SVC_FILE"; then
            $SUDO sed -i "/^Group=/a ${_gpu_groups_line}" "$SVC_FILE"
        elif grep -q '^User=' "$SVC_FILE"; then
            $SUDO sed -i "/^User=/a ${_gpu_groups_line}" "$SVC_FILE"
        fi
        $SUDO systemctl daemon-reload
        ok "Service GPU access updated: ${_gpu_groups_line#SupplementaryGroups=}."
    fi
fi

# Forward numeric host GPU group IDs to managed containers. Host account group
# membership is intentionally left unchanged; Docker receives only the groups
# needed by the isolated Vulkan sidecar.
_gpu_group_ids="$(system_gpu_group_ids)"
_gpu_group_ids_line=""
if [ -n "$_gpu_group_ids" ]; then
    _gpu_group_ids_line="Environment=\"AURAGO_GPU_GROUP_IDS=${_gpu_group_ids}\""
fi
if [ -f "$SVC_FILE" ]; then
    _current_gpu_group_ids_line="$(grep '^Environment=.*AURAGO_GPU_GROUP_IDS=' "$SVC_FILE" | head -n 1 || true)"
    if [ "$_current_gpu_group_ids_line" != "$_gpu_group_ids_line" ]; then
        if [ -z "$_gpu_group_ids_line" ]; then
            $SUDO sed -i '/^Environment=.*AURAGO_GPU_GROUP_IDS=/d' "$SVC_FILE"
        elif [ -n "$_current_gpu_group_ids_line" ]; then
            $SUDO sed -i "/^Environment=.*AURAGO_GPU_GROUP_IDS=/c\\${_gpu_group_ids_line}" "$SVC_FILE"
        elif grep -q '^SupplementaryGroups=' "$SVC_FILE"; then
            $SUDO sed -i "/^SupplementaryGroups=/a ${_gpu_group_ids_line}" "$SVC_FILE"
        elif grep -q '^Group=' "$SVC_FILE"; then
            $SUDO sed -i "/^Group=/a ${_gpu_group_ids_line}" "$SVC_FILE"
        elif grep -q '^User=' "$SVC_FILE"; then
            $SUDO sed -i "/^User=/a ${_gpu_group_ids_line}" "$SVC_FILE"
        fi
        $SUDO systemctl daemon-reload
        if [ -n "$_gpu_group_ids" ]; then
            ok "Managed-container GPU groups updated: ${_gpu_group_ids}."
        else
            ok "Removed stale managed-container GPU group IDs."
        fi
    fi
fi

# Keep systemd's stop deadline slightly above AuraGo's 45-second internal
# shutdown deadline. A dedicated drop-in avoids position-dependent edits to
# legacy service files.
if [ -f "$SVC_FILE" ]; then
    _dropin_tmp="$(mktemp)"
    printf '%s\n' \
        '[Service]' \
        'TimeoutStopSec=60s' > "$_dropin_tmp"
    if ! $SUDO mkdir -p "$SYSTEMD_DROPIN_DIR" ||
       ! $SUDO install -o root -g root -m 0644 "$_dropin_tmp" "$SYSTEMD_STOP_TIMEOUT_DROPIN"; then
        rm -f -- "$_dropin_tmp"
        abort_update "Could not install the systemd stop-timeout drop-in."
    fi
    rm -f -- "$_dropin_tmp"
    SYSTEMD_DROPIN_CHANGED=true
    if ! $SUDO systemctl daemon-reload ||
       ! systemd-analyze verify aurago.service >/dev/null 2>&1; then
        restore_service_stop_timeout_dropin || warn "Could not restore the previous systemd stop-timeout drop-in after verification failed."
        abort_update "systemd rejected the AuraGo service configuration after installing the stop-timeout drop-in."
    fi
    ok "Service stop timeout drop-in verified at 60 seconds."
fi

# ── Service restart ────────────────────────────────────────────────────
section "Restart"

LAUNCH_BIN="$DIR/bin/aurago_linux"
[ -x "$LAUNCH_BIN" ] || LAUNCH_BIN="$DIR/bin/aurago"
STARTED_AFTER_UPDATE=false
START_MODE=""

start_updated_directly() {
    [ -x "$LAUNCH_BIN" ] || return 1
    mkdir -p "$DIR/log"
    if [ -z "${AURAGO_MASTER_KEY:-}" ] && [ -f "$DIR/.env" ]; then
        AURAGO_MASTER_KEY="$(read_master_key_from_env "$DIR/.env")"
        export AURAGO_MASTER_KEY
    fi
    nohup "$LAUNCH_BIN" --config "$DIR/config.yaml" >>"${DIR}/log/aurago.log" 2>&1 &
    LAUNCH_PID=$!
    START_MODE="direct"
    STARTED_AFTER_UPDATE=true
    info "AuraGo starting directly (PID=$LAUNCH_PID)..."
}

if $NO_RESTART; then
    warn "Skipping restart (--no-restart flag set). Start manually:"
    echo "   sudo systemctl restart aurago   OR   ./start.sh"
elif command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files aurago.service >/dev/null 2>&1; then
    info "Starting aurago systemd service..."
    if $SUDO systemctl start aurago; then
        START_MODE="systemd"
        STARTED_AFTER_UPDATE=true
    else
        warn "sudo not available — starting aurago directly (systemd will adopt on next boot)"
        start_updated_directly || abort_update "No executable AuraGo binary is available after the update."
    fi
else
    start_updated_directly || abort_update "No executable AuraGo binary is available after the update."
fi

if $STARTED_AFTER_UPDATE; then
    if [ "$START_MODE" = "systemd" ]; then
        # A systemd start failed health check is a core failure and must use
        # the same complete rollback path as any other core-readiness failure.
        _active_wait=0
        while ! systemctl is-active --quiet aurago 2>/dev/null && [ "$_active_wait" -lt 20 ]; do
            sleep 1
            _active_wait=$((_active_wait + 1))
        done
        if ! systemctl is-active --quiet aurago 2>/dev/null; then
            abort_update "The updated systemd process did not become active."
        fi
        ok "Updated systemd process is active."
    fi

    if ! "$LAUNCH_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 60s; then
        abort_update "Core readiness failed after the update."
    fi
    ok "Core readiness verified."

    if [ "$TSNET_WAS_READY" = "ready" ]; then
        TSNET_HEALTH_OUTPUT=""
        if ! TSNET_HEALTH_OUTPUT="$("$LAUNCH_BIN" --config "$DIR/config.yaml" --healthcheck --healthcheck-timeout 210s --healthcheck-require-tsnet 2>&1)"; then
            [ -z "$TSNET_HEALTH_OUTPUT" ] || printf '%s\n' "$TSNET_HEALTH_OUTPUT" >&2
            warn "AuraGo core is healthy, but the previously working tsnet node did not recover."
            warn "The updated binary and the state backup are being kept; the core update does not need rollback."
            warn "$(tsnet_failure_guidance "$TSNET_HEALTH_OUTPUT")"
            die "Update incomplete: tsnet readiness failed. Backup: $BACKUP_DIR"
        fi
        ok "tsnet readiness verified."
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
