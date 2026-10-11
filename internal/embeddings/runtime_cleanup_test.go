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
	foreign := []string{"ort-linux-amd64-webgpu", "ort-windows-amd64-webgpu", "ort-darwin-arm64-webgpu"}
	for _, id := range foreign {
		writeFile(filepath.Join(cacheDir, "runtimes", id, "libonnxruntime.bin"))
		writeFile(filepath.Join(cacheDir, "downloads", id+".whl"))
	}
	cuda, ok := onnxrt.CUDARuntimeAsset("linux", "amd64")
	if !ok {
		t.Fatal("linux/amd64 CUDA runtime asset missing")
	}
	writeFile(filepath.Join(cacheDir, "runtimes", cuda.ID, "lib", "libonnxruntime.bin"))
	writeFile(onnxrt.ArchivePath(cacheDir, cuda))

	candidates := candidateMatrix("linux", "amd64", "auto", hardwareInfo{NVIDIA: true, Vulkan: true})
	selected, ok := findCandidate(candidates, "onnx-cpu")
	if !ok {
		t.Fatal("onnx-cpu candidate missing")
	}
	manager := &Manager{options: LocalOptions{CacheDir: cacheDir}, logger: discardEmbeddingLogger()}
	manager.cleanupUnusedGPURuntimes(selected, candidates)

	for _, id := range foreign {
		if _, err := os.Stat(filepath.Join(cacheDir, "runtimes", id, "libonnxruntime.bin")); err != nil {
			t.Fatalf("foreign runtime %s was removed: %v", id, err)
		}
		if _, err := os.Stat(filepath.Join(cacheDir, "downloads", id+".whl")); err != nil {
			t.Fatalf("foreign archive %s was removed: %v", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "runtimes", cuda.ID)); !os.IsNotExist(err) {
		t.Fatalf("unused CUDA runtime was kept: %v", err)
	}
	if _, err := os.Stat(onnxrt.ArchivePath(cacheDir, cuda)); !os.IsNotExist(err) {
		t.Fatalf("unused CUDA archive was kept: %v", err)
	}
}
