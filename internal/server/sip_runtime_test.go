package server

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/sipphone"
)

func TestSIPStartupCleanupResolvesOnlyAfterAllSessionsArePurged(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	memPath := filepath.Join(dir, "memory.db")
	mem, err := memory.NewSQLiteMemory(memPath, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer mem.Close()
	if err := mem.InitJournalTables(); err != nil {
		t.Fatal(err)
	}
	db, err := planner.InitDB(filepath.Join(dir, "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, fingerprint := range []string{"sip_transient_session_purge_failed", "sip_unrelated"} {
		if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{Source: "sip", Title: "SIP operational issue", Detail: "Previous failure", Severity: "high", Fingerprint: fingerprint}); err != nil {
			t.Fatal(err)
		}
	}
	store, err := sipphone.OpenStore(filepath.Join(dir, "sip_calls.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range []string{"sip-failed", "sip-healthy"} {
		if err := store.Upsert(context.Background(), sipphone.CallRecord{ID: session, SessionID: session, Direction: "inbound", State: sipphone.StateEnded, StartedAt: time.Now().Add(-72 * time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	fixture, err := sql.Open("sqlite", memPath)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	for _, session := range []string{"sip-failed", "sip-healthy"} {
		if _, err := fixture.Exec(`INSERT INTO messages(session_id,role,content) VALUES (?,'user','test transcript')`, session); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.Exec(`CREATE TRIGGER fail_sip_purge BEFORE DELETE ON messages WHEN OLD.session_id='sip-failed' BEGIN SELECT RAISE(ABORT,'test purge failure'); END`); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	config.ApplySIPDefaults(&cfg.SIP)
	cfg.Directories.DataDir = dir
	cfg.Runtime.IsDocker = true
	cfg.SIP.HistoryRetentionDays = 1
	s := &Server{Cfg: cfg, Logger: logger, ShortTermMem: mem, PlannerDB: db}
	previous := sipphone.DefaultManager()
	t.Cleanup(func() { sipphone.SetDefaultManager(previous) })
	start := func(ctx context.Context) {
		t.Helper()
		if err := s.initSIP(ctx); err != nil {
			t.Fatal(err)
		}
		if s.SIPBrowserMedia != nil {
			media := s.SIPBrowserMedia
			t.Cleanup(func() { _ = media.Close() })
		}
		manager := s.SIPPhone
		t.Cleanup(func() { _ = manager.Close() })
	}
	start(context.Background())
	var status string
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint='sip_transient_session_purge_failed'`).Scan(&status); err != nil || status != "open" {
		t.Fatalf("partial cleanup status=%q err=%v", status, err)
	}
	var failed, healthy int
	if err := fixture.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id='sip-failed'`).Scan(&failed); err != nil {
		t.Fatal(err)
	}
	if err := fixture.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id='sip-healthy'`).Scan(&healthy); err != nil {
		t.Fatal(err)
	}
	if failed != 1 || healthy != 0 {
		t.Fatalf("partial cleanup retained failed=%d healthy=%d rows", failed, healthy)
	}
	history, err := sql.Open("sqlite", filepath.Join(dir, "sip_calls.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer history.Close()
	var calls int
	if err := history.QueryRow(`SELECT COUNT(*) FROM sip_calls`).Scan(&calls); err != nil || calls != 2 {
		t.Fatalf("failed cleanup lost its retry references: calls=%d err=%v", calls, err)
	}
	_ = s.SIPPhone.Close()
	if s.SIPBrowserMedia != nil {
		_ = s.SIPBrowserMedia.Close()
	}
	if _, err := fixture.Exec(`DROP TRIGGER fail_sip_purge`); err != nil {
		t.Fatal(err)
	}
	start(context.Background())
	var occurrences int
	if err := db.QueryRow(`SELECT status,occurrences FROM operational_issues WHERE fingerprint='sip_transient_session_purge_failed'`).Scan(&status, &occurrences); err != nil || status != "done" || occurrences != 2 {
		t.Fatalf("recovered cleanup status=%q occurrences=%d err=%v", status, occurrences, err)
	}
	if err := fixture.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id IN ('sip-failed','sip-healthy')`).Scan(&failed); err != nil || failed != 0 {
		t.Fatalf("recovered cleanup retained %d transcript rows, err=%v", failed, err)
	}
	if err := history.QueryRow(`SELECT COUNT(*) FROM sip_calls`).Scan(&calls); err != nil || calls != 0 {
		t.Fatalf("successful cleanup did not apply history retention: calls=%d err=%v", calls, err)
	}
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint='sip_unrelated'`).Scan(&status); err != nil || status != "open" {
		t.Fatalf("unrelated issue status=%q err=%v", status, err)
	}
}

func TestSIPStartupCleanupPreservesIssueWhenUnavailable(t *testing.T) {
	for _, reason := range []string{"missing_memory", "cancelled"} {
		t.Run(reason, func(t *testing.T) {
			dir := t.TempDir()
			db, err := planner.InitDB(filepath.Join(dir, "planner.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := planner.RecordOperationalIssue(db, planner.OperationalIssue{Source: "sip", Title: "SIP operational issue", Detail: "Previous failure", Severity: "high", Fingerprint: "sip_transient_session_purge_failed"}); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{}
			config.ApplySIPDefaults(&cfg.SIP)
			cfg.Directories.DataDir = dir
			cfg.Runtime.IsDocker = true
			s := &Server{Cfg: cfg, PlannerDB: db, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
			ctx := context.Background()
			if reason == "cancelled" {
				mem, err := memory.NewSQLiteMemory(":memory:", s.Logger)
				if err != nil {
					t.Fatal(err)
				}
				defer mem.Close()
				s.ShortTermMem = mem
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			previous := sipphone.DefaultManager()
			defer sipphone.SetDefaultManager(previous)
			if err := s.initSIP(ctx); err != nil && reason != "cancelled" {
				t.Fatal(err)
			}
			if s.SIPPhone != nil {
				defer s.SIPPhone.Close()
			}
			if s.SIPBrowserMedia != nil {
				defer s.SIPBrowserMedia.Close()
			}
			var status string
			var occurrences int
			if err := db.QueryRow(`SELECT status,occurrences FROM operational_issues WHERE fingerprint='sip_transient_session_purge_failed'`).Scan(&status, &occurrences); err != nil || status != "open" || occurrences != 1 {
				t.Fatalf("unverified cleanup status=%q occurrences=%d err=%v", status, occurrences, err)
			}
		})
	}
}
