// Package tools – web_capture: rod-based screenshot and PDF capture.
package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// webCaptureResult is the JSON payload returned by WebCapture.
type webCaptureResult struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	File      string `json:"file,omitempty"`
	Message   string `json:"message,omitempty"`
}

// WebCapture takes a screenshot (PNG) or renders a PDF of the given URL using
// a headless Chromium browser via go-rod (no Gotenberg required).
//
// operation – "screenshot" or "pdf"
// rawURL    – target page URL
// selector  – optional CSS selector to wait for before capture
// fullPage  – capture full scrollable page (screenshot only)
// outputDir – directory where the output file is saved; defaults to workdir
func WebCapture(operation, rawURL, selector string, fullPage bool, outputDir string) string {
	encode := func(r webCaptureResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	// Validate operation
	operation = strings.ToLower(strings.TrimSpace(operation))
	if operation != "screenshot" && operation != "pdf" {
		return encode(webCaptureResult{Status: "error", Message: "operation must be 'screenshot' or 'pdf'"})
	}

	// Validate URL
	if rawURL == "" {
		return encode(webCaptureResult{Status: "error", Message: "url is required"})
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return encode(webCaptureResult{Status: "error", Message: "url must be a valid http or https URL"})
	}

	// Resolve output directory
	if outputDir == "" {
		outputDir = "agent_workspace/workdir"
	}
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("cannot create output dir: %v", err)})
	}

	// Launch headless browser
	u, err := launcher.New().
		Headless(true).
		NoSandbox(true).
		Launch()
	if err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("browser launch failed: %v", err)})
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("browser connect failed: %v", err)})
	}
	defer browser.MustClose()

	page, err := browser.Page(proto.TargetCreateTarget{URL: rawURL})
	if err != nil {
		return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("open page failed: %v", err)})
	}

	if err := page.WaitLoad(); err != nil {
		// non-fatal; continue with possibly partial page
		_ = err
	}

	if selector != "" {
		_ = page.WaitElementsMoreThan(selector, 0) // best-effort
	}

	// Build a safe filename from URL + timestamp
	ts := time.Now().Format("20060102_150405")
	host := parsed.Hostname()
	var filename string

	switch operation {
	case "screenshot":
		filename = filepath.Join(outputDir, fmt.Sprintf("screenshot_%s_%s.png", host, ts))
		quality := 90
		var screenshotBytes []byte
		if fullPage {
			screenshotBytes, err = page.Screenshot(true, &proto.PageCaptureScreenshot{
				Format:  proto.PageCaptureScreenshotFormatPng,
				Quality: &quality,
			})
		} else {
			screenshotBytes, err = page.Screenshot(false, &proto.PageCaptureScreenshot{
				Format:  proto.PageCaptureScreenshotFormatPng,
				Quality: &quality,
			})
		}
		if err != nil {
			return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("screenshot failed: %v", err)})
		}
		if err := os.WriteFile(filename, screenshotBytes, 0o640); err != nil {
			return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("write file failed: %v", err)})
		}

	case "pdf":
		filename = filepath.Join(outputDir, fmt.Sprintf("page_%s_%s.pdf", host, ts))
		pdfReader, err := page.PDF(&proto.PagePrintToPDF{
			PrintBackground: true,
		})
		if err != nil {
			return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("pdf render failed: %v", err)})
		}
		pdfBytes, err := io.ReadAll(pdfReader)
		if err != nil {
			return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("read pdf stream failed: %v", err)})
		}
		if err := os.WriteFile(filename, pdfBytes, 0o640); err != nil {
			return encode(webCaptureResult{Status: "error", Message: fmt.Sprintf("write file failed: %v", err)})
		}
	}

	return encode(webCaptureResult{
		Status:    "success",
		Operation: operation,
		File:      filename,
		Message:   fmt.Sprintf("%s saved to %s", operation, filename),
	})
}
