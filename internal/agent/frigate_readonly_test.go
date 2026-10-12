package agent

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestFrigateReadOnly(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "snapshot") {
			_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xdb})
			return
		}
		_, _ = w.Write([]byte(`{"service":{"uptime":1}}`))
	}))
	defer server.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	dispatch := func(readonly bool, operation, eventID string) string {
		t.Helper()
		cfg := &config.Config{}
		cfg.Frigate.Enabled = true
		cfg.Frigate.ReadOnly = readonly
		cfg.Frigate.StoreMedia = true
		cfg.Frigate.URL = server.URL
		cfg.Directories.DataDir = t.TempDir()
		out, ok := dispatchPlatform(context.Background(), ToolCall{
			Action:    "frigate",
			Operation: operation,
			ID:        eventID,
		}, &DispatchContext{Cfg: cfg, Logger: logger})
		if !ok {
			t.Fatal("frigate dispatch was not handled")
		}
		return out
	}

	blocked := dispatch(true, "save_config", "")
	if calls != 0 {
		t.Fatalf("blocked write made %d requests", calls)
	}
	if !strings.Contains(blocked, `"status":"error"`) || !strings.Contains(blocked, "read-only") || !strings.Contains(blocked, "blocked") {
		t.Fatalf("blocked output = %s", blocked)
	}

	unknown := dispatch(false, "save_config", "")
	if calls != 0 {
		t.Fatalf("unknown operation made %d requests", calls)
	}
	if !strings.Contains(unknown, "Unknown frigate operation") || strings.Contains(unknown, "read-only") {
		t.Fatalf("unknown output = %s", unknown)
	}

	status := dispatch(true, "status", "")
	if calls != 1 || !strings.Contains(status, `"uptime":1`) || strings.Contains(status, "read-only") {
		t.Fatalf("calls = %d, status = %s", calls, status)
	}

	snapshot := dispatch(true, "event_snapshot", "door-1")
	if calls != 2 || strings.Contains(snapshot, "read-only") || !strings.Contains(snapshot, `"stored":true`) {
		t.Fatalf("calls = %d, snapshot = %s", calls, snapshot)
	}
}
