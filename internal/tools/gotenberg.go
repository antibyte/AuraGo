package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/httporigin"
	"aurago/internal/security"
)

// Each call keeps its timeout; the default transport shares connection pools.
func getGotenbergClient(timeoutSec int) *http.Client {
	t := 120
	if timeoutSec > 0 {
		t = timeoutSec
	}
	return httporigin.NewClient(time.Duration(t) * time.Second)
}

// gotenbergRequest sends a multipart/form-data request to a Gotenberg endpoint.
// formFields are simple key-value pairs; files maps filename → content.
// Returns the response body (typically a PDF or image) on success.
func gotenbergRequest(ctx context.Context, cfg *config.GotenbergConfig, route string, formFields map[string]string, files map[string][]byte) ([]byte, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Gotenberg URL is not configured")
	}
	client := getGotenbergClient(cfg.Timeout)
	// A redirect must not replay a conversion upload, even on the same origin.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	ctx, cancel := context.WithTimeout(ctx, client.Timeout)
	defer cancel()
	if _, err := verifyGotenbergSecurity(ctx, cfg); err != nil {
		return nil, err
	}
	url := strings.TrimRight(cfg.URL, "/") + route

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Write form fields
	for k, v := range formFields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("write field %s: %w", k, err)
		}
	}

	// Write file parts
	for name, data := range files {
		if strings.ContainsAny(name, "\r\n\x00") {
			return nil, fmt.Errorf("multipart filename contains control characters")
		}
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			return nil, fmt.Errorf("create file part %s: %w", name, err)
		}
		if _, err := part.Write(data); err != nil {
			return nil, fmt.Errorf("write file part %s: %w", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("read Gotenberg PDF/image output (maximum 10 MiB): %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gotenberg returned HTTP %d: %s", resp.StatusCode, gotenbergTruncate(string(body), 500))
	}
	return body, nil
}

// saveGotenbergOutput writes response bytes to the output directory and returns metadata.
func saveGotenbergOutput(ctx context.Context, data []byte, outputDir, filename, ext string) (filePath, webPath string, err error) {
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return "", "", fmt.Errorf("create output dir: %w", err)
	}
	return saveDocumentOutput(ctx, data, outputDir, filename, ext)
}

// ── Gotenberg operations ─────────────────────────────────────────────────────

// pullDockerImage pulls tagged images for the managed Ollama sidecars.
// A reference without a tag is pulled as ":latest" (the Engine would pull
// every tag otherwise). The pull reads the Engine's progress stream to its end
// on the streaming pull client, bounded to dockerPullFallbackTimeout.
func pullDockerImage(cfg DockerConfig, image string) error {
	parts := strings.SplitN(image, ":", 2)
	fromImage := parts[0]
	tag := "latest"
	if len(parts) == 2 {
		tag = parts[1]
	}
	err := pullDockerImageQuery(context.Background(), cfg, image, url.Values{"fromImage": {fromImage}, "tag": {tag}})
	var pullErr *dockerPullError
	if errors.As(err, &pullErr) && pullErr.StatusCode != 0 {
		if pullErr.Message != "" {
			return fmt.Errorf("Docker returned HTTP %d during image pull: %s", pullErr.StatusCode, pullErr.Message)
		}
		return fmt.Errorf("Docker returned HTTP %d during image pull", pullErr.StatusCode)
	}
	return err
}

// gotenbergErrJSON returns a properly JSON-encoded error response.
// Using json.Marshal avoids broken JSON when error messages contain '"' (e.g. Go
// http errors: Get "http://host:3000/health": dial tcp ...: connection refused).
func gotenbergErrJSON(msg string) string {
	b, _ := json.Marshal(map[string]string{"status": "error", "message": msg})
	return string(b)
}

// gotenbergOKJSON returns a properly JSON-encoded success response with file metadata.
func gotenbergOKJSON(filePath, webPath, filename string) string {
	b, _ := json.Marshal(map[string]string{
		"status":    "success",
		"file_path": filePath,
		"web_path":  webPath,
		"filename":  filename,
	})
	return string(b)
}

// GotenbergHealth checks if the Gotenberg sidecar is reachable.
func GotenbergHealth(ctx context.Context, cfg *config.GotenbergConfig) string {
	probeError := func(message string, reachable bool) string {
		out, _ := json.Marshal(map[string]interface{}{"status": "error", "message": message, "reachable": reachable, "security_verified": false})
		return string(out)
	}
	if cfg == nil || cfg.URL == "" {
		return probeError("Gotenberg URL is not configured", false)
	}
	url := strings.TrimRight(cfg.URL, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return probeError("Cannot create Gotenberg health request", false)
	}
	resp, err := getGotenbergClient(cfg.Timeout).Do(req)
	if err != nil {
		return probeError("Gotenberg unreachable", false)
	}
	defer resp.Body.Close()
	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return probeError("Failed to read health response: "+err.Error(), true)
	}
	if resp.StatusCode == 200 {
		// body is already valid JSON from Gotenberg; embed it as-is under "details".
		// If it's not valid JSON, fall back to a plain string field.
		var detailsRaw json.RawMessage
		if json.Unmarshal(body, &detailsRaw) != nil {
			detailsRaw, _ = json.Marshal(string(body))
		}
		version, securityErr := verifyGotenbergSecurity(ctx, cfg)
		if securityErr != nil {
			out, _ := json.Marshal(map[string]interface{}{
				"status": "error", "message": securityErr.Error(), "reachable": true, "security_verified": false,
			})
			return string(out)
		}
		out, _ := json.Marshal(map[string]interface{}{
			"status":            "success",
			"message":           "Gotenberg is healthy",
			"details":           detailsRaw,
			"reachable":         true,
			"security_verified": true,
			"version":           version,
		})
		return string(out)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status": "error", "message": fmt.Sprintf("Gotenberg returned HTTP %d", resp.StatusCode),
		"reachable": true, "security_verified": false,
	})
	return string(out)
}

// GotenbergURLToPDF converts a URL to PDF via Chromium.
func GotenbergURLToPDF(ctx context.Context, cfg *config.GotenbergConfig, outputDir, url, filename, paperSize string, landscape bool) string {
	if err := security.ValidateSSRF(url); err != nil {
		return gotenbergErrJSON(fmt.Sprintf("SSRF validation failed: %v", err))
	}
	fields := paperSizeFields(paperSize)
	fields["url"] = url
	fields["printBackground"] = "true"
	if landscape {
		fields["landscape"] = "true"
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/chromium/convert/url", fields, nil)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(ctx, data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergHTMLToPDF converts HTML content to PDF via Chromium. With opts.BlockRemoteContent
// the HTML goes through restrictRemoteContent first, so the render stays off the network.
func GotenbergHTMLToPDF(ctx context.Context, cfg *config.GotenbergConfig, outputDir, htmlContent, filename, paperSize string, landscape bool, opts DocumentCreatorOptions) string {
	if opts.BlockRemoteContent {
		htmlContent = restrictRemoteContent(htmlContent)
	}
	fields := paperSizeFields(paperSize)
	fields["printBackground"] = "true"
	if landscape {
		fields["landscape"] = "true"
	}

	files := map[string][]byte{
		"index.html": []byte(htmlContent),
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/chromium/convert/html", fields, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(ctx, data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergMarkdownToPDF converts Markdown content to PDF via Chromium.
// Gotenberg expects an index.html with {{ toHTML "file.md" }} and the .md file.
// With opts.BlockRemoteContent the wrapper page gets the restrictRemoteContent policy and the
// Markdown loses its risky tags, so the render stays off the network.
func GotenbergMarkdownToPDF(ctx context.Context, cfg *config.GotenbergConfig, outputDir, markdownContent, filename, paperSize string, landscape bool, opts DocumentCreatorOptions) string {
	fields := paperSizeFields(paperSize)
	fields["printBackground"] = "true"
	if landscape {
		fields["landscape"] = "true"
	}

	// Gotenberg Markdown route requires an index.html wrapper and the .md file
	indexHTML := `<!DOCTYPE html>
<html><head><meta charset="utf-8"><style>
body { font-family: sans-serif; margin: 2cm; line-height: 1.6; }
h1,h2,h3 { color: #333; }
code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
pre { background: #f4f4f4; padding: 1em; border-radius: 5px; overflow-x: auto; }
table { border-collapse: collapse; width: 100%; }
th,td { border: 1px solid #ddd; padding: 8px; text-align: left; }
th { background: #f2f2f2; }
</style></head><body>{{ toHTML "content.md" }}</body></html>`
	if opts.BlockRemoteContent {
		indexHTML = restrictRemoteContent(indexHTML)
		markdownContent = restrictRemoteMarkdown(markdownContent)
	}

	files := map[string][]byte{
		"index.html": []byte(indexHTML),
		"content.md": []byte(markdownContent),
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/chromium/convert/markdown", fields, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(ctx, data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergConvertDocument converts office documents (docx, xlsx, pptx, etc.) to PDF via LibreOffice.
func GotenbergConvertDocument(ctx context.Context, cfg *config.GotenbergConfig, outputDir, sourceFilePath, filename, paperSize string, landscape bool) string {
	// Read the source file
	sources, err := readDocumentSources(ctx, "convert_document", []string{sourceFilePath})
	if err != nil {
		return gotenbergErrJSON("read source file: " + err.Error())
	}

	fields := paperSizeFields(paperSize)
	if landscape {
		fields["landscape"] = "true"
	}

	srcName := filepath.Base(sourceFilePath)
	files := map[string][]byte{
		srcName: sources[0],
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/libreoffice/convert", fields, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	if filename == "" {
		filename = strings.TrimSuffix(srcName, filepath.Ext(srcName))
	}

	filePath, webPath, err := saveGotenbergOutput(ctx, data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	b, _ := json.Marshal(map[string]string{
		"status":    "success",
		"file_path": filePath,
		"web_path":  webPath,
		"filename":  filepath.Base(filePath),
		"source":    srcName,
	})
	return string(b)
}

// GotenbergMergePDFs merges multiple PDF files into one.
func GotenbergMergePDFs(ctx context.Context, cfg *config.GotenbergConfig, outputDir string, sourcePaths []string, filename string) string {
	sources, err := readDocumentSources(ctx, "merge_pdfs", sourcePaths)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	files := make(map[string][]byte, len(sourcePaths))
	for i, p := range sourcePaths {
		// Gotenberg merges in alphabetical order of filenames
		name := fmt.Sprintf("%02d_%s", i+1, filepath.Base(p))
		files[name] = sources[i]
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/pdf-engines/merge", nil, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(ctx, data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	b, _ := json.Marshal(map[string]interface{}{
		"status":       "success",
		"file_path":    filePath,
		"web_path":     webPath,
		"filename":     filepath.Base(filePath),
		"merged_count": len(sourcePaths),
	})
	return string(b)
}

// GotenbergScreenshotURL takes a screenshot of a URL via Chromium.
func GotenbergScreenshotURL(ctx context.Context, cfg *config.GotenbergConfig, outputDir, url, filename string) string {
	if err := security.ValidateSSRF(url); err != nil {
		return gotenbergErrJSON(fmt.Sprintf("SSRF validation failed: %v", err))
	}
	fields := map[string]string{
		"url":    url,
		"format": "png",
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/chromium/screenshot/url", fields, nil)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(ctx, data, outputDir, filename, ".png")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergScreenshotHTML takes a screenshot of HTML content via Chromium. With
// opts.BlockRemoteContent the HTML goes through restrictRemoteContent first, so the render
// stays off the network.
func GotenbergScreenshotHTML(ctx context.Context, cfg *config.GotenbergConfig, outputDir, htmlContent, filename string, opts DocumentCreatorOptions) string {
	if opts.BlockRemoteContent {
		htmlContent = restrictRemoteContent(htmlContent)
	}
	fields := map[string]string{
		"format": "png",
	}
	files := map[string][]byte{
		"index.html": []byte(htmlContent),
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/chromium/screenshot/html", fields, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(ctx, data, outputDir, filename, ".png")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// paperSizeFields returns Gotenberg form fields for standard paper sizes (in inches).
func paperSizeFields(size string) map[string]string {
	fields := make(map[string]string)
	switch strings.ToUpper(size) {
	case "A3":
		fields["paperWidth"] = "11.7"
		fields["paperHeight"] = "16.54"
	case "A5":
		fields["paperWidth"] = "5.83"
		fields["paperHeight"] = "8.27"
	case "LETTER":
		fields["paperWidth"] = "8.5"
		fields["paperHeight"] = "11"
	case "LEGAL":
		fields["paperWidth"] = "8.5"
		fields["paperHeight"] = "14"
	case "TABLOID":
		fields["paperWidth"] = "11"
		fields["paperHeight"] = "17"
	default: // A4
		fields["paperWidth"] = "8.27"
		fields["paperHeight"] = "11.7"
	}
	return fields
}

// gotenbergTruncate truncates a string to maxLen, appending "…" if truncated.
func gotenbergTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
