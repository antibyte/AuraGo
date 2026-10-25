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
			{"keyed without certificate", true, false, true, false, true},
			{"invalid legacy", true, true, false, false, false},
			{"invalid strict", true, true, false, true, false},
			{"invalid keyed without certificate", true, false, false, false, false},
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
    case "$1" in */SHA256SUMS.sig) %t || return 1;; *.pem) %t || return 1;; *) return 1;; esac
    printf fixture
}
_download_optional() { fetch_optional_url_to_file "$@"; }
warn() { :; }
ok() { :; }
die() { echo REJECTED; exit 81; }
`, strict, tc.sig, tc.cert)
				tools := fmt.Sprintf(`
release_verifier_tool() { printf 'signature_fixture'; }
signature_fixture() { %t; }
`, tc.valid)
				output, err := runUpdateScript(t, fixture+helper+tools+"\nverify_release_checksums_signature\necho ACCEPTED\n")
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
		sign := strings.LastIndex(script, "sign-release-checksums.")
		checksum := strings.Index(script, "Generating SHA256SUMS")
		if path == "make_deploy.sh" {
			checksum = strings.Index(script, `: > "$DEPLOY_DIR/SHA256SUMS"`)
		}
		publish := strings.Index(script, "git add .")
		if path == "make_release.ps1" {
			publish = strings.Index(script, "git push origin HEAD:refs/heads/main")
		}
		if sign < 0 || checksum < 0 || publish < 0 || sign <= checksum || sign >= publish {
			t.Fatalf("%s must sign finished checksums before publication", path)
		}
	}
}

func TestWindowsReleaseSigningPreflightRunsBeforeCleanup(t *testing.T) {
	for _, path := range []string{"make_release.ps1", "make_release.bat"} {
		script := readRepoFile(t, path)
		check := strings.Index(script, "-CheckPrerequisites")
		cleanup := strings.Index(script, "Get-ChildItem -LiteralPath 'deploy'")
		if check < 0 || cleanup < 0 || check >= cleanup {
			t.Fatalf("%s must check signing before cleaning generated artifacts or building", path)
		}
	}
}

func TestWindowsReleaseUploadsDraftBeforePublishingAndRetention(t *testing.T) {
	for _, tc := range []struct {
		path, create, upload string
	}{
		{"make_release.bat", `gh release create "!VERSION!" --draft`, "gh release upload"},
		{"make_release.ps1", `$ghArgs = @("release", "create", $Version) + $assets + @("--draft"`, "gh @ghArgs"},
	} {
		script := readRepoFile(t, tc.path)
		create := strings.Index(script, tc.create)
		upload := strings.LastIndex(script, tc.upload)
		publish := strings.Index(script, "gh release edit")
		retention := strings.Index(script, "gh release list --limit 100 --exclude-drafts --exclude-pre-releases")
		if create < 0 || upload <= create || publish <= upload || retention <= publish {
			t.Fatalf("%s must upload to a draft, publish it, then retain stable AuraGo releases", tc.path)
		}
		if !strings.Contains(script[publish:retention], "--draft=false --latest") {
			t.Fatalf("%s must explicitly publish the completed draft as latest", tc.path)
		}
		if !strings.Contains(script[publish:retention], "AURAGO_KEEP_RELEASES") || !strings.Contains(script[publish:retention], "Keeping all existing releases and tags.") {
			t.Fatalf("%s must support publication without deleting existing releases or tags", tc.path)
		}
		if tc.path == "make_release.bat" {
			if !strings.Contains(script[publish:retention], "if errorlevel 1 (") || !strings.Contains(script[publish:retention], "exit /b 1") {
				t.Fatal("batch publication failures must stop before retention")
			}
		} else if !strings.Contains(script[upload:publish], "if ($LASTEXITCODE -ne 0) { throw") || !strings.Contains(script[publish:retention], "if ($LASTEXITCODE -ne 0) { throw") {
			t.Fatal("PowerShell upload and publication failures must stop before retention")
		}
	}
}
