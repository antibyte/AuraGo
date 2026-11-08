package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
)

const (
	mirageModelName        = "mirage"
	mirageRequestTimeout   = 90 * time.Second // first request after a cold start may wait for the model
	mirageMaxResponseBytes = 1 << 20
)

// Hooks so tests can point the client at a fake sidecar and fake the main
// model's capabilities.
var (
	mirageBaseURLFor        = func(cfg *config.Config) string { return cfg.MirageBaseURL() }
	mirageMainSupportsImage = func(cfg *config.Config) bool { return llm.ResolveConfigProviderCapabilities(cfg).Image }
)

// UseMirageVision reports whether image analysis should go to the local Mirage
// sidecar. An explicitly configured vision provider always wins. In "fallback"
// mode Mirage is used only while the main model cannot take images (a
// simplification of the server's mainProviderSupportsImageMultimodal, which
// additionally consults multimodal_provider_types_extra).
func UseMirageVision(cfg *config.Config) bool {
	if cfg == nil || !cfg.Vision.Mirage.Enabled {
		return false
	}
	if strings.TrimSpace(cfg.Vision.Provider) != "" {
		return false
	}
	if cfg.MirageMode() == config.MirageModeAlways {
		return true
	}
	return !mirageMainSupportsImage(cfg)
}

// analyzeWithMirage asks the sidecar for a text description of a data-URL
// image. It deliberately never reads cfg.LLM.APIKey: the sidecar needs no key
// and must not receive the main provider's token.
func analyzeWithMirage(ctx context.Context, imageReference string, cfg *config.Config) (string, int, int, error) {
	if !strings.HasPrefix(imageReference, "data:image/") {
		return "", 0, 0, fmt.Errorf("mirage vision can only analyze local images, not URLs")
	}
	body, err := json.Marshal(map[string]interface{}{
		"model": mirageModelName,
		"messages": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "image_url", "image_url": map[string]string{"url": imageReference}},
			},
		}},
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage request encoding failed: %w", err)
	}
	endpoint := strings.TrimRight(mirageBaseURLFor(cfg), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout:       mirageRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage sidecar is not reachable (is it enabled and started?): %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, mirageMaxResponseBytes))
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage response read failed: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusServiceUnavailable:
		return "", 0, 0, fmt.Errorf("mirage model is still loading, try again shortly")
	case resp.StatusCode != http.StatusOK:
		return "", 0, 0, fmt.Errorf("mirage vision error (status %d)", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("no analysis received in mirage response")
	}
	return parsed.Choices[0].Message.Content, 0, 0, nil
}
