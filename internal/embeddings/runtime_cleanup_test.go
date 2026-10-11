package embeddings

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/onnxrt"
)

// Person detection keeps its WebGPU runtimes in the shared embeddings runtime
// cache; embeddings' GPU cleanup must only remove its own candidates.
func TestCleanupUnusedGPURuntimesKeepsForeignRuntimes(t *testing.T) {
	cacheDir := t.TempDir()
	writeFile := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var foreign []onnxrt.AssetSpec
	for _, platform := range []struct{ goos, goarch string }{{"linux", "amd64"}, {"windows", "amd64"}, {"darwin", "arm64"}} {
		spec, ok := onnxrt.WebGPURuntimeAsset(platform.goos, platform.goarch)
		if !ok {
			t.Fatalf("%s/%s WebGPU runtime asset missing", platform.goos, platform.goarch)
		}
		foreign = append(foreign, spec)
		writeFile(filepath.Join(onnxrt.RuntimeDir(cacheDir, spec), "libonnxruntime.bin"))
		writeFile(onnxrt.ArchivePath(cacheDir, spec))
	}
	cuda, ok := onnxrt.CUDARuntimeAsset("linux", "amd64")
	if !ok {
		t.Fatal("linux/amd64 CUDA runtime asset missing")
	}
	writeFile(filepath.Join(onnxrt.RuntimeDir(cacheDir, cuda), "lib", "libonnxruntime.bin"))
	writeFile(onnxrt.ArchivePath(cacheDir, cuda))

	candidates := candidateMatrix("linux", "amd64", "auto", hardwareInfo{NVIDIA: true, Vulkan: true})
	selected, ok := findCandidate(candidates, "onnx-cpu")
	if !ok {
		t.Fatal("onnx-cpu candidate missing")
	}
	manager := &Manager{options: LocalOptions{CacheDir: cacheDir}, logger: discardEmbeddingLogger()}
	manager.cleanupUnusedGPURuntimes(selected, candidates)

	for _, spec := range foreign {
		if _, err := os.Stat(filepath.Join(onnxrt.RuntimeDir(cacheDir, spec), "libonnxruntime.bin")); err != nil {
			t.Fatalf("foreign runtime %s was removed: %v", spec.ID, err)
		}
		if _, err := os.Stat(onnxrt.ArchivePath(cacheDir, spec)); err != nil {
			t.Fatalf("foreign archive %s was removed: %v", spec.ID, err)
		}
	}
	if _, err := os.Stat(onnxrt.RuntimeDir(cacheDir, cuda)); !os.IsNotExist(err) {
		t.Fatalf("unused CUDA runtime was kept: %v", err)
	}
	if _, err := os.Stat(onnxrt.ArchivePath(cacheDir, cuda)); !os.IsNotExist(err) {
		t.Fatalf("unused CUDA archive was kept: %v", err)
	}
}
