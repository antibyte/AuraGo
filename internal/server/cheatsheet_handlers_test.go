package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/tools"
)

func TestHandleCheatSheetsListReturnsGenericDBError(t *testing.T) {
	t.Parallel()

	db, err := tools.InitCheatsheetDB(t.TempDir() + "\\cheatsheets.db")
	if err != nil {
		t.Fatalf("InitCheatsheetDB() error = %v", err)
	}
	_ = db.Close()

	s := &Server{
		CheatsheetDB: db,
		Logger:       slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cheatsheets", nil)
	rec := httptest.NewRecorder()

	handleCheatSheets(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Failed to load cheat sheets") || strings.Contains(strings.ToLower(body), "database is closed") {
		t.Fatalf("expected generic db error, got %q", body)
	}
}
