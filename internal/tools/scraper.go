package tools

import (
	"aurago/internal/scraper"
	"aurago/internal/security"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"
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

type WebScraperOptions struct {
	Mode            string
	WaitForSelector string
}

type webScraperRSSItem struct {
	Title       string `json:"title,omitempty"`
	Link        string `json:"link,omitempty"`
	Description string `json:"description,omitempty"`
	Published   string `json:"published,omitempty"`
	GUID        string `json:"guid,omitempty"`
}

// ExecuteWebScraper fetches a URL using colly + go-readability, converts the main
// article content to Markdown, then returns a compact JSON result.
// SSRF protection, prompt-injection scanning, and <external_data> isolation are
// all handled by the underlying AgentScraper.
func ExecuteWebScraper(rawURL string) string {
	return ExecuteWebScraperWithOptions(rawURL, WebScraperOptions{Mode: "auto"})
}

func ExecuteWebScraperWithOptions(rawURL string, options WebScraperOptions) string {
	mode := strings.ToLower(strings.TrimSpace(options.Mode))
	if mode == "" {
		mode = "auto"
	}
	switch mode {
	case "rss":
		return executeWebScraperRSS(rawURL)
	case "dynamic":
		return executeWebScraperDynamic(rawURL, options.WaitForSelector)
	case "static":
		return executeWebScraperStatic(rawURL, "static")
	case "auto":
		return executeWebScraperAuto(rawURL, options.WaitForSelector)
	default:
		return formatError("invalid web_scraper mode: use auto, static, dynamic, or rss")
	}
}

func executeWebScraperAuto(rawURL, waitForSelector string) string {
	if looksLikeFeedURL(rawURL) {
		if rss := executeWebScraperRSS(rawURL); !strings.Contains(rss, `"status":"error"`) {
			return rss
		}
	}
	s := scraper.New(scraperGuardian)
	result, err := s.FetchStatic(rawURL)
	if err != nil {
		return formatError(fmt.Sprintf("scrape failed: %v", err))
	}
	if scrapeResultLooksThin(result) {
		dynamic, dynErr := s.FetchDynamic(rawURL, waitForSelector)
		if dynErr == nil && !scrapeResultLooksThin(dynamic) {
			return formatScrapeResult("dynamic", dynamic, "")
		}
		if dynErr != nil {
			return formatScrapeResult("static", result, fmt.Sprintf("Dynamic fallback failed: %v", dynErr))
		}
	}
	return formatScrapeResult("static", result, "")
}

func executeWebScraperStatic(rawURL, mode string) string {
	s := scraper.New(scraperGuardian)
	result, err := s.FetchStatic(rawURL)
	if err != nil {
		return formatError(fmt.Sprintf("scrape failed: %v", err))
	}
	return formatScrapeResult(mode, result, "")
}

func executeWebScraperDynamic(rawURL, waitForSelector string) string {
	s := scraper.New(scraperGuardian)
	result, err := s.FetchDynamic(rawURL, waitForSelector)
	if err != nil {
		return formatError(fmt.Sprintf("dynamic scrape failed: %v", err))
	}
	return formatScrapeResult("dynamic", result, "")
}

func formatScrapeResult(mode string, result *scraper.ScrapeResult, warning string) string {
	out := map[string]interface{}{
		"status":  "success",
		"mode":    mode,
		"title":   result.Title,
		"content": result.Markdown,
	}
	if len(result.Links) > 0 {
		out["links"] = result.Links
	}
	if warning != "" {
		out["warning"] = warning
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func executeWebScraperRSS(rawURL string) string {
	feed, err := fetchRSSFeed(rawURL)
	if err != nil {
		return formatError(fmt.Sprintf("rss scrape failed: %v", err))
	}
	out := map[string]interface{}{
		"status":  "success",
		"mode":    "rss",
		"title":   feed.Title,
		"content": rssItemsContent(feed.Items),
		"items":   feed.Items,
	}
	b, _ := json.Marshal(out)
	return string(b)
}

type parsedRSSFeed struct {
	Title string
	Items []webScraperRSSItem
}

type rssXML struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Title string       `xml:"title"`
		Items []rssXMLItem `xml:"item"`
	} `xml:"channel"`
}

type rssXMLItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

type atomXML struct {
	XMLName xml.Name       `xml:"feed"`
	Title   string         `xml:"title"`
	Entries []atomXMLEntry `xml:"entry"`
}

type atomXMLEntry struct {
	Title   string        `xml:"title"`
	Summary string        `xml:"summary"`
	Content string        `xml:"content"`
	Updated string        `xml:"updated"`
	ID      string        `xml:"id"`
	Links   []atomXMLLink `xml:"link"`
}

type atomXMLLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

func fetchRSSFeed(rawURL string) (parsedRSSFeed, error) {
	if err := security.ValidateSSRF(rawURL); err != nil {
		return parsedRSSFeed{}, fmt.Errorf("URL not allowed: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return parsedRSSFeed{}, err
	}
	req.Header.Set("User-Agent", "AuraGo/1.0 (+https://github.com/antibyte/AuraGo)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.5")
	resp, err := scraperHTTPClient.Do(req)
	if err != nil {
		return parsedRSSFeed{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parsedRSSFeed{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return parsedRSSFeed{}, err
	}
	return parseRSSOrAtom(body)
}

func parseRSSOrAtom(body []byte) (parsedRSSFeed, error) {
	var rss rssXML
	if err := xml.Unmarshal(body, &rss); err == nil && strings.EqualFold(rss.XMLName.Local, "rss") {
		items := make([]webScraperRSSItem, 0, len(rss.Channel.Items))
		for _, item := range rss.Channel.Items {
			items = append(items, webScraperRSSItem{
				Title:       strings.TrimSpace(item.Title),
				Link:        strings.TrimSpace(item.Link),
				Description: strings.TrimSpace(item.Description),
				Published:   strings.TrimSpace(item.PubDate),
				GUID:        strings.TrimSpace(item.GUID),
			})
		}
		return parsedRSSFeed{Title: strings.TrimSpace(rss.Channel.Title), Items: items}, nil
	}
	var atom atomXML
	if err := xml.Unmarshal(body, &atom); err == nil && strings.EqualFold(atom.XMLName.Local, "feed") {
		items := make([]webScraperRSSItem, 0, len(atom.Entries))
		for _, entry := range atom.Entries {
			items = append(items, webScraperRSSItem{
				Title:       strings.TrimSpace(entry.Title),
				Link:        atomEntryLink(entry),
				Description: firstNonEmpty(strings.TrimSpace(entry.Summary), strings.TrimSpace(entry.Content)),
				Published:   strings.TrimSpace(entry.Updated),
				GUID:        strings.TrimSpace(entry.ID),
			})
		}
		return parsedRSSFeed{Title: strings.TrimSpace(atom.Title), Items: items}, nil
	}
	return parsedRSSFeed{}, fmt.Errorf("unsupported RSS/Atom XML")
}

func atomEntryLink(entry atomXMLEntry) string {
	for _, link := range entry.Links {
		if strings.TrimSpace(link.Href) == "" {
			continue
		}
		if link.Rel == "" || link.Rel == "alternate" {
			return strings.TrimSpace(link.Href)
		}
	}
	if len(entry.Links) > 0 {
		return strings.TrimSpace(entry.Links[0].Href)
	}
	return ""
}

func rssItemsContent(items []webScraperRSSItem) string {
	var sb strings.Builder
	for _, item := range items {
		if item.Title != "" {
			sb.WriteString("- ")
			sb.WriteString(item.Title)
		}
		if item.Link != "" {
			sb.WriteString(" ")
			sb.WriteString(item.Link)
		}
		if item.Description != "" {
			sb.WriteString(": ")
			sb.WriteString(item.Description)
		}
		if item.Title != "" || item.Link != "" || item.Description != "" {
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String())
}

func looksLikeFeedURL(rawURL string) bool {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	base := path.Base(u)
	return strings.HasSuffix(base, ".xml") ||
		strings.HasSuffix(base, ".rss") ||
		strings.Contains(u, "/rss") ||
		strings.Contains(u, "/atom") ||
		strings.Contains(u, "feed")
}

func scrapeResultLooksThin(result *scraper.ScrapeResult) bool {
	if result == nil {
		return true
	}
	content := strings.TrimSpace(result.Markdown)
	raw := strings.ToLower(result.RawHTML)
	return len(content) < 200 && strings.Contains(raw, "<script")
}

func formatError(msg string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"status":  "error",
		"message": msg,
	})
	return string(b)
}
