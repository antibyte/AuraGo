package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var imageGenHTTPClient = &http.Client{Timeout: 120 * time.Second}

// generateOpenRouter generates an image using OpenRouter's chat/completions endpoint.
// Supports models like flux-2-pro, gpt-5-image that return base64 images inline.
func generateOpenRouter(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	url := strings.TrimRight(baseURL, "/") + "/chat/completions"

	body := map[string]interface{}{
		"model": cfg.Model,
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
	}

	// If source image is provided (image-to-image), include it as a vision message
	if opts.SourceImage != "" {
		srcData, err := loadSourceImage(opts.SourceImage)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load source image: %w", err)
		}
		body["messages"] = []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(srcData),
						},
					},
					{
						"type": "text",
						"text": prompt,
					},
				},
			},
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("OpenRouter request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("OpenRouter returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	// Parse response — look for base64 image data in choices[0].message.content
	var result struct {
		Choices []struct {
			Message struct {
				Content interface{} `json:"content"` // may be string or array
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse OpenRouter response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, "", fmt.Errorf("OpenRouter returned no choices")
	}

	content := result.Choices[0].Message.Content

	// Content might be a string with base64, or an array of content blocks
	switch v := content.(type) {
	case string:
		// Try to decode as raw base64
		imgData, err := base64.StdEncoding.DecodeString(v)
		if err == nil && len(imgData) > 100 {
			return imgData, "png", nil
		}
		// Check if it contains a data URI
		if idx := strings.Index(v, "base64,"); idx >= 0 {
			b64 := v[idx+7:]
			if end := strings.IndexAny(b64, "\" \n\r)"); end > 0 {
				b64 = b64[:end]
			}
			imgData, err = base64.StdEncoding.DecodeString(b64)
			if err == nil && len(imgData) > 100 {
				return imgData, detectFormat(imgData), nil
			}
		}
		return nil, "", fmt.Errorf("could not extract image data from OpenRouter response")

	case []interface{}:
		// Array of content blocks — look for type=image_url
		for _, block := range v {
			m, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			if m["type"] == "image_url" {
				if imgURL, ok := m["image_url"].(map[string]interface{}); ok {
					if urlStr, ok := imgURL["url"].(string); ok {
						if idx := strings.Index(urlStr, "base64,"); idx >= 0 {
							b64 := urlStr[idx+7:]
							imgData, err := base64.StdEncoding.DecodeString(b64)
							if err == nil && len(imgData) > 100 {
								return imgData, detectFormat(imgData), nil
							}
						}
					}
				}
			}
		}
		return nil, "", fmt.Errorf("no image_url block found in OpenRouter response")

	default:
		return nil, "", fmt.Errorf("unexpected content type in OpenRouter response")
	}
}
