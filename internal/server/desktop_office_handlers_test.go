package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/office"
)

func TestDesktopOfficeWorkbookOptimisticLocking(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}

	path := "Documents/budget.xlsx"
	initialData, err := office.EncodeWorkbook(testWorkbook("initial"))
	if err != nil {
		t.Fatalf("EncodeWorkbook initial: %v", err)
	}
	if err := svc.WriteFileBytes(context.Background(), path, initialData, desktop.SourceUser); err != nil {
		t.Fatalf("WriteFileBytes initial: %v", err)
	}

	getResp := doOfficeWorkbookRequest(t, s, http.MethodGet, "/api/desktop/office/workbook?path="+url.QueryEscape(path), nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body %s", getResp.Code, getResp.Body.String())
	}
	version := decodeOfficeWorkbookResponse(t, getResp).OfficeVersion
	if len(version) == 0 || string(version) == "null" {
		t.Fatalf("GET response missing office_version: %s", getResp.Body.String())
	}

	saveResp := doOfficeWorkbookRequest(t, s, http.MethodPut, "/api/desktop/office/workbook", map[string]interface{}{
		"path":           path,
		"workbook":       testWorkbook("saved"),
		"office_version": json.RawMessage(version),
	})
	if saveResp.Code != http.StatusOK {
		t.Fatalf("save status = %d, body %s", saveResp.Code, saveResp.Body.String())
	}
	freshVersion := decodeOfficeWorkbookResponse(t, saveResp).OfficeVersion
	if len(freshVersion) == 0 || string(freshVersion) == "null" {
		t.Fatalf("save response missing fresh office_version: %s", saveResp.Body.String())
	}

	externalData, err := office.EncodeWorkbook(testWorkbook("external"))
	if err != nil {
		t.Fatalf("EncodeWorkbook external: %v", err)
	}
	if err := svc.WriteFileBytes(context.Background(), path, externalData, desktop.SourceUser); err != nil {
		t.Fatalf("WriteFileBytes external: %v", err)
	}
	absPath, err := svc.ResolvePath(path)
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	forcedModTime := time.Now().UTC().Add(2 * time.Second)
	if err := os.Chtimes(absPath, forcedModTime, forcedModTime); err != nil {
		t.Fatalf("Chtimes external workbook: %v", err)
	}

	staleResp := doOfficeWorkbookRequest(t, s, http.MethodPut, "/api/desktop/office/workbook", map[string]interface{}{
		"path":           path,
		"workbook":       testWorkbook("stale overwrite"),
		"office_version": json.RawMessage(freshVersion),
	})
	if staleResp.Code != http.StatusConflict {
		t.Fatalf("stale save status = %d, want %d, body %s", staleResp.Code, http.StatusConflict, staleResp.Body.String())
	}

	sameSizePath := "Documents/same-size.xlsx"
	sameSizeData, err := office.EncodeWorkbook(testWorkbook("same-size-before"))
	if err != nil {
		t.Fatalf("EncodeWorkbook same-size initial: %v", err)
	}
	if err := svc.WriteFileBytes(context.Background(), sameSizePath, sameSizeData, desktop.SourceUser); err != nil {
		t.Fatalf("WriteFileBytes same-size initial: %v", err)
	}
	sameSizeGetResp := doOfficeWorkbookRequest(t, s, http.MethodGet, "/api/desktop/office/workbook?path="+url.QueryEscape(sameSizePath), nil)
	if sameSizeGetResp.Code != http.StatusOK {
		t.Fatalf("same-size GET status = %d, body %s", sameSizeGetResp.Code, sameSizeGetResp.Body.String())
	}
	sameSizeVersion := decodeOfficeWorkbookResponse(t, sameSizeGetResp).OfficeVersion
	sameSizeAbsPath, err := svc.ResolvePath(sameSizePath)
	if err != nil {
		t.Fatalf("ResolvePath same-size workbook: %v", err)
	}
	sameSizeInfo, err := os.Stat(sameSizeAbsPath)
	if err != nil {
		t.Fatalf("stat same-size workbook: %v", err)
	}
	mutatedSameSize := mutateOneByte(sameSizeData)
	if err := os.WriteFile(sameSizeAbsPath, mutatedSameSize, 0o644); err != nil {
		t.Fatalf("write same-size workbook mutation: %v", err)
	}
	if err := os.Chtimes(sameSizeAbsPath, sameSizeInfo.ModTime(), sameSizeInfo.ModTime()); err != nil {
		t.Fatalf("restore same-size workbook mtime: %v", err)
	}
	sameSizeStaleResp := doOfficeWorkbookRequest(t, s, http.MethodPut, "/api/desktop/office/workbook", map[string]interface{}{
		"path":           sameSizePath,
		"workbook":       testWorkbook("same-size overwrite"),
		"office_version": json.RawMessage(sameSizeVersion),
	})
	if sameSizeStaleResp.Code != http.StatusConflict {
		t.Fatalf("same-size stale save status = %d, want %d, body %s", sameSizeStaleResp.Code, http.StatusConflict, sameSizeStaleResp.Body.String())
	}
	afterSameSizeStale, err := os.ReadFile(sameSizeAbsPath)
	if err != nil {
		t.Fatalf("read same-size workbook after stale save: %v", err)
	}
	if !bytes.Equal(afterSameSizeStale, mutatedSameSize) {
		t.Fatal("same-size stale workbook save overwrote externally changed content")
	}

	createPath := "Documents/new-workbook.xlsx"
	createResp := doOfficeWorkbookRequest(t, s, http.MethodPost, "/api/desktop/office/workbook", map[string]interface{}{
		"path":     createPath,
		"workbook": testWorkbook("created"),
	})
	if createResp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", createResp.Code, createResp.Body.String())
	}
	createdVersion := decodeOfficeWorkbookResponse(t, createResp).OfficeVersion
	if len(createdVersion) == 0 || string(createdVersion) == "null" {
		t.Fatalf("create response missing office_version: %s", createResp.Body.String())
	}

	missingVersionResp := doOfficeWorkbookRequest(t, s, http.MethodPut, "/api/desktop/office/workbook", map[string]interface{}{
		"path":     createPath,
		"workbook": testWorkbook("overwrite"),
	})
	if missingVersionResp.Code != http.StatusConflict {
		t.Fatalf("save without version status = %d, want %d, body %s", missingVersionResp.Code, http.StatusConflict, missingVersionResp.Body.String())
	}
}

func TestDesktopOfficeDocumentOptimisticLocking(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}

	path := "Documents/notes.txt"
	initialData := []byte("initial document")
	if err := svc.WriteFileBytes(context.Background(), path, initialData, desktop.SourceUser); err != nil {
		t.Fatalf("WriteFileBytes initial document: %v", err)
	}

	getResp := doOfficeDocumentRequest(t, s, http.MethodGet, "/api/desktop/office/document?path="+url.QueryEscape(path), nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("document GET status = %d, body %s", getResp.Code, getResp.Body.String())
	}
	version := decodeOfficeDocumentResponse(t, getResp).OfficeVersion
	if len(version) == 0 || string(version) == "null" {
		t.Fatalf("document GET response missing office_version: %s", getResp.Body.String())
	}

	saveResp := doOfficeDocumentRequest(t, s, http.MethodPut, "/api/desktop/office/document", map[string]interface{}{
		"path":           path,
		"text":           "saved document",
		"office_version": json.RawMessage(version),
	})
	if saveResp.Code != http.StatusOK {
		t.Fatalf("document save status = %d, body %s", saveResp.Code, saveResp.Body.String())
	}
	freshVersion := decodeOfficeDocumentResponse(t, saveResp).OfficeVersion
	if len(freshVersion) == 0 || string(freshVersion) == "null" {
		t.Fatalf("document save response missing fresh office_version: %s", saveResp.Body.String())
	}

	sameSizeAbsPath, err := svc.ResolvePath(path)
	if err != nil {
		t.Fatalf("ResolvePath document: %v", err)
	}
	currentData, err := os.ReadFile(sameSizeAbsPath)
	if err != nil {
		t.Fatalf("read document before mutation: %v", err)
	}
	currentInfo, err := os.Stat(sameSizeAbsPath)
	if err != nil {
		t.Fatalf("stat document before mutation: %v", err)
	}
	if err := os.WriteFile(sameSizeAbsPath, mutateOneByte(currentData), 0o644); err != nil {
		t.Fatalf("write same-size document mutation: %v", err)
	}
	if err := os.Chtimes(sameSizeAbsPath, currentInfo.ModTime(), currentInfo.ModTime()); err != nil {
		t.Fatalf("restore document mtime: %v", err)
	}

	staleResp := doOfficeDocumentRequest(t, s, http.MethodPut, "/api/desktop/office/document", map[string]interface{}{
		"path":           path,
		"text":           "stale overwrite",
		"office_version": json.RawMessage(freshVersion),
	})
	if staleResp.Code != http.StatusConflict {
		t.Fatalf("document stale save status = %d, want %d, body %s", staleResp.Code, http.StatusConflict, staleResp.Body.String())
	}
	afterStaleData, err := os.ReadFile(sameSizeAbsPath)
	if err != nil {
		t.Fatalf("read document after stale save: %v", err)
	}
	if !bytes.Equal(afterStaleData, mutateOneByte(currentData)) {
		t.Fatal("document stale save overwrote externally changed content")
	}

	createPath := "Documents/new-notes.txt"
	createResp := doOfficeDocumentRequest(t, s, http.MethodPost, "/api/desktop/office/document", map[string]interface{}{
		"path": createPath,
		"text": "created document",
	})
	if createResp.Code != http.StatusOK {
		t.Fatalf("document create status = %d, body %s", createResp.Code, createResp.Body.String())
	}
	createdVersion := decodeOfficeDocumentResponse(t, createResp).OfficeVersion
	if len(createdVersion) == 0 || string(createdVersion) == "null" {
		t.Fatalf("document create response missing office_version: %s", createResp.Body.String())
	}

	missingVersionResp := doOfficeDocumentRequest(t, s, http.MethodPut, "/api/desktop/office/document", map[string]interface{}{
		"path": createPath,
		"text": "overwrite",
	})
	if missingVersionResp.Code != http.StatusConflict {
		t.Fatalf("document save without version status = %d, want %d, body %s", missingVersionResp.Code, http.StatusConflict, missingVersionResp.Body.String())
	}
}

func TestOfficeOptimisticConcurrentCreateUsesPathLock(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	const writers = 16
	path := "Documents/concurrent-create.xlsx"

	var ready sync.WaitGroup
	var start sync.WaitGroup
	var done sync.WaitGroup
	statuses := make(chan int, writers)
	start.Add(1)
	for i := 0; i < writers; i++ {
		i := i
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			start.Wait()
			resp := doOfficeWorkbookRequest(t, s, http.MethodPost, "/api/desktop/office/workbook", map[string]interface{}{
				"path":     path,
				"workbook": testWorkbook(fmt.Sprintf("writer-%d", i)),
			})
			statuses <- resp.Code
		}()
	}
	ready.Wait()
	start.Done()
	done.Wait()
	close(statuses)

	var created, conflicts int
	for status := range statuses {
		switch status {
		case http.StatusOK:
			created++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("concurrent create returned unexpected status %d", status)
		}
	}
	if created != 1 || conflicts != writers-1 {
		t.Fatalf("concurrent create statuses: %d created, %d conflicts; want 1 created, %d conflicts", created, conflicts, writers-1)
	}
}

func newDesktopOfficeTestServer(t *testing.T) *Server {
	t.Helper()

	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(t.TempDir(), "workspace")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(t.TempDir(), "desktop.db")
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), "data")
	cfg.SQLite.MediaRegistryPath = filepath.Join(cfg.Directories.DataDir, "media_registry.db")
	cfg.SQLite.ImageGalleryPath = filepath.Join(cfg.Directories.DataDir, "image_gallery.db")
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	t.Cleanup(func() {
		if s.DesktopService != nil {
			_ = s.DesktopService.Close()
		}
	})
	return s
}

func doOfficeWorkbookRequest(t *testing.T, s *Server, method, target string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := httptest.NewRecorder()
	handleDesktopOfficeWorkbook(s).ServeHTTP(resp, req)
	return resp
}

func doOfficeDocumentRequest(t *testing.T, s *Server, method, target string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := httptest.NewRecorder()
	handleDesktopOfficeDocument(s).ServeHTTP(resp, req)
	return resp
}

type officeWorkbookTestResponse struct {
	Status        string          `json:"status"`
	OfficeVersion json.RawMessage `json:"office_version"`
}

type officeDocumentTestResponse struct {
	Status        string          `json:"status"`
	OfficeVersion json.RawMessage `json:"office_version"`
}

func decodeOfficeWorkbookResponse(t *testing.T, resp *httptest.ResponseRecorder) officeWorkbookTestResponse {
	t.Helper()

	var body officeWorkbookTestResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v; body %s", err, resp.Body.String())
	}
	return body
}

func decodeOfficeDocumentResponse(t *testing.T, resp *httptest.ResponseRecorder) officeDocumentTestResponse {
	t.Helper()

	var body officeDocumentTestResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v; body %s", err, resp.Body.String())
	}
	return body
}

func mutateOneByte(data []byte) []byte {
	mutated := bytes.Clone(data)
	if len(mutated) == 0 {
		return mutated
	}
	mutated[len(mutated)/2] ^= 0x01
	return mutated
}

func testWorkbook(value string) office.Workbook {
	return office.Workbook{
		Sheets: []office.Sheet{
			{
				Name: "Sheet1",
				Rows: [][]office.Cell{{{Value: value}}},
			},
		},
	}
}
