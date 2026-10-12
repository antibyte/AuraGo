package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
)

func TestPersonAlertAgentQueueIsBoundedAndDrainsInOrder(t *testing.T) {
	var q personAlertAgentQueue
	for i := range personAlertAgentQueueSize {
		if err := q.enqueue(fmt.Sprintf("p%d", i)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	if err := q.enqueue("overflow"); !errors.Is(err, persondetect.ErrAgentQueueFull) {
		t.Fatalf("seventeenth prompt: %v, want ErrAgentQueueFull", err)
	}
	delivered := make(chan string, personAlertAgentQueueSize)
	q.start(context.Background(), func(_ context.Context, prompt string) { delivered <- prompt })
	q.start(context.Background(), func(context.Context, string) { t.Error("a second consumer started") })
	for i := range personAlertAgentQueueSize {
		select {
		case got := <-delivered:
			if want := fmt.Sprintf("p%d", i); got != want {
				t.Fatalf("delivered %q, want %q", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("prompt %d was not delivered", i)
		}
	}
	q.stop()
	var skip *persondetect.SkipError
	if err := q.enqueue("late"); !errors.As(err, &skip) || skip.Reason != "agent unavailable" {
		t.Fatalf("enqueue after stop: %v", err)
	}
	q.stop() // idempotent
}

func TestPersonAlertAgentQueueEndsWithTheServerContext(t *testing.T) {
	var q personAlertAgentQueue
	ctx, cancel := context.WithCancel(context.Background())
	q.start(ctx, func(context.Context, string) {})
	cancel()
	done := make(chan struct{})
	go func() {
		q.stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop hung after the server context ended")
	}
}

func TestCameraDetectionRunConfigMirrorsUptimeKuma(t *testing.T) {
	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	run := s.buildCameraDetectionRunConfig()
	if run.SessionID != "camera_detection" || run.MessageSource != "camera_detection" || !run.SuppressTurnSideEffects {
		t.Fatalf("run = session %q, source %q, suppress %v", run.SessionID, run.MessageSource, run.SuppressTurnSideEffects)
	}
	if run.Config != cfg || run.Logger != s.Logger || run.Manifest == nil {
		t.Fatalf("shared fields were not copied: %+v", run)
	}
	uptime := s.buildUptimeKumaRunConfig()
	if uptime.SessionID != "uptime_kuma" || uptime.MessageSource != "uptime_kuma" || uptime.SuppressTurnSideEffects {
		t.Fatal("the Uptime Kuma run config must stay unchanged")
	}
}
