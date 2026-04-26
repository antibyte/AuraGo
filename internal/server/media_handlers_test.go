package server

import (
	"bytes"
	"encoding/json"
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

func int64String(v int64) string {
	return strconv.FormatInt(v, 10)
}
