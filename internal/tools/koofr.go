package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// KoofrConfig holds the configuration needed to access Koofr API.
type KoofrConfig struct {
	BaseURL     string
	Username    string
	AppPassword string
}

type mountInfo struct {
	ID        string `json:"id"`
	IsPrimary bool   `json:"isPrimary"`
}

var koofrHTTPClient = &http.Client{Timeout: 30 * time.Second}

// ExecuteKoofr performs operations on the Koofr API.
// Valid actions: list, read, write, mkdir, delete, rename, copy.
func ExecuteKoofr(cfg KoofrConfig, action, path, dest, content string) string {
	if cfg.Username == "" || cfg.AppPassword == "" {
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": "Koofr credentials are not configured"})
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://app.koofr.net"
	}

	mountID, err := getPrimaryMountID(baseURL, cfg.Username, cfg.AppPassword)
	if err != nil {
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Failed to get Koofr mount ID: %v", err)})
	}

	safePath := path
	if !strings.HasPrefix(safePath, "/") {
		safePath = "/" + safePath
	}
	if safePath == "//" {
		safePath = "/"
	}

	safeDest := dest
	if dest != "" && !strings.HasPrefix(safeDest, "/") {
		safeDest = "/" + safeDest
	}

	var respBytes []byte

	switch action {
	case "list":
		reqURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/list?path=%s", baseURL, mountID, url.QueryEscape(safePath))
		respBytes, err = doKoofrRequest("GET", reqURL, cfg.Username, cfg.AppPassword, "application/json", nil)

	case "read":
		reqURL := fmt.Sprintf("%s/content/api/v2/mounts/%s/files/get?path=%s", baseURL, mountID, url.QueryEscape(safePath))
		respBytes, err = doKoofrRequest("GET", reqURL, cfg.Username, cfg.AppPassword, "", nil)
		if err == nil {
			return koofrSuccessContentResponse(respBytes)
		}

	case "write":
		reqURL := fmt.Sprintf("%s/content/api/v2/mounts/%s/files/put?path=%s", baseURL, mountID, url.QueryEscape(safePath))

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		var filename string
		if dest != "" {
			filename = filepath.Base(dest)
		} else {
			filename = "file.txt"
		}

		part, errWriter := writer.CreateFormFile("content", filename)
		if errWriter != nil {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Failed to create multipart writer: %v", errWriter)})
		}
		part.Write([]byte(content))
		writer.Close()

		respBytes, err = doKoofrRequest("POST", reqURL, cfg.Username, cfg.AppPassword, writer.FormDataContentType(), body)
		if err == nil {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "success", "message": "File written successfully"})
		}

	case "mkdir":
		clean := strings.TrimRight(safePath, "/")
		if clean == "" {
			clean = "/"
		}
		parentDir := filepath.Dir(clean) // On linux/slash paths, this gives string
		// fix path separators for Windows environments affecting parentDir logic
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		if parentDir == "" || parentDir == clean {
			parentDir = "/"
		}
		newName := filepath.Base(clean)

		reqURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/folder?path=%s", baseURL, mountID, url.QueryEscape(parentDir))

		payload := map[string]string{"name": newName}
		payloadBytes, _ := json.Marshal(payload)

		respBytes, err = doKoofrRequest("POST", reqURL, cfg.Username, cfg.AppPassword, "application/json", bytes.NewReader(payloadBytes))

	case "delete":
		reqURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/remove?path=%s", baseURL, mountID, url.QueryEscape(safePath))
		respBytes, err = doKoofrRequest("DELETE", reqURL, cfg.Username, cfg.AppPassword, "application/json", nil)
		if err == nil {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "success", "message": "Deleted successfully"})
		}

	case "rename", "move":
		if safeDest == "" {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": "Destination path required for rename/move"})
		}
		reqURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/move?path=%s", baseURL, mountID, url.QueryEscape(safePath))

		payload := map[string]string{"to": safeDest}
		payloadBytes, _ := json.Marshal(payload)

		respBytes, err = doKoofrRequest("POST", reqURL, cfg.Username, cfg.AppPassword, "application/json", bytes.NewReader(payloadBytes))
		if err == nil {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "success", "message": "Moved/Renamed successfully"})
		}

	case "copy":
		if safeDest == "" {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": "Destination path required for copy"})
		}
		reqURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/copy?path=%s", baseURL, mountID, url.QueryEscape(safePath))

		payload := map[string]string{"to": safeDest}
		payloadBytes, _ := json.Marshal(payload)

		respBytes, err = doKoofrRequest("POST", reqURL, cfg.Username, cfg.AppPassword, "application/json", bytes.NewReader(payloadBytes))
		if err == nil {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "success", "message": "Copied successfully"})
		}

	default:
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Unsupported Koofr action: %s", action)})
	}

	if err != nil {
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": "Koofr API request failed", "details": fmt.Sprintf("%v", err)})
	}

	if len(respBytes) > 0 {
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "success", "response": jsonRawOrString(respBytes)})
	}

	return marshalPrefixedToolJSON(map[string]interface{}{"status": "success"})
}

func koofrSuccessContentResponse(content []byte) string {
	response := map[string]string{
		"status":  "success",
		"content": string(content),
	}
	data, err := json.Marshal(response)
	if err != nil {
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": "Failed to encode Koofr response"})
	}
	return "Tool Output: " + string(data)
}

func getPrimaryMountID(baseURL, username, password string) (string, error) {
	reqURL := fmt.Sprintf("%s/api/v2/mounts", baseURL)
	respBytes, err := doKoofrRequest("GET", reqURL, username, password, "application/json", nil)
	if err != nil {
		return "", err
	}

	var parsed map[string]interface{}
	// Sometimes it returns { "mounts": [...] }, sometimes directly a list if we're not careful. The API docs says it returns a list of mounts or an object with "mounts".
	if err := json.Unmarshal(respBytes, &parsed); err == nil {
		if mountsData, ok := parsed["mounts"]; ok {
			mountsBytes, _ := json.Marshal(mountsData)
			var mounts []mountInfo
			if err := json.Unmarshal(mountsBytes, &mounts); err == nil {
				return findPrimaryMount(mounts)
			}
		}
	}

	var mounts []mountInfo
	if err := json.Unmarshal(respBytes, &mounts); err != nil {
		return "", fmt.Errorf("unexpected mount response format")
	}

	return findPrimaryMount(mounts)
}

func findPrimaryMount(mounts []mountInfo) (string, error) {
	if len(mounts) == 0 {
		return "", fmt.Errorf("no mounts found")
	}
	for _, m := range mounts {
		if m.IsPrimary {
			return m.ID, nil
		}
	}
	return mounts[0].ID, nil
}

func doKoofrRequest(method, reqURL, username, password, contentType string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(username, password)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := koofrHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returns status %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}
