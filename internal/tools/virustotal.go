package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ExecuteVirusTotalScan performs a search using the VirusTotal API (v3)
func ExecuteVirusTotalScan(apiKey string, resource string) string {
	if apiKey == "" {
		return formatError("VirusTotal API Key is missing. Please configure it in settings.")
	}

	if resource == "" {
		return formatError("Resource to scan is required")
	}

	client := &http.Client{Timeout: 15 * time.Second}

	endpoint := fmt.Sprintf("https://www.virustotal.com/api/v3/search?query=%s", url.QueryEscape(resource))
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return formatError(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.Header.Set("x-apikey", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return formatError(fmt.Sprintf("VirusTotal request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return formatError(fmt.Sprintf("VirusTotal HTTP Error %d", resp.StatusCode))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return formatError(fmt.Sprintf("Failed to read VirusTotal response: %v", err))
	}

	// Unmarshal and re-marshal to ensure pretty formatting and valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return formatError("Failed to parse VirusTotal response JSON")
	}

	// Wrap in a standard tool execution format
	resultMap := map[string]interface{}{
		"status": "success",
		"result": result,
	}

	b, _ := json.MarshalIndent(resultMap, "", "  ")
	return string(b)
}
