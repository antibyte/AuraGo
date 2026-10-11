package persondetect

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

// TestPrepareIntegration runs prepare in-process for both tiers with the
// runtime production uses for it (the WebGPU wheel where one exists). Keep it
// the only test that loads ONNX Runtime into the test process itself, and
// only one ONNX Runtime library: detector tests use worker subprocesses.
func TestPrepareIntegration(t *testing.T) {
	requireIntegration(t)
	runtimeRoot := integrationRuntime(t, true)
	if goruntime.GOOS == "windows" {
		t.Setenv("PATH", runtimeRoot+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	for _, tier := range []ModelTier{ModelStandard, ModelAccurate} {
		t.Run(string(tier), func(t *testing.T) {
			spec, _ := Model(tier)
			input := integrationModel(t, tier)
			outputDir := t.TempDir()
			output := filepath.Join(outputDir, "model.webgpu.onnx")
			count, verification, err := prepareModel(runtimeRoot, input, output)
			if err != nil {
				t.Fatal(err)
			}
			// Both folded D-FINE graphs have three integral MatMuls sharing
			// one [33] initializer (overview facts).
			if count != 3 {
				t.Fatalf("patched %d MatMul nodes, want 3 for %s", count, spec.Name)
			}
			info, err := os.Stat(output)
			if err != nil || info.Size() == 0 {
				t.Fatalf("patched model: %v", err)
			}
			entries, err := os.ReadDir(outputDir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("output directory holds %d entries (folded or temp files left?): %v", len(entries), err)
			}
			t.Logf("prepared %s: %d -> %d bytes, %d node(s) patched; reference check: %s", spec.Name, spec.Asset.Size, info.Size(), count, verification)
		})
	}
}
