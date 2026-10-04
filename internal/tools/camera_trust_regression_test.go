package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFrigateConfigRedactsBeforeReturningEitherFormat(t *testing.T) {
	for _, raw := range []bool{false, true} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if raw {
				fmt.Fprint(w, "# comment-secret\nmqtt:\n  password: mqtt-secret\ncameras:\n  front:\n    ffmpeg:\n      inputs:\n      - path: rtsp://user:camera-secret@camera/live\n    detect:\n      enabled: true\n      width: 1920\ncustom_token: unknown-secret\n")
			} else {
				fmt.Fprint(w, `{"mqtt":{"password":"mqtt-secret"},"cameras":{"front":{"detect":{"enabled":true,"width":1920},"ffmpeg":{"inputs":[{"path":"rtsp://user:camera-secret@camera/live"}]}}},"custom_token":"unknown-secret"}`)
			}
		}))
		out := FrigateConfigRead(FrigateConfig{URL: srv.URL}, raw)
		srv.Close()
		for _, secret := range []string{"comment-secret", "mqtt-secret", "camera-secret", "unknown-secret"} {
			if strings.Contains(out, secret) {
				t.Errorf("configuration leaked %s", secret)
			}
		}
		if !strings.Contains(out, "1920") || !strings.Contains(out, "REDACTED") {
			t.Fatalf("sanitized configuration missing: %s", out)
		}
	}
	for _, body := range []string{"unparseable-fixture", "a: &alias {password: fixture-secret}\nb: *alias", "a: 1\n---\npassword: fixture-secret", "a: !custom fixture-secret", "a: 1\na: fixture-secret"} {
		if _, err := redactFrigateConfig([]byte(body), true); err == nil {
			t.Errorf("unsupported configuration accepted: %q", body)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "fixture-error-secret")
	}))
	defer srv.Close()
	if out := FrigateConfigRead(FrigateConfig{URL: srv.URL}, true); strings.Contains(out, "fixture-error-secret") || !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("raw error forwarded: %s", out)
	}
}

func TestFrigateResponseBodyOutlivesHeadersAndRejectsForeignRedirect(t *testing.T) {
	var foreignCalls atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls.Add(1) }))
	defer foreign.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, foreign.URL, http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(10 * time.Millisecond)
		fmt.Fprint(w, `{"width":1920}`)
	}))
	defer srv.Close()
	if data, _, err := frigateRequest(FrigateConfig{URL: srv.URL}, http.MethodGet, "/api/config"); err != nil || !strings.Contains(string(data), "1920") {
		t.Fatalf("response cancelled at headers: %v", err)
	}
	if _, _, err := frigateRequest(FrigateConfig{URL: srv.URL, APIToken: "fixture-token"}, http.MethodGet, "/redirect"); err == nil {
		t.Fatal("foreign redirect allowed")
	}
	if foreignCalls.Load() != 0 {
		t.Fatal("foreign endpoint received credentials")
	}
}

func TestPrinterSnapshotRedirectCannotEscapeOrigin(t *testing.T) {
	var calls atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer foreign.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, foreign.URL, http.StatusFound) }))
	defer srv.Close()
	if _, _, err := FetchThreeDPrinterSnapshot(context.Background(), srv.URL); err == nil {
		t.Fatal("foreign camera redirect accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("foreign camera target reached")
	}
}

func TestGo2RTCStatusReturnsFailedCurrentProbe(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		fmt.Fprint(w, `{"version":"fixture"}`)
	}))
	defer srv.Close()
	manager := testGo2RTCManager(t, srv.URL, "fixture-password", false)
	if got := manager.Status(context.Background()); !got.APIUsable {
		t.Fatalf("initial probe failed: %+v", got)
	}
	fail.Store(true)
	got := manager.Status(context.Background())
	if got.APIUsable || got.LastError == "" || got.LastChecked == "" || manager.Available() {
		t.Fatalf("stale successful status: %+v", got)
	}
}
