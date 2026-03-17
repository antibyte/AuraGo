package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// OneDriveClient holds auth state for Microsoft Graph OneDrive API calls.
type OneDriveClient struct {
	AccessToken  string
	RefreshToken string
	TokenExpiry  time.Time
	ClientID     string
	ClientSecret string
	TenantID     string
	Vault        *security.Vault
}

var odHTTPClient = &http.Client{Timeout: 60 * time.Second}

// NewOneDriveClient builds a client from config + vault.
func NewOneDriveClient(cfg config.Config, vault *security.Vault) (*OneDriveClient, error) {
	od := cfg.OneDrive
	if od.AccessToken == "" {
		return nil, fmt.Errorf("no OneDrive access token — connect via Settings > OneDrive")
	}

	clientSecret := od.ClientSecret
	if clientSecret == "" && vault != nil {
		s, _ := vault.ReadSecret("onedrive_client_secret")
		clientSecret = s
	}

	var expiry time.Time
	if od.TokenExpiry != "" {
		expiry, _ = time.Parse(time.RFC3339, od.TokenExpiry)
	}

	tenantID := od.TenantID
	if tenantID == "" {
		tenantID = "common"
	}

	return &OneDriveClient{
		AccessToken:  od.AccessToken,
		RefreshToken: od.RefreshToken,
		TokenExpiry:  expiry,
		ClientID:     od.ClientID,
		ClientSecret: clientSecret,
		TenantID:     tenantID,
		Vault:        vault,
	}, nil
}

// refreshIfNeeded refreshes the access token if it has expired or is about to expire.
func (c *OneDriveClient) refreshIfNeeded() error {
	if c.RefreshToken == "" {
		return nil
	}
	if !c.TokenExpiry.IsZero() && time.Now().Before(c.TokenExpiry.Add(-60*time.Second)) {
		return nil // Still valid
	}

	form := url.Values{
		"client_id":     {c.ClientID},
		"refresh_token": {c.RefreshToken},
		"grant_type":    {"refresh_token"},
		"scope":         {"Files.ReadWrite.All offline_access"},
	}
	if c.ClientSecret != "" {
		form.Set("client_secret", c.ClientSecret)
	}

	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(c.TenantID))
	resp, err := odHTTPClient.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tok struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	c.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		c.RefreshToken = tok.RefreshToken
	}
	c.TokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)

	// Persist updated token to vault
	if c.Vault != nil {
		tokenData, _ := json.Marshal(map[string]string{
			"access_token":  c.AccessToken,
			"refresh_token": c.RefreshToken,
			"token_expiry":  c.TokenExpiry.Format(time.RFC3339),
		})
		if err := c.Vault.WriteSecret("oauth_onedrive", string(tokenData)); err != nil {
			slog.Warn("OneDrive: failed to persist refreshed token to vault", "error", err)
		}
	}

	return nil
}

// request makes an authenticated HTTP request to Microsoft Graph.
func (c *OneDriveClient) request(method, rawURL string, body interface{}) ([]byte, int, error) {
	if err := c.refreshIfNeeded(); err != nil {
		return nil, 0, err
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, rawURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := odHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

func odErrJSON(format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	out, _ := json.Marshal(map[string]string{"status": "error", "message": msg})
	return string(out)
}

// ODErrJSON is the exported version for use by the dispatch layer.
func ODErrJSON(format string, args ...interface{}) string {
	return odErrJSON(format, args...)
}

func odOkJSON(data interface{}) string {
	out, _ := json.Marshal(data)
	return "<external_data>" + string(out) + "</external_data>"
}

// odEscapePath escapes each path segment individually, preserving forward slashes
// as URL path separators. Use instead of url.PathEscape for multi-level paths.
func odEscapePath(path string) string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	escaped := make([]string, len(parts))
	for i, p := range parts {
		escaped[i] = url.PathEscape(p)
	}
	return strings.Join(escaped, "/")
}

// ExecuteOneDrive dispatches a OneDrive operation.
func (c *OneDriveClient) ExecuteOneDrive(operation, path, destination, content string, maxResults int) string {
	// Guard against path traversal
	for _, p := range []string{path, destination} {
		for _, part := range strings.Split(p, "/") {
			if part == ".." {
				return odErrJSON("Invalid path: path traversal ('..') is not allowed")
			}
		}
	}
	switch operation {
	case "list":
		return c.listItems(path, maxResults)
	case "info":
		return c.getItemInfo(path)
	case "read", "download":
		return c.readFile(path)
	case "search":
		return c.search(content, maxResults)
	case "quota":
		return c.getQuota()
	case "upload", "write":
		return c.uploadFile(path, content)
	case "mkdir":
		return c.createFolder(path)
	case "delete":
		return c.deleteItem(path)
	case "move":
		return c.moveItem(path, destination)
	case "copy":
		return c.copyItem(path, destination)
	case "share":
		return c.createShareLink(path)
	default:
		return odErrJSON("Unknown OneDrive operation: %s. Valid operations: list, info, read, download, search, quota, upload, write, mkdir, delete, move, copy, share", operation)
	}
}

// ── OneDrive Operations ────────────────────────────────────────────────────

func (c *OneDriveClient) listItems(path string, maxResults int) string {
	if maxResults <= 0 {
		maxResults = 50
	}

	var apiURL string
	if path == "" || path == "/" {
		apiURL = fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root/children?$top=%d&$select=name,size,lastModifiedDateTime,folder,file", maxResults)
	} else {
		apiURL = fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s:/children?$top=%d&$select=name,size,lastModifiedDateTime,folder,file", odEscapePath(path), maxResults)
	}

	data, status, err := c.request("GET", apiURL, nil)
	if err != nil {
		return odErrJSON("List failed: %v", err)
	}
	if status != 200 {
		return odErrJSON("List failed (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Value []struct {
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			Modified string `json:"lastModifiedDateTime"`
			Folder   *struct {
				ChildCount int `json:"childCount"`
			} `json:"folder,omitempty"`
			File *struct {
				MimeType string `json:"mimeType"`
			} `json:"file,omitempty"`
		} `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return odErrJSON("Failed to parse list response: %v", err)
	}

	type item struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Size       int64  `json:"size,omitempty"`
		Modified   string `json:"modified"`
		ChildCount int    `json:"child_count,omitempty"`
		MimeType   string `json:"mime_type,omitempty"`
	}
	items := make([]item, 0, len(result.Value))
	for _, v := range result.Value {
		i := item{Name: v.Name, Size: v.Size, Modified: v.Modified}
		if v.Folder != nil {
			i.Type = "folder"
			i.ChildCount = v.Folder.ChildCount
		} else {
			i.Type = "file"
			if v.File != nil {
				i.MimeType = v.File.MimeType
			}
		}
		items = append(items, i)
	}

	return odOkJSON(map[string]interface{}{
		"status": "ok",
		"path":   path,
		"count":  len(items),
		"items":  items,
	})
}

func (c *OneDriveClient) getItemInfo(path string) string {
	if path == "" || path == "/" {
		return odErrJSON("Path is required for info operation")
	}

	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s?$select=name,size,lastModifiedDateTime,createdDateTime,folder,file,webUrl,id,parentReference", odEscapePath(path))
	data, status, err := c.request("GET", apiURL, nil)
	if err != nil {
		return odErrJSON("Info failed: %v", err)
	}
	if status != 200 {
		return odErrJSON("Info failed (HTTP %d): %s", status, string(data))
	}

	var info map[string]interface{}
	_ = json.Unmarshal(data, &info)
	return odOkJSON(map[string]interface{}{"status": "ok", "item": info})
}

func (c *OneDriveClient) readFile(path string) string {
	if path == "" {
		return odErrJSON("Path is required for read operation")
	}

	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s:/content", odEscapePath(path))
	if err := c.refreshIfNeeded(); err != nil {
		return odErrJSON("Auth refresh failed: %v", err)
	}

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)

	// Don't auto-follow redirects to handle the download URL
	noRedirectClient := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return odErrJSON("Read failed: %v", err)
	}
	defer resp.Body.Close()

	// Graph API redirects to the actual download URL
	if resp.StatusCode == 302 {
		downloadURL := resp.Header.Get("Location")
		if downloadURL == "" {
			return odErrJSON("Got redirect but no Location header")
		}
		dlResp, err := odHTTPClient.Get(downloadURL)
		if err != nil {
			return odErrJSON("Download failed: %v", err)
		}
		defer dlResp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(dlResp.Body, 512*1024)) // Limit to 512KB for text content
		return odOkJSON(map[string]interface{}{
			"status":    "ok",
			"path":      path,
			"size":      len(body),
			"content":   string(body),
			"truncated": len(body) >= 512*1024,
		})
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return odErrJSON("Read failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	return odOkJSON(map[string]interface{}{
		"status":    "ok",
		"path":      path,
		"size":      len(body),
		"content":   string(body),
		"truncated": len(body) >= 512*1024,
	})
}

func (c *OneDriveClient) search(query string, maxResults int) string {
	if query == "" {
		return odErrJSON("Query is required for search operation (pass it in the 'content' parameter)")
	}
	if maxResults <= 0 {
		maxResults = 25
	}

	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root/search(q='%s')?$top=%d&$select=name,size,lastModifiedDateTime,folder,file,parentReference,webUrl",
		url.QueryEscape(query), maxResults)
	data, status, err := c.request("GET", apiURL, nil)
	if err != nil {
		return odErrJSON("Search failed: %v", err)
	}
	if status != 200 {
		return odErrJSON("Search failed (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Value []struct {
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			Modified string `json:"lastModifiedDateTime"`
			WebURL   string `json:"webUrl"`
			Folder   *struct {
				ChildCount int `json:"childCount"`
			} `json:"folder,omitempty"`
			File *struct {
				MimeType string `json:"mimeType"`
			} `json:"file,omitempty"`
			ParentReference struct {
				Path string `json:"path"`
			} `json:"parentReference"`
		} `json:"value"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return odErrJSON("Failed to parse search response: %v", err)
	}

	type searchItem struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Size       int64  `json:"size,omitempty"`
		Modified   string `json:"modified"`
		ParentPath string `json:"parent_path"`
	}
	items := make([]searchItem, 0, len(result.Value))
	for _, v := range result.Value {
		i := searchItem{Name: v.Name, Size: v.Size, Modified: v.Modified, ParentPath: v.ParentReference.Path}
		if v.Folder != nil {
			i.Type = "folder"
		} else {
			i.Type = "file"
		}
		items = append(items, i)
	}

	return odOkJSON(map[string]interface{}{
		"status": "ok",
		"query":  query,
		"count":  len(items),
		"items":  items,
	})
}

func (c *OneDriveClient) getQuota() string {
	data, status, err := c.request("GET", "https://graph.microsoft.com/v1.0/me/drive?$select=quota", nil)
	if err != nil {
		return odErrJSON("Quota failed: %v", err)
	}
	if status != 200 {
		return odErrJSON("Quota failed (HTTP %d): %s", status, string(data))
	}

	var result struct {
		Quota struct {
			Total     int64  `json:"total"`
			Used      int64  `json:"used"`
			Remaining int64  `json:"remaining"`
			State     string `json:"state"`
		} `json:"quota"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return odErrJSON("Failed to parse quota response: %v", err)
	}

	return odOkJSON(map[string]interface{}{
		"status":       "ok",
		"total_mb":     result.Quota.Total / (1024 * 1024),
		"used_mb":      result.Quota.Used / (1024 * 1024),
		"remaining_mb": result.Quota.Remaining / (1024 * 1024),
		"state":        result.Quota.State,
	})
}

func (c *OneDriveClient) uploadFile(path, content string) string {
	if path == "" {
		return odErrJSON("Path is required for upload operation")
	}
	if content == "" {
		return odErrJSON("Content is required for upload operation")
	}

	// For small files (< 4MB), use simple upload
	if len(content) > 4*1024*1024 {
		return odErrJSON("Content too large (%d bytes). Simple upload supports up to 4 MB.", len(content))
	}
	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s:/content", odEscapePath(path))

	if err := c.refreshIfNeeded(); err != nil {
		return odErrJSON("Auth refresh failed: %v", err)
	}

	req, _ := http.NewRequest("PUT", apiURL, strings.NewReader(content))
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := odHTTPClient.Do(req)
	if err != nil {
		return odErrJSON("Upload failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return odErrJSON("Upload failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	_ = json.Unmarshal(body, &result)

	return odOkJSON(map[string]interface{}{
		"status": "ok",
		"path":   path,
		"name":   result.Name,
		"size":   result.Size,
	})
}

func (c *OneDriveClient) createFolder(path string) string {
	if path == "" {
		return odErrJSON("Path is required for mkdir operation")
	}

	cleanPath := strings.TrimPrefix(path, "/")
	parts := strings.Split(cleanPath, "/")
	folderName := parts[len(parts)-1]

	var apiURL string
	if len(parts) == 1 {
		apiURL = "https://graph.microsoft.com/v1.0/me/drive/root/children"
	} else {
		parentPath := strings.Join(parts[:len(parts)-1], "/")
		apiURL = fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s:/children", odEscapePath(parentPath))
	}

	payload := map[string]interface{}{
		"name":                              folderName,
		"folder":                            map[string]interface{}{},
		"@microsoft.graph.conflictBehavior": "fail",
	}

	data, status, err := c.request("POST", apiURL, payload)
	if err != nil {
		return odErrJSON("Mkdir failed: %v", err)
	}
	if status != 201 && status != 200 {
		return odErrJSON("Mkdir failed (HTTP %d): %s", status, string(data))
	}

	return odOkJSON(map[string]interface{}{
		"status": "ok",
		"path":   path,
	})
}

func (c *OneDriveClient) deleteItem(path string) string {
	if path == "" {
		return odErrJSON("Path is required for delete operation")
	}

	apiURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s", odEscapePath(path))
	data, status, err := c.request("DELETE", apiURL, nil)
	if err != nil {
		return odErrJSON("Delete failed: %v", err)
	}
	if status != 204 && status != 200 {
		return odErrJSON("Delete failed (HTTP %d): %s", status, string(data))
	}

	return odOkJSON(map[string]interface{}{
		"status": "ok",
		"path":   path,
	})
}

func (c *OneDriveClient) moveItem(path, destination string) string {
	if path == "" || destination == "" {
		return odErrJSON("Both path and destination are required for move operation")
	}

	// Get item ID first
	itemURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s?$select=id", odEscapePath(path))
	data, status, err := c.request("GET", itemURL, nil)
	if err != nil {
		return odErrJSON("Move failed (lookup): %v", err)
	}
	if status != 200 {
		return odErrJSON("Move failed — source not found (HTTP %d): %s", status, string(data))
	}

	var item struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data, &item)

	// Resolve destination parent and new name
	destClean := strings.TrimPrefix(destination, "/")
	destParts := strings.Split(destClean, "/")
	newName := destParts[len(destParts)-1]

	payload := map[string]interface{}{
		"name": newName,
	}

	if len(destParts) > 1 {
		parentPath := strings.Join(destParts[:len(destParts)-1], "/")
		parentURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s?$select=id", odEscapePath(parentPath))
		pData, pStatus, pErr := c.request("GET", parentURL, nil)
		if pErr != nil || pStatus != 200 {
			return odErrJSON("Move failed — destination parent not found: %s", string(pData))
		}
		var parent struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(pData, &parent)
		payload["parentReference"] = map[string]string{"id": parent.ID}
	}

	moveURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s", url.PathEscape(item.ID))
	data, status, err = c.request("PATCH", moveURL, payload)
	if err != nil {
		return odErrJSON("Move failed: %v", err)
	}
	if status != 200 {
		return odErrJSON("Move failed (HTTP %d): %s", status, string(data))
	}

	return odOkJSON(map[string]interface{}{
		"status":      "ok",
		"source":      path,
		"destination": destination,
	})
}

func (c *OneDriveClient) copyItem(path, destination string) string {
	if path == "" || destination == "" {
		return odErrJSON("Both path and destination are required for copy operation")
	}

	// Get item ID
	itemURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s?$select=id", odEscapePath(path))
	data, status, err := c.request("GET", itemURL, nil)
	if err != nil {
		return odErrJSON("Copy failed (lookup): %v", err)
	}
	if status != 200 {
		return odErrJSON("Copy failed — source not found (HTTP %d): %s", status, string(data))
	}

	var item struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data, &item)

	destClean := strings.TrimPrefix(destination, "/")
	destParts := strings.Split(destClean, "/")
	newName := destParts[len(destParts)-1]

	payload := map[string]interface{}{
		"name": newName,
	}

	// Resolve parent for copy target
	if len(destParts) > 1 {
		parentPath := strings.Join(destParts[:len(destParts)-1], "/")
		parentURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s?$select=id", odEscapePath(parentPath))
		pData, pStatus, pErr := c.request("GET", parentURL, nil)
		if pErr != nil || pStatus != 200 {
			return odErrJSON("Copy failed — destination parent not found: %s", string(pData))
		}
		var parent struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(pData, &parent)
		payload["parentReference"] = map[string]string{"driveId": "", "id": parent.ID}
	} else {
		// Copy to root
		rootURL := "https://graph.microsoft.com/v1.0/me/drive/root?$select=id"
		rData, rStatus, rErr := c.request("GET", rootURL, nil)
		if rErr != nil || rStatus != 200 {
			return odErrJSON("Copy failed — cannot resolve root folder")
		}
		var root struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(rData, &root)
		payload["parentReference"] = map[string]string{"id": root.ID}
	}

	copyURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s/copy", url.PathEscape(item.ID))
	data, status, err = c.request("POST", copyURL, payload)
	if err != nil {
		return odErrJSON("Copy failed: %v", err)
	}
	// Copy returns 202 Accepted (async) or 200
	if status != 202 && status != 200 {
		return odErrJSON("Copy failed (HTTP %d): %s", status, string(data))
	}

	return odOkJSON(map[string]interface{}{
		"status":      "ok",
		"source":      path,
		"destination": destination,
		"note":        "Copy may be processed asynchronously by Microsoft",
	})
}

func (c *OneDriveClient) createShareLink(path string) string {
	if path == "" {
		return odErrJSON("Path is required for share operation")
	}

	// Get item ID
	itemURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s?$select=id", odEscapePath(path))
	data, status, err := c.request("GET", itemURL, nil)
	if err != nil {
		return odErrJSON("Share failed (lookup): %v", err)
	}
	if status != 200 {
		return odErrJSON("Share failed — item not found (HTTP %d): %s", status, string(data))
	}

	var item struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(data, &item)

	shareURL := fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s/createLink", url.PathEscape(item.ID))
	payload := map[string]interface{}{
		"type":  "view",
		"scope": "anonymous",
	}

	data, status, err = c.request("POST", shareURL, payload)
	if err != nil {
		return odErrJSON("Share failed: %v", err)
	}
	if status != 200 && status != 201 {
		return odErrJSON("Share failed (HTTP %d): %s", status, string(data))
	}

	var shareResp struct {
		Link struct {
			WebURL string `json:"webUrl"`
			Type   string `json:"type"`
		} `json:"link"`
	}
	_ = json.Unmarshal(data, &shareResp)

	return odOkJSON(map[string]interface{}{
		"status":    "ok",
		"path":      path,
		"share_url": shareResp.Link.WebURL,
		"link_type": shareResp.Link.Type,
	})
}
