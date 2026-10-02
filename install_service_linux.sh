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

btk_pkg_manager() {
    if btk_has apt-get; then printf 'apt\n'
    elif btk_has dnf; then printf 'dnf\n'
    elif btk_has pacman; then printf 'pacman\n'
    elif btk_has zypper; then printf 'zypper\n'
    else printf 'unknown\n'
    fi
}

btk_pipewire_pulse_unit() {
    local dir
    for dir in /usr/lib/systemd/user /lib/systemd/user /etc/systemd/user; do
        if [ -e "${BTK_ROOT:-}$dir/pipewire-pulse.service" ]; then return 0; fi
    done
    return 1
}

# An installed PulseAudio server stays; everything else gets PipeWire.
btk_audio_stack() {
    if btk_has pulseaudio && ! btk_pipewire_pulse_unit; then printf 'pulseaudio\n'; else printf 'pipewire\n'; fi
}

# btk_packages <manager> <pipewire|pulseaudio> <with_ffmpeg true|false>
btk_packages() {
    local list
    case "$1:$2" in
        apt:pipewire) list="bluez pipewire pipewire-bin pipewire-pulse wireplumber libspa-0.2-bluetooth" ;;
        apt:pulseaudio) list="bluez pulseaudio-module-bluetooth pulseaudio-utils" ;;
        dnf:pipewire) list="bluez pipewire pipewire-pulseaudio pipewire-utils wireplumber" ;;
        dnf:pulseaudio) list="bluez pulseaudio-module-bluetooth pulseaudio-utils" ;;
        pacman:pipewire) list="bluez bluez-utils pipewire pipewire-pulse wireplumber" ;;
        pacman:pulseaudio) list="bluez bluez-utils pulseaudio-bluetooth libpulse" ;;
        zypper:pipewire) list="bluez pipewire pipewire-pulseaudio pipewire-tools wireplumber" ;;
        zypper:pulseaudio) list="bluez pulseaudio-module-bluetooth pulseaudio-utils" ;;
        *) return 1 ;;
    esac
    if [ "$3" = true ]; then
        if [ "$1" = dnf ]; then list="$list ffmpeg-free"; else list="$list ffmpeg"; fi
    fi
    printf '%s\n' "$list"
}

btk_pkg_installed() {
    case "$1" in
        apt) dpkg-query -W -f='${Status}' "$2" 2>/dev/null | grep -q 'ok installed' ;;
        dnf | zypper) rpm -q "$2" >/dev/null 2>&1 ;;
        pacman) pacman -Q "$2" >/dev/null 2>&1 ;;
        *) return 1 ;;
    esac
}

btk_pkg_command() {
    case "$1" in
        apt) printf 'sudo apt-get install -y\n' ;;
        dnf) printf 'sudo dnf install -y\n' ;;
        pacman) printf 'sudo pacman -S --needed\n' ;;
        zypper) printf 'sudo zypper install\n' ;;
    esac
}

# btk_pkg_add <manager> package...
btk_pkg_add() {
    local mgr="$1"
    shift
    case "$mgr" in
        apt)
            btk_run "Installing $*" $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@" && return 0
            btk_run "Refreshing the package lists" $SUDO apt-get update
            btk_run "Installing $*" $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
            ;;
        dnf) btk_run "Installing $*" $SUDO dnf install -y "$@" ;;
        pacman)
            btk_run "Installing $*" $SUDO pacman -S --needed --noconfirm "$@" && return 0
            btk_run "Installing $*" $SUDO pacman -Sy --needed --noconfirm "$@"
            ;;
        zypper) btk_run "Installing $*" $SUDO zypper --non-interactive install "$@" ;;
        *) return 1 ;;
    esac
}

# btk_install_packages <pipewire|pulseaudio>: installs only what is missing.
btk_install_packages() {
    local mgr list pkg missing="" ffmpeg=false
    mgr="$(btk_pkg_manager)"
    btk_has ffmpeg || ffmpeg=true
    if ! list="$(btk_packages "$mgr" "$1" "$ffmpeg")"; then
        btk_fail "Installing the Bluetooth packages (unknown package manager)" \
            "install bluez, pipewire, pipewire-pulse, wireplumber, the PipeWire Bluetooth plugin and ffmpeg"
        return 1
    fi
    for pkg in $list; do
        btk_pkg_installed "$mgr" "$pkg" || missing="$missing $pkg"
    done
    missing="${missing# }"
    if [ -z "$missing" ]; then
        btk_done "Bluetooth packages are installed"
        return 0
    fi
    # shellcheck disable=SC2086 # package names are single words
    if btk_pkg_add "$mgr" $missing; then
        btk_done "Installed $missing"
        return 0
    fi
    btk_fail "Installing $missing" "$(btk_pkg_command "$mgr") $missing"
    return 1
}

btk_user_uid() { id -u "$1" 2>/dev/null || true; }

btk_user_home() {
    local home=""
    if btk_has getent; then home="$(getent passwd "$1" 2>/dev/null | cut -d: -f6)" || home=""; fi
    printf '%s\n' "$home"
}

# btk_as_user <user> <uid> command...: runs inside that user's systemd and
# D-Bus session (directly, through runuser as root, or through sudo).
btk_as_user() {
    local user="$1" uid="$2" runtime
    shift 2
    runtime="/run/user/$uid"
    if [ "$(id -un)" = "$user" ]; then
        XDG_RUNTIME_DIR="$runtime" DBUS_SESSION_BUS_ADDRESS="unix:path=$runtime/bus" "$@"
    elif [ "$(id -u)" = 0 ] && btk_has runuser; then
        runuser -u "$user" -- env XDG_RUNTIME_DIR="$runtime" DBUS_SESSION_BUS_ADDRESS="unix:path=$runtime/bus" "$@"
    else
        ${SUDO:-sudo} -u "$user" env XDG_RUNTIME_DIR="$runtime" DBUS_SESSION_BUS_ADDRESS="unix:path=$runtime/bus" "$@"
    fi
}

btk_enable_bluez() {
    if ! btk_has systemctl; then
        btk_fail "Starting the Bluetooth service" "start bluetoothd with your init system"
        return 1
    fi
    if systemctl is-enabled --quiet bluetooth.service 2>/dev/null &&
        systemctl is-active --quiet bluetooth.service 2>/dev/null; then
        btk_done "Bluetooth service is running"
    elif $SUDO systemctl enable --now bluetooth.service >>"$BTK_LOG" 2>&1; then
        btk_done "Bluetooth service enabled and started"
    else
        btk_fail "Starting the Bluetooth service" "sudo systemctl enable --now bluetooth.service"
        return 1
    fi
    if btk_has rfkill && rfkill list bluetooth 2>/dev/null | grep -qi 'soft blocked: yes'; then
        $SUDO rfkill unblock bluetooth >>"$BTK_LOG" 2>&1 ||
            btk_warn "Bluetooth is switched off in software; run: sudo rfkill unblock bluetooth"
    fi
    return 0
}

# Linger keeps the user's audio session running without a login.
btk_enable_linger() {
    local user="$1" uid="$2"
    if [ ! -e "${BTK_ROOT:-}/var/lib/systemd/linger/$user" ] &&
        ! $SUDO loginctl enable-linger "$user" >>"$BTK_LOG" 2>&1; then
        btk_fail "Keeping the audio session of $user running without a login" "sudo loginctl enable-linger $user"
        return 1
    fi
    if ! systemctl is-active --quiet "user@$uid.service" 2>/dev/null &&
        ! $SUDO systemctl start "user@$uid.service" >>"$BTK_LOG" 2>&1; then
        btk_fail "Starting the user session of $user" "sudo systemctl start user@$uid.service"
        return 1
    fi
    btk_done "Audio session of $user runs without a login"
}

btk_audio_units() {
    if [ "$1" = pulseaudio ]; then
        printf 'pulseaudio.socket\n'
    else
        printf 'pipewire.socket pipewire-pulse.socket wireplumber.service\n'
    fi
}

# btk_enable_user_units <user> <uid> <pipewire|pulseaudio>
btk_enable_user_units() {
    local user="$1" uid="$2" units
    units="$(btk_audio_units "$3")"
    # shellcheck disable=SC2086 # unit names are single words
    if btk_as_user "$user" "$uid" systemctl --user enable --now $units >>"$BTK_LOG" 2>&1; then
        btk_done "Audio services enabled for $user"
        return 0
    fi
    btk_fail "Enabling the audio services for $user" \
        "sudo -u $user env XDG_RUNTIME_DIR=/run/user/$uid systemctl --user enable --now $units"
    return 1
}

btk_wireplumber_version() {
    btk_has wireplumber || return 0
    wireplumber --version 2>/dev/null | grep -Eo '[0-9]+\.[0-9]+(\.[0-9]+)?' | tail -n 1
    return 0
}

# Without a logind seat (headless servers) WirePlumber keeps its BlueZ
# monitor off; audio devices then pair but fail with
# br-connection-profile-unavailable.
btk_wireplumber_headless() {
    local user="$1" uid="$2" home version dir file content
    home="$(btk_user_home "$user")"
    version="$(btk_wireplumber_version)"
    if [ -z "$home" ] || [ -z "$version" ]; then
        btk_fail "Configuring WirePlumber for a server without a screen" "install WirePlumber, then run ./update.sh --bluetooth"
        return 1
    fi
    case "$version" in
        0.4 | 0.4.*)
            dir="$home/.config/wireplumber/bluetooth.lua.d"
            file="$dir/80-aurago-bluez-headless.lua"
            content='bluez_monitor.properties["with-logind"] = false'
            ;;
        *)
            dir="$home/.config/wireplumber/wireplumber.conf.d"
            file="$dir/80-aurago-bluez-headless.conf"
            content="$(printf '%s\n' 'wireplumber.profiles = {' '  main = {' '    monitor.bluez.seat-monitoring = disabled' '  }' '}')"
            ;;
    esac
    if [ "$(cat "$file" 2>/dev/null)" = "$content" ]; then
        btk_done "WirePlumber handles Bluetooth without a screen"
        return 0
    fi
    if btk_as_user "$user" "$uid" mkdir -p "$dir" >>"$BTK_LOG" 2>&1 &&
        printf '%s\n' "$content" | btk_as_user "$user" "$uid" tee "$file" >/dev/null 2>>"$BTK_LOG"; then
        btk_as_user "$user" "$uid" systemctl --user restart wireplumber.service >>"$BTK_LOG" 2>&1 || true
        btk_done "WirePlumber handles Bluetooth without a screen ($file)"
        return 0
    fi
    btk_fail "Writing $file" "run ./update.sh --bluetooth as $user"
    return 1
}

btk_dropin_path() { printf '%s/etc/systemd/system/%s.service.d/aurago-bluetooth.conf\n' "${BTK_ROOT:-}" "$1"; }

# btk_dropin_content <uid|""> <with_group true|false>
btk_dropin_content() {
    if [ -n "$1" ]; then printf '[Unit]\nWants=user@%s.service\nAfter=user@%s.service\n\n' "$1" "$1"; fi
    printf '[Service]\n'
    if [ -n "$1" ]; then printf 'Environment=XDG_RUNTIME_DIR=/run/user/%s\n' "$1"; fi
    if [ "$2" = true ]; then printf 'SupplementaryGroups=bluetooth\n'; fi
    return 0
}

btk_verify_unit() {
    btk_has systemd-analyze || return 0
    systemd-analyze verify "$1.service" >>"$BTK_LOG" 2>&1
}

btk_remove_dropin() {
    local path
    path="$(btk_dropin_path "$1")"
    [ -e "$path" ] || return 0
    $SUDO rm -f "$path" && $SUDO systemctl daemon-reload >>"$BTK_LOG" 2>&1
}

# btk_write_dropin <service> <user> <uid>: the AuraGo service joins the
# user's audio session. A drop-in systemd rejects is removed again.
btk_write_dropin() {
    local service="$1" user="$2" uid="$3" path tmp content group=false
    path="$(btk_dropin_path "$service")"
    if btk_has getent && getent group bluetooth >/dev/null 2>&1; then group=true; fi
    if [ "$user" = root ]; then uid=""; fi
    if [ -z "$uid" ] && [ "$group" = false ]; then
        btk_remove_dropin "$service"
        return 0
    fi
    content="$(btk_dropin_content "$uid" "$group")"
    if [ "$(cat "$path" 2>/dev/null)" = "$content" ]; then
        btk_done "AuraGo service is set up for Bluetooth"
        return 0
    fi
    tmp="$(mktemp)" || { btk_fail "Writing $path" "see documentation/bluetooth.md, Installer setup"; return 1; }
    printf '%s\n' "$content" > "$tmp"
    if ! $SUDO mkdir -p "${path%/*}" || ! $SUDO install -o root -g root -m 0644 "$tmp" "$path"; then
        rm -f "$tmp"
        btk_fail "Writing $path" "see documentation/bluetooth.md, Installer setup"
        return 1
    fi
    rm -f "$tmp"
    if $SUDO systemctl daemon-reload >>"$BTK_LOG" 2>&1 && btk_verify_unit "$service"; then
        btk_done "AuraGo service joins the audio session ($path)"
        return 0
    fi
    $SUDO rm -f "$path"
    $SUDO systemctl daemon-reload >>"$BTK_LOG" 2>&1 || true
    btk_fail "Checking $path with systemd (the file was removed again)" "sudo systemd-analyze verify $service.service"
    return 1
}

# btk_config_set <config.yaml> <key> <value>: sets bluetooth.<key>, adding
# the key or the section when missing. `cat >` keeps mode and owner.
btk_config_set() {
    local file="$1" tmp
    [ -f "$file" ] || return 1
    tmp="$(mktemp)" || return 1
    if ! awk -v key="$2" -v value="$3" '
        function emit_missing(  pad) {
            if (done) return
            pad = (indent == "" ? "    " : indent)
            print pad key ": " value
            done = 1
        }
        /^bluetooth:/ {
            insec = 1; seen = 1
            if ($0 ~ /^bluetooth:[[:space:]]*(#.*)?$/) print; else print "bluetooth:"
            next
        }
        insec && /^[^[:space:]#]/ { emit_missing(); insec = 0 }
        insec && /^[[:space:]]+[^[:space:]#]/ {
            if (indent == "") { match($0, /^[[:space:]]+/); indent = substr($0, 1, RLENGTH) }
            if (!done && index($0, indent key ":") == 1) { print indent key ": " value; done = 1; next }
        }
        { print }
        END {
            if (insec) emit_missing()
            if (!seen) { print "bluetooth:"; print "    " key ": " value }
        }
    ' "$file" > "$tmp"; then
        rm -f "$tmp"
        return 1
    fi
    if ! { cat "$tmp" > "$file"; } 2>/dev/null && ! $SUDO tee "$file" < "$tmp" >/dev/null; then
        rm -f "$tmp"
        return 1
    fi
    rm -f "$tmp"
    return 0
}

# btk_check_endpoints <user> <uid> <stack>: read-only. Headphones and speakers
# connect only after PipeWire/PulseAudio registered A2DP (0000110b) with BlueZ.
btk_check_endpoints() {
    local adapter uuids i hint
    if ! adapter="$(btk_first_adapter)"; then
        btk_info "No Bluetooth adapter detected. Once one is plugged in, AuraGo shows the Bluetooth app."
        return 0
    fi
    btk_has busctl || return 0
    if [ "$(busctl get-property org.bluez "/org/bluez/$adapter" org.bluez.Adapter1 Powered 2>/dev/null)" = "b false" ]; then
        btk_info "The Bluetooth adapter is off; turn it on in AuraGo's Bluetooth app."
        return 0
    fi
    for ((i = 0; i < ${BTK_ENDPOINT_RETRIES:-5}; i++)); do
        uuids="$(busctl get-property org.bluez "/org/bluez/$adapter" org.bluez.Adapter1 UUIDs 2>/dev/null)" || uuids=""
        case "$uuids" in
            *0000110b-* | *0000110B-*)
                btk_done "Audio devices can connect"
                return 0
                ;;
        esac
        sleep "${BTK_ENDPOINT_DELAY:-2}"
    done
    if [ "$3" = pulseaudio ]; then
        hint="sudo -u $1 env XDG_RUNTIME_DIR=/run/user/$2 pactl load-module module-bluetooth-discover"
    else
        hint="sudo -u $1 env XDG_RUNTIME_DIR=/run/user/$2 systemctl --user restart wireplumber.service"
    fi
    btk_fail "Registering the audio profiles with BlueZ" "$hint"
    return 1
}

btk_summary() {
    local entry kind=ok title="BLUETOOTH READY"
    local -a lines=()
    for entry in "${BTK_DONE[@]}"; do lines+=("${T_OK:-+} $entry"); done
    for entry in "${BTK_FAILED[@]}"; do lines+=("${T_WARN:-!} ${entry%%|*}" "    run: ${entry#*|}"); done
    if [ "${#BTK_FAILED[@]}" -gt 0 ]; then
        kind=warn
        title="BLUETOOTH NEEDS ATTENTION"
        lines+=("Details: $BTK_LOG")
    fi
    if declare -F tui_box >/dev/null; then
        tui_box "$kind" "$title" "${lines[@]}"
    else
        printf '\n== %s ==\n' "$title"
        printf '  %s\n' "${lines[@]}"
    fi
    return 0
}

_btk_apply() {
    local dir="$1" user="$2" service="${3:-}" previous uid stack
    BTK_DONE=()
    BTK_FAILED=()
    { : >>"$BTK_LOG"; } 2>/dev/null || BTK_LOG=/dev/null
    btk_info "Preparing this server for Bluetooth (details: $BTK_LOG)"
    previous="$(btk_read_state "$dir")"
    # Stored first: a failed step is retried by the next update, not asked again.
    btk_write_state "$dir" enabled "$user" || btk_warn "Could not save the Bluetooth choice to $(btk_state_file "$dir")."
    uid="$(btk_user_uid "$user")"
    if [ -z "$uid" ]; then
        btk_fail "Finding the service user $user" "id $user"
        btk_summary
        return 0
    fi
    stack="$(btk_audio_stack)"
    btk_install_packages "$stack"
    btk_enable_bluez
    if [ "$user" = root ]; then
        btk_warn "AuraGo runs as root. Headphones and speakers need a regular user account; device management still works."
    elif btk_enable_linger "$user" "$uid" && btk_enable_user_units "$user" "$uid" "$stack" && [ "$stack" = pipewire ]; then
        btk_wireplumber_headless "$user" "$uid"
    fi
    if [ -n "$service" ] && [ -e "${BTK_ROOT:-}/etc/systemd/system/$service.service" ]; then
        btk_write_dropin "$service" "$user" "$uid"
    fi
    # config.yaml changes only with a new decision; afterwards the Config page owns it.
    if [ "$previous" != enabled ]; then
        if btk_config_set "$dir/config.yaml" enabled true && btk_config_set "$dir/config.yaml" allow_playback true; then
            btk_done "config.yaml: Bluetooth and playback turned on"
        else
            btk_fail "Turning on Bluetooth in $dir/config.yaml" "set bluetooth.enabled: true and bluetooth.allow_playback: true"
        fi
    fi
    if [ "$user" != root ]; then btk_check_endpoints "$user" "$uid" "$stack"; fi
    btk_summary
    return 0
}

# btk_apply <installdir> <service-user> <service-name|"">: never fails the caller.
btk_apply() { ( trap - ERR; set +e +u +o pipefail; _btk_apply "$@" ); return 0; }

_btk_decline() {
    local dir="$1" service="${2:-}" user="${3:-}"
    # A stored "no" is final until the answer changes.
    [ "$(btk_read_state "$dir")" != declined ] || return 0
    btk_write_state "$dir" declined "$user" || btk_warn "Could not save the Bluetooth choice to $(btk_state_file "$dir")."
    if [ -n "$service" ]; then
        btk_remove_dropin "$service" || btk_warn "Could not remove $(btk_dropin_path "$service")."
    fi
    if [ -f "$dir/config.yaml" ] && ! btk_config_set "$dir/config.yaml" enabled false; then
        btk_warn "Could not set bluetooth.enabled: false in $dir/config.yaml."
    fi
    btk_info "Bluetooth stays off in AuraGo. Turn it on later with: ./update.sh --bluetooth"
    return 0
}

# btk_decline <installdir> <service-name|""> [service-user]: never fails the caller.
btk_decline() { ( trap - ERR; set +e +u +o pipefail; _btk_decline "$@" ); return 0; }

# btk_run_choice <enabled|declined|skip> <installdir> <service-user> <service-name|"">
btk_run_choice() {
    case "$1" in
        enabled) btk_apply "$2" "$3" "${4:-}" ;;
        declined) btk_decline "$2" "${4:-}" "$3" ;;
    esac
    return 0
}
# <<< AURAGO-BLUETOOTH-KIT v1 <<<

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
    [[ "$group_id" -gt 0 ]] || return 1
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
        [[ -n "$group_id" ]] || continue
        duplicate=false
        for existing in "${ids[@]}"; do
            if [[ "$existing" == "$group_id" ]]; then
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
    if [[ ${#groups[@]} -gt 0 ]]; then
        local joined
        joined="${groups[*]}"
        printf 'SupplementaryGroups=%s' "$joined"
    fi
}

systemd_serial_groups_line() {
    local groups=() group_name
    for group_name in dialout uucp; do
        if system_group_exists "$group_name"; then
            groups+=("$group_name")
        fi
    done
    if [ "${#groups[@]}" -gt 0 ]; then
        printf 'SupplementaryGroups=%s' "${groups[*]}"
    fi
}

systemd_escape_path_value() {
    local value="$1"
    local escaped=""
    local char
    local index
    for ((index = 0; index < ${#value}; index++)); do
        char="${value:index:1}"
        case "$char" in
            " ") escaped+="\\x20" ;;
            $'\t') escaped+="\\x09" ;;
            '"') escaped+="\\x22" ;;
            "'") escaped+="\\x27" ;;
            "\\") escaped+="\\x5c" ;;
            "%") escaped+="%%" ;;
            $'\r'|$'\n') return 1 ;;
            *) escaped+="$char" ;;
        esac
    done
    printf '%s' "$escaped"
}

warn_if_systemd_hardening_conflicts() {
    local config_path="$1"
    [[ -f "$config_path" ]] || return 0
    if grep -Eq '^[[:space:]]+sudo_enabled:[[:space:]]*true([[:space:]]|$)' "$config_path"; then
        warn "config.yaml enables sudo features. The generated unit will allow privilege escalation."
    fi
    if grep -Eq '^[[:space:]]+sudo_unrestricted:[[:space:]]*true([[:space:]]|$)' "$config_path"; then
        warn "config.yaml enables sudo_unrestricted. The generated unit will not use ProtectSystem=strict."
    fi
}

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

# Ensure the binary is executable (Windows git push often loses the +x bit)
chmod +x "$BINARY_PATH" 2>/dev/null || true
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

warn_if_systemd_hardening_conflicts "$CONFIG_PATH"

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
GPU_GROUPS_LINE="$(systemd_gpu_groups_line)"
SERIAL_GROUPS_LINE="$(systemd_serial_groups_line)"
if [ -n "$SERIAL_GROUPS_LINE" ]; then
    info "Granting the service USB serial access: ${SERIAL_GROUPS_LINE#SupplementaryGroups=}"
fi
GPU_GROUP_IDS="$(system_gpu_group_ids)"
GPU_GROUP_IDS_LINE=""
NO_NEW_PRIVILEGES_LINE="NoNewPrivileges=true"
PROTECT_SYSTEM_LINE="ProtectSystem=strict"
if grep -Eq '^[[:space:]]+sudo_enabled:[[:space:]]*true([[:space:]]|$)' "$CONFIG_PATH"; then
    NO_NEW_PRIVILEGES_LINE="# NoNewPrivileges=true disabled because sudo_enabled is enabled"
fi
if grep -Eq '^[[:space:]]+sudo_unrestricted:[[:space:]]*true([[:space:]]|$)' "$CONFIG_PATH"; then
    PROTECT_SYSTEM_LINE="# ProtectSystem=strict disabled because sudo_unrestricted is enabled"
fi
if [[ -n "$GPU_GROUPS_LINE" ]]; then
    info "Granting the service access to available GPU groups: ${GPU_GROUPS_LINE#SupplementaryGroups=}"
fi
if [[ -n "$GPU_GROUP_IDS" ]]; then
    GPU_GROUP_IDS_LINE="Environment=\"AURAGO_GPU_GROUP_IDS=${GPU_GROUP_IDS}\""
    info "Forwarding host GPU group IDs to managed containers: ${GPU_GROUP_IDS}"
fi
SYSTEMD_INSTALL_DIR="$(systemd_escape_path_value "$INSTALL_DIR")" || error "Install path contains unsupported control characters."
SYSTEMD_BINARY_PATH="$(systemd_escape_path_value "$BINARY_PATH")" || error "Binary path contains unsupported control characters."
SYSTEMD_CONFIG_PATH="$(systemd_escape_path_value "$CONFIG_PATH")" || error "Config path contains unsupported control characters."
SYSTEMD_CREDENTIAL_FILE="$(systemd_escape_path_value "$CREDENTIAL_FILE")" || error "Credential path contains unsupported control characters."
SYSTEMD_CREDENTIAL_DIR="$(systemd_escape_path_value "$CREDENTIAL_DIR")" || error "Credential directory contains unsupported control characters."
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
${GPU_GROUPS_LINE}
${SERIAL_GROUPS_LINE}
${GPU_GROUP_IDS_LINE}
WorkingDirectory=${SYSTEMD_INSTALL_DIR}
ExecStart=${SYSTEMD_BINARY_PATH} --config ${SYSTEMD_CONFIG_PATH}
Restart=always
RestartSec=5
TimeoutStopSec=60s
EnvironmentFile=${SYSTEMD_CREDENTIAL_FILE}
StandardOutput=append:${SYSTEMD_INSTALL_DIR}/log/aurago.log
StandardError=append:${SYSTEMD_INSTALL_DIR}/log/aurago.err

# Allow binding privileged ports (80, 443) without root.
# Compatible with NoNewPrivileges — systemd sets the capability before the prctl call.
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Security hardening
${NO_NEW_PRIVILEGES_LINE}
${PROTECT_SYSTEM_LINE}
ReadWritePaths=${SYSTEMD_INSTALL_DIR} ${SYSTEMD_CREDENTIAL_DIR}
ProtectHome=read-only
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

# Ensure log directory exists
mkdir -p "${INSTALL_DIR}/log"
chown -R "${SUDO_USER:-root}:$(id -gn "${SUDO_USER:-root}")" "${INSTALL_DIR}/log"

# 5. Reload systemd and enable service
if command -v systemd-analyze >/dev/null 2>&1; then
    systemd-analyze verify "${SERVICE_FILE}" || error "Generated systemd unit failed validation."
fi
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
