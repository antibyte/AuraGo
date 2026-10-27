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

// chromiumSandboxBlock returns the two AppArmor helper functions as written in
// an installer script. install.sh and update.sh must carry the same text.
func chromiumSandboxBlock(t *testing.T, source, name string) string {
	t.Helper()
	start := strings.Index(source, "chromium_sandbox_profile_text() {\n")
	applyStart := strings.Index(source, "apply_chromium_sandbox_profile() {\n")
	if start < 0 || applyStart < start {
		t.Fatalf("%s: chromium sandbox helpers are missing", name)
	}
	end := strings.Index(source[applyStart:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s: apply_chromium_sandbox_profile has no end", name)
	}
	return source[start : applyStart+end+3]
}

func TestChromiumSandboxHelpersAreIdenticalInInstallAndUpdate(t *testing.T) {
	install := chromiumSandboxBlock(t, readRepoFile(t, "install.sh"), "install.sh")
	update := chromiumSandboxBlock(t, readRepoFile(t, "update.sh"), "update.sh")
	if install != update {
		t.Fatal("install.sh and update.sh must carry byte-identical chromium sandbox helpers")
	}
}

func TestChromiumSandboxProfileCallSites(t *testing.T) {
	const call = `apply_chromium_sandbox_profile "$(bluetooth_service_user)"`
	update := readRepoFile(t, "update.sh")
	if got := strings.Count(update, `apply_chromium_sandbox_profile "`); got != 3 {
		t.Fatalf("update.sh calls apply_chromium_sandbox_profile %d times, want 3", got)
	}
	// Both "already current" exits repair the host before they leave.
	early := regexp.MustCompile(`(?m)^( +)` + regexp.QuoteMeta(call) + `\n( +)bluetooth_apply_without_update\n( +)exit 0$`)
	if got := len(early.FindAllString(update, -1)); got != 2 {
		t.Fatalf("update.sh applies the profile before %d early exits, want 2", got)
	}
	// The main flow follows the standalone setcap call, not the one inside the rollback.
	if !strings.Contains(update, "\napply_aurago_setcap_if_available\n"+call+"\n") {
		t.Fatal("update.sh must apply the profile right after the main-flow setcap call")
	}

	install := readRepoFile(t, "install.sh")
	const installCall = `apply_chromium_sandbox_profile "${SERVICE_USER:-${SUDO_USER:-$(id -un)}}"`
	if strings.Count(install, `apply_chromium_sandbox_profile "`) != 1 || !strings.Contains(install, "\n"+installCall+"\n") {
		t.Fatal("install.sh must call apply_chromium_sandbox_profile exactly once with the service user")
	}
	service := strings.Index(install, "# ── Optional systemd service")
	bluetooth := strings.Index(install, "# ── Bluetooth without a service")
	at := strings.Index(install, installCall)
	if service < 0 || bluetooth < 0 || at < service || at > bluetooth {
		t.Fatal("install.sh must apply the profile after the systemd service block")
	}
}

const chromiumSandboxHarness = `
set -euo pipefail
root=$(mktemp -d)
trap 'rm -rf -- "$root"' EXIT
SUDO=''
CHROMIUM_USERNS_SYSCTL="$root/sysctl"
CHROMIUM_APPARMOR_DIR="$root/etc"
profile="$root/etc/aurago-chromium"
calls="$root/calls"; warns="$root/warns"; oks="$root/oks"
parser_fail=0; install_fail=0

getent() {
    case "$2" in
        aurago) echo 'aurago:x:998:998::/home/aurago:/usr/sbin/nologin' ;;
        spaced) echo 'spaced:x:1001:1001::/home/with space:/bin/bash' ;;
        glob) echo 'glob:x:1002:1002::/home/*:/bin/bash' ;;
        root) echo 'root:x:0:0::/root:/bin/bash' ;;
        *) return 2 ;;
    esac
}
apparmor_parser() {
    printf 'apparmor_parser %s\n' "$*" >> "$calls"
    if [ "$1" = -Q ]; then [ "$parser_fail" != 1 ]; fi
}
install() {
    printf 'install %s\n' "$*" >> "$calls"
    [ "$install_fail" != 1 ] && cp "${@: -2:1}" "${@: -1}"
}
ok() { printf '%s\n' "$*" >> "$oks"; }
warn() { printf '%s\n' "$*" >> "$warns"; }
fail() { echo "FAIL: $1" >&2; exit 1; }

reset() {
    rm -rf "$root/etc" "$root/sysctl"
    mkdir -p "$root/etc/abi"
    : > "$root/etc/abi/4.0"
    printf '1\n' > "$root/sysctl"
    : > "$calls"; : > "$warns"; : > "$oks"
    parser_fail=0; install_fail=0
}
silent() { [ ! -e "$profile" ] && [ ! -s "$calls" ] && [ ! -s "$warns" ] && [ ! -s "$oks" ]; }
one_warning() { [ "$(grep -c '' "$warns")" = 1 ] && grep -q 'documentation/installation.md' "$warns"; }

expected=$(cat <<'PROFILE'
# Managed by AuraGo install.sh/update.sh. Lets the Chromium that AuraGo's
# headless browser tools download create its sandbox (user namespaces).
abi <abi/4.0>,
include <tunables/global>

profile aurago-chromium /home/aurago/.cache/rod/browser/chromium-*/chrome flags=(unconfined) {
  userns,

  # Site-specific additions and overrides. See local/README for details.
  include if exists <local/aurago-chromium>
}
PROFILE
)

reset; printf '0\n' > "$root/sysctl"
apply_chromium_sandbox_profile aurago
silent || fail "sysctl 0 must be a silent no-op"

reset; rm -f "$root/sysctl"
apply_chromium_sandbox_profile aurago
silent || fail "a missing sysctl file must be a silent no-op"

reset
apply_chromium_sandbox_profile root
silent || fail "root must be a silent no-op"
apply_chromium_sandbox_profile ''
silent || fail "an empty user must be a silent no-op"

reset
apply_chromium_sandbox_profile aurago
[ "$(cat "$profile")" = "$expected" ] || fail "profile text differs"
grep -Fxq 'profile aurago-chromium /home/aurago/.cache/rod/browser/chromium-*/chrome flags=(unconfined) {' "$profile" || fail "profile line missing"
[ "$(grep -c '' "$calls")" = 3 ] || fail "want validate, install, reload"
sed -n 1p "$calls" | grep -q '^apparmor_parser -Q -K ' || fail "first call must validate"
sed -n 2p "$calls" | grep -q '^install -m 0644 ' || fail "second call must install"
[ "$(sed -n 3p "$calls")" = "apparmor_parser -r $profile" ] || fail "third call must reload"
[ "$(grep -c '' "$oks")" = 1 ] && [ ! -s "$warns" ] || fail "want one ok and no warning"
tmp=$(sed -n 1p "$calls"); tmp=${tmp##* }
[ ! -e "$tmp" ] || fail "temp file left behind"

: > "$calls"; : > "$oks"
apply_chromium_sandbox_profile aurago
[ ! -s "$calls" ] && [ ! -s "$warns" ] && [ ! -s "$oks" ] || fail "an identical profile must not be touched"

printf 'stale\n' > "$profile"
apply_chromium_sandbox_profile aurago
[ "$(cat "$profile")" = "$expected" ] && [ -s "$calls" ] || fail "a stale profile must be replaced"

reset; parser_fail=1
apply_chromium_sandbox_profile aurago
[ ! -e "$profile" ] && one_warning || fail "an invalid profile must warn once and not be installed"
if grep -q '^install ' "$calls"; then fail "an invalid profile must not reach install"; fi

reset; install_fail=1
apply_chromium_sandbox_profile aurago
one_warning && [ ! -s "$oks" ] || fail "a failed install must warn once"

for user in spaced glob nosuchuser; do
    reset
    apply_chromium_sandbox_profile "$user"
    [ ! -e "$profile" ] && [ ! -s "$calls" ] && one_warning || fail "home of $user must warn once and write nothing"
done

reset; rm -rf "$root/etc/abi"
apply_chromium_sandbox_profile aurago
[ ! -e "$profile" ] && [ ! -s "$calls" ] && one_warning || fail "a missing abi/4.0 must warn once and write nothing"

echo CHROMIUM_SANDBOX_HARNESS_OK
`

func TestChromiumSandboxProfileBehaviour(t *testing.T) {
	bash := "bash"
	if runtime.GOOS == "windows" {
		bash = filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
	}
	if _, err := exec.LookPath(bash); err != nil {
		t.Skip("bash is unavailable")
	}
	for _, path := range []string{"install.sh", "update.sh"} {
		t.Run(path, func(t *testing.T) {
			block := chromiumSandboxBlock(t, readRepoFile(t, path), path)
			// The functions run under the installers' own strict mode.
			script := "set -euo pipefail\n" + block + chromiumSandboxHarness
			output, err := exec.Command(bash, "-c", script).CombinedOutput()
			if err != nil || !strings.Contains(string(output), "CHROMIUM_SANDBOX_HARNESS_OK") {
				t.Fatalf("chromium sandbox helpers failed: %v\n%s", err, output)
			}
		})
	}
}
