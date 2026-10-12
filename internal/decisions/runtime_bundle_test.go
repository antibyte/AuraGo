package decisions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTestRuntimeBundle(t *testing.T, directory, backend string, executable ...[]byte) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	binary := []byte("local pinned runtime test executable")
	if len(executable) > 0 {
		binary = executable[0]
	}
	binaryPath := filepath.Join(directory, executableName(runtime.GOOS))
	if err := os.WriteFile(binaryPath, binary, 0o700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binary)
	pins := DefaultManifest()
	manifest := map[string]any{
		"schema":   runtimeManifestSchema,
		"protocol": runtimeProtocol,
		"build": map[string]any{
			"target": runtimeTarget(runtime.GOOS, runtime.GOARCH), "backend": backend,
			"llama_cpp_commit": llamaCPPCommit, "llama_cpp_patch_sha256": llamaCPPPatchSHA256,
		},
		"artifact": map[string]any{
			"file": executableName(runtime.GOOS), "sha256": hex.EncodeToString(digest[:]), "size_bytes": len(binary),
		},
		"models": []map[string]any{
			{"role": "model", "repo": pins.Repository, "revision": modelRevision, "file": pins.Model.Filename, "sha256": pins.Model.SHA256, "size_bytes": pins.Model.Size},
			{"role": "projector", "repo": pins.Repository, "revision": modelRevision, "file": pins.Projector.Filename, "sha256": pins.Projector.SHA256, "size_bytes": pins.Projector.Size},
		},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "release-manifest.json"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return binaryPath
}

func TestRuntimeManifestVerifiesLocalBundleAndRejectsTampering(t *testing.T) {
	backend := runtimeBackends(runtime.GOOS)[0]
	directory := t.TempDir()
	binary := writeTestRuntimeBundle(t, directory, backend)
	if _, _, err := validateRuntimeManifest(directory, backend, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("valid pinned runtime bundle rejected: %v", err)
	}
	if err := os.WriteFile(binary, []byte("tampered executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validateRuntimeManifest(directory, backend, runtime.GOOS, runtime.GOARCH); err == nil {
		t.Fatal("modified runtime binary passed the release manifest")
	}
}

func TestProvisionNativeRuntimeCopiesOnlyVerifiedLocalBundle(t *testing.T) {
	backend := runtimeBackends(runtime.GOOS)[0]
	sourceRoot, dataRoot := t.TempDir(), t.TempDir()
	source := filepath.Join(sourceRoot, "bin", "decisions-runtime", runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
	writeTestRuntimeBundle(t, source, backend)
	if err := provisionNativeRuntime(t.Context(), sourceRoot, dataRoot); err != nil {
		t.Fatalf("provision local runtime bundle: %v", err)
	}
	installed := filepath.Join(dataRoot, runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
	if _, _, err := validateRuntimeManifest(installed, backend, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("provisioned runtime bundle did not verify: %v", err)
	}
}

func TestProvisionNativeRuntimeDoesNotInventMissingArtifacts(t *testing.T) {
	err := provisionNativeRuntime(t.Context(), t.TempDir(), t.TempDir())
	if ErrorCode(err) != CodeRuntimeArtifactMissing {
		t.Fatalf("missing local bundle error = %v", err)
	}
}
