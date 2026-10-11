package onnxrt

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var testWheelMembers = []string{
	"onnxruntime/capi/libonnxruntime.so.1.27.0",
	"onnxruntime/capi/libonnxruntime_providers_shared.so",
}

func serveTestWheel(t *testing.T, members []string) (*AssetCache, AssetSpec) {
	t.Helper()
	content := buildTestZip(t, map[string]string{
		"onnxruntime/capi/libonnxruntime.so.1.27.0":           "runtime library",
		"onnxruntime/capi/libonnxruntime_providers_shared.so": "shared providers",
		"onnxruntime/__init__.py":                             "python package",
		"onnxruntime_webgpu-1.27.0.dist-info/RECORD":          "record",
	})
	cache, baseURL := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	spec := pinnedSpec("ort-test-webgpu", baseURL+"/onnxruntime_webgpu.whl", AssetWheel, content)
	spec.Members = members
	return cache, spec
}

func TestEnsureRuntimeExtractsWheelMembersFlattened(t *testing.T) {
	cache, spec := serveTestWheel(t, testWheelMembers)
	root, err := cache.EnsureRuntime(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if got := strings.Join(names, ","); got != ".complete.json,libonnxruntime.so.1.27.0,libonnxruntime_providers_shared.so" {
		t.Fatalf("runtime entries = %s", got)
	}
	raw, err := os.ReadFile(filepath.Join(root, "libonnxruntime.so.1.27.0"))
	if err != nil || string(raw) != "runtime library" {
		t.Fatalf("runtime library = %q, %v", raw, err)
	}
	archive := ArchivePath(cache.Root(), spec)
	if _, err := os.Stat(archive); err != nil || !strings.HasSuffix(archive, ".whl") {
		t.Fatalf("wheel archive %s: %v", archive, err)
	}
	again, err := cache.EnsureRuntime(context.Background(), spec)
	if err != nil || again != root {
		t.Fatalf("second EnsureRuntime = %q, %v", again, err)
	}
}

func TestEnsureRuntimeRejectsWheelWithMissingMember(t *testing.T) {
	members := append(append([]string(nil), testWheelMembers...), "onnxruntime/capi/dxil.dll")
	cache, spec := serveTestWheel(t, members)
	_, err := cache.EnsureRuntime(context.Background(), spec)
	if err == nil || !strings.Contains(err.Error(), "wheel is missing onnxruntime/capi/dxil.dll") {
		t.Fatalf("EnsureRuntime error = %v", err)
	}
	if _, statErr := os.Stat(RuntimeDir(cache.Root(), spec)); !os.IsNotExist(statErr) {
		t.Fatalf("incomplete runtime directory exists: %v", statErr)
	}
}

func TestValidateAssetSpecChecksWheelMembers(t *testing.T) {
	base := pinnedSpec("wheel", "https://example.invalid/x.whl", AssetWheel, []byte("x"))
	cases := map[string][]string{
		"empty":     nil,
		"traversal": {"../libonnxruntime.so"},
		"absolute":  {"/onnxruntime/capi/onnxruntime.dll"},
		"backslash": {`onnxruntime\capi\onnxruntime.dll`},
		"unclean":   {"onnxruntime/./capi/onnxruntime.dll"},
		"duplicate": {"a/onnxruntime.dll", "b/onnxruntime.dll"},
	}
	for name, members := range cases {
		spec := base
		spec.Members = members
		if err := ValidateAssetSpec(spec); err == nil {
			t.Errorf("%s members accepted: %v", name, members)
		}
	}
	spec := base
	spec.Members = testWheelMembers
	if err := ValidateAssetSpec(spec); err != nil {
		t.Fatalf("valid wheel members rejected: %v", err)
	}
}
