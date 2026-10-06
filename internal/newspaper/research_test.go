package newspaper

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewspaperResearchStatsPersistAndRevocationPreventsPublication(t *testing.T) {
	ctx := context.Background()
	var readOnly atomic.Bool
	s, err := New(Options{Path: filepath.Join(t.TempDir(), "newspaper.db"), Policy: func() Policy { return Policy{Enabled: true, ReadOnly: readOnly.Load()} }, Research: func(_ context.Context, _ Profile, now time.Time, progress func(Progress)) (Draft, error) {
		draft := testDraft(now)
		progress(Progress{Phase: "editing", Source: &draft.Sources[0], Research: &ResearchStats{Searches: 6, Candidates: 120, Read: 25, Accepted: 1, Rejected: map[string]int{"stale": 3}}})
		readOnly.Store(true)
		return draft, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Store().Start(ctx, "2026-10-03", false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.execute(ctx, ctx, r, DefaultProfile())
	got, err := s.Run(ctx, r.ID)
	if err != nil || got.Status != "cancelled" || got.Research == nil || got.Research.Candidates != 120 || got.Research.Rejected["stale"] != 3 {
		t.Fatalf("run: %+v %v", got, err)
	}
	if _, err := s.Get(ctx, r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("published after permission revoked: %v", err)
	}
	sources, err := s.Store().RunSources(ctx, r.ID)
	if err != nil || len(sources) != 1 {
		t.Fatalf("evidence lost: %+v %v", sources, err)
	}
	var legacy Run
	if err := json.Unmarshal([]byte(`{"id":"old-run","status":"published"}`), &legacy); err != nil || legacy.Research != nil {
		t.Fatalf("legacy run: %+v %v", legacy, err)
	}
}

func TestRunSourceTruncationIsRecordedWithoutDroppingEditionEvidence(t *testing.T) {
	ctx := context.Background()
	p := DefaultProfile()
	s, err := New(Options{
		Path: filepath.Join(t.TempDir(), "newspaper.db"),
		Policy: func() Policy {
			return Policy{Enabled: true, Budget: BudgetConfig{Mode: "fixed", MaxPages: 1}}
		},
		Research: func(_ context.Context, _ Profile, now time.Time, progress func(Progress)) (Draft, error) {
			draft := testDraft(now)
			progress(Progress{Phase: "reading", Source: &draft.Sources[0]})
			extra := draft.Sources[0]
			extra.ID = "src-extra"
			progress(Progress{Phase: "reading", Source: &extra, Research: &ResearchStats{}})
			return draft, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run, err := s.Store().Start(ctx, time.Now().Format("2006-01-02"), false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	budget := ResolveBudget(p, BudgetConfig{Mode: "fixed", MaxPages: 1})
	s.execute(ctx, WithBudget(ctx, budget), run, p)
	got, err := s.Run(ctx, run.ID)
	if err != nil || got.Status != "published" || got.Research == nil || got.Research.SourceTruncation != 1 || got.Research.Budget == nil || got.Research.Budget.Pages != 1 {
		t.Fatalf("run: %+v err=%v", got, err)
	}
	sources, err := s.RunSources(ctx, run.ID)
	if err != nil || len(sources) != 1 || sources[0].ID != "src-1" {
		t.Fatalf("run sources: %+v err=%v", sources, err)
	}
	edition, err := s.Get(ctx, run.ID)
	if err != nil || len(edition.Sources) != 1 || edition.Sources[0].ID != "src-1" {
		t.Fatalf("edition sources: %+v err=%v", edition.Sources, err)
	}
}

func TestFailedRunPersistsResolvedBudgetWithoutProgress(t *testing.T) {
	ctx := context.Background()
	s, err := New(Options{
		Path: filepath.Join(t.TempDir(), "newspaper.db"),
		Policy: func() Policy {
			return Policy{Enabled: true}
		},
		Research: func(context.Context, Profile, time.Time, func(Progress)) (Draft, error) {
			return Draft{}, errors.New("research unavailable")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run, err := s.Store().Start(ctx, time.Now().Format("2006-01-02"), false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	budget := ResolveBudget(DefaultProfile(), BudgetConfig{Mode: "auto"})
	s.execute(ctx, WithBudget(ctx, budget), run, DefaultProfile())
	got, err := s.Run(ctx, run.ID)
	if err != nil || got.Status != "failed" || got.Research == nil || got.Research.Budget == nil || *got.Research.Budget != budget {
		t.Fatalf("failed run lost budget snapshot: run=%+v err=%v", got, err)
	}
}

func TestAutoSchedulerUsesProfileBudgetSnapshot(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 6, 7, 18, 0, 0, time.UTC)
	type observation struct {
		budget   Budget
		deadline time.Time
		ok       bool
	}
	started := make(chan observation, 1)
	s, err := New(Options{
		Path: filepath.Join(t.TempDir(), "newspaper.db"),
		Now:  func() time.Time { return now },
		Policy: func() Policy {
			return Policy{Enabled: true, MaxMinutes: 30, Budget: BudgetConfig{Mode: "auto", MaxPages: 60, MaxSearches: 32, MaxMinutes: 30}}
		},
		Research: func(runCtx context.Context, _ Profile, runTime time.Time, _ func(Progress)) (Draft, error) {
			budget, ok := BudgetFromContext(runCtx)
			deadline, _ := runCtx.Deadline()
			started <- observation{budget: budget, deadline: deadline, ok: ok}
			return testDraft(runTime), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	p, err := s.Profile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.Sections = append([]string(nil), Sections...)
	p.Interests = []string{"selected topic"}
	p.City = "Berlin"
	p.TimeZone = "UTC"
	p.ReadyTime = "08:00"
	p.Daily = true
	if _, err := s.SaveProfile(ctx, p); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-started:
		remaining := time.Until(got.deadline)
		if !got.ok || got.budget.Topics != 12 || got.budget.Pages != 168 || got.budget.Searches != 56 || got.budget.Minutes != 34 || remaining < 33*time.Minute || remaining > 34*time.Minute {
			t.Fatalf("scheduled budget: %+v deadline in %s", got, remaining)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("auto-budget scheduler did not start at its earlier lead time")
	}
}
