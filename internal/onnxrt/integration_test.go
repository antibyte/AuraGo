package onnxrt

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("AURAGO_PERSONDETECT_INTEGRATION") != "1" {
		t.Skip("set AURAGO_PERSONDETECT_INTEGRATION=1 to download and test the pinned ONNX Runtime")
	}
}

// loadedIntegrationRuntime remembers which ORT library this test process has
// loaded. Two different ORT libraries must never share one process (their
// exported symbols would interpose), which is also why production loads ORT
// only in single-purpose worker processes.
var loadedIntegrationRuntime string

func ensureIntegrationRuntime(t *testing.T, spec AssetSpec, ok bool) string {
	t.Helper()
	if !ok {
		t.Skipf("no pinned runtime for %s/%s", goruntime.GOOS, goruntime.GOARCH)
	}
	if loadedIntegrationRuntime != "" && loadedIntegrationRuntime != spec.ID {
		t.Skipf("%s is already loaded in this process; run this test alone with -run", loadedIntegrationRuntime)
	}
	loadedIntegrationRuntime = spec.ID
	root := os.Getenv("AURAGO_PERSONDETECT_CACHE")
	if root == "" {
		root = filepath.Join("..", "..", "disposable", "person-detection", "cache")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	runtimeRoot, err := NewAssetCache(filepath.Join(absolute, "embeddings"), "AuraGo/onnxrt-test", nil).EnsureRuntime(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	return runtimeRoot
}

func appendField(b []byte, number protowire.Number, value []byte) []byte {
	b = protowire.AppendTag(b, number, protowire.BytesType)
	return protowire.AppendBytes(b, value)
}

func appendVarintField(b []byte, number protowire.Number, value uint64) []byte {
	b = protowire.AppendTag(b, number, protowire.VarintType)
	return protowire.AppendVarint(b, value)
}

func testIdentityNode(input, output string) []byte {
	var node []byte
	node = appendField(node, 1, []byte(input))
	node = appendField(node, 2, []byte(output))
	node = appendField(node, 3, []byte("identity_"+output))
	return appendField(node, 4, []byte("Identity"))
}

func testValueInfo(name string, elementType uint64, dims ...int64) []byte {
	var shape []byte
	for _, dim := range dims {
		shape = appendField(shape, 1, appendVarintField(nil, 1, uint64(dim)))
	}
	tensor := appendVarintField(nil, 1, elementType)
	tensor = appendField(tensor, 2, shape)
	info := appendField(nil, 1, []byte(name))
	return appendField(info, 2, appendField(nil, 1, tensor))
}

// identityModel is y = Identity(x) (float32 [2,3]) and m = Identity(n)
// (int64 [2]), IR version 8, opset 17.
func identityModel() []byte {
	var graph []byte
	graph = appendField(graph, 1, testIdentityNode("x", "y"))
	graph = appendField(graph, 1, testIdentityNode("n", "m"))
	graph = appendField(graph, 2, []byte("identity"))
	graph = appendField(graph, 11, testValueInfo("x", tensorElementFloat, 2, 3))
	graph = appendField(graph, 11, testValueInfo("n", tensorElementInt64, 2))
	graph = appendField(graph, 12, testValueInfo("y", tensorElementFloat, 2, 3))
	graph = appendField(graph, 12, testValueInfo("m", tensorElementInt64, 2))
	opset := appendField(nil, 1, []byte(""))
	opset = appendVarintField(opset, 2, 17)
	var model []byte
	model = appendVarintField(model, 1, 8)
	model = appendField(model, 2, []byte("aurago-onnxrt-test"))
	model = appendField(model, 7, graph)
	return appendField(model, 8, opset)
}

const testTensorElementDouble = 11

func testNode(op string, inputs, outputs []string, attributes ...[]byte) []byte {
	var node []byte
	for _, input := range inputs {
		node = appendField(node, 1, []byte(input))
	}
	for _, output := range outputs {
		node = appendField(node, 2, []byte(output))
	}
	node = appendField(node, 3, []byte("node_"+outputs[0]))
	node = appendField(node, 4, []byte(op))
	for _, attribute := range attributes {
		node = appendField(node, 5, attribute)
	}
	return node
}

// testIntAttribute is an AttributeProto with name=1, i=3 and type=20 (INT).
func testIntAttribute(name string, value int64) []byte {
	attribute := appendField(nil, 1, []byte(name))
	attribute = appendVarintField(attribute, 3, uint64(value))
	return appendVarintField(attribute, 20, 2)
}

func testValueInfoNoShape(name string, elementType uint64) []byte {
	tensor := appendVarintField(nil, 1, elementType)
	info := appendField(nil, 1, []byte(name))
	return appendField(info, 2, appendField(nil, 1, tensor))
}

func testModel(name string, graph []byte) []byte {
	opset := appendField(nil, 1, []byte(""))
	opset = appendVarintField(opset, 2, 17)
	var model []byte
	model = appendVarintField(model, 1, 8)
	model = appendField(model, 2, []byte(name))
	model = appendField(model, 7, graph)
	return appendField(model, 8, opset)
}

// edgeCaseModel takes x float32 [2,3] and n int64 [1] and returns
// d = Cast(x, to=double), z = ConstantOfShape(n) (float32, zero-size for
// n = [0]) and s = Size(x) (rank-0 int64).
func edgeCaseModel() []byte {
	var graph []byte
	graph = appendField(graph, 1, testNode("Cast", []string{"x"}, []string{"d"}, testIntAttribute("to", testTensorElementDouble)))
	graph = appendField(graph, 1, testNode("ConstantOfShape", []string{"n"}, []string{"z"}))
	graph = appendField(graph, 1, testNode("Size", []string{"x"}, []string{"s"}))
	graph = appendField(graph, 2, []byte("edge_cases"))
	graph = appendField(graph, 11, testValueInfo("x", tensorElementFloat, 2, 3))
	graph = appendField(graph, 11, testValueInfo("n", tensorElementInt64, 1))
	graph = appendField(graph, 12, testValueInfoNoShape("d", testTensorElementDouble))
	graph = appendField(graph, 12, testValueInfoNoShape("z", tensorElementFloat))
	graph = appendField(graph, 12, testValueInfoNoShape("s", tensorElementInt64))
	return testModel("aurago-onnxrt-edge-cases", graph)
}

// passThroughModel has graph input x as a graph output too, plus
// y = Identity(x). ORT 1.26 returns the output x as an OrtValue that points
// at the caller's input buffer.
func passThroughModel() []byte {
	var graph []byte
	graph = appendField(graph, 1, testNode("Identity", []string{"x"}, []string{"y"}))
	graph = appendField(graph, 2, []byte("pass_through"))
	graph = appendField(graph, 11, testValueInfo("x", tensorElementFloat, 2, 3))
	graph = appendField(graph, 12, testValueInfo("x", tensorElementFloat, 2, 3))
	graph = appendField(graph, 12, testValueInfo("y", tensorElementFloat, 2, 3))
	return testModel("aurago-onnxrt-pass-through", graph)
}

func runIdentity(t *testing.T, session *Session) {
	t.Helper()
	x := Tensor{Shape: []int64{2, 3}, Float32: []float32{1, 2, 3, 4, 5, 6}}
	n := Tensor{Shape: []int64{2}, Int64: []int64{7, -8}}
	outputs, err := session.Run(map[string]Tensor{"x": x, "n": n}, []string{"y", "m"})
	if err != nil {
		t.Fatal(err)
	}
	y, m := outputs["y"], outputs["m"]
	if !slices.Equal(y.Shape, x.Shape) || !slices.Equal(y.Float32, x.Float32) || y.Int64 != nil {
		t.Fatalf("y = %+v", y)
	}
	if !slices.Equal(m.Shape, n.Shape) || !slices.Equal(m.Int64, n.Int64) || m.Float32 != nil {
		t.Fatalf("m = %+v", m)
	}
}

func TestCPURuntimeIntegration(t *testing.T) {
	requireIntegration(t)
	spec, ok := CPURuntimeAsset(goruntime.GOOS, goruntime.GOARCH)
	root := ensureIntegrationRuntime(t, spec, ok)
	rt, err := Load(root, "aurago-onnxrt-test")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.Version() != CPURuntimeVersion {
		t.Fatalf("version = %q", rt.Version())
	}
	providers, err := rt.Providers()
	if err != nil || !containsFold(providers, "CPUExecutionProvider") {
		t.Fatalf("providers = %v, %v", providers, err)
	}
	temp := t.TempDir()
	optimized := filepath.Join(temp, "identity.optimized.onnx")
	session, err := rt.NewSession(identityModel(), SessionOptions{
		Threads:            1,
		DisableSpinning:    true,
		Optimization:       OptimizationBasic,
		OptimizedModelPath: optimized,
		ProfilePrefix:      filepath.Join(temp, "profile"),
	})
	if err != nil {
		t.Fatal(err)
	}
	runIdentity(t, session)
	if err := rt.Close(); err == nil || !strings.Contains(err.Error(), "open session") {
		t.Fatalf("runtime Close with an open session = %v", err)
	}
	runIdentity(t, session) // the refused Close left the library loaded
	profile, err := session.EndProfiling()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(profile)
	var events []map[string]any
	if err != nil || json.Unmarshal(raw, &events) != nil || len(events) == 0 {
		t.Fatalf("profile %s: %v (%d events)", profile, err, len(events))
	}
	_ = os.Remove(profile)
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(optimized); err != nil || info.Size() == 0 {
		t.Fatalf("optimized model not written: %v", err)
	}
	_, err = rt.NewSession(identityModel(), SessionOptions{Provider: ProviderWebGPU})
	if err == nil || !strings.Contains(err.Error(), "is not available in this ONNX Runtime bundle") {
		t.Fatalf("WebGPU on the CPU bundle: %v", err)
	}
	if err := rt.Close(); err != nil {
		t.Fatalf("runtime Close after all sessions closed = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("second session Close = %v", err)
	}
}

// TestCPURuntimeIntegrationOutputs covers output shapes and types that the
// detector and embeddings models never produce but the binding must handle.
func TestCPURuntimeIntegrationOutputs(t *testing.T) {
	requireIntegration(t)
	spec, ok := CPURuntimeAsset(goruntime.GOOS, goruntime.GOARCH)
	root := ensureIntegrationRuntime(t, spec, ok)
	rt, err := Load(root, "aurago-onnxrt-test")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	open := func(t *testing.T, model []byte, opts SessionOptions) *Session {
		t.Helper()
		session, err := rt.NewSession(model, opts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := session.Close(); err != nil {
				t.Errorf("session Close = %v", err)
			}
		})
		return session
	}

	t.Run("output aliasing an input is copied", func(t *testing.T) {
		session := open(t, passThroughModel(), SessionOptions{Threads: 1})
		x := Tensor{Shape: []int64{2, 3}, Float32: []float32{1, 2, 3, 4, 5, 6}}
		outputs, err := session.Run(map[string]Tensor{"x": x}, []string{"x", "y"})
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"x", "y"} {
			got := outputs[name]
			if !slices.Equal(got.Shape, x.Shape) || !slices.Equal(got.Float32, x.Float32) || got.Int64 != nil {
				t.Fatalf("%s = %+v", name, got)
			}
			if &got.Float32[0] == &x.Float32[0] {
				t.Fatalf("output %s shares the input buffer", name)
			}
		}
	})

	t.Run("zero-size and rank-0 outputs", func(t *testing.T) {
		session := open(t, edgeCaseModel(), SessionOptions{Threads: 1})
		outputs, err := session.Run(map[string]Tensor{
			"x": {Shape: []int64{2, 3}, Float32: []float32{1, 2, 3, 4, 5, 6}},
			"n": {Shape: []int64{1}, Int64: []int64{0}},
		}, []string{"z", "s"})
		if err != nil {
			t.Fatal(err)
		}
		z, s := outputs["z"], outputs["s"]
		if !slices.Equal(z.Shape, []int64{0}) || z.Float32 == nil || len(z.Float32) != 0 || z.Int64 != nil {
			t.Fatalf("zero-size z = %#v", z)
		}
		if len(s.Shape) != 0 || !slices.Equal(s.Int64, []int64{6}) || s.Float32 != nil {
			t.Fatalf("rank-0 s = %#v", s)
		}
	})

	t.Run("unsupported output element type is rejected", func(t *testing.T) {
		session := open(t, edgeCaseModel(), SessionOptions{Threads: 1})
		_, err := session.Run(map[string]Tensor{
			"x": {Shape: []int64{2, 3}, Float32: []float32{1, 2, 3, 4, 5, 6}},
			"n": {Shape: []int64{1}, Int64: []int64{1}},
		}, []string{"z", "d"})
		if err == nil || !strings.Contains(err.Error(), "unsupported output element type 11") {
			t.Fatalf("double output: %v", err)
		}
	})

	t.Run("explicit CPU provider name", func(t *testing.T) {
		session := open(t, identityModel(), SessionOptions{Threads: 1, Provider: "CPUExecutionProvider"})
		runIdentity(t, session)
	})

	t.Run("Close removes a profile that was never ended", func(t *testing.T) {
		dir := t.TempDir()
		session, err := rt.NewSession(identityModel(), SessionOptions{Threads: 1, ProfilePrefix: filepath.Join(dir, "profile")})
		if err != nil {
			t.Fatal(err)
		}
		runIdentity(t, session)
		if err := session.Close(); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("profile directory after Close: %v, %v", entries, err)
		}
		if _, err := session.EndProfiling(); err == nil {
			t.Fatal("EndProfiling after Close succeeded")
		}
	})
}

func TestWebGPURuntimeIntegration(t *testing.T) {
	requireIntegration(t)
	if os.Getenv("AURAGO_PERSONDETECT_GPU") != "1" {
		t.Skip("set AURAGO_PERSONDETECT_GPU=1 to test the WebGPU wheel on a GPU")
	}
	spec, ok := WebGPURuntimeAsset(goruntime.GOOS, goruntime.GOARCH)
	root := ensureIntegrationRuntime(t, spec, ok)
	if goruntime.GOOS == "windows" {
		// Dawn loads dxcompiler.dll/dxil.dll by name; workers get the runtime
		// directory on PATH through ConfigureWorkerCommand.
		t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	rt, err := Load(root, "aurago-onnxrt-test")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.Version() != WebGPURuntimeVersion {
		t.Fatalf("version = %q", rt.Version())
	}
	providers, err := rt.Providers()
	if err != nil || !containsFold(providers, ProviderWebGPU) {
		t.Fatalf("providers = %v, %v", providers, err)
	}
	session, err := rt.NewSession(identityModel(), SessionOptions{Threads: 1, Provider: ProviderWebGPU})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	runIdentity(t, session)
}
