package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMeshCoreSerialServicePermissions(t *testing.T) {
	bash := "bash"
	if runtime.GOOS == "windows" {
		bash = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
	}
	if _, err := exec.LookPath(bash); err != nil {
		t.Skip("bash is unavailable")
	}
	extract := func(source, start, end string) string {
		t.Helper()
		i := strings.Index(source, start)
		if i < 0 {
			t.Fatalf("missing start %q", start)
		}
		j := strings.Index(source[i:], end)
		if j < 0 {
			t.Fatalf("missing end %q", end)
		}
		return source[i : i+j]
	}
	run := func(script string) {
		t.Helper()
		cmd := exec.Command(bash, "-c", script)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("serial permission check failed: %v\n%s", err, output)
		}
	}
	for _, path := range []string{"install.sh", "install_service_linux.sh", "update.sh"} {
		t.Run(path, func(t *testing.T) {
			source := readRepoFile(t, path)
			helper := extract(source, "systemd_serial_groups_line() {", "\n}\n") + "\n}\n"
			run("set -eu\n" + helper + `
system_group_exists() { [[ " $available " == *" $1 "* ]]; }
for available in '' dialout uucp 'dialout uucp'; do
    expected=''
    [ -z "$available" ] || expected="SupplementaryGroups=$available"
    [ "$(systemd_serial_groups_line)" = "$expected" ]
done
`)
			if path != "update.sh" {
				if !strings.Contains(source, `SERIAL_GROUPS_LINE="$(systemd_serial_groups_line)"`) ||
					!strings.Contains(source, "${GPU_GROUPS_LINE}\n${SERIAL_GROUPS_LINE}\n${GPU_GROUP_IDS_LINE}") {
					t.Fatal("installer must add serial groups to the service separately from GPU container IDs")
				}
				return
			}
			restore := extract(source, "restore_service_stop_timeout_dropin() {", "\nabort_update() {")
			migration := extract(source, "# Keep systemd's stop deadline", "# ── Service restart")
			for _, fail := range []string{"false", "true"} {
				for _, existed := range []string{"false", "true"} {
					fixture := `
set -eu
root=$(mktemp -d)
trap 'rm -rf -- "$root"' EXIT
SUDO=''
SVC_FILE="$root/aurago.service"
printf '[Service]\nSupplementaryGroups=video custom\n' > "$SVC_FILE"
SYSTEMD_DROPIN_DIR="$root/aurago.service.d"
SYSTEMD_STOP_TIMEOUT_DROPIN="$SYSTEMD_DROPIN_DIR/20-aurago-stop-timeout.conf"
SYSTEMD_DROPIN_BACKUP="$root/backup"
SYSTEMD_DROPIN_CHANGED=false
available=dialout
system_group_exists() { [[ " $available " == *" $1 "* ]]; }
systemctl() { [ "$1" = daemon-reload ]; }
systemd-analyze() { ! $fail; }
install() { cp "${@: -2:1}" "${@: -1}"; }
ok() { :; }
warn() { :; }
abort_update() { exit 42; }
mkdir -p "$SYSTEMD_DROPIN_DIR"
if $SYSTEMD_DROPIN_EXISTED; then
    printf '[Service]\nTimeoutStopSec=90s\nSupplementaryGroups=uucp\n' > "$SYSTEMD_STOP_TIMEOUT_DROPIN"
    cp "$SYSTEMD_STOP_TIMEOUT_DROPIN" "$SYSTEMD_DROPIN_BACKUP"
fi
`
					checks := `
grep -Fxq 'SupplementaryGroups=video custom' "$SVC_FILE"
if $fail; then
    [ "$result" = 42 ]
    if $SYSTEMD_DROPIN_EXISTED; then
        cmp "$SYSTEMD_STOP_TIMEOUT_DROPIN" "$SYSTEMD_DROPIN_BACKUP"
    else
        [ ! -e "$SYSTEMD_STOP_TIMEOUT_DROPIN" ]
    fi
else
    [ "$result" = 0 ]
    grep -Fxq 'SupplementaryGroups=dialout' "$SYSTEMD_STOP_TIMEOUT_DROPIN"
    grep -Fxq 'TimeoutStopSec=60s' "$SYSTEMD_STOP_TIMEOUT_DROPIN"
    [ "$(grep -c '^SupplementaryGroups=' "$SYSTEMD_STOP_TIMEOUT_DROPIN")" = 1 ]
fi
`
					// Run twice to exercise idempotency; failed verification must restore the prior file.
					run("fail=" + fail + "\nSYSTEMD_DROPIN_EXISTED=" + existed + "\n" + fixture + helper + restore +
						"for attempt in 1 2; do\nresult=0\n(\n" + migration + "\n) || result=$?\n" + checks + "\ndone\n")
				}
			}
		})
	}
}
