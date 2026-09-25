package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"aurago/internal/llm"
	"aurago/internal/newspaper"
	"aurago/internal/scraper"
	"aurago/internal/tools"

	"github.com/PuerkitoBio/goquery"
	"github.com/itlightning/dateparse"
	openai "github.com/sashabaranov/go-openai"
)

type newspaperHit struct{ Title, URL, Published string }
type newspaperQuery struct{ Section, Text string }

func newspaperQueries(p newspaper.Profile, date string) []newspaperQuery {
	terms := map[string]string{"regional": "regional news", "national": "national news", "international": "world news", "politics": "politics news", "economy": "economy business news", "culture": "culture arts news", "technology": "technology news", "science": "science research news", "environment": "environment climate news", "health": "health medicine news", "sport": "sport news"}
	if strings.HasPrefix(p.Language, "de") {
		terms = map[string]string{"regional": "regionale Nachrichten", "national": "Deutschland Nachrichten", "international": "internationale Nachrichten", "politics": "Politik Nachrichten", "economy": "Wirtschaft Nachrichten", "culture": "Kultur Nachrichten", "technology": "Technik Nachrichten", "science": "Wissenschaft Nachrichten", "environment": "Umwelt Klima Nachrichten", "health": "Gesundheit Medizin Nachrichten", "sport": "Sport Nachrichten"}
	}
	queries := []newspaperQuery{}
	for _, section := range p.Sections {
		place := p.Country
		if section == "regional" {
			place = strings.TrimSpace(p.City + " " + p.Region + " " + p.Country)
		}
		queries = append(queries, newspaperQuery{Section: section, Text: strings.TrimSpace(terms[section] + " " + place + " " + date)})
	}
	for _, interest := range p.Interests {
		queries = append(queries, newspaperQuery{Section: "interests", Text: interest + " latest news " + p.Language + " " + date})
	}
	if len(queries) > 24 {
		queries = queries[:24]
	}
	return queries
}

func newspaperCandidateOrder(p newspaper.Profile, queries []newspaperQuery, searchQueries int) []int {
	buckets := make(map[string][]int, len(p.Sections)+1)
	for i := searchQueries; i < len(queries); i++ {
		buckets[queries[i].Section] = append(buckets[queries[i].Section], i)
	}
	for i := 0; i < searchQueries; i++ {
		buckets[queries[i].Section] = append(buckets[queries[i].Section], i)
	}
	sections := append([]string(nil), p.Sections...)
	if len(p.Interests) > 0 {
		sections = append(sections, "interests")
	}
	order := make([]int, 0, len(queries))
	for round := 0; len(order) < len(queries); round++ {
		before := len(order)
		for _, section := range sections {
			if bucket := buckets[section]; round < len(bucket) {
				order = append(order, bucket[round])
			}
		}
		if len(order) == before {
			break
		}
	}
	return order
}

func canonicalNewspaperURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return "", errors.New("invalid public URL")
	}
	u.Fragment = ""
	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || lower == "fbclid" || lower == "gclid" {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func newspaperPublished(raw, html string) *time.Time {
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(html)); err == nil {
		value, _ := doc.Find("meta[property='article:published_time'],meta[name='date'],meta[itemprop='datePublished']").First().Attr("content")
		if value == "" {
			value, _ = doc.Find("time[datetime]").First().Attr("datetime")
		}
		if value != "" {
			raw = value
		}
	}
	if at, err := dateparse.ParseAny(raw); err == nil && !at.IsZero() {
		v := at.UTC()
		return &v
	}
	return nil
}

func newspaperBound(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > maxRunes {
		r = r[:maxRunes]
	}
	return string(r)
}

func newspaperExcluded(text string, terms []string) bool {
	text = strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(term))) {
			return true
		}
	}
	return false
}

func (s *Server) newspaperResearch(ctx context.Context, p newspaper.Profile, cutoff time.Time, progress func(newspaper.Progress)) (newspaper.Draft, error) {
	result := newspaper.Draft{Stories: []newspaper.Story{}, Sources: []newspaper.Source{}}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !s.newspaperSkillReady || !cfg.Newspaper.Enabled || cfg.Newspaper.ReadOnly || !cfg.VirtualDesktop.Enabled || cfg.VirtualDesktop.ReadOnly || !cfg.Agent.AllowNetworkRequests || !cfg.Tools.WebScraper.Enabled || s.LLMClient == nil || cfg.LLM.Model == "" {
		return result, errors.New("research needs an enabled model, network access and page reading")
	}
	searchReady := cfg.BraveSearch.Enabled && cfg.BraveSearch.APIKey != ""
	if !searchReady && len(p.RSSFeeds) == 0 {
		return result, errors.New("research needs Brave Search or a curated RSS feed")
	}
	if s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("newspaper") {
		return result, errors.New("provider spending policy blocks research")
	}
	loc, _ := time.LoadLocation(p.TimeZone)
	date := cutoff.In(loc).Format("2006-01-02")
	queries := newspaperQueries(p, date)
	searchQueries := len(queries)
	lists := make([][]newspaperHit, searchQueries)
	covered := make([]bool, searchQueries)
	progress(newspaper.Progress{Phase: "finding", Message: "Finding current source pages"})
	for i, q := range queries {
		if !searchReady {
			break
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		latest := s.ConfigSnapshot()
		if latest == nil || !latest.Newspaper.Enabled || latest.Newspaper.ReadOnly || !latest.Agent.AllowNetworkRequests || !latest.BraveSearch.Enabled {
			return result, errors.New("research permission revoked")
		}
		raw := tools.ExecuteBraveSearch(latest.BraveSearch.APIKey, q.Text, 4, p.Country, p.Language, ctx)
		var response struct {
			Status  string         `json:"status"`
			Results []newspaperHit `json:"results"`
		}
		if json.Unmarshal([]byte(raw), &response) == nil && response.Status == "success" {
			lists[i] = response.Results
		}
	}
	for _, feed := range p.RSSFeeds {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		latest := s.ConfigSnapshot()
		if latest == nil || !latest.Newspaper.Enabled || latest.Newspaper.ReadOnly || !latest.Agent.AllowNetworkRequests || !latest.Tools.WebScraper.Enabled {
			return result, errors.New("research permission revoked")
		}
		hits, err := fetchNewspaperFeed(ctx, feed.URL)
		if err != nil {
			progress(newspaper.Progress{Phase: "finding", Message: "A configured RSS feed could not be read"})
			continue
		}
		queries = append(queries, newspaperQuery{Section: feed.Section})
		lists = append(lists, hits)
	}
	maxPages := cfg.Newspaper.MaxPages
	if maxPages < 1 || maxPages > 60 {
		maxPages = 60
	}
	maxStories := map[string]int{"brief": 6, "standard": 12, "in_depth": 16}[p.Length]
	seenURL := map[string]bool{}
	seenTitle := map[string]bool{}
	if s.Newspaper != nil {
		recent, _ := s.Newspaper.List(ctx, 7)
		for _, e := range recent {
			for _, src := range e.Sources {
				seenURL[src.URL] = true
			}
		}
	}
	attempts := 0
	order := newspaperCandidateOrder(p, queries, searchQueries)
	for index := 0; index < 4 && attempts < maxPages && len(result.Stories) < maxStories; index++ {
		for _, qi := range order {
			hits := lists[qi]
			if index >= len(hits) || attempts >= maxPages || len(result.Stories) >= maxStories {
				continue
			}
			if err := ctx.Err(); err != nil {
				return result, err
			}
			latest := s.ConfigSnapshot()
			if latest == nil || !latest.Newspaper.Enabled || latest.Newspaper.ReadOnly || !latest.Tools.WebScraper.Enabled || !latest.Agent.AllowNetworkRequests {
				return result, errors.New("research permission revoked")
			}
			h := hits[index]
			if newspaperExcluded(h.Title, p.Exclusions) {
				continue
			}
			canonical, err := canonicalNewspaperURL(h.URL)
			if err != nil || seenURL[canonical] {
				continue
			}
			seenURL[canonical] = true
			attempts++
			progress(newspaper.Progress{Phase: "reading", Sources: len(result.Sources), Stories: len(result.Stories), Message: fmt.Sprintf("Reading source %d of %d", attempts, maxPages)})
			page, err := scraper.New(s.Guardian).WithContext(ctx).FetchStatic(canonical)
			if err != nil || page == nil || len([]rune(page.Markdown)) < 180 || newspaperExcluded(page.Title, p.Exclusions) {
				continue
			}
			title := newspaperBound(page.Title, 180)
			key := strings.ToLower(strings.Join(strings.Fields(title), " "))
			if key == "" || seenTitle[key] {
				continue
			}
			seenTitle[key] = true
			published := newspaperPublished(h.Published, page.RawHTML)
			if published != nil && (published.After(time.Now().UTC().Add(time.Hour)) || time.Since(*published) > 7*24*time.Hour) {
				continue
			}
			u, _ := url.Parse(canonical)
			hash := sha256.Sum256([]byte(canonical))
			source := newspaper.Source{ID: "src-" + hex.EncodeToString(hash[:6]), URL: canonical, Publisher: u.Hostname(), Title: title, PublishedAt: published, RetrievedAt: time.Now().UTC(), Excerpt: newspaperBound(page.Markdown, 5000)}
			result.Sources = append(result.Sources, source)
			progress(newspaper.Progress{Phase: "reading", Sources: len(result.Sources), Stories: len(result.Stories), Message: "Source captured for verification", Source: &source})
			progress(newspaper.Progress{Phase: "editing", Sources: len(result.Sources), Stories: len(result.Stories), Message: "Writing sourced stories"})
			story, err := s.newspaperWriteStory(ctx, p, queries[qi].Section, source)
			if err != nil {
				result.Sources = result.Sources[:len(result.Sources)-1]
				continue
			}
			story.ID = fmt.Sprintf("story-%d", len(result.Stories)+1)
			story.Section = queries[qi].Section
			story.SourceIDs = []string{source.ID}
			story.SingleSource = true
			for i := range story.Paragraphs {
				story.Paragraphs[i].SourceIDs = []string{source.ID}
			}
			if err = newspaper.ValidateDraft(newspaper.Draft{Stories: []newspaper.Story{story}, Sources: []newspaper.Source{source}}, p, time.Now().UTC()); err != nil {
				result.Sources = result.Sources[:len(result.Sources)-1]
				continue
			}
			result.Stories = append(result.Stories, story)
			if qi < searchQueries {
				covered[qi] = true
			} else {
				for i, query := range queries[:searchQueries] {
					if query.Section == queries[qi].Section && query.Section != "interests" {
						covered[i] = true
					}
				}
			}
		}
	}
	progress(newspaper.Progress{Phase: "checking", Sources: len(result.Sources), Stories: len(result.Stories), Message: "Checking citations and publication rules"})
	if err := newspaper.ValidateDraft(result, p, time.Now().UTC()); err != nil {
		return result, err
	}
	for _, ok := range covered {
		if !ok {
			result.Partial = true
			break
		}
	}
	return result, nil
}

func (s *Server) newspaperWriteStory(ctx context.Context, p newspaper.Profile, section string, source newspaper.Source) (newspaper.Story, error) {
	latest := s.ConfigSnapshot()
	if latest == nil || !latest.Newspaper.Enabled || latest.Newspaper.ReadOnly || s.LLMClient == nil {
		return newspaper.Story{}, errors.New("research permission revoked")
	}
	if s.BudgetTracker != nil && s.BudgetTracker.IsBlocked("newspaper") {
		return newspaper.Story{}, errors.New("provider spending policy blocks research")
	}
	guide := newspaper.Skill + "\nReturn exactly one JSON object with headline, deck and paragraphs (2-4). Each paragraph has text and evidence_quote. The evidence_quote must be an exact consecutive substring of at least 20 characters from the source text that supports that paragraph. If the page lacks enough substantiated news, return {\"headline\":\"\",\"deck\":\"\",\"paragraphs\":[]}."
	input := fmt.Sprintf("Language: %s\nSection: %s\nPublication date: %s\nPublisher: %s\nArticle title: %s\nPublished: %v\n<external_data source_id=\"%s\">\n%s\n</external_data>", p.Language, section, time.Now().Format("2006-01-02"), source.Publisher, source.Title, source.PublishedAt, source.ID, source.Excerpt)
	request := openai.ChatCompletionRequest{Model: latest.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: guide}, {Role: openai.ChatMessageRoleUser, Content: input}}, MaxTokens: 1500, Temperature: 0.2}
	response, err := s.LLMClient.CreateChatCompletion(ctx, request)
	if err != nil {
		return newspaper.Story{}, fmt.Errorf("write newspaper story: %w", err)
	}
	content, err := llm.JSONContentFromResponse(response)
	if err != nil {
		return newspaper.Story{}, err
	}
	var raw struct {
		Headline   string `json:"headline"`
		Deck       string `json:"deck"`
		Paragraphs []struct {
			Text          string `json:"text"`
			EvidenceQuote string `json:"evidence_quote"`
		} `json:"paragraphs"`
	}
	if err = json.Unmarshal([]byte(content), &raw); err != nil {
		return newspaper.Story{}, err
	}
	story := newspaper.Story{Headline: newspaperBound(raw.Headline, 180), Deck: newspaperBound(raw.Deck, 350), Paragraphs: []newspaper.Paragraph{}}
	for _, para := range raw.Paragraphs {
		story.Paragraphs = append(story.Paragraphs, newspaper.Paragraph{Text: newspaperBound(para.Text, 1400), EvidenceQuote: para.EvidenceQuote})
	}
	return story, nil
}
