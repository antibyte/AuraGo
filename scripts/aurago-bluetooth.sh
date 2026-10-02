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

# btk_pkg_install <manager> package...
btk_pkg_install() {
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
    if btk_pkg_install "$mgr" $missing; then
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
# <<< AURAGO-BLUETOOTH-KIT v1 <<<
