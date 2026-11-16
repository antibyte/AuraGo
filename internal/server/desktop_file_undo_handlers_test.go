package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
)

func TestDesktopFileUndoHandlerReturnsOppositeReceipt(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile(context.Background(), "Desktop/source.txt", "content", desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	_, receipt, err := svc.MovePathWithReceipt(context.Background(), "Desktop/source.txt", "Documents/source.txt", desktop.SourceUser, nil)
	if err != nil || receipt == "" {
		t.Fatalf("create receipt: %q %v", receipt, err)
	}
	body, _ := json.Marshal(map[string]string{"receipt": receipt})
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/file-undo", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handleDesktopFileUndo(s).ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Status      string `json:"status"`
		From        string `json:"from"`
		Path        string `json:"path"`
		UndoReceipt string `json:"undo_receipt"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.From != "Documents/source.txt" || result.Path != "Desktop/source.txt" || result.UndoReceipt == "" {
		t.Fatalf("unexpected undo response: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, filepath.FromSlash(result.Path)))
	if err != nil || string(data) != "content" {
		t.Fatalf("restored file=%q err=%v", data, err)
	}
}

func TestDesktopFileUndoHandlerMarksUnavailableReceipt(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	body, _ := json.Marshal(map[string]string{"receipt": "expired-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/file-undo", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handleDesktopFileUndo(s).ServeHTTP(response, req)
	if response.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result map[string]string
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["code"] != "file_undo_unavailable" {
		t.Fatalf("response=%v", result)
	}
}

func TestDesktopFilePatchReturnsUndoReceiptForFreeTarget(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile(context.Background(), "Desktop/source.txt", "content", desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"old_path": "Desktop/source.txt", "new_path": "Documents/source.txt"})
	req := httptest.NewRequest(http.MethodPatch, "/api/desktop/file", bytes.NewReader(body))
	req.Header.Set("If-None-Match", "*")
	response := httptest.NewRecorder()
	handleDesktopFile(s).ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Path        string `json:"path"`
		UndoReceipt string `json:"undo_receipt"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Path != "Documents/source.txt" || result.UndoReceipt == "" {
		t.Fatalf("PATCH response omitted receipt: %+v", result)
	}
}

func TestDesktopTrashHandlerKeepsReceiptsOutOfEvents(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, hub, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile(context.Background(), "Desktop/delete.txt", "content", desktop.SourceUser); err != nil {
		t.Fatal(err)
	}
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	body, _ := json.Marshal(map[string]interface{}{"paths": []string{"Desktop/delete.txt"}})
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/trash", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handleDesktopTrash(s).ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		Moves []desktop.TrashMove `json:"moves"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Moves) != 1 || result.Moves[0].UndoReceipt == "" {
		t.Fatalf("trash response omitted receipt: %+v", result.Moves)
	}
	if hub == nil {
		t.Fatal("desktop event hub was not initialized")
	}
	select {
	case event := <-events:
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "undo_receipt") || strings.Contains(string(encoded), result.Moves[0].UndoReceipt) {
			t.Fatalf("receipt leaked through desktop event: %s", encoded)
		}
	case <-time.After(time.Second):
		t.Fatal("trash event was not broadcast")
	}
}
