package tools

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebDAVConfig holds the WebDAV connection parameters.
type WebDAVConfig struct {
	AuthType string
	URL      string // Base URL, e.g. https://cloud.example.com/remote.php/dav/files/user/
	Username string
	Password string
	Token    string
}

// webdavHTTPClient is a shared HTTP client for WebDAV calls.
var webdavHTTPClient = &http.Client{Timeout: 60 * time.Second}

// ── WebDAV XML types for PROPFIND parsing ────────────────────────────

type davMultistatus struct {
	XMLName   xml.Name      `xml:"multistatus"`
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href     string      `xml:"href"`
	Propstat davPropstat `xml:"propstat"`
}

type davPropstat struct {
	Prop   davProp `xml:"prop"`
	Status string  `xml:"status"`
}

type davProp struct {
	DisplayName  string `xml:"displayname"`
	ContentLen   int64  `xml:"getcontentlength"`
	ContentType  string `xml:"getcontenttype"`
	LastModified string `xml:"getlastmodified"`
	ResourceType struct {
		Collection *struct{} `xml:"collection"`
	} `xml:"resourcetype"`
}

// ── Internal helpers ─────────────────────────────────────────────────

// webdavURL joins the base URL with a sub-path.
func webdavURL(cfg WebDAVConfig, path string) string {
	base := strings.TrimRight(cfg.URL, "/")
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return base + "/"
	}
	segments := strings.Split(trimmed, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return base + "/" + strings.Join(segments, "/")
}

// webdavRequest performs a generic WebDAV HTTP request.
func webdavRequest(cfg WebDAVConfig, method, url string, body io.Reader, extraHeaders map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.AuthType)) {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	default:
		req.SetBasicAuth(cfg.Username, cfg.Password)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	return webdavHTTPClient.Do(req)
}

// davEncode returns a JSON string from any value.
func davEncode(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ── Public operations ────────────────────────────────────────────────

// WebDAVList performs a PROPFIND on the given path and returns a directory listing.
func WebDAVList(cfg WebDAVConfig, path string) string {
	url := webdavURL(cfg, path)

	propfindBody := `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getcontenttype/>
    <d:getlastmodified/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`

	resp, err := webdavRequest(cfg, "PROPFIND", url, strings.NewReader(propfindBody), map[string]string{
		"Content-Type": "application/xml",
		"Depth":        "1",
	})
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("PROPFIND failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode != 207 {
		body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read PROPFIND error response: %v", err)})
		}
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("PROPFIND returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))})
	}

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read PROPFIND response: %v", err)})
	}
	var ms davMultistatus
	if err := xml.Unmarshal(data, &ms); err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to parse PROPFIND response: %v", err)})
	}

	type entry struct {
		Name     string `json:"name"`
		IsDir    bool   `json:"is_dir"`
		Size     int64  `json:"size"`
		Modified string `json:"modified,omitempty"`
		Type     string `json:"content_type,omitempty"`
	}
	var items []entry

	// Skip the first response (it's the directory itself)
	for i, r := range ms.Responses {
		if i == 0 {
			continue
		}
		name := r.Propstat.Prop.DisplayName
		if name == "" {
			// Extract from href
			parts := strings.Split(strings.TrimRight(r.Href, "/"), "/")
			if len(parts) > 0 {
				name = parts[len(parts)-1]
			}
		}
		isDir := r.Propstat.Prop.ResourceType.Collection != nil
		items = append(items, entry{
			Name:     name,
			IsDir:    isDir,
			Size:     r.Propstat.Prop.ContentLen,
			Modified: r.Propstat.Prop.LastModified,
			Type:     r.Propstat.Prop.ContentType,
		})
	}

	return davEncode(FSResult{Status: "success", Message: fmt.Sprintf("Listed %d entries in %s", len(items), path), Data: items})
}

// WebDAVRead downloads a file from WebDAV and returns its content.
func WebDAVRead(cfg WebDAVConfig, path string) string {
	if path == "" {
		return davEncode(FSResult{Status: "error", Message: "'path' is required for read"})
	}

	url := webdavURL(cfg, path)
	resp, err := webdavRequest(cfg, "GET", url, nil, nil)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("GET failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("File not found: %s", path)})
	}
	if resp.StatusCode != 200 {
		body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read GET error response: %v", err)})
		}
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("GET returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))})
	}

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read response body: %v", err)})
	}

	// Cap text output to avoid flooding the LLM context
	text := string(data)
	if len(text) > 8000 {
		text = text[:8000] + fmt.Sprintf("\n\n[...truncated, file has %d bytes total]", len(data))
	}

	return davEncode(FSResult{Status: "success", Message: fmt.Sprintf("Read %d bytes from %s", len(data), path), Data: text})
}

// WebDAVWrite uploads content to a file on WebDAV.
func WebDAVWrite(cfg WebDAVConfig, path, content string) string {
	if path == "" || content == "" {
		return davEncode(FSResult{Status: "error", Message: "'path' and 'content' are required for write"})
	}

	url := webdavURL(cfg, path)
	resp, err := webdavRequest(cfg, "PUT", url, strings.NewReader(content), map[string]string{
		"Content-Type": "application/octet-stream",
	})
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("PUT failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return davEncode(FSResult{Status: "success", Message: fmt.Sprintf("Wrote %d bytes to %s", len(content), path)})
	}

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read PUT error response: %v", err)})
	}
	return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("PUT returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))})
}

// WebDAVMkdir creates a directory (collection) on WebDAV.
func WebDAVMkdir(cfg WebDAVConfig, path string) string {
	if path == "" {
		return davEncode(FSResult{Status: "error", Message: "'path' is required for mkdir"})
	}

	url := webdavURL(cfg, path)
	resp, err := webdavRequest(cfg, "MKCOL", url, nil, nil)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("MKCOL failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 {
		return davEncode(FSResult{Status: "success", Message: fmt.Sprintf("Directory created: %s", path)})
	}
	if resp.StatusCode == 405 {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Directory already exists: %s", path)})
	}

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read MKCOL error response: %v", err)})
	}
	return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("MKCOL returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))})
}

// WebDAVDelete removes a file or directory from WebDAV.
func WebDAVDelete(cfg WebDAVConfig, path string) string {
	if path == "" {
		return davEncode(FSResult{Status: "error", Message: "'path' is required for delete"})
	}

	url := webdavURL(cfg, path)
	resp, err := webdavRequest(cfg, "DELETE", url, nil, nil)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("DELETE failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 200 {
		return davEncode(FSResult{Status: "success", Message: fmt.Sprintf("Deleted: %s", path)})
	}
	if resp.StatusCode == 404 {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Not found: %s", path)})
	}

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read DELETE error response: %v", err)})
	}
	return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("DELETE returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))})
}

// WebDAVMove moves or renames a file/directory on WebDAV.
func WebDAVMove(cfg WebDAVConfig, srcPath, dstPath string) string {
	if srcPath == "" || dstPath == "" {
		return davEncode(FSResult{Status: "error", Message: "'path' and 'destination' are required for move"})
	}

	srcURL := webdavURL(cfg, srcPath)
	dstURL := webdavURL(cfg, dstPath)

	resp, err := webdavRequest(cfg, "MOVE", srcURL, nil, map[string]string{
		"Destination": dstURL,
		"Overwrite":   "F",
	})
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("MOVE failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 || resp.StatusCode == 204 {
		return davEncode(FSResult{Status: "success", Message: fmt.Sprintf("Moved %s → %s", srcPath, dstPath)})
	}
	if resp.StatusCode == 412 {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Destination already exists: %s", dstPath)})
	}

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read MOVE error response: %v", err)})
	}
	return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("MOVE returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))})
}

// WebDAVInfo retrieves metadata for a single file/directory via PROPFIND depth=0.
func WebDAVInfo(cfg WebDAVConfig, path string) string {
	if path == "" {
		return davEncode(FSResult{Status: "error", Message: "'path' is required for info"})
	}

	url := webdavURL(cfg, path)
	propfindBody := `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getcontenttype/>
    <d:getlastmodified/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`

	resp, err := webdavRequest(cfg, "PROPFIND", url, strings.NewReader(propfindBody), map[string]string{
		"Content-Type": "application/xml",
		"Depth":        "0",
	})
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("PROPFIND failed: %v", err)})
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Not found: %s", path)})
	}
	if resp.StatusCode != 207 {
		body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
		if err != nil {
			return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read PROPFIND info error response: %v", err)})
		}
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("PROPFIND returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))})
	}

	data, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to read PROPFIND info response: %v", err)})
	}
	var ms davMultistatus
	if err := xml.Unmarshal(data, &ms); err != nil {
		return davEncode(FSResult{Status: "error", Message: fmt.Sprintf("Failed to parse response: %v", err)})
	}

	if len(ms.Responses) == 0 {
		return davEncode(FSResult{Status: "error", Message: "No metadata returned"})
	}

	r := ms.Responses[0]
	isDir := r.Propstat.Prop.ResourceType.Collection != nil

	info := map[string]interface{}{
		"name":     r.Propstat.Prop.DisplayName,
		"is_dir":   isDir,
		"size":     r.Propstat.Prop.ContentLen,
		"modified": r.Propstat.Prop.LastModified,
		"type":     r.Propstat.Prop.ContentType,
	}

	return davEncode(FSResult{Status: "success", Data: info})
}

// truncate is a helper to cap long strings.
func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
