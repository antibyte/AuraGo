package onnxrt

import (
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

// fakeRuntime is an open Runtime whose C function table is replaced by Go
// closures, so lifetime rules can be tested without ONNX Runtime. Functions
// left nil panic when called, which proves that a code path made no native
// call.
func fakeRuntime(released *[]string) *Runtime {
	r := &Runtime{env: 1, memoryInfo: 2}
	r.functions.releaseSession = func(uintptr) { *released = append(*released, "session") }
	r.functions.releaseMemoryInfo = func(uintptr) { *released = append(*released, "memory") }
	r.functions.releaseEnv = func(uintptr) { *released = append(*released, "env") }
	r.functions.createSessionOptions = func(out *uintptr) uintptr {
		*out = 10
		return 0
	}
	r.functions.setSessionGraphOptimizationLevel = func(uintptr, int32) uintptr { return 0 }
	r.functions.releaseSessionOptions = func(uintptr) {}
	r.functions.createSessionFromArray = func(_ uintptr, _ unsafe.Pointer, _ uintptr, _ uintptr, out *uintptr) uintptr {
		*out = 20
		return 0
	}
	return r
}

func TestRuntimeCloseRefusesOpenSessions(t *testing.T) {
	var released []string
	r := fakeRuntime(&released)
	session, err := r.NewSession([]byte{1}, SessionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err == nil || !strings.Contains(err.Error(), "open session") {
		t.Fatalf("Close with an open session = %v", err)
	}
	if len(released) != 0 || r.closed() {
		t.Fatalf("Close with an open session released %v (closed=%v)", released, r.closed())
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("second session Close = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(released, ","); got != "session,memory,env" {
		t.Fatalf("released %s", got)
	}
}

func TestFailedNewSessionDoesNotCountAsOpen(t *testing.T) {
	var released []string
	r := fakeRuntime(&released)
	message := []byte("invalid model\x00")
	r.functions.getErrorMessage = func(uintptr) *byte { return &message[0] }
	r.functions.releaseStatus = func(uintptr) {}
	r.functions.createSessionFromArray = func(_ uintptr, _ unsafe.Pointer, _ uintptr, _ uintptr, _ *uintptr) uintptr {
		return 99
	}
	if _, err := r.NewSession([]byte{1}, SessionOptions{}); err == nil || !strings.Contains(err.Error(), "invalid model") {
		t.Fatalf("NewSession = %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close after a failed NewSession = %v", err)
	}
}

func TestSessionCloseAfterRuntimeCloseFails(t *testing.T) {
	session := Session{runtime: &Runtime{}, handle: 5}
	if err := session.Close(); err == nil {
		t.Fatal("Close of a session whose runtime is closed succeeded")
	}
}

func TestPrepareSessionOptions(t *testing.T) {
	cpu, _, err := prepareSessionOptions(SessionOptions{Provider: "cpuExecutionProvider"})
	if err != nil || cpu.Provider != ProviderCPU {
		t.Fatalf("CPUExecutionProvider = %q, %v; want CPU", cpu.Provider, err)
	}
	webgpu, level, err := prepareSessionOptions(SessionOptions{
		Provider:        ProviderWebGPU,
		ProviderOptions: map[string]string{"deviceId": "0"},
		Optimization:    OptimizationBasic,
		Threads:         math.MaxInt32,
	})
	if err != nil || webgpu.Provider != ProviderWebGPU || level != 1 {
		t.Fatalf("WebGPU options = %+v, %d, %v", webgpu, level, err)
	}
	invalid := map[string]SessionOptions{
		"negative threads":       {Threads: -1},
		"unknown optimization":   {Optimization: 42},
		"CPU provider options":   {ProviderOptions: map[string]string{"a": "b"}},
		"named CPU options":      {Provider: "CPUExecutionProvider", ProviderOptions: map[string]string{"a": "b"}},
		"NUL optimized path":     {OptimizedModelPath: "model\x00.onnx"},
		"NUL profile prefix":     {ProfilePrefix: "profile\x00"},
		"NUL provider":           {Provider: "WebGpu\x00ExecutionProvider"},
		"NUL provider key":       {Provider: ProviderWebGPU, ProviderOptions: map[string]string{"a\x00": "b"}},
		"NUL provider value":     {Provider: ProviderWebGPU, ProviderOptions: map[string]string{"a": "b\x00"}},
		"threads above MaxInt32": {},
	}
	if strconv.IntSize == 64 {
		tooMany := int64(math.MaxInt32) + 1
		invalid["threads above MaxInt32"] = SessionOptions{Threads: int(tooMany)}
	} else {
		delete(invalid, "threads above MaxInt32")
	}
	for name, opts := range invalid {
		if _, _, err := prepareSessionOptions(opts); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestNewSessionValidatesOptionsBeforeNativeCalls(t *testing.T) {
	r := &Runtime{env: 1}
	if _, err := r.NewSession([]byte{1}, SessionOptions{Threads: -1}); err == nil {
		t.Fatal("negative threads accepted")
	}
	if _, err := r.NewSession([]byte{1}, SessionOptions{ProfilePrefix: "a\x00b"}); err == nil {
		t.Fatal("NUL profile prefix accepted")
	}
}

func TestCStringHelpersRejectNUL(t *testing.T) {
	value, err := cBytes("name")
	if err != nil || !slices.Equal(value, []byte("name\x00")) {
		t.Fatalf("cBytes = %q, %v", value, err)
	}
	if _, err := cBytes("na\x00me"); err == nil {
		t.Fatal("cBytes accepted a NUL byte")
	}
	if _, _, err := ortPath(filepath.Join("dir", "model.onnx")); err != nil {
		t.Fatalf("ortPath = %v", err)
	}
	if _, _, err := ortPath("dir\x00model.onnx"); err == nil {
		t.Fatal("ortPath accepted a NUL byte")
	}
	if _, err := Load(t.TempDir(), "log\x00id"); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("Load with a NUL log ID = %v", err)
	}
}

func TestRunRejectsInvalidInputsBeforeNativeCalls(t *testing.T) {
	session := &Session{runtime: &Runtime{env: 1}, handle: 1}
	valid := Tensor{Shape: []int64{1}, Float32: []float32{1}}
	cases := map[string]struct {
		inputs  map[string]Tensor
		outputs []string
	}{
		"NUL input name":  {map[string]Tensor{"a": valid, "x\x00": valid}, []string{"y"}},
		"NUL output name": {map[string]Tensor{"x": valid}, []string{"y\x00"}},
		"invalid tensor":  {map[string]Tensor{"a": valid, "x": {Shape: []int64{2}, Float32: []float32{1}}}, []string{"y"}},
	}
	for name, c := range cases {
		if _, err := session.Run(c.inputs, c.outputs); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
