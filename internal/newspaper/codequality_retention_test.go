package newspaper

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneScheduledRunsPreservesCurrentClaimAndRollsBackFailure(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "newspaper.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { store.Close() }()
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	latest := ""
	for day := 0; day < 100; day++ {
		now := base.AddDate(0, 0, day)
		latest = now.Format("2006-01-02")
		run, err := store.StartScheduled(ctx, latest, now)
		if err != nil {
			t.Fatal(err)
		}
		run.Status = "failed"
		if err := store.UpdateRun(ctx, run, "fixture completed"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.db.Exec(`CREATE TRIGGER reject_prune BEFORE DELETE ON newspaper_runs BEGIN SELECT RAISE(ABORT, 'fixture prune blocked'); END`); err != nil {
		t.Fatal(err)
	}
	if err := store.Prune(ctx, 30); err == nil {
		t.Fatal("expected injected delete failure")
	}
	var claims int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM newspaper_scheduler_attempts`).Scan(&claims); err != nil || claims != 100 {
		t.Fatalf("claims=%d err=%v", claims, err)
	}
	if _, err := store.db.Exec(`DROP TRIGGER reject_prune`); err != nil {
		t.Fatal(err)
	}
	if err := store.Prune(ctx, 30); err != nil {
		t.Fatal(err)
	}
	var runs int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM newspaper_runs`).Scan(&runs); err != nil || runs != 60 {
		t.Fatalf("runs=%d err=%v", runs, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.StartScheduled(ctx, latest, base.AddDate(0, 0, 99)); !errors.Is(err, ErrConflict) {
		t.Fatalf("current date replayed after restart: %v", err)
	}
}

func TestCancelledResearchCannotPublishValidDraftButDeadlineCan(t *testing.T) {
	for _, fixture := range []struct {
		name      string
		cancelled bool
		deadline  bool
	}{{"cancel", true, false}, {"deadline", false, true}, {"cancel_after_deadline", true, true}} {
		t.Run(fixture.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			researchCtx := ctx
			if fixture.deadline {
				var stop context.CancelFunc
				researchCtx, stop = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer stop()
			}
			s, err := New(Options{Path: filepath.Join(t.TempDir(), "newspaper.db"), Policy: func() Policy { return Policy{Enabled: true} }, Research: func(_ context.Context, _ Profile, now time.Time, _ func(Progress)) (Draft, error) {
				if fixture.cancelled {
					cancel()
				}
				return testDraft(now), researchCtx.Err()
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r, err := s.Store().Start(context.Background(), "2026-10-03", false, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			s.execute(ctx, researchCtx, r, DefaultProfile())
			e, err := s.Get(context.Background(), r.ID)
			if fixture.cancelled && !errors.Is(err, ErrNotFound) {
				t.Fatalf("cancel published: %v", err)
			}
			if !fixture.cancelled && (err != nil || !e.Partial) {
				t.Fatalf("deadline lost valid partial draft: partial=%v err=%v", e.Partial, err)
			}
			if fixture.cancelled {
				run, err := s.Run(context.Background(), r.ID)
				if err != nil || run.Status != "cancelled" {
					t.Fatalf("cancelled run: %s err=%v", run.Status, err)
				}
			}
		})
	}
}
