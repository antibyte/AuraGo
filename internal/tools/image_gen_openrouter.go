package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
		"modalities": []string{"image"},
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

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("OpenRouter returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	// Parse response — look for image data in choices[0].message
	var result struct {
		Choices []struct {
			Message struct {
				Content interface{} `json:"content"` // may be string or array
				Images  []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"` // used by models like bytedance-seed/seedream-*
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse OpenRouter response: %w", err)
	}
	if len(result.Choices) == 0 {
		return nil, "", fmt.Errorf("OpenRouter returned no choices")
	}

	msg := result.Choices[0].Message

	// ── Check message.images first (e.g. bytedance-seed/seedream-*) ──────
	for _, img := range msg.Images {
		urlStr := img.ImageURL.URL
		if urlStr == "" {
			continue
		}
		// Base64 data URL: data:image/png;base64,...
		if idx := strings.Index(urlStr, "base64,"); idx >= 0 {
			b64 := urlStr[idx+7:]
			if imgData, err := base64.StdEncoding.DecodeString(b64); err == nil && len(imgData) > 100 {
				return imgData, detectFormat(imgData), nil
			}
		}
		// Direct HTTP URL
		if imgData, ext, err := tryDownloadImageURL(urlStr); err == nil {
			return imgData, ext, nil
		}
	}

	content := msg.Content

	// Content might be a string with base64/URL, or an array of content blocks
	switch v := content.(type) {
	case string:
		if imgData, ext, err := tryDecodeImageString(v); err == nil {
			return imgData, ext, nil
		}
		return nil, "", fmt.Errorf("could not extract image data from OpenRouter string response (len=%d, preview=%q)", len(v), truncateError(v))

	case []interface{}:
		if imgData, ext, err := extractImageFromAnyResponse(v); err == nil {
			return imgData, ext, nil
		}
		return nil, "", fmt.Errorf("no image_url block found in OpenRouter response")

	case map[string]interface{}:
		if imgData, ext, err := extractImageFromAnyResponse(v); err == nil {
			return imgData, ext, nil
		}
		return nil, "", fmt.Errorf("could not extract image data from OpenRouter object response")

	case nil:
		return nil, "", fmt.Errorf("OpenRouter returned null content (model may not support image generation or prompt was refused)")

	default:
		// Final fallback: search the full raw response recursively for image payloads.
		var anyResp interface{}
		if err := json.Unmarshal(respBody, &anyResp); err == nil {
			if imgData, ext, walkErr := extractImageFromAnyResponse(anyResp); walkErr == nil {
				return imgData, ext, nil
			}
		}
		return nil, "", fmt.Errorf("unexpected content type in OpenRouter response: %T (raw: %s)", content, truncateError(string(respBody)))
	}
}
