package audit

import (
	"fmt"
	"os"
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
		"ref: ${{ github.sha }}", "run: ./scripts/install-ci-cosign.ps1",
		"AURAGO_KEEP_RELEASES: '1'", "--from-release latest",
		"./scripts/make_release.ps1 -Version $env:RELEASE_VERSION",
		"gh workflow run docker-publish.yml", "--ref $env:RELEASE_VERSION",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("release workflow lost security or publication contract %q", required)
		}
	}
	installer := readRepoFile(t, "scripts/install-ci-cosign.ps1")
	for _, pin := range []string{"v3.1.4", "191066624", "cbe953787074611f1334cbf37dc1cf953995bb8c45398eb4fc7bb781d29c1f2a", "145688404", "424e2c97d0b45fd7a6b608a87f090f35633f4c23ece5f092d48bba11aaf4bbfa"} {
		if !strings.Contains(installer, pin) {
			t.Fatalf("CI Cosign installer lost reviewed pin %q", pin)
		}
	}
	for _, forbidden := range []string{"pull_request_target:", "AURAGO_ALLOW_UNSIGNED_RELEASES", "--clobber", "secrets."} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("release workflow must not contain %q", forbidden)
		}
	}
}

func TestReleaseCICosignDownloadRejectsUnverifiedBinary(t *testing.T) {
	ps, err := exec.LookPath("pwsh")
	if err != nil {
		ps, err = exec.LookPath("powershell")
		if err != nil {
			t.Skip("PowerShell is unavailable")
		}
	}
	installer, err := filepath.Abs(filepath.Join("..", "..", "scripts", "install-ci-cosign.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		size int64
		exit int
		want string
	}{
		{"download_failure", 0, 7, "Pinned Cosign download failed"},
		{"wrong_size", 1, 0, "Cosign size mismatch"},
		{"wrong_hash", 191066624, 0, "Cosign SHA-256 mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pathFile := filepath.Join(dir, "github-path")
			fixture := fmt.Sprintf(`function gh {
    $index = [Array]::IndexOf($args, '--output')
    if ($index -lt 0) { throw 'Download destination missing' }
    $stream = [IO.File]::Open($args[$index + 1], [IO.FileMode]::Create)
    try { $stream.SetLength(%d) } finally { $stream.Dispose() }
    $global:LASTEXITCODE = %d
}
& $env:AURAGO_COSIGN_INSTALLER_FIXTURE
`, tc.size, tc.exit)
			fixturePath := filepath.Join(dir, "install.ps1")
			if err := os.WriteFile(fixturePath, []byte(fixture), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(ps, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", fixturePath)
			cmd.Env = append(os.Environ(), "RUNNER_TEMP="+dir, "GITHUB_PATH="+pathFile, "RUNNER_OS=Windows", "RUNNER_ARCH=X64", "AURAGO_COSIGN_INSTALLER_FIXTURE="+installer)
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.want) {
				t.Fatalf("unverified binary was not rejected: %v\n%s", err, output)
			}
			if _, err := os.Stat(filepath.Join(dir, "aurago-cosign", "cosign.exe.pending")); !os.IsNotExist(err) {
				t.Fatal("unverified Cosign download was not cleaned up")
			}
			if _, err := os.Stat(pathFile); !os.IsNotExist(err) {
				t.Fatal("unverified Cosign directory reached GITHUB_PATH")
			}
		})
	}
}
