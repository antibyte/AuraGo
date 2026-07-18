package embeddings

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestCandidateMatrixAlwaysIncludesCPUFallbacks(t *testing.T) {
	hardware := hardwareInfo{NVIDIA: true, Vulkan: true, DirectML: true}
	candidates := candidateMatrix("windows", "amd64", "cuda", hardware)
	assertCandidate(t, candidates, "onnx-cuda")
	assertCandidate(t, candidates, "llama-cuda")
	assertCandidate(t, candidates, "onnx-cpu")
	assertCandidate(t, candidates, "llama-cpu")
	if _, ok := findCandidate(candidates, "llama-vulkan"); ok {
		t.Fatal("explicit CUDA selection should not include Vulkan")
	}
}

func TestCandidateMatrixSkipsUnsupportedWindowsARM64GPU(t *testing.T) {
	hardware := hardwareInfo{NVIDIA: true, Vulkan: true, DirectML: true}
	candidates := candidateMatrix("windows", "arm64", "auto", hardware)
	for _, current := range candidates {
		if current.GPU {
			t.Fatalf("unexpected Windows ARM64 GPU candidate: %s", current.ID)
		}
	}
	assertCandidate(t, candidates, "onnx-cpu")
	assertCandidate(t, candidates, "llama-cpu")
}

func TestBenchmarkSelectionPrefersVerifiedGPUThenFastest(t *testing.T) {
	values := []struct {
		candidate candidate
		result    BenchmarkResult
	}{
		{candidate: candidate{ID: "onnx-cpu"}, result: BenchmarkResult{LatencyMS: 1}},
		{candidate: candidate{ID: "onnx-cuda", GPU: true}, result: BenchmarkResult{LatencyMS: 8}},
		{candidate: candidate{ID: "llama-vulkan", GPU: true}, result: BenchmarkResult{LatencyMS: 5}},
	}
	selected, ok := selectBenchmarkWinner(values)
	if !ok || selected.ID != "llama-vulkan" {
		t.Fatalf("selected = %#v, %v; want llama-vulkan", selected, ok)
	}
}

func TestManagerFallsBackLiveWithinONNXFingerprint(t *testing.T) {
	ready := make(chan struct{})
	close(ready)
	fallbackVector := make([]float32, GraniteDimensions)
	fallbackVector[0] = 1
	manager := &Manager{
		active: &testEmbedder{
			status: Status{Runtime: "onnxruntime", Backend: "cuda", GPU: true, Fingerprint: onnxFingerprint()},
			err:    errors.New("sidecar crashed"),
		},
		fallback: &testEmbedder{
			status:  Status{Runtime: "onnxruntime", Backend: "cpu", Fingerprint: onnxFingerprint()},
			vectors: [][]float32{fallbackVector},
		},
		status: Status{State: "ready"},
		ready:  ready,
		logger: discardEmbeddingLogger(),
	}
	vectors, err := manager.Embed(context.Background(), []string{"fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || len(vectors[0]) != GraniteDimensions {
		t.Fatalf("unexpected fallback vectors: %#v", vectors)
	}
	status := manager.Status()
	if status.State != "degraded" || !strings.Contains(status.FallbackReason, "sidecar crashed") {
		t.Fatalf("fallback status was not recorded: %#v", status)
	}
}

func TestManagerSchedulesReindexBeforeCrossFormatFallback(t *testing.T) {
	cacheDir := t.TempDir()
	ready := make(chan struct{})
	close(ready)
	fallbackVector := make([]float32, GraniteDimensions)
	fallbackVector[0] = 1
	manager := &Manager{
		options: LocalOptions{
			CacheDir:        cacheDir,
			ResetMarkerPath: filepath.Join(cacheDir, "embeddings_reset_pending.json"),
			Backend:         "auto",
			ContextSize:     2048,
			BatchSize:       2048,
		},
		active: &testEmbedder{
			status: Status{Runtime: "llama.cpp", Backend: "vulkan", GPU: true, Fingerprint: ggufFingerprint()},
			err:    errors.New("sidecar crashed"),
		},
		fallback: &testEmbedder{
			status:  Status{Runtime: "onnxruntime", Backend: "cpu", Fingerprint: onnxFingerprint()},
			vectors: [][]float32{fallbackVector},
		},
		status: Status{State: "ready"},
		ready:  ready,
		logger: discardEmbeddingLogger(),
	}
	_, err := manager.Embed(context.Background(), []string{"safe fallback"})
	if err == nil || !strings.Contains(err.Error(), "controlled reindex") {
		t.Fatalf("cross-format fallback error = %v", err)
	}
	if _, err := os.Stat(manager.options.ResetMarkerPath); err != nil {
		t.Fatalf("controlled reset marker missing: %v", err)
	}
	state, err := manager.loadSelection()
	if err != nil {
		t.Fatal(err)
	}
	if state.CandidateID != "onnx-cpu" || state.EmbeddingFingerprint != onnxFingerprint() {
		t.Fatalf("unsafe fallback selection persisted: %#v", state)
	}
	if !manager.Status().RestartRequired {
		t.Fatal("cross-format fallback did not require restart")
	}
}

func TestGraniteFingerprintsCaptureFormatAndRuntime(t *testing.T) {
	onnx := onnxFingerprint()
	gguf := ggufFingerprint()
	if onnx == gguf {
		t.Fatal("ONNX and GGUF fingerprints must differ")
	}
	for _, expected := range []string{"format=onnx", "quantization=int8-dynamic", "pooling=cls", "normalization=l2", "dimensions=384", "runtime=onnxruntime-1.26.0"} {
		if !bytes.Contains([]byte(onnx), []byte(expected)) {
			t.Fatalf("ONNX fingerprint %q does not contain %q", onnx, expected)
		}
	}
	for _, expected := range []string{"format=gguf", "quantization=q8_0", "runtime=llama.cpp-b9994"} {
		if !bytes.Contains([]byte(gguf), []byte(expected)) {
			t.Fatalf("GGUF fingerprint %q does not contain %q", gguf, expected)
		}
	}
}

func TestSecureArchiveDestinationRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../escape", "../../escape", `C:\escape`, "/absolute"} {
		if _, err := secureArchiveDestination(root, name); err == nil {
			t.Fatalf("secureArchiveDestination(%q) unexpectedly succeeded", name)
		}
	}
	if destination, err := secureArchiveDestination(root, "runtime/bin/llama-server"); err != nil || !pathWithinRoot(destination, root) {
		t.Fatalf("safe destination = %q, %v", destination, err)
	}
}

func TestExtractZipSecureRejectsSymlink(t *testing.T) {
	temp := t.TempDir()
	archivePath := filepath.Join(temp, "runtime.zip")
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	header := &zip.FileHeader{Name: "runtime/link"}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("../outside")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractZipSecure(archivePath, filepath.Join(temp, "out")); err == nil {
		t.Fatal("expected symlink archive to be rejected")
	}
}

func TestFileMatchesRejectsManipulatedCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asset.bin")
	content := []byte("verified asset")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	if !fileMatches(path, int64(len(content)), hex.EncodeToString(hash[:])) {
		t.Fatal("expected valid cache file")
	}
	if err := os.WriteFile(path, []byte("tampered asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fileMatches(path, int64(len("tampered asset")), hex.EncodeToString(hash[:])) {
		t.Fatal("manipulated cache file unexpectedly validated")
	}
}

func TestAssetDownloadCleansInterruptedPartAndReusesOfflineCache(t *testing.T) {
	content := []byte("checksum-verified embedding asset")
	hash := sha256.Sum256(content)
	interrupt := true
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		if interrupt {
			_, _ = w.Write(content[:len(content)/2])
			return
		}
		_, _ = w.Write(content)
	}))
	cache := newAssetCache(t.TempDir(), nil)
	cache.client = server.Client()
	spec := assetSpec{
		ID:     "test-asset",
		URL:    server.URL + "/asset",
		Size:   int64(len(content)),
		SHA256: hex.EncodeToString(hash[:]),
		Kind:   assetFile,
	}
	destination := filepath.Join(cache.root, "models", "asset.bin")
	if err := cache.ensureDirectAsset(context.Background(), spec, destination); err == nil {
		t.Fatal("interrupted download unexpectedly succeeded")
	}
	if _, err := os.Stat(destination + ".part"); !os.IsNotExist(err) {
		t.Fatalf("partial download was not cleaned up: %v", err)
	}
	interrupt = false
	if err := cache.ensureDirectAsset(context.Background(), spec, destination); err != nil {
		t.Fatal(err)
	}
	server.Close()
	if err := cache.ensureDirectAsset(context.Background(), spec, destination); err != nil {
		t.Fatalf("verified offline cache was not reused: %v", err)
	}
}

func TestAssetDownloadRejectsMatchingSizeWithWrongHash(t *testing.T) {
	content := []byte("expected")
	tampered := []byte("tampered")
	if len(content) != len(tampered) {
		t.Fatal("test data must have matching lengths")
	}
	hash := sha256.Sum256(content)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tampered)
	}))
	defer server.Close()
	cache := newAssetCache(t.TempDir(), nil)
	cache.client = server.Client()
	spec := assetSpec{
		ID:     "tampered",
		URL:    server.URL,
		Size:   int64(len(content)),
		SHA256: hex.EncodeToString(hash[:]),
		Kind:   assetFile,
	}
	destination := filepath.Join(cache.root, "asset.bin")
	if err := cache.ensureDirectAsset(context.Background(), spec, destination); err == nil {
		t.Fatal("tampered download unexpectedly succeeded")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("tampered destination exists: %v", err)
	}
}

func TestGraniteTokenizerGoldenIDs(t *testing.T) {
	path := os.Getenv("AURAGO_GRANITE_TOKENIZER")
	if path == "" {
		t.Skip("set AURAGO_GRANITE_TOKENIZER to the pinned tokenizer.json for the reference golden test")
	}
	tokenizer, err := loadGraniteTokenizer(path)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string][]int{
		"Hallo Welt!":                   {179934, 44972, 22547, 0, 179938},
		"AuraGo runs local embeddings.": {179934, 137336, 11763, 13469, 2641, 158816, 13, 179938},
		"ローカル埋め込み":                      {179934, 12994, 63663, 6936, 6148, 193, 17329, 122854, 179938},
		"func main returns code":        {179934, 5558, 2701, 7254, 3429, 179938},
	}
	for text, expected := range tests {
		actual, err := tokenizer.encode(text, 2048)
		if err != nil {
			t.Fatalf("encode %q: %v", text, err)
		}
		if !equalInts(actual, expected) {
			t.Errorf("encode %q = %v, want %v", text, actual, expected)
		}
	}
}

func TestRuntimeEnvironmentPrependsPrivateDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "lib"), 0o750); err != nil {
		t.Fatal(err)
	}
	environment := runtimeEnvironment(root)
	pathValue := environmentValue(environment, "PATH")
	if !bytes.Contains([]byte(pathValue), []byte(root)) {
		t.Fatalf("PATH %q does not contain runtime root", pathValue)
	}
}

func TestProcessLogsStayBounded(t *testing.T) {
	buffer := newLimitedBuffer(16)
	_, _ = buffer.Write([]byte("0123456789abcdefghijklmnopqrstuvwxyz"))
	if got := buffer.String(); got != "klmnopqrstuvwxyz" {
		t.Fatalf("bounded log = %q", got)
	}
}

func TestManagerRejectsUnsupportedBackend(t *testing.T) {
	_, err := NewLocalGranite(LocalOptions{CacheDir: t.TempDir(), Backend: "quantum"})
	if err == nil {
		t.Fatal("expected unsupported backend to be rejected")
	}
}

func TestManagerSchedulesControlledResetForRuntimeFormatChange(t *testing.T) {
	dataDir := t.TempDir()
	markerPath := filepath.Join(dataDir, "embeddings_reset_pending.json")
	manager := &Manager{options: LocalOptions{ResetMarkerPath: markerPath}}
	if err := manager.scheduleReset("local_granite_runtime_change:onnx-cpu_to_llama-vulkan"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	var marker resetMarker
	if err := json.Unmarshal(raw, &marker); err != nil {
		t.Fatal(err)
	}
	if marker.Reason != "local_granite_runtime_change:onnx-cpu_to_llama-vulkan" || marker.RequestedAt.IsZero() {
		t.Fatalf("unexpected reset marker: %#v", marker)
	}
}

func TestSelectionStateRestoresLegacyRuntimeFingerprint(t *testing.T) {
	cacheDir := t.TempDir()
	manager := &Manager{options: LocalOptions{CacheDir: cacheDir}}
	raw := []byte(`{"version":1,"selection_fingerprint":"old","hardware_fingerprint":"hw","candidate_id":"llama-vulkan"}`)
	if err := os.WriteFile(filepath.Join(cacheDir, "selection.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := manager.loadSelection()
	if err != nil {
		t.Fatal(err)
	}
	if state.EmbeddingFingerprint != ggufFingerprint() {
		t.Fatalf("restored fingerprint = %q, want GGUF fingerprint", state.EmbeddingFingerprint)
	}
}

func TestValidateGraniteVector(t *testing.T) {
	vector := make([]float32, GraniteDimensions)
	vector[0] = 1
	if err := validateGraniteVector(vector); err != nil {
		t.Fatalf("unit vector rejected: %v", err)
	}
	vector[0] = 2
	if err := validateGraniteVector(vector); err == nil {
		t.Fatal("non-normalized vector unexpectedly accepted")
	}
}

func TestDetectHardwareProducesStableFingerprint(t *testing.T) {
	first := detectHardware(context.Background())
	second := detectHardware(context.Background())
	if first.Fingerprint == "" || first.Fingerprint != second.Fingerprint {
		t.Fatalf("hardware fingerprints differ: %q != %q", first.Fingerprint, second.Fingerprint)
	}
}

func TestLlamaGPUVerificationRequiresBackendAndOffloadEvidence(t *testing.T) {
	if !llamaGPUVerified("vulkan", "ggml_vulkan: using device GPU0; offloaded 13 layers to GPU") {
		t.Fatal("expected Vulkan offload evidence to validate")
	}
	if llamaGPUVerified("cuda", "CUDA backend available but no model offload occurred") {
		t.Fatal("backend name without offload evidence must not validate")
	}
}

func TestDockerLlamaContainerIsPrivatePinnedAndHardened(t *testing.T) {
	payload, err := dockerLlamaContainerPayload(
		llamaDockerCUDAImage,
		"/app/data/embeddings/models/granite.gguf",
		"ephemeral-key",
		"cuda",
		2048,
		2048,
		"aurago-container-id",
		"aurago_default",
	)
	if err != nil {
		t.Fatal(err)
	}
	if payload["Image"] != llamaDockerCUDAImage || !bytes.Contains([]byte(llamaDockerCUDAImage), []byte("@sha256:")) {
		t.Fatalf("sidecar image is not digest-pinned: %#v", payload["Image"])
	}
	hostConfig, ok := payload["HostConfig"].(map[string]any)
	if !ok {
		t.Fatalf("HostConfig has unexpected type: %T", payload["HostConfig"])
	}
	if hostConfig["ReadonlyRootfs"] != true {
		t.Fatal("sidecar root filesystem must be read-only")
	}
	if _, published := hostConfig["PortBindings"]; published {
		t.Fatal("sidecar must not publish a host port")
	}
	deviceRequests, ok := hostConfig["DeviceRequests"].([]map[string]any)
	if !ok || len(deviceRequests) != 1 || deviceRequests[0]["Driver"] != "nvidia" {
		t.Fatalf("CUDA device request missing: %#v", hostConfig["DeviceRequests"])
	}
	command, ok := payload["Cmd"].([]string)
	if !ok {
		t.Fatalf("Cmd has unexpected type: %T", payload["Cmd"])
	}
	joined := strings.Join(command, " ")
	for _, expected := range []string{"--embedding", "--pooling cls", "--alias " + GraniteModelID, "--batch-size 2048", "--ubatch-size 2048", "--n-gpu-layers 999"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("sidecar command %q does not contain %q", joined, expected)
		}
	}
	if strings.Contains(joined, "--model-alias") {
		t.Fatalf("sidecar command %q uses an argument unsupported by llama.cpp b9994", joined)
	}
}

func TestRuntimeManifestMatchesCurrentPlatformWhenSupported(t *testing.T) {
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		return
	}
	if asset, ok := onnxRuntimeAsset(runtime.GOOS, runtime.GOARCH, "cpu"); !ok || asset.SHA256 == "" || asset.Size <= 0 {
		t.Fatalf("missing ONNX CPU runtime manifest for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func assertCandidate(t *testing.T, candidates []candidate, id string) {
	t.Helper()
	if _, ok := findCandidate(candidates, id); !ok {
		t.Fatalf("candidate %s missing from %#v", id, candidates)
	}
}

func equalInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

type testEmbedder struct {
	status  Status
	vectors [][]float32
	err     error
}

func (embedder *testEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return embedder.vectors, embedder.err
}

func (embedder *testEmbedder) Dimensions() int {
	return GraniteDimensions
}

func (embedder *testEmbedder) ModelID() string {
	return GraniteModelID
}

func (embedder *testEmbedder) Fingerprint() string {
	return embedder.status.Fingerprint
}

func (embedder *testEmbedder) Status() Status {
	return embedder.status
}

func (embedder *testEmbedder) Close() error {
	return nil
}
