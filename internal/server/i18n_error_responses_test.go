package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/ui"
)

func TestLocalizedErrorResponsesContainMessageNotEncodedJSON(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Server.UILanguage = "de"

	tests := []struct {
		name   string
		handle http.HandlerFunc
		path   string
		want   string
	}{
		{"contacts", handleContacts(s), "/api/contacts", "Kontaktdatenbank nicht initialisiert"},
		{"knowledge", handleKnowledgeFiles(s), "/api/knowledge", "Wissensspeicher ist nicht konfiguriert"},
		{"planner", handleAppointments(s), "/api/appointments", "Planer-Datenbank nicht initialisiert"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.handle.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
			}
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body["error"] != tt.want {
				t.Fatalf("error = %q, want %q", body["error"], tt.want)
			}
		})
	}
}
