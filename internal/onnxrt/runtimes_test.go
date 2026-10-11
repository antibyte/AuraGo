package onnxrt

import (
	"strings"
	"testing"
)

type wantRuntime struct {
	id      string
	kind    AssetKind
	members int
}

func TestRuntimeAssetLookupsPerPlatform(t *testing.T) {
	cases := []struct {
		goos, goarch      string
		cpu, cuda, webgpu *wantRuntime
	}{
		{"linux", "amd64", &wantRuntime{"ort-linux-amd64-cpu", AssetTarGzip, 0}, &wantRuntime{"ort-linux-amd64-cuda", AssetTarGzip, 0}, &wantRuntime{"ort-linux-amd64-webgpu", AssetWheel, 2}},
		{"linux", "arm64", &wantRuntime{"ort-linux-arm64-cpu", AssetTarGzip, 0}, nil, nil},
		{"windows", "amd64", &wantRuntime{"ort-windows-amd64-cpu", AssetZip, 0}, &wantRuntime{"ort-windows-amd64-cuda", AssetZip, 0}, &wantRuntime{"ort-windows-amd64-webgpu", AssetWheel, 4}},
		{"windows", "arm64", &wantRuntime{"ort-windows-arm64-cpu", AssetZip, 0}, nil, nil},
		{"darwin", "arm64", &wantRuntime{"ort-darwin-arm64-cpu", AssetTarGzip, 0}, nil, &wantRuntime{"ort-darwin-arm64-webgpu", AssetWheel, 1}},
		{"darwin", "amd64", nil, nil, nil},
	}
	check := func(label string, spec AssetSpec, ok bool, want *wantRuntime) {
		t.Helper()
		if want == nil {
			if ok {
				t.Errorf("%s: unexpected asset %s", label, spec.ID)
			}
			return
		}
		if !ok || spec.ID != want.id || spec.Kind != want.kind || len(spec.Members) != want.members {
			t.Errorf("%s = %+v, %v; want %+v", label, spec, ok, *want)
		}
	}
	for _, tc := range cases {
		label := tc.goos + "/" + tc.goarch
		spec, ok := CPURuntimeAsset(tc.goos, tc.goarch)
		check(label+" cpu", spec, ok, tc.cpu)
		spec, ok = CUDARuntimeAsset(tc.goos, tc.goarch)
		check(label+" cuda", spec, ok, tc.cuda)
		spec, ok = WebGPURuntimeAsset(tc.goos, tc.goarch)
		check(label+" webgpu", spec, ok, tc.webgpu)
	}
}

func TestRuntimeAssetsArePinned(t *testing.T) {
	if CPURuntimeVersion != "1.26.0" || WebGPURuntimeVersion != "1.27.0" {
		t.Fatalf("runtime versions = %s / %s", CPURuntimeVersion, WebGPURuntimeVersion)
	}
	for id, spec := range runtimeAssets {
		if spec.ID != id {
			t.Errorf("manifest key %s holds ID %s", id, spec.ID)
		}
		if err := ValidateAssetSpec(spec); err != nil {
			t.Errorf("%s: %v", id, err)
		}
		version := CPURuntimeVersion
		if spec.Kind == AssetWheel {
			version = WebGPURuntimeVersion
		}
		if !strings.Contains(spec.URL, version) {
			t.Errorf("%s URL %s does not name %s", id, spec.URL, version)
		}
	}
	pins := map[string]struct {
		sha  string
		size int64
	}{
		"linux":   {"fe5e49e00ad71bc667316d57670bfbf525438783d4a3468e0d337314ec985cbe", 25_163_729},
		"windows": {"7ef99275b13e8cb9584bd0db7a6f00ebf76095601eeccf7d34749b89ee991c19", 27_226_899},
		"darwin":  {"8e6c43281318283a31977ebe752e7b0454db9b2dc42c768ac88641952357196a", 24_081_620},
	}
	for goos, pin := range pins {
		goarch := "amd64"
		if goos == "darwin" {
			goarch = "arm64"
		}
		spec, ok := WebGPURuntimeAsset(goos, goarch)
		if !ok || spec.SHA256 != pin.sha || spec.Size != pin.size {
			t.Errorf("%s WebGPU wheel = %+v", goos, spec)
		}
	}
}

func TestRuntimeAssetLookupReturnsCopies(t *testing.T) {
	first, _ := WebGPURuntimeAsset("linux", "amd64")
	first.Members[0] = "mutated"
	second, _ := WebGPURuntimeAsset("linux", "amd64")
	if second.Members[0] == "mutated" {
		t.Fatal("WebGPURuntimeAsset exposes the manifest's member slice")
	}
}

// Every spec a runtime lookup returns goes straight into EnsureRuntime, so it
// must pass ValidateAssetSpec and name an archive kind EnsureRuntime unpacks.
// Every manifest entry must be reachable through a lookup.
func TestRuntimeAssetLookupsReturnInstallableSpecs(t *testing.T) {
	platforms := []struct{ goos, goarch string }{
		{"linux", "amd64"}, {"linux", "arm64"}, {"linux", "arm"},
		{"windows", "amd64"}, {"windows", "arm64"},
		{"darwin", "amd64"}, {"darwin", "arm64"},
	}
	lookups := map[string]func(goos, goarch string) (AssetSpec, bool){
		"cpu":    CPURuntimeAsset,
		"cuda":   CUDARuntimeAsset,
		"webgpu": WebGPURuntimeAsset,
	}
	reached := make(map[string]bool, len(runtimeAssets))
	for _, platform := range platforms {
		for backend, lookup := range lookups {
			spec, ok := lookup(platform.goos, platform.goarch)
			if !ok {
				continue
			}
			label := platform.goos + "/" + platform.goarch + " " + backend
			if err := ValidateAssetSpec(spec); err != nil {
				t.Errorf("%s: %v", label, err)
			}
			if err := validateRuntimeArchiveKind(spec); err != nil {
				t.Errorf("%s: %v", label, err)
			}
			reached[spec.ID] = true
		}
	}
	for id := range runtimeAssets {
		if !reached[id] {
			t.Errorf("runtime asset %s is not returned by any lookup", id)
		}
	}
}
