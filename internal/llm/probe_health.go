package llm

import (
	"context"
	"errors"
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
		return probeOllamaTags(ctx, baseURL)
	case "anthropic":
		return probeModelsEndpoints(ctx, baseURL, apiKey, pt)
	default:
		if client != nil {
			if _, err := client.ListModels(ctx); err == nil {
				return nil
			} else if !unsupportedHealthProbe(err) {
				return err
			}
		}
		return probeModelsEndpoints(ctx, baseURL, apiKey)
	}
}

func unsupportedHealthProbe(err error) bool {
	var api *openai.APIError
	return errors.As(err, &api) && (api.HTTPStatusCode == 404 || api.HTTPStatusCode == 405 || api.HTTPStatusCode == 501)
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
	client := newProbeHTTPClient(10 * time.Second)
	client.Transport = &rateLimitAwareTransport{base: client.Transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("ollama tags probe status %d", resp.StatusCode)
}

func probeModelsEndpoints(ctx context.Context, baseURL, apiKey string, providerTypes ...string) error {
	client := newProbeHTTPClient(10 * time.Second)
	client.Transport = &rateLimitAwareTransport{base: client.Transport}
	anthropic := len(providerTypes) > 0 && strings.EqualFold(providerTypes[0], "anthropic")
	for _, modelsURL := range modelsProbeCandidates(baseURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
		if err != nil {
			continue
		}
		if anthropic {
			req.Header.Set("x-api-key", strings.TrimSpace(apiKey))
			req.Header.Set("anthropic-version", anthropicAPIVersion)
		} else if strings.TrimSpace(apiKey) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if resp.StatusCode != 404 && resp.StatusCode != 405 && resp.StatusCode != 501 {
			return &openai.APIError{HTTPStatusCode: resp.StatusCode, Message: "models health probe failed"}
		}
	}
	return &LLMError{Category: ErrCategoryProbeInconclusive, Message: "provider has no supported models health endpoint"}
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
