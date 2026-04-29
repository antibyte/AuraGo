// Package tools – crawler: multi-page site crawler using Colly.
package tools

import (
	"aurago/internal/security"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/gocolly/colly/v2"
)

// crawlPage represents one crawled page in the results.
type crawlPage struct {
	URL            string `json:"url"`
	Title          string `json:"title"`
	ContentPreview string `json:"content_preview"` // first 500 chars of text
}

// crawlResult is the JSON payload returned by ExecuteCrawler.
type crawlResult struct {
	Status       string      `json:"status"`
	PagesCrawled int         `json:"pages_crawled"`
	LinksFound   int         `json:"links_found"`
	Pages        []crawlPage `json:"pages,omitempty"`
	Message      string      `json:"message,omitempty"`
}

// crawlerGuardian isolates external content to prevent prompt injection.
var crawlerGuardian = security.NewGuardian(nil)

// ExecuteCrawler performs a multi-page crawl starting from startURL.
//
// maxDepth       – link depth to follow (1-5, default: 2)
// maxPages       – maximum total pages to crawl (1-100, default: 20)
// allowedDomains – comma-separated whitelist; empty = auto-detect from startURL
// selector       – optional CSS selector to extract specific content from each page
func ExecuteCrawler(startURL string, maxDepth, maxPages int, allowedDomains, selector string) string {
	encode := func(r crawlResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	// Validate URL
	if startURL == "" {
		return encode(crawlResult{Status: "error", Message: "url is required"})
	}
	parsed, err := url.ParseRequestURI(startURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return encode(crawlResult{Status: "error", Message: "url must be a valid http or https URL"})
	}

	// Clamp parameters
	if maxDepth <= 0 || maxDepth > 5 {
		maxDepth = 2
	}
	if maxPages <= 0 || maxPages > 100 {
		maxPages = 20
	}

	// Resolve allowed domains
	var domains []string
	if allowedDomains != "" {
		for _, d := range strings.Split(allowedDomains, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				domains = append(domains, d)
			}
		}
	} else {
		domains = []string{parsed.Hostname()}
	}

	// Track pages and links
	var mu sync.Mutex
	var pages []crawlPage
	linkSet := make(map[string]bool)
	pageCount := 0

	if err := security.ValidateSSRF(startURL); err != nil {
		return encode(crawlResult{Status: "error", Message: fmt.Sprintf("URL not allowed: %v", err)})
	}

	c := colly.NewCollector(
		colly.AllowedDomains(domains...),
		colly.MaxDepth(maxDepth),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
	)
	c.Limit(&colly.LimitRule{
		Parallelism: 2,
	})

	// Collect links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := e.Request.AbsoluteURL(e.Attr("href"))
		if href == "" {
			return
		}
		hParsed, err := url.Parse(href)
		if err != nil || (hParsed.Scheme != "http" && hParsed.Scheme != "https") {
			return
		}
		if err := security.ValidateSSRF(href); err != nil {
			return
		}
		mu.Lock()
		linkSet[href] = true
		shouldVisit := pageCount < maxPages
		mu.Unlock()
		if shouldVisit {
			_ = e.Request.Visit(href)
		}
	})

	// Process each page
	c.OnResponse(func(r *colly.Response) {
		mu.Lock()
		if pageCount >= maxPages {
			mu.Unlock()
			return
		}
		pageCount++
		mu.Unlock()

		page := crawlPage{
			URL: r.Request.URL.String(),
		}

		bodyStr := string(r.Body)

		// Extract title
		if idx := strings.Index(bodyStr, "<title"); idx >= 0 {
			if start := strings.Index(bodyStr[idx:], ">"); start >= 0 {
				if end := strings.Index(bodyStr[idx+start:], "</title>"); end >= 0 {
					page.Title = strings.TrimSpace(bodyStr[idx+start+1 : idx+start+end])
				}
			}
		}

		// Extract content
		var text string
		if selector != "" {
			// Use the HTML element directly via a nested collector isn't practical here,
			// so we fall back to full-page text extraction.
			text = extractPlainText(bodyStr)
		} else {
			text = extractPlainText(bodyStr)
		}

		// Limit preview to 500 chars
		if len(text) > 500 {
			text = text[:500] + "…"
		}
		// Isolate external content with Guardian
		page.ContentPreview = crawlerGuardian.ScanExternalContent(r.Request.URL.String(), text)

		mu.Lock()
		pages = append(pages, page)
		mu.Unlock()
	})

	// Start crawl
	if err := c.Visit(startURL); err != nil {
		return encode(crawlResult{Status: "error", Message: fmt.Sprintf("crawl start failed: %v", err)})
	}
	c.Wait()

	return encode(crawlResult{
		Status:       "success",
		PagesCrawled: len(pages),
		LinksFound:   len(linkSet),
		Pages:        pages,
	})
}

// extractPlainText strips HTML tags and collapses whitespace.
func extractPlainText(html string) string {
	// Reuse the package-level regex patterns from scraper.go
	text := reScript.ReplaceAllString(html, " ")
	text = reStyle.ReplaceAllString(text, " ")
	text = reTag.ReplaceAllString(text, " ")
	text = reSpace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}
