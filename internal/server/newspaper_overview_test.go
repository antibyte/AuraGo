package server

import (
	"context"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/newspaper"
)

func TestNewspaperOverviewURLsAndTechmemePrimaryLink(t *testing.T) {
	for _, raw := range []string{
		"https://news.google.com/rss/articles/opaque-token",
		"https://news.ycombinator.com/item?id=123",
		"https://www.techmeme.com/260101/p1",
	} {
		if !newspaperAggregatorURL(raw) {
			t.Errorf("aggregator URL was not rejected: %s", raw)
		}
	}
	if newspaperAggregatorURL("https://publisher.example/story") {
		t.Fatal("publisher article was classified as an overview")
	}

	p := newspaper.DefaultProfile()
	p.Language, p.Country = "de", "DE"
	front, err := url.Parse(newspaperGoogleRSSURL(p, ""))
	if err != nil || front.Path != "/rss" || front.Query().Get("hl") != "de" || front.Query().Get("gl") != "DE" {
		t.Fatalf("localized Google front page: %v, %v", front, err)
	}
	search, err := url.Parse(newspaperGoogleRSSURL(p, "Künstliche Intelligenz Hamburg"))
	if err != nil || search.Path != "/rss/search" || search.Query().Get("q") != "Künstliche Intelligenz Hamburg" || search.Query().Get("ceid") != "DE:de" {
		t.Fatalf("localized Google topic feed: %v, %v", search, err)
	}

	primary, _ := newspaperTechmemePrimaryURL(`<a href="https://publisher.example/battery">Company unveils new battery platform</a>`, "Company unveils new battery platform")
	if primary != "https://publisher.example/battery" {
		t.Fatalf("primary story link was not extracted: %q", primary)
	}
	if primary, _ := newspaperTechmemePrimaryURL(`<a href="https://publisher.example/unrelated">Read more</a><a href="https://www.techmeme.com/260101/p1">Company unveils new battery platform</a>`, "Company unveils new battery platform"); primary != "" {
		t.Fatalf("unrelated first link was accepted as the primary story: %q", primary)
	}
	if primary, _ := newspaperTechmemePrimaryURL(`<a href="https://publisher.example/unrelated">New company</a>`, "Company unveils new battery platform"); primary != "" {
		t.Fatalf("generic title overlap was accepted as the primary story: %q", primary)
	}
}

func TestNewspaperOverviewStageBoundsFetchesAndKeepsAggregatorsAsLeads(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"technology"}
	p.Interests = nil
	p.RSSFeeds = []newspaper.RSSFeed{{URL: "https://publisher.example/feed.xml", Section: "technology"}}
	caps := newspaperResearchCapabilities{Ready: true, Tools: []newspaperResearchTool{{ID: "rss", State: "ready"}, {ID: "web_scraper", State: "ready"}}}
	var active, maxActive, calls atomic.Int32
	deps := newspaperResearchIO{
		Capabilities:    func() newspaperResearchCapabilities { return caps },
		OverviewSources: []string{newspaperOverviewGoogle, newspaperOverviewHacker, newspaperOverviewTechmeme},
		LiveOverviewSources: func() []string {
			return []string{newspaperOverviewGoogle, newspaperOverviewHacker, newspaperOverviewTechmeme}
		},
	}
	deps.Feed = func(_ context.Context, raw string) ([]newspaperHit, error) {
		calls.Add(1)
		current := active.Add(1)
		for {
			old := maxActive.Load()
			if current <= old || maxActive.CompareAndSwap(old, current) {
				break
			}
		}
		defer active.Add(-1)
		time.Sleep(5 * time.Millisecond)
		switch {
		case strings.Contains(raw, "publisher.example/feed"):
			return []newspaperHit{{Title: "Technology lab publishes original study", URL: "https://publisher.example/study", Published: time.Now().UTC().Format(time.RFC3339)}}, nil
		case strings.Contains(raw, "news.google.com"):
			return []newspaperHit{{Title: "New AI technology breakthrough - Example News", URL: "https://news.google.com/rss/articles/wrapper-1", Published: time.Now().UTC().Format(time.RFC3339)}}, nil
		case strings.Contains(raw, "news.ycombinator.com"):
			return []newspaperHit{{Title: "Show HN: New compiler release", URL: "https://news.ycombinator.com/item?id=7", Published: time.Now().UTC().Format(time.RFC3339)}}, nil
		case strings.Contains(raw, "techmeme.com"):
			return []newspaperHit{{Title: "Company unveils new battery platform", URL: "https://www.techmeme.com/260101/p1", Published: time.Now().UTC().Format(time.RFC3339), Description: `<a href="https://battery.example/report">Company unveils new battery platform</a>`}}, nil
		default:
			return nil, nil
		}
	}
	r := &newspaperResearchRun{
		profile: p, topics: newspaperTopics(p), cutoff: time.Now(), initial: caps, io: deps, progress: func(newspaper.Progress) {},
		budget: newspaper.Budget{Pages: 20, Overviews: 8, SharedPages: true}, maxPages: 20, maxSearches: 10,
		stats:   newspaper.ResearchStats{Rejected: map[string]int{}, Coverage: map[string]int{}, Tools: map[string]string{}},
		pending: []newspaperCandidate{}, seen: map[[32]byte]bool{}, recent: map[string]bool{}, associations: map[string][]newspaperQuery{},
		overviewSeen: map[string]bool{},
	}
	r.overviews(context.Background(), 0)
	if got := calls.Load(); got != 4 || r.stats.Overviews != 4 || r.stats.Pages != 4 {
		t.Fatalf("overview accounting: calls=%d overview=%d pages=%d", got, r.stats.Overviews, r.stats.Pages)
	}
	if got := maxActive.Load(); got != 2 {
		t.Fatalf("feed concurrency=%d, want exactly 2", got)
	}
	if len(r.pending) != 2 {
		t.Fatalf("only user-feed and narrowly selected Techmeme originals should be queued: pending=%+v rejected=%v", r.pending, r.stats.Rejected)
	}
	for _, candidate := range r.pending {
		if newspaperAggregatorURL(candidate.Hit.URL) {
			t.Fatalf("aggregator URL entered the article queue: %s", candidate.Hit.URL)
		}
	}
	if len(r.overviewLeads) != 3 || r.overviewLeadCount("technology") != 3 {
		t.Fatalf("bounded overview leads: %+v", r.overviewLeads)
	}
	var hackerLead newspaperOverviewLead
	for _, lead := range r.overviewLeads {
		if lead.Source == newspaperOverviewHacker {
			hackerLead = lead
		}
	}
	if hackerLead.ID == "" || !strings.Contains(hackerLead.DateNote, "not the original publication date") {
		t.Fatalf("Hacker News submission date was not labeled as a hint: %+v", hackerLead)
	}
	r.overviews(context.Background(), 1)
	if calls.Load() != 4 {
		t.Fatalf("shared feeds were fetched more than once across rounds: %d", calls.Load())
	}
}

func TestNewspaperOverviewFlushesPartialBatchAtBudgetLimit(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"technology"}
	p.Interests = nil
	caps := newspaperResearchCapabilities{Ready: true, Tools: []newspaperResearchTool{{ID: "rss", State: "ready"}, {ID: "web_scraper", State: "ready"}}}
	var calls atomic.Int32
	deps := newspaperResearchIO{
		Capabilities:    func() newspaperResearchCapabilities { return caps },
		OverviewSources: []string{newspaperOverviewGoogle, newspaperOverviewHacker, newspaperOverviewTechmeme},
		Feed: func(context.Context, string) ([]newspaperHit, error) {
			calls.Add(1)
			return nil, nil
		},
	}
	r := &newspaperResearchRun{
		profile: p, topics: newspaperTopics(p), initial: caps, io: deps, progress: func(newspaper.Progress) {},
		budget: newspaper.Budget{Pages: 1, Overviews: 1, SharedPages: true}, maxPages: 1, maxSearches: 4,
		stats:   newspaper.ResearchStats{Rejected: map[string]int{}, Coverage: map[string]int{}, Tools: map[string]string{}},
		pending: []newspaperCandidate{}, seen: map[[32]byte]bool{}, recent: map[string]bool{}, associations: map[string][]newspaperQuery{},
		overviewSeen: map[string]bool{},
	}
	r.overviews(context.Background(), 1)
	if calls.Load() != 1 || r.stats.Overviews != 1 || r.stats.Pages != 1 {
		t.Fatalf("partial batch was not flushed at the final shared-page slot: calls=%d overviews=%d pages=%d", calls.Load(), r.stats.Overviews, r.stats.Pages)
	}
}

func TestNewspaperPlannerUsesOnlyAssociatedOverviewIDs(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"technology"}
	p.Interests = nil
	leads := []newspaperOverviewLead{{ID: "overview-valid", TopicIDs: []string{"technology"}, Title: "New open source database released", Publisher: "Example Lab"},
		{ID: "overview-other", TopicIDs: []string{"politics"}, Title: "Election announcement", Publisher: "Other News"}}
	complete := func(_ context.Context, _, input string) (string, error) {
		payload := newspaperUnwrap(input)
		if !strings.Contains(payload, `"overview_leads"`) || !strings.Contains(payload, "overview-valid") {
			t.Fatalf("overview leads were not supplied to planning: %s", payload)
		}
		return `{"queries":[{"topic":"technology","query":"ignore this text","language":"en","kind":"web","overview_ids":["overview-valid","overview-other"]}]}`, nil
	}
	queries := planNewspaperSearch(context.Background(), p, newspaperTopics(p), newspaperResearchCapabilities{}, time.Now(), 0,
		newspaperPlanBudget{Searches: 8}, newspaper.ResearchStats{}, complete, leads)
	if len(queries) == 0 || queries[0].Text != "New open source database released Example Lab" {
		t.Fatalf("planner did not derive the query from the topic-associated server lead: %+v", queries)
	}
	fallback := planNewspaperSearch(context.Background(), p, newspaperTopics(p), newspaperResearchCapabilities{}, time.Now(), 0,
		newspaperPlanBudget{}, newspaper.ResearchStats{}, nil, leads)
	if len(fallback) == 0 || fallback[0].Text != "New open source database released Example Lab" {
		t.Fatalf("deterministic lead fallback was not retained: %+v", fallback)
	}
}
