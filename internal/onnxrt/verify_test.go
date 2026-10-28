package onnxrt

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyRuntimeNeverRepairsOrDownloads(t *testing.T) {
	archive := buildTestZip(t, map[string]string{"runtime/onnxruntime.dll": "fixture native library"})
	hits := 0
	cache, base := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; _, _ = w.Write(archive) }))
	spec := pinnedSpec("verify-fixture", base+"/runtime.zip", AssetZip, archive)
	if _, err := cache.VerifyRuntime(context.Background(), spec); err == nil || hits != 0 {
		t.Fatal("passive check installed missing runtime")
	}
	dir, err := cache.EnsureRuntime(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.VerifyRuntime(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("unexpected downloads %d", hits)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime", "onnxruntime.dll"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.VerifyRuntime(context.Background(), spec); err == nil || hits != 1 {
		t.Fatal("corrupt runtime repaired or trusted")
	}
}
