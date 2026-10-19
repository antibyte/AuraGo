package audit

import (
	"fmt"
	"strings"
	"testing"
)

func TestReleaseSignatureSuppliedMetadataNeverDowngrades(t *testing.T) {
	for _, path := range []string{"update.sh", "install.sh"} {
		script := readRepoFile(t, path)
		start := strings.Index(script, "strict_release_verify_enabled() {")
		if start < 0 {
			t.Fatal("release verifier helper not found")
		}
		end := strings.Index(script[start:], "\nfetch_release_checksums() {")
		if end < 0 {
			t.Fatal("release verifier helper not found")
		}
		helper := strings.ReplaceAll(script[start:start+end], "\r\n", "\n")
		for _, tc := range []struct {
			name                              string
			sig, cert, valid, strict, success bool
		}{
			{"valid", true, true, true, false, true},
			{"invalid legacy", true, true, false, false, false},
			{"invalid strict", true, true, false, true, false},
			{"missing certificate", true, false, true, false, false},
			{"missing signature", false, true, true, false, false},
			{"legacy absence", false, false, false, false, true},
			{"strict absence", false, false, false, true, false},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				strict := "0"
				if tc.strict {
					strict = "1"
				}
				fixture := fmt.Sprintf(`
RELEASE_BASE=https://fixture.invalid
GITHUB_REPO=example/project
AURAGO_STRICT_RELEASE_VERIFY=%s
AURAGO_RELEASE_PUBLIC_KEY=''
RELEASE_CHECKSUMS_FILE="$(mktemp)"
trap 'rm -f -- "$RELEASE_CHECKSUMS_FILE"' EXIT
fetch_optional_url_to_file() {
    case "$1" in *.sig) %t;; *.pem) %t;; *) return 1;; esac
}
_download_optional() { fetch_optional_url_to_file "$@"; }
cosign() { %t; }
warn() { :; }
ok() { :; }
die() { echo REJECTED; exit 81; }
`, strict, tc.sig, tc.cert, tc.valid)
				output, err := runUpdateScript(t, fixture+helper+"\nverify_release_checksums_signature\necho ACCEPTED\n")
				if (err == nil) != tc.success || strings.Contains(output, "ACCEPTED") != tc.success {
					t.Fatalf("signature policy success=%v error=%v output=%s", tc.success, err, output)
				}
			})
		}
	}
}

func TestReleaseSignerRunsAfterChecksumsBeforePublishing(t *testing.T) {
	for _, path := range []string{"make_release.ps1", "make_release.bat", "make_deploy.sh"} {
		script := readRepoFile(t, path)
		sign := strings.Index(script, "sign-release-checksums.")
		checksum := strings.Index(script, "Generating SHA256SUMS")
		if path == "make_deploy.sh" {
			checksum = strings.Index(script, `: > "$DEPLOY_DIR/SHA256SUMS"`)
		}
		publish := strings.Index(script, "git add .")
		if sign < 0 || checksum < 0 || publish < 0 || sign <= checksum || sign >= publish {
			t.Fatalf("%s must sign finished checksums before publication", path)
		}
	}
}
