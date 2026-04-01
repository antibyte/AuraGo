package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// generateIdeogram generates an image using Ideogram's REST API (V_2 model).
func generateIdeogram(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.ideogram.ai"
	}
	url := strings.TrimRight(baseURL, "/") + "/generate"

	model := cfg.Model
	if model == "" {
		model = "V_2"
	}

	imageReq := map[string]interface{}{
		"prompt": prompt,
		"model":  model,
	}
	if opts.Size != "" {
		ar := sizeToIdeogramAspect(opts.Size)
		if ar != "" {
			imageReq["aspect_ratio"] = ar
		}
	}
	if opts.Style != "" {
		imageReq["style_type"] = strings.ToUpper(opts.Style)
	}

	body := map[string]interface{}{
		"image_request": imageReq,
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
	req.Header.Set("Api-Key", cfg.APIKey)

	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Ideogram request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Ideogram returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	var result struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse Ideogram response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, "", fmt.Errorf("Ideogram returned no images")
	}

	// Download the image from the returned URL
	imgData, err := downloadImage(result.Data[0].URL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download Ideogram image: %w", err)
	}
	return imgData, detectFormat(imgData), nil
}

// sizeToIdeogramAspect converts a size string to Ideogram's aspect ratio format.
func sizeToIdeogramAspect(size string) string {
	switch size {
	case "1024x1024":
		return "ASPECT_1_1"
	case "1344x768", "1536x640":
		return "ASPECT_16_9"
	case "768x1344", "640x1536":
		return "ASPECT_9_16"
	case "1152x896", "1216x832":
		return "ASPECT_4_3"
	case "896x1152", "832x1216":
		return "ASPECT_3_4"
	default:
		return "ASPECT_1_1"
	}
}
