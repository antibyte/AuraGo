package newspaper

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestResolveAutoBudgetScalesWithDistinctTopics(t *testing.T) {
	cases := []struct {
		topics, pages, searches, overviews, minutes, candidates, stories, calls int
	}{
		{1, 36, 12, 4, 20, 80, 12, 48},
		{2, 48, 16, 6, 20, 80, 12, 48},
		{4, 72, 24, 10, 20, 128, 12, 48},
		{8, 120, 40, 18, 26, 256, 12, 48},
		{12, 168, 56, 26, 34, 384, 12, 48},
		{20, 264, 88, 42, 50, 640, 20, 80},
		{31, 396, 128, 64, 60, 800, 31, 124},
	}
	for _, tc := range cases {
		p := profileWithTopics(tc.topics)
		budget := ResolveBudget(p, BudgetConfig{Mode: "auto", MaxPages: 1, MaxSearches: 1, MaxMinutes: 1})
		if budget.Mode != "auto" || budget.Topics != tc.topics || budget.Pages != tc.pages || budget.Searches != tc.searches || budget.Overviews != tc.overviews || budget.Minutes != tc.minutes || budget.Candidates != tc.candidates || budget.Stories != tc.stories || budget.EditorCalls != tc.calls || budget.SharedPages {
			t.Errorf("topics=%d: got %+v", tc.topics, budget)
		}
	}
}

func TestResolveFixedBudgetPreservesAbsoluteLimits(t *testing.T) {
	p := profileWithTopics(2)
	p.Length = "brief"
	defaults := ResolveBudget(p, BudgetConfig{})
	if defaults.Mode != "fixed" || defaults.Pages != 60 || defaults.Searches != 32 || defaults.Overviews != 6 || defaults.Minutes != 30 || defaults.Candidates != 200 || defaults.Stories != 6 || defaults.EditorCalls != 24 || !defaults.SharedPages {
		t.Fatalf("fixed defaults: %+v", defaults)
	}
	bounded := ResolveBudget(p, BudgetConfig{Mode: "fixed", MaxPages: 10, MaxSearches: 17, MaxMinutes: 7})
	if bounded.Pages != 10 || bounded.Searches != 17 || bounded.Overviews != 6 || bounded.Minutes != 7 {
		t.Fatalf("fixed limits: %+v", bounded)
	}
	legacy := ResolveBudget(p, BudgetConfig{MaxPages: -1, MaxSearches: -1, MaxMinutes: 61})
	if legacy.Pages != 60 || legacy.Searches != 1 || legacy.Minutes != 30 {
		t.Fatalf("legacy clamps: %+v", legacy)
	}
}

func TestBudgetContextAndRunSourceRetention(t *testing.T) {
	ctx := WithBudget(context.Background(), Budget{Mode: "auto", Pages: 36})
	if got, ok := BudgetFromContext(ctx); !ok || got.Mode != "auto" || got.Pages != 36 {
		t.Fatalf("budget context: %+v, %v", got, ok)
	}

	store, err := Open(filepath.Join(t.TempDir(), "newspaper.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	run, err := store.Start(context.Background(), "2026-10-06", false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= maxResearchSources; i++ {
		inserted, truncated, err := store.RecordRunSource(context.Background(), run.ID, Source{ID: "source-" + strconv.Itoa(i)}, maxResearchSources)
		if err != nil || !inserted || truncated {
			t.Fatalf("source %d: inserted=%v truncated=%v err=%v", i, inserted, truncated, err)
		}
	}
	inserted, truncated, err := store.RecordRunSource(context.Background(), run.ID, Source{ID: "source-401"}, maxResearchSources)
	if err != nil || inserted || !truncated {
		t.Fatalf("overflow source: inserted=%v truncated=%v err=%v", inserted, truncated, err)
	}
	inserted, truncated, err = store.RecordRunSource(context.Background(), run.ID, Source{ID: "source-1"}, maxResearchSources)
	if err != nil || inserted || truncated {
		t.Fatalf("duplicate source: inserted=%v truncated=%v err=%v", inserted, truncated, err)
	}
	sources, err := store.RunSources(context.Background(), run.ID)
	if err != nil || len(sources) != maxResearchSources {
		t.Fatalf("retained %d sources: %v", len(sources), err)
	}
}

func profileWithTopics(count int) Profile {
	p := Profile{Length: "standard"}
	sections := min(count, len(Sections))
	p.Sections = append(p.Sections, Sections[:sections]...)
	for i := sections; i < count; i++ {
		p.Interests = append(p.Interests, "topic-"+strconv.Itoa(i-sections))
	}
	return p
}
