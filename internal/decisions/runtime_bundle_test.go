package decisions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	if _, _, err := validateRuntimeManifest(t.Context(), directory, backend, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("valid pinned runtime bundle rejected: %v", err)
	}
	if err := os.WriteFile(binary, []byte("tampered executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := validateRuntimeManifest(t.Context(), directory, backend, runtime.GOOS, runtime.GOARCH); err == nil {
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
	if _, _, err := validateRuntimeManifest(t.Context(), installed, backend, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Fatalf("provisioned runtime bundle did not verify: %v", err)
	}
}

func TestProvisionNativeRuntimeDoesNotInventMissingArtifacts(t *testing.T) {
	err := provisionNativeRuntime(t.Context(), t.TempDir(), t.TempDir())
	if ErrorCode(err) != CodeRuntimeArtifactMissing {
		t.Fatalf("missing local bundle error = %v", err)
	}
}

func TestCopyAtomicCancelledPublicationPreservesRuntime(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "source"), filepath.Join(root, "runtime")
	for path, value := range map[string]string{source: "new", destination: "old"} {
		if err := os.WriteFile(path, []byte(value), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := copyAtomic(ctx, source, destination); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "old" {
		t.Fatalf("original lost: %q, %v", got, err)
	}
	if _, err := os.Stat(destination + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary file retained: %v", err)
	}
	if err := copyAtomic(t.Context(), source, destination); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(destination); err != nil || string(got) != "new" {
		t.Fatalf("replacement failed: %q, %v", got, err)
	}
}
