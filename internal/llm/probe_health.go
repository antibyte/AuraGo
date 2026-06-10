package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

func probePrimaryHealth(ctx context.Context, client *openai.Client, providerType, baseURL, apiKey string) error {
	pt := strings.ToLower(strings.TrimSpace(providerType))

	switch pt {
	case "ollama":
		if err := probeOllamaTags(ctx, baseURL); err == nil {
			return nil
		}
	case "anthropic":
		if err := probeModelsEndpoints(ctx, baseURL, apiKey); err == nil {
			return nil
		}
	default:
		if client != nil {
			if _, err := client.ListModels(ctx); err == nil {
				return nil
			}
		}
		if err := probeModelsEndpoints(ctx, baseURL, apiKey); err == nil {
			return nil
		}
	}

	return fmt.Errorf("primary health probe failed for provider %q", providerType)
}

func probeOllamaTags(ctx context.Context, baseURL string) error {
	ollamaBase := strings.TrimSuffix(strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(baseURL), "/"), "/v1"), "/")
	if ollamaBase == "" {
		return fmt.Errorf("empty ollama base url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaBase+"/api/tags", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("ollama tags probe status %d", resp.StatusCode)
}

func probeModelsEndpoints(ctx context.Context, baseURL, apiKey string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	for _, modelsURL := range modelsProbeCandidates(baseURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
		if err != nil {
			continue
		}
		if strings.TrimSpace(apiKey) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
	}
	return fmt.Errorf("models endpoint probe failed")
}

func modelsProbeCandidates(baseURL string) []string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil
	}
	stripped := base
	if strings.HasSuffix(stripped, "/v1") {
		stripped = strings.TrimSuffix(stripped, "/v1")
	}
	if strings.HasSuffix(stripped, "/api") {
		return []string{stripped + "/v1/models"}
	}
	return []string{
		stripped + "/api/v1/models",
		stripped + "/v1/models",
		base + "/models",
	}
}