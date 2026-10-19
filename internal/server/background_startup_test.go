package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestBackgroundTasksStartAfterHTTPAndTLSListenerBind(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprintf("tls=%t", secure), func(t *testing.T) {
			reserved, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := reserved.Addr().String()
			reserved.Close()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			manager := tools.NewBackgroundTaskManager(t.TempDir(), logger)
			defer manager.Close()
			s := &Server{Cfg: &config.Config{}, Logger: logger, BackgroundTasks: manager}
			server := newAgentHTTPServer(addr, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			client, scheme := &http.Client{Timeout: time.Second}, "http"
			if secure {
				certificate := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				server.TLSConfig = certificate.TLS.Clone()
				certificate.Close()
				client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
				scheme = "https"
			}
			defer client.CloseIdleConnections()
			manager.SetLoopbackExecutorContext(func(ctx context.Context, _, _, _ string, _ time.Duration) error {
				if !s.ready.Load() {
					return fmt.Errorf("background task started before readiness")
				}
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+addr+"/", nil)
				if err != nil {
					return err
				}
				response, err := client.Do(request)
				if err != nil {
					return err
				}
				response.Body.Close()
				if response.StatusCode != http.StatusNoContent {
					return fmt.Errorf("unexpected status %d", response.StatusCode)
				}
				return nil
			})
			task, err := manager.ScheduleFollowUp("startup fixture", tools.BackgroundTaskScheduleOptions{MaxRetries: 0})
			if err != nil {
				t.Fatal(err)
			}
			shutdown, served := make(chan struct{}), make(chan error, 1)
			go func() { served <- s.serveWithShutdown(server, nil, nil, shutdown) }()
			defer func() {
				close(shutdown)
				select {
				case err := <-served:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(3 * time.Second):
					t.Error("server did not stop")
				}
			}()
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				stored, _ := manager.GetTask(task.ID)
				if stored.Status == tools.BackgroundTaskStatusCompleted {
					return
				}
				if stored.Status == tools.BackgroundTaskStatusFailed {
					t.Fatalf("zero-retry task failed during startup: %s", stored.LastError)
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatal("due background task never started after listener bind")
		})
	}
}

func TestFailedListenerBindDoesNotConsumeBackgroundTask(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := tools.NewBackgroundTaskManager(t.TempDir(), logger)
	defer manager.Close()
	manager.SetLoopbackExecutorContext(func(context.Context, string, string, string, time.Duration) error {
		t.Error("task started without a bound listener")
		return nil
	})
	task, err := manager.ScheduleFollowUp("must remain queued", tools.BackgroundTaskScheduleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: &config.Config{}, Logger: logger, BackgroundTasks: manager}
	if err := s.serveWithShutdown(newAgentHTTPServer(occupied.Addr().String(), http.NotFoundHandler()), nil, nil, make(chan struct{})); err == nil {
		t.Fatal("occupied listener unexpectedly bound")
	}
	stored, _ := manager.GetTask(task.ID)
	if stored.Status != tools.BackgroundTaskStatusQueued || stored.ExecutionID != "" {
		t.Fatalf("failed startup consumed task: %+v", stored)
	}
}
