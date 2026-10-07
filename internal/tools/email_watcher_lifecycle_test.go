package tools

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestEmailWatcherStopCancelsTLSHandshakeAndCanRestart(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := &config.Config{EmailAccounts: []config.EmailAccount{{
		ID: "fixture", IMAPHost: "127.0.0.1", IMAPPort: listener.Addr().(*net.TCPAddr).Port,
		Username: "fixture", Password: "fixture", WatchEnabled: true,
	}}}
	watcher := NewEmailWatcher(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	defer watcher.Stop()
	for run := 0; run < 2; run++ {
		accepted, closed := make(chan struct{}), make(chan struct{})
		go func() {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				close(accepted)
				close(closed)
				return
			}
			defer conn.Close()
			close(accepted)
			_, _ = io.Copy(io.Discard, conn) // Never complete the TLS handshake.
			close(closed)
		}()
		ctx, cancel := context.WithCancel(context.Background())
		watcher.StartContext(ctx)
		select {
		case <-accepted:
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("watcher never opened IMAP connection")
		}
		if run == 1 {
			cancel() // Parent shutdown must have the same effect as explicit Stop.
		}
		done := make(chan struct{})
		go func() {
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); watcher.Stop() }()
			}
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			cancel()
			t.Fatal("Stop did not join blocked IMAP worker")
		}
		cancel()
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("IMAP socket remained open after Stop")
		}
	}
}

// The watcher's loopback is an internal turn: it carries the internal
// follow-up header and the per-process token like the other loopbacks, so the
// server labels it internal and accepts it when web auth is enabled.
func TestEmailWatcherNotificationIsAnInternalLoopback(t *testing.T) {
	type seen struct{ followUp, token string }
	got := make(chan seen, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		got <- seen{r.Header.Get("X-Internal-FollowUp"), r.Header.Get("X-Internal-Token")}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.Port, _ = strconv.Atoi(port)
	watcher := NewEmailWatcher(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	watcher.internalToken = "email-loopback-token"
	watcher.notifyAgent(context.Background(), "fixture")
	select {
	case h := <-got:
		if h.followUp != "true" || h.token != "email-loopback-token" {
			t.Fatalf("loopback headers = %+v, want internal follow-up with the process token", h)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("notification was not sent")
	}

	// The server start path hands the token to the watcher it starts.
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := closed.Addr().(*net.TCPAddr).Port
	_ = closed.Close()
	watchCfg := &config.Config{EmailAccounts: []config.EmailAccount{{
		ID: "fixture", IMAPHost: "127.0.0.1", IMAPPort: closedPort,
		Username: "fixture", Password: "fixture", WatchEnabled: true,
	}}}
	started := StartEmailWatcherContext(context.Background(), watchCfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil, "started-token")
	if started == nil {
		t.Fatal("watcher with a watch-enabled account was not started")
	}
	defer started.Stop()
	if started.internalToken != "started-token" {
		t.Fatalf("started watcher token = %q, want the process token", started.internalToken)
	}
}

func TestEmailWatcherNotificationHonorsCancellation(t *testing.T) {
	entered, cancelled := make(chan struct{}), make(chan struct{})
	headerCh := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		headerCh <- r.Header.Clone()
		close(entered)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Server.Port, _ = strconv.Atoi(port)
	watcher := NewEmailWatcher(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	watcher.SetInternalToken("internal-secret")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); watcher.notifyAgent(ctx, "fixture") }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("notification was not sent")
	}
	select {
	case headers := <-headerCh:
		if headers.Get("X-Internal-FollowUp") != "true" || headers.Get("X-Internal-Token") != "internal-secret" {
			t.Fatalf("notification internal-auth headers = %#v", headers)
		}
	case <-time.After(time.Second):
		t.Fatal("notification headers were not observed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("notification did not stop on cancellation")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("notification transport remained open")
	}
}
