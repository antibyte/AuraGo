package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var (
	bluetoothKitBegin = regexp.MustCompile(`(?m)^# >>> AURAGO-BLUETOOTH-KIT[^\n]*\n`)
	bluetoothKitEnd   = regexp.MustCompile(`(?m)^# <<< AURAGO-BLUETOOTH-KIT[^\n]*\n`)
)

// extractBluetoothKit returns the kit block including both marker lines and
// fails unless exactly one marker pair exists.
func extractBluetoothKit(t *testing.T, name, script string) string {
	t.Helper()
	begins := bluetoothKitBegin.FindAllStringIndex(script, -1)
	ends := bluetoothKitEnd.FindAllStringIndex(script, -1)
	if len(begins) != 1 || len(ends) != 1 {
		t.Fatalf("%s must contain exactly one AURAGO-BLUETOOTH-KIT marker pair, found %d begin / %d end", name, len(begins), len(ends))
	}
	if ends[0][0] < begins[0][0] {
		t.Fatalf("%s has its AURAGO-BLUETOOTH-KIT end marker before the begin marker", name)
	}
	return script[begins[0][0]:ends[0][1]]
}

func findBash() (string, bool) {
	bash := "bash"
	if runtime.GOOS == "windows" {
		bash = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
	}
	_, err := exec.LookPath(bash)
	return bash, err == nil
}

func bluetoothKitShell(t *testing.T) string {
	t.Helper()
	bash, ok := findBash()
	if !ok {
		t.Skip("bash is unavailable")
	}
	return bash
}

// bluetoothKitPrelude gives every scenario a scratch root ($BTK_ROOT), an
// install dir ($D) and stub system commands that append "name args" to
// $STUB_LOG. Stub behavior is steered by STUB_* variables.
const bluetoothKitPrelude = `set -euo pipefail
trap 'echo "scenario failed at line $LINENO: $BASH_COMMAND" >&2' ERR
T="$(mktemp -d)"
trap 'rm -rf -- "$T"' EXIT
mkdir -p "$T/bin" "$T/root/etc/systemd/system" "$T/root/sys/class/bluetooth" "$T/home/svc" "$T/aurago/data"
export STUB_LOG="$T/calls.log" STUB_HOME="$T/home/svc"
: > "$STUB_LOG"
stub() {
    { printf '#!/bin/sh\n'; printf 'echo "%s $*" >> "$STUB_LOG"\n' "$1"; printf '%s\n' "${2:-exit 0}"; } > "$T/bin/$1"
    chmod +x "$T/bin/$1"
}
for c in loginctl rfkill apt-get dnf pacman zypper rpm chown runuser; do stub "$c"; done
stub sudo 'while [ $# -gt 0 ]; do case "$1" in -n) shift ;; -u) shift 2 ;; *) break ;; esac; done; exec "$@"'
stub systemctl '[ -z "${XDG_RUNTIME_DIR:-}" ] || echo "  xdg=$XDG_RUNTIME_DIR" >> "$STUB_LOG"
case "$1" in is-enabled|is-active) exit "${STUB_ACTIVE_RC:-1}" ;; esac'
stub systemd-analyze 'exit "${STUB_VERIFY_RC:-0}"'
stub systemd-detect-virt 'exit "${STUB_CONTAINER_RC:-1}"'
stub install 'for a; do prev="${last:-}"; last="$a"; done; cp "$prev" "$last"'
stub id 'case "$#:$1" in
1:-u) echo "${STUB_UID:-1001}" ;;
1:-un) echo "${STUB_USER:-svc}" ;;
2:-u) if [ "$2" = root ]; then echo 0; else echo "${STUB_SVC_UID:-1001}"; fi ;;
*) exit 1 ;;
esac'
stub getent 'case "$1" in
passwd) echo "$2:x:1001:1001::$STUB_HOME:/bin/bash" ;;
group) case " ${STUB_GROUPS:-} " in *" $2 "*) echo "$2:x:112:" ;; *) exit 2 ;; esac ;;
esac'
stub wireplumber 'echo "Compiled with libwireplumber ${STUB_WP_VERSION:-0.5.13}"; echo "Linked with libwireplumber ${STUB_WP_VERSION:-0.5.13}"'
stub busctl 'case "$*" in *Powered*) echo "b ${STUB_POWERED:-true}" ;; *UUIDs*) echo "as 2 ${STUB_UUIDS:-}" ;; esac'
stub dpkg-query 'for a; do p="$a"; done; case " ${STUB_INSTALLED:-} " in *" $p "*) printf "install ok installed" ;; *) exit 1 ;; esac'
export PATH="$T/bin:$PATH" BTK_ROOT="$T/root" BTK_LOG="$T/bt.log" BTK_ENDPOINT_RETRIES=2 BTK_ENDPOINT_DELAY=0
unset XDG_RUNTIME_DIR DBUS_SESSION_BUS_ADDRESS AURAGO_BLUETOOTH BTK_TTY_IN BTK_TTY_OUT
SUDO=""
D="$T/aurago"
called() { grep -Fxq -- "$1" "$STUB_LOG" || { echo "missing call: $1"; cat "$STUB_LOG"; exit 1; }; }
not_called() { if grep -Fq -- "$1" "$STUB_LOG"; then echo "unexpected call: $1"; cat "$STUB_LOG"; exit 1; fi; }
lacks() { if grep -Fq -- "$1" "$2"; then echo "unexpected text in $2: $1"; cat "$2"; exit 1; fi; }
# Steps are written for btk_apply's relaxed shell; run them the same way.
step() { ( set +e +u +o pipefail; "$@" ); }
write_config() {
    printf '%s\n' 'bluetooth:' '    enabled: true' '    readonly: true' '    allow_playback: false' 'server:' '    host: 127.0.0.1' > "$D/config.yaml"
}
`

// bluetoothKitHave replaces command detection so each scenario decides which
// commands exist, independent of the machine running the test.
const bluetoothKitHave = `
HAVE=""
btk_has() { case " $HAVE " in *" $1 "*) return 0 ;; esac; return 1; }
`

// runBashScript feeds the script through stdin: on Windows, MSYS bash
// re-parses a long -c argument and breaks on quoted glob characters.
func runBashScript(t *testing.T, bash, script string) (string, error) {
	t.Helper()
	cmd := exec.Command(bash, "-s")
	cmd.Stdin = strings.NewReader(script)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func runBluetoothKit(t *testing.T, scenario string) string {
	t.Helper()
	bash := bluetoothKitShell(t)
	kit := extractBluetoothKit(t, "scripts/aurago-bluetooth.sh", readRepoFile(t, "scripts/aurago-bluetooth.sh"))
	output, err := runBashScript(t, bash, bluetoothKitPrelude+kit+bluetoothKitHave+scenario)
	if err != nil {
		t.Fatalf("bluetooth kit scenario failed: %v\n%s", err, output)
	}
	return output
}

func TestBluetoothKitResolvesChoiceInOrder(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
# No flag, no environment, no stored answer, no terminal: skip, write nothing.
[ "$(btk_resolve_choice "$D" "" false)" = skip ]
[ "$(btk_resolve_choice "$D")" = skip ]
[ ! -e "$D/data/bluetooth-setup" ]

printf 'declined\n' > "$D/data/bluetooth-setup"
[ "$(btk_read_state "$D")" = declined ]
[ "$(btk_resolve_choice "$D" "" false)" = declined ]
[ "$(AURAGO_BLUETOOTH=yes btk_resolve_choice "$D" "" false)" = enabled ]
[ "$(AURAGO_BLUETOOTH=yes btk_resolve_choice "$D" no false)" = declined ]
[ "$(btk_resolve_choice "$D" yes false)" = enabled ]
printf 'garbage\n' > "$D/data/bluetooth-setup"
[ -z "$(btk_read_state "$D")" ]
rm "$D/data/bluetooth-setup"

# The question goes to the terminal; Enter means no.
export BTK_TTY_IN="$T/tty.in" BTK_TTY_OUT="$T/tty.out"
printf 'y\n' > "$BTK_TTY_IN"
[ "$(btk_resolve_choice "$D" "" true)" = enabled ]
grep -Fq 'Use Bluetooth in AuraGo (manage devices, headphones and speakers)? [y/N]' "$BTK_TTY_OUT"
grep -Fq 'No Bluetooth adapter detected; everything is prepared for a later dongle.' "$BTK_TTY_OUT"
[ ! -e "$D/data/bluetooth-setup" ]
: > "$BTK_TTY_OUT"
printf '\n' > "$BTK_TTY_IN"
touch "$BTK_ROOT/sys/class/bluetooth/hci0"
[ "$(btk_resolve_choice "$D" "" true)" = declined ]
lacks 'No Bluetooth adapter detected' "$BTK_TTY_OUT"

# Containers are never asked, whatever the flag says.
touch "$BTK_ROOT/.dockerenv"
[ "$(btk_resolve_choice "$D" yes true)" = skip ]
rm "$BTK_ROOT/.dockerenv"
HAVE="systemd-detect-virt"
export STUB_CONTAINER_RC=0
[ "$(btk_resolve_choice "$D" yes true)" = skip ]
`)
}

func TestBluetoothKitStoresDecision(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
btk_write_state "$D" enabled svc
[ "$(btk_read_state "$D")" = enabled ]
not_called "chown"
btk_write_state "$T/fresh" declined other
[ "$(btk_read_state "$T/fresh")" = declined ]
called "chown other $T/fresh/data/bluetooth-setup"
called "chown other $T/fresh/data"
`)
}

func TestBluetoothKitInstallsOnlyMissingPackages(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
[ "$(btk_packages apt pipewire false)" = "bluez pipewire pipewire-bin pipewire-pulse wireplumber libspa-0.2-bluetooth" ]
[ "$(btk_packages apt pulseaudio true)" = "bluez pulseaudio-module-bluetooth pulseaudio-utils ffmpeg" ]
[ "$(btk_packages dnf pipewire true)" = "bluez pipewire pipewire-pulseaudio pipewire-utils wireplumber ffmpeg-free" ]
[ "$(btk_packages pacman pulseaudio true)" = "bluez bluez-utils pulseaudio-bluetooth libpulse ffmpeg" ]
[ "$(btk_packages zypper pipewire false)" = "bluez pipewire pipewire-pulseaudio pipewire-tools wireplumber" ]
if btk_packages unknown pipewire false >/dev/null; then exit 1; fi

# An installed PulseAudio stays unless PipeWire already serves Pulse clients.
HAVE="pulseaudio"
[ "$(btk_audio_stack)" = pulseaudio ]
mkdir -p "$BTK_ROOT/usr/lib/systemd/user"
touch "$BTK_ROOT/usr/lib/systemd/user/pipewire-pulse.service"
[ "$(btk_audio_stack)" = pipewire ]
HAVE=""
[ "$(btk_audio_stack)" = pipewire ]

# Only missing packages; apt refreshes its lists once after a failed install.
HAVE="apt-get ffmpeg"
export STUB_INSTALLED="bluez pipewire"
stub apt-get 'case "$1" in install) [ -e "$STUB_LOG.updated" ] ;; update) touch "$STUB_LOG.updated" ;; esac'
step btk_install_packages pipewire
called "apt-get install -y pipewire-bin pipewire-pulse wireplumber libspa-0.2-bluetooth"
called "apt-get update"

# Everything present: the package manager is not called at all.
: > "$STUB_LOG"
export STUB_INSTALLED="bluez pipewire pipewire-bin pipewire-pulse wireplumber libspa-0.2-bluetooth"
step btk_install_packages pipewire
not_called "apt-get"

# Unknown package manager: reported with a manual hint, never fatal.
HAVE=""
if step btk_install_packages pipewire > "$T/out" 2>&1; then exit 1; fi
grep -Fq 'unknown package manager' "$T/out"
`)
}

func TestBluetoothKitPreparesAudioSession(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
HAVE="systemctl rfkill getent wireplumber"
stub rfkill 'case "$1" in list) echo "0: hci0: Bluetooth"; echo "        Soft blocked: yes" ;; esac'
step btk_enable_bluez
called "systemctl enable --now bluetooth.service"
called "rfkill unblock bluetooth"

step btk_enable_linger svc 1001
called "loginctl enable-linger svc"
called "systemctl start user@1001.service"

step btk_enable_user_units svc 1001 pipewire
called "systemctl --user enable --now pipewire.socket pipewire-pulse.socket wireplumber.service"
called "  xdg=/run/user/1001"

step btk_wireplumber_headless svc 1001
f="$STUB_HOME/.config/wireplumber/wireplumber.conf.d/80-aurago-bluez-headless.conf"
grep -Fxq 'wireplumber.profiles = {' "$f"
grep -Fxq '    monitor.bluez.seat-monitoring = disabled' "$f"
called "systemctl --user restart wireplumber.service"

# A prepared host needs no root commands and no WirePlumber restart.
: > "$STUB_LOG"
export STUB_ACTIVE_RC=0
mkdir -p "$BTK_ROOT/var/lib/systemd/linger"
touch "$BTK_ROOT/var/lib/systemd/linger/svc"
stub rfkill 'case "$1" in list) echo "0: hci0: Bluetooth"; echo "        Soft blocked: no" ;; esac'
step btk_enable_bluez
step btk_enable_linger svc 1001
step btk_wireplumber_headless svc 1001
not_called "systemctl enable --now bluetooth.service"
not_called "rfkill unblock"
not_called "loginctl"
not_called "systemctl start"
not_called "systemctl --user restart"

# WirePlumber 0.4 reads Lua fragments instead.
export STUB_WP_VERSION=0.4.17
step btk_wireplumber_headless svc 1001
grep -Fxq 'bluez_monitor.properties["with-logind"] = false' "$STUB_HOME/.config/wireplumber/bluetooth.lua.d/80-aurago-bluez-headless.lua"

# PulseAudio keeps its own socket unit.
step btk_enable_user_units svc 1001 pulseaudio
called "systemctl --user enable --now pulseaudio.socket"

# Root reaches another user's session through runuser.
: > "$STUB_LOG"
export STUB_USER=root STUB_UID=0
HAVE="$HAVE runuser"
step btk_enable_user_units svc 1001 pipewire
called "runuser -u svc -- env XDG_RUNTIME_DIR=/run/user/1001 DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus systemctl --user enable --now pipewire.socket pipewire-pulse.socket wireplumber.service"
`)
}

func TestBluetoothKitServiceDropIn(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
HAVE="getent systemd-analyze"
d="$BTK_ROOT/etc/systemd/system/aurago.service.d/aurago-bluetooth.conf"
export STUB_GROUPS="bluetooth"
step btk_write_dropin aurago svc 1001
[ "$(cat "$d")" = "$(printf '%s\n' '[Unit]' 'Wants=user@1001.service' 'After=user@1001.service' '' '[Service]' 'Environment=XDG_RUNTIME_DIR=/run/user/1001' 'SupplementaryGroups=bluetooth')" ]
called "systemctl daemon-reload"
called "systemd-analyze verify aurago.service"

# The group line exists only when the group does; the account is never changed.
export STUB_GROUPS=""
step btk_write_dropin aurago svc 1001
lacks 'SupplementaryGroups' "$d"

# root has no audio session: only the group, or no drop-in at all.
export STUB_GROUPS="bluetooth"
step btk_write_dropin aurago root 0
[ "$(cat "$d")" = "$(printf '%s\n' '[Service]' 'SupplementaryGroups=bluetooth')" ]
export STUB_GROUPS=""
step btk_write_dropin aurago root 0
[ ! -e "$d" ]

# A drop-in systemd rejects is removed so the service always starts.
export STUB_VERIFY_RC=1
: > "$STUB_LOG"
if step btk_write_dropin aurago svc 1001; then exit 1; fi
[ ! -e "$d" ]
[ "$(grep -c '^systemctl daemon-reload$' "$STUB_LOG")" = 2 ]
`)
}

func TestBluetoothKitEditsConfig(t *testing.T) {
	t.Parallel()
	scenario := `
c="$D/config.yaml"
write_config
step btk_config_set "$c" enabled false
step btk_config_set "$c" allow_playback true
[ "$(cat "$c")" = "$(printf '%s\n' 'bluetooth:' '    enabled: false' '    readonly: true' '    allow_playback: true' 'server:' '    host: 127.0.0.1')" ]

# A missing key is added with the section's own indentation.
printf '%s\n' 'bluetooth:' '  readonly: true' 'server:' '  host: 0.0.0.0' > "$c"
step btk_config_set "$c" enabled true
[ "$(cat "$c")" = "$(printf '%s\n' 'bluetooth:' '  readonly: true' '  enabled: true' 'server:' '  host: 0.0.0.0')" ]

# Same key names in other sections stay untouched; a missing section is appended.
printf '%s\n' 'server:' '    host: 0.0.0.0' '    enabled: false' > "$c"
step btk_config_set "$c" enabled true
[ "$(cat "$c")" = "$(printf '%s\n' 'server:' '    host: 0.0.0.0' '    enabled: false' 'bluetooth:' '    enabled: true')" ]

# An empty inline section becomes a block section.
printf '%s\n' 'bluetooth: {}' 'server:' '    host: 0.0.0.0' > "$c"
step btk_config_set "$c" enabled true
[ "$(cat "$c")" = "$(printf '%s\n' 'bluetooth:' '    enabled: true' 'server:' '    host: 0.0.0.0')" ]

# The section at the end of the file.
printf '%s\n' 'bluetooth:' '    enabled: true' > "$c"
step btk_config_set "$c" allow_playback true
[ "$(cat "$c")" = "$(printf '%s\n' 'bluetooth:' '    enabled: true' '    allow_playback: true')" ]

if step btk_config_set "$D/missing.yaml" enabled true; then exit 1; fi
`
	if runtime.GOOS != "windows" {
		scenario += `
write_config
chmod 600 "$c"
step btk_config_set "$c" enabled true
[ "$(stat -c %a "$c")" = 600 ]
`
	}
	runBluetoothKit(t, scenario)
}
