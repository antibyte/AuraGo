package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// The evaluation parent proves only its connection here. A game must still
// provide the normal build-bound canvas and gameplay reports to pass validation.
type gameMakerEvalBrowserProbe struct {
	mu       sync.Mutex
	lastPoll time.Time
	accepted int
	rejected int
}

func (p *gameMakerEvalBrowserProbe) polled(request *http.Request, now time.Time) {
	// Operator status queries must not make a disconnected browser look alive.
	if request.URL.Query().Get("browser") != "1" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastPoll = now
}

func (p *gameMakerEvalBrowserProbe) reported(accepted bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if accepted {
		p.accepted++
	} else {
		p.rejected++
	}
}

func (p *gameMakerEvalBrowserProbe) resetReports() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accepted, p.rejected = 0, 0
}

func (p *gameMakerEvalBrowserProbe) snapshot(now time.Time) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	var age any
	connected := false
	if !p.lastPoll.IsZero() {
		elapsed := now.Sub(p.lastPoll)
		if elapsed < 0 {
			elapsed = 0
		}
		age = elapsed.Seconds()
		connected = elapsed <= 5*time.Second
	}
	return map[string]any{"connected": connected, "last_poll_seconds_ago": age, "accepted_reports": p.accepted, "rejected_reports": p.rejected}
}

func waitForGameMakerEvalBrowser(ctx context.Context, probe *gameMakerEvalBrowserProbe) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if probe.snapshot(time.Now())["connected"] == true {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func TestGameMakerEvalBrowserProbe(t *testing.T) {
	probe := &gameMakerEvalBrowserProbe{}
	now := time.Now()
	probe.polled(httptest.NewRequest("GET", "/state", nil), now)
	if got := probe.snapshot(now); got["connected"] != false || got["last_poll_seconds_ago"] != nil {
		t.Fatalf("missing browser is not connected: %v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForGameMakerEvalBrowser(ctx, probe); err != context.Canceled {
		t.Fatalf("disconnected evaluation must be cancellable: %v", err)
	}
	probe.polled(httptest.NewRequest("GET", "/state?browser=1", nil), now)
	if err := waitForGameMakerEvalBrowser(context.Background(), probe); err != nil {
		t.Fatal(err)
	}
	probe.reported(true)
	probe.reported(false)
	got := probe.snapshot(now.Add(6 * time.Second))
	if got["connected"] != false || got["accepted_reports"] != 1 || got["rejected_reports"] != 1 {
		t.Fatalf("stale connection must not count as a game report: %v", got)
	}
	probe.resetReports()
	got = probe.snapshot(now)
	if got["connected"] != true || got["accepted_reports"] != 0 || got["rejected_reports"] != 0 {
		t.Fatalf("next task retains connection, not old reports: %v", got)
	}
}
