package server

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"aurago/internal/scraper"
	"aurago/internal/security"

	"github.com/PuerkitoBio/goquery"
	"github.com/itlightning/dateparse"
	"golang.org/x/net/publicsuffix"
)

func newspaperUnwrap(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "<external_data>") && strings.HasSuffix(text, "</external_data>") {
		body := strings.TrimSuffix(strings.TrimPrefix(text, "<external_data>"), "</external_data>")
		body, _ = security.IsolatedPayload(strings.TrimSpace(body))
		return body
	}
	return text
}

func newspaperPublisherKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if domain, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return domain
	}
	return host
}

type newspaperArticleMetadata struct {
	Title, Publisher, Published string
	Article                     bool
}

func newspaperMetadata(rawHTML string) newspaperArticleMetadata {
	meta := newspaperArticleMetadata{}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return meta
	}
	meta.Published, _ = doc.Find("meta[property='article:published_time'],meta[name='date'],meta[itemprop='datePublished']").First().Attr("content")
	meta.Publisher, _ = doc.Find("meta[property='og:site_name']").First().Attr("content")
	typ, _ := doc.Find("meta[property='og:type']").First().Attr("content")
	meta.Article = typ == "article" || meta.Published != ""
	var walk func(any, int)
	walk = func(value any, depth int) {
		if depth > 8 {
			return
		}
		switch v := value.(type) {
		case map[string]any:
			typ, _ := json.Marshal(v["@type"])
			if strings.Contains(strings.ToLower(string(typ)), "article") {
				meta.Article = true
				if meta.Title == "" {
					meta.Title, _ = v["headline"].(string)
				}
				if meta.Published == "" {
					meta.Published, _ = v["datePublished"].(string)
				}
				if publisher, ok := v["publisher"].(map[string]any); ok && meta.Publisher == "" {
					meta.Publisher, _ = publisher["name"].(string)
				}
			}
			for _, child := range v {
				walk(child, depth+1)
			}
		case []any:
			for _, child := range v {
				walk(child, depth+1)
			}
		}
	}
	doc.Find("script[type='application/ld+json']").Each(func(i int, selection *goquery.Selection) {
		text := selection.Text()
		if i >= 8 || len(text) > 64*1024 {
			return
		}
		var value any
		if json.Unmarshal([]byte(text), &value) == nil {
			walk(value, 0)
		}
	})
	if meta.Published == "" {
		meta.Published, _ = doc.Find("time[datetime]").First().Attr("datetime")
	}
	return meta
}

func newspaperDate(raw string) *time.Time {
	if at, err := dateparse.ParseAny(raw); err == nil && !at.IsZero() {
		at = at.UTC()
		return &at
	}
	return nil
}

func newspaperTerms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	seen := map[string]bool{}
	terms := []string{}
	for _, word := range words {
		if len([]rune(word)) < 2 || seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, word)
	}
	return terms
}

// Select whole, verbatim source passages; order remains the article's order.
// Isolation is applied to the complete model payload, never byte-truncated tags.
func newspaperEvidenceExcerpt(markdown, topic string) string {
	text := newspaperUnwrap(markdown)
	if len([]rune(text)) <= 5600 {
		return text
	}
	blocks := strings.Split(text, "\n\n")
	if len(blocks) == 1 {
		return newspaperBound(text, 5600)
	}
	type ranked struct {
		index, score int
		text         string
	}
	items := make([]ranked, 0, min(len(blocks), 1000))
	terms := newspaperTerms(topic)
	for i, block := range blocks {
		if i >= 1000 {
			break
		}
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		score := 0
		if i < 2 {
			score += 3
		}
		lower := strings.ToLower(block)
		for _, term := range terms {
			if strings.Contains(lower, term) {
				score += 2
			}
		}
		items = append(items, ranked{index: i, score: score, text: block})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	selected := []ranked{}
	remaining := 5600
	for _, item := range items {
		if remaining < 80 {
			break
		}
		if len([]rune(item.text))+2 > remaining {
			item.text = newspaperBound(item.text, remaining-2)
		}
		selected = append(selected, item)
		remaining -= len([]rune(item.text)) + 2
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].index < selected[j].index })
	parts := make([]string, len(selected))
	for i, item := range selected {
		parts[i] = item.text
	}
	return strings.Join(parts, "\n\n")
}

// Overview pages can contribute one bounded layer of leads, never evidence.
func newspaperOverviewLinks(rawURL string, page *scraper.ScrapeResult, meta newspaperArticleMetadata) []newspaperHit {
	base, err := url.Parse(rawURL)
	if err != nil || meta.Article {
		return nil
	}
	path := strings.Trim(base.Path, "/")
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(page.RawHTML))
	if err != nil {
		return nil
	}
	// Longer index pages can contain substantial snippets. Repeated linked
	// headlines identify them even when their total text exceeds a short article.
	linkedHeadlines := doc.Find("main h2 a[href],main h3 a[href]").Length()
	if path != "" && path != "news" && path != "nachrichten" && linkedHeadlines < 4 && !(len(page.Links) >= 8 && len([]rune(newspaperUnwrap(page.Markdown))) < 800) {
		return nil
	}
	hits := []newspaperHit{}
	seen := map[string]bool{}
	doc.Find("main a[href],article a[href],h2 a[href],h3 a[href]").Each(func(_ int, a *goquery.Selection) {
		if len(hits) >= 10 {
			return
		}
		title := strings.TrimSpace(a.Text())
		if len([]rune(title)) < 12 {
			return
		}
		href, _ := a.Attr("href")
		relative, err := url.Parse(href)
		if err != nil {
			return
		}
		link, err := canonicalNewspaperURL(base.ResolveReference(relative).String())
		if err != nil || link == rawURL || seen[link] {
			return
		}
		seen[link] = true
		hits = append(hits, newspaperHit{Title: newspaperBound(title, 180), URL: link})
	})
	return hits
}
