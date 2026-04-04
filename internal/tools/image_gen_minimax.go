package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func generateMiniMax(cfg ImageGenConfig, prompt string, opts ImageGenOptions) ([]byte, string, error) {
	model := cfg.Model
	if model == "" {
		model = "image-01"
	}

	url := strings.TrimRight(cfg.BaseURL, "/")
	if url == "" {
		url = "https://api.minimax.io/v1/image_generation"
	}

	jsonBody, err := json.Marshal(map[string]interface{}{
		"model":           model,
		"prompt":          prompt,
		"aspect_ratio":    sizeToMiniMaxAspect(opts.Size),
		"response_format": "base64",
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal MiniMax request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, "", fmt.Errorf("failed to create MiniMax request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("MiniMax image request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read MiniMax response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("MiniMax returned status %d: %s", resp.StatusCode, truncateError(string(respBody)))
	}

	var result struct {
		Data struct {
			ImageBase64 []string `json:"image_base64"`
		} `json:"data"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, "", fmt.Errorf("failed to parse MiniMax response: %w", err)
	}
	if result.BaseResp.StatusCode != 0 {
		return nil, "", fmt.Errorf("MiniMax API error: %s (code %d)", result.BaseResp.StatusMsg, result.BaseResp.StatusCode)
	}
	if len(result.Data.ImageBase64) == 0 {
		return nil, "", fmt.Errorf("MiniMax returned no images")
	}

	imgData, err := base64.StdEncoding.DecodeString(result.Data.ImageBase64[0])
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode MiniMax image: %w", err)
	}

	return imgData, "jpeg", nil
}

func sizeToMiniMaxAspect(size string) string {
	switch size {
	case "1024x1024":
		return "1:1"
	case "1344x768", "1536x640", "1792x1024":
		return "16:9"
	case "768x1344", "640x1536", "1024x1792":
		return "9:16"
	case "1152x896", "1216x832":
		return "4:3"
	case "896x1152", "832x1216":
		return "3:4"
	default:
		return "1:1"
	}
}
