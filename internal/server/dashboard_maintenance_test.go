package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestDashboardMaintenanceStatusEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	started := time.Date(2026, 6, 10, 4, 0, 0, 0, time.UTC)
	finished := started.Add(8 * time.Minute)
	if err := stm.InsertMaintenanceRun(started, finished, "completed", memory.MaintenancePhaseResults{
		JournalRemoved: 1,
	}); err != nil {
		t.Fatalf("InsertMaintenanceRun: %v", err)
	}

	cfg := &config.Config{}
	cfg.Maintenance.Enabled = true
	cfg.Maintenance.Time = "04:00"
	s := &Server{ShortTermMem: stm, Cfg: cfg, Logger: logger}

	req := httptest.NewRequest("GET", "/api/dashboard/maintenance/status", nil)
	rec := httptest.NewRecorder()
	handleDashboardMaintenanceStatus(s).ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	maintenance := body["maintenance"].(map[string]interface{})
	if maintenance["last_status"] != "completed" {
		t.Fatalf("last_status = %v, want completed", maintenance["last_status"])
	}
	if maintenance["next_run"] == "" {
		t.Fatal("expected next_run to be populated")
	}
}