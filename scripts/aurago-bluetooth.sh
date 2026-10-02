#!/usr/bin/env bash
# AuraGo Bluetooth host preparation kit.
#
# The block between the KIT markers is embedded verbatim into install.sh,
# update.sh and install_service_linux.sh (they stay single-file). Edit it
# here, then run:
#
#     bash scripts/sync-bluetooth-kit.sh
#
# internal/audit fails when an embedded copy differs from this block.

# >>> AURAGO-BLUETOOTH-KIT v1 >>>
# Bluetooth host preparation (source: scripts/aurago-bluetooth.sh; sync with
# scripts/sync-bluetooth-kit.sh). Needs only the caller's SUDO (empty when
# root). Callers use btk_resolve_choice and btk_run_choice. No step aborts
# the caller; failures end up in a summary with the command to run by hand.

BTK_LOG="${BTK_LOG:-${TMPDIR:-/tmp}/aurago-bluetooth-setup.log}"
BTK_DONE=()
BTK_FAILED=()

btk_has() { command -v "$1" >/dev/null 2>&1; }
btk_info() { if declare -F info >/dev/null; then info "$*"; else printf '[INFO] %s\n' "$*"; fi; }
btk_ok() { if declare -F ok >/dev/null; then ok "$*"; else printf '[ OK ] %s\n' "$*"; fi; }
btk_warn() { if declare -F warn >/dev/null; then warn "$*"; else printf '[WARN] %s\n' "$*"; fi; }
btk_done() { BTK_DONE+=("$1"); btk_ok "$1"; }
# btk_fail <what failed> <command to run by hand>
btk_fail() { BTK_FAILED+=("$1|$2"); btk_warn "$1 failed. Run by hand: $2"; }

# btk_run <label> command...: spinner through the TUI kit when present,
# otherwise the output goes to $BTK_LOG.
btk_run() {
    local label="$1"
    shift
    if declare -F tui_run >/dev/null; then
        TUI_RUN_SUDO=1 tui_run "$label" "$@" </dev/null
    else
        btk_info "$label"
        "$@" >>"$BTK_LOG" 2>&1 </dev/null
    fi
}

btk_in_container() {
    if btk_has systemd-detect-virt && systemd-detect-virt --container --quiet >/dev/null 2>&1; then
        return 0
    fi
    [ -e "${BTK_ROOT:-}/.dockerenv" ] || [ -e "${BTK_ROOT:-}/run/.containerenv" ]
}

# Prints the first adapter name (hci0); fails when there is none.
btk_first_adapter() {
    local entry
    for entry in "${BTK_ROOT:-}"/sys/class/bluetooth/hci*; do
        [ -e "$entry" ] || continue
        case "${entry##*/}" in *:*) continue ;; esac
        printf '%s\n' "${entry##*/}"
        return 0
    done
    return 1
}
btk_adapter_present() { btk_first_adapter >/dev/null; }

btk_state_file() { printf '%s/data/bluetooth-setup\n' "$1"; }

# Prints the stored decision (enabled|declined) or nothing.
btk_read_state() {
    local file value=""
    file="$(btk_state_file "$1")"
    [ -r "$file" ] || return 0
    value="$(head -n 1 "$file" 2>/dev/null | tr -d '[:space:]')" || value=""
    case "$value" in enabled | declined) printf '%s\n' "$value" ;; esac
    return 0
}

btk_normalize_choice() {
    case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
        yes | y | true | 1 | on | enabled) printf 'enabled\n' ;;
        no | n | false | 0 | off | declined) printf 'declined\n' ;;
    esac
    return 0
}

# btk_resolve_choice <installdir> <flag yes|no|""> <may_prompt true|false>
# Prints enabled, declined or skip. Order: container (skip), flag,
# AURAGO_BLUETOOTH, stored decision, question on the terminal. Writes nothing.
btk_resolve_choice() {
    local dir="$1" flag="${2:-}" may_prompt="${3:-false}" choice="" answer=""
    local tty_out="${BTK_TTY_OUT:-/dev/tty}"
    if btk_in_container; then
        btk_info "Bluetooth setup skipped: Bluetooth is not supported inside containers." >&2
        printf 'skip\n'
        return 0
    fi
    choice="$(btk_normalize_choice "$flag")"
    [ -n "$choice" ] || choice="$(btk_normalize_choice "${AURAGO_BLUETOOTH:-}")"
    [ -n "$choice" ] || choice="$(btk_read_state "$dir")"
    if [ -z "$choice" ] && [ "$may_prompt" = true ]; then
        if ! btk_adapter_present; then
            printf '  %s\n' "No Bluetooth adapter detected; everything is prepared for a later dongle." >> "$tty_out"
        fi
        printf '  %b?%b  %s ' "${CYAN:-}" "${NC:-}" "Use Bluetooth in AuraGo (manage devices, headphones and speakers)? [y/N]:" >> "$tty_out"
        read -r answer < "${BTK_TTY_IN:-/dev/tty}" || true
        case "$answer" in
            [yYjJ] | [yY][eE][sS] | [jJ][aA]) choice=enabled ;;
            *) choice=declined ;;
        esac
    fi
    printf '%s\n' "${choice:-skip}"
}

# btk_write_state <installdir> <enabled|declined> [service-user]
btk_write_state() {
    local dir="$1" value="$2" user="${3:-}" file parent created=false
    file="$(btk_state_file "$dir")"
    parent="${file%/*}"
    if [ ! -d "$parent" ]; then
        mkdir -p "$parent" 2>/dev/null || $SUDO mkdir -p "$parent" || return 1
        created=true
    fi
    if ! { printf '%s\n' "$value" > "$file"; } 2>/dev/null; then
        printf '%s\n' "$value" | $SUDO tee "$file" >/dev/null || return 1
    fi
    if [ -n "$user" ] && [ "$user" != "$(id -un)" ]; then
        $SUDO chown "$user" "$file" 2>/dev/null || true
        if [ "$created" = true ]; then $SUDO chown "$user" "$parent" 2>/dev/null || true; fi
    fi
    return 0
}
# <<< AURAGO-BLUETOOTH-KIT v1 <<<
