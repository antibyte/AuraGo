package onnxrt

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

func versionedLibraryName() string {
	switch goruntime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "libonnxruntime.1.27.0.dylib"
	default:
		return "libonnxruntime.so.1.27.0"
	}
}

func TestLibraryNameMatchesPlatform(t *testing.T) {
	want := "libonnxruntime.so"
	switch goruntime.GOOS {
	case "windows":
		want = "onnxruntime.dll"
	case "darwin":
		want = "libonnxruntime.dylib"
	}
	if got := LibraryName(); got != want {
		t.Fatalf("LibraryName() = %q, want %q", got, want)
	}
}

func TestFindLibraryAcceptsVersionedNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"onnxruntime_providers_shared.dll", "libonnxruntime_providers_shared.so", versionedLibraryName()} {
		path := filepath.Join(root, "lib", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("library"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	found, err := FindLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(found) != versionedLibraryName() {
		t.Fatalf("FindLibrary = %q", found)
	}
}

func TestFindLibraryReturnsAbsolutePathForRelativeRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "runtime", versionedLibraryName())
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("library"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	found, err := FindLibrary("runtime")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(found) || filepath.Base(found) != versionedLibraryName() {
		t.Fatalf("FindLibrary(relative) = %q, want an absolute path to %s", found, versionedLibraryName())
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o750); err != nil {
		t.Fatal(err)
	}
	_, err = FindLibrary("empty")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(filepath.Join(root, "empty"))) {
		t.Fatalf("FindLibrary(relative empty dir) error = %v, want the absolute directory", err)
	}
}

func TestFindLibraryReportsMissingLibrary(t *testing.T) {
	if _, err := FindLibrary(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not found below") {
		t.Fatalf("FindLibrary error = %v", err)
	}
}

func TestLoadFailsWithoutLibrary(t *testing.T) {
	if _, err := Load(t.TempDir(), "test"); err == nil || !strings.Contains(err.Error(), "not found below") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestLoadRejectsNonLibraryFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, versionedLibraryName()), []byte("not a shared library"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, "test"); err == nil || !strings.Contains(err.Error(), "load native library") {
		t.Fatalf("Load error = %v", err)
	}
}

func TestGraphOptimizationLevels(t *testing.T) {
	cases := map[GraphOptimization]int32{OptimizationAll: 99, OptimizationBasic: 1, OptimizationExtended: 2, OptimizationDisabled: 0}
	for level, want := range cases {
		got, err := level.ortLevel()
		if err != nil || got != want {
			t.Fatalf("ortLevel(%d) = %d, %v; want %d", level, got, err, want)
		}
	}
	if _, err := GraphOptimization(42).ortLevel(); err == nil {
		t.Fatal("unknown optimization level accepted")
	}
	var zero SessionOptions
	if zero.Optimization != OptimizationAll {
		t.Fatal("zero SessionOptions must request ORT level 99")
	}
}

func TestAppendProviderName(t *testing.T) {
	cases := map[string]string{
		ProviderWebGPU: "WebGPU",
		ProviderCUDA:   "CUDAExecutionProvider",
		ProviderCoreML: "CoreMLExecutionProvider",
	}
	for provider, want := range cases {
		if got := appendProviderName(provider); got != want {
			t.Fatalf("appendProviderName(%q) = %q, want %q", provider, got, want)
		}
	}
}

func TestContainsFoldIgnoresCase(t *testing.T) {
	if !containsFold([]string{"CPUExecutionProvider", "CUDAExecutionProvider"}, "cudaexecutionprovider") {
		t.Fatal("case-insensitive provider lookup failed")
	}
	if containsFold([]string{"CPUExecutionProvider"}, ProviderWebGPU) {
		t.Fatal("missing provider reported as present")
	}
}

func TestValidateTensor(t *testing.T) {
	for _, tensor := range []Tensor{
		{Shape: []int64{2, 3}, Float32: make([]float32, 6)},
		{Shape: []int64{4}, Int64: []int64{1, 2, 3, 4}},
	} {
		if err := validateTensor("x", tensor); err != nil {
			t.Fatalf("valid tensor rejected: %v", err)
		}
	}
	invalid := map[string]Tensor{
		"none":         {Shape: []int64{1}},
		"both":         {Shape: []int64{1}, Float32: []float32{1}, Int64: []int64{1}},
		"no shape":     {Float32: []float32{1}},
		"zero dim":     {Shape: []int64{0}, Float32: []float32{}},
		"negative dim": {Shape: []int64{-1}, Float32: []float32{1}},
		"count":        {Shape: []int64{2, 2}, Float32: make([]float32, 3)},
	}
	for name, tensor := range invalid {
		if err := validateTensor("x", tensor); err == nil {
			t.Errorf("%s tensor accepted", name)
		}
	}
}

func TestSortedKeys(t *testing.T) {
	if got := strings.Join(sortedKeys(map[string]string{"b": "2", "a": "1"}), ","); got != "a,b" {
		t.Fatalf("sortedKeys = %s", got)
	}
	if len(sortedKeys(nil)) != 0 {
		t.Fatal("sortedKeys(nil) not empty")
	}
}

func TestClosedRuntimeAndSessionFailSafely(t *testing.T) {
	var closed Runtime
	if _, err := closed.NewSession([]byte{1}, SessionOptions{}); err == nil {
		t.Fatal("closed runtime created a session")
	}
	if _, err := closed.Providers(); err == nil {
		t.Fatal("closed runtime listed providers")
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("closed runtime Close = %v", err)
	}
	var session Session
	if _, err := session.Run(map[string]Tensor{"x": {Shape: []int64{1}, Float32: []float32{1}}}, []string{"y"}); err == nil {
		t.Fatal("closed session ran")
	}
	if _, err := session.EndProfiling(); err == nil {
		t.Fatal("closed session ended profiling")
	}
	if err := session.Close(); err != nil {
		t.Fatalf("closed session Close = %v", err)
	}
}
