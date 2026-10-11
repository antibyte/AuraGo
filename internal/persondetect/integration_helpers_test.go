package persondetect

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
	"time"

	"aurago/internal/onnxrt"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("AURAGO_PERSONDETECT_INTEGRATION") != "1" {
		t.Skip("set AURAGO_PERSONDETECT_INTEGRATION=1 to download ONNX Runtime and D-FINE and run them")
	}
}

// integrationCacheDir mirrors the data directory: <cache>/embeddings is the
// shared runtime cache, <cache>/persondetect/models the model directory.
func integrationCacheDir(t *testing.T) string {
	t.Helper()
	root := os.Getenv("AURAGO_PERSONDETECT_CACHE")
	if root == "" {
		root = filepath.Join("..", "..", "disposable", "person-detection", "cache")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		t.Fatal(err)
	}
	return absolute
}

// integrationRuntime installs the WebGPU wheel when preferWebGPU is set and
// one exists for this platform, otherwise the CPU bundle.
func integrationRuntime(t *testing.T, preferWebGPU bool) string {
	t.Helper()
	spec, ok := onnxrt.WebGPURuntimeAsset(goruntime.GOOS, goruntime.GOARCH)
	if !preferWebGPU || !ok {
		spec, ok = onnxrt.CPURuntimeAsset(goruntime.GOOS, goruntime.GOARCH)
	}
	if !ok {
		t.Skipf("no pinned ONNX Runtime for %s/%s", goruntime.GOOS, goruntime.GOARCH)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	root, err := onnxrt.NewAssetCache(filepath.Join(integrationCacheDir(t), "embeddings"), "AuraGo/persondetect-test", nil).EnsureRuntime(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// integrationModel returns <cache>/persondetect/models/<tier>/<asset file>,
// the layout the detector uses below DetectorOptions.ModelDir.
func integrationModel(t *testing.T, tier ModelTier) string {
	t.Helper()
	spec, ok := Model(tier)
	if !ok {
		t.Fatalf("unknown model tier %s", tier)
	}
	modelDir := filepath.Join(integrationCacheDir(t), "persondetect", "models")
	path := filepath.Join(modelDir, string(tier), spec.Asset.FileName)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	if err := onnxrt.NewAssetCache(modelDir, "AuraGo/persondetect-test", nil).EnsureFile(ctx, spec.Asset, path); err != nil {
		t.Fatal(err)
	}
	return path
}
