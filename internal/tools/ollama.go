package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaConfig holds the Ollama API connection parameters.
type OllamaConfig struct {
	URL string // e.g. "http://localhost:11434"
}

// ollamaHTTPClient is a shared HTTP client for Ollama API calls.
var ollamaHTTPClient = &http.Client{Timeout: 120 * time.Second}

// ollamaRequest performs a generic HTTP request against the Ollama REST API.
func ollamaRequest(cfg OllamaConfig, method, endpoint string, body string) ([]byte, int, error) {
	url := strings.TrimRight(cfg.URL, "/") + endpoint

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ollamaHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// OllamaListModels returns all locally available models.
func OllamaListModels(cfg OllamaConfig) string {
	data, code, err := ollamaRequest(cfg, "GET", "/api/tags", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to list models: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
			Digest     string `json:"digest"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Sprintf(`{"status":"ok","raw":%s}`, string(data))
	}

	type modelSummary struct {
		Name       string `json:"name"`
		SizeGB     string `json:"size_gb"`
		ModifiedAt string `json:"modified_at"`
	}
	summaries := make([]modelSummary, 0, len(result.Models))
	for _, m := range result.Models {
		summaries = append(summaries, modelSummary{
			Name:       m.Name,
			SizeGB:     fmt.Sprintf("%.2f", float64(m.Size)/(1024*1024*1024)),
			ModifiedAt: m.ModifiedAt,
		})
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(summaries),
		"models": summaries,
	})
	return string(out)
}

// OllamaListRunning returns currently loaded/running models.
func OllamaListRunning(cfg OllamaConfig) string {
	data, code, err := ollamaRequest(cfg, "GET", "/api/ps", "")
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to list running models: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return fmt.Sprintf(`{"status":"ok","data":%s}`, string(data))
}

// OllamaShowModel returns metadata/details about a specific model.
func OllamaShowModel(cfg OllamaConfig, modelName string) string {
	if modelName == "" {
		return `{"status":"error","message":"model name is required."}`
	}
	body := fmt.Sprintf(`{"name":%q}`, modelName)
	data, code, err := ollamaRequest(cfg, "POST", "/api/show", body)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to show model: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	// The response can be very large (template, modelfile, etc). Trim to essentials.
	var info struct {
		ModelInfo map[string]interface{} `json:"model_info"`
		Details   interface{}            `json:"details"`
		License   string                 `json:"license"`
	}
	if json.Unmarshal(data, &info) == nil {
		out, _ := json.Marshal(map[string]interface{}{
			"status":     "ok",
			"model":      modelName,
			"details":    info.Details,
			"model_info": info.ModelInfo,
		})
		return string(out)
	}
	return fmt.Sprintf(`{"status":"ok","data":%s}`, string(data))
}

// OllamaPullModel pulls (downloads) a model. This is synchronous and may take a long time.
func OllamaPullModel(cfg OllamaConfig, modelName string) string {
	if modelName == "" {
		return `{"status":"error","message":"model name is required."}`
	}
	body := fmt.Sprintf(`{"name":%q,"stream":false}`, modelName)
	// Use a longer timeout for pull operations
	client := &http.Client{Timeout: 30 * time.Minute}
	url := strings.TrimRight(cfg.URL, "/") + "/api/pull"
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Failed to create pull request: %v"}`, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Pull failed: %v"}`, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, resp.StatusCode, string(data))
	}
	return fmt.Sprintf(`{"status":"ok","message":"Model '%s' pulled successfully.","data":%s}`, modelName, string(data))
}

// OllamaDeleteModel removes a model from local storage.
func OllamaDeleteModel(cfg OllamaConfig, modelName string) string {
	if modelName == "" {
		return `{"status":"error","message":"model name is required."}`
	}
	body := fmt.Sprintf(`{"name":%q}`, modelName)
	data, code, err := ollamaRequest(cfg, "DELETE", "/api/delete", body)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Delete failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return fmt.Sprintf(`{"status":"ok","message":"Model '%s' deleted successfully."}`, modelName)
}

// OllamaCopyModel creates a copy/alias of an existing model.
func OllamaCopyModel(cfg OllamaConfig, source, destination string) string {
	if source == "" || destination == "" {
		return `{"status":"error","message":"source and destination model names are required."}`
	}
	body := fmt.Sprintf(`{"source":%q,"destination":%q}`, source, destination)
	data, code, err := ollamaRequest(cfg, "POST", "/api/copy", body)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Copy failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return fmt.Sprintf(`{"status":"ok","message":"Model copied: %s → %s"}`, source, destination)
}

// OllamaLoadModel preloads a model into memory without generating.
func OllamaLoadModel(cfg OllamaConfig, modelName string) string {
	if modelName == "" {
		return `{"status":"error","message":"model name is required."}`
	}
	body := fmt.Sprintf(`{"model":%q}`, modelName)
	data, code, err := ollamaRequest(cfg, "POST", "/api/generate", body)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Load failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return fmt.Sprintf(`{"status":"ok","message":"Model '%s' loaded into memory."}`, modelName)
}

// OllamaUnloadModel unloads a model from memory by setting keep_alive to 0.
func OllamaUnloadModel(cfg OllamaConfig, modelName string) string {
	if modelName == "" {
		return `{"status":"error","message":"model name is required."}`
	}
	body := fmt.Sprintf(`{"model":%q,"keep_alive":0}`, modelName)
	data, code, err := ollamaRequest(cfg, "POST", "/api/generate", body)
	if err != nil {
		return fmt.Sprintf(`{"status":"error","message":"Unload failed: %v"}`, err)
	}
	if code != 200 {
		return fmt.Sprintf(`{"status":"error","http_code":%d,"message":%q}`, code, string(data))
	}
	return fmt.Sprintf(`{"status":"ok","message":"Model '%s' unloaded from memory."}`, modelName)
}
