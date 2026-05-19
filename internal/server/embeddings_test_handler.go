package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

// handleEmbeddingsTest tests the embeddings provider connection by attempting to generate a dummy embedding.
func handleEmbeddingsTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		provider := cfg.Embeddings.Provider
		if provider == "disabled" || provider == "" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Embeddings are disabled in the configuration.",
			})
			return
		}

		embedURL := cfg.Embeddings.BaseURL
		embedKey := cfg.Embeddings.APIKey
		embedModel := cfg.Embeddings.Model

		if provider == "internal" {
			embedURL = cfg.LLM.BaseURL
			embedKey = cfg.LLM.APIKey
			if embedModel == "" {
				embedModel = cfg.Embeddings.InternalModel
			}
		}
		if provider == "external" {
			if embedURL == "" {
				embedURL = cfg.Embeddings.ExternalURL
			}
			if embedModel == "" {
				embedModel = cfg.Embeddings.ExternalModel
			}
		}

		if embedModel == "" {
			embedModel = "text-embedding-3-small"
		}

		// Resolve API key from vault if empty
		if embedKey == "" && s.Vault != nil {
			vaultKey := "provider_" + provider + "_api_key"
			if val, err := s.Vault.ReadSecret(vaultKey); err == nil && val != "" {
				embedKey = val
			}
		}

		if embedKey == "" && provider != "ollama" && provider != "local-ollama-embeddings" && !strings.Contains(embedURL, "localhost") && !strings.Contains(embedURL, "127.0.0.1") {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("API key for provider '%s' is empty. Please enter it in the Provider settings or Vault.", provider),
			})
			return
		}

		ef := chromem.NewEmbeddingFuncOpenAICompat(embedURL, embedKey, embedModel, nil)
		
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		vec, err := ef(ctx, "connection test")
		if err != nil {
			s.Logger.Error("Embeddings connection test failed", "provider", provider, "url", embedURL, "model", embedModel, "error", err)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("Test failed: %v", err),
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": fmt.Sprintf("Connection successful! Generated vector of size %d using model '%s'.", len(vec), embedModel),
		})
	}
}
