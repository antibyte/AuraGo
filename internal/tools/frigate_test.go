package tools

import (
	"net/http"
	"net/http/httptest"
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
		if r.URL.Path != "/api/events" || q.Get("camera") != "doorbell" || q.Get("label") != "person" || q.Get("has_clip") != "true" || q.Get("limit") != "5" {
			t.Fatalf("unexpected request path=%q query=%s", r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"id":"event-1"}]`))
	}))
	defer server.Close()

	hasClip := true
	out := FrigateEvents(FrigateConfig{URL: server.URL}, FrigateEventParams{Camera: "doorbell", Label: "person", HasClip: &hasClip, Limit: 5})
	if !strings.Contains(out, `"event-1"`) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFrigateMediaRequiresEventID(t *testing.T) {
	_, _, err := FrigateMedia(FrigateConfig{URL: "http://example.invalid"}, "event_snapshot", FrigateMediaParams{})
	if err == nil || !strings.Contains(err.Error(), "event_id is required") {
		t.Fatalf("err = %v, want event_id error", err)
	}
}
