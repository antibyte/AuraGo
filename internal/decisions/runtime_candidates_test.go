package decisions

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/onnxrt"
)

func TestNativeInstallReusesVerifiedPackagesWithoutOtherDownloads(t *testing.T) {
	for _, tc := range []struct {
		name, location string
		cpu, consent   bool
	}{
		{name: "developer accelerator", location: "developer", consent: true},
		{name: "provisioned accelerator", location: "provisioned", consent: true},
		{name: "CPU consent preserves accelerator preference", location: "provisioned", cpu: true, consent: true},
		{name: "CPU without consent still needs accelerator", location: "provisioned", cpu: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, data := t.TempDir(), t.TempDir()
			managed := filepath.Join(data, "decisions", "runtime")
			backend := "vulkan"
			if runtime.GOOS == "darwin" {
				backend = "metal"
			}
			if tc.cpu {
				backend = "cpu"
			}
			directory := filepath.Join(managed, runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
			if tc.location == "developer" {
				directory = filepath.Join(source, "bin", "decisions-runtime", runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
			}
			writeTestRuntimeBundle(t, directory, backend)
			previous := http.DefaultTransport
			calls := 0
			http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, errors.New("offline runtime fixture")
			})
			t.Cleanup(func() { http.DefaultTransport = previous })
			manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: "native"}, data, WithRuntimeDirectory(source))
			if check := manager.CheckInstallation(t.Context()); !check.RuntimeAvailable {
				t.Fatalf("verified package not recognized: %+v", check)
			}
			err := ensureNativeRuntime(t.Context(), source, managed, tc.consent, nil)
			if tc.cpu {
				if err == nil || calls == 0 {
					t.Fatalf("CPU package bypassed accelerator acquisition: %v, requests=%d", err, calls)
				}
			} else if err != nil || calls != 0 {
				t.Fatalf("existing package required another download: %v, requests=%d", err, calls)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if err := ensureNativeRuntime(ctx, source, managed, tc.consent, nil); ErrorCode(err) != CodeCancelled {
				t.Fatalf("cancellation: %v", err)
			}
		})
	}
}

func cachedRuntimePackage(t *testing.T, managed, backend string) runtimePackage {
	t.Helper()
	directory := t.TempDir()
	writeTestRuntimeBundle(t, directory, backend)
	raw, _ := os.ReadFile(filepath.Join(directory, "release-manifest.json"))
	var manifest runtimeManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Schema = "aurago.decisions.runtime-manifest.v2"
	manifest.Files = []runtimeManifestFile{{File: manifest.Artifact.File, SHA256: manifest.Artifact.SHA256, Size: manifest.Artifact.SizeBytes}}
	raw, _ = json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(directory, "release-manifest.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range []string{manifest.Artifact.File, "release-manifest.json"} {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	digest, manifestDigest := sha256.Sum256(archive.Bytes()), sha256.Sum256(raw)
	entry := runtimePackage{ID: "decisions-test-" + backend, Target: runtimeTarget(runtime.GOOS, runtime.GOARCH), Backend: backend,
		URL: "https://github.com/antibyte/AuraGo/releases/download/decisions-runtime-v0.0.1/test-" + backend + ".zip", Size: int64(archive.Len()), SHA256: hex.EncodeToString(digest[:]), ManifestSHA256: hex.EncodeToString(manifestDigest[:]), UnpackedSize: 4096}
	path := onnxrt.ArchivePath(filepath.Join(managed, "packages"), entry.asset())
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestNativePackageResolutionSkipsStaleDeveloperAndUsesPinnedCachedArchives(t *testing.T) {
	source, data := t.TempDir(), t.TempDir()
	managed := filepath.Join(data, "decisions", "runtime")
	previous := releaseRuntimeCatalog
	releaseRuntimeCatalog = runtimeCatalog{}
	t.Cleanup(func() { releaseRuntimeCatalog = previous })
	for _, backend := range runtimeInstallBackends(true) {
		releaseRuntimeCatalog.Packages = append(releaseRuntimeCatalog.Packages, cachedRuntimePackage(t, managed, backend))
	}
	backend := runtimeInstallBackends(true)[0]
	developer := filepath.Join(source, "bin", "decisions-runtime", runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
	writeTestRuntimeBundle(t, developer, backend)
	manifest := filepath.Join(developer, "release-manifest.json")
	raw, _ := os.ReadFile(manifest)
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(raw), llamaCPPCommit, strings.Repeat("0", 40), 1)), 0600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: "native"}, data, WithRuntimeDirectory(source))
	if check := manager.CheckInstallation(t.Context()); check.Code != CodeRuntimeAttestationFailed {
		t.Fatalf("stale candidate: %+v", check)
	}
	if err := ensureNativeRuntime(t.Context(), source, managed, true, nil); err != nil {
		t.Fatal(err)
	}
	if check := manager.CheckInstallation(t.Context()); !check.RuntimeAvailable || check.Code != "" {
		t.Fatalf("valid alternative masked: %+v", check)
	}
	for _, entry := range releaseRuntimeCatalog.Packages {
		bundles, err := nativeRuntimeCandidates(t.Context(), source, managed, entry.Backend)
		if err != nil || len(bundles) != 1 {
			t.Fatalf("backend %s: %v %+v", entry.Backend, err, bundles)
		}
	}
	if err := ensureNativeRuntime(t.Context(), source, managed, true, nil); err != nil {
		t.Fatalf("reused release: %v", err)
	}
	if _, err := discoverNativeDevices(t.Context(), managed, source, ""); ErrorCode(err) != "runtime_dependencies_missing" {
		t.Fatalf("discovery did not verify the same release candidates: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := nativeRuntimeCandidates(ctx, source, managed, backend); ErrorCode(err) != CodeCancelled {
		t.Fatalf("cancel: %v", err)
	}
	entry := releaseRuntimeCatalog.Packages[0]
	entry.ManifestSHA256 = strings.Repeat("0", 64)
	releaseRuntimeCatalog.Packages[0] = entry
	if _, err := nativeRuntimeCandidates(t.Context(), source, managed, backend); ErrorCode(err) != CodeRuntimeAttestationFailed {
		t.Fatalf("bad release manifest: %v", err)
	}
	entry = releaseRuntimeCatalog.Packages[len(releaseRuntimeCatalog.Packages)-1]
	directory := onnxrt.RuntimeDir(filepath.Join(managed, "packages"), entry.asset())
	if err := os.WriteFile(filepath.Join(directory, executableName(runtime.GOOS)), []byte("changed runtime"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := nativeRuntimeCandidates(t.Context(), source, managed, entry.Backend); ErrorCode(err) != CodeRuntimeAttestationFailed {
		t.Fatalf("bad release binary: %v", err)
	}
}
