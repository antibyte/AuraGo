package audit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMakeDeployUsesExactReleaseArtifactManifests(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "make_deploy.sh"))
	if err != nil {
		t.Fatalf("read make_deploy.sh: %v", err)
	}
	script := string(data)
	for _, marker := range []string{
		"derive_release_artifact_lists",
		"RELEASE_DEPLOY_ASSETS=(\"$RESOURCES\" install.sh update.sh)",
		"RELEASE_BIN_ASSETS=()",
		"for target in \"${TARGETS[@]}\"; do",
		"for target in \"${REMOTE_TARGETS[@]}\"; do",
		"for asset in \"${RELEASE_DEPLOY_ASSETS[@]}\"; do",
		"for asset in \"${RELEASE_BIN_ASSETS[@]}\"; do",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("make_deploy.sh missing release manifest contract %q", marker)
		}
	}
}

func TestMakeDeployValidatesTargetsAndCanSkipPublishing(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "make_deploy.sh"))
	if err != nil {
		t.Fatalf("read make_deploy.sh: %v", err)
	}
	script := string(data)
	for _, marker := range []string{
		"validate_targets",
		"go tool dist list",
		"duplicate target",
		"--no-publish",
		"PUBLISH_RELEASE=false",
		"Skipping publish (--no-publish).",
		"while [ \"$#\" -gt 0 ]; do",
		"cp \"$OUT\" \"bin/aurago-remote_linux_arm64\"",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("make_deploy.sh missing build safety contract %q", marker)
		}
	}
}

func TestMakeDeployCustomTargetArtifactsAreChecksummedAndCleaned(t *testing.T) {
	if os.Getenv("AURAGO_RUN_RELEASE_BUILD_TESTS") != "1" {
		t.Skip("set AURAGO_RUN_RELEASE_BUILD_TESTS=1 to run the release build regression: AURAGO_RUN_RELEASE_BUILD_TESTS=1 go test ./internal/audit -run '^TestMakeDeployCustomTargetArtifactsAreChecksummedAndCleaned$' -count=1")
	}

	root := filepath.Join("..", "..")
	dockerfile := filepath.Join(root, "deploy", "docker", "Dockerfile.code-studio")
	originalDockerfile, err := os.ReadFile(dockerfile)
	if err != nil {
		t.Fatalf("read versioned Dockerfile before release build: %v", err)
	}
	cleanupReleaseArtifacts(t, root)
	t.Cleanup(func() { cleanupReleaseArtifacts(t, root) })

	runReleaseSubset(t, root, "linux/386", "linux/386")
	manifest := releaseChecksumBasenames(t, filepath.Join(root, "deploy", "SHA256SUMS"))
	for _, asset := range []string{"aurago_linux_386", "aurago-remote_linux_386"} {
		if manifest[asset] != 1 {
			t.Fatalf("custom release asset %q has %d checksum entries, want 1", asset, manifest[asset])
		}
		if _, err := os.Stat(filepath.Join(root, "deploy", asset)); err != nil {
			t.Fatalf("custom release asset %q missing: %v", asset, err)
		}
	}

	runReleaseSubset(t, root, "linux/amd64", "linux/amd64")
	for _, asset := range []string{"aurago_linux_386", "aurago-remote_linux_386"} {
		if _, err := os.Stat(filepath.Join(root, "deploy", asset)); !os.IsNotExist(err) {
			t.Fatalf("stale custom release asset %q still exists, err=%v", asset, err)
		}
	}
	if got, err := os.ReadFile(dockerfile); err != nil {
		t.Fatalf("read versioned Dockerfile after release build: %v", err)
	} else if string(got) != string(originalDockerfile) {
		t.Fatal("release cleanup modified deploy/docker/Dockerfile.code-studio")
	}
}

func TestMakeDeployCustomTargetBuildRequiresOptInBeforeSideEffects(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("release_build_contract_test.go")
	if err != nil {
		t.Fatalf("read release build regression: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "func TestMakeDeployCustomTargetArtifactsAreChecksummedAndCleaned")
	end := strings.Index(source[start+1:], "\nfunc ")
	if start < 0 || end < 0 {
		t.Fatal("could not locate custom release regression body")
	}
	body := source[start : start+1+end]
	gate := strings.Index(body, `if os.Getenv("AURAGO_RUN_RELEASE_BUILD_TESTS") != "1"`)
	if gate < 0 {
		t.Fatal("custom release regression must require AURAGO_RUN_RELEASE_BUILD_TESTS=1")
	}
	for _, sideEffect := range []string{"cleanupReleaseArtifacts", "releaseBash", "runReleaseSubset"} {
		if index := strings.Index(body, sideEffect); index >= 0 && gate > index {
			t.Fatalf("opt-in gate must precede %s", sideEffect)
		}
	}
}

func TestMakeDeployCustomTargetBuildOptOutPreservesSentinel(t *testing.T) {
	t.Setenv("AURAGO_RUN_RELEASE_BUILD_TESTS", "")
	root := filepath.Join("..", "..")
	sentinel := filepath.Join(root, "deploy", ".release-build-optout-sentinel")
	if _, err := os.Stat(sentinel); err == nil {
		t.Skipf("sentinel already exists: %s", sentinel)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat sentinel: %v", err)
	}
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(sentinel) })

	cmd := exec.Command(os.Args[0], "-test.run", "^TestMakeDeployCustomTargetArtifactsAreChecksummedAndCleaned$", "-test.v")
	cmd.Dir = "."
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run custom release regression without opt-in: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "AURAGO_RUN_RELEASE_BUILD_TESTS=1") {
		t.Fatalf("opt-out regression did not report its opt-in instruction:\n%s", output)
	}
	data, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("sentinel was removed by opt-out regression: %v", err)
	}
	if string(data) != "keep me" {
		t.Fatalf("sentinel content = %q, want unchanged", data)
	}
}

func runReleaseSubset(t *testing.T, root, targets, remoteTargets string) {
	t.Helper()
	bash := releaseBash(t)
	command := "AURAGO_TARGETS=" + shellQuote(targets) + " AURAGO_REMOTE_TARGETS=" + shellQuote(remoteTargets) + " ./make_deploy.sh --no-publish"
	if runtime.GOOS == "windows" {
		command = "PATH='/c/Program Files/Go/bin:/usr/bin:/bin' " + command
	}
	cmd := exec.Command(bash, "-lc", command)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run release subset %q/%q: %v\n%s", targets, remoteTargets, err, output)
	}
}

func releaseBash(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		const gitBash = `C:\Program Files\Git\bin\bash.exe`
		if _, err := os.Stat(gitBash); err == nil {
			return gitBash
		}
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not available")
	}
	return bash
}

func releaseChecksumBasenames(t *testing.T, path string) map[string]int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	entries := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("invalid checksum entry %q", line)
		}
		entries[fields[1]]++
	}
	return entries
}

func cleanupReleaseArtifacts(t *testing.T, root string) {
	t.Helper()
	for _, asset := range []string{
		"resources.dat", "install.sh", "update.sh", "SHA256SUMS", "SHA256SUMS.sig", "SHA256SUMS.pem",
		"aurago_linux_386", "aurago-remote_linux_386", "aurago-remote_linux_amd64", "aurago-remote_linux_arm64",
	} {
		if err := os.Remove(filepath.Join(root, "deploy", asset)); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove deploy artifact %q: %v", asset, err)
		}
	}
	for _, asset := range []string{
		"aurago_linux", "aurago_linux_arm64", "config-merger_linux", "config-merger_linux_arm64",
		"aurago-remote_linux", "aurago-remote_linux_arm64",
	} {
		if err := os.Remove(filepath.Join(root, "bin", asset)); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove bin artifact %q: %v", asset, err)
		}
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
}

func TestMakeDeployWritesEachChecksumBasenameOnlyOnce(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "make_deploy.sh"))
	if err != nil {
		t.Fatalf("read make_deploy.sh: %v", err)
	}
	script := string(data)
	for _, marker := range []string{
		"checksum_seen=\" \"",
		"if [[ \"$checksum_seen\" == *\" $asset \"* ]]; then",
		"continue",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("make_deploy.sh missing checksum basename de-duplication contract %q", marker)
		}
	}
}
