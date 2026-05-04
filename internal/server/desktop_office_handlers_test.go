package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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
}

func TestDesktopOfficeWorkbookCreateWithoutVersionAllowed(t *testing.T) {
	s := newDesktopOfficeTestServer(t)

	resp := doOfficeWorkbookRequest(t, s, http.MethodPost, "/api/desktop/office/workbook", map[string]interface{}{
		"path":     "Documents/new-workbook.xlsx",
		"workbook": testWorkbook("created"),
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body %s", resp.Code, resp.Body.String())
	}
	version := decodeOfficeWorkbookResponse(t, resp).OfficeVersion
	if len(version) == 0 || string(version) == "null" {
		t.Fatalf("create response missing office_version: %s", resp.Body.String())
	}
}

func TestDesktopOfficeWorkbookExistingSaveWithoutVersionRejected(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}

	path := "Documents/existing.xlsx"
	data, err := office.EncodeWorkbook(testWorkbook("existing"))
	if err != nil {
		t.Fatalf("EncodeWorkbook: %v", err)
	}
	if err := svc.WriteFileBytes(context.Background(), path, data, desktop.SourceUser); err != nil {
		t.Fatalf("WriteFileBytes: %v", err)
	}

	resp := doOfficeWorkbookRequest(t, s, http.MethodPut, "/api/desktop/office/workbook", map[string]interface{}{
		"path":     path,
		"workbook": testWorkbook("overwrite"),
	})
	if resp.Code != http.StatusConflict {
		t.Fatalf("save without version status = %d, want %d, body %s", resp.Code, http.StatusConflict, resp.Body.String())
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

type officeWorkbookTestResponse struct {
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
