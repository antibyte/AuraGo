package persondetect

import (
	"bytes"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

// TestPrepareIntegration runs the prepare command in-process for both tiers
// with the runtime production uses for it (the WebGPU wheel where one
// exists). Keep it the only test that loads ONNX Runtime into the test process
// itself, and only one ONNX Runtime library: detector tests use worker
// subprocesses.
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
			// Leftovers of a killed prepare of the same output.
			for _, stale := range []string{output + ".folded-00000000deadbeef.onnx", output + ".tmp-00000000deadbeef"} {
				if err := os.WriteFile(stale, []byte("stale"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			code := RunWorker([]string{"prepare", "--runtime", runtimeRoot, "--in", input, "--out", output}, strings.NewReader(""), &stdout, &stderr)
			lines := decodeLines(t, stdout.Bytes())
			if code != 0 || len(lines) != 1 || lines[0]["type"] != "prepared" {
				t.Fatalf("prepare: code %d, lines %v, stderr %s", code, lines, stderr.String())
			}
			// Both folded D-FINE graphs have three integral MatMuls sharing
			// one [33] initializer (overview facts).
			if count := lines[0]["patched_nodes"]; count != float64(3) {
				t.Fatalf("patched %v MatMul nodes, want 3 for %s", count, spec.Name)
			}
			summary := strings.TrimSpace(stderr.String())
			if !strings.Contains(summary, "reference check: matched") || !strings.Contains(summary, "person(s) >= 0.5") {
				t.Fatalf("stderr summary: %q", summary)
			}
			info, err := os.Stat(output)
			if err != nil || info.Size() == 0 {
				t.Fatalf("patched model: %v", err)
			}
			entries, err := os.ReadDir(outputDir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("output directory holds %d entries (folded or temp files left?): %v", len(entries), err)
			}
			t.Logf("%s: %d -> %d bytes; %s", spec.Name, spec.Asset.Size, info.Size(), summary)
		})
	}
}
