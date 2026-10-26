package flows

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A6: test runs and the other runs (live, agent, call) are capped per flow separately.
func TestFB4PruneRunsCapsTestAndLiveRunsSeparately(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaafa")
	day := 24 * time.Hour
	addRunAt(t, s, f, "run_aaaaaaaafaa", ModeLive, RunSuccess, storeNow.Add(-5*day))
	addRunAt(t, s, f, "run_aaaaaaaafab", ModeLive, RunError, storeNow.Add(-4*day))
	addRunAt(t, s, f, "run_aaaaaaaafac", ModeTest, RunSuccess, storeNow.Add(-3*day))
	addRunAt(t, s, f, "run_aaaaaaaafad", ModeTest, RunSuccess, storeNow.Add(-2*day))
	addRunAt(t, s, f, "run_aaaaaaaafae", ModeTest, RunCancelled, storeNow.Add(-day))
	addRunAt(t, s, f, "run_aaaaaaaafaf", ModeAgent, RunSuccess, storeNow)

	n, err := s.PruneRunsByMode(ctx, 0, 2, 1, storeNow)
	if err != nil || n != 3 {
		t.Fatalf("PruneRunsByMode = %d, %v; want 3 (the oldest live run and the two older test runs)", n, err)
	}
	for _, id := range []string{"run_aaaaaaaafaa", "run_aaaaaaaafac", "run_aaaaaaaafad"} {
		if _, _, err := s.GetRun(ctx, id); !errors.Is(err, ErrRunNotFound) {
			t.Errorf("%s must be pruned, got %v", id, err)
		}
	}
	for _, id := range []string{"run_aaaaaaaafab", "run_aaaaaaaafae", "run_aaaaaaaafaf"} {
		if _, _, err := s.GetRun(ctx, id); err != nil {
			t.Errorf("%s must remain: %v", id, err)
		}
	}
}

// Newer test runs never push a live run out, also through the PruneRuns wrapper.
func TestFB4TestRunsDoNotPushLiveRunsOut(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := createStoredFlow(t, s, "flow_aaaaaaaafb")
	addRunAt(t, s, f, "run_aaaaaaaafba", ModeLive, RunSuccess, storeNow.Add(-time.Hour))
	for i, id := range []string{"run_aaaaaaaafbb", "run_aaaaaaaafbc", "run_aaaaaaaafbd"} {
		addRunAt(t, s, f, id, ModeTest, RunSuccess, storeNow.Add(time.Duration(i)*time.Minute))
	}
	if _, err := s.PruneRuns(ctx, 0, 2, storeNow); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.GetRun(ctx, "run_aaaaaaaafba"); err != nil {
		t.Fatalf("the live run was pushed out by newer test runs: %v", err)
	}
}

func TestFB4ServiceTestRunCapDefaultsAndIsClamped(t *testing.T) {
	for _, c := range []struct {
		cfg  ServiceConfig
		want int
	}{
		{ServiceConfig{}, DefaultMaxTestRunsPerFlow},
		{ServiceConfig{MaxRunsPerFlow: 10}, 10},
		{ServiceConfig{MaxRunsPerFlow: 200, MaxTestRunsPerFlow: 5}, 5},
	} {
		s := NewService(openTestStore(t), newTestRegistry(t), &Services{Location: time.UTC}, nil, c.cfg, discardLogger())
		if s.cfg.MaxTestRunsPerFlow != c.want {
			t.Errorf("cfg %+v: test run cap = %d, want %d", c.cfg, s.cfg.MaxTestRunsPerFlow, c.want)
		}
		_ = s.Shutdown(context.Background())
	}
}
