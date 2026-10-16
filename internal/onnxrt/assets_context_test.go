package onnxrt

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCancelledCacheVerificationPreservesInstalledRuntime(t *testing.T) {
	content := buildTestZip(t, map[string]string{"runtime.bin": "verified"})
	calls := 0
	cache, base := newTestAssetCache(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write(content) }))
	spec := pinnedSpec("context-test", base+"/runtime.zip", AssetZip, content)
	directory, err := cache.EnsureRuntime(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := cache.EnsureRuntime(ctx, spec); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(directory, "runtime.bin"))
	if err != nil || string(raw) != "verified" || calls != 1 {
		t.Fatalf("cancelled verification changed installed files: %v", err)
	}
	if _, err := hashFileContext(ctx, filepath.Join(directory, "runtime.bin")); !errors.Is(err, context.Canceled) {
		t.Fatalf("hash ignored cancellation: %v", err)
	}
}

func TestRuntimeExtractionHonorsCancellation(t *testing.T) {
	content := buildTestZip(t, map[string]string{"runtime.bin": "verified"})
	archive := filepath.Join(t.TempDir(), "runtime.zip")
	if err := os.WriteFile(archive, content, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	target := t.TempDir()
	if err := extractZipSecureContext(ctx, archive, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("extract = %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "runtime.bin")); !os.IsNotExist(err) {
		t.Fatal("cancelled extraction created runtime")
	}
}
