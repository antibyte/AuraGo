package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"html"
	"net/url"
	"sort"
	"strings"
	"time"

	"aurago/internal/newspaper"

	"github.com/PuerkitoBio/goquery"
)

const (
	newspaperOverviewGoogle   = "google_news"
	newspaperOverviewHacker   = "hacker_news"
	newspaperOverviewTechmeme = "techmeme"
	newspaperOverviewLimit    = 96
	newspaperOverviewPerTopic = 3
	newspaperOverviewTimeout  = 30 * time.Second
)

// A lead is bounded, server-assigned planning metadata. It is never article
// evidence; only a separately discovered original can be queued, and that URL
// still goes through the normal guarded article reader.
type newspaperOverviewLead struct {
	ID           string   `json:"id"`
	TopicIDs     []string `json:"topics"`
	Title        string   `json:"headline"`
	Publisher    string   `json:"publisher"`
	DateHint     string   `json:"date_hint,omitempty"`
	DateNote     string   `json:"date_note,omitempty"`
	Description  string   `json:"description,omitempty"`
	Source       string   `json:"source"`
	canonicalURL string
}

type newspaperOverviewRequest struct {
	key   string
	url   string
	modes []newspaperOverviewMode
}

type newspaperOverviewMode struct {
	source string
	topics []newspaperTopic
	shared bool
	feeds  []newspaper.RSSFeed
}

type newspaperOverviewResult struct {
	request newspaperOverviewRequest
	hits    []newspaperHit
	err     error
}

func newspaperAggregatorURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	for _, domain := range []string{"news.google.com", "news.ycombinator.com", "techmeme.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func newspaperGoogleRSSURL(p newspaper.Profile, query string) string {
	lang := strings.ToLower(strings.TrimSpace(p.Language))
	if lang == "" {
		lang = "en"
	}
	country := strings.ToUpper(strings.TrimSpace(p.Country))
	if len(country) != 2 {
		country = "US"
	}
	values := url.Values{"ceid": {country + ":" + lang}, "gl": {country}, "hl": {lang}}
	path := "https://news.google.com/rss"
	if strings.TrimSpace(query) != "" {
		values.Set("q", strings.TrimSpace(query))
		path += "/search"
	}
	return path + "?" + values.Encode()
}

func (r *newspaperResearchRun) overviews(ctx context.Context, round int) {
	if ctx == nil || ctx.Err() != nil || r.io.Feed == nil {
		return
	}
	r.round = round
	stage, cancel := context.WithTimeout(ctx, newspaperOverviewTimeout)
	defer cancel()

	requests := r.initialOverviewRequests()
	r.runOverviewRequests(stage, requests)
	if stage.Err() != nil || !r.allowed("rss") || !r.overviewSourceEnabled(newspaperOverviewGoogle) {
		return
	}
	// Topic-specific Google feeds fill gaps left by the shared front page and
	// specialist feeds. Each topic gets at most one such query per run.
	queries := newspaperFallbackPlan(r.profile, r.topics, round)
	byTopic := make(map[string]newspaperQuery, len(queries))
	for _, query := range queries {
		byTopic[query.Topic] = query
	}
	fill := make([]newspaperOverviewRequest, 0, len(r.topics))
	for _, topic := range r.topics {
		if r.overviewLeadCount(topic.ID) >= newspaperOverviewPerTopic {
			continue
		}
		query := byTopic[topic.ID]
		key, rawURL := newspaperOverviewRequestKey(newspaperGoogleRSSURL(r.profile, query.Text))
		if key == "" || r.overviewSeen[key] {
			continue
		}
		fill = append(fill, newspaperOverviewRequest{key: key, url: rawURL, modes: []newspaperOverviewMode{{source: newspaperOverviewGoogle, topics: []newspaperTopic{topic}}}})
	}
	r.runOverviewRequests(stage, fill)
}

func (r *newspaperResearchRun) initialOverviewRequests() []newspaperOverviewRequest {
	requests := make([]newspaperOverviewRequest, 0, len(r.profile.RSSFeeds)+3)
	index := map[string]int{}
	add := func(rawURL string, mode newspaperOverviewMode) {
		key, resolvedURL := newspaperOverviewRequestKey(rawURL)
		if key == "" {
			return
		}
		if pos, ok := index[key]; ok {
			requests[pos].modes = append(requests[pos].modes, mode)
			return
		}
		index[key] = len(requests)
		requests = append(requests, newspaperOverviewRequest{key: key, url: resolvedURL, modes: []newspaperOverviewMode{mode}})
	}
	for _, feed := range r.profile.RSSFeeds {
		if !r.topicExists(feed.Section) {
			continue
		}
		add(feed.URL, newspaperOverviewMode{source: "user_feed", feeds: []newspaper.RSSFeed{feed}})
	}

	allTopics := append([]newspaperTopic(nil), r.topics...)
	if r.overviewSourceEnabled(newspaperOverviewGoogle) {
		add(newspaperGoogleRSSURL(r.profile, ""), newspaperOverviewMode{source: newspaperOverviewGoogle, topics: allTopics, shared: true})
	}
	if topics := newspaperSpecialistTopics(r.topics); len(topics) > 0 && r.overviewSourceEnabled(newspaperOverviewHacker) {
		add("https://news.ycombinator.com/rss", newspaperOverviewMode{source: newspaperOverviewHacker, topics: topics, shared: true})
	}
	if topics := newspaperSpecialistTopics(r.topics); len(topics) > 0 && r.overviewSourceEnabled(newspaperOverviewTechmeme) {
		add("https://www.techmeme.com/feed.xml", newspaperOverviewMode{source: newspaperOverviewTechmeme, topics: topics, shared: true})
	}
	return requests
}

func newspaperOverviewRequestKey(rawURL string) (string, string) {
	canonical, err := canonicalNewspaperURL(rawURL)
	if err != nil || len(canonical) > 2048 {
		return "", ""
	}
	return "request:" + canonical, rawURL
}

func (r *newspaperResearchRun) overviewSourceEnabled(source string) bool {
	selected := false
	for _, configured := range r.io.OverviewSources {
		if strings.EqualFold(strings.TrimSpace(configured), source) {
			selected = true
			break
		}
	}
	if !selected {
		return false
	}
	if r.io.LiveOverviewSources == nil {
		return true
	}
	for _, live := range r.io.LiveOverviewSources() {
		if strings.EqualFold(strings.TrimSpace(live), source) {
			return true
		}
	}
	return false
}

func (r *newspaperResearchRun) enabledOverviewModes(modes []newspaperOverviewMode) []newspaperOverviewMode {
	if !r.allowed("rss") {
		return nil
	}
	result := make([]newspaperOverviewMode, 0, len(modes))
	for _, mode := range modes {
		if mode.source == "user_feed" || r.overviewSourceEnabled(mode.source) {
			result = append(result, mode)
		}
	}
	return result
}

func (r *newspaperResearchRun) runOverviewRequests(ctx context.Context, requests []newspaperOverviewRequest) {
	for start := 0; start < len(requests) && ctx.Err() == nil; {
		batch := make([]newspaperOverviewRequest, 0, 2)
		for start < len(requests) && len(batch) < 2 {
			request := requests[start]
			start++
			if request.key == "" || r.overviewSeen[request.key] {
				continue
			}
			request.modes = r.enabledOverviewModes(request.modes)
			if len(request.modes) == 0 {
				continue
			}
			if !r.canOverview(ctx) {
				if len(batch) == 0 {
					return
				}
				break
			}
			r.overviewSeen[request.key] = true
			r.recordOverview()
			batch = append(batch, request)
		}
		if len(batch) == 0 {
			continue
		}
		results := make(chan newspaperOverviewResult, len(batch))
		for _, request := range batch {
			request := request
			go func() {
				request.modes = r.enabledOverviewModes(request.modes)
				if len(request.modes) == 0 || ctx.Err() != nil {
					results <- newspaperOverviewResult{request: request, err: context.Canceled}
					return
				}
				hits, err := r.io.Feed(ctx, request.url)
				results <- newspaperOverviewResult{request: request, hits: hits, err: err}
			}()
		}
		for received := 0; received < len(batch); received++ {
			select {
			case result := <-results:
				r.consumeOverview(result)
			case <-ctx.Done():
				return
			}
		}
	}
}

func (r *newspaperResearchRun) consumeOverview(result newspaperOverviewResult) {
	request := result.request
	if result.err != nil {
		if result.err != context.Canceled && result.err != context.DeadlineExceeded {
			r.reject("overview_failed")
		}
		r.stats.Tools["rss"] = "failed"
		r.report("finding", nil)
		return
	}
	r.stats.Tools["rss"] = "ready"
	for _, mode := range request.modes {
		for rank, hit := range result.hits {
			if mode.source == "user_feed" {
				r.consumeUserFeedHit(mode.feeds, hit, rank)
				continue
			}
			r.consumeAggregatorHit(mode, hit, rank)
		}
	}
	r.report("finding", nil)
}

func (r *newspaperResearchRun) consumeUserFeedHit(feeds []newspaper.RSSFeed, hit newspaperHit, rank int) {
	for _, feed := range feeds {
		if feed.Section == "interests" {
			for _, topic := range r.topics {
				if topic.Section == "interests" && newspaperMatchesInterest(hit.Title+" "+hit.Description, topic.Label) {
					r.admit(newspaperCandidate{Hit: hit, Query: newspaperQuery{Section: topic.Section, Topic: topic.ID, Text: topic.Label, Language: r.profile.Language}, Rank: rank})
				}
			}
			continue
		}
		for _, topic := range r.topics {
			if topic.Section == feed.Section {
				r.admit(newspaperCandidate{Hit: hit, Query: newspaperQuery{Section: topic.Section, Topic: topic.ID, Language: r.profile.Language}, Rank: rank})
			}
		}
	}
}

func (r *newspaperResearchRun) consumeAggregatorHit(mode newspaperOverviewMode, hit newspaperHit, rank int) {
	canonical, err := canonicalNewspaperURL(hit.URL)
	if err != nil {
		r.reject("unsafe_url")
		return
	}
	title := newspaperBound(newspaperUnwrap(hit.Title), 180)
	description := newspaperFeedText(hit.Description)
	publisher, title := newspaperOverviewPublisher(mode.source, title, canonical, hit.Publisher)
	topics := mode.topics
	if mode.shared && mode.source == newspaperOverviewGoogle {
		topics = newspaperTopOverviewMatches(r.profile, mode.topics, title+" "+description+" "+publisher)
	}
	originalURL := ""
	if mode.source == newspaperOverviewTechmeme {
		originalURL, _ = newspaperTechmemePrimaryURL(hit.Description, title)
	} else if !newspaperAggregatorURL(canonical) {
		originalURL = canonical
	}
	if originalURL != "" {
		for _, topic := range topics {
			r.admit(newspaperCandidate{Hit: newspaperHit{Title: title, URL: originalURL, Description: description}, Query: newspaperQuery{Section: topic.Section, Topic: topic.ID, Text: title + " " + publisher, Language: r.profile.Language, Kind: "web"}, Rank: rank})
		}
	}
	if len(topics) == 0 {
		return
	}
	note := ""
	if mode.source == newspaperOverviewHacker {
		note = "submission date hint only; not the original publication date"
	}
	lead := newspaperOverviewLead{Title: title, Publisher: publisher, DateHint: newspaperBound(hit.Published, 80), DateNote: note,
		Description: newspaperBound(description, 300), Source: mode.source}
	if originalURL != "" {
		lead.canonicalURL = originalURL
	} else {
		lead.canonicalURL = canonical
	}
	for _, topic := range topics {
		lead.TopicIDs = append(lead.TopicIDs, topic.ID)
	}
	r.addOverviewLead(lead)
}

func (r *newspaperResearchRun) addOverviewLead(lead newspaperOverviewLead) {
	if lead.Title == "" || lead.canonicalURL == "" || len(r.overviewLeads) >= newspaperOverviewLimit {
		return
	}
	canonical, err := canonicalNewspaperURL(lead.canonicalURL)
	if err != nil || len(canonical) > 2048 {
		return
	}
	normalizedTitle := newspaperOverviewTitleKey(lead.Title)
	for i := range r.overviewLeads {
		current := &r.overviewLeads[i]
		if current.canonicalURL != canonical && newspaperOverviewTitleKey(current.Title) != normalizedTitle {
			continue
		}
		for _, topic := range lead.TopicIDs {
			if newspaperOverviewHasTopic(current.TopicIDs, topic) || r.overviewLeadCount(topic) >= newspaperOverviewPerTopic {
				continue
			}
			current.TopicIDs = append(current.TopicIDs, topic)
		}
		return
	}
	acceptedTopics := make([]string, 0, len(lead.TopicIDs))
	for _, topic := range lead.TopicIDs {
		if r.overviewLeadCount(topic) >= newspaperOverviewPerTopic {
			continue
		}
		acceptedTopics = appendNewspaperOverviewTopic(acceptedTopics, topic)
	}
	if len(acceptedTopics) == 0 {
		return
	}
	lead.TopicIDs = acceptedTopics
	lead.canonicalURL = canonical
	lead.ID = newspaperOverviewID(lead.Source, canonical, normalizedTitle)
	r.overviewLeads = append(r.overviewLeads, lead)
	if len(r.overviewLeads) > newspaperOverviewLimit {
		r.overviewLeads = r.overviewLeads[:newspaperOverviewLimit]
	}
}

func (r *newspaperResearchRun) overviewLeadCount(topicID string) int {
	count := 0
	for _, lead := range r.overviewLeads {
		if newspaperOverviewHasTopic(lead.TopicIDs, topicID) {
			count++
		}
	}
	return count
}

func (r *newspaperResearchRun) topicExists(section string) bool {
	for _, topic := range r.topics {
		if topic.Section == section {
			return true
		}
	}
	return false
}

func newspaperSpecialistTopics(topics []newspaperTopic) []newspaperTopic {
	result := make([]newspaperTopic, 0, len(topics))
	for _, topic := range topics {
		if newspaperTechnologyTopic(topic) {
			result = append(result, topic)
		}
	}
	return result
}

func newspaperTechnologyTopic(topic newspaperTopic) bool {
	if topic.Section == "technology" {
		return true
	}
	terms := strings.Join(newspaperTerms(topic.Label), " ")
	for _, term := range []string{"tech", "technology", "software", "computer", "computing", "programming", "linux", "open source", "open-source", "artificial intelligence", "cybersecurity", "chip", "hardware", "ki", "künstliche intelligenz", "informatik", "software", "programmierung", "cybersicherheit"} {
		if strings.Contains(terms, term) {
			return true
		}
	}
	return false
}

func newspaperTopOverviewMatches(p newspaper.Profile, topics []newspaperTopic, text string) []newspaperTopic {
	type scored struct {
		topic newspaperTopic
		score int
	}
	items := make([]scored, 0, len(topics))
	for _, topic := range topics {
		score := newspaperOverviewTopicScore(p, topic, text)
		if score > 0 {
			items = append(items, scored{topic: topic, score: score})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	if len(items) > 3 {
		items = items[:3]
	}
	result := make([]newspaperTopic, 0, len(items))
	for _, item := range items {
		result = append(result, item.topic)
	}
	return result
}

func newspaperOverviewTopicScore(p newspaper.Profile, topic newspaperTopic, text string) int {
	text = " " + strings.Join(newspaperTerms(text), " ") + " "
	score := 0
	for _, term := range newspaperTerms(topic.Label) {
		if len([]rune(term)) >= 3 && strings.Contains(text, " "+term+" ") {
			score += 4
		}
	}
	var terms []string
	switch topic.Section {
	case "regional":
		terms = []string{"local", "regional", "city", "town", "council", "community", "lokal", "stadt", "gemeinde", "stadtrat"}
		terms = append(terms, newspaperTerms(p.City)...)
		terms = append(terms, newspaperTerms(p.Region)...)
	case "national":
		terms = []string{"national", "country", "parliament", "government", "national", "bundestag", "bundesregierung", "landesregierung"}
	case "international":
		terms = []string{"world", "global", "international", "worldwide", "welt", "international"}
	case "politics":
		terms = []string{"politics", "political", "government", "election", "parliament", "minister", "policy", "politik", "regierung", "wahl", "parlament"}
	case "economy":
		terms = []string{"economy", "business", "company", "market", "finance", "wirtschaft", "unternehmen", "markt", "finanzen"}
	case "culture":
		terms = []string{"culture", "art", "music", "film", "book", "kultur", "kunst", "musik", "film", "buch"}
	case "technology":
		terms = []string{"technology", "tech", "software", "computer", "security", "ai", "chip", "technology", "technik", "software", "computer", "ki", "sicherheit"}
	case "science":
		terms = []string{"science", "research", "study", "researchers", "scientists", "science", "forschung", "studie", "wissenschaft"}
	case "environment":
		terms = []string{"environment", "climate", "energy", "emissions", "umwelt", "klima", "energie"}
	case "health":
		terms = []string{"health", "medical", "medicine", "hospital", "health", "gesundheit", "medizin", "krankenhaus"}
	case "sport":
		terms = []string{"sports", "sport", "football", "soccer", "basketball", "match", "sport", "fußball", "spiel"}
	}
	for _, term := range terms {
		if strings.Contains(text, " "+term+" ") {
			score++
		}
	}
	return score
}

func newspaperOverviewPublisher(source, title, rawURL, feedPublisher string) (string, string) {
	if publisher := strings.TrimSpace(feedPublisher); publisher != "" {
		if at := strings.LastIndex(title, " - "); at > 0 && strings.EqualFold(strings.TrimSpace(title[at+3:]), publisher) {
			title = strings.TrimSpace(title[:at])
		}
		return newspaperBound(publisher, 80), title
	}
	if source == newspaperOverviewGoogle {
		if at := strings.LastIndex(title, " - "); at > 0 && at+3 < len(title) {
			publisher := strings.TrimSpace(title[at+3:])
			if len([]rune(publisher)) <= 80 {
				return publisher, strings.TrimSpace(title[:at])
			}
		}
		return "Google News", title
	}
	if source == newspaperOverviewHacker {
		if !newspaperAggregatorURL(rawURL) {
			return newspaperPublisherKey(rawURL), title
		}
		return "Hacker News", title
	}
	if source == newspaperOverviewTechmeme {
		return "Techmeme", title
	}
	return newspaperPublisherKey(rawURL), title
}

func newspaperFeedText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	if doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw)); err == nil {
		raw = doc.Text()
	}
	raw = html.UnescapeString(raw)
	return strings.Join(strings.Fields(raw), " ")
}

func newspaperTechmemePrimaryURL(description, headline string) (string, string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(description))
	if err != nil {
		return "", ""
	}
	headlineTerms := newspaperTechmemeSubstantiveTerms(headline)
	if len(headlineTerms) < 2 {
		return "", ""
	}
	minimumMatches := max(2, (len(headlineTerms)+1)/2)
	bestURL, bestText, bestScore := "", "", 0
	doc.Find("a[href]").EachWithBreak(func(_ int, node *goquery.Selection) bool {
		raw, ok := node.Attr("href")
		if !ok {
			return true
		}
		base, _ := url.Parse("https://www.techmeme.com/")
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return true
		}
		canonical, err := canonicalNewspaperURL(base.ResolveReference(parsed).String())
		if err != nil || newspaperAggregatorURL(canonical) {
			return true
		}
		anchorText := strings.Join(strings.Fields(node.Text()), " ")
		anchorTerms := map[string]bool{}
		for _, term := range newspaperTerms(anchorText) {
			anchorTerms[term] = true
		}
		matched := 0
		for _, term := range headlineTerms {
			if len([]rune(term)) >= 3 && anchorTerms[term] {
				matched++
			}
		}
		if matched >= minimumMatches && matched > bestScore {
			bestURL, bestText, bestScore = canonical, anchorText, matched
		}
		return true
	})
	if bestURL == "" {
		return "", ""
	}
	return bestURL, bestText
}

func newspaperTechmemeSubstantiveTerms(headline string) []string {
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "announces": true, "announced": true,
		"are": true, "as": true, "at": true, "be": true, "by": true, "company": true,
		"for": true, "from": true, "has": true, "have": true, "in": true, "into": true,
		"is": true, "its": true, "launch": true, "launches": true, "new": true, "of": true,
		"on": true, "or": true, "reveals": true, "revealed": true, "says": true, "the": true,
		"their": true, "this": true, "to": true, "unveils": true, "was": true, "were": true,
		"will": true, "with": true,
	}
	terms := newspaperTerms(headline)
	result := make([]string, 0, len(terms))
	for _, term := range terms {
		if len([]rune(term)) >= 3 && !stop[term] {
			result = append(result, term)
		}
	}
	return result
}

func newspaperOverviewTitleKey(title string) string {
	return strings.Join(newspaperTerms(title), " ")
}

func newspaperOverviewID(source, canonicalURL, title string) string {
	hash := sha256.Sum256([]byte(source + "\x00" + canonicalURL + "\x00" + title))
	return "overview-" + hex.EncodeToString(hash[:8])
}

func appendNewspaperOverviewTopic(values []string, value string) []string {
	if !newspaperOverviewHasTopic(values, value) {
		return append(values, value)
	}
	return values
}

func newspaperOverviewHasTopic(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
