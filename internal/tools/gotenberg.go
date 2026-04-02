package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// gotenbergClient is the shared HTTP client for Gotenberg requests.
var gotenbergClient *http.Client

// getGotenbergClient returns a lazily-initialised HTTP client with the configured timeout.
func getGotenbergClient(timeoutSec int) *http.Client {
	if gotenbergClient != nil {
		return gotenbergClient
	}
	t := 120
	if timeoutSec > 0 {
		t = timeoutSec
	}
	gotenbergClient = &http.Client{Timeout: time.Duration(t) * time.Second}
	return gotenbergClient
}

// gotenbergRequest sends a multipart/form-data request to a Gotenberg endpoint.
// formFields are simple key-value pairs; files maps filename → content.
// Returns the response body (typically a PDF or image) on success.
func gotenbergRequest(ctx context.Context, cfg *config.GotenbergConfig, route string, formFields map[string]string, files map[string][]byte) ([]byte, error) {
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
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, name))
		h.Set("Content-Type", "application/octet-stream")
		part, err := writer.CreatePart(h)
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

	resp, err := getGotenbergClient(cfg.Timeout).Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, fmt.Errorf("read gotenberg response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gotenberg returned HTTP %d: %s", resp.StatusCode, gotenbergTruncate(string(body), 500))
	}
	return body, nil
}

// saveGotenbergOutput writes response bytes to the output directory and returns metadata.
func saveGotenbergOutput(data []byte, outputDir, filename, ext string) (filePath, webPath string, err error) {
	if filename == "" {
		filename = fmt.Sprintf("doc_%d", time.Now().Unix())
	}
	// Sanitise filename to prevent path traversal
	filename = filepath.Base(filename)
	if !strings.HasSuffix(strings.ToLower(filename), ext) {
		filename += ext
	}

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return "", "", fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outPath, data, 0640); err != nil {
		return "", "", fmt.Errorf("write file: %w", err)
	}

	webPath = "/files/documents/" + filename
	return outPath, webPath, nil
}

// ── Gotenberg operations ─────────────────────────────────────────────────────

const gotenbergContainerName = "aurago_gotenberg"
const gotenbergImage = "gotenberg/gotenberg:8"

// EnsureGotenbergRunning ensures the Gotenberg container is running.
// It checks the container state via the Docker API and, if missing or stopped,
// creates/starts it. Safe to call multiple times.
func EnsureGotenbergRunning(dockerHost string, logger interface {
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}) {
	dockerCfg := DockerConfig{Host: dockerHost}

	// Check if ANY gotenberg container (any name) is already running.
	// This prevents creating a duplicate when the user already has a gotenberg
	// container running via docker-compose or another means.
	listData, listCode, listErr := dockerRequest(dockerCfg, "GET",
		`/containers/json?filters={"status":["running"],"ancestor":["gotenberg/gotenberg:8"]}`, "")
	if listErr == nil && listCode == 200 {
		var containers []map[string]interface{}
		if json.Unmarshal(listData, &containers) == nil && len(containers) > 0 {
			logger.Info("[Gotenberg] Container already running (external)", "count", len(containers))
			return
		}
	}

	// Inspect our own managed container
	data, code, err := dockerRequest(dockerCfg, "GET", "/containers/"+gotenbergContainerName+"/json", "")
	if err != nil {
		logger.Warn("[Gotenberg] Docker unavailable, skipping auto-start", "error", err)
		return
	}

	if code == 200 {
		// Container exists — check if it's running
		var info map[string]interface{}
		if json.Unmarshal(data, &info) == nil {
			if state, ok := info["State"].(map[string]interface{}); ok {
				if running, _ := state["Running"].(bool); running {
					logger.Info("[Gotenberg] Container already running")
					return
				}
			}
		}
		// Exists but stopped — just start it
		_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+gotenbergContainerName+"/start", "")
		if startErr != nil || (startCode != 204 && startCode != 304) {
			logger.Error("[Gotenberg] Failed to start existing container", "code", startCode, "error", startErr)
			return
		}
		logger.Info("[Gotenberg] Container started")
		return
	}

	if code != 404 {
		logger.Warn("[Gotenberg] Unexpected Docker inspect response, skipping auto-start", "code", code)
		return
	}

	// Container does not exist — create and start it
	payload := map[string]interface{}{
		"Image": gotenbergImage,
		"HostConfig": map[string]interface{}{
			"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
			"CapAdd":        []string{"SYS_ADMIN"},
			"ShmSize":       268435456, // 256 MiB
			"PortBindings": map[string]interface{}{
				"3000/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": "3000"}},
			},
		},
		"ExposedPorts": map[string]interface{}{
			"3000/tcp": struct{}{},
		},
	}
	body, _ := json.Marshal(payload)
	_, createCode, createErr := dockerRequest(dockerCfg, "POST", "/containers/create?name="+gotenbergContainerName, string(body))
	if createCode == 404 {
		// Image not present locally — pull it, then retry the create once.
		logger.Info("[Gotenberg] Image not found locally, pulling...", "image", gotenbergImage)
		if pullErr := pullDockerImage(dockerCfg, gotenbergImage); pullErr != nil {
			logger.Error("[Gotenberg] Image pull failed", "image", gotenbergImage, "error", pullErr)
			return
		}
		logger.Info("[Gotenberg] Image pulled successfully", "image", gotenbergImage)
		_, createCode, createErr = dockerRequest(dockerCfg, "POST", "/containers/create?name="+gotenbergContainerName, string(body))
	}
	if createErr != nil || createCode != 201 {
		logger.Error("[Gotenberg] Failed to create container", "code", createCode, "error", createErr)
		return
	}

	_, startCode, startErr := dockerRequest(dockerCfg, "POST", "/containers/"+gotenbergContainerName+"/start", "")
	if startErr != nil || (startCode != 204 && startCode != 304) {
		logger.Error("[Gotenberg] Failed to start new container", "code", startCode, "error", startErr)
		return
	}
	logger.Info("[Gotenberg] Container created and started", "image", gotenbergImage)
}

// pullDockerImage pulls a Docker image via the Docker Engine API.
// Uses a dedicated HTTP client with a long timeout (10 min) since image
// downloads can take considerable time on slow connections.
func pullDockerImage(cfg DockerConfig, image string) error {
	parts := strings.SplitN(image, ":", 2)
	fromImage := parts[0]
	tag := "latest"
	if len(parts) == 2 {
		tag = parts[1]
	}
	endpoint := "/images/create?fromImage=" + url.QueryEscape(fromImage) + "&tag=" + url.QueryEscape(tag)

	// Build a dedicated long-timeout client — the shared dockerHTTPClient is
	// only 60 s which is too short for a real image pull.
	client := getDockerClient(cfg)
	pullClient := &http.Client{
		Transport: client.Transport,
		Timeout:   10 * time.Minute,
	}

	reqURL := "http://localhost/" + dockerAPIVersion + endpoint
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("build pull request: %w", err)
	}
	resp, err := pullClient.Do(req)
	if err != nil {
		return fmt.Errorf("pull request failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxHTTPResponseSize)) // consume bounded streaming progress output
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Docker returned HTTP %d during image pull", resp.StatusCode)
	}
	return nil
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
	url := strings.TrimRight(cfg.URL, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return gotenbergErrJSON("create request: " + err.Error())
	}
	resp, err := getGotenbergClient(cfg.Timeout).Do(req)
	if err != nil {
		return gotenbergErrJSON("Gotenberg unreachable: " + err.Error())
	}
	defer resp.Body.Close()
	body, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return gotenbergErrJSON("Failed to read health response: " + err.Error())
	}
	if resp.StatusCode == 200 {
		// body is already valid JSON from Gotenberg; embed it as-is under "details".
		// If it's not valid JSON, fall back to a plain string field.
		var detailsRaw json.RawMessage
		if json.Unmarshal(body, &detailsRaw) != nil {
			detailsRaw, _ = json.Marshal(string(body))
		}
		out, _ := json.Marshal(map[string]interface{}{
			"status":  "success",
			"message": "Gotenberg is healthy",
			"details": detailsRaw,
		})
		return string(out)
	}
	return gotenbergErrJSON(fmt.Sprintf("Gotenberg returned HTTP %d: %s", resp.StatusCode, gotenbergTruncate(string(body), 300)))
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

	filePath, webPath, err := saveGotenbergOutput(data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergHTMLToPDF converts HTML content to PDF via Chromium.
func GotenbergHTMLToPDF(ctx context.Context, cfg *config.GotenbergConfig, outputDir, htmlContent, filename, paperSize string, landscape bool) string {
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

	filePath, webPath, err := saveGotenbergOutput(data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergMarkdownToPDF converts Markdown content to PDF via Chromium.
// Gotenberg expects an index.html with {{ toHTML "file.md" }} and the .md file.
func GotenbergMarkdownToPDF(ctx context.Context, cfg *config.GotenbergConfig, outputDir, markdownContent, filename, paperSize string, landscape bool) string {
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

	files := map[string][]byte{
		"index.html": []byte(indexHTML),
		"content.md": []byte(markdownContent),
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/chromium/convert/markdown", fields, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(data, outputDir, filename, ".pdf")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergConvertDocument converts office documents (docx, xlsx, pptx, etc.) to PDF via LibreOffice.
func GotenbergConvertDocument(ctx context.Context, cfg *config.GotenbergConfig, outputDir, sourceFilePath, filename, paperSize string, landscape bool) string {
	// Read the source file
	srcData, err := os.ReadFile(sourceFilePath)
	if err != nil {
		return gotenbergErrJSON("read source file: " + err.Error())
	}

	fields := paperSizeFields(paperSize)
	if landscape {
		fields["landscape"] = "true"
	}

	srcName := filepath.Base(sourceFilePath)
	files := map[string][]byte{
		srcName: srcData,
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/libreoffice/convert", fields, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	if filename == "" {
		filename = strings.TrimSuffix(srcName, filepath.Ext(srcName))
	}

	filePath, webPath, err := saveGotenbergOutput(data, outputDir, filename, ".pdf")
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
	if len(sourcePaths) < 2 {
		return `{"status":"error","message":"merge_pdfs requires at least 2 PDF files"}`
	}

	files := make(map[string][]byte, len(sourcePaths))
	for i, p := range sourcePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			return gotenbergErrJSON("read file " + filepath.Base(p) + ": " + err.Error())
		}
		// Gotenberg merges in alphabetical order of filenames
		name := fmt.Sprintf("%02d_%s", i+1, filepath.Base(p))
		files[name] = data
	}

	data, err := gotenbergRequest(ctx, cfg, "/forms/pdf-engines/merge", nil, files)
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}

	filePath, webPath, err := saveGotenbergOutput(data, outputDir, filename, ".pdf")
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

	filePath, webPath, err := saveGotenbergOutput(data, outputDir, filename, ".png")
	if err != nil {
		return gotenbergErrJSON(err.Error())
	}
	return gotenbergOKJSON(filePath, webPath, filepath.Base(filePath))
}

// GotenbergScreenshotHTML takes a screenshot of HTML content via Chromium.
func GotenbergScreenshotHTML(ctx context.Context, cfg *config.GotenbergConfig, outputDir, htmlContent, filename string) string {
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

	filePath, webPath, err := saveGotenbergOutput(data, outputDir, filename, ".png")
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
