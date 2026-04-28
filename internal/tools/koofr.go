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
		filename := koofrUploadFilename(dest, "file.txt")
		reqURL := koofrUploadURL(baseURL, mountID, safePath, filename)
		_, written, uploadErr := uploadKoofrMultipart(reqURL, cfg.Username, cfg.AppPassword, strings.NewReader(content))
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
		if err := ensureKoofrDirectory(baseURL, mountID, uploadDir, cfg.Username, cfg.AppPassword); err != nil {
			return marshalPrefixedToolJSON(map[string]interface{}{"status": "error", "message": fmt.Sprintf("Failed to prepare Koofr target directory: %v", err), "remote_directory": uploadDir})
		}
		reqURL := koofrUploadURL(baseURL, mountID, uploadDir, filename)
		_, written, uploadErr := uploadKoofrMultipart(reqURL, cfg.Username, cfg.AppPassword, source)
		if uploadErr != nil {
			err = uploadErr
			break
		}
		visible, verifyErr := verifyKoofrUploadVisible(baseURL, mountID, uploadDir, filename, cfg.Username, cfg.AppPassword)
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
		if !visible {
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

	if koofrActionRequiresExplicitResult(action) {
		return marshalPrefixedToolJSON(map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Koofr %s reached an unexpected empty success path; no verified result was produced", action),
		})
	}
	return marshalPrefixedToolJSON(map[string]interface{}{"status": "success"})
}

func koofrFilesURL(baseURL, mountID, operation, safePath string) string {
	return fmt.Sprintf("%s/api/v2/mounts/%s/files/%s?path=%s", baseURL, mountID, operation, url.QueryEscape(safePath))
}

func koofrActionRequiresExplicitResult(action string) bool {
	switch action {
	case "write", "upload", "mkdir", "delete", "rename", "move", "copy":
		return true
	default:
		return false
	}
}

func koofrUploadURL(baseURL, mountID, safePath, filename string) string {
	q := url.Values{}
	q.Set("path", safePath)
	q.Set("filename", filename)
	q.Set("info", "true")
	q.Set("overwrite", "true")
	q.Set("autorename", "false")
	q.Set("overwriteIgnoreNonexisting", "")
	return fmt.Sprintf("%s/content/api/v2/mounts/%s/files/put?%s", baseURL, mountID, q.Encode())
}

func koofrJoinPath(dir, filename string) string {
	cleanDir := strings.TrimRight(dir, "/")
	if cleanDir == "" {
		cleanDir = "/"
	}
	if cleanDir == "/" {
		return "/" + strings.TrimLeft(filename, "/")
	}
	return cleanDir + "/" + strings.TrimLeft(filename, "/")
}

func koofrDirectoryPrefixes(dir string) []string {
	clean := strings.Trim(strings.TrimSpace(dir), "/")
	if clean == "" {
		return nil
	}
	parts := strings.Split(clean, "/")
	prefixes := make([]string, 0, len(parts))
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current += "/" + part
		prefixes = append(prefixes, current)
	}
	return prefixes
}

func ensureKoofrDirectory(baseURL, mountID, dir, username, password string) error {
	for _, current := range koofrDirectoryPrefixes(dir) {
		_, err := doKoofrRequest("GET", koofrFilesURL(baseURL, mountID, "list", current), username, password, "application/json", nil)
		if err == nil {
			continue
		}
		if !isKoofrStatus(err, http.StatusNotFound) {
			return err
		}

		parentDir := pathpkg.Dir(current)
		if parentDir == "." || parentDir == "" {
			parentDir = "/"
		}
		payloadBytes, _ := json.Marshal(map[string]string{"name": pathpkg.Base(current)})
		reqURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/folder?path=%s", baseURL, mountID, url.QueryEscape(parentDir))
		_, err = doKoofrRequest("POST", reqURL, username, password, "application/json", bytes.NewReader(payloadBytes))
		if err != nil && !isKoofrStatus(err, http.StatusConflict) {
			return err
		}
	}
	return nil
}

func verifyKoofrUploadVisible(baseURL, mountID, uploadDir, filename, username, password string) (bool, error) {
	filePath := koofrJoinPath(uploadDir, filename)
	infoURL := fmt.Sprintf("%s/api/v2/mounts/%s/files/info?path=%s", baseURL, mountID, url.QueryEscape(filePath))
	if _, err := doKoofrRequest("GET", infoURL, username, password, "application/json", nil); err == nil {
		return true, nil
	} else if !isKoofrStatus(err, http.StatusNotFound) {
		return false, err
	}

	listBytes, err := doKoofrRequest("GET", koofrFilesURL(baseURL, mountID, "list", uploadDir), username, password, "application/json", nil)
	if err != nil {
		return false, err
	}
	return koofrListContainsFilename(listBytes, filename), nil
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

func uploadKoofrMultipart(reqURL, username, password string, r io.Reader) ([]byte, int64, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "dummy")
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
