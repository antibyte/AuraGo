package embeddings

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCandidateMatrixAlwaysIncludesCPUFallbacks(t *testing.T) {
	hardware := hardwareInfo{NVIDIA: true, Vulkan: true}
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
	hardware := hardwareInfo{NVIDIA: true, Vulkan: true}
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

func TestExtractTarGzipSecureExtractsFilesWithoutMaterializingLinks(t *testing.T) {
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "runtime/bin/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	}); err != nil {
		t.Fatal(err)
	}
	content := []byte("runtime")
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "runtime/bin/libonnxruntime.so.1",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.WriteHeader(&tar.Header{
		Name:     "runtime/bin/libonnxruntime.so",
		Typeflag: tar.TypeSymlink,
		Linkname: "libonnxruntime.so.1",
		Mode:     0o777,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "runtime.tar.gz")
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := extractTarGzipSecure(archivePath, target); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(target, "runtime", "bin", "libonnxruntime.so.1"))
	if err != nil || string(raw) != "runtime" {
		t.Fatalf("extracted runtime = %q, %v", raw, err)
	}
	if _, err := os.Lstat(filepath.Join(target, "runtime", "bin", "libonnxruntime.so")); !os.IsNotExist(err) {
		t.Fatalf("archive symlink was materialized: %v", err)
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
	for key, value := range map[string]string{
		"AURAGO_MASTER_KEY":   "master-secret",
		"OPENAI_API_KEY":      "api-secret",
		"CUSTOM_AUTH_TOKEN":   "token-secret",
		"MY_DB_PASSWORD":      "password-secret",
		"PROJECT_CREDENTIALS": "credentials-secret",
	} {
		t.Setenv(key, value)
	}
	environment = runtimeEnvironment(root)
	for _, entry := range environment {
		upper := strings.ToUpper(entry)
		for _, forbidden := range []string{
			"AURAGO_MASTER_KEY=",
			"OPENAI_API_KEY=",
			"CUSTOM_AUTH_TOKEN=",
			"MY_DB_PASSWORD=",
			"PROJECT_CREDENTIALS=",
		} {
			if strings.HasPrefix(upper, forbidden) {
				t.Fatalf("sensitive environment variable escaped into native process: %s", forbidden)
			}
		}
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

func TestFuncEmbedderCompatibilityAndVectorMath(t *testing.T) {
	embedder := NewFuncEmbedder(func(_ context.Context, text string) ([]float32, error) {
		if text == "error" {
			return nil, errors.New("remote failed")
		}
		return []float32{3, 4}, nil
	}, "custom-model", "custom-fingerprint")
	vectors, err := embedder.Embed(context.Background(), []string{"one", "two"})
	if err != nil || len(vectors) != 2 {
		t.Fatalf("custom embedder vectors = %#v, %v", vectors, err)
	}
	if embedder.Dimensions() != 2 || embedder.ModelID() != "custom-model" ||
		embedder.Fingerprint() != "custom-fingerprint" || embedder.Status().State != "ready" {
		t.Fatalf("custom embedder metadata is inconsistent: %#v", embedder.Status())
	}
	normalized := append([]float32(nil), vectors[0]...)
	if err := l2Normalize(normalized); err != nil {
		t.Fatal(err)
	}
	if math.Abs(cosineSimilarity(normalized, []float32{0.6, 0.8})-1) > 0.0001 {
		t.Fatalf("normalized vector cosine mismatch: %#v", normalized)
	}
	if err := l2Normalize([]float32{0, 0}); err == nil {
		t.Fatal("zero vector unexpectedly normalized")
	}
	if err := l2Normalize([]float32{float32(math.NaN())}); err == nil {
		t.Fatal("non-finite vector unexpectedly normalized")
	}
	if _, err := embedder.Embed(context.Background(), []string{"error"}); err == nil {
		t.Fatal("custom provider error was not propagated")
	}
	if err := embedder.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := embedder.Embed(context.Background(), []string{"closed"}); err == nil {
		t.Fatal("closed custom embedder accepted input")
	}
	if embedder.Status().State != "closed" {
		t.Fatalf("closed custom embedder status = %#v", embedder.Status())
	}
}

func TestRuntimeDiscoveryAndBackendHelpers(t *testing.T) {
	root := t.TempDir()
	serverName := "llama-server"
	if runtime.GOOS == "windows" {
		serverName += ".exe"
	}
	serverPath := filepath.Join(root, "nested", serverName)
	if err := os.MkdirAll(filepath.Dir(serverPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serverPath, []byte("binary"), 0o750); err != nil {
		t.Fatal(err)
	}
	if found, err := findLlamaServer(root); err != nil || found != serverPath {
		t.Fatalf("llama-server discovery = %q, %v", found, err)
	}
	if port, err := availableLoopbackPort(); err != nil || port <= 0 {
		t.Fatalf("loopback port = %d, %v", port, err)
	}
	if key, err := randomAPIKey(); err != nil || len(key) < 32 {
		t.Fatalf("random API key length = %d, %v", len(key), err)
	}
	if onnxProviderName("cuda") != "CUDAExecutionProvider" ||
		onnxProviderName("coreml") != "CoreMLExecutionProvider" ||
		onnxProviderName("directml") != "" {
		t.Fatal("ONNX provider mapping still exposes an unsupported backend")
	}
	if !containsStringFold([]string{"CPUExecutionProvider", "CUDAExecutionProvider"}, "cudaexecutionprovider") {
		t.Fatal("case-insensitive provider lookup failed")
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
	modelMount := dockerModelMount{
		Type:          "volume",
		Source:        "aurago_data",
		Target:        "/app/data/embeddings/models/gguf",
		VolumeSubpath: "embeddings/models/gguf",
	}
	payload, err := dockerLlamaContainerPayload(
		llamaDockerCUDAImage,
		"/app/data/embeddings/models/granite.gguf",
		"ephemeral-key",
		"cuda",
		2048,
		2048,
		modelMount,
		"aurago-granite-private",
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
	if _, inherited := hostConfig["VolumesFrom"]; inherited {
		t.Fatal("sidecar must not inherit AuraGo volumes")
	}
	mounts, ok := hostConfig["Mounts"].([]map[string]any)
	if !ok || len(mounts) != 1 || mounts[0]["ReadOnly"] != true {
		t.Fatalf("sidecar mounts = %#v, want exactly one read-only model mount", hostConfig["Mounts"])
	}
	volumeOptions, ok := mounts[0]["VolumeOptions"].(map[string]any)
	if !ok || volumeOptions["Subpath"] != modelMount.VolumeSubpath {
		t.Fatalf("model volume subpath = %#v", mounts[0]["VolumeOptions"])
	}
	if hostConfig["NetworkMode"] != "aurago-granite-private" {
		t.Fatalf("sidecar network = %#v", hostConfig["NetworkMode"])
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
	networkPayload := dockerPrivateNetworkPayload("aurago-granite-private")
	if networkPayload["Internal"] != true || networkPayload["Attachable"] != false {
		t.Fatalf("embedding sidecar network is not private: %#v", networkPayload)
	}
}

func TestDockerModelMountUsesReadOnlyVolumeSubpathAndRejectsUnsafeOldEngine(t *testing.T) {
	mounts := []dockerContainerMount{{
		Type:        "volume",
		Name:        "aurago_data",
		Destination: "/app/data",
	}}
	selected, err := selectDockerModelMount("/app/data/embeddings/models/gguf", "1.45", mounts)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Source != "aurago_data" || selected.VolumeSubpath != "embeddings/models/gguf" {
		t.Fatalf("selected mount = %#v", selected)
	}
	if _, err := selectDockerModelMount("/app/data/embeddings/models/gguf", "1.44", mounts); err == nil {
		t.Fatal("old Docker Engine unexpectedly accepted a parent named volume")
	}
	exact := []dockerContainerMount{{
		Type:        "volume",
		Name:        "aurago_embeddings",
		Destination: "/app/data/embeddings",
	}}
	selected, err = selectDockerModelMount("/app/data/embeddings/models/gguf", "1.44", exact)
	if err != nil || selected.VolumeSubpath != "" || selected.Target != "/app/data/embeddings" {
		t.Fatalf("old Docker Engine exact mount = %#v, %v", selected, err)
	}
}

func TestRuntimeManifestRepairsTamperedMissingAndUnexpectedFilesOffline(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, content := range map[string]string{
		"runtime/bin/runtime.dll": "verified runtime",
		"runtime/bin/helper.exe":  "verified helper",
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	content := archive.Bytes()
	hash := sha256.Sum256(content)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	cache := newAssetCache(t.TempDir(), nil)
	cache.client = server.Client()
	spec := assetSpec{
		ID:     "test-runtime",
		URL:    server.URL + "/runtime.zip",
		Size:   int64(len(content)),
		SHA256: hex.EncodeToString(hash[:]),
		Kind:   assetZip,
	}
	root, err := cache.ensureRuntimeAsset(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	server.Close()

	runtimeFile := filepath.Join(root, "runtime", "bin", "runtime.dll")
	helperFile := filepath.Join(root, "runtime", "bin", "helper.exe")
	assertRepaired := func(path, want string) {
		t.Helper()
		raw, readErr := os.ReadFile(path)
		if readErr != nil || string(raw) != want {
			t.Fatalf("repaired file %s = %q, %v; want %q", path, raw, readErr, want)
		}
	}
	if err := os.WriteFile(runtimeFile, []byte("tampered runtime"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.ensureRuntimeAsset(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	assertRepaired(runtimeFile, "verified runtime")

	if err := os.Remove(helperFile); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.ensureRuntimeAsset(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	assertRepaired(helperFile, "verified helper")

	unexpected := filepath.Join(root, "runtime", "bin", "injected.dll")
	if err := os.WriteFile(unexpected, []byte("unexpected"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.ensureRuntimeAsset(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(unexpected); !os.IsNotExist(err) {
		t.Fatalf("unexpected runtime file was not removed: %v", err)
	}

	if err := os.Remove(runtimeFile); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(helperFile, runtimeFile); err == nil {
		if _, err := cache.ensureRuntimeAsset(context.Background(), spec); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(runtimeFile)
		if err != nil {
			t.Fatalf("linked runtime file was not replaced: %v", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("linked runtime file was not replaced: mode=%v", info.Mode())
		}
		assertRepaired(runtimeFile, "verified runtime")
	}
}

func TestBenchmarkCoordinatorDeduplicatesFailuresAndStopsAtClose(t *testing.T) {
	rootCtx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	close(ready)
	unblock := make(chan struct{})
	var setupCalls atomic.Int32
	vector := make([]float32, GraniteDimensions)
	vector[0] = 1
	manager := &Manager{
		active: &testEmbedder{
			status: Status{Runtime: "onnxruntime", Backend: "cuda", Fingerprint: onnxFingerprint()},
			err:    errors.New("GPU backend failed"),
		},
		fallback: &testEmbedder{
			status:  Status{Runtime: "onnxruntime", Backend: "cpu", Fingerprint: onnxFingerprint()},
			vectors: [][]float32{vector},
		},
		status:  Status{State: "ready"},
		logger:  discardEmbeddingLogger(),
		rootCtx: rootCtx,
		cancel:  cancel,
		ready:   ready,
		setupOverride: func(ctx context.Context, _ bool) error {
			setupCalls.Add(1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-unblock:
				return nil
			}
		},
	}
	var callers sync.WaitGroup
	for i := 0; i < 12; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			_, _ = manager.Embed(context.Background(), []string{"parallel failure"})
		}()
	}
	callers.Wait()
	deadline := time.Now().Add(time.Second)
	for setupCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if setupCalls.Load() != 1 {
		t.Fatalf("parallel backend failures started %d benchmarks, want 1", setupCalls.Load())
	}
	close(unblock)
	manager.initWG.Wait()
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	manager.triggerBackgroundBenchmark()
	time.Sleep(10 * time.Millisecond)
	if setupCalls.Load() != 1 {
		t.Fatalf("benchmark started after Close: %d calls", setupCalls.Load())
	}
}

func TestBenchmarkCoordinatorManualRequestJoinsRunningBenchmark(t *testing.T) {
	rootCtx, cancel := context.WithCancel(context.Background())
	unblock := make(chan struct{})
	var setupCalls atomic.Int32
	manager := &Manager{
		logger:  discardEmbeddingLogger(),
		rootCtx: rootCtx,
		cancel:  cancel,
		ready:   make(chan struct{}),
		setupOverride: func(ctx context.Context, _ bool) error {
			setupCalls.Add(1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-unblock:
				return nil
			}
		},
	}
	manager.triggerBackgroundBenchmark()
	done := make(chan error, 1)
	go func() {
		done <- manager.Rebenchmark(context.Background())
	}()
	deadline := time.Now().Add(time.Second)
	for setupCalls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if setupCalls.Load() != 1 {
		t.Fatalf("manual request did not join running benchmark: %d calls", setupCalls.Load())
	}
	close(unblock)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBenchmarkCoordinatorCooldownAndManualBypass(t *testing.T) {
	rootCtx, cancel := context.WithCancel(context.Background())
	var setupCalls atomic.Int32
	manager := &Manager{
		logger:  discardEmbeddingLogger(),
		rootCtx: rootCtx,
		cancel:  cancel,
		ready:   make(chan struct{}),
		setupOverride: func(context.Context, bool) error {
			setupCalls.Add(1)
			return errors.New("probe failed")
		},
	}
	manager.triggerBackgroundBenchmark()
	manager.initWG.Wait()
	manager.triggerBackgroundBenchmark()
	manager.initWG.Wait()
	if setupCalls.Load() != 1 {
		t.Fatalf("automatic cooldown allowed %d setup calls, want 1", setupCalls.Load())
	}
	if err := manager.Rebenchmark(context.Background()); err == nil || !strings.Contains(err.Error(), "probe failed") {
		t.Fatalf("manual cooldown bypass error = %v", err)
	}
	if setupCalls.Load() != 2 {
		t.Fatalf("manual request did not bypass cooldown: %d calls", setupCalls.Load())
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBenchmarkCoordinatorCloseCancelsRootTaskQuickly(t *testing.T) {
	rootCtx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	manager := &Manager{
		logger:  discardEmbeddingLogger(),
		rootCtx: rootCtx,
		cancel:  cancel,
		ready:   make(chan struct{}),
		setupOverride: func(ctx context.Context, _ bool) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		},
	}
	manager.triggerBackgroundBenchmark()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background benchmark did not start")
	}
	start := time.Now()
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("manager shutdown took %v after cancellation", elapsed)
	}
}

func TestManagerUsesPersistedActualFingerprintBeforeActivation(t *testing.T) {
	cacheDir := t.TempDir()
	seed := &Manager{options: LocalOptions{CacheDir: cacheDir}}
	if err := seed.saveSelection(selectionState{
		Version:              1,
		SelectionFingerprint: "previous-selection",
		HardwareFingerprint:  "previous-hardware",
		CandidateID:          "llama-vulkan",
		EmbeddingFingerprint: ggufFingerprint(),
		SelectedAt:           time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	manager, err := NewLocalGranite(LocalOptions{
		CacheDir:        cacheDir,
		ResetMarkerPath: filepath.Join(cacheDir, "reset.json"),
		Backend:         "auto",
		Logger:          discardEmbeddingLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := manager.Fingerprint(); got != ggufFingerprint() {
		t.Fatalf("pre-activation fingerprint = %q, want persisted GGUF fingerprint", got)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
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
