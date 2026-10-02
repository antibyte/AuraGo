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

func TestBluetoothKitApplyPreparesPipeWireHost(t *testing.T) {
	t.Parallel()
	out := runBluetoothKit(t, `
HAVE="apt-get systemctl getent wireplumber busctl systemd-analyze ffmpeg"
export STUB_INSTALLED="bluez pipewire pipewire-bin pipewire-pulse wireplumber libspa-0.2-bluetooth"
export STUB_GROUPS="bluetooth" STUB_UUIDS='"0000110a-0000-1000-8000-00805f9b34fb" "0000110b-0000-1000-8000-00805f9b34fb"'
touch "$BTK_ROOT/sys/class/bluetooth/hci0" "$BTK_ROOT/etc/systemd/system/aurago.service"
write_config
btk_apply "$D" svc aurago
called "systemctl enable --now bluetooth.service"
called "loginctl enable-linger svc"
called "systemctl --user enable --now pipewire.socket pipewire-pulse.socket wireplumber.service"
[ -f "$STUB_HOME/.config/wireplumber/wireplumber.conf.d/80-aurago-bluez-headless.conf" ]
grep -Fxq 'Environment=XDG_RUNTIME_DIR=/run/user/1001' "$BTK_ROOT/etc/systemd/system/aurago.service.d/aurago-bluetooth.conf"
grep -Fxq '    enabled: true' "$D/config.yaml"
grep -Fxq '    allow_playback: true' "$D/config.yaml"
[ "$(cat "$D/data/bluetooth-setup")" = enabled ]
not_called "apt-get"

# Repair runs leave config.yaml to the Config page.
sed -i 's/allow_playback: true/allow_playback: false/' "$D/config.yaml"
btk_apply "$D" svc aurago
grep -Fxq '    allow_playback: false' "$D/config.yaml"
echo "after-apply"
`)
	for _, want := range []string{"Audio devices can connect", "BLUETOOTH READY", "after-apply"} {
		if !strings.Contains(out, want) {
			t.Fatalf("apply output lacks %q:\n%s", want, out)
		}
	}
}

func TestBluetoothKitApplyForRootSkipsAudio(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
HAVE="apt-get systemctl getent systemd-analyze ffmpeg"
export STUB_INSTALLED="bluez pipewire pipewire-bin pipewire-pulse wireplumber libspa-0.2-bluetooth" STUB_GROUPS="bluetooth"
touch "$BTK_ROOT/etc/systemd/system/aurago.service"
write_config
btk_apply "$D" root aurago > "$T/out" 2>&1
not_called "loginctl"
not_called "systemctl --user"
[ "$(cat "$BTK_ROOT/etc/systemd/system/aurago.service.d/aurago-bluetooth.conf")" = "$(printf '%s\n' '[Service]' 'SupplementaryGroups=bluetooth')" ]
grep -Fq 'AuraGo runs as root' "$T/out"
grep -Fq 'BLUETOOTH READY' "$T/out"
`)
}

func TestBluetoothKitApplyNeverAbortsTheCaller(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
HAVE="apt-get systemctl getent wireplumber busctl systemd-analyze"
for c in apt-get loginctl systemctl systemd-analyze busctl wireplumber install; do stub "$c" 'exit 1'; done
touch "$BTK_ROOT/sys/class/bluetooth/hci0" "$BTK_ROOT/etc/systemd/system/aurago.service"
btk_apply "$D" svc aurago > "$T/out" 2>&1
grep -Fq 'BLUETOOTH NEEDS ATTENTION' "$T/out"
grep -Fq 'sudo systemctl enable --now bluetooth.service' "$T/out"
grep -Fq 'sudo loginctl enable-linger svc' "$T/out"
[ ! -e "$BTK_ROOT/etc/systemd/system/aurago.service.d/aurago-bluetooth.conf" ]
# The decision is kept so the next update retries instead of asking again.
[ "$(cat "$D/data/bluetooth-setup")" = enabled ]
echo survived
`)
}

func TestBluetoothKitDeclineIsFinal(t *testing.T) {
	t.Parallel()
	runBluetoothKit(t, `
HAVE="getent"
d="$BTK_ROOT/etc/systemd/system/aurago.service.d/aurago-bluetooth.conf"
mkdir -p "${d%/*}"
printf '[Service]\nSupplementaryGroups=bluetooth\n' > "$d"
write_config
printf 'enabled\n' > "$D/data/bluetooth-setup"
btk_run_choice declined "$D" svc aurago
[ ! -e "$d" ]
called "systemctl daemon-reload"
grep -Fxq '    enabled: false' "$D/config.yaml"
grep -Fxq '    allow_playback: false' "$D/config.yaml"
[ "$(cat "$D/data/bluetooth-setup")" = declined ]

# A stored "no" changes nothing on later runs, even after manual changes.
sed -i 's/enabled: false/enabled: true/' "$D/config.yaml"
: > "$STUB_LOG"
btk_run_choice declined "$D" svc aurago
btk_run_choice skip "$D" svc aurago
grep -Fxq '    enabled: true' "$D/config.yaml"
[ ! -s "$STUB_LOG" ]
`)
}

// The installers stay single-file (curl | bash, release assets), so the kit
// is embedded. Fix drift with: bash scripts/sync-bluetooth-kit.sh
func TestBluetoothKitIsEmbeddedVerbatim(t *testing.T) {
	t.Parallel()
	source := extractBluetoothKit(t, "scripts/aurago-bluetooth.sh", readRepoFile(t, "scripts/aurago-bluetooth.sh"))
	bash, haveBash := findBash()
	for _, path := range []string{"install.sh", "update.sh", "install_service_linux.sh"} {
		script := readRepoFile(t, path)
		if got := extractBluetoothKit(t, path, script); got != source {
			t.Fatalf("%s embeds a stale Bluetooth kit; run `bash scripts/sync-bluetooth-kit.sh`", path)
		}
		if !haveBash {
			continue
		}
		cmd := exec.Command(bash, "-n")
		cmd.Stdin = strings.NewReader(script)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s has a bash syntax error: %v\n%s", path, err, output)
		}
	}
}

// requireInOrder fails unless every snippet occurs after the previous one.
func requireInOrder(t *testing.T, name, source string, snippets ...string) {
	t.Helper()
	offset := 0
	for _, snippet := range snippets {
		i := strings.Index(source[offset:], snippet)
		if i < 0 {
			t.Fatalf("%s must contain, after the previous call site:\n%s", name, snippet)
		}
		offset += i + len(snippet)
	}
}

func TestInstallShPreparesBluetooth(t *testing.T) {
	t.Parallel()
	requireInOrder(t, "install.sh", readRepoFile(t, "install.sh"),
		"# >>> AURAGO-BLUETOOTH-KIT",
		"ensure_docker_engine\n",
		`BT_CHOICE="$(btk_resolve_choice "$INSTALL_DIR" "" "$INTERACTIVE_TTY")"`,
		"SERVICE_ENABLED=1\n        btk_run_choice \"$BT_CHOICE\" \"$INSTALL_DIR\" \"$SERVICE_USER\" \"$SYSTEMD_SERVICE\"\n        $SUDO systemctl start \"$SYSTEMD_SERVICE\"",
		"if [ \"$SERVICE_INSTALLED\" != \"true\" ]; then\n    btk_run_choice \"$BT_CHOICE\" \"$INSTALL_DIR\" \"${SUDO_USER:-$(id -un)}\" \"\"\nfi",
		`section "Done"`,
	)
}

func TestUpdateShAppliesBluetoothChoice(t *testing.T) {
	t.Parallel()
	source := readRepoFile(t, "update.sh")
	requireInOrder(t, "update.sh", source,
		"--bluetooth)    BT_FLAG=yes ;;",
		"--no-bluetooth) BT_FLAG=no ;;",
		"# >>> AURAGO-BLUETOOTH-KIT",
		`BT_STORED="$(btk_read_state "$DIR")"`,
		`BT_CHOICE="$(btk_resolve_choice "$DIR" "$BT_FLAG" "$_bt_may_prompt")"`,
		"no files or services were changed.\"\n        bluetooth_apply_without_update\n        exit 0",
		"no files or services were changed.\"\n            bluetooth_apply_without_update\n            exit 0",
		"bluetooth_apply_choice\n\n# Keep systemd's stop deadline",
		"# ── Service restart",
	)

	bash := bluetoothKitShell(t)
	start := strings.Index(source, "bluetooth_service_user() {")
	end := strings.Index(source, "\n# Asked once; the answer lives in data/bluetooth-setup.")
	if start < 0 || end < start {
		t.Fatal("update.sh must define the Bluetooth helpers before resolving the choice")
	}
	script := `set -euo pipefail
log=""
btk_run_choice() { log="$log $1"; }
stat_owner() { echo svc; }
systemctl() { return 1; }
confirm() { return 0; }
info() { :; }
warn() { :; }
NO_RESTART=false
DIR=/x
SUDO=""
` + source[start:end] + `
# Unchanged stored decision: an up-to-date run changes nothing.
BT_FLAG=""; BT_STORED=enabled; BT_CHOICE=enabled
bluetooth_apply_without_update; [ -z "$log" ]
BT_CHOICE=skip; BT_STORED=""
bluetooth_apply_without_update; [ -z "$log" ]
# A new answer or an explicit flag is applied even without an update.
BT_CHOICE=enabled
bluetooth_apply_without_update; [ "$log" = " enabled" ]
log=""; BT_FLAG=yes; BT_STORED=enabled
bluetooth_apply_without_update; [ "$log" = " enabled" ]
log=""; BT_FLAG=""; BT_STORED=enabled; BT_CHOICE=declined
bluetooth_apply_without_update; [ "$log" = " declined" ]
`
	if output, err := runBashScript(t, bash, script); err != nil {
		t.Fatalf("update.sh Bluetooth helpers failed: %v\n%s", err, output)
	}
}

func TestInstallServiceLinuxPreparesBluetooth(t *testing.T) {
	t.Parallel()
	requireInOrder(t, "install_service_linux.sh", readRepoFile(t, "install_service_linux.sh"),
		"# >>> AURAGO-BLUETOOTH-KIT",
		`systemctl enable "${SERVICE_NAME}"`,
		`SUDO=""`,
		`BT_CHOICE="$(btk_resolve_choice "$INSTALL_DIR" "" "$_bt_may_prompt")"`,
		`btk_run_choice "$BT_CHOICE" "$INSTALL_DIR" "$(id -un "${SUDO_USER:-root}")" "$SERVICE_NAME"`,
		`ok "AuraGo service has been installed and enabled."`,
	)
}
