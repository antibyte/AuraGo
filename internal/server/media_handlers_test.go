package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func newMediaTestServer(t *testing.T) (*Server, int64, int64, int64) {
	t.Helper()

	tmpDir := t.TempDir()
	audioDir := filepath.Join(tmpDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		t.Fatalf("create audio dir: %v", err)
	}

	db, err := tools.InitMediaRegistryDB(filepath.Join(tmpDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("init media registry db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mkItem := func(filename string) int64 {
		path := filepath.Join(audioDir, filename)
		if err := os.WriteFile(path, []byte("audio"), 0644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
		id, _, err := tools.RegisterMedia(db, tools.MediaItem{
			MediaType: "audio",
			Filename:  filename,
			FilePath:  path,
			WebPath:   "/files/audio/" + filename,
		})
		if err != nil {
			t.Fatalf("register %s: %v", filename, err)
		}
		return id
	}

	id1 := mkItem("one.mp3")
	id2 := mkItem("two.mp3")
	id3 := mkItem("three.mp3")

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	s := &Server{
		Cfg:             cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		MediaRegistryDB: db,
	}
	return s, id1, id2, id3
}

func TestMediaBulkDeleteDeletesSelectedItems(t *testing.T) {
	s, id1, id2, id3 := newMediaTestServer(t)

	body := []byte(`{"ids":[` + int64String(id1) + `,` + int64String(id2) + `]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/media/bulk-delete", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handleMediaBulkDelete(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var res struct {
		Status  string                   `json:"status"`
		Deleted int                      `json:"deleted"`
		Failed  []mediaBulkDeleteFailure `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.Status != "ok" || res.Deleted != 2 || len(res.Failed) != 0 {
		t.Fatalf("response = %+v", res)
	}
	if _, err := tools.GetMedia(s.MediaRegistryDB, id1); err == nil {
		t.Fatal("id1 should be deleted")
	}
	if _, err := tools.GetMedia(s.MediaRegistryDB, id2); err == nil {
		t.Fatal("id2 should be deleted")
	}
	if _, err := tools.GetMedia(s.MediaRegistryDB, id3); err != nil {
		t.Fatalf("id3 should remain: %v", err)
	}
}

func TestMediaBulkDeleteRejectsEmptySelection(t *testing.T) {
	s, _, _, _ := newMediaTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/media/bulk-delete", bytes.NewReader([]byte(`{"ids":[]}`)))
	rr := httptest.NewRecorder()
	handleMediaBulkDelete(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestMediaBulkDeleteReportsPartialFailure(t *testing.T) {
	s, id1, _, id3 := newMediaTestServer(t)

	body := []byte(`{"ids":[` + int64String(id1) + `,999999]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/media/bulk-delete", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handleMediaBulkDelete(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var res struct {
		Status  string                   `json:"status"`
		Deleted int                      `json:"deleted"`
		Failed  []mediaBulkDeleteFailure `json:"failed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.Status != "partial" || res.Deleted != 1 || len(res.Failed) != 1 {
		t.Fatalf("response = %+v", res)
	}
	if _, err := tools.GetMedia(s.MediaRegistryDB, id1); err == nil {
		t.Fatal("id1 should be deleted")
	}
	if _, err := tools.GetMedia(s.MediaRegistryDB, id3); err != nil {
		t.Fatalf("id3 should remain: %v", err)
	}
}

func TestMediaListSkipsUnservableRegistryItems(t *testing.T) {
	tmpDir := t.TempDir()
	imageDir := filepath.Join(tmpDir, "generated_images")
	privateDir := filepath.Join(tmpDir, "private")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("create image dir: %v", err)
	}
	if err := os.MkdirAll(privateDir, 0755); err != nil {
		t.Fatalf("create private dir: %v", err)
	}
	existingPath := filepath.Join(imageDir, "existing.png")
	if err := os.WriteFile(existingPath, []byte("fake image"), 0644); err != nil {
		t.Fatalf("write existing image: %v", err)
	}
	privatePath := filepath.Join(privateDir, "stale.png")
	if err := os.WriteFile(privatePath, []byte("private image"), 0644); err != nil {
		t.Fatalf("write private image: %v", err)
	}

	db, err := tools.InitMediaRegistryDB(filepath.Join(tmpDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("init media registry db: %v", err)
	}
	defer db.Close()

	if _, _, err := tools.RegisterMedia(db, tools.MediaItem{
		MediaType: "image",
		Filename:  "existing.png",
		FilePath:  existingPath,
		WebPath:   "/files/generated_images/existing.png",
	}); err != nil {
		t.Fatalf("register existing image: %v", err)
	}
	if _, _, err := tools.RegisterMedia(db, tools.MediaItem{
		MediaType: "image",
		Filename:  "stale.png",
		FilePath:  privatePath,
		WebPath:   "/files/generated_images/stale.png",
	}); err != nil {
		t.Fatalf("register stale image: %v", err)
	}
	if _, _, err := tools.RegisterMedia(db, tools.MediaItem{
		MediaType: "image",
		Filename:  "missing.png",
		FilePath:  filepath.Join(imageDir, "missing.png"),
		WebPath:   "/files/generated_images/missing.png",
	}); err != nil {
		t.Fatalf("register missing image: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	s := &Server{
		Cfg:             cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		MediaRegistryDB: db,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media?type=image", nil)
	rr := httptest.NewRecorder()
	handleMediaList(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Status string            `json:"status"`
		Items  []tools.MediaItem `json:"items"`
		Total  int               `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("response = %+v", body)
	}
	if body.Items[0].Filename != "existing.png" {
		t.Fatalf("filename = %q, want existing.png", body.Items[0].Filename)
	}
}

func TestMediaListPaginatesBeyondFiveThousandDisplayableItems(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := tools.InitMediaRegistryDB(filepath.Join(tmpDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("init media registry db: %v", err)
	}
	defer db.Close()

	for i := 0; i < 5001; i++ {
		if _, _, err := tools.RegisterMedia(db, tools.MediaItem{
			MediaType: "audio",
			Filename:  fmt.Sprintf("audio-%04d.mp3", i),
			WebPath:   fmt.Sprintf("https://media.example.test/audio-%04d.mp3", i),
		}); err != nil {
			t.Fatalf("register audio %d: %v", i, err)
		}
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	s := &Server{
		Cfg:             cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		MediaRegistryDB: db,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/media?type=audio&limit=1&offset=5000", nil)
	rr := httptest.NewRecorder()
	handleMediaList(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Status string            `json:"status"`
		Items  []tools.MediaItem `json:"items"`
		Total  int               `json:"total"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || body.Total != 5001 || len(body.Items) != 1 {
		t.Fatalf("response = %+v", body)
	}
}

func TestMediaDeleteDoesNotRemoveUnsafeFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	privateDir := filepath.Join(tmpDir, "private")
	if err := os.MkdirAll(privateDir, 0755); err != nil {
		t.Fatalf("create private dir: %v", err)
	}
	privatePath := filepath.Join(privateDir, "keep.mp3")
	if err := os.WriteFile(privatePath, []byte("keep"), 0644); err != nil {
		t.Fatalf("write private file: %v", err)
	}

	db, err := tools.InitMediaRegistryDB(filepath.Join(tmpDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("init media registry db: %v", err)
	}
	defer db.Close()
	id, _, err := tools.RegisterMedia(db, tools.MediaItem{
		MediaType: "audio",
		Filename:  "keep.mp3",
		FilePath:  privatePath,
		WebPath:   "/files/audio/keep.mp3",
	})
	if err != nil {
		t.Fatalf("register unsafe file path item: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	s := &Server{
		Cfg:             cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		MediaRegistryDB: db,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/media/"+int64String(id), nil)
	rr := httptest.NewRecorder()
	handleMediaByID(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(privatePath); err != nil {
		t.Fatalf("unsafe file_path should not be removed: %v", err)
	}
	if _, err := tools.GetMedia(db, id); err == nil {
		t.Fatal("registry item should be soft-deleted")
	}
}

func TestMediaDeleteDoesNotTreatExternalWebPathAsLocalFile(t *testing.T) {
	tmpDir := t.TempDir()
	audioDir := filepath.Join(tmpDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		t.Fatalf("create audio dir: %v", err)
	}
	localPath := filepath.Join(audioDir, "keep.mp3")
	if err := os.WriteFile(localPath, []byte("keep"), 0644); err != nil {
		t.Fatalf("write local audio: %v", err)
	}

	db, err := tools.InitMediaRegistryDB(filepath.Join(tmpDir, "media_registry.db"))
	if err != nil {
		t.Fatalf("init media registry db: %v", err)
	}
	defer db.Close()
	id, _, err := tools.RegisterMedia(db, tools.MediaItem{
		MediaType: "audio",
		Filename:  "keep.mp3",
		WebPath:   "https://cdn.example.test/files/audio/keep.mp3",
	})
	if err != nil {
		t.Fatalf("register external web path item: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = tmpDir
	s := &Server{
		Cfg:             cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		MediaRegistryDB: db,
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/media/"+int64String(id), nil)
	rr := httptest.NewRecorder()
	handleMediaByID(s).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(localPath); err != nil {
		t.Fatalf("external web_path should not remove local file: %v", err)
	}
	if _, err := tools.GetMedia(db, id); err == nil {
		t.Fatal("registry item should be soft-deleted")
	}
}

func int64String(v int64) string {
	return strconv.FormatInt(v, 10)
}
