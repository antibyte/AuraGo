package telegram

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/providerutil"
)

// AnalyzeImage sends an image file to OpenRouter using a vision LLM for analysis.
func AnalyzeImage(filePath string, cfg *config.Config) (string, error) {
	apiKey := strings.TrimSpace(cfg.Vision.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(cfg.LLM.APIKey)
	}

	baseURL := providerutil.NormalizeBaseURL(strings.TrimSpace(cfg.Vision.BaseURL))
	if baseURL == "" {
		baseURL = providerutil.NormalizeBaseURL(strings.TrimSpace(cfg.LLM.BaseURL))
	}

	model := strings.TrimSpace(cfg.Vision.Model)
	if model == "" {
		model = "google/gemini-2.5-flash-lite-preview-09-2025"
	}

	// 1. Read and Base64 encode the image file
	imageData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}

	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(filePath), ".png") {
		mimeType = "image/png"
	} else if strings.HasSuffix(strings.ToLower(filePath), ".gif") {
		mimeType = "image/gif"
	} else if strings.HasSuffix(strings.ToLower(filePath), ".webp") {
		mimeType = "image/webp"
	}

	encodedImage := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, encodedImage)

	// 2. Construct OpenAI-style Vision Payload
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
					{
						Type: "text",
						Text: "Describe this image in detail. What do you see? If there is text, transcribe it. If there are people, describe their actions.",
					},
					{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL: dataURL,
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 3. Send Request to OpenRouter
	reqURL := baseURL + "/chat/completions"
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/andre/aurago")
	req.Header.Set("X-Title", "AuraGo")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vision API error (status %d): %s", resp.StatusCode, string(body))
	}

	// 4. Parse Response
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no analysis received in response")
	}

	return result.Choices[0].Message.Content, nil
}
