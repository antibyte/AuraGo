package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newAccessLogTestHandler(level slog.Level) (http.Handler, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level}))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Status") == "401" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return accessLogMiddleware(logger, inner, false), &buf
}

func TestAccessLogRecordsDashboardMutationsAndErrors(t *testing.T) {
	handler, buf := newAccessLogTestHandler(slog.LevelInfo)
	for _, tc := range []struct {
		method, path, status, wantLevel string
	}{
		{http.MethodDelete, "/api/dashboard/audit/7", "", "INFO"},
		{http.MethodPut, "/api/dashboard/cronjobs", "", "INFO"},
		{http.MethodPost, "/api/dashboard/core-memory/mutate", "", "INFO"},
		{http.MethodGet, "/api/dashboard/system", "401", "WARN"},
		{http.MethodGet, "/api/dashboard/system", "", ""},
		{http.MethodGet, "/events", "", ""},
	} {
		buf.Reset()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.status != "" {
			req.Header.Set("X-Test-Status", tc.status)
		}
		handler.ServeHTTP(httptest.NewRecorder(), req)
		out := strings.TrimSpace(buf.String())
		if tc.wantLevel == "" {
			if out != "" {
				t.Fatalf("%s %s: unexpected Info-level access log: %s", tc.method, tc.path, out)
			}
			continue
		}
		if out == "" {
			t.Fatalf("%s %s: no access log line written", tc.method, tc.path)
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(out), &entry); err != nil {
			t.Fatalf("%s %s: parse log line %q: %v", tc.method, tc.path, out, err)
		}
		if entry["level"] != tc.wantLevel || entry["path"] != tc.path || entry["method"] != tc.method {
			t.Fatalf("%s %s: log entry = %v, want level %s", tc.method, tc.path, entry, tc.wantLevel)
		}
	}
}

func TestAccessLogKeepsDashboardPollsAtDebug(t *testing.T) {
	handler, buf := newAccessLogTestHandler(slog.LevelDebug)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/dashboard/system", nil))
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("poll was not logged at Debug: %q (%v)", buf.String(), err)
	}
	if entry["level"] != "DEBUG" {
		t.Fatalf("poll level = %v, want DEBUG", entry["level"])
	}
}
