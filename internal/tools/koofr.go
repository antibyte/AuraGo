package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
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

type koofrAPIError struct {
	statusCode int
	body       string
}

func (e *koofrAPIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("API returns status %d: %s", e.statusCode, e.body)
}

var koofrHTTPClient = security.NewSSRFProtectedHTTPClient(30 * time.Second)

// ExecuteKoofr performs operations on the Koofr API.
// Valid actions: list, read, download, write, upload, mkdir, delete, rename, copy.
func ExecuteKoofr(cfg KoofrConfig, action, path, dest, content, localPath, workspaceDir string) string {
	if cfg.Username == "" || cfg.AppPassword == "" {
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": "Koofr credentials are not configured"})
	}
	action = strings.TrimSpace(strings.ToLower(action))

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://app.koofr.net"
	}

	mountID, err := getPrimaryMountID(baseURL, cfg.Username, cfg.AppPassword)
	if err != nil {
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Failed to get Koofr mount ID: %v", err)})
	}

	safePath := normalizeKoofrPath(path)
	safeDest := normalizeKoofrPath(dest)

	var respBytes []byte

	switch action {
	case "list":
		respBytes, err = doKoofrRequest("GET", koofrFilesURL(baseURL, mountID, "list", safePath), cfg.Username, cfg.AppPassword, "application/json", nil)
		if isKoofrStatus(err, http.StatusNotFound) {
			for _, fallbackPath := range koofrPathFallbacks(safePath) {
				respBytes, err = doKoofrRequest("GET", koofrFilesURL(baseURL, mountID, "list", fallbackPath), cfg.Username, cfg.AppPassword, "application/json", nil)
				if err == nil || !isKoofrStatus(err, http.StatusNotFound) {
					break
				}
			}
		}

	case "read":
		reqURL := fmt.Sprintf("%s/content/api/v2/mounts/%s/files/get?path=%s", baseURL, mountID, url.QueryEscape(safePath))
		respBytes, err = doKoofrRequest("GET", reqURL, cfg.Username, cfg.AppPassword, "", nil)
		if err == nil {
			if contentType, isText := classifyKoofrContent(respBytes); !isText {
				return marshalPrefixedToolJSON(map[string]interface{}{
					"status":       "error",
					"message":      "Koofr read only supports text files. This file appears to be binary. Use operation 'download' with a destination inside the workspace (for example 'workdir/song.mp3').",
					"content_type": contentType,
					"bytes":        len(respBytes),
				})
			}
			return koofrSuccessContentResponse(respBytes)
		}

	case "download":
		if strings.TrimSpace(dest) == "" {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": "Destination path required for download"})
		}
		reqURL := fmt.Sprintf("%s/content/api/v2/mounts/%s/files/get?path=%s", baseURL, mountID, url.QueryEscape(safePath))
		respBytes, err = doKoofrRequest("GET", reqURL, cfg.Username, cfg.AppPassword, "", nil)
		if err == nil {
			resolvedDest, err := resolveKoofrDownloadDestination(workspaceDir, dest)
			if err != nil {
				return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Invalid download destination: %v", err)})
			}
			if err := os.MkdirAll(filepath.Dir(resolvedDest), 0o755); err != nil {
				return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Failed to create destination directory: %v", err)})
			}
			if err := os.WriteFile(resolvedDest, respBytes, 0o644); err != nil {
				return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Failed to write downloaded file: %v", err)})
			}
			contentType, _ := classifyKoofrContent(respBytes)
			return marshalPrefixedToolJSON(map[string]interface{}{
				"status":       "success",
				"message":      fmt.Sprintf("Downloaded %d bytes from %s", len(respBytes), safePath),
				"path":         resolvedDest,
				"content_type": contentType,
			})
		}

	case "write":
		if content == "" {
			return marshalPrefixedToolJSON(map[string]interface{}{
				"status":  "error",
				"message": "Koofr write requires non-empty content. To upload an existing local file such as a generated image, use operation 'upload' with local_path, path as the Koofr target directory, and destination as the remote filename.",
			})
		}
		reqURL := fmt.Sprintf("%s/content/api/v2/mounts/%s/files/put?path=%s", baseURL, mountID, url.QueryEscape(safePath))
		filename := koofrUploadFilename(dest, "file.txt")
		_, written, uploadErr := uploadKoofrMultipart(reqURL, cfg.Username, cfg.AppPassword, filename, strings.NewReader(content))
		if uploadErr != nil {
			err = uploadErr
			break
		}
		return marshalPrefixedToolJSON(map[string]interface{}{"status": "success", "message": "File written successfully", "bytes": written})

	case "upload":
		source, size, err := openKoofrUploadSource(workspaceDir, localPath)
		if err != nil {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": err.Error()})
		}
		defer source.Close()
		uploadDir, filename := resolveKoofrUploadTarget(safePath, dest, filepath.Base(localPath))
		reqURL := fmt.Sprintf("%s/content/api/v2/mounts/%s/files/put?path=%s", baseURL, mountID, url.QueryEscape(uploadDir))
		_, written, uploadErr := uploadKoofrMultipart(reqURL, cfg.Username, cfg.AppPassword, filename, source)
		if uploadErr != nil {
			err = uploadErr
			break
		}
		listBytes, verifyErr := doKoofrRequest("GET", koofrFilesURL(baseURL, mountID, "list", uploadDir), cfg.Username, cfg.AppPassword, "application/json", nil)
		if verifyErr != nil {
			return marshalPrefixedToolJSON(map[string]interface{}{
				"status":           "error",
				"message":          "File upload was accepted by Koofr, but AuraGo could not verify the file in the target directory",
				"details":          fmt.Sprintf("%v", verifyErr),
				"bytes":            written,
				"expected_bytes":   size,
				"remote_directory": uploadDir,
				"filename":         filename,
			})
		}
		if !koofrListContainsFilename(listBytes, filename) {
			return marshalPrefixedToolJSON(map[string]interface{}{
				"status":           "error",
				"message":          "File upload was accepted by Koofr, but the uploaded file is not visible in the target directory",
				"bytes":            written,
				"expected_bytes":   size,
				"remote_directory": uploadDir,
				"filename":         filename,
			})
		}
		return marshalPrefixedToolJSON(map[string]interface{}{
			"status":           "success",
			"message":          "File uploaded successfully",
			"bytes":            written,
			"expected_bytes":   size,
			"remote_directory": uploadDir,
			"filename":         filename,
		})

	case "mkdir":
		clean := strings.TrimRight(safePath, "/")
		if clean == "" {
			clean = "/"
		}
		parentDir := pathpkg.Dir(clean)
		if parentDir == "" || parentDir == clean {
			parentDir = "/"
		}
		newName := pathpkg.Base(clean)

		reqURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/folder?path=%s", baseURL, mountID, url.QueryEscape(parentDir))

		payload := map[string]string{"name": newName}
		payloadBytes, _ := json.Marshal(payload)

		respBytes, err = doKoofrRequest("POST", reqURL, cfg.Username, cfg.AppPassword, "application/json", bytes.NewReader(payloadBytes))
		if isKoofrStatus(err, http.StatusNotFound) {
			for _, fallbackParent := range koofrPathFallbacks(parentDir) {
				reqURL = fmt.Sprintf("%s/api/v2/mounts/%s/files/folder?path=%s", baseURL, mountID, url.QueryEscape(fallbackParent))
				respBytes, err = doKoofrRequest("POST", reqURL, cfg.Username, cfg.AppPassword, "application/json", bytes.NewReader(payloadBytes))
				if err == nil || !isKoofrStatus(err, http.StatusNotFound) {
					break
				}
			}
		}

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
		if koofrDestinationLooksLocal(dest) {
			return marshalPrefixedToolJSON(map[string]interface{}{
				"status":  "error",
				"message": "Koofr copy only copies within Koofr storage. To download into the agent workspace, use operation 'download' with destination like 'workdir/song.mp3'.",
			})
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

func koofrFilesURL(baseURL, mountID, operation, safePath string) string {
	return fmt.Sprintf("%s/api/v2/mounts/%s/files/%s?path=%s", baseURL, mountID, operation, url.QueryEscape(safePath))
}

func koofrPathFallbacks(safePath string) []string {
	if safePath == "" || safePath == "/" {
		return nil
	}
	var fallbacks []string
	if !strings.HasSuffix(safePath, "/") {
		fallbacks = append(fallbacks, safePath+"/")
	}
	if strings.HasPrefix(safePath, "/") {
		fallbacks = append(fallbacks, strings.TrimPrefix(safePath, "/"))
	}
	return fallbacks
}

func isKoofrStatus(err error, status int) bool {
	apiErr, ok := err.(*koofrAPIError)
	return ok && apiErr.statusCode == status
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

func uploadKoofrMultipart(reqURL, username, password, filename string, r io.Reader) ([]byte, int64, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("content", filename)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create multipart writer: %w", err)
	}
	written, err := io.Copy(part, r)
	if err != nil {
		return nil, written, fmt.Errorf("failed to stream upload content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, written, fmt.Errorf("failed to close multipart writer: %w", err)
	}
	respBytes, err := doKoofrRequest("POST", reqURL, username, password, writer.FormDataContentType(), body)
	return respBytes, written, err
}

func openKoofrUploadSource(workspaceDir, localPath string) (*os.File, int64, error) {
	if strings.TrimSpace(localPath) == "" {
		return nil, 0, fmt.Errorf("local_path is required for Koofr upload")
	}
	resolved, err := secureResolve(workspaceDir, localPath)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid upload source: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to stat upload source: %w", err)
	}
	if info.IsDir() {
		return nil, 0, fmt.Errorf("upload source is a directory")
	}
	if info.Size() == 0 {
		return nil, 0, fmt.Errorf("upload source is empty; refusing to create a 0-byte Koofr file")
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open upload source: %w", err)
	}
	return file, info.Size(), nil
}

func koofrUploadFilename(dest, fallback string) string {
	filename := strings.TrimSpace(dest)
	if filename == "" {
		filename = fallback
	}
	filename = filepath.Base(filepath.FromSlash(filename))
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		return "file.bin"
	}
	return filename
}

func resolveKoofrUploadTarget(safePath, dest, fallbackFilename string) (string, string) {
	filename := koofrUploadFilename(dest, fallbackFilename)
	uploadDir := safePath
	if strings.TrimSpace(dest) == "" && !strings.HasSuffix(safePath, "/") {
		base := pathpkg.Base(safePath)
		if base != "." && base != "/" && pathpkg.Ext(base) != "" {
			filename = koofrUploadFilename(base, fallbackFilename)
			uploadDir = pathpkg.Dir(safePath)
			if uploadDir == "." || uploadDir == "" {
				uploadDir = "/"
			}
		}
	}
	return uploadDir, filename
}

func koofrListContainsFilename(respBytes []byte, filename string) bool {
	type koofrListedFile struct {
		Name string `json:"name"`
	}
	var wrapped struct {
		Files []koofrListedFile `json:"files"`
	}
	if err := json.Unmarshal(respBytes, &wrapped); err == nil {
		for _, file := range wrapped.Files {
			if file.Name == filename {
				return true
			}
		}
	}

	var files []koofrListedFile
	if err := json.Unmarshal(respBytes, &files); err != nil {
		return false
	}
	for _, file := range files {
		if file.Name == filename {
			return true
		}
	}
	return false
}

func normalizeKoofrPath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	if trimmed == "//" {
		return "/"
	}
	return trimmed
}

func classifyKoofrContent(content []byte) (string, bool) {
	if len(content) == 0 {
		return "application/octet-stream", true
	}
	sample := content
	if len(sample) > 512 {
		sample = sample[:512]
	}
	contentType := http.DetectContentType(sample)
	if strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "javascript") {
		return contentType, true
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return contentType, false
	}
	return contentType, utf8.Valid(sample)
}

func koofrDestinationLooksLocal(dest string) bool {
	slash := filepath.ToSlash(strings.TrimSpace(dest))
	return slash == "workdir" ||
		slash == "/workdir" ||
		strings.HasPrefix(slash, "workdir/") ||
		strings.HasPrefix(slash, "/workdir/") ||
		strings.HasPrefix(slash, "agent_workspace/workdir/") ||
		strings.HasPrefix(slash, "/agent_workspace/workdir/")
}

func resolveKoofrDownloadDestination(workspaceDir, dest string) (string, error) {
	if strings.TrimSpace(workspaceDir) == "" {
		return "", fmt.Errorf("workspace_dir is not configured")
	}
	normalized := filepath.ToSlash(strings.TrimSpace(dest))
	switch {
	case normalized == "workdir" || normalized == "/workdir":
		normalized = "."
	case strings.HasPrefix(normalized, "workdir/"):
		normalized = strings.TrimPrefix(normalized, "workdir/")
	case strings.HasPrefix(normalized, "/workdir/"):
		normalized = strings.TrimPrefix(normalized, "/workdir/")
	}
	resolved, err := secureResolve(workspaceDir, filepath.FromSlash(normalized))
	if err != nil {
		return "", err
	}
	return resolved, nil
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
		return nil, &koofrAPIError{statusCode: resp.StatusCode, body: string(respBytes)}
	}

	return respBytes, nil
}
