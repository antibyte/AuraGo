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
