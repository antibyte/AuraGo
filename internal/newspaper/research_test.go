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
