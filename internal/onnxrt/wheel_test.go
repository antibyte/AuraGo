package onnxrt

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
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

func runtimeEntryNames(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func TestEnsureRuntimeExtractsWheelMembersFlattened(t *testing.T) {
	cache, spec := serveTestWheel(t, testWheelMembers)
	root, err := cache.EnsureRuntime(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if got := runtimeEntryNames(t, root); got != ".complete.json,libonnxruntime.so.1.27.0,libonnxruntime_providers_shared.so" {
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

// The archive SHA-256 alone does not pin a wheel runtime: the extracted files
// also depend on Members, so a changed member list must re-extract.
func TestEnsureRuntimeReextractsWheelWhenMembersChange(t *testing.T) {
	library := testWheelMembers[0]
	cache, spec := serveTestWheel(t, []string{library})
	root, err := cache.EnsureRuntime(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if got := runtimeEntryNames(t, root); got != ".complete.json,libonnxruntime.so.1.27.0" {
		t.Fatalf("runtime entries with one member = %s", got)
	}

	spec.Members = append([]string(nil), testWheelMembers...)
	if _, err := cache.EnsureRuntime(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if got := runtimeEntryNames(t, root); got != ".complete.json,libonnxruntime.so.1.27.0,libonnxruntime_providers_shared.so" {
		t.Fatalf("runtime entries after adding a member = %s", got)
	}

	spec.Members = []string{library}
	if _, err := cache.EnsureRuntime(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if got := runtimeEntryNames(t, root); got != ".complete.json,libonnxruntime.so.1.27.0" {
		t.Fatalf("runtime entries after removing a member = %s", got)
	}
}

type testWheelEntry struct {
	name string
	mode os.FileMode // 0: a plain file created with zip.Writer.Create
	body string
}

func writeTestWheel(t *testing.T, entries []testWheelEntry) string {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, entry := range entries {
		var (
			file io.Writer
			err  error
		)
		if entry.mode == 0 {
			file, err = writer.Create(entry.name)
		} else {
			header := &zip.FileHeader{Name: entry.name}
			header.SetMode(entry.mode)
			file, err = writer.CreateHeader(header)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "test.whl")
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func TestExtractWheelMembersRejectsUnsafeEntries(t *testing.T) {
	member := "pkg/lib.so"
	cases := []struct {
		name    string
		entries []testWheelEntry
		want    string
	}{
		{"symlink", []testWheelEntry{{name: member, mode: os.ModeSymlink | 0o777, body: "../outside"}}, "is not a regular file"},
		{"directory mode", []testWheelEntry{{name: member, mode: os.ModeDir | 0o755}}, "is not a regular file"},
		{"duplicate", []testWheelEntry{{name: member, body: "first"}, {name: member, body: "second"}}, "appears more than once"},
		{"backslash duplicate", []testWheelEntry{{name: member, body: "first"}, {name: `pkg\lib.so`, body: "second"}}, "appears more than once"},
		{"directory entry", []testWheelEntry{{name: member + "/"}}, "wheel is missing pkg/lib.so"},
	}
	for _, tc := range cases {
		archivePath := writeTestWheel(t, tc.entries)
		target := t.TempDir()
		err := extractWheelMembers(archivePath, target, []string{member})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: extractWheelMembers error = %v, want %q", tc.name, err, tc.want)
		}
		if tc.name == "symlink" || tc.name == "directory mode" || tc.name == "directory entry" {
			if got := runtimeEntryNames(t, target); got != "" {
				t.Errorf("%s: target holds %s", tc.name, got)
			}
		}
	}
}

func TestValidateAssetSpecChecksWheelMembers(t *testing.T) {
	base := pinnedSpec("wheel", "https://example.invalid/x.whl", AssetWheel, []byte("x"))
	cases := map[string][]string{
		"empty":           nil,
		"traversal":       {"../libonnxruntime.so"},
		"absolute":        {"/onnxruntime/capi/onnxruntime.dll"},
		"backslash":       {`onnxruntime\capi\onnxruntime.dll`},
		"unclean":         {"onnxruntime/./capi/onnxruntime.dll"},
		"trailing slash":  {"onnxruntime/capi/"},
		"dot":             {"."},
		"duplicate":       {"a/onnxruntime.dll", "b/onnxruntime.dll"},
		"case duplicate":  {"a/onnxruntime.dll", "b/ONNXRuntime.DLL"},
		"runtime marker":  {"onnxruntime/.complete.json"},
		"hidden":          {"onnxruntime/.hidden.so"},
		"trailing dot":    {"onnxruntime/capi/onnxruntime.dll."},
		"trailing space":  {"onnxruntime/capi/onnxruntime.dll "},
		"colon":           {"onnxruntime/capi/onnxruntime.dll:stream"},
		"single char":     {"onnxruntime/capi/a"},
		"leading dash":    {"onnxruntime/capi/-lib.so"},
		"space in middle": {"onnxruntime/capi/lib onnx.so"},
	}
	for name, members := range cases {
		spec := base
		spec.Members = members
		if err := ValidateAssetSpec(spec); err == nil {
			t.Errorf("%s members accepted: %v", name, members)
		}
	}
	for _, members := range [][]string{
		testWheelMembers,
		{"pkg/libc++_shared.so", "pkg/DirectML.dll", "pkg/libonnxruntime.1.27.0.dylib"},
	} {
		spec := base
		spec.Members = members
		if err := ValidateAssetSpec(spec); err != nil {
			t.Errorf("valid wheel members %v rejected: %v", members, err)
		}
	}
}

func TestValidateAssetSpecRejectsMembersOnNonWheelKinds(t *testing.T) {
	for _, kind := range []AssetKind{AssetFile, AssetZip, AssetTarGzip} {
		spec := pinnedSpec("not-a-wheel", "https://example.invalid/x.bin", kind, []byte("x"))
		if err := ValidateAssetSpec(spec); err != nil {
			t.Errorf("%s spec without members rejected: %v", kind, err)
		}
		spec.Members = []string{"pkg/lib.so"}
		if err := ValidateAssetSpec(spec); err == nil {
			t.Errorf("%s spec with members accepted", kind)
		}
	}
}
