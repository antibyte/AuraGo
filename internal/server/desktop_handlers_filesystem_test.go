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
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/desktop"
)

func newDesktopFilesystemTestServer(t *testing.T) *Server {
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

func TestDesktopFilesystemSearch(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}

	// Create some files
	_ = svc.WriteFileBytes(context.Background(), "folder1/file1.txt", []byte("hello"), desktop.SourceUser)
	_ = svc.WriteFileBytes(context.Background(), "folder1/subfolder/file2.log", []byte("world"), desktop.SourceUser)
	_ = svc.WriteFileBytes(context.Background(), "folder2/file3.txt", []byte("other"), desktop.SourceUser)

	// Query /api/desktop/search?path=folder1&query=file
	req := httptest.NewRequest(http.MethodGet, "/api/desktop/search?path=folder1&query=file", nil)
	resp := httptest.NewRecorder()
	handleDesktopSearch(s).ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("search status = %d, body %s", resp.Code, resp.Body.String())
	}

	var searchResp struct {
		Status string `json:"status"`
		Files  []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		t.Fatalf("decode search response: %v", err)
	}

	if len(searchResp.Files) != 2 {
		t.Fatalf("expected 2 files in folder1 search, got %d: %+v", len(searchResp.Files), searchResp.Files)
	}
}

func TestDesktopFilesystemSymlinkAndFolderSize(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		t.Fatalf("getDesktopService: %v", err)
	}

	// 1. Test Symlink
	// Create target file
	_ = svc.WriteFileBytes(context.Background(), "target.txt", []byte("target content"), desktop.SourceUser)

	// Call /api/desktop/symlink
	body := map[string]string{
		"target_path": "target.txt",
		"link_path":   "mylink.txt",
	}
	bodyBytes, _ := json.Marshal(body)
	reqSymlink := httptest.NewRequest(http.MethodPost, "/api/desktop/symlink", bytes.NewReader(bodyBytes))
	reqSymlink.Header.Set("Content-Type", "application/json")
	respSymlink := httptest.NewRecorder()
	handleDesktopSymlink(s).ServeHTTP(respSymlink, reqSymlink)

	if respSymlink.Code != http.StatusOK {
		bodyStr := respSymlink.Body.String()
		if strings.Contains(bodyStr, "A required privilege is not held") || strings.Contains(bodyStr, "privilege") {
			t.Log("Skipping symlink validation due to missing OS privilege (required privilege not held)")
		} else {
			t.Fatalf("symlink status = %d, body = %s", respSymlink.Code, bodyStr)
		}
	} else {
		// Verify symlink resolved/exists
		absLink, err := svc.ResolvePath("mylink.txt")
		if err != nil {
			t.Fatalf("ResolvePath: %v", err)
		}
		fi, err := os.Lstat(absLink)
		if err != nil {
			t.Fatalf("lstat symlink: %v", err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("expected symlink, got mode: %v", fi.Mode())
		}
	}

	// 2. Test Folder Size Calculation
	// Create folder and files
	_ = svc.WriteFileBytes(context.Background(), "calc/file1.txt", []byte("123"), desktop.SourceUser)      // 3 bytes
	_ = svc.WriteFileBytes(context.Background(), "calc/sub/file2.txt", []byte("4567"), desktop.SourceUser) // 4 bytes

	reqSize := httptest.NewRequest(http.MethodGet, "/api/desktop/folder-size?path="+url.QueryEscape("calc"), nil)
	respSize := httptest.NewRecorder()
	handleDesktopFolderSize(s).ServeHTTP(respSize, reqSize)

	if respSize.Code != http.StatusOK {
		t.Fatalf("folder-size status = %d, body = %s", respSize.Code, respSize.Body.String())
	}

	var sizeResp struct {
		Status string `json:"status"`
		Size   int64  `json:"size"`
	}
	if err := json.NewDecoder(respSize.Body).Decode(&sizeResp); err != nil {
		t.Fatalf("decode folder-size response: %v", err)
	}

	if sizeResp.Size != 7 {
		t.Fatalf("expected folder size 7, got %d", sizeResp.Size)
	}
}
