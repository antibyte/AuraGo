package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"
)

func TestNewServerFromOptionsReusesInjectedBudgetWithoutAnotherWriter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	trackerCfg := &config.Config{}
	trackerCfg.Budget.Enabled = true
	tracker := budget.NewTracker(trackerCfg, logger, t.TempDir())
	t.Cleanup(tracker.Flush)
	if tracker == nil {
		t.Fatal("fixture budget tracker is disabled")
	}
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Directories.DataDir = t.TempDir()
	cfg.SQLite.NetworkSharesPath = filepath.Join(t.TempDir(), "network-shares.db")
	s := newServerFromOptions(StartOptions{Cfg: cfg, Logger: logger, BudgetTracker: tracker})
	if s.NetworkShares != nil {
		t.Cleanup(func() { _ = s.NetworkShares.Close() })
	}
	if s.BudgetTracker != tracker {
		t.Fatal("server replaced the originating budget tracker")
	}
	// A second NewTracker would create this ledger immediately and arm its own
	// reset timer, even if Start replaced its pointer with the injected tracker.
	if _, err := os.Stat(filepath.Join(cfg.Directories.DataDir, "budget.json")); !os.IsNotExist(err) {
		t.Fatalf("server initialized an independent budget writer: %v", err)
	}
}

func TestTrackHTTPPassesBudgetOwnershipAndRequestCancellation(t *testing.T) {
	tracker := budget.NewTracker(&config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	defer tracker.Flush()
	s := &Server{BudgetTracker: tracker}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.trackHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, category := budget.Tracking(r.Context())
		if got != tracker || category != "chat" {
			t.Fatalf("HTTP budget ownership = %p/%q, want %p/chat", got, category, tracker)
		}
		cancel()
		if r.Context().Err() != context.Canceled {
			t.Fatal("budget context discarded request cancellation")
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil).WithContext(parent))
}
