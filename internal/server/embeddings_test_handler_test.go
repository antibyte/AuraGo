package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/embeddings"
	"aurago/internal/memory"
)

type fakeActiveEmbeddingVectorDB struct {
	memory.VectorDB
	status         embeddings.Status
	vector         []float32
	testErr        error
	benchmarkErr   error
	testText       string
	benchmarkCalls int
}

func (fake *fakeActiveEmbeddingVectorDB) EmbeddingStatus() embeddings.Status {
	return fake.status
}

func (fake *fakeActiveEmbeddingVectorDB) TestEmbedding(_ context.Context, text string) ([]float32, error) {
	fake.testText = text
	return fake.vector, fake.testErr
}

func (fake *fakeActiveEmbeddingVectorDB) RebenchmarkEmbeddings(context.Context) error {
	fake.benchmarkCalls++
	return fake.benchmarkErr
}

func TestEmbeddingsStatusUsesActiveRuntime(t *testing.T) {
	runtime := &fakeActiveEmbeddingVectorDB{status: embeddings.Status{
		State:      "ready",
		Provider:   embeddings.LocalGraniteProvider,
		Runtime:    "onnxruntime",
		Backend:    "cpu",
		Dimensions: embeddings.GraniteDimensions,
	}}
	server := &Server{
		LongTermMem: runtime,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	request := httptest.NewRequest(http.MethodGet, "/api/embeddings/status", nil)
	response := httptest.NewRecorder()
	handleEmbeddingsStatus(server).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", response.Code, response.Body.String())
	}
	var status embeddings.Status
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Provider != embeddings.LocalGraniteProvider || status.Runtime != "onnxruntime" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestEmbeddingsTestUsesActiveEmbedder(t *testing.T) {
	runtime := &fakeActiveEmbeddingVectorDB{
		status: embeddings.Status{State: "ready", Provider: "custom"},
		vector: make([]float32, 768),
	}
	server := &Server{
		LongTermMem: runtime,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	request := httptest.NewRequest(http.MethodPost, "/api/embeddings/test", nil)
	response := httptest.NewRecorder()
	handleEmbeddingsTest(server).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", response.Code, response.Body.String())
	}
	if runtime.testText != "AuraGo local embedding connection test" {
		t.Fatalf("test text = %q", runtime.testText)
	}
	var body struct {
		Status     string `json:"status"`
		Dimensions int    `json:"dimensions"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Dimensions != 768 {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestEmbeddingsBenchmarkReportsRuntimeError(t *testing.T) {
	runtime := &fakeActiveEmbeddingVectorDB{
		status:       embeddings.Status{State: "degraded", Provider: embeddings.LocalGraniteProvider},
		benchmarkErr: errors.New("GPU probe failed"),
	}
	server := &Server{
		LongTermMem: runtime,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	request := httptest.NewRequest(http.MethodPost, "/api/embeddings/benchmark", nil)
	response := httptest.NewRecorder()
	handleEmbeddingsBenchmark(server).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || runtime.benchmarkCalls != 1 {
		t.Fatalf("status = %d, benchmark calls = %d, body = %s", response.Code, runtime.benchmarkCalls, response.Body.String())
	}
}

func TestLocalGraniteRejectsMultimodalConfiguration(t *testing.T) {
	var cfg config.Config
	cfg.Embeddings.Provider = embeddings.LocalGraniteProvider
	cfg.Embeddings.Multimodal = true
	if err := validateLocalGraniteEmbeddingMode(&cfg); err == nil {
		t.Fatal("expected text-only local Granite validation error")
	}
	cfg.Embeddings.Multimodal = false
	if err := validateLocalGraniteEmbeddingMode(&cfg); err != nil {
		t.Fatalf("text-only local Granite config was rejected: %v", err)
	}
	cfg.Embeddings.Provider = "custom-multimodal"
	cfg.Embeddings.Multimodal = true
	if err := validateLocalGraniteEmbeddingMode(&cfg); err != nil {
		t.Fatalf("custom multimodal provider was rejected: %v", err)
	}
}
