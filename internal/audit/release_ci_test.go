package audit

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseCIDispatchPinsSourceAndPropagatesFailures(t *testing.T) {
	ps, err := exec.LookPath("pwsh")
	if err != nil {
		ps, err = exec.LookPath("powershell")
		if err != nil {
			t.Skip("PowerShell is unavailable")
		}
	}
	fixture := filepath.Join("..", "..", "scripts", "test-release-ci.ps1")
	if output, err := exec.Command(ps, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", fixture).CombinedOutput(); err != nil {
		t.Fatalf("offline release dispatch fixture: %v\n%s", err, output)
	}
}

func TestReleaseCIRequiresReviewedSourceAndVerifiedKeylessPublication(t *testing.T) {
	workflow := readRepoFile(t, ".github/workflows/release.yml")
	for _, required := range []string{
		"contents: read", "needs: verify-source", "id-token: write",
		`"$GITHUB_REF" = refs/heads/main`, `"$EXPECTED_SHA" = "$GITHUB_SHA"`,
		`--commit "$GITHUB_SHA"`, `gh run watch "$gate" --exit-status`,
		"--workflow training-dataset.yml", `if [ -n "$training_gate" ]; then`,
		`gh run watch "$training_gate" --exit-status`,
		"ref: ${{ github.sha }}", "cosign-release: v3.1.4",
		"AURAGO_KEEP_RELEASES: '1'", "--from-release latest",
		"./make_release.ps1 -Version $env:RELEASE_VERSION",
		"gh workflow run docker-publish.yml", "--ref $env:RELEASE_VERSION",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow lost security or publication contract %q", required)
		}
	}
	for _, forbidden := range []string{"pull_request_target:", "AURAGO_ALLOW_UNSIGNED_RELEASES", "--clobber", "secrets."} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow must not contain %q", forbidden)
		}
	}
}
