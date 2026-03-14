package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// generateOpenAI generates an image using OpenAI's dedicated /images/generations endpoint.
// Supports dall-e-3 and dall-e-2 models.
func generateOpenAI(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// Image-to-image uses /images/edits
	if opts.SourceImage != "" {
		return generateOpenAIEdit(cfg, prompt, opts)
	}

	url := strings.TrimRight(baseURL, "/") + "/images/generations"

	body := map[string]interface{}{
		"model":           cfg.Model,
		"prompt":          prompt,
		"n":               1,
		"response_format": "b64_json",
	}
	if opts.Size != "" {
		body["size"] = opts.Size
	}
	if opts.Quality != "" {
		body["quality"] = opts.Quality
	}
	if opts.Style != "" {
		body["style"] = opts.Style
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
		return nil, "", fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("OpenAI returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	var result struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse OpenAI response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, "", fmt.Errorf("OpenAI returned no images")
	}

	imgData, err := base64.StdEncoding.DecodeString(result.Data[0].B64JSON)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image data: %w", err)
	}
	return imgData, "png", nil
}

// generateOpenAIEdit handles image-to-image via /images/edits (dall-e-2 only).
func generateOpenAIEdit(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimRight(baseURL, "/") + "/images/edits"

	srcData, err := loadSourceImage(opts.SourceImage)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load source image: %w", err)
	}

	// Build multipart form
	body, contentType, err := buildMultipartForm(map[string]string{
		"prompt":          prompt,
		"model":           cfg.Model,
		"n":               "1",
		"response_format": "b64_json",
	}, map[string][]byte{
		"image": srcData,
	})
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("OpenAI edit request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("OpenAI edit returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse OpenAI edit response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, "", fmt.Errorf("OpenAI edit returned no images")
	}

	imgData, err := base64.StdEncoding.DecodeString(result.Data[0].B64JSON)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode edit image data: %w", err)
	}
	return imgData, "png", nil
}
