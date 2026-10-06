package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/newspaper"
)

func TestNewspaperBudgetPreviewBeforeActivation(t *testing.T) {
	s, reader, _ := testDesktopPermissionServer(t)
	s.Cfg.VirtualDesktop.Enabled = false
	s.Cfg.Newspaper.BudgetMode = "fixed"
	admin, _, err := s.TokenManager.Create("budget preview", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := newspaperAutoFixture(31)
	body, _ := json.Marshal(map[string]any{"profile": p, "budget_mode": "auto", "overview_sources": []string{"google_news"}})
	request := func(token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/desktop/newspaper/budget-preview", strings.NewReader(string(body)))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.handleNewspaper(w, r)
		return w
	}
	if w := request(reader); w.Code != http.StatusForbidden {
		t.Fatalf("reader preview: %d", w.Code)
	}
	w := request(admin)
	var response struct {
		Budget newspaper.Budget `json:"effective_budget"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Budget.Pages != 396 || response.Budget.Stories != 31 || response.Budget.Minutes != 60 {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	if s.Cfg.Newspaper.BudgetMode != "fixed" || s.Newspaper != nil {
		t.Fatal("preview mutated configuration or started a service")
	}
}

func TestNewspaperInvalidConfigNeverReplacesSavedFile(t *testing.T) {
	for _, setup := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		original := []byte("server:\n  host: 127.0.0.1\nnewspaper:\n  budget_mode: fixed\n")
		if err := os.WriteFile(path, original, 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConfigPath = path
		s := &Server{Cfg: cfg, Logger: slog.Default()}
		if setup {
			_, err = applyConfigPatch(s, map[string]interface{}{"newspaper": map[string]interface{}{"budget_mode": "unbounded"}})
			if err == nil || !strings.Contains(err.Error(), "newspaper.budget_mode") {
				t.Fatalf("setup validation: %v", err)
			}
		} else {
			w := httptest.NewRecorder()
			handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"newspaper":{"overview_sources":["unknown-provider"]}}`)))
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "newspaper.overview_sources") {
				t.Fatalf("config validation: %d %s", w.Code, w.Body.String())
			}
		}
		saved, err := os.ReadFile(path)
		if err != nil || string(saved) != string(original) {
			t.Fatalf("invalid config replaced saved file: %v", err)
		}
	}
}
