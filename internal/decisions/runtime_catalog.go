package decisions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/fileutil"
	"aurago/internal/onnxrt"
)

type runtimePackage struct {
	ID             string   `json:"id"`
	Target         string   `json:"target"`
	Backend        string   `json:"backend"`
	URL            string   `json:"url"`
	Size           int64    `json:"size"`
	SHA256         string   `json:"sha256"`
	ManifestSHA256 string   `json:"manifest_sha256"`
	UnpackedSize   int64    `json:"unpacked_size"`
	Requirements   []string `json:"requirements"`
}

type runtimeImage struct {
	Backend   string `json:"backend"`
	Reference string `json:"reference"`
}

type runtimeCatalog struct {
	Version  string           `json:"version"`
	Packages []runtimePackage `json:"packages"`
	Images   []runtimeImage   `json:"images"`
}

// Set only by the release generator after anonymous artifact verification.
// An unpublished release has no download authority; never synthesize pins.
var releaseRuntimeCatalog runtimeCatalog

func (entry runtimePackage) asset() onnxrt.AssetSpec {
	return onnxrt.AssetSpec{ID: entry.ID, URL: entry.URL, Size: entry.Size, SHA256: entry.SHA256, Kind: onnxrt.AssetZip}
}

func (catalog runtimeCatalog) native(goos, goarch, backend string) (runtimePackage, bool) {
	for _, entry := range catalog.Packages {
		if entry.Target == runtimeTarget(goos, goarch) && entry.Backend == backend {
			return entry, true
		}
	}
	return runtimePackage{}, false
}

func (catalog runtimeCatalog) image(backend string) string {
	for _, entry := range catalog.Images {
		if entry.Backend == backend {
			return entry.Reference
		}
	}
	return ""
}

func runtimeInstallBackends(cpuAllowed bool) []string {
	var result []string
	if runtime.GOOS == "darwin" {
		result = []string{"metal"}
	} else {
		_, nvidiaErr := os.Stat("/dev/nvidiactl")
		_, smiErr := exec.LookPath("nvidia-smi")
		if nvidiaErr == nil || smiErr == nil {
			result = append(result, "cuda")
		}
		result = append(result, "vulkan")
	}
	if cpuAllowed {
		result = append(result, "cpu")
	}
	return result
}

func ensureNativeRuntime(ctx context.Context, sourceRoot, managedRoot string, cpuAllowed bool, progress func(string, int64, int64)) error {
	var integrityErr error
	available := make(map[string]bool)
	for _, backend := range runtimeBackends(runtime.GOOS) {
		bundles, err := nativeRuntimeCandidates(ctx, sourceRoot, managedRoot, backend)
		if ErrorCode(err) == CodeCancelled {
			return err
		}
		if ErrorCode(err) == CodeRuntimeAttestationFailed {
			integrityErr = err
		}
		if len(bundles) == 0 {
			continue
		}
		bundle := bundles[0]
		source := filepath.Join(sourceRoot, "bin", "decisions-runtime", runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)
		if bundle.directory == source {
			if err := copyVerifiedRuntimeBundle(ctx, source, filepath.Join(managedRoot, runtimeTarget(runtime.GOOS, runtime.GOARCH), backend)); err != nil {
				return err
			}
		}
		available[backend] = true
	}
	cache := onnxrt.NewAssetCache(filepath.Join(managedRoot, "packages"), "AuraGo-Decisions", func(status onnxrt.DownloadStatus) {
		if progress != nil {
			progress("downloading_runtime", status.Downloaded, status.Total)
		}
	})
	found := len(available) > 0
	for _, backend := range runtimeInstallBackends(cpuAllowed) {
		if available[backend] {
			continue
		}
		entry, ok := releaseRuntimeCatalog.native(runtime.GOOS, runtime.GOARCH, backend)
		if !ok {
			continue
		}
		found = true
		if err := onnxrt.ValidateAssetSpec(entry.asset()); err != nil {
			return NewError(CodeRuntimeAttestationFailed, err)
		}
		if !validSHA256(entry.ManifestSHA256) || entry.UnpackedSize <= 0 || !strings.HasPrefix(entry.URL, "https://github.com/antibyte/AuraGo/releases/download/decisions-runtime-v") {
			return NewError(CodeRuntimeAttestationFailed, fmt.Errorf("invalid runtime catalog"))
		}
		directory := onnxrt.RuntimeDir(cache.Root(), entry.asset())
		if validateDownloadedRuntime(ctx, directory, backend) == nil {
			if _, _, err := validateRuntimeManifest(ctx, directory, backend, runtime.GOOS, runtime.GOARCH); err == nil {
				continue
			}
		}
		if ctx.Err() != nil {
			return NewError(CodeCancelled, ctx.Err())
		}
		if err := requireRuntimeDiskSpace(managedRoot, entry.Size+entry.UnpackedSize); err != nil {
			return err
		}
		directory, err := cache.EnsureRuntime(ctx, entry.asset())
		if ctx.Err() != nil {
			return NewError(CodeCancelled, ctx.Err())
		}
		if err != nil {
			if integrityErr != nil {
				return NewError(CodeRuntimeAttestationFailed, errors.Join(integrityErr, err))
			}
			return NewError("runtime_download_failed", err)
		}
		if err := validateDownloadedRuntime(ctx, directory, backend); err != nil {
			return err
		}
		if _, _, err := validateRuntimeManifest(ctx, directory, backend, runtime.GOOS, runtime.GOARCH); err != nil {
			return NewError(CodeRuntimeAttestationFailed, err)
		}
	}
	if !found {
		if integrityErr != nil {
			return integrityErr
		}
		return NewError("runtime_release_unavailable", nil)
	}
	return nil
}

func validateDownloadedRuntime(ctx context.Context, directory, backend string) error {
	entry, ok := releaseRuntimeCatalog.native(runtime.GOOS, runtime.GOARCH, backend)
	if !ok || filepath.Base(directory) != entry.ID {
		return nil
	}
	digest, err := fileSHA256(ctx, filepath.Join(directory, "release-manifest.json"))
	if err != nil || !strings.EqualFold(digest, entry.ManifestSHA256) {
		return NewError(CodeRuntimeAttestationFailed, err)
	}
	return nil
}

func requireRuntimeDiskSpace(path string, needed int64) error {
	available, err := fileutil.FreeDiskBytes(path)
	if err != nil {
		return NewError("disk_space_unavailable", err)
	}
	if available < needed+(64<<20) {
		return NewError("insufficient_disk_space", nil)
	}
	return nil
}
