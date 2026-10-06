package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/newspaper"
	"aurago/internal/scraper"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func newspaperFixtureConfig() *config.Config {
	c := &config.Config{}
	c.VirtualDesktop.Enabled, c.Newspaper.Enabled = true, true
	c.Agent.AllowNetworkRequests, c.Tools.WebScraper.Enabled = true, true
	c.BraveSearch.Enabled, c.BraveSearch.APIKey, c.LLM.Model = true, "fixture-key", "fixture-editor"
	return c
}

func newspaperFixturePage(raw string) *scraper.ScrapeResult {
	quote := "The council approved a public research library with an open consultation period."
	return &scraper.ScrapeResult{Title: "Original report " + raw,
		Markdown: security.IsolateExternalData(quote + "\n\n" + strings.Repeat("Details and context are available from the original institution. ", 4) + raw),
		RawHTML:  `<meta property="og:type" content="article"><meta property="article:published_time" content="` + time.Now().UTC().Format(time.RFC3339) + `">`}
}

func newspaperFixtureComplete(ctx context.Context, guide, input string) (string, error) {
	if strings.Contains(guide, "Task: PLAN RESEARCH") {
		return "invalid plan exercises deterministic fallback", nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var payload struct {
		Source newspaper.Source `json:"source"`
	}
	if err := json.Unmarshal([]byte(newspaperUnwrap(input)), &payload); err != nil {
		return "", err
	}
	quote := strings.Split(payload.Source.Excerpt, "\n\n")[0]
	result, err := json.Marshal(map[string]any{"headline": payload.Source.Title, "deck": "A sourced report.", "paragraphs": []any{map[string]string{"text": "The institution announced a research library and public consultation.", "evidence_quote": quote}}})
	return string(result), err
}

func newspaperFixtureIO(p newspaper.Profile) newspaperResearchIO {
	return newspaperResearchIO{
		Capabilities: func() newspaperResearchCapabilities {
			return resolveNewspaperCapabilities(newspaperFixtureConfig(), p, true, true)
		},
		Search: func(_ context.Context, _ string, q newspaperQuery, _ string) (newspaperSearchBatch, error) {
			batch := newspaperSearchBatch{}
			for i := 0; i < 20; i++ {
				batch.Hits = append(batch.Hits, newspaperHit{Title: fmt.Sprintf("%s original news %d", q.Topic, i), URL: fmt.Sprintf("https://publisher-%d.example/%s/%d", i%6, q.Topic, i)})
			}
			return batch, nil
		},
		Feed: func(context.Context, string) ([]newspaperHit, error) {
			return nil, errors.New("fixture feed unavailable")
		},
		Fetch: func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
			return newspaperFixturePage(raw), nil
		},
		Complete: newspaperFixtureComplete,
		Wait:     func(ctx context.Context, _ time.Duration) error { return ctx.Err() },
	}
}

func TestNewspaperResearchBreadthAndConcurrency(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Length = "standard"
	p.Sections = []string{"regional", "international", "politics", "technology", "science", "culture"}
	p.City = "Berlin"
	deps := newspaperFixtureIO(p)
	var mu sync.Mutex
	activeReads, peakReads, activeEditors, peakEditors := 0, 0, 0, 0
	domains := map[string]int{}
	deps.Fetch = func(ctx context.Context, raw string) (*scraper.ScrapeResult, error) {
		domain := newspaperPublisherKey(raw)
		mu.Lock()
		activeReads++
		peakReads = max(peakReads, activeReads)
		domains[domain]++
		if domains[domain] > 1 {
			t.Errorf("concurrent reads for %s", domain)
		}
		mu.Unlock()
		defer func() { mu.Lock(); activeReads--; domains[domain]--; mu.Unlock() }()
		if err := newspaperWait(ctx, 3*time.Millisecond); err != nil {
			return nil, err
		}
		u, _ := url.Parse(raw)
		parts := strings.Split(u.Path, "/")
		rank, _ := strconv.Atoi(parts[len(parts)-1])
		if rank < 4 {
			return &scraper.ScrapeResult{Title: "Unreadable", Markdown: "A snippet is not an article."}, nil
		}
		return newspaperFixturePage(raw), nil
	}
	deps.Complete = func(ctx context.Context, guide, input string) (string, error) {
		if strings.Contains(guide, "Task: PLAN RESEARCH") {
			return newspaperFixtureComplete(ctx, guide, input)
		}
		mu.Lock()
		activeEditors++
		peakEditors = max(peakEditors, activeEditors)
		mu.Unlock()
		defer func() { mu.Lock(); activeEditors--; mu.Unlock() }()
		if err := newspaperWait(ctx, 5*time.Millisecond); err != nil {
			return "", err
		}
		return newspaperFixtureComplete(ctx, guide, input)
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, func(progress newspaper.Progress) { stats = progress.Research })
	if err != nil {
		t.Fatal(err)
	}
	if len(draft.Stories) != 12 || stats.Candidates < 60 || stats.Accepted != 12 || len(stats.Coverage) != 6 || len(stats.Gaps) != 0 {
		t.Fatalf("insufficient edition: stories=%d stats=%+v", len(draft.Stories), stats)
	}
	if stats.Pages > 60 || stats.Searches > 32 || stats.Plans > 2 || stats.Rejected["unreadable"] == 0 {
		t.Fatalf("budgets or deep result selection: %+v", stats)
	}
	for _, source := range draft.Sources {
		parts := strings.Split(source.URL, "/")
		rank, _ := strconv.Atoi(parts[len(parts)-1])
		if rank < 4 {
			t.Fatalf("unread hit entered edition: %s", source.URL)
		}
	}
	if peakReads != 4 || peakEditors != 2 {
		t.Fatalf("concurrency reads=%d editors=%d", peakReads, peakEditors)
	}
	if err := newspaper.ValidateDraft(draft, p, time.Now()); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d candidates, %d fetched pages, %d evidenced articles across %d sections; peaks %d reads / %d editors", stats.Candidates, stats.Read, stats.Accepted, len(stats.Coverage), peakReads, peakEditors)
}

func TestNewspaperCapabilitiesAndPermissionSnapshot(t *testing.T) {
	p := newspaper.DefaultProfile()
	for _, tc := range []struct {
		name, reason, tool, state string
		change                    func(*config.Config)
	}{
		{"ready", "", "brave_search", "ready", func(*config.Config) {}},
		{"key missing", "", "brave_search", "needs_setup", func(c *config.Config) { c.BraveSearch.APIKey = "" }},
		{"brave disabled", "", "brave_search", "disabled", func(c *config.Config) { c.BraveSearch.Enabled = false }},
		{"network disabled", "network_disabled", "ddg_search", "blocked", func(c *config.Config) { c.Agent.AllowNetworkRequests = false }},
		{"scraper disabled", "scraper_disabled", "web_scraper", "disabled", func(c *config.Config) { c.Tools.WebScraper.Enabled = false }},
		{"read only", "read_only", "brave_search", "blocked", func(c *config.Config) { c.Newspaper.ReadOnly = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newspaperFixtureConfig()
			tc.change(c)
			caps := resolveNewspaperCapabilities(c, p, true, true)
			if caps.Reason != tc.reason || caps.Ready != (tc.reason == "") {
				t.Fatalf("caps: %+v", caps)
			}
			for _, tool := range caps.Tools {
				if tool.ID == tc.tool && tool.State != tc.state {
					t.Fatalf("tool: %+v", tool)
				}
			}
		})
	}
	deps := newspaperFixtureIO(p)
	var enabled atomic.Bool
	deps.Capabilities = func() newspaperResearchCapabilities {
		c := newspaperFixtureConfig()
		c.BraveSearch.Enabled = enabled.Load()
		return resolveNewspaperCapabilities(c, p, true, true)
	}
	original := deps.Search
	deps.Search = func(ctx context.Context, backend string, q newspaperQuery, freshness string) (newspaperSearchBatch, error) {
		enabled.Store(true)
		if backend != "ddg_search" {
			t.Errorf("new permission added during run: %s", backend)
		}
		return original(ctx, backend, q, freshness)
	}
	if _, err := runNewspaperResearch(context.Background(), p, time.Now(), 20, 12, deps, nil); err != nil {
		t.Fatal(err)
	}

	var revoked atomic.Bool
	deps = newspaperFixtureIO(p)
	deps.Capabilities = func() newspaperResearchCapabilities {
		c := newspaperFixtureConfig()
		c.Agent.AllowNetworkRequests = !revoked.Load()
		return resolveNewspaperCapabilities(c, p, true, true)
	}
	deps.Fetch = func(context.Context, string) (*scraper.ScrapeResult, error) {
		revoked.Store(true)
		return nil, errors.New("revoked")
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, func(v newspaper.Progress) { stats = v.Research })
	if err == nil || len(draft.Stories) != 0 || stats.Pages > 4 {
		t.Fatalf("revocation: %+v, %+v, %v", draft, stats, err)
	}
}

func TestNewspaperRetryFallbackAndSharedSearchBudget(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science"}
	for _, longWait := range []bool{false, true} {
		deps := newspaperFixtureIO(p)
		calls := []string{}
		deps.Search = func(_ context.Context, backend string, q newspaperQuery, _ string) (newspaperSearchBatch, error) {
			calls = append(calls, backend)
			if backend == "ddg_search" {
				return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Science news", URL: "https://science.example/report"}}}, nil
			}
			delay := time.Second
			if longWait {
				delay = time.Hour
			}
			return newspaperSearchBatch{}, &tools.BraveSearchError{StatusCode: 429, Code: "rate_limited", Temporary: true, RetryAfter: delay}
		}
		var stats *newspaper.ResearchStats
		_, err := runNewspaperResearch(context.Background(), p, time.Now(), 5, 5, deps, func(v newspaper.Progress) { stats = v.Research })
		if err != nil || stats.Searches > 5 {
			t.Fatalf("fallback: %v %+v %v", calls, stats, err)
		}
		want := "brave_news,brave_news,ddg_search,ddg_search"
		if longWait {
			want = "brave_news,ddg_search,ddg_search"
		}
		if strings.Join(calls, ",") != want {
			t.Fatalf("attempts = %v; want %s", calls, want)
		}
	}
}

func TestNewspaperCooldownBeyondPhaseUsesIndependentSearch(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science"}
	deps := newspaperFixtureIO(p)
	calls := []string{}
	deps.Wait = func(_ context.Context, delay time.Duration) error {
		if delay > 0 {
			t.Fatal("waited for a cooldown beyond the phase deadline")
		}
		return nil
	}
	deps.Search = func(_ context.Context, backend string, _ newspaperQuery, _ string) (newspaperSearchBatch, error) {
		calls = append(calls, backend)
		if backend == "ddg_search" {
			return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Science news", URL: "https://science.example/report"}}}, nil
		}
		return newspaperSearchBatch{}, &tools.BraveSearchError{StatusCode: 429, Code: "rate_limited", Temporary: true, RetryAfter: 2 * time.Second}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	draft, err := runNewspaperResearch(ctx, p, time.Now(), 10, 10, deps, nil)
	if err != nil || len(draft.Stories) != 1 || strings.Join(calls, ",") != "brave_news,ddg_search" {
		t.Fatalf("fallback near deadline: calls=%v stories=%d err=%v", calls, len(draft.Stories), err)
	}
}

func TestNewspaperFollowUpDatesAndEvidence(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science"}
	deps := newspaperFixtureIO(p)
	windows := []string{}
	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	deps.Search = func(_ context.Context, _ string, q newspaperQuery, freshness string) (newspaperSearchBatch, error) {
		windows = append(windows, freshness)
		if freshness == "pd" {
			return newspaperSearchBatch{}, nil
		}
		return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Original research", URL: "https://institute.example/paper", Published: old.Format(time.RFC3339)}}}, nil
	}
	deps.Fetch = func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
		page := newspaperFixturePage(raw)
		page.RawHTML = `<script type="application/ld+json">{"@graph":[{"@type":"NewsArticle","headline":"Original research publication","datePublished":"` + old.Format(time.RFC3339) + `","publisher":{"name":"Research institute"}}]}</script>`
		return page, nil
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 10, 10, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || stats.Plans != 2 || len(draft.Stories) != 1 || draft.Sources[0].PublishedAt == nil || !draft.Sources[0].PublishedAt.Equal(old) {
		t.Fatalf("follow-up: %+v %+v %v", draft, stats, err)
	}
	if !strings.Contains(strings.Join(windows, ","), "pw") {
		t.Fatalf("no expanded search: %v", windows)
	}
	deps.Complete = func(ctx context.Context, guide, input string) (string, error) {
		if strings.Contains(guide, "Task: PLAN RESEARCH") {
			return "{}", nil
		}
		return `{"headline":"Unsubstantiated","paragraphs":[{"text":"An invented claim.","evidence_quote":"This quotation was never present in the original article."}]}`, nil
	}
	draft, err = runNewspaperResearch(context.Background(), p, time.Now(), 10, 10, deps, func(v newspaper.Progress) { stats = v.Research })
	if err == nil || len(draft.Stories) != 0 || stats.Rejected["quote_mismatch"] == 0 {
		t.Fatalf("unproven story accepted: %+v %+v %v", draft, stats, err)
	}
}

func TestNewspaperPlanningTargetsAndFallback(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"regional", "science"}
	p.Interests = []string{"Quantencomputer"}
	p.City, p.Country, p.Exclusions, p.EmailTo = "Wien", "AT", []string{"Fußball"}, "private@example.org"
	cap := resolveNewspaperCapabilities(newspaperFixtureConfig(), p, true, true)
	var input string
	queries := planNewspaperSearch(context.Background(), p, newspaperTopics(p), cap, time.Now(), 0, newspaperPlanBudget{Searches: 8}, newspaper.ResearchStats{}, func(_ context.Context, _, payload string) (string, error) {
		input = newspaperUnwrap(payload)
		return `{"queries":[{"topic":"science","query":"quantum photonics new study site:institute.example","language":"en","kind":"web"},{"topic":"unselected","query":"unrelated","language":"en","kind":"news"},{"topic":"interest:0","query":"Quantencomputer neue Studie","language":"de","kind":"news"}]}`, nil
	})
	if len(queries) != 3 || queries[0].Topic != "regional" || !strings.Contains(queries[0].Text, "Wien") || !strings.Contains(queries[0].Text, "Österreich") || queries[1].Language != "en" || queries[2].Section != "interests" {
		t.Fatalf("plan=%+v", queries)
	}
	if strings.Contains(input, p.EmailTo) || !strings.Contains(input, "Fußball") || !strings.Contains(input, "brave_search") {
		t.Fatalf("unexpected planner context: %s", input)
	}
}

func TestNewspaperExtractionAndFeedBounds(t *testing.T) {
	longIndex := &scraper.ScrapeResult{Markdown: strings.Repeat("A short lead from a different report. ", 100), RawHTML: `<main><h2><a href="/a">First original report</a></h2><h2><a href="/b">Second original report</a></h2><h3><a href="/c">Third original report</a></h3><h3><a href="/d">Fourth original report</a></h3></main>`}
	if hits := newspaperOverviewLinks("https://source.example/science", longIndex, newspaperArticleMetadata{}); len(hits) != 4 {
		t.Fatalf("long overview treated as article: %+v", hits)
	}
	late := "The quantum photonics laboratory reported a measurable breakthrough in the new study."
	body := strings.Repeat("General background without the relevant finding. ", 160) + "\n\n" + late
	if excerpt := newspaperEvidenceExcerpt(body, "quantum photonics new study"); !strings.Contains(excerpt, late) || len([]rune(excerpt)) > 5600 {
		t.Fatalf("late evidence missing or unbounded: %d", len([]rune(excerpt)))
	}
	rss := "<rss><channel>"
	for i := 0; i < 50; i++ {
		rss += fmt.Sprintf("<item><title>Report %d</title><link>https://source.example/%d</link><pubDate>%s</pubDate></item>", i, i, time.Now().Add(time.Duration(i)*time.Minute).Format(time.RFC1123Z))
	}
	hits, err := parseNewspaperFeed("https://source.example/feed", []byte(rss+"</channel></rss>"))
	if err != nil || len(hits) != 40 || hits[0].Title != "Report 49" {
		t.Fatalf("feed=%+v %v", hits, err)
	}
	for _, tc := range []struct{ raw, want string }{{"https://NEWS.example/report?utm_source=x&id=7#foo", "https://news.example/report?id=7"}, {"https://news.example/report?id=7&utm_campaign=x", "https://news.example/report?id=7"}} {
		if got, err := canonicalNewspaperURL(tc.raw); err != nil || got != tc.want {
			t.Fatalf("canonical %s %v", got, err)
		}
	}
	p := newspaper.DefaultProfile()
	p.Sections = []string{"culture"}
	deps := newspaperFixtureIO(p)
	deps.Search = func(context.Context, string, newspaperQuery, string) (newspaperSearchBatch, error) {
		return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Overview", URL: "https://source.example/"}}}, nil
	}
	deps.Fetch = func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
		if strings.HasSuffix(raw, "/") {
			return &scraper.ScrapeResult{Title: "Headlines", RawHTML: `<main><a href="/report">A new public research library</a><a href="/news">More daily news coverage</a></main>`}, nil
		}
		if strings.HasSuffix(raw, "/news") {
			return &scraper.ScrapeResult{Title: "More", RawHTML: `<main><a href="/too-deep">This article must not be followed</a></main>`}, nil
		}
		if strings.HasSuffix(raw, "too-deep") {
			t.Error("followed more than one overview layer")
		}
		page := newspaperFixturePage(raw)
		page.RawHTML = "" // Undated originals must remain undated.
		return page, nil
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 10, 8, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || len(draft.Stories) != 1 || draft.Sources[0].PublishedAt != nil || stats.Pages != 3 || stats.Rejected["overview"] != 2 {
		t.Fatalf("overview: %+v %+v %v", draft, stats, err)
	}
}

func TestNewspaperCancellationAndShrinkingBudgets(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science", "culture"}
	deps := newspaperFixtureIO(p)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(ctx, p, time.Now(), 60, 32, deps, func(v newspaper.Progress) {
		stats = v.Research
		if v.Stories >= 1 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || len(draft.Stories) == 0 || !draft.Partial {
		t.Fatalf("lost partial results: %+v %v", draft, err)
	}
	deps = newspaperFixtureIO(p)
	deps.Limits = func() (int, int) { return 1, 1 }
	draft, err = runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || len(draft.Stories) != 1 || stats.Pages != 1 || stats.Searches != 1 || len(stats.Gaps) != 1 {
		t.Fatalf("shared limits: %+v %+v %v", draft, stats, err)
	}
}
