package tools

import (
	"context"
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

	out := FrigateStatus(context.Background(), FrigateConfig{URL: server.URL, APIToken: "secret-token"})
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
	out := FrigateEvents(context.Background(), FrigateConfig{URL: server.URL}, FrigateEventParams{Camera: "doorbell", Label: "person", HasClip: &hasClip, Limit: 5, Offset: 10})
	if !strings.Contains(out, `"event-1"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateReviewsBuildsOffsetQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/review" || q.Get("cameras") != "garage" || q.Get("limit") != "25" || q.Get("offset") != "50" {
			t.Fatalf("unexpected request path=%q query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"review-1"}]`))
	}))
	defer server.Close()

	out := FrigateReviews(context.Background(), FrigateConfig{URL: server.URL}, FrigateReviewParams{Camera: "garage", Limit: 25, Offset: 50})
	if !strings.Contains(out, `"review-1"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateReviewsBuildsCurrentFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/review" || q.Get("cameras") != "doorbell,garage" || q.Get("labels") != "person" || q.Get("zones") != "porch" || q.Get("reviewed") != "0" || q.Get("severity") != "alert" {
			t.Fatalf("unexpected request path=%q query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"review-2"}]`))
	}))
	defer server.Close()

	reviewed := false
	out := FrigateReviews(context.Background(), FrigateConfig{URL: server.URL}, FrigateReviewParams{
		Cameras:  "doorbell,garage",
		Labels:   "person",
		Zones:    "porch",
		Reviewed: &reviewed,
		Severity: "alert",
	})
	if !strings.Contains(out, `"review-2"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateReviewActivityUsesMotionEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/api/review/activity/motion" || q.Get("cameras") != "front" {
			t.Fatalf("unexpected request path=%q query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"motion":true}`))
	}))
	defer server.Close()

	out := FrigateReviewActivity(context.Background(), FrigateConfig{URL: server.URL}, FrigateReviewParams{Cameras: "front"})
	if !strings.Contains(out, `"motion":true`) {
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

	out := FrigateStatus(context.Background(), FrigateConfig{URL: server.URL})
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if !strings.Contains(out, `"uptime":42`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateStatusCancelledContextDoesNotRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"service":{"uptime":1}}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := FrigateStatus(ctx, FrigateConfig{URL: server.URL})
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
	if !strings.Contains(out, `"status":"error"`) || strings.Contains(out, "uptime") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateStatusHidesUpstreamErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fixture-error-secret", http.StatusInternalServerError)
	}))
	defer server.Close()

	out := FrigateStatus(context.Background(), FrigateConfig{URL: server.URL})
	if strings.Contains(out, "fixture-error-secret") {
		t.Fatalf("upstream body leaked: %s", out)
	}
	if !strings.Contains(out, "HTTP 500") || !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateMediaRequiresEventID(t *testing.T) {
	_, err := FrigateMedia(context.Background(), FrigateConfig{URL: "http://example.invalid"}, "event_snapshot", FrigateMediaParams{}, "", nil, false)
	if err == nil || !strings.Contains(err.Error(), "event_id is required") {
		t.Fatalf("err = %v, want event_id error", err)
	}
}

func TestFrigateMediaExportRecordingUsesClipEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/front/start/1767225600/end/1767225900/clip.mp4" {
			t.Fatalf("path = %q, want recording clip endpoint", r.URL.Path)
		}
		_, _ = w.Write([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2'})
	}))
	defer server.Close()

	result, err := FrigateMedia(context.Background(), FrigateConfig{URL: server.URL}, "export_recording", FrigateMediaParams{
		Camera:    "front",
		StartTime: "1767225600",
		EndTime:   "1767225900",
	}, t.TempDir(), nil, true)
	if err != nil {
		t.Fatalf("FrigateMedia error = %v", err)
	}
	if result.Bytes == 0 {
		t.Fatal("expected media data")
	}
	if !strings.Contains(result.ContentType, "video/mp4") {
		t.Fatalf("contentType = %q, want video/mp4", result.ContentType)
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

func TestFrigateMediaRejectsDisallowedTypes(t *testing.T) {
	ftyp := []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2'}
	cases := []struct {
		name      string
		operation string
		body      []byte
		wantType  string
		wantOK    bool
	}{
		{name: "html", operation: "event_snapshot", body: []byte("<!DOCTYPE html><p>fixture-error-secret</p>"), wantOK: false},
		{name: "snapshot ftyp", operation: "event_snapshot", body: ftyp, wantOK: false},
		{name: "clip ftyp", operation: "event_clip", body: ftyp, wantType: "video/mp4", wantOK: true},
		{name: "clip junk", operation: "event_clip", body: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()
			dataDir := t.TempDir()
			params := FrigateMediaParams{EventID: "door-1"}
			result, err := FrigateMedia(context.Background(), FrigateConfig{URL: server.URL}, tc.operation, params, dataDir, nil, true)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("FrigateMedia error = %v", err)
				}
				if result.ContentType != tc.wantType || !result.Stored || result.SHA256 == "" {
					t.Fatalf("result = %+v", result)
				}
				return
			}
			if err == nil || strings.Contains(err.Error(), "fixture-error-secret") {
				t.Fatalf("err = %v", err)
			}
			assertNoFrigateMediaFiles(t, dataDir)
		})
	}
}

func TestFrigateMediaStoreDisabledDiscardsFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xdb})
	}))
	defer server.Close()
	dataDir := t.TempDir()
	result, err := FrigateMedia(context.Background(), FrigateConfig{URL: server.URL}, "event_snapshot", FrigateMediaParams{EventID: "door-1"}, dataDir, nil, false)
	if err != nil {
		t.Fatalf("FrigateMedia error = %v", err)
	}
	if result.Stored || result.Bytes != 4 || result.ContentType != "image/jpeg" || result.LocalPath != "" || result.SHA256 != "" {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Message, "frigate.store_media is disabled") {
		t.Fatalf("message = %q", result.Message)
	}
	assertNoFrigateMediaFiles(t, dataDir)
}

func TestFrigateMediaLimitIsIndependentOfJSONLimit(t *testing.T) {
	const n = 10*1024*1024 + 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := []byte("fixture-error-secret")
		if r.URL.Path != "/api/stats" {
			prefix = []byte{0xff, 0xd8, 0xff, 0xdb}
		}
		writeSizedBody(w, n, prefix)
	}))
	defer server.Close()

	status := FrigateStatus(context.Background(), FrigateConfig{URL: server.URL})
	if strings.Contains(status, "fixture-error-secret") || !strings.Contains(status, `"status":"error"`) || !strings.Contains(status, "10485760") {
		t.Fatalf("status = %s", status)
	}

	dataDir := t.TempDir()
	result, err := FrigateMedia(context.Background(), FrigateConfig{URL: server.URL}, "event_snapshot", FrigateMediaParams{EventID: "door-1"}, dataDir, nil, true)
	if err != nil {
		t.Fatalf("FrigateMedia error = %v", err)
	}
	if result.Bytes != n || result.ContentType != "image/jpeg" || !result.Stored {
		t.Fatalf("result = %+v", result)
	}
	info, err := os.Stat(result.LocalPath)
	if err != nil {
		t.Fatalf("stored file missing: %v", err)
	}
	if info.Size() != n {
		t.Fatalf("size = %d, want %d", info.Size(), n)
	}
}

func TestFrigateMediaRejectsBodyOverLimit(t *testing.T) {
	const n = maxFrigateMediaBytes + 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSizedBody(w, n, []byte("fixture-error-secret"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	_, err := FrigateMedia(context.Background(), FrigateConfig{URL: server.URL}, "event_clip", FrigateMediaParams{EventID: "door-1"}, dataDir, nil, true)
	if err == nil || strings.Contains(err.Error(), "fixture-error-secret") || !strings.Contains(err.Error(), "268435456") {
		t.Fatalf("err = %v", err)
	}
	assertNoFrigateMediaFiles(t, dataDir)
}

func writeSizedBody(w http.ResponseWriter, n int64, prefix []byte) {
	if len(prefix) > 0 {
		if _, err := w.Write(prefix); err != nil {
			return
		}
		n -= int64(len(prefix))
	}
	buf := make([]byte, 32*1024)
	for n > 0 {
		chunk := int64(len(buf))
		if chunk > n {
			chunk = n
		}
		if _, err := w.Write(buf[:chunk]); err != nil {
			return
		}
		n -= chunk
	}
}

func assertNoFrigateMediaFiles(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			t.Fatalf("left file %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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
