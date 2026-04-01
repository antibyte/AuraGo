package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// generateStability generates an image using Stability AI's REST API.
// Supports sd3-core model via multipart/form-data.
func generateStability(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	model := cfg.Model
	if model == "" {
		model = "sd3-core"
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.stability.ai"
	}
	url := strings.TrimRight(baseURL, "/") + "/v2beta/stable-image/generate/sd3"

	fields := map[string]string{
		"prompt":        prompt,
		"model":         model,
		"output_format": "png",
	}

	// Map size to aspect_ratio for Stability API
	if opts.Size != "" {
		ar := sizeToAspectRatio(opts.Size)
		if ar != "" {
			fields["aspect_ratio"] = ar
		}
	}

	var files map[string][]byte
	if opts.SourceImage != "" {
		srcData, err := loadSourceImage(opts.SourceImage)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load source image: %w", err)
		}
		files = map[string][]byte{"image": srcData}
		fields["mode"] = "image-to-image"
		fields["strength"] = "0.7"
	}

	body, contentType, err := buildMultipartForm(fields, files)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Stability AI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Stability AI returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	var result struct {
		Image     string `json:"image"` // base64-encoded image
		Artifacts []struct {
			Base64 string `json:"base64"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse Stability response: %w", err)
	}

	// Try the direct image field first (v2beta), then artifacts (v1)
	b64 := result.Image
	if b64 == "" && len(result.Artifacts) > 0 {
		b64 = result.Artifacts[0].Base64
	}
	if b64 == "" {
		return nil, "", fmt.Errorf("Stability AI returned no image data")
	}

	imgData, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode Stability image: %w", err)
	}
	return imgData, "png", nil
}

// sizeToAspectRatio converts a size string like "1024x1024" to an aspect ratio like "1:1".
func sizeToAspectRatio(size string) string {
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return ""
	}
	// Common mappings
	switch size {
	case "1024x1024":
		return "1:1"
	case "1152x896", "1216x832":
		return "4:3"
	case "1344x768", "1536x640":
		return "16:9"
	case "768x1344", "640x1536":
		return "9:16"
	case "896x1152", "832x1216":
		return "3:4"
	default:
		return "1:1"
	}
}
