package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestHandleAgentMailStatusReportsDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodGet, "/api/agentmail/status", nil)
	rec := httptest.NewRecorder()
	handleAgentMailStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "disabled" {
		t.Fatalf("status = %#v, want disabled", payload["status"])
	}
}

func TestHandleAgentMailTestListsInboxes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/inboxes" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing bearer auth")
		}
		_, _ = w.Write([]byte(`{"inboxes":[{"inbox_id":"inbox-1","email_address":"bot@example.com"}]}`))
	}))
	defer upstream.Close()

	cfg := &config.Config{}
	cfg.AgentMail.Enabled = true
	cfg.AgentMail.APIKey = "test-key"
	cfg.AgentMail.BaseURL = upstream.URL
	s := &Server{Cfg: cfg, Logger: slog.Default()}

	req := httptest.NewRequest(http.MethodPost, "/api/agentmail/test", nil)
	rec := httptest.NewRecorder()
	handleAgentMailTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("status = %#v, want ok; body=%s", payload["status"], rec.Body.String())
	}
	if payload["inbox_count"].(float64) != 1 {
		t.Fatalf("inbox_count = %#v, want 1", payload["inbox_count"])
	}
}

func TestAgentMailRelaySuppressedInEggMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.AgentMail.Enabled = true
	cfg.AgentMail.RelayToAgent = true
	cfg.AgentMail.APIKey = "test-key"
	cfg.AgentMail.InboxID = "inbox-1"
	cfg.AgentMail.BaseURL = "http://example.invalid"
	cfg.EggMode.Enabled = true

	s := &Server{Cfg: cfg, Logger: slog.Default()}
	s.configureAgentMailRelay(cfg)
	if s.AgentMailService != nil {
		t.Fatal("AgentMailService should not start in egg mode")
	}
}

func TestLoadAgentMailRelayCheatsheet(t *testing.T) {
	db, err := tools.InitCheatsheetDB(filepath.Join(t.TempDir(), "cheatsheets.db"))
	if err != nil {
		t.Fatalf("InitCheatsheetDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	sheet, err := tools.CheatsheetCreate(db, "Mail policy", "Triage and summarize first.", "user")
	if err != nil {
		t.Fatalf("CheatsheetCreate: %v", err)
	}

	s := &Server{CheatsheetDB: db, Logger: slog.Default()}
	got := s.loadAgentMailRelayCheatsheet(sheet.ID)
	if got.ID != sheet.ID || got.Name != "Mail policy" || got.Content != "Triage and summarize first." {
		t.Fatalf("relay cheatsheet = %+v", got)
	}
}
