package llamacpp

import (
	"strings"
	"testing"

	"aurago/internal/onnxrt"
)

func TestRuntimeManifestPinsBuild(t *testing.T) {
	runtimes := Runtimes()
	if len(runtimes) != 13 {
		t.Fatalf("manifest has %d entries, want 13", len(runtimes))
	}
	for id, spec := range runtimes {
		if spec.ID != id {
			t.Errorf("%s: ID = %q", id, spec.ID)
		}
		if !strings.Contains(spec.URL, "/"+Build+"/") {
			t.Errorf("%s: URL %q does not pin %s", id, spec.URL, Build)
		}
		if spec.Kind != onnxrt.AssetZip && spec.Kind != onnxrt.AssetTarGzip {
			t.Errorf("%s: kind %q", id, spec.Kind)
		}
		if err := onnxrt.ValidateAssetSpec(spec); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	before, ok := RuntimeAsset("linux", "amd64", "cpu")
	if !ok {
		t.Fatal("linux/amd64/cpu missing")
	}
	runtimes["llama-linux-amd64-cpu"] = AssetSpec{ID: "changed"}
	delete(runtimes, "llama-linux-arm64-cpu")
	after, _ := RuntimeAsset("linux", "amd64", "cpu")
	if after.ID != before.ID || after.SHA256 != before.SHA256 {
		t.Fatal("changing the copy changed RuntimeAsset")
	}
	if _, ok := RuntimeAsset("linux", "arm64", "cpu"); !ok {
		t.Fatal("deleting from the copy removed a manifest entry")
	}
}

func TestRuntimeAssetAndSupplements(t *testing.T) {
	asset, ok := RuntimeAsset("darwin", "arm64", "cpu")
	if !ok || asset.ID != "llama-darwin-arm64-metal" {
		t.Fatalf("darwin/arm64/cpu = %+v, %v", asset, ok)
	}
	if _, ok := RuntimeAsset("windows", "arm64", "cuda"); ok {
		t.Fatal("windows/arm64/cuda must not resolve")
	}
	for _, tc := range []struct {
		goos, goarch, backend string
		want                  int
	}{
		{"windows", "amd64", "cuda", 1},
		{"windows", "amd64", "vulkan", 0},
		{"linux", "amd64", "cuda", 0},
		{"windows", "arm64", "cuda", 0},
	} {
		got := SupplementalAssets(tc.goos, tc.goarch, tc.backend)
		if len(got) != tc.want {
			t.Fatalf("%s/%s/%s: %d archives, want %d", tc.goos, tc.goarch, tc.backend, len(got), tc.want)
		}
		if tc.want == 1 && got[0].ID != WindowsCUDARuntimeID {
			t.Fatalf("supplement = %q", got[0].ID)
		}
	}
}

func TestDockerImageDigestPinned(t *testing.T) {
	for _, backend := range []string{"cpu", "cuda", "vulkan"} {
		image, ok := DockerImage(backend)
		if !ok || !strings.Contains(image, "@sha256:") {
			t.Fatalf("%s: %q, %v", backend, image, ok)
		}
	}
	if _, ok := DockerImage("metal"); ok {
		t.Fatal("metal has no Docker image")
	}
}
