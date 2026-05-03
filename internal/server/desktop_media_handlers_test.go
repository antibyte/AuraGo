package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDesktopMediaMountFilesAPI(t *testing.T) {
	t.Parallel()

	srv, dataDir := testDesktopMediaServer(t)
	audioDir := filepath.Join(dataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatalf("mkdir audio: %v", err)
	}
	if err := os.WriteFile(filepath.Join(audioDir, "song.mp3"), []byte("mp3"), 0o644); err != nil {
		t.Fatalf("write song: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/files?path=Music", nil)
	rr := httptest.NewRecorder()
	handleDesktopFiles(srv)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Files []struct {
			Name      string `json:"name"`
			Path      string `json:"path"`
			WebPath   string `json:"web_path"`
			MediaKind string `json:"media_kind"`
			MIMEType  string `json:"mime_type"`
			Mount     string `json:"mount"`
		} `json:"files"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("files = %+v", resp.Files)
	}
	got := resp.Files[0]
	if got.Name != "song.mp3" || got.Path != "Music/song.mp3" || got.WebPath != "/files/audio/song.mp3" || got.MediaKind != "audio" || got.MIMEType != "audio/mpeg" || got.Mount != "Music" {
		t.Fatalf("unexpected file entry: %+v", got)
	}
}

func TestDesktopMediaMountFilesAPIRecursivePagination(t *testing.T) {
	t.Parallel()

	srv, dataDir := testDesktopMediaServer(t)
	audioDir := filepath.Join(dataDir, "audio")
	if err := os.MkdirAll(filepath.Join(audioDir, "sets"), 0o755); err != nil {
		t.Fatalf("mkdir nested audio: %v", err)
	}
	for _, path := range []string{"song.mp3", "sets/live.ogg", "sets/demo.opus"} {
		if err := os.WriteFile(filepath.Join(audioDir, filepath.FromSlash(path)), []byte("audio"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/desktop/files?path=Music&recursive=true&limit=2", nil)
	rr := httptest.NewRecorder()
	handleDesktopFiles(srv)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Files   []struct{ Path string } `json:"files"`
		HasMore bool                    `json:"has_more"`
		Limit   int                     `json:"limit"`
		Offset  int                     `json:"offset"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Files) != 2 || !resp.HasMore || resp.Limit != 2 || resp.Offset != 0 {
		t.Fatalf("unexpected first page: %+v", resp)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/desktop/files?path=Music&recursive=true&limit=2&offset=2", nil)
	rr = httptest.NewRecorder()
	handleDesktopFiles(srv)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rr.Code, rr.Body.String())
	}
	resp = struct {
		Files   []struct{ Path string } `json:"files"`
		HasMore bool                    `json:"has_more"`
		Limit   int                     `json:"limit"`
		Offset  int                     `json:"offset"`
	}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Files) != 1 || resp.HasMore || resp.Offset != 2 {
		t.Fatalf("unexpected second page: %+v", resp)
	}
}

func TestDesktopMediaMountFilePatchAndDeleteAPI(t *testing.T) {
	t.Parallel()

	srv, dataDir := testDesktopMediaServer(t)
	photosDir := filepath.Join(dataDir, "generated_images")
	if err := os.MkdirAll(photosDir, 0o755); err != nil {
		t.Fatalf("mkdir photos: %v", err)
	}
	if err := os.WriteFile(filepath.Join(photosDir, "old.png"), []byte("png"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	patchBody := []byte(`{"old_path":"Photos/old.png","new_path":"Photos/new.png"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/desktop/file", bytes.NewReader(patchBody))
	rr := httptest.NewRecorder()
	handleDesktopFile(srv)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("patch status = %d body = %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(photosDir, "new.png")); err != nil {
		t.Fatalf("renamed image missing: %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/desktop/file?path=Photos/new.png", nil)
	rr = httptest.NewRecorder()
	handleDesktopFile(srv)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete status = %d body = %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(filepath.Join(photosDir, "new.png")); !os.IsNotExist(err) {
		t.Fatalf("image still exists or unexpected stat error: %v", err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/desktop/file", strings.NewReader(`{"old_path":"Music/a.mp3","new_path":"Photos/a.mp3"}`))
	rr = httptest.NewRecorder()
	handleDesktopFile(srv)(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("cross-mount patch status = %d, want 400", rr.Code)
	}
}

func testDesktopMediaServer(t *testing.T) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.WorkspaceDir = filepath.Join(tmp, "workspace", "virtual_desktop")
	cfg.SQLite.VirtualDesktopPath = filepath.Join(tmp, "virtual_desktop.db")
	cfg.SQLite.MediaRegistryPath = filepath.Join(tmp, "media_registry.db")
	cfg.SQLite.ImageGalleryPath = filepath.Join(tmp, "image_gallery.db")
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = filepath.Join(tmp, "workspace")
	cfg.Tools.DocumentCreator.OutputDir = filepath.Join(dataDir, "documents")
	srv := newServerFromOptions(StartOptions{
		Cfg:          cfg,
		Logger:       slog.Default(),
		AccessLogger: slog.Default(),
		ShutdownCh:   make(chan struct{}),
	})
	t.Cleanup(func() {
		srv.DesktopMu.Lock()
		if srv.DesktopService != nil {
			_ = srv.DesktopService.Close()
		}
		srv.DesktopMu.Unlock()
	})
	return srv, dataDir
}
