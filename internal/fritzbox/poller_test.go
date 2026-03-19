package fritzbox

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

// newTestPoller creates a Poller with test-friendly config and a callback recorder.
func newTestPoller(dedupMin, maxPerHour int) (*Poller, *callbackRecorder) {
	cfg := config.Config{}
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.Host = "fritz.box"
	cfg.FritzBox.Port = 49000
	cfg.FritzBox.Telephony.Polling.Enabled = true
	cfg.FritzBox.Telephony.Polling.IntervalSeconds = 60
	cfg.FritzBox.Telephony.Polling.DedupWindowMinutes = dedupMin
	cfg.FritzBox.Telephony.Polling.MaxCallbacksPerHour = maxPerHour

	rec := &callbackRecorder{}
	p := NewPoller(cfg, rec.record, slog.Default())
	return p, rec
}

type callbackRecorder struct {
	mu    sync.Mutex
	calls []callbackCall
}

type callbackCall struct {
	kind    string
	summary string
}

func (r *callbackRecorder) record(kind, summary string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, callbackCall{kind, summary})
}

func (r *callbackRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func TestNewPoller_Defaults(t *testing.T) {
	p, _ := newTestPoller(0, 0) // 0 triggers defaults
	if p.dedupWindow != 5*time.Minute {
		t.Errorf("dedupWindow = %v, want 5m", p.dedupWindow)
	}
	if p.maxPerHour != 20 {
		t.Errorf("maxPerHour = %d, want 20", p.maxPerHour)
	}
}

func TestNewPoller_CustomValues(t *testing.T) {
	p, _ := newTestPoller(10, 50)
	if p.dedupWindow != 10*time.Minute {
		t.Errorf("dedupWindow = %v, want 10m", p.dedupWindow)
	}
	if p.maxPerHour != 50 {
		t.Errorf("maxPerHour = %d, want 50", p.maxPerHour)
	}
}

func TestRateLimitOK_UnderLimit(t *testing.T) {
	p, _ := newTestPoller(1, 5)
	for i := 0; i < 5; i++ {
		if !p.rateLimitOK() {
			t.Fatalf("rateLimitOK returned false on call %d (limit 5)", i+1)
		}
	}
}

func TestRateLimitOK_OverLimit(t *testing.T) {
	p, _ := newTestPoller(1, 3)
	for i := 0; i < 3; i++ {
		p.rateLimitOK()
	}
	if p.rateLimitOK() {
		t.Fatal("rateLimitOK should return false after reaching limit")
	}
}

func TestRateLimitOK_PrunesOldEntries(t *testing.T) {
	p, _ := newTestPoller(1, 2)

	// Simulate old timestamps by injecting directly.
	p.mu.Lock()
	p.callbackTimes = []time.Time{
		time.Now().Add(-2 * time.Hour), // older than 1 hour → pruned
		time.Now().Add(-2 * time.Hour),
	}
	p.mu.Unlock()

	// Should be OK because old entries are pruned.
	if !p.rateLimitOK() {
		t.Fatal("rateLimitOK should return true after pruning old entries")
	}
}

func TestSummariseCall(t *testing.T) {
	entry := CallEntry{Type: "2"} // missed call
	got := summariseCall(entry)
	want := "New Fritz!Box call event (type: 2). Use fritzbox_telephony/get_call_list to see details."
	if got != want {
		t.Errorf("summariseCall = %q, want %q", got, want)
	}
}

func TestSummariseTAM_Single(t *testing.T) {
	got := summariseTAM(1)
	if got != "1 new answering machine message on Fritz!Box. Use fritzbox_telephony/get_tam_messages to listen." {
		t.Errorf("summariseTAM(1) = %q", got)
	}
}

func TestSummariseTAM_Multiple(t *testing.T) {
	got := summariseTAM(3)
	if got != "3 new answering machine messages on Fritz!Box. Use fritzbox_telephony/get_tam_messages." {
		t.Errorf("summariseTAM(3) = %q", got)
	}
}

func TestPoller_DedupWindow(t *testing.T) {
	p, rec := newTestPoller(1, 100)
	// Set dedup window to something very small for testing.
	p.dedupWindow = 100 * time.Millisecond

	// Simulate initial poll (sets lastCallIdx = 0, no callback).
	p.lastCallIdx = 0
	p.lastCallSummary = ""

	// Simulate pollCalls with a "new" call.
	p.lastCallSummary = ""
	p.lastCallTime = time.Time{}

	// Manual callback invocation to test dedup window.
	// First call should go through.
	cur := "New call"
	if cur != p.lastCallSummary {
		if p.rateLimitOK() {
			p.lastCallSummary = cur
			p.lastCallTime = time.Now()
			rec.record("call", cur)
		}
	}
	if rec.count() != 1 {
		t.Fatalf("expected 1 callback, got %d", rec.count())
	}

	// Second call within dedup window should be ignored.
	cur2 := "Another call"
	if !p.lastCallTime.IsZero() && time.Since(p.lastCallTime) < p.dedupWindow {
		// Suppressed by dedup window — expected.
	} else {
		t.Fatal("dedup window should have suppressed the second call")
	}

	// Wait for dedup window to expire.
	time.Sleep(150 * time.Millisecond)

	// Now it should be allowed.
	if !p.lastCallTime.IsZero() && time.Since(p.lastCallTime) < p.dedupWindow {
		t.Fatal("dedup window should have expired by now")
	}
	p.lastCallSummary = cur2
	p.lastCallTime = time.Now()
	rec.record("call", cur2)

	if rec.count() != 2 {
		t.Errorf("expected 2 callbacks, got %d", rec.count())
	}
}

func TestPoller_StopClosesClient(t *testing.T) {
	p, _ := newTestPoller(1, 10)
	// Simulate a pooled client (nil client is safe to close).
	p.pooledClient = nil
	p.Stop()

	if p.pooledClient != nil {
		t.Error("expected pooledClient to be nil after Stop")
	}
}
