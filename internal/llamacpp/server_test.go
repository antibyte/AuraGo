package llamacpp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerHelpers(t *testing.T) {
	root := t.TempDir()
	if _, err := FindServer(root); err == nil || !strings.Contains(err.Error(), "not found below") {
		t.Fatalf("empty root: err = %v", err)
	}
	name := "LLAMA-SERVER"
	if filepath.Separator == '\\' {
		name += ".EXE"
	}
	dir := filepath.Join(root, "nested")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, name)
	if err := os.WriteFile(want, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindServer(root)
	if err != nil || got != want {
		t.Fatalf("FindServer = %q, %v, want %q", got, err, want)
	}

	port, err := LoopbackPort()
	if err != nil || port <= 0 {
		t.Fatalf("LoopbackPort = %d, %v", port, err)
	}
	first, err := RandomAPIKey()
	if err != nil || len(first) != 43 {
		t.Fatalf("RandomAPIKey = %q, %v", first, err)
	}
	if second, _ := RandomAPIKey(); second == first {
		t.Fatal("two API keys are equal")
	}

	for _, tc := range []struct {
		backend, logs string
		want          bool
	}{
		{"vulkan", "using device Vulkan0; offloaded 13/13 layers to GPU", true},
		{"vulkan", "using device CPU only", false},
		{"cuda", "offloaded 13/13 layers to GPU", false},
		{"metal", "ggml_metal_init: using device; offloaded 25/25 layers", true},
		{"metal", "offloaded 25/25 layers", false},
	} {
		if got := GPUVerified(tc.backend, tc.logs); got != tc.want {
			t.Errorf("GPUVerified(%q, %q) = %v, want %v", tc.backend, tc.logs, got, tc.want)
		}
	}
}
