package rocketchat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
)

func rocketConfig(endpoint string) *config.Config {
	cfg := &config.Config{}
	cfg.RocketChat.Enabled = true
	cfg.RocketChat.URL = endpoint
	cfg.RocketChat.AuthToken = "fixture-rocket-secret"
	cfg.RocketChat.UserID = "bot"
	cfg.RocketChat.Channel = "room & other"
	cfg.RocketChat.AllowedUsers = []string{"id:sender"}
	return cfg
}

func awaitRocket(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("Rocket.Chat runtime did not complete")
	}
}

func TestRocketChatHistoryPaginatesChronologicallyAcrossEqualTimestamps(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		q := r.URL.Query()
		if q.Get("roomId") != "room & other" || q.Get("sort") != `{"ts":1,"_id":1}` || q.Get("inclusive") != "true" {
			t.Errorf("invalid query: %s", r.URL.RawQuery)
		}
		oldest, err := time.Parse(time.RFC3339Nano, q.Get("oldest"))
		if err != nil {
			t.Error(err)
		}
		offset, _ := strconv.Atoi(q.Get("offset"))
		var all []map[string]any
		for i := 0; i < 127; i++ {
			// Seventy messages at one timestamp span two pages.
			ts := start.Add(time.Duration(i/70) * time.Millisecond)
			if ts.Before(oldest) {
				continue
			}
			all = append(all, map[string]any{"_id": fmt.Sprintf("m%03d", i), "ts": ts.Format(time.RFC3339Nano)})
		}
		if offset > len(all) {
			t.Errorf("invalid offset %d", offset)
			offset = len(all)
		}
		all = all[offset:]
		if len(all) > 50 {
			all = all[:50]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "messages": all})
	}))
	defer srv.Close()
	cursor := historyCursor{since: start}
	var ids []string
	for page := 0; page < 5; page++ {
		messages, err := fetchNewMessages(context.Background(), rocketConfig(srv.URL), "room & other", cursor, start.Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}
		for _, msg := range messages {
			ids = append(ids, msg.ID)
			cursor.advance(msg)
		}
		if len(messages) < 50 {
			break
		}
	}
	if len(ids) != 127 || requests != 3 {
		t.Fatalf("messages=%d requests=%d", len(ids), requests)
	}
	for i, id := range ids {
		if want := fmt.Sprintf("m%03d", i); id != want {
			t.Fatalf("message %d = %s, want %s", i, id, want)
		}
	}
	var legacy messageTimestamp
	if err := json.Unmarshal([]byte(`{"$date":1767225600000}`), &legacy); err != nil || legacy.Date != start.UnixMilli() {
		t.Fatalf("legacy timestamp=%v error=%v", legacy, err)
	}
}

func TestRocketChatPollUsesCurrentSnapshotAndStopsAtRevocation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/channels.info" {
			fmt.Fprint(w, `{"channel":{"_id":"room"}}`)
			return
		}
		oldest, _ := time.Parse(time.RFC3339Nano, r.URL.Query().Get("oldest"))
		var messages []map[string]any
		for i := 0; i < 3; i++ {
			messages = append(messages, map[string]any{"_id": fmt.Sprint(i), "msg": "hello", "u": map[string]string{"_id": "sender"}, "ts": oldest.Format(time.RFC3339Nano)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"messages": messages})
	}))
	defer srv.Close()
	cfg := rocketConfig(srv.URL)
	current := cfg.Clone()
	current.Agent.ToolOutputLimit = 17
	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pollLoop(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), func() (*config.Config, llm.ChatClient) { return current, nil }, func(ctx context.Context, c *config.Config, _ llm.ChatClient, _ string, _ message) {
		calls++
		if c.Agent.ToolOutputLimit != 17 {
			t.Error("stale configuration")
		}
		current = current.Clone()
		current.RocketChat.AllowedUsers = nil
	})
	if calls != 1 {
		t.Fatalf("delivered %d messages after revocation", calls)
	}
	if ctx.Err() != nil {
		t.Fatal("poller ignored revoked settings")
	}
}

func TestRocketChatStopCancelsAndDrainsWorker(t *testing.T) {
	cfg := rocketConfig("https://example.invalid")
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	b := startRuntime(context.Background(), cfg, func(ctx context.Context) { close(started); <-ctx.Done(); close(cancelled); <-release })
	awaitRocket(t, started)
	changed := cfg.Clone()
	changed.RocketChat.AllowedUsers = nil
	b.CancelIfConfigChanged(changed)
	awaitRocket(t, cancelled)
	stopped := make(chan struct{})
	go func() { b.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("Stop returned before worker drained")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	awaitRocket(t, stopped)
	b.Stop()
}

func TestRocketChatRequestCancelsAndBindsCredentialsToOrigin(t *testing.T) {
	var leaked atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer foreign.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, foreign.URL, http.StatusFound) }))
	defer redirect.Close()
	cfg := rocketConfig(redirect.URL)
	if _, _, err := rcRequestContext(context.Background(), cfg, "GET", "/me", ""); err == nil {
		t.Fatal("foreign redirect accepted")
	}
	if leaked.Load() != 0 {
		t.Fatal("credentials reached foreign origin")
	}
	started, done := make(chan struct{}), make(chan struct{})
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(done) }))
	defer blocked.Close()
	cfg = rocketConfig(blocked.URL)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { defer close(finished); _, _, _ = rcRequestContext(ctx, cfg, "GET", "/me", "") }()
	awaitRocket(t, started)
	cancel()
	awaitRocket(t, done)
	awaitRocket(t, finished)
}
