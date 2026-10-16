package decisions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReleaseRuntimeValidatesEveryPackagedFile(t *testing.T) {
	directory := t.TempDir()
	writeTestRuntimeBundle(t, directory, "cpu")
	path := filepath.Join(directory, "release-manifest.json")
	raw, _ := os.ReadFile(path)
	var manifest runtimeManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Schema = "aurago.decisions.runtime-manifest.v2"
	library := filepath.Join(directory, "library.dll")
	if err := os.WriteFile(library, []byte("verified dependency"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{manifest.Artifact.File, "library.dll"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		digest, err := fileSHA256(t.Context(), filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		manifest.Files = append(manifest.Files, runtimeManifestFile{File: name, Size: info.Size(), SHA256: digest})
	}
	raw, _ = json.Marshal(manifest)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	verify := func() error {
		_, _, err := validateRuntimeManifest(t.Context(), directory, "cpu", runtime.GOOS, runtime.GOARCH)
		return err
	}
	if err := verify(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(library, []byte("modified dependency"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(); err == nil {
		t.Fatal("changed DLL was trusted")
	}
	if err := os.WriteFile(library, []byte("verified dependency"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "injected.dll"), []byte("unexpected"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verify(); err == nil {
		t.Fatal("unlisted dependency was trusted")
	}
}

func TestNativeSetupReusesProvisionedPackageWithoutBuildTree(t *testing.T) {
	root := t.TempDir()
	writeTestRuntimeBundle(t, filepath.Join(root, runtimeTarget(runtime.GOOS, runtime.GOARCH), "cpu"), "cpu")
	previous := releaseRuntimeCatalog
	releaseRuntimeCatalog = runtimeCatalog{}
	defer func() { releaseRuntimeCatalog = previous }()
	if err := ensureNativeRuntime(t.Context(), t.TempDir(), root, true, nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ensureNativeRuntime(ctx, t.TempDir(), root, true, nil); ErrorCode(err) != CodeCancelled {
		t.Fatalf("cancelled setup = %v", err)
	}
}
