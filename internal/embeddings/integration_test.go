package embeddings

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestONNXCPUIntegration(t *testing.T) {
	if os.Getenv("AURAGO_GRANITE_INTEGRATION") != "1" {
		t.Skip("set AURAGO_GRANITE_INTEGRATION=1 to download and test the pinned ONNX CPU runtime")
	}
	asset, ok := onnxRuntimeAsset(runtime.GOOS, runtime.GOARCH, "cpu")
	if !ok {
		t.Skipf("ONNX Runtime 1.26.0 CPU archive is unavailable for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	cacheDir := integrationCacheDir(t)
	cache := newAssetCache(cacheDir, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	runtimeRoot, err := cache.ensureRuntimeAsset(ctx, asset)
	if err != nil {
		t.Fatal(err)
	}
	model := modelPath(cacheDir, "onnx")
	if err := cache.ensureDirectAsset(ctx, graniteONNXModelAsset, model); err != nil {
		t.Fatal(err)
	}
	tokenizerFile := tokenizerPath(cacheDir)
	if err := cache.ensureDirectAsset(ctx, graniteTokenizerAsset, tokenizerFile); err != nil {
		t.Fatal(err)
	}
	tokenizer, err := loadGraniteTokenizer(tokenizerFile)
	if err != nil {
		t.Fatal(err)
	}
	session, err := newORTSession(runtimeRoot, model, "cpu")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	texts := integrationTexts()
	batch, err := tokenizer.encodeBatch(texts, 2048)
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := session.Embed(batch)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegrationVectors(t, vectors, len(texts))
	assertSemanticPair(t, vectors)
}

func TestLlamaCPUIntegration(t *testing.T) {
	if os.Getenv("AURAGO_GRANITE_INTEGRATION") != "1" {
		t.Skip("set AURAGO_GRANITE_INTEGRATION=1 to download and test the pinned llama.cpp CPU runtime")
	}
	asset, ok := llamaRuntimeAsset(runtime.GOOS, runtime.GOARCH, "cpu")
	if !ok {
		t.Skipf("llama.cpp b9994 CPU archive is unavailable for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	cacheDir := integrationCacheDir(t)
	cache := newAssetCache(cacheDir, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	runtimeRoot, err := cache.ensureRuntimeAsset(ctx, asset)
	if err != nil {
		t.Fatal(err)
	}
	model := modelPath(cacheDir, "gguf")
	if err := cache.ensureDirectAsset(ctx, graniteGGUFModelAsset, model); err != nil {
		t.Fatal(err)
	}
	embedder, err := newLlamaEmbedder(ctx, runtimeRoot, model, "cpu", 2048, 2048)
	if err != nil {
		t.Fatal(err)
	}
	defer embedder.Close()
	texts := integrationTexts()
	vectors, err := embedder.Embed(ctx, texts)
	if err != nil {
		t.Fatal(err)
	}
	assertIntegrationVectors(t, vectors, len(texts))
	assertSemanticPair(t, vectors)
}

func TestOptionalHardwareBackend(t *testing.T) {
	backend := os.Getenv("AURAGO_GRANITE_HARDWARE_BACKEND")
	if backend == "" {
		t.Skip("set AURAGO_GRANITE_HARDWARE_BACKEND to cuda, directml, coreml, metal, or vulkan")
	}
	hardware := detectHardware(context.Background())
	candidates := candidateMatrix(runtime.GOOS, runtime.GOARCH, backend, hardware)
	var selected candidate
	found := false
	for _, current := range candidates {
		if current.Backend == backend && current.GPU {
			selected = current
			found = true
			break
		}
	}
	if !found {
		t.Skipf("%s is unavailable on %s/%s", backend, runtime.GOOS, runtime.GOARCH)
	}
	cacheDir := integrationCacheDir(t)
	manager := &Manager{
		options: LocalOptions{
			CacheDir:        cacheDir,
			ResetMarkerPath: filepath.Join(cacheDir, "reset.json"),
			Backend:         backend,
			ContextSize:     2048,
			BatchSize:       2048,
		},
		cache:  newAssetCache(cacheDir, nil),
		logger: discardEmbeddingLogger(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	embedder, err := manager.startCandidate(ctx, selected)
	if err != nil {
		t.Skipf("hardware backend probe unavailable: %v", err)
	}
	defer embedder.Close()
	vectors, err := embedder.Embed(ctx, integrationTexts())
	if err != nil {
		t.Fatal(err)
	}
	assertIntegrationVectors(t, vectors, len(integrationTexts()))
	if !embedder.Status().GPUVerified {
		t.Fatalf("%s produced vectors but did not verify actual GPU execution", backend)
	}
}

func integrationCacheDir(t *testing.T) string {
	t.Helper()
	if cacheDir := os.Getenv("AURAGO_GRANITE_INTEGRATION_CACHE"); cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o750); err != nil {
			t.Fatal(err)
		}
		return cacheDir
	}
	return t.TempDir()
}

func integrationTexts() []string {
	return []string{
		"Wie richte ich lokale Embeddings ein?",
		"How do I configure local embeddings?",
		"ローカル埋め込みを設定する方法",
		"func Embed(ctx context.Context, text string) ([]float32, error)",
		"Berlin ist die Hauptstadt von Deutschland.",
		"Welche Stadt ist die Hauptstadt Deutschlands?",
		"Bananen wachsen in tropischen Regionen.",
	}
}

func assertIntegrationVectors(t *testing.T, vectors [][]float32, expected int) {
	t.Helper()
	if len(vectors) != expected {
		t.Fatalf("vector count = %d, want %d", len(vectors), expected)
	}
	for i := range vectors {
		if err := validateGraniteVector(vectors[i]); err != nil {
			t.Fatalf("vector %d: %v", i, err)
		}
	}
}

func assertSemanticPair(t *testing.T, vectors [][]float32) {
	t.Helper()
	if len(vectors) < 7 {
		t.Fatal("semantic integration corpus is incomplete")
	}
	related := cosineSimilarity(vectors[4], vectors[5])
	unrelated := cosineSimilarity(vectors[4], vectors[6])
	if related <= unrelated {
		t.Fatalf("semantic pair score %.4f is not above unrelated score %.4f", related, unrelated)
	}
}

func discardEmbeddingLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
