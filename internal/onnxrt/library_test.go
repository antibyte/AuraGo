package onnxrt

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenLibraryReportsMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-library")
	library, err := OpenLibrary(missing)
	if err == nil {
		_ = library.Close()
		t.Fatal("OpenLibrary accepted a missing file")
	}
	if !strings.Contains(err.Error(), "load native library") {
		t.Fatalf("OpenLibrary error = %v", err)
	}
}

func TestClosedLibraryIsSafe(t *testing.T) {
	var library *Library
	if err := library.Close(); err != nil {
		t.Fatalf("nil library Close = %v", err)
	}
	if _, err := library.Lookup("OrtGetApiBase"); err == nil {
		t.Fatal("nil library Lookup succeeded")
	}
}
