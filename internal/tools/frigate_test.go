package tools

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrigateStatusAddsBearerToken(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/stats" {
			t.Fatalf("path = %q, want /api/stats", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"service":{"uptime":12}}`))
	}))
	defer server.Close()

	out := FrigateStatus(FrigateConfig{URL: server.URL, APIToken: "secret-token"})
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(out, `"uptime":12`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateEventsBuildsQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/events" || q.Get("camera") != "doorbell" || q.Get("label") != "person" || q.Get("has_clip") != "true" || q.Get("limit") != "5" || q.Get("offset") != "10" {
			t.Fatalf("unexpected request path=%q query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"event-1"}]`))
	}))
	defer server.Close()

	hasClip := true
	out := FrigateEvents(FrigateConfig{URL: server.URL}, FrigateEventParams{Camera: "doorbell", Label: "person", HasClip: &hasClip, Limit: 5, Offset: 10})
	if !strings.Contains(out, `"event-1"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateReviewsBuildsOffsetQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/review" || q.Get("camera") != "garage" || q.Get("limit") != "25" || q.Get("offset") != "50" {
			t.Fatalf("unexpected request path=%q query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"review-1"}]`))
	}))
	defer server.Close()

	out := FrigateReviews(FrigateConfig{URL: server.URL}, FrigateReviewParams{Camera: "garage", Limit: 25, Offset: 50})
	if !strings.Contains(out, `"review-1"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateStatusRetriesTransientServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "temporary failure", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"service":{"uptime":42}}`))
	}))
	defer server.Close()

	out := FrigateStatus(FrigateConfig{URL: server.URL})
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if !strings.Contains(out, `"uptime":42`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateMediaRequiresEventID(t *testing.T) {
	_, _, err := FrigateMedia(FrigateConfig{URL: "http://example.invalid"}, "event_snapshot", FrigateMediaParams{})
	if err == nil || !strings.Contains(err.Error(), "event_id is required") {
		t.Fatalf("err = %v, want event_id error", err)
	}
}

func TestStoreFrigateMediaWritesSafeFileAndRegistersMedia(t *testing.T) {
	dataDir := t.TempDir()
	db := initFrigateMediaTestDB(t)
	result, err := StoreFrigateMedia(dataDir, db, "event_snapshot", FrigateMediaParams{EventID: `door/../../event 1`}, []byte{0xff, 0xd8, 0xff, 0xdb}, "image/jpeg")
	if err != nil {
		t.Fatalf("StoreFrigateMedia error = %v", err)
	}
	if !result.Stored {
		t.Fatal("expected stored result")
	}
	if result.LocalPath == "" || !strings.HasPrefix(result.LocalPath, filepath.Join(dataDir, "frigate_media")) {
		t.Fatalf("local path %q should stay inside data/frigate_media", result.LocalPath)
	}
	if strings.Contains(filepath.Base(result.LocalPath), "..") || strings.Contains(filepath.Base(result.LocalPath), "/") || strings.Contains(filepath.Base(result.LocalPath), `\`) {
		t.Fatalf("unsafe filename %q", filepath.Base(result.LocalPath))
	}
	if result.WebPath == "" || !strings.HasPrefix(result.WebPath, "/files/frigate_media/") {
		t.Fatalf("web path = %q, want /files/frigate_media prefix", result.WebPath)
	}
	if result.SHA256 == "" {
		t.Fatal("expected sha256")
	}
	if result.MediaID == 0 {
		t.Fatal("expected media registry id")
	}
	if _, err := os.Stat(result.LocalPath); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
}

func initFrigateMediaTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := InitMediaRegistryDB(filepath.Join(t.TempDir(), "media.db"))
	if err != nil {
		t.Fatalf("InitMediaRegistryDB error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
