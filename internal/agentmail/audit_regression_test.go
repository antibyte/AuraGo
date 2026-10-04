package agentmail

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMutationIsNeverReplayedByDefault(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.Error(w, "unavailable", 503) }))
	defer srv.Close()
	c, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "fixture-only", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SendMessage(context.Background(), "inbox", SendMessageRequest{Text: "fixture"})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("mutation attempts %d, error %v", calls.Load(), err)
	}
}

func TestAcceptedMutationWithLostResponseIsNotRepeated(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		conn.Close()
	}))
	defer srv.Close()
	c, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "fixture-only", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SendMessage(context.Background(), "inbox", SendMessageRequest{Text: "fixture"})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("accepted mutation repeated: attempts=%d err=%v", calls.Load(), err)
	}
}

func TestRelayWaitsForSuccessfulSeedAndStops(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()
	c, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "fixture-only", HTTPClient: srv.Client(), DisableRetries: true})
	if err != nil {
		t.Fatal(err)
	}
	var notifications atomic.Int32
	s := NewService(ServiceConfig{Config: Config{Enabled: true, RelayToAgent: true, InboxID: "inbox", APIKey: "fixture-only"}, Client: c, Notify: func(context.Context, string) error { notifications.Add(1); return nil }})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	s.Stop(ctx)
	if s.Running() || notifications.Load() != 0 {
		t.Fatal("unseeded or stopped relay remained active")
	}
}

func TestAcceptedRelaySurvivesFailedLabel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unavailable", 503) }))
	defer srv.Close()
	c, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "fixture-only", HTTPClient: srv.Client(), DisableRetries: true})
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	s := NewService(ServiceConfig{Config: Config{InboxID: "inbox"}, Client: c, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Notify: func(context.Context, string) error { calls.Add(1); return nil }})
	_ = s.handleMessage(context.Background(), Message{ID: "m1", Text: "fixture"})
	_ = s.handleMessage(context.Background(), Message{ID: "m1", Text: "fixture"})
	if calls.Load() != 1 {
		t.Fatalf("relay delivered %d times", calls.Load())
	}
}
