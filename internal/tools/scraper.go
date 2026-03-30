package tools

import (
	"aurago/internal/scraper"
	"aurago/internal/security"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Pre-compiled regexps for HTML cleaning (used by shared helpers in this package).
var (
	reScript = regexp.MustCompile(`(?is)<script.*?>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style.*?>.*?</style>`)
	reTag    = regexp.MustCompile(`(?is)<[^>]+>`)
	reSpace  = regexp.MustCompile(`\s+`)
)

// Shared HTTP client for DDG and other helpers (avoids per-call allocation).
var scraperHTTPClient = security.NewSSRFProtectedHTTPClient(15 * time.Second)

// scraperGuardian is a package-level Guardian used to scan and isolate web content.
var scraperGuardian = security.NewGuardian(nil)

// ExecuteWebScraper fetches a URL using colly + go-readability, converts the main
// article content to Markdown, then returns a compact JSON result.
// SSRF protection, prompt-injection scanning, and <external_data> isolation are
// all handled by the underlying AgentScraper.
func ExecuteWebScraper(rawURL string) string {
	s := scraper.New(scraperGuardian)
	result, err := s.FetchStatic(rawURL)
	if err != nil {
		return formatError(fmt.Sprintf("scrape failed: %v", err))
	}
	out := map[string]interface{}{
		"status":  "success",
		"title":   result.Title,
		"content": result.Markdown,
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func formatError(msg string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"status":  "error",
		"message": msg,
	})
	return string(b)
}
