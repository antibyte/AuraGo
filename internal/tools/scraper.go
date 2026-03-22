package tools

import (
	"aurago/internal/security"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Pre-compiled regexps for HTML cleaning (used by scraper and shared helpers).
var (
	reScript = regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style.*?>.*?</style>`)
	reTag    = regexp.MustCompile(`(?is)<[^>]+>`)
	reSpace  = regexp.MustCompile(`\s+`)
)

// Shared HTTP client for scraper/DDG (avoids per-call allocation).
var scraperHTTPClient = &http.Client{Timeout: 15 * time.Second}

// scraperGuardian is a package-level Guardian used to scan and isolate web content.
// It has no logger so threats are not logged here; callers with a logger should
// use the internal/scraper package's AgentScraper instead.
var scraperGuardian = security.NewGuardian(nil)

// ExecuteWebScraper fetches a URL, removes script/style tags, extracts plain text,
// then scans for prompt injection and isolates the content in <external_data> tags.
func ExecuteWebScraper(rawURL string) string {
	// SSRF protection: reject private/internal addresses and non-HTTP(S) schemes.
	if err := security.ValidateSSRF(rawURL); err != nil {
		return formatError(fmt.Sprintf("URL not allowed: %v", err))
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return formatError(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")

	resp, err := scraperHTTPClient.Do(req)
	if err != nil {
		return formatError(fmt.Sprintf("Request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return formatError(fmt.Sprintf("HTTP Error %d: %s", resp.StatusCode, resp.Status))
	}

	// Reject non-text content types to avoid wasting resources on binary files.
	if ct := resp.Header.Get("Content-Type"); ct != "" &&
		!strings.HasPrefix(ct, "text/") &&
		!strings.HasPrefix(ct, "application/xhtml") &&
		!strings.HasPrefix(ct, "application/json") {
		return formatError(fmt.Sprintf("Unsupported content type: %s", ct))
	}

	// Limit body reads to 5 MB to prevent memory exhaustion.
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return formatError(fmt.Sprintf("Failed to read body: %v", err))
	}
	htmlStr := string(bodyBytes)

	// Remove scripts and styles
	htmlStr = reScript.ReplaceAllString(htmlStr, " ")
	htmlStr = reStyle.ReplaceAllString(htmlStr, " ")

	// Remove all other HTML tags
	textStr := reTag.ReplaceAllString(htmlStr, " ")

	// Clean up whitespaces
	textStr = reSpace.ReplaceAllString(textStr, " ")
	textStr = strings.TrimSpace(textStr)

	// Limit to 10k characters
	if len(textStr) > 10000 {
		textStr = textStr[:10000]
	}

	// Scan for prompt injection and wrap in isolation tags.
	// ScanExternalContent always isolates; threats are detected even if not logged.
	isolated := scraperGuardian.ScanExternalContent(rawURL, textStr)

	result := map[string]interface{}{
		"status":  "success",
		"content": isolated,
	}
	b, _ := json.Marshal(result)
	return string(b)
}

func formatError(msg string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"status":  "error",
		"message": msg,
	})
	return string(b)
}
