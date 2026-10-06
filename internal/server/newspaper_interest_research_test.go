package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/newspaper"
	"aurago/internal/scraper"
	"aurago/internal/security"
)

func TestNewspaperForeignOriginalsRetainSelectedInterests(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections, p.Interests, p.Length = nil, []string{"KI", "Batterieforschung"}, "brief"
	deps := newspaperFixtureIO(p)
	deps.Search = func(_ context.Context, _ string, query newspaperQuery, _ string) (newspaperSearchBatch, error) {
		hits := []newspaperHit{}
		for i := 0; i < 3; i++ {
			hits = append(hits, newspaperHit{Title: "Original research announcement", URL: fmt.Sprintf("https://institute-%d.example/%s/%d", i, query.Topic, i)})
		}
		return newspaperSearchBatch{Hits: hits}, nil
	}
	deps.Fetch = func(_ context.Context, raw string) (*scraper.ScrapeResult, error) {
		page := newspaperFixturePage(raw)
		quote := "The institute published new artificial intelligence results with an openly available evaluation."
		if strings.Contains(raw, "interest:1") {
			quote = "Scientists published a new lithium battery study with open measurements and reproducible methods."
		}
		if strings.HasSuffix(raw, "/0") {
			quote = "The local football team announced its new season tickets and the dates of the first matches."
		}
		page.Markdown = security.IsolateExternalData(quote + "\n\n" + strings.Repeat("The original institution provides further background and details. ", 4) + raw)
		return page, nil
	}
	var mu sync.Mutex
	seenTopics := map[string]bool{}
	deps.Complete = func(ctx context.Context, guide, input string) (string, error) {
		if strings.Contains(guide, "Task: PLAN RESEARCH") {
			return `{"queries":[{"topic":"interest:0","query":"artificial intelligence research announcement","language":"en","kind":"web"},{"topic":"interest:1","query":"lithium battery study","language":"en","kind":"web"}]}`, nil
		}
		var payload struct {
			Topic  string           `json:"topic"`
			Source newspaper.Source `json:"source"`
		}
		if err := json.Unmarshal([]byte(newspaperUnwrap(input)), &payload); err != nil {
			return "", err
		}
		if payload.Topic != "KI" && payload.Topic != "Batterieforschung" {
			return "", fmt.Errorf("editor lost selected interest: %q", payload.Topic)
		}
		mu.Lock()
		seenTopics[payload.Topic] = true
		mu.Unlock()
		// The editor can decline an unrelated original despite its search topic.
		if strings.Contains(payload.Source.Excerpt, "football") {
			return `{"headline":"","paragraphs":[]}`, nil
		}
		return newspaperFixtureComplete(ctx, guide, input)
	}
	var stats *newspaper.ResearchStats
	draft, err := runNewspaperResearch(context.Background(), p, time.Now(), 15, 8, deps, func(v newspaper.Progress) { stats = v.Research })
	if err != nil || len(draft.Stories) != 4 || stats.Coverage["interest:0"] != 2 || stats.Coverage["interest:1"] != 2 || stats.Rejected["editorial_decline"] != 2 || len(seenTopics) != 2 {
		t.Fatalf("foreign interest research: stories=%d stats=%+v topics=%v err=%v", len(draft.Stories), stats, seenTopics, err)
	}
}

func TestNewspaperShortInterestsAreUsefulFeedLeads(t *testing.T) {
	for _, interest := range []string{"KI", "AI", "5G"} {
		if !newspaperMatchesInterest("Neue "+interest+"-Forschung veröffentlicht", interest) {
			t.Errorf("short interest %q discarded before reading its original", interest)
		}
	}
}
