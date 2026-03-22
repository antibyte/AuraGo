// Package scraper provides static and dynamic web scraping for LLM agent tools.
// Static fetching uses github.com/gocolly/colly/v2 (fast, no JS).
// Dynamic fetching uses github.com/go-rod/rod (headless Chromium, JS rendering).
// Both paths clean and convert the result to Markdown via go-readability + html-to-markdown.
// All Markdown output is scanned for prompt injection and wrapped in <external_data> isolation
// tags before being returned to the caller, consistent with the rest of the Guardian framework.
package scraper

import (
	"aurago/internal/security"
	"fmt"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	readability "github.com/go-shiori/go-readability"
	"github.com/gocolly/colly/v2"
)

// ── Types ────────────────────────────────────────────────────────────────────

// ScrapeResult holds the structured output of a scrape operation.
type ScrapeResult struct {
	Title    string   // Page title extracted by readability
	Markdown string   // Main article content converted to Markdown
	RawHTML  string   // Full raw HTML as received from the server
	Links    []string // All absolute href links found on the page
}

// Scraper is the interface that both static and dynamic scrapers satisfy.
type Scraper interface {
	// FetchStatic retrieves a page without executing JavaScript.
	FetchStatic(url string) (*ScrapeResult, error)

	// FetchDynamic retrieves a page using a headless browser so JavaScript runs.
	// If waitForSelector is non-empty, the browser waits until the selector
	// appears in the DOM before capturing the HTML.
	FetchDynamic(url string, waitForSelector string) (*ScrapeResult, error)
}

// ── AgentScraper ─────────────────────────────────────────────────────────────

// AgentScraper is the production implementation of Scraper.
// If guardian is non-nil, all Markdown output is scanned for prompt injection
// patterns and wrapped in <external_data> isolation tags before being returned.
type AgentScraper struct {
	guardian *security.Guardian // nil = no scanning (e.g. unit tests)
}

// New returns a ready-to-use AgentScraper.
// Pass a *security.Guardian to enable prompt-injection scanning and output isolation.
// Pass nil to skip scanning (useful in tests).
func New(g *security.Guardian) *AgentScraper {
	return &AgentScraper{guardian: g}
}

// FetchStatic fetches a URL using colly (no JavaScript) and returns a ScrapeResult.
func (a *AgentScraper) FetchStatic(rawURL string) (*ScrapeResult, error) {
	result := &ScrapeResult{}
	var rawHTML strings.Builder
	var links []string

	if err := security.ValidateSSRF(rawURL); err != nil {
		return nil, fmt.Errorf("URL not allowed: %w", err)
	}

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
		colly.MaxDepth(1),
	)
	c.SetRequestTimeout(20 * time.Second)

	// Capture the full HTML response body.
	c.OnResponse(func(r *colly.Response) {
		rawHTML.Write(r.Body)
	})

	// Collect all anchor href values and resolve them to absolute URLs.
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		abs := e.Request.AbsoluteURL(href)
		if abs != "" {
			links = append(links, abs)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		// Error is surfaced via c.Visit return below.
	})

	if err := c.Visit(rawURL); err != nil {
		return nil, fmt.Errorf("colly visit %s: %w", rawURL, err)
	}

	result.RawHTML = rawHTML.String()
	result.Links = dedupLinks(links)

	// Enrich with readability + markdown conversion, then scan and isolate.
	if err := processForLLM(result, rawURL, a.guardian); err != nil {
		// Non-fatal: return raw content with a fallback title.
		result.Title = rawURL
	}

	return result, nil
}

// FetchDynamic launches a headless Chromium instance, navigates to the URL, optionally
// waits for a CSS selector, then captures the fully-rendered HTML.
func (a *AgentScraper) FetchDynamic(rawURL string, waitForSelector string) (*ScrapeResult, error) {
	result := &ScrapeResult{}

	if err := security.ValidateSSRF(rawURL); err != nil {
		return nil, fmt.Errorf("URL not allowed: %w", err)
	}

	// Launch a headless browser; prefer a system Chrome/Chromium installation,
	// fall back to rod's auto-download mechanism.
	u, err := launcher.New().
		Headless(true).
		NoSandbox(true). // Required in containerised / CI environments.
		Launch()
	if err != nil {
		return nil, fmt.Errorf("rod launcher: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("rod connect: %w", err)
	}
	defer browser.MustClose()

	page, err := browser.Page(proto.TargetCreateTarget{URL: rawURL})
	if err != nil {
		return nil, fmt.Errorf("rod open page %s: %w", rawURL, err)
	}

	// Wait for the page to reach a network-idle state.
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("rod wait load: %w", err)
	}

	// If a specific selector is requested, wait until it is visible.
	if waitForSelector != "" {
		if err := page.WaitElementsMoreThan(waitForSelector, 0); err != nil {
			// Log but don't abort — page may still be usable.
			_ = fmt.Errorf("rod wait selector %q: %w", waitForSelector, err)
		}
	}

	// Collect all <a href> links via JavaScript evaluation.
	rawLinks, err := page.Eval(`() => Array.from(document.querySelectorAll('a[href]')).map(a => a.href)`)
	if err == nil {
		for _, v := range rawLinks.Value.Arr() {
			if href := v.Str(); href != "" {
				result.Links = append(result.Links, href)
			}
		}
		result.Links = dedupLinks(result.Links)
	}

	// Capture the fully-rendered outer HTML.
	html, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("rod get HTML: %w", err)
	}
	result.RawHTML = html

	if err := processForLLM(result, rawURL, a.guardian); err != nil {
		result.Title = rawURL
	}

	return result, nil
}

// ── processForLLM ─────────────────────────────────────────────────────────────

// processForLLM extracts the main article from rawHTML using go-readability,
// converts it to Markdown, then scans for prompt injection and wraps the output
// in <external_data> isolation tags. This mirrors the Guardian.ScanExternalContent
// contract used throughout the agent framework: third-party content is always
// isolated so the LLM cannot be tricked into treating it as trusted instructions.
func processForLLM(result *ScrapeResult, rawURL string, g *security.Guardian) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	article, err := readability.FromReader(
		strings.NewReader(result.RawHTML),
		parsedURL,
	)
	if err != nil {
		return fmt.Errorf("readability: %w", err)
	}

	result.Title = article.Title

	// Convert the readability-cleaned HTML to Markdown.
	var md string
	converted, err := htmltomarkdown.ConvertString(article.Content)
	if err != nil {
		// Fall back to plain text excerpt if conversion fails.
		md = article.TextContent
	} else {
		md = strings.TrimSpace(converted)
	}

	// Scan and isolate: web content is inherently untrusted and may contain
	// deliberate prompt injection ("ignore previous instructions", role spoofing, etc.).
	// ScanExternalContent always wraps in <external_data> tags and logs any threats.
	if g != nil {
		result.Markdown = g.ScanExternalContent(rawURL, md)
	} else {
		// No guardian provided (e.g. unit tests): still isolate, just don't scan.
		result.Markdown = security.IsolateExternalData(md)
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// dedupLinks removes duplicate URLs while preserving insertion order.
func dedupLinks(links []string) []string {
	seen := make(map[string]struct{}, len(links))
	out := make([]string, 0, len(links))
	for _, l := range links {
		if _, ok := seen[l]; !ok {
			seen[l] = struct{}{}
			out = append(out, l)
		}
	}
	return out
}
