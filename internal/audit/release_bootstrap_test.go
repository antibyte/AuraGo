package audit

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func releaseBootstrap(t *testing.T, path string) string {
	t.Helper()
	source := strings.ReplaceAll(readRepoFile(t, path), "\r\n", "\n")
	start := strings.Index(source, "# BEGIN GENERATED RELEASE VERIFIER\n")
	end := strings.Index(source, "# END GENERATED RELEASE VERIFIER\n")
	if start < 0 || end < start {
		t.Fatal("generated release bootstrap missing: " + path)
	}
	return source[start:end]
}

func TestReleaseBootstrapScriptsMatch(t *testing.T) {
	if releaseBootstrap(t, "install.sh") != releaseBootstrap(t, "update.sh") ||
		releaseBootstrap(t, "install.sh") != releaseBootstrap(t, "repair-update.sh") {
		t.Fatal("installer, updater and repair must share the generated verification bootstrap")
	}
}

func TestReleaseBootstrapRejectsUnsignedOfficialReleases(t *testing.T) {
	for _, path := range []string{"install.sh", "update.sh", "repair-update.sh"} {
		body := `
RELEASE_BASE=https://fixture.invalid
GITHUB_REPO=antibyte/AuraGo
AURAGO_RELEASE_PUBLIC_KEY=''
AURAGO_STRICT_RELEASE_VERIFY=0
RELEASE_CHECKSUMS_FILE="$(mktemp)"
trap 'rm -f "$RELEASE_CHECKSUMS_FILE"' EXIT
fetch_optional_url_to_file() { return 1; }
strict_release_verify_enabled() { return 1; }
die() { echo REJECTED; exit 81; }
` + releaseBootstrap(t, path) + "\nverify_release_checksums_signature\necho ACCEPTED\n"
		if output, err := runUpdateScript(t, body); err == nil || strings.Contains(output, "ACCEPTED") {
			t.Fatalf("%s allowed an unsigned downgrade: %v %s", path, err, output)
		}
	}
}

func TestReleaseBootstrapAutomaticOfflineVerification(t *testing.T) {
	dir := t.TempDir()
	name := "release-verifier"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	helper := filepath.Join(dir, name)
	build := exec.Command("go", "build", "-o", helper, "../../cmd/release-verifier")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture helper: %v %s", err, output)
	}
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	manifest, err := filepath.Abs("../releaseverify/testdata/SHA256SUMS")
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := releaseBootstrap(t, "install.sh")
	for _, tc := range []struct {
		name, damage string
		valid        bool
		downloads    int
	}{
		{"clean installation and cache reuse", "", true, 1},
		{"corrupt download", `printf 'UNTRUSTED executable'`, false, 1},
		{"interrupted download", `printf partial; return 1`, false, 1},
		{"cancelled download", `printf partial; kill -TERM "$BASHPID"`, false, 1},
		{"corrupt cache is repaired", "", true, 1},
		{"cache symlink", "", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			install := filepath.Join(dir, strings.ReplaceAll(tc.name, " ", "-"))
			if tc.name == "cache symlink" {
				if err := os.MkdirAll(install, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(dir, filepath.Join(install, ".aurago-update")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			download := tc.damage
			if download == "" {
				download = fmt.Sprintf("cat '%s'", filepath.ToSlash(helper))
			}
			fixture := fmt.Sprintf(`
INSTALL_DIR='%s'
RELEASE_BASE=https://fixture.invalid/release
GITHUB_REPO=antibyte/AuraGo
AURAGO_RELEASE_PUBLIC_KEY=''
RELEASE_CHECKSUMS_FILE='%s'
warn() { echo "$*" >&2; }
ok() { echo VERIFIED; }
die() { echo REJECTED; exit 81; }
strict_release_verify_enabled() { return 1; }
sha256_file() { sha256sum "$1" | cut -d ' ' -f 1; }
fetch_optional_url_to_file() {
    case "$1" in
        */SHA256SUMS.sig) cat '%s.sig' ;;
        */%s) echo FETCH_NATIVE >&2; %s ;;
        *) return 1 ;;
    esac
}
`, filepath.ToSlash(install), filepath.ToSlash(manifest), filepath.ToSlash(manifest), name, download)
			// Override only the test artifact's pinned bytes, retaining the real cache/download code.
			spec := fmt.Sprintf("\nrelease_verifier_spec() { printf '%%s\\n' '%s|%d|%s|https://fixture.invalid'; }\n", name, len(data), digest)
			if tc.name == "corrupt cache is repaired" {
				spec += fmt.Sprintf("mkdir -p \"$INSTALL_DIR/.aurago-update/tools/%s-%s\"\nprintf damaged > \"$INSTALL_DIR/.aurago-update/tools/%s-%s/%s\"\n", name, digest[:16], name, digest[:16], name)
			}
			output, runErr := runUpdateScript(t, fixture+bootstrap+spec+"\nverify_release_checksums_signature\nverify_release_checksums_signature\necho ACCEPTED\n")
			if (runErr == nil) != tc.valid || strings.Contains(output, "ACCEPTED") != tc.valid {
				t.Fatalf("valid=%v err=%v output=%s", tc.valid, runErr, output)
			}
			if strings.Count(output, "FETCH_NATIVE") != tc.downloads {
				t.Fatalf("expected %d downloads, including cache reuse: %s", tc.downloads, output)
			}
			_ = filepath.WalkDir(install, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr == nil && strings.HasPrefix(entry.Name(), ".download.") {
					t.Errorf("partial download retained: %s", path)
				}
				return nil
			})
		})
	}
}

func TestReleaseBootstrapOperatorKeyDoesNotFallback(t *testing.T) {
	body := `
RELEASE_BASE=https://fixture.invalid
GITHUB_REPO=antibyte/AuraGo
AURAGO_RELEASE_PUBLIC_KEY=operator-public-key
RELEASE_CHECKSUMS_FILE="$(mktemp)"
trap 'rm -f "$RELEASE_CHECKSUMS_FILE"' EXIT
fetch_optional_url_to_file() {
    case "$1" in *.pem) echo UNSAFE_CERTIFICATE_LOOKUP >&2;; esac
    printf signature
}

die() { echo REJECTED; exit 81; }
` + releaseBootstrap(t, "update.sh") + `
release_verifier_tool() { printf '%s_fixture' "$1"; }
native_fixture() { return 1; }
cosign_fixture() {
    case "$*" in *--certificate*) echo UNSAFE_CERTIFICATE_FALLBACK; return 0;; esac
    return 1
}
verify_release_checksums_signature
echo ACCEPTED
`
	output, err := runUpdateScript(t, body)
	if err == nil || strings.Contains(output, "UNSAFE") || strings.Contains(output, "ACCEPTED") {
		t.Fatalf("operator trust was replaced: %v %s", err, output)
	}
}

func TestReleaseBootstrapRejectsOversizedMetadataBeforeToolExecution(t *testing.T) {
	body := `
RELEASE_BASE=https://fixture.invalid
GITHUB_REPO=antibyte/AuraGo
AURAGO_RELEASE_PUBLIC_KEY=''
RELEASE_CHECKSUMS_FILE="$(mktemp)"
trap 'rm -f "$RELEASE_CHECKSUMS_FILE"' EXIT
fetch_optional_url_to_file() {
    case "$1" in *.sig) head -c 4097 /dev/zero;; *) return 1;; esac
}
die() { echo REJECTED; exit 81; }
` + releaseBootstrap(t, "update.sh") + `
release_verifier_tool() { echo UNEXPECTED_TOOL_EXECUTION; return 0; }
verify_release_checksums_signature
echo ACCEPTED
`
	output, err := runUpdateScript(t, body)
	if err == nil || strings.Contains(output, "UNEXPECTED") || strings.Contains(output, "ACCEPTED") {
		t.Fatalf("oversized metadata reached execution: %v %s", err, output)
	}
}

func TestReleaseVerifierRepairPreservesInstallation(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		verify, hash, modern bool
	}{
		{"verified repair", true, true, true},
		{"signature failure", false, true, true},
		{"payload failure", true, false, true},
		{"old updater release", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			state := filepath.Join(dir, ".aurago-update")
			work := filepath.Join(state, "work")
			if err := os.MkdirAll(work, 0700); err != nil {
				t.Fatal(err)
			}
			old := []byte("old updater\n")
			next := []byte("#!/bin/bash\n# BEGIN GENERATED RELEASE VERIFIER\n# fixture\n")
			if !tc.modern {
				next = []byte("#!/bin/bash\n# old updater without bootstrap\n")
			}
			for name, data := range map[string][]byte{"update.sh": old, "config.yaml": []byte("preserve config"), "new-updater": next} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(next))
			if !tc.hash {
				hash = strings.Repeat("0", 64)
			}
			source := readRepoFile(t, "repair-update.sh")
			start := strings.Index(source, "# REPAIR RELEASE PREFLIGHT\n")
			if start < 0 {
				t.Fatal("repair preflight missing")
			}
			fixture := fmt.Sprintf(`
DIR='%s'
UPDATE_STATE='%s'
WORK='%s'
GITHUB_REPO=antibyte/AuraGo
RUN_UPDATE=false
die() { echo REJECTED; exit 81; }
ok() { echo REPAIRED; }
sha256_file() { sha256sum "$1" | cut -d ' ' -f 1; }
verify_release_checksums_signature() { %t; }
release_verifier_fetch() {
    case "$1" in
        */latest) printf '{"tag_name":"v-fixture"}' > "$2" ;;
        */SHA256SUMS) printf '%%s  update.sh\n' '%s' > "$2" ;;
        */update.sh) cp "$DIR/new-updater" "$2" ;;
        *) return 1 ;;
    esac
}
`, filepath.ToSlash(dir), filepath.ToSlash(state), filepath.ToSlash(work), tc.verify, hash)
			output, err := runUpdateScript(t, fixture+source[start:])
			valid := tc.verify && tc.hash && tc.modern
			if (err == nil) != valid {
				t.Fatalf("valid=%v error=%v output=%s", valid, err, output)
			}
			got, err := os.ReadFile(filepath.Join(dir, "update.sh"))
			if err != nil {
				t.Fatal(err)
			}
			want := old
			if valid {
				want = next
				backup, err := os.ReadFile(filepath.Join(state, "update.sh.before-repair"))
				if err != nil || string(backup) != string(old) {
					t.Fatalf("previous updater not preserved: %v", err)
				}
			}
			if string(got) != string(want) {
				t.Fatal("repair changed the updater at the wrong trust boundary")
			}
			config, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
			if err != nil || string(config) != "preserve config" {
				t.Fatalf("configuration changed: %v", err)
			}
		})
	}
}

func TestReleaseVerifierBuildersPackageAutomaticVerification(t *testing.T) {
	for _, path := range []string{"make_deploy.sh", "make_release.ps1", "make_release.bat"} {
		source := readRepoFile(t, path)
		if !strings.Contains(source, "build-release-verifier.py") || !strings.Contains(source, "--check") ||
			!strings.Contains(source, "repair-update.sh") || !strings.Contains(source, "release-verifier_") {
			t.Fatalf("%s must build/check and package automatic verification", path)
		}
	}
}
