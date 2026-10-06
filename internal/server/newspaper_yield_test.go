package server

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/newspaper"
	"aurago/internal/scraper"
)

func newspaperAutoFixture(topics int) (newspaper.Profile, newspaperResearchIO) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"regional", "national", "international", "politics", "economy", "culture", "technology", "science", "environment", "health", "sport"}[:min(topics, 11)]
	p.City, p.Length = "Berlin", "standard"
	for i := 11; i < topics; i++ {
		p.Interests = append(p.Interests, fmt.Sprintf("research topic %d", i))
	}
	deps := newspaperFixtureIO(p)
	b := newspaper.ResolveBudget(p, newspaper.BudgetConfig{Mode: "auto"})
	deps.Budget = &b
	return p, deps
}

func TestNewspaperAutoYieldAndTopicCoverage(t *testing.T) {
	for _, count := range []int{8, 20, 31} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			p, deps := newspaperAutoFixture(count)
			var stats *newspaper.ResearchStats
			draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, func(v newspaper.Progress) { stats = v.Research })
			if err != nil || len(draft.Stories) != max(12, count) || len(stats.Coverage) != count || len(stats.Gaps) != 0 {
				t.Fatalf("topics=%d stories=%d stats=%+v err=%v", count, len(draft.Stories), stats, err)
			}
			if stats.Pages > deps.Budget.Pages || stats.Searches > deps.Budget.Searches || stats.EditorCalls > deps.Budget.EditorCalls {
				t.Fatalf("budget exceeded: %+v", stats)
			}
		})
	}
}

func TestNewspaperFollowUpReserveAddsVerifiedNews(t *testing.T) {
	p, deps := newspaperAutoFixture(8)
	deps.Search = func(_ context.Context, _ string, q newspaperQuery, freshness string) (newspaperSearchBatch, error) {
		batch := newspaperSearchBatch{}
		for i := 0; i < 20; i++ {
			batch.Hits = append(batch.Hits, newspaperHit{Title: fmt.Sprintf("%s %s news %d", q.Topic, freshness, i), URL: fmt.Sprintf("https://publisher-%d.example/%s/%s/%d", i%6, freshness, q.Topic, i)})
		}
		return batch, nil
	}
	deps.Fetch = func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
		if strings.Contains(raw, "/pd/") {
			return &scraper.ScrapeResult{Title: "Not an article", Markdown: "Unavailable"}, nil
		}
		return newspaperFixturePage(raw), nil
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || len(draft.Stories) != 12 || len(stats.Coverage) != 8 {
		t.Fatalf("follow-up yield: stories=%d stats=%+v err=%v", len(draft.Stories), stats, err)
	}
	if stats.Rounds[0].Pages > 72 || stats.Rounds[0].Searches > 24 || stats.Rounds[1].Read == 0 || stats.Rounds[1].Accepted != 12 {
		t.Fatalf("reserve not preserved: %+v", stats)
	}
	// The previous coordinator spent every page in the initial pd queue. This
	// fixture has zero usable pd originals, so those identical first-round
	// results could not yield a verified story without a follow-up read.
}

func TestNewspaperDeferredOriginalFetchedOnce(t *testing.T) {
	p, deps := newspaperAutoFixture(1)
	var fetched atomic.Int32
	deps.Search = func(context.Context, string, newspaperQuery, string) (newspaperSearchBatch, error) {
		return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Research original", URL: "https://institute.example/research"}}}, nil
	}
	deps.Fetch = func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
		fetched.Add(1)
		page := newspaperFixturePage(raw)
		page.RawHTML = `<meta property="article:published_time" content="` + time.Now().Add(-48*time.Hour).Format(time.RFC3339) + `">`
		return page, nil
	}
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, nil)
	if err != nil || len(draft.Stories) != 1 || fetched.Load() != 1 {
		t.Fatalf("cached original: articles=%d reads=%d err=%v", len(draft.Stories), fetched.Load(), err)
	}
}

func TestNewspaperEditorialRepairIsBounded(t *testing.T) {
	for _, decline := range []bool{false, true} {
		p, deps := newspaperAutoFixture(1)
		deps.Search = func(context.Context, string, newspaperQuery, string) (newspaperSearchBatch, error) {
			return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Original", URL: "https://institute.example/report"}}}, nil
		}
		var calls atomic.Int32
		deps.Complete = func(ctx context.Context, guide, input string) (string, error) {
			if strings.Contains(guide, "Task: PLAN RESEARCH") {
				return "{}", nil
			}
			calls.Add(1)
			if decline {
				return `{"declined":true,"headline":"","paragraphs":[]}`, nil
			}
			if !strings.Contains(guide, "Regenerate from the same source") {
				return `{"headline":`, nil
			}
			return newspaperFixtureComplete(ctx, guide, input)
		}
		var stats *newspaper.ResearchStats
		draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, func(v newspaper.Progress) { stats = v.Research })
		if decline {
			if err == nil || len(draft.Stories) != 0 || calls.Load() != 1 || stats.Repairs != 0 || stats.Rejected["editorial_decline"] != 1 {
				t.Fatalf("decline retried: %+v %v", stats, err)
			}
		} else if err != nil || len(draft.Stories) != 1 || calls.Load() != 2 || stats.EditorCalls != 2 || stats.Repairs != 1 {
			t.Fatalf("repair: %+v %v", stats, err)
		}
	}
}

func TestNewspaperFollowUpDoesNotWaitForEditor(t *testing.T) {
	p, deps := newspaperAutoFixture(1)
	followup := make(chan struct{})
	var started atomic.Bool
	deps.Search = func(_ context.Context, _ string, q newspaperQuery, freshness string) (newspaperSearchBatch, error) {
		if freshness == "pw" && started.CompareAndSwap(false, true) {
			close(followup)
		}
		return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Original " + freshness, URL: "https://institute.example/" + freshness}}}, nil
	}
	deps.Complete = func(ctx context.Context, guide, input string) (string, error) {
		if strings.Contains(guide, "Task: PLAN RESEARCH") {
			return "{}", nil
		}
		select {
		case <-followup:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		return newspaperFixtureComplete(ctx, guide, input)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	draft, err := runNewspaperResearch(ctx, p, time.Now(), 60, 32, deps, nil)
	if err != nil || len(draft.Stories) != 2 || !started.Load() {
		t.Fatalf("editor blocked follow-up: %d %v", len(draft.Stories), err)
	}
}

func TestNewspaperRunBudgetCannotExpand(t *testing.T) {
	p, deps := newspaperAutoFixture(8)
	deps.Budget.Pages, deps.Budget.Searches = 5, 4
	larger := newspaper.ResolveBudget(p, newspaper.BudgetConfig{Mode: "auto"})
	deps.LiveBudget = func() newspaper.Budget { return larger }
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || len(draft.Stories) == 0 || stats.Pages > 5 || stats.Searches > 4 || stats.Budget.Pages != 5 {
		t.Fatalf("expanded active run: %+v %v", stats, err)
	}
}

func TestNewspaperDecodedQuotesKeepExactValidation(t *testing.T) {
	p, deps := newspaperAutoFixture(1)
	deps.Search = func(context.Context, string, newspaperQuery, string) (newspaperSearchBatch, error) {
		return newspaperSearchBatch{Hits: []newspaperHit{{Title: "Original", URL: "https://institute.example/encoded"}}}, nil
	}
	deps.Fetch = func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
		page := newspaperFixturePage(raw)
		page.Markdown = "The council approved science &amp; culture funding after a public consultation.\n\n" + strings.Repeat("The institution published the detailed resolution today. ", 5)
		return page, nil
	}
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 60, 32, deps, nil)
	if err != nil || len(draft.Stories) != 1 || !strings.Contains(draft.Stories[0].Paragraphs[0].EvidenceQuote, "science & culture") {
		t.Fatalf("decoded quote: %+v %v", draft, err)
	}
	if err := newspaper.ValidateDraft(draft, p, time.Now()); err != nil {
		t.Fatal(err)
	}
}
