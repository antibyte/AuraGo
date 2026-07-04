package server

import (
	"aurago/internal/config"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/tools"
)

func newTestCheatsheetServer(t *testing.T) *Server {
	t.Helper()
	db, err := tools.InitCheatsheetDB(t.TempDir() + "/cheatsheets.db")
	if err != nil {
		t.Fatalf("InitCheatsheetDB() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &Server{
		CheatsheetDB: db,
		Cfg:          &config.Config{},
		Logger:       slog.Default(),
	}
}

func TestHandleCheatSheetsListReturnsGenericDBError(t *testing.T) {
	t.Parallel()

	db, err := tools.InitCheatsheetDB(t.TempDir() + "/cheatsheets.db")
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

func TestHandleCheatSheetsCRUD(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	handler := handleCheatSheets(s)
	byIDHandler := handleCheatSheetByID(s)

	// ── CREATE ──
	req := httptest.NewRequest(http.MethodPost, "/api/cheatsheets",
		strings.NewReader(`{"name":"Test Sheet","content":"Hello World"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var created tools.CheatSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("create: unmarshal error: %v", err)
	}
	if created.Name != "Test Sheet" || created.Content != "Hello World" {
		t.Fatalf("create: unexpected values: %+v", created)
	}
	if created.CreatedBy != "user" {
		t.Fatalf("create: created_by = %q, want %q", created.CreatedBy, "user")
	}

	// ── LIST ──
	req = httptest.NewRequest(http.MethodGet, "/api/cheatsheets", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("list: status = %d, want %d", rec.Code, http.StatusOK)
	}
	var list []tools.CheatSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list: unmarshal error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list: got %d items, want 1", len(list))
	}

	// ── GET by ID ──
	req = httptest.NewRequest(http.MethodGet, "/api/cheatsheets/"+created.ID, nil)
	rec = httptest.NewRecorder()
	byIDHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// ── UPDATE ──
	req = httptest.NewRequest(http.MethodPut, "/api/cheatsheets/"+created.ID,
		strings.NewReader(`{"name":"Updated Sheet","content":"New content"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	byIDHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update: status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var updated tools.CheatSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("update: unmarshal error: %v", err)
	}
	if updated.Name != "Updated Sheet" || updated.Content != "New content" {
		t.Fatalf("update: unexpected values: %+v", updated)
	}

	// ── DELETE ──
	req = httptest.NewRequest(http.MethodDelete, "/api/cheatsheets/"+created.ID, nil)
	rec = httptest.NewRecorder()
	byIDHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// ── GET after DELETE → 404 ──
	req = httptest.NewRequest(http.MethodGet, "/api/cheatsheets/"+created.ID, nil)
	rec = httptest.NewRecorder()
	byIDHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCheatSheetsPersistsTags(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	handler := handleCheatSheets(s)
	byIDHandler := handleCheatSheetByID(s)

	req := httptest.NewRequest(http.MethodPost, "/api/cheatsheets",
		strings.NewReader(`{"name":"Tagged","content":"Hello","tags":["ops"," deploy ","ops",""]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var created tools.CheatSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("create: unmarshal error: %v", err)
	}
	if got := strings.Join(created.Tags, ","); got != "deploy,ops" {
		t.Fatalf("created tags = %q, want deploy,ops", got)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/cheatsheets/"+created.ID,
		strings.NewReader(`{"tags":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	byIDHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update: status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var updated tools.CheatSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("update: unmarshal error: %v", err)
	}
	if len(updated.Tags) != 0 {
		t.Fatalf("updated tags = %#v, want empty", updated.Tags)
	}
}

func TestHandleCheatSheetsCreatedByAlwaysUser(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	handler := handleCheatSheets(s)

	// Attempt to set created_by to "agent" via HTTP — should be ignored
	req := httptest.NewRequest(http.MethodPost, "/api/cheatsheets",
		strings.NewReader(`{"name":"Agent Sheet","content":"test","created_by":"agent"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var sheet tools.CheatSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &sheet); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if sheet.CreatedBy != "user" {
		t.Fatalf("created_by = %q, want %q (HTTP should always set 'user')", sheet.CreatedBy, "user")
	}
}

func TestHandleCheatSheetsListFiltersCreatedByUser(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	handler := handleCheatSheets(s)

	userSheet, err := tools.CheatsheetCreate(s.CheatsheetDB, "User Sheet", "user content", "user")
	if err != nil {
		t.Fatalf("create user sheet: %v", err)
	}
	if _, err := tools.CheatsheetCreate(s.CheatsheetDB, "Agent Sheet", "agent content", "agent"); err != nil {
		t.Fatalf("create agent sheet: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cheatsheets?active=true&created_by=user", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var list []tools.CheatSheet
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(list) != 1 || list[0].ID != userSheet.ID {
		t.Fatalf("filtered list = %+v, want only %q", list, userSheet.ID)
	}
}

func TestHandleCheatSheetsDeleteNotFound(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	byIDHandler := handleCheatSheetByID(s)

	req := httptest.NewRequest(http.MethodDelete, "/api/cheatsheets/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	byIDHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCheatSheetsDeleteLockedForbidden(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	byIDHandler := handleCheatSheetByID(s)

	sheet, err := tools.CheatsheetCreate(s.CheatsheetDB, "Locked Sheet", "content", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	locked := true
	if _, err := tools.CheatsheetUpdate(s.CheatsheetDB, sheet.ID, nil, nil, nil, nil, &locked); err != nil {
		t.Fatalf("lock: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/cheatsheets/"+sheet.ID, nil)
	rec := httptest.NewRecorder()
	byIDHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCheatsheetContentLimit(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	handler := handleCheatSheets(s)

	// Content exceeding MaxContentChars should be rejected
	bigContent := strings.Repeat("x", tools.MaxContentChars+1)
	body := `{"name":"Big Sheet","content":"` + bigContent + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/cheatsheets",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleCheatSheetAttachmentsListMissingSheetReturnsNotFound(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	handler := handleCheatSheetAttachments(s)

	req := httptest.NewRequest(http.MethodGet, "/api/cheatsheets/missing/attachments", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestHandleCheatSheetKnowledgeAttachmentValidatesExtensionBeforeRead(t *testing.T) {
	t.Parallel()
	s := newTestCheatsheetServer(t)
	knowledgeDir := t.TempDir()
	s.Cfg.Indexing.Directories = []config.IndexingDirectory{{Path: knowledgeDir}}
	if err := os.Mkdir(filepath.Join(knowledgeDir, "image.png"), 0750); err != nil {
		t.Fatalf("mkdir image.png: %v", err)
	}
	sheet, err := tools.CheatsheetCreate(s.CheatsheetDB, "Sheet", "content", "user")
	if err != nil {
		t.Fatalf("create sheet: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/cheatsheets/"+sheet.ID+"/attachments",
		strings.NewReader(`{"source":"knowledge","filename":"image.png"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handleCheatSheetAttachments(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "extension") {
		t.Fatalf("body = %q, want extension validation error", rec.Body.String())
	}
}
