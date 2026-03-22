package memory

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MultimodalEmbedder computes embedding vectors for non-text files (images, audio)
// by calling a multimodal-capable embedding API. Supports OpenAI-compatible and
// Google Vertex AI formats.
type MultimodalEmbedder struct {
	baseURL string
	apiKey  string
	model   string
	format  string // "openai", "vertex", "auto"
	logger  *slog.Logger
	client  *http.Client
}

// NewMultimodalEmbedder creates a new MultimodalEmbedder.
// format should be "openai", "vertex", or "auto" (auto-detect from provider type).
func NewMultimodalEmbedder(baseURL, apiKey, model, format, providerType string, logger *slog.Logger) *MultimodalEmbedder {
	resolvedFormat := format
	if resolvedFormat == "" || resolvedFormat == "auto" {
		resolvedFormat = detectFormat(providerType, baseURL)
	}
	return &MultimodalEmbedder{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		format:  resolvedFormat,
		logger:  logger,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// detectFormat guesses the API format from provider type and base URL.
func detectFormat(providerType, baseURL string) string {
	pt := strings.ToLower(providerType)
	bu := strings.ToLower(baseURL)
	if strings.Contains(pt, "google") || strings.Contains(pt, "vertex") ||
		strings.Contains(bu, "googleapis.com") || strings.Contains(bu, "generativelanguage") {
		return "vertex"
	}
	return "openai"
}

// EmbedFile reads a file (image or audio), base64-encodes it, and calls the
// multimodal embedding API. Returns the embedding vector.
func (me *MultimodalEmbedder) EmbedFile(ctx context.Context, filePath string) ([]float32, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filepath.Base(filePath), err)
	}

	mimeType := detectMIME(filePath)
	b64 := base64.StdEncoding.EncodeToString(data)

	switch me.format {
	case "vertex":
		return me.embedVertex(ctx, mimeType, b64)
	default:
		return me.embedOpenAI(ctx, mimeType, b64)
	}
}

// embedOpenAI sends a multimodal embedding request using the OpenAI-compatible format.
// POST {baseURL}/embeddings with input as an array containing an image_url or inline_data object.
func (me *MultimodalEmbedder) embedOpenAI(ctx context.Context, mimeType, b64 string) ([]float32, error) {
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, b64)

	payload := map[string]interface{}{
		"model": me.model,
		"input": map[string]interface{}{
			"type":      "image_url",
			"image_url": map[string]string{"url": dataURI},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", me.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+me.apiKey)

	return me.doRequest(req)
}

// embedVertex sends a multimodal embedding request using the Google Vertex AI format.
// POST {baseURL} with instances containing inline_data.
func (me *MultimodalEmbedder) embedVertex(ctx context.Context, mimeType, b64 string) ([]float32, error) {
	payload := map[string]interface{}{
		"instances": []map[string]interface{}{
			{
				"content": map[string]interface{}{
					"parts": []map[string]interface{}{
						{
							"inline_data": map[string]string{
								"mime_type": mimeType,
								"data":      b64,
							},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := me.baseURL
	if !strings.HasSuffix(endpoint, ":predict") {
		// Append :predict to the URL path, not the host.
		// Parse and rebuild to avoid breaking host:port.
		u, parseErr := url.Parse(endpoint)
		if parseErr == nil {
			u.Path = strings.TrimRight(u.Path, "/") + ":predict"
			endpoint = u.String()
		} else {
			endpoint = strings.TrimRight(endpoint, "/") + ":predict"
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if me.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+me.apiKey)
	}

	return me.doVertexRequest(req)
}

// doRequest performs an HTTP request and parses an OpenAI-compatible embedding response.
func (me *MultimodalEmbedder) doRequest(req *http.Request) ([]float32, error) {
	resp, err := me.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, truncateResponse(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	return normalizeEmbedding(result.Data[0].Embedding), nil
}

// doVertexRequest performs an HTTP request and parses a Google Vertex embedding response.
func (me *MultimodalEmbedder) doVertexRequest(req *http.Request) ([]float32, error) {
	resp, err := me.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, truncateResponse(respBody))
	}

	var result struct {
		Predictions []struct {
			Embeddings struct {
				Values []float32 `json:"values"`
			} `json:"embeddings"`
		} `json:"predictions"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(result.Predictions) == 0 || len(result.Predictions[0].Embeddings.Values) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	return normalizeEmbedding(result.Predictions[0].Embeddings.Values), nil
}

// normalizeEmbedding normalizes a vector to unit length (L2 norm).
func normalizeEmbedding(v []float32) []float32 {
	var sum float64
	for _, val := range v {
		sum += float64(val) * float64(val)
	}
	norm := math.Sqrt(sum)
	if norm == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, val := range v {
		out[i] = float32(float64(val) / norm)
	}
	return out
}

// detectMIME returns a MIME type based on file extension.
func detectMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	case ".svg":
		return "image/svg+xml"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	case ".m4a":
		return "audio/mp4"
	case ".wma":
		return "audio/x-ms-wma"
	case ".pdf":
		return "application/pdf"
	default:
		return "application/octet-stream"
	}
}

// truncateResponse truncates a response body for error messages.
func truncateResponse(body []byte) string {
	const maxLen = 300
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
