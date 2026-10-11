package onnxrt

import (
	"path/filepath"
	"runtime"
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

func TestLibraryLifecycleWithSystemLibrary(t *testing.T) {
	var path, symbol string
	switch runtime.GOOS {
	case "windows":
		path, symbol = "kernel32.dll", "GetCurrentProcessId"
	case "linux":
		path, symbol = "libc.so.6", "getpid"
	case "darwin":
		path, symbol = "/usr/lib/libSystem.B.dylib", "getpid"
	default:
		t.Skipf("no known system library on %s", runtime.GOOS)
	}
	library, err := OpenLibrary(path)
	if err != nil {
		t.Fatalf("OpenLibrary(%s) = %v", path, err)
	}
	address, err := library.Lookup(symbol)
	if err != nil || address == 0 {
		t.Fatalf("Lookup(%s) = %#x, %v", symbol, address, err)
	}
	if _, err := library.Lookup("AuraGoOnnxrtMissingSymbol"); err == nil {
		t.Fatal("Lookup of a missing symbol succeeded")
	} else if !strings.Contains(err.Error(), "resolve native symbol AuraGoOnnxrtMissingSymbol") {
		t.Fatalf("missing symbol error = %v", err)
	}
	if err := library.Close(); err != nil {
		t.Fatalf("first Close = %v", err)
	}
	if err := library.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
	if _, err := library.Lookup(symbol); err == nil {
		t.Fatal("Lookup after Close succeeded")
	}
}
