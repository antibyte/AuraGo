package tools

import (
	"aurago/internal/scraper"
	"aurago/internal/security"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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
	Selector        string
	Fields          map[string]string
	OutputFormat    string
	Attribute       string
	Limit           int
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

	hasSelector := strings.TrimSpace(options.Selector) != ""

	switch mode {
	case "rss":
		if hasSelector {
			return formatError("selector is not supported in rss mode")
		}
		return executeWebScraperRSS(rawURL)
	case "dynamic":
		if hasSelector {
			return executeWebScraperStructured(rawURL, options, true)
		}
		return executeWebScraperDynamic(rawURL, options.WaitForSelector)
	case "static":
		if hasSelector {
			return executeWebScraperStructured(rawURL, options, false)
		}
		return executeWebScraperStatic(rawURL, "static")
	case "auto":
		if hasSelector {
			return executeWebScraperStructuredAuto(rawURL, options)
		}
		return executeWebScraperAuto(rawURL, options.WaitForSelector)
	default:
		return formatError("invalid web_scraper mode: use auto, static, dynamic, or rss")
	}
}

func executeWebScraperAuto(rawURL, waitForSelector string) string {
	if rss, ok := executeWebScraperRSSAuto(rawURL); ok {
		return rss
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
	feed, err := fetchRSSFeed(rawURL, false)
	if err != nil {
		return formatError(fmt.Sprintf("rss scrape failed: %v", err))
	}
	return formatRSSFeedResult(rawURL, feed)
}

func executeWebScraperRSSAuto(rawURL string) (string, bool) {
	feed, err := fetchRSSFeed(rawURL, !looksLikeFeedURL(rawURL))
	if err != nil {
		return "", false
	}
	return formatRSSFeedResult(rawURL, feed), true
}

func formatRSSFeedResult(rawURL string, feed parsedRSSFeed) string {
	safeFeed := isolateRSSFeedText(rawURL, feed)
	out := map[string]interface{}{
		"status":  "success",
		"mode":    "rss",
		"title":   safeFeed.Title,
		"content": scraperGuardian.ScanExternalContent(rawURL, rssItemsContent(feed.Items)),
		"items":   safeFeed.Items,
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func isolateRSSFeedText(rawURL string, feed parsedRSSFeed) parsedRSSFeed {
	safe := parsedRSSFeed{
		Title: isolateRSSField(rawURL, feed.Title),
		Items: make([]webScraperRSSItem, 0, len(feed.Items)),
	}
	for _, item := range feed.Items {
		item.Title = isolateRSSField(rawURL, item.Title)
		item.Description = isolateRSSField(rawURL, item.Description)
		safe.Items = append(safe.Items, item)
	}
	return safe
}

func isolateRSSField(rawURL, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return scraperGuardian.ScanExternalContent(rawURL, value)
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

func fetchRSSFeed(rawURL string, requireFeedResponse bool) (parsedRSSFeed, error) {
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
	if requireFeedResponse && !rssResponseLooksLikeFeed(resp.Header.Get("Content-Type"), body) {
		return parsedRSSFeed{}, fmt.Errorf("response is not an RSS/Atom feed")
	}
	return parseRSSOrAtom(body)
}

func rssResponseLooksLikeFeed(contentType string, body []byte) bool {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch {
	case ct == "application/rss+xml",
		ct == "application/atom+xml",
		ct == "application/xml",
		ct == "text/xml",
		strings.HasSuffix(ct, "+xml"):
		return true
	}

	trimmed := bytes.TrimSpace(body)
	if bytes.HasPrefix(trimmed, []byte{0xef, 0xbb, 0xbf}) {
		trimmed = bytes.TrimSpace(trimmed[3:])
	}
	lower := strings.ToLower(string(trimmed))
	if strings.HasPrefix(lower, "<?xml") {
		if idx := strings.Index(lower, "?>"); idx >= 0 {
			lower = strings.TrimSpace(lower[idx+2:])
		}
	}
	return strings.HasPrefix(lower, "<rss") || strings.HasPrefix(lower, "<feed")
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

func executeWebScraperStructured(rawURL string, options WebScraperOptions, dynamic bool) string {
	s := scraper.New(scraperGuardian)
	var result *scraper.ScrapeResult
	var err error
	if dynamic {
		result, err = s.FetchDynamic(rawURL, options.WaitForSelector)
	} else {
		result, err = s.FetchStatic(rawURL)
	}
	if err != nil {
		return formatError(fmt.Sprintf("scrape failed: %v", err))
	}
	mode := "static"
	if dynamic {
		mode = "dynamic"
	}
	return extractStructuredHTML(result.RawHTML, rawURL, options, mode)
}

func executeWebScraperStructuredAuto(rawURL string, options WebScraperOptions) string {
	s := scraper.New(scraperGuardian)
	result, err := s.FetchStatic(rawURL)
	if err != nil {
		return formatError(fmt.Sprintf("scrape failed: %v", err))
	}
	if scrapeResultLooksThin(result) {
		dynamic, dynErr := s.FetchDynamic(rawURL, options.WaitForSelector)
		if dynErr == nil && !scrapeResultLooksThin(dynamic) {
			return extractStructuredHTML(dynamic.RawHTML, rawURL, options, "dynamic")
		}
		if dynErr != nil {
			structured := extractStructuredHTML(result.RawHTML, rawURL, options, "static")
			return withWarning(structured, fmt.Sprintf("Dynamic fallback failed: %v", dynErr))
		}
	}
	return extractStructuredHTML(result.RawHTML, rawURL, options, "static")
}

func extractStructuredHTML(rawHTML, rawURL string, options WebScraperOptions, mode string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return formatError(fmt.Sprintf("failed to parse HTML: %v", err))
	}

	selector := strings.TrimSpace(options.Selector)
	outputFormat := strings.ToLower(strings.TrimSpace(options.OutputFormat))
	if outputFormat == "" {
		outputFormat = "auto"
	}

	switch outputFormat {
	case "auto":
		if len(options.Fields) > 0 {
			outputFormat = "rows"
		} else if strings.TrimSpace(options.Attribute) != "" {
			outputFormat = "list"
		} else {
			outputFormat = "text"
		}
	}

	limit := options.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	out := map[string]interface{}{
		"status":        "success",
		"mode":          mode,
		"selector":      selector,
		"output_format": outputFormat,
	}

	switch outputFormat {
	case "text":
		matches := isolateStrings(rawURL, extractTextList(doc, selector, limit))
		out["count"] = len(matches)
		out["matches"] = matches
	case "html":
		matches := isolateStrings(rawURL, extractHTMLList(doc, selector, limit))
		out["count"] = len(matches)
		out["matches"] = matches
	case "list":
		attribute := strings.TrimSpace(options.Attribute)
		matches := isolateStrings(rawURL, extractAttributeList(doc, selector, attribute, limit))
		out["count"] = len(matches)
		out["attribute"] = attribute
		out["matches"] = matches
	case "rows":
		fields := normalizeFields(options.Fields)
		matches := isolateRows(rawURL, extractRows(doc, selector, fields, limit))
		out["count"] = len(matches)
		out["fields"] = fieldKeys(fields)
		out["matches"] = matches
	case "table":
		headers, rows := extractTable(doc, selector, limit)
		out["count"] = len(rows)
		out["headers"] = isolateStrings(rawURL, headers)
		out["rows"] = isolateTableRows(rawURL, rows)
	default:
		return formatError(fmt.Sprintf("unsupported output_format: %s", outputFormat))
	}

	b, _ := json.Marshal(out)
	return string(b)
}

func extractTextList(doc *goquery.Document, selector string, limit int) []string {
	var out []string
	doc.Find(selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(out) >= limit {
			return false
		}
		out = append(out, strings.TrimSpace(s.Text()))
		return true
	})
	return out
}

func extractHTMLList(doc *goquery.Document, selector string, limit int) []string {
	var out []string
	doc.Find(selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(out) >= limit {
			return false
		}
		html, _ := s.Html()
		out = append(out, strings.TrimSpace(html))
		return true
	})
	return out
}

func extractAttributeList(doc *goquery.Document, selector, attribute string, limit int) []string {
	var out []string
	doc.Find(selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(out) >= limit {
			return false
		}
		val, _ := s.Attr(attribute)
		out = append(out, val)
		return true
	})
	return out
}

func extractRows(doc *goquery.Document, selector string, fields map[string]string, limit int) []map[string]string {
	var out []map[string]string
	doc.Find(selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(out) >= limit {
			return false
		}
		row := make(map[string]string, len(fields))
		for name, fieldSelector := range fields {
			sel, attr := parseFieldSelector(fieldSelector)
			var val string
			if sel == "" {
				if attr != "" {
					val, _ = s.Attr(attr)
				} else {
					val = strings.TrimSpace(s.Text())
				}
			} else {
				found := s.Find(sel).First()
				if attr != "" {
					val, _ = found.Attr(attr)
				} else {
					val = strings.TrimSpace(found.Text())
				}
			}
			row[name] = val
		}
		out = append(out, row)
		return true
	})
	return out
}

func extractTable(doc *goquery.Document, selector string, limit int) ([]string, [][]string) {
	table := doc.Find(selector).First()

	var headers []string
	table.Find("thead tr th").Each(func(i int, s *goquery.Selection) {
		headers = append(headers, strings.TrimSpace(s.Text()))
	})

	var rows [][]string
	rowSelector := "tbody tr"
	if table.Find(rowSelector).Length() == 0 {
		rowSelector = "tr"
	}

	table.Find(rowSelector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if len(rows) >= limit {
			return false
		}
		var cells []string
		s.Find("th, td").Each(func(j int, c *goquery.Selection) {
			cells = append(cells, strings.TrimSpace(c.Text()))
		})
		if len(cells) == 0 {
			return true
		}
		if len(headers) == 0 && i == 0 {
			headers = cells
			return true
		}
		rows = append(rows, cells)
		return true
	})

	return headers, rows
}

func parseFieldSelector(field string) (selector, attribute string) {
	if idx := strings.LastIndex(field, "@"); idx >= 0 {
		return strings.TrimSpace(field[:idx]), strings.TrimSpace(field[idx+1:])
	}
	return strings.TrimSpace(field), ""
}

func normalizeFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for k, v := range fields {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	return out
}

func fieldKeys(fields map[string]string) []string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func withWarning(resultJSON, warning string) string {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &payload); err != nil {
		return resultJSON
	}
	payload["warning"] = warning
	b, _ := json.Marshal(payload)
	return string(b)
}

func isolateValue(rawURL, value string) string {
	if value == "" {
		return ""
	}
	return scraperGuardian.ScanExternalContent(rawURL, value)
}

func isolateStrings(rawURL string, values []string) []string {
	for i, v := range values {
		values[i] = isolateValue(rawURL, v)
	}
	return values
}

func isolateRows(rawURL string, rows []map[string]string) []map[string]string {
	for i, row := range rows {
		for k, v := range row {
			row[k] = isolateValue(rawURL, v)
		}
		rows[i] = row
	}
	return rows
}

func isolateTableRows(rawURL string, rows [][]string) [][]string {
	for i, row := range rows {
		rows[i] = isolateStrings(rawURL, row)
	}
	return rows
}

func formatError(msg string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"status":  "error",
		"message": msg,
	})
	return string(b)
}
