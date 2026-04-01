package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// generateGoogleImagen generates an image using Google's Generative Language API (Imagen 3).
func generateGoogleImagen(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	model := cfg.Model
	if model == "" {
		model = "imagen-3.0-generate-002"
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	url := fmt.Sprintf("%s/models/%s:predict?key=%s",
		strings.TrimRight(baseURL, "/"), model, cfg.APIKey)

	body := map[string]interface{}{
		"instances": []map[string]interface{}{
			{"prompt": prompt},
		},
		"parameters": map[string]interface{}{
			"sampleCount": 1,
		},
	}

	// Add aspect ratio if size specified
	if opts.Size != "" {
		ar := sizeToGoogleAspect(opts.Size)
		if ar != "" {
			body["parameters"].(map[string]interface{})["aspectRatio"] = ar
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

	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Google Imagen request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("Google Imagen returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	var result struct {
		Predictions []struct {
			BytesBase64Encoded string `json:"bytesBase64Encoded"`
			MimeType           string `json:"mimeType"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse Google Imagen response: %w", err)
	}
	if len(result.Predictions) == 0 {
		return nil, "", fmt.Errorf("Google Imagen returned no predictions")
	}

	imgData, err := base64.StdEncoding.DecodeString(result.Predictions[0].BytesBase64Encoded)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode Google Imagen image: %w", err)
	}

	format := "png"
	if result.Predictions[0].MimeType != "" {
		format = strings.TrimPrefix(result.Predictions[0].MimeType, "image/")
	}
	return imgData, format, nil
}

// sizeToGoogleAspect converts a size string to Google Imagen aspect ratio format.
func sizeToGoogleAspect(size string) string {
	switch size {
	case "1024x1024":
		return "1:1"
	case "1344x768", "1536x640":
		return "16:9"
	case "768x1344", "640x1536":
		return "9:16"
	case "1152x896", "1216x832":
		return "4:3"
	case "896x1152", "832x1216":
		return "3:4"
	default:
		return "1:1"
	}
}
