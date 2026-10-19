package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/tools"
)

func backgroundReplayFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	manager := tools.NewBackgroundTaskManager(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { _ = manager.Close() })
	identity := make(chan string, 1)
	manager.SetLoopbackExecutorContext(func(_ context.Context, id, _, _ string, _ time.Duration) error {
		select {
		case identity <- id:
		default:
		}
		return tools.ErrBackgroundTaskPending
	})
	task, err := manager.ScheduleFollowUp("local replay fixture", tools.BackgroundTaskScheduleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	manager.Start()
	select {
	case id := <-identity:
		return &Server{internalToken: "test-process-token", BackgroundTasks: manager}, id, task.ID
	case <-time.After(3 * time.Second):
		t.Fatal("background owner did not start")
		return nil, "", ""
	}
}

func backgroundRequest(ctx context.Context, id, content string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(content)).WithContext(ctx)
	r.RemoteAddr = "127.0.0.1:9000"
	r.Header.Set("X-Internal-FollowUp", "true")
	r.Header.Set("X-Internal-Token", "test-process-token")
	r.Header.Set("X-Background-Execution-ID", id)
	return r
}

func TestBackgroundCompletionReplaysAfterDisconnectAcrossHandlers(t *testing.T) {
	s, executionID, _ := backgroundReplayFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	next := func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
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
	go func() { first(httptest.NewRecorder(), backgroundRequest(ctx, executionID, `{}`)); close(returned) }()
	<-started
	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("disconnected waiter blocked")
	}
	conflict := httptest.NewRecorder()
	second(conflict, backgroundRequest(context.Background(), executionID, `{"changed":true}`))
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", conflict.Code)
	}
	close(release)
	s.backgroundCompletions.mu.Lock()
	done := s.backgroundCompletions.entries[executionID].done
	s.backgroundCompletions.mu.Unlock()
	<-done
	for i := 0; i < 2; i++ {
		replay := httptest.NewRecorder()
		second(replay, backgroundRequest(context.Background(), executionID, `{}`))
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
		{"unowned", "test-process-token", `{}`, http.StatusConflict},
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
	s, executionID, _ := backgroundReplayFixture(t)
	calls := 0
	h := withBackgroundCompletionReplay(s, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(strings.Repeat("x", 128*1024+1)))
	})
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h(w, backgroundRequest(context.Background(), executionID, `{}`))
		if w.Code == http.StatusAccepted {
			s.backgroundCompletions.mu.Lock()
			done := s.backgroundCompletions.entries[executionID].done
			s.backgroundCompletions.mu.Unlock()
			<-done
			w = httptest.NewRecorder()
			h(w, backgroundRequest(context.Background(), executionID, `{}`))
		}
		if w.Code != http.StatusInternalServerError || w.Body.Len() > 128*1024 {
			t.Fatalf("unbounded response: %d/%d", w.Code, w.Body.Len())
		}
	}
	if calls != 1 {
		t.Fatalf("failed execution repeated %d times", calls)
	}
}

func TestBackgroundCompletionTaskCancelAndShutdownReachOriginalRun(t *testing.T) {
	for _, mode := range []string{"cancel", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			s, executionID, taskID := backgroundReplayFixture(t)
			started, stopped, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			finish := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(finish)
			var calls atomic.Int32
			handler := withBackgroundCompletionReplay(s, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if owner, ok := r.Context().Value(backgroundExecutionOwnerKey{}).(context.Context); !ok || owner.Done() != r.Context().Done() {
					t.Error("missing authenticated run owner")
				}
				close(started)
				<-r.Context().Done()
				close(stopped)
				<-release
				w.WriteHeader(http.StatusConflict)
			})
			handler(httptest.NewRecorder(), backgroundRequest(context.Background(), executionID, `{}`))
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("original run did not start")
			}
			closed := make(chan struct{})
			if mode == "cancel" {
				if !s.BackgroundTasks.CancelTask(taskID) {
					t.Fatal("task cancel failed")
				}
			} else {
				go func() { _ = s.BackgroundTasks.Close(); close(closed) }()
			}
			select {
			case <-stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("cancel did not reach original run")
			}
			if mode == "shutdown" {
				select {
				case <-closed:
					t.Fatal("shutdown closed stores before original run drained")
				default:
				}
			}
			finish()
			s.backgroundCompletions.mu.Lock()
			done := s.backgroundCompletions.entries[executionID].done
			s.backgroundCompletions.mu.Unlock()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("original run did not drain")
			}
			if mode == "shutdown" {
				select {
				case <-closed:
				case <-time.After(3 * time.Second):
					t.Fatal("manager did not finish shutdown")
				}
			}
			handler(httptest.NewRecorder(), backgroundRequest(context.Background(), executionID, `{}`))
			if calls.Load() != 1 {
				t.Fatal("canceled execution was replayed as a new run")
			}
		})
	}
}
