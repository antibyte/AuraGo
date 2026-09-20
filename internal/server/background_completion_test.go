package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func backgroundRequest(ctx context.Context, id, content string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(content)).WithContext(ctx)
	r.RemoteAddr = "127.0.0.1:9000"
	r.Header.Set("X-Internal-FollowUp", "true")
	r.Header.Set("X-Internal-Token", "test-process-token")
	r.Header.Set("X-Background-Execution-ID", id)
	return r
}

func TestBackgroundCompletionReplaysAfterDisconnectAcrossHandlers(t *testing.T) {
	s := &Server{internalToken: "test-process-token"}
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	next := func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(started)
		<-release
		if r.Context().Err() != nil {
			t.Error("execution inherited client cancellation")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"completed":true}`))
	}
	first, second := withBackgroundCompletionReplay(s, next), withBackgroundCompletionReplay(s, next)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	returned := make(chan struct{})
	go func() { first(httptest.NewRecorder(), backgroundRequest(ctx, "job-1", `{}`)); close(returned) }()
	<-started
	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("disconnected waiter blocked")
	}
	conflict := httptest.NewRecorder()
	second(conflict, backgroundRequest(context.Background(), "job-1", `{"changed":true}`))
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", conflict.Code)
	}
	close(release)
	s.backgroundCompletions.mu.Lock()
	done := s.backgroundCompletions.entries["job-1"].done
	s.backgroundCompletions.mu.Unlock()
	<-done
	for i := 0; i < 2; i++ {
		replay := httptest.NewRecorder()
		second(replay, backgroundRequest(context.Background(), "job-1", `{}`))
		if replay.Code != http.StatusCreated || replay.Body.String() != `{"completed":true}` {
			t.Fatalf("unexpected replay: %d %s", replay.Code, replay.Body.String())
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("executions = %d, want 1", calls.Load())
	}
}

func TestBackgroundCompletionRejectsUntrustedAndStreamingRequests(t *testing.T) {
	s := &Server{internalToken: "test-process-token"}
	h := withBackgroundCompletionReplay(s, func(http.ResponseWriter, *http.Request) { t.Error("unexpected execution") })
	for _, test := range []struct {
		name, token, body string
		status            int
	}{
		{"forged", "wrong", `{}`, http.StatusForbidden},
		{"stream", "test-process-token", `{"stream":true}`, http.StatusBadRequest},
		{"invalid", "test-process-token", `{`, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := backgroundRequest(context.Background(), "job", test.body)
			r.Header.Set("X-Internal-Token", test.token)
			w := httptest.NewRecorder()
			h(w, r)
			if w.Code != test.status {
				t.Fatalf("status = %d", w.Code)
			}
		})
	}
}

func TestBackgroundCompletionBoundsResponsesAndKeepsFailures(t *testing.T) {
	s := &Server{internalToken: "test-process-token"}
	calls := 0
	h := withBackgroundCompletionReplay(s, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(strings.Repeat("x", 128*1024+1)))
	})
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h(w, backgroundRequest(context.Background(), "job", `{}`))
		if w.Code == http.StatusAccepted {
			s.backgroundCompletions.mu.Lock()
			done := s.backgroundCompletions.entries["job"].done
			s.backgroundCompletions.mu.Unlock()
			<-done
			w = httptest.NewRecorder()
			h(w, backgroundRequest(context.Background(), "job", `{}`))
		}
		if w.Code != http.StatusInternalServerError || w.Body.Len() > 128*1024 {
			t.Fatalf("unbounded response: %d/%d", w.Code, w.Body.Len())
		}
	}
	if calls != 1 {
		t.Fatalf("failed execution repeated %d times", calls)
	}
}
