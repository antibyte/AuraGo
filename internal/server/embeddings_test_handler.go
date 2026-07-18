package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aurago/internal/embeddings"
)

type activeEmbeddingRuntime interface {
	EmbeddingStatus() embeddings.Status
	TestEmbedding(context.Context, string) ([]float32, error)
	RebenchmarkEmbeddings(context.Context) error
}

func serverEmbeddingRuntime(s *Server) (activeEmbeddingRuntime, bool) {
	if s == nil || s.LongTermMem == nil {
		return nil, false
	}
	runtime, ok := s.LongTermMem.(activeEmbeddingRuntime)
	return runtime, ok
}

// handleEmbeddingsTest tests the exact Embedder used by VectorDB and KG.
func handleEmbeddingsTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runtime, ok := serverEmbeddingRuntime(s)
		if !ok {
			jsonError(w, "The active embedding runtime is unavailable.", http.StatusServiceUnavailable)
			return
		}
		timeout := 30 * time.Second
		status := runtime.EmbeddingStatus()
		if status.Provider == embeddings.LocalGraniteProvider || status.State == "setting_up" || status.State == "benchmarking" {
			timeout = 10 * time.Minute
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		vector, err := runtime.TestEmbedding(ctx, "AuraGo local embedding connection test")
		if err != nil {
			s.Logger.Error("Embeddings test failed", "provider", status.Provider, "runtime", status.Runtime, "backend", status.Backend, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("Test failed: %v", err),
				"runtime": runtime.EmbeddingStatus(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"message":    fmt.Sprintf("Embedding generated successfully with %d dimensions.", len(vector)),
			"dimensions": len(vector),
			"runtime":    runtime.EmbeddingStatus(),
		})
	}
}

func handleEmbeddingsStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runtime, ok := serverEmbeddingRuntime(s)
		if !ok {
			jsonError(w, "The active embedding runtime is unavailable.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(runtime.EmbeddingStatus())
	}
}

func handleEmbeddingsBenchmark(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runtime, ok := serverEmbeddingRuntime(s)
		if !ok {
			jsonError(w, "The active embedding runtime is unavailable.", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
		defer cancel()
		if err := runtime.RebenchmarkEmbeddings(ctx); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
				"runtime": runtime.EmbeddingStatus(),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"runtime": runtime.EmbeddingStatus(),
		})
	}
}
