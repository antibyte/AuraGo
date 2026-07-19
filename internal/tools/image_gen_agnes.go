package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const defaultAgnesImageModel = "agnes-image-2.1-flash"

// generateAgnesImage calls Agnes AI's OpenAI-compatible image generation API.
// Agnes expects return_base64 for text-to-image and the response format inside
// extra_body for image-to-image requests.
func generateAgnesImage(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	endpoint := strings.TrimRight(cfg.BaseURL, "/")
	if endpoint == "" {
		endpoint = "https://apihub.agnes-ai.com/v1"
	}
	if !strings.HasSuffix(endpoint, "/images/generations") {
		endpoint += "/images/generations"
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultAgnesImageModel
	}
	body := map[string]interface{}{
		"model":  model,
		"prompt": prompt,
	}
	if opts.Size != "" {
		body["size"] = opts.Size
	} else {
		body["size"] = "1K"
	}

	if opts.SourceImage == "" {
		body["return_base64"] = true
	} else {
		srcData, err := loadSourceImage(opts.SourceImage)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load Agnes AI source image: %w", err)
		}
		mimeType := http.DetectContentType(srcData)
		body["extra_body"] = map[string]interface{}{
			"image": []string{
				"data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(srcData),
			},
			"response_format": "b64_json",
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal Agnes AI image request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create Agnes AI image request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Agnes AI image request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read Agnes AI image response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("Agnes AI returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse Agnes AI image response: %w", err)
	}
	imgData, format, err := extractImageFromAnyResponse(result)
	if err != nil {
		return nil, "", fmt.Errorf("Agnes AI returned no image data: %w", err)
	}
	return imgData, format, nil
}
