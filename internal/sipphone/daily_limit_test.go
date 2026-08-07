package sipphone

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emiago/diago/diagotest"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	_ "modernc.org/sqlite"
)

func TestStoreMigratesV2AndBackfillsBrowserMediaMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sip_calls.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE sip_calls (
id TEXT PRIMARY KEY, direction TEXT NOT NULL, remote_party TEXT NOT NULL, started_at INTEGER NOT NULL,
answered_at INTEGER, ended_at INTEGER, state TEXT NOT NULL, end_reason TEXT NOT NULL DEFAULT '',
backend TEXT NOT NULL, session_id TEXT NOT NULL DEFAULT '', persist_transcripts INTEGER NOT NULL DEFAULT 1);
INSERT INTO sip_calls(id,direction,remote_party,started_at,state,backend) VALUES
('browser','outbound','redacted',1,'ended','browser'),('agent','outbound','redacted',2,'ended','classic');
PRAGMA user_version=2;`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	calls, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	modes := map[string]string{}
	for _, call := range calls {
		modes[call.ID] = call.MediaMode
	}
	if modes["browser"] != MediaModeBrowser || modes["agent"] != MediaModeAgent {
		t.Fatalf("unexpected migrated media modes: %#v", modes)
	}
}

func TestStoreAgentDailyAdmissionIsAtomicAndPersistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sip_calls.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 7, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1)
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for index := 0; index < 20; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			record := CallRecord{ID: randomCallID(), Direction: "outbound", RemoteParty: "redacted", StartedAt: start.Add(time.Duration(index) * time.Minute), State: StateConnecting, Backend: "classic", MediaMode: MediaModeAgent}
			_, ok, admissionErr := store.AdmitAgentOutbound(context.Background(), record, start, end, 10)
			if admissionErr != nil {
				t.Errorf("admission %d: %v", index, admissionErr)
				return
			}
			if ok {
				admitted.Add(1)
			}
		}(index)
	}
	wg.Wait()
	if got := admitted.Load(); got != 10 {
		t.Fatalf("admitted calls = %d, want 10", got)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	used, err := store.CountAgentOutbound(context.Background(), start, end)
	if err != nil || used != 10 {
		t.Fatalf("persistent daily usage = %d, err=%v", used, err)
	}
}

func TestLocalDayBoundsAreDSTSafe(t *testing.T) {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}
	previous := time.Local
	time.Local = location
	t.Cleanup(func() { time.Local = previous })
	for _, test := range []struct {
		now      time.Time
		duration time.Duration
	}{
		{time.Date(2026, 3, 29, 12, 0, 0, 0, location), 23 * time.Hour},
		{time.Date(2026, 10, 25, 12, 0, 0, 0, location), 25 * time.Hour},
	} {
		start, end := localDayBounds(test.now)
		if got := end.Sub(start); got != test.duration {
			t.Fatalf("day %s duration = %v, want %v", start.Format("2006-01-02"), got, test.duration)
		}
	}
}

func TestManagerCountsRejectedAgentCallsButExemptsBrowser(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.BrowserMedia.Enabled = true
	cfg.Voice.MaxOutboundCallsPerDay = 1
	cfg.Outbound.AllowedUsers = nil
	cfg.Outbound.AllowedE164Prefixes = nil
	ua, err := sipgo.NewUA()
	if err != nil {
		t.Fatal(err)
	}
	defer ua.Close()
	var invites atomic.Int32
	endpoint := diagotest.NewDiagoClientTest(ua, func(req *sip.Request) *sip.Response {
		if req.Method == sip.INVITE {
			invites.Add(1)
			return sip.NewResponseFromRequest(req, sip.StatusBusyHere, "Busy Here", nil)
		}
		return sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	})
	manager, err := NewManager(cfg, t.TempDir(), readyTestBackendFactory, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.endpoint = endpoint
	manager.rootCtx = context.Background()
	if _, err := manager.Dial(context.Background(), "sip:alice@example.com"); err != nil {
		t.Fatal(err)
	}
	waitForNoActiveCall(t, manager)
	if _, err := manager.DialBrowser(context.Background(), "sip:bob@example.com", &recordingMediaPeer{}); err != nil {
		t.Fatal(err)
	}
	waitForNoActiveCall(t, manager)
	if _, err := manager.Dial(context.Background(), "sip:carol@example.com"); !errors.Is(err, ErrAgentDailyCallLimit) {
		t.Fatalf("second agent dial error = %v, want daily limit", err)
	}
	usage, err := manager.DailyAgentCallUsage(context.Background())
	if err != nil || usage.Used != 1 || usage.Remaining != 0 {
		t.Fatalf("daily usage = %+v, err=%v", usage, err)
	}
	if got := invites.Load(); got != 2 {
		t.Fatalf("INVITE count = %d, want one agent and one browser attempt", got)
	}
}

func TestManagerAgentDialFailsClosedWhenHistoryIsUnavailable(t *testing.T) {
	cfg := validTestSIPConfig()
	ua, err := sipgo.NewUA()
	if err != nil {
		t.Fatal(err)
	}
	defer ua.Close()
	var invites atomic.Int32
	endpoint := diagotest.NewDiagoClientTest(ua, func(req *sip.Request) *sip.Response {
		if req.Method == sip.INVITE {
			invites.Add(1)
		}
		return sip.NewResponseFromRequest(req, sip.StatusBusyHere, "Busy Here", nil)
	})
	manager, err := NewManager(cfg, t.TempDir(), readyTestBackendFactory, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.endpoint = endpoint
	manager.rootCtx = context.Background()
	manager.mu.Unlock()
	if _, err := manager.Dial(context.Background(), "sip:alice@example.com"); err == nil || !strings.Contains(err.Error(), "daily call admission failed") {
		t.Fatalf("Dial error = %v, want fail-closed history error", err)
	}
	if invites.Load() != 0 {
		t.Fatal("history failure still sent an INVITE")
	}
}

func waitForNoActiveCall(t *testing.T, manager *Manager) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if manager.Status().ActiveCall == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("SIP call did not finish")
}
