package tools

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"aurago/internal/security"
)

// Pre-compiled regexps for DDG result parsing.
var (
	reResultAnchor = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
	reTableCell    = regexp.MustCompile(`(?is)<td\b([^>]*)>(.*?)</td>`)
	reHTMLAttr     = regexp.MustCompile(`(?is)\b([a-zA-Z0-9_-]+)\s*=\s*"([^"]*)"|\b([a-zA-Z0-9_-]+)\s*=\s*'([^']*)'`)
	reHTMLTag      = regexp.MustCompile(`(?is)<[^>]+>`)
	reMultiWS      = regexp.MustCompile(`\s+`)
)

type ddgTitleLink struct {
	title string
	link  string
}

// ExecuteDDGSearch performs a DuckDuckGo HTML search
func ExecuteDDGSearch(query string, maxResults int) string {
	if maxResults <= 0 {
		maxResults = 5
	}

	// Create form data for DDG Lite
	formData := url.Values{}
	formData.Set("q", query)

	req, err := http.NewRequest("POST", "https://lite.duckduckgo.com/lite/", strings.NewReader(formData.Encode()))
	if err != nil {
		return formatError(fmt.Sprintf("Failed to create request: %v", err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := scraperHTTPClient.Do(req)
	if err != nil {
		return formatError(fmt.Sprintf("DDG request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return formatError(fmt.Sprintf("DDG HTTP Error %d", resp.StatusCode))
	}

	bodyBytes, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return formatError(fmt.Sprintf("Failed to read DDG response: %v", err))
	}
	htmlStr := string(bodyBytes)

	titleLinks := parseDDGTitleLinks(htmlStr)
	snippets := parseDDGSnippets(htmlStr)

	results := make([]map[string]interface{}, 0)
	limit := len(titleLinks)
	if limit > maxResults {
		limit = maxResults
	}

	for i := 0; i < limit; i++ {
		snippet := ""
		if i < len(snippets) {
			snippet = snippets[i]
		}

		results = append(results, map[string]interface{}{
			"title":   security.IsolateExternalData(titleLinks[i].title),
			"link":    titleLinks[i].link,
			"snippet": security.IsolateExternalData(snippet),
		})
	}

	if len(results) == 0 {
		return formatError("No parseable DDG results found. DuckDuckGo may have returned a bot-check, consent page, no-results page, or changed markup.")
	}

	resultMap := map[string]interface{}{
		"status":  "success",
		"results": results,
	}
	b, _ := json.Marshal(resultMap)
	return string(b)
}

func parseDDGTitleLinks(htmlStr string) []ddgTitleLink {
	matches := reResultAnchor.FindAllStringSubmatch(htmlStr, -1)
	results := make([]ddgTitleLink, 0, len(matches))
	for _, match := range matches {
		attrs := htmlAttrs(match[1])
		if !hasHTMLClass(attrs["class"], "result-link") && !hasHTMLClass(attrs["class"], "result-url") {
			continue
		}
		link := strings.TrimSpace(attrs["href"])
		title := stripHTML(match[2])
		if link == "" || title == "" {
			continue
		}
		results = append(results, ddgTitleLink{
			title: title,
			link:  link,
		})
	}
	return results
}

func parseDDGSnippets(htmlStr string) []string {
	matches := reTableCell.FindAllStringSubmatch(htmlStr, -1)
	snippets := make([]string, 0, len(matches))
	for _, match := range matches {
		attrs := htmlAttrs(match[1])
		if !hasHTMLClass(attrs["class"], "result-snippet") {
			continue
		}
		snippet := stripHTML(match[2])
		if snippet != "" {
			snippets = append(snippets, snippet)
		}
	}
	return snippets
}

func htmlAttrs(raw string) map[string]string {
	attrs := make(map[string]string)
	for _, match := range reHTMLAttr.FindAllStringSubmatch(raw, -1) {
		name := match[1]
		value := match[2]
		if name == "" {
			name = match[3]
			value = match[4]
		}
		attrs[strings.ToLower(name)] = stdhtml.UnescapeString(value)
	}
	return attrs
}

func hasHTMLClass(classAttr, wanted string) bool {
	for _, className := range strings.Fields(classAttr) {
		if strings.EqualFold(className, wanted) {
			return true
		}
	}
	return false
}

func stripHTML(htmlStr string) string {
	text := reHTMLTag.ReplaceAllString(htmlStr, " ")
	text = stdhtml.UnescapeString(text)
	text = reMultiWS.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
