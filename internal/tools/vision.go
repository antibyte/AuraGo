package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/providerutil"
)

var visionHTTPClient = &http.Client{Timeout: 60 * time.Second}

// AnalyzeImageWithPrompt sends an image file to the configured Vision LLM for analysis.
// The prompt parameter controls what the model should focus on.
func AnalyzeImageWithPrompt(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
	resolvedPath, err := resolveToolInputPath(filePath, cfg)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid image file path: %w", err)
	}

	// Resolved by config.ResolveProviders (falls back to main LLM)
	apiKey := strings.TrimSpace(cfg.Vision.APIKey)
	baseURL := providerutil.NormalizeBaseURL(strings.TrimSpace(cfg.Vision.BaseURL))

	model := strings.TrimSpace(cfg.Vision.Model)
	if model == "" {
		model = "google/gemini-2.0-flash-001"
	}

	if apiKey == "" {
		apiKey = strings.TrimSpace(cfg.LLM.APIKey)
	}
	if apiKey == "" {
		return "", 0, 0, fmt.Errorf("vision API key is not configured — set vision.provider or vision.api_key in config, or ensure the main LLM API key is set as fallback")
	}
	if baseURL == "" {
		baseURL = providerutil.NormalizeBaseURL(strings.TrimSpace(cfg.LLM.BaseURL))
	}
	if baseURL == "" {
		return "", 0, 0, fmt.Errorf("vision base URL is not configured")
	}

	// Read and base64-encode the image
	visionInfo, err := os.Stat(resolvedPath)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to stat image file: %w", err)
	}
	const maxVisionBytes = 50 * 1024 * 1024 // 50 MB
	if visionInfo.Size() > maxVisionBytes {
		return "", 0, 0, fmt.Errorf("image file too large (%d bytes, max 50 MB)", visionInfo.Size())
	}
	imageData, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to read image file: %w", err)
	}

	mimeType := "image/jpeg"
	lower := strings.ToLower(resolvedPath)
	switch {
	case strings.HasSuffix(lower, ".png"):
		mimeType = "image/png"
	case strings.HasSuffix(lower, ".gif"):
		mimeType = "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		mimeType = "image/webp"
	case strings.HasSuffix(lower, ".bmp"):
		mimeType = "image/bmp"
	}

	encodedImage := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, encodedImage)

	// OpenAI-compatible vision payload
	type ImageURL struct {
		URL string `json:"url"`
	}
	type ContentPart struct {
		Type     string    `json:"type"`
		Text     string    `json:"text,omitempty"`
		ImageURL *ImageURL `json:"image_url,omitempty"`
	}
	type Message struct {
		Role    string        `json:"role"`
		Content []ContentPart `json:"content"`
	}
	type RequestBody struct {
		Model    string    `json:"model"`
		Messages []Message `json:"messages"`
	}

	payload := RequestBody{
		Model: model,
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentPart{
					{Type: "text", Text: prompt},
					{Type: "image_url", ImageURL: &ImageURL{URL: dataURL}},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to marshal vision payload: %w", err)
	}

	reqURL := baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create vision request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/antibyte/AuraGo")
	req.Header.Set("X-Title", "AuraGo")

	resp, err := visionHTTPClient.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("vision request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return "", 0, 0, fmt.Errorf("vision API error (status %d) and failed to read response body: %w", resp.StatusCode, err)
		}
		return "", 0, 0, fmt.Errorf("vision API error (status %d, provider=%s, base_url=%s, auth_present=%t): %s", resp.StatusCode, strings.TrimSpace(cfg.Vision.ProviderType), baseURL, apiKey != "", string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, 0, fmt.Errorf("failed to decode vision response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("no analysis received in vision response")
	}

	return result.Choices[0].Message.Content, result.Usage.PromptTokens, result.Usage.CompletionTokens, nil
}
