package decisions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/onnxrt"
)

func TestPublishedRuntimeCatalogCoversSupportedPlatforms(t *testing.T) {
	catalog := releaseRuntimeCatalog
	if matched, _ := regexp.MatchString(`^[0-9]+\.[0-9]+\.[0-9]+$`, catalog.Version); !matched {
		t.Fatal("a published and anonymously verified runtime catalog is required")
	}
	want := map[string][]string{
		"windows-amd64": {"cuda", "vulkan", "cpu"},
		"windows-arm64": {"vulkan", "cpu"},
		"linux-amd64":   {"cuda", "vulkan", "cpu"},
		"linux-arm64":   {"cuda", "vulkan", "cpu"},
		"darwin-amd64":  {"metal", "cpu"},
		"darwin-arm64":  {"metal", "cpu"},
	}
	if len(catalog.Packages) != 15 || len(catalog.Images) != 3 {
		t.Fatalf("incomplete runtime catalog: %d packages, %d images", len(catalog.Packages), len(catalog.Images))
	}
	for target, backends := range want {
		goos, goarch, _ := strings.Cut(target, "-")
		for _, backend := range backends {
			entry, ok := catalog.native(goos, goarch, backend)
			if !ok {
				t.Fatalf("missing runtime: %s/%s", target, backend)
			}
			id := "decisions-" + catalog.Version + "-" + target + "-" + backend
			url := "https://github.com/antibyte/AuraGo/releases/download/decisions-runtime-v" + catalog.Version + "/" + id + ".zip"
			if entry.ID != id || entry.URL != url || !validSHA256(entry.ManifestSHA256) || entry.UnpackedSize <= 0 || len(entry.Requirements) == 0 {
				t.Fatalf("incomplete or unversioned package pins: %s", id)
			}
			if err := onnxrt.ValidateAssetSpec(entry.asset()); err != nil {
				t.Fatalf("invalid package %s: %v", id, err)
			}
		}
	}
	const prefix = "ghcr.io/antibyte/aurago-decisions-runtime@sha256:"
	for _, backend := range []string{"cuda", "vulkan", "cpu"} {
		reference := catalog.image(backend)
		if !strings.HasPrefix(reference, prefix) || !validSHA256(strings.TrimPrefix(reference, prefix)) {
			t.Fatalf("missing immutable image pin: %s", backend)
		}
	}
}

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
