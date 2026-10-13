// Package tools – crawler: multi-page site crawler using Colly.
package tools

import (
	"aurago/internal/security"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// crawlPage represents one crawled page in the results.
type crawlPage struct {
	URL            string `json:"url"`
	Title          string `json:"title"`
	ContentPreview string `json:"content_preview"` // first 500 runes of text
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
func ExecuteCrawler(startURL string, maxDepth, maxPages int, allowedDomains, selector string, contexts ...context.Context) string {
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
	c.IgnoreRobotsTxt = false
	c.MaxBodySize = security.MaxSSRFResponseBytes
	c.SetClient(security.HTTPClientWithContext(security.NewSSRFProtectedHTTPClient(20*time.Second), requestContext(contexts)))
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
		shouldVisit := pageCount < maxPages && requestContext(contexts).Err() == nil
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

		var title, preview string
		if len(r.Body) == security.MaxSSRFResponseBytes {
			preview = "page skipped because it exceeds 5 MiB"
		} else {
			bodyStr := string(r.Body)
			if idx := strings.Index(bodyStr, "<title"); idx >= 0 {
				if start := strings.Index(bodyStr[idx:], ">"); start >= 0 {
					if end := strings.Index(bodyStr[idx+start:], "</title>"); end >= 0 {
						title = strings.TrimSpace(bodyStr[idx+start+1 : idx+start+end])
					}
				}
			}
			if selector != "" {
				preview = extractSelectorText(bodyStr, selector)
			} else {
				preview = extractPlainText(bodyStr)
			}
			preview = cutPreviewRunes(preview, 500)
		}
		source := r.Request.URL.String()
		page.Title = crawlerGuardian.ScanExternalContent(source, title)
		page.ContentPreview = crawlerGuardian.ScanExternalContent(source, preview)

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

func extractSelectorText(html, selector string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	var parts []string
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		if text := strings.TrimSpace(s.Text()); text != "" {
			parts = append(parts, text)
		}
	})
	return strings.Join(parts, " ")
}

func cutPreviewRunes(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
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
