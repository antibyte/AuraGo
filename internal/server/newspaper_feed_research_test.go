package server

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"aurago/internal/newspaper"
)

func TestNewspaperFeedResearchWithUnavailableSearch(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = nil
	p.Interests = []string{"library"}
	p.Length = "brief"
	p.RSSFeeds = []newspaper.RSSFeed{{URL: "https://publisher.example/feed", Section: "interests"}}
	deps := newspaperFixtureIO(p)
	deps.Capabilities = func() newspaperResearchCapabilities {
		cfg := newspaperFixtureConfig()
		cfg.BraveSearch.Enabled = false
		return resolveNewspaperCapabilities(cfg, p, true, true)
	}
	feedCalls := 0
	deps.Feed = func(context.Context, string) ([]newspaperHit, error) {
		feedCalls++
		body := "<rss><channel>"
		for i := 0; i < 45; i++ {
			body += fmt.Sprintf("<item><title>Original announcement %d</title><description>Research library opens</description><link>https://publisher.example/report/%d</link></item>", i, i)
		}
		return parseNewspaperFeed(p.RSSFeeds[0].URL, []byte(body+"</channel></rss>"))
	}
	deps.Search = func(_ context.Context, backend string, _ newspaperQuery, _ string) (newspaperSearchBatch, error) {
		if backend != "ddg_search" {
			t.Errorf("disabled backend: %s", backend)
		}
		return newspaperSearchBatch{}, errors.New("fixture search outage")
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 15, 1, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || feedCalls != 1 || len(draft.Stories) != 6 || stats.Candidates != 40 || stats.Pages != stats.Read+1 || stats.Coverage["interest:0"] != 6 {
		t.Fatalf("feed research: stories=%d stats=%+v calls=%d err=%v", len(draft.Stories), stats, feedCalls, err)
	}
}
