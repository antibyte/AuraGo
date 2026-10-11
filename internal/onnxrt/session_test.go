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

// TestRunReleasesEveryValueExactlyOnce checks Run's cleanup on every exit
// path with a fake function table: each OrtValue it created is released
// exactly once, and the outputs go before the inputs.
func TestRunReleasesEveryValueExactlyOnce(t *testing.T) {
	const (
		inputA, inputB   = uintptr(101), uintptr(102)
		outputY, outputZ = uintptr(201), uintptr(202)
	)
	cases := []struct {
		name         string
		failCreateAt int // 1-based input whose tensor creation fails; 0 = none
		runFails     bool
		missing      uintptr // output value ORT leaves null
		failRead     uintptr // output whose shape query fails
		wantErr      string
		wantReleased []uintptr
	}{
		{name: "first input creation fails", failCreateAt: 1, wantErr: "create a tensor: create failed"},
		{name: "second input creation fails", failCreateAt: 2, wantErr: "create b tensor: create failed", wantReleased: []uintptr{inputA}},
		{name: "run fails", runFails: true, wantErr: "run failed", wantReleased: []uintptr{inputA, inputB}},
		{name: "output missing", missing: outputZ, wantErr: "ONNX output z is missing", wantReleased: []uintptr{outputY, inputA, inputB}},
		{name: "first output unreadable", failRead: outputY, wantErr: "read output y: get output shape: read failed", wantReleased: []uintptr{outputY, outputZ, inputA, inputB}},
		{name: "second output unreadable", failRead: outputZ, wantErr: "read output z: get output shape: read failed", wantReleased: []uintptr{outputY, outputZ, inputA, inputB}},
		{name: "success", wantReleased: []uintptr{outputY, outputZ, inputA, inputB}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var closed []string
			r := fakeRuntime(&closed)
			var created, released []uintptr
			createCalls := 0
			outputData := map[uintptr][]float32{outputY: {1, 2}, outputZ: {3, 4}}
			failure := []byte("create failed\x00")
			r.functions.getErrorMessage = func(status uintptr) *byte {
				switch status {
				case 91:
					return &failure[0]
				case 92:
					return &[]byte("run failed\x00")[0]
				default:
					return &[]byte("read failed\x00")[0]
				}
			}
			r.functions.releaseStatus = func(uintptr) {}
			r.functions.createTensorWithData = func(_ uintptr, _ unsafe.Pointer, _ uintptr, _ *int64, _ uintptr, _ int32, out *uintptr) uintptr {
				createCalls++
				if createCalls == c.failCreateAt {
					return 91
				}
				*out = 100 + uintptr(createCalls)
				created = append(created, *out)
				return 0
			}
			r.functions.run = func(_ uintptr, _ uintptr, _ **byte, inputValues *uintptr, inputCount uintptr, _ **byte, outputCount uintptr, outputValues *uintptr) uintptr {
				if got := unsafe.Slice(inputValues, inputCount); !slices.Equal(got, []uintptr{inputA, inputB}) {
					t.Errorf("run got inputs %v, want name-sorted %v", got, []uintptr{inputA, inputB})
				}
				if c.runFails {
					return 92
				}
				outputs := unsafe.Slice(outputValues, outputCount)
				for i, value := range []uintptr{outputY, outputZ} {
					if value != c.missing {
						outputs[i] = value
						created = append(created, value)
					}
				}
				return 0
			}
			r.functions.releaseValue = func(value uintptr) {
				if slices.Contains(released, value) {
					t.Errorf("value %d released twice", value)
				}
				released = append(released, value)
			}
			r.functions.getTensorTypeAndShape = func(value uintptr, out *uintptr) uintptr {
				if value == c.failRead {
					return 93
				}
				*out = value + 1000
				return 0
			}
			r.functions.getTensorElementType = func(_ uintptr, out *int32) uintptr {
				*out = tensorElementFloat
				return 0
			}
			r.functions.getDimensionsCount = func(_ uintptr, out *uintptr) uintptr {
				*out = 1
				return 0
			}
			r.functions.getDimensions = func(_ uintptr, out *int64, _ uintptr) uintptr {
				*out = 2
				return 0
			}
			r.functions.getTensorShapeElementCount = func(_ uintptr, out *uintptr) uintptr {
				*out = 2
				return 0
			}
			r.functions.getTensorMutableData = func(value uintptr, out *unsafe.Pointer) uintptr {
				*out = unsafe.Pointer(&outputData[value][0])
				return 0
			}
			r.functions.releaseTensorTypeAndShapeInfo = func(uintptr) {}

			session, err := r.NewSession([]byte{1}, SessionOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := session.Close(); err != nil {
					t.Error(err)
				}
			}()
			inputs := map[string]Tensor{
				"b": {Shape: []int64{1}, Float32: []float32{2}},
				"a": {Shape: []int64{1}, Float32: []float32{1}},
			}
			result, err := session.Run(inputs, []string{"y", "z"})
			if c.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(result["y"].Float32, []float32{1, 2}) || !slices.Equal(result["z"].Float32, []float32{3, 4}) {
					t.Fatalf("result = %+v", result)
				}
			} else if err == nil || err.Error() != c.wantErr || result != nil {
				t.Fatalf("Run = %v, %v; want error %q", result, err, c.wantErr)
			}
			if !slices.Equal(released, c.wantReleased) {
				t.Fatalf("released %v, want %v (outputs before inputs)", released, c.wantReleased)
			}
			if len(created) != len(released) {
				t.Fatalf("created %v but released %v", created, released)
			}
			for _, value := range created {
				if !slices.Contains(released, value) {
					t.Errorf("value %d was never released", value)
				}
			}
		})
	}
}
