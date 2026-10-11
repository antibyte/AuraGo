package persondetect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	"aurago/internal/onnxrt"
)

// gpuModelManifest is the sidecar of a cached WebGPU model.
type gpuModelManifest struct {
	SourceSHA256 string `json:"source_sha256"`
	ORTVersion   string `json:"ort_version"`
	PatchedNodes int    `json:"patched_nodes"`
}

// gpuModelPaths are <ModelDir>/<tier>/<stem>.webgpu-<version>.onnx and its
// .json sidecar, <stem> being the pinned asset's file name without extension
// ("model").
func (s *setup) gpuModelPaths() (string, string) {
	name := s.spec.Asset.FileName
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	base := filepath.Join(s.opts.ModelDir, string(s.spec.Tier), stem+".webgpu-"+onnxrt.WebGPURuntimeVersion)
	return base + ".onnx", base + ".json"
}

// prepareGPUModel reuses the cached patched model when its sidecar names the
// pinned source SHA-256 and the WebGPU runtime version; otherwise it runs
// prepare in a worker subprocess and writes the sidecar. It holds the model
// file's lock throughout, so concurrent setups never prepare one output at the
// same time, and returns only after the prepare subprocess has exited.
func (s *setup) prepareGPUModel(ctx context.Context, runtimeRoot, modelPath string) (string, error) {
	gpuPath, manifestPath := s.gpuModelPaths()
	unlock, err := lockGPUModel(ctx, gpuPath)
	if err != nil {
		return "", err
	}
	defer unlock()
	if cachedGPUModelValid(gpuPath, manifestPath, s.spec.Asset.SHA256) {
		return gpuPath, nil
	}
	_ = os.Remove(manifestPath)
	count, err := runPrepare(ctx, s.opts.Executable, runtimeRoot, modelPath, gpuPath, s.deps.startTimeout)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(gpuModelManifest{
		SourceSHA256: s.spec.Asset.SHA256,
		ORTVersion:   onnxrt.WebGPURuntimeVersion,
		PatchedNodes: count,
	})
	if err != nil {
		return "", fmt.Errorf("marshal GPU model manifest: %w", err)
	}
	if err := writeFileAtomic(manifestPath, raw, 0o640); err != nil {
		return "", err
	}
	return gpuPath, nil
}

func cachedGPUModelValid(gpuPath, manifestPath, sourceSHA256 string) bool {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}
	var manifest gpuModelManifest
	if json.Unmarshal(raw, &manifest) != nil {
		return false
	}
	if !strings.EqualFold(manifest.SourceSHA256, sourceSHA256) ||
		manifest.ORTVersion != onnxrt.WebGPURuntimeVersion || manifest.PatchedNodes < 1 {
		return false
	}
	info, err := os.Stat(gpuPath)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

// discardGPUModel drops the cached WebGPU model so the next setup prepares it
// again; the sidecar goes first so a half-removed cache never looks valid. It
// takes the model file's lock and gives up when ctx ends first.
func (s *setup) discardGPUModel(ctx context.Context) {
	gpuPath, manifestPath := s.gpuModelPaths()
	unlock, err := lockGPUModel(ctx, gpuPath)
	if err != nil {
		return
	}
	defer unlock()
	_ = os.Remove(manifestPath)
	_ = os.Remove(gpuPath)
}

// gpuModelLocks serializes the prepares and discards of one WebGPU model file
// within the process: a prepare sweeps the temp files an overlapping prepare
// of the same output is still writing and makes it fail
// (sweepStalePrepareFiles). Each value is a chan struct{} of capacity 1, so
// waiting for it can honour a context.
var gpuModelLocks sync.Map

func lockGPUModel(ctx context.Context, path string) (unlock func(), err error) {
	value, _ := gpuModelLocks.LoadOrStore(gpuModelLockKey(path), make(chan struct{}, 1))
	slot := value.(chan struct{})
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// gpuModelLockKey is the absolute path, lower-cased where file names are
// usually case-insensitive.
func gpuModelLockKey(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if goruntime.GOOS == "windows" || goruntime.GOOS == "darwin" {
		path = strings.ToLower(path)
	}
	return path
}
