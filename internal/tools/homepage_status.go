package tools

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type homepageLocalPreviewStatus struct {
	Available  bool   `json:"available"`
	HTTPStatus int    `json:"http_status,omitempty"`
	ProjectDir string `json:"project_dir,omitempty"`
	Error      string `json:"error,omitempty"`
}

// A running container can still serve an empty 404 from a missing bind mount.
func homepageProbeLocalPreview(cfg HomepageConfig) homepageLocalPreviewStatus {
	projectDir, _ := loadWebserverState(cfg.WorkspacePath)
	result := homepageLocalPreviewStatus{ProjectDir: projectDir, Error: "unavailable"}
	port := cfg.WebServerPort
	if port <= 0 {
		port = 8080
	}
	ctx, cancel := context.WithTimeout(nativeHTTPContext(cfg.Context), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
	if err != nil {
		return result
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		result.Error = "http_error"
		return result
	}
	data, err := homepageReadDocument(resp.Body)
	if err != nil {
		result.Error = "invalid_document"
		return result
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if _, err := homepageDocumentIntegrity(data); err != nil || !strings.HasPrefix(strings.ToLower(contentType), "text/html") {
		result.Error = "invalid_document"
		return result
	}
	result.Available, result.Error = true, ""
	return result
}
