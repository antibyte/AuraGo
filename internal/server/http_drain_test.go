package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHTTPDrainClosesHijackedWebSocketBeforeWaiting(t *testing.T) {
	s := &Server{}
	upgraded := make(chan struct{})
	handlerDone := make(chan struct{})
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := s.trackHTTP(gzipMiddleware(accessLogMiddleware(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer close(handlerDone)
		defer conn.Close()
		close(upgraded)
		// Request cancellation alone cannot interrupt this WebSocket read.
		_, _, _ = conn.ReadMessage()
	}), false)))
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case <-upgraded:
	case <-time.After(time.Second):
		t.Fatal("WebSocket handler did not start")
	}

	s.beginHTTPDrain()
	requestsDone := make(chan struct{})
	go func() {
		s.httpRequests.Wait()
		close(requestsDone)
	}()
	select {
	case <-requestsDone:
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket read kept HTTP shutdown waiting")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("WebSocket handler did not exit")
	}
	s.httpDrainMu.Lock()
	remaining := len(s.httpHijacked)
	s.httpDrainMu.Unlock()
	if remaining != 0 {
		t.Fatalf("tracked WebSockets after drain = %d", remaining)
	}
}

// A handler that waits on the server lifetime instead of its request context
// (the go2rtc image pull) must not hold HTTP shutdown: the drain also ends the
// server lifetime, on shutdownCh and when Serve ends on its own.
func TestHTTPDrainEndsTheServerLifetime(t *testing.T) {
	s := &Server{}
	lifetime, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.setServerLifetimeCancel(cancel)
	started := make(chan struct{})
	handler := s.trackHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-lifetime.Done() // ignores r.Context(), like a detached sidecar pull
		w.WriteHeader(http.StatusOK)
	}))
	go handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/go2rtc/start", nil))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	s.beginHTTPDrain()
	requestsDone := make(chan struct{})
	go func() {
		s.httpRequests.Wait()
		close(requestsDone)
	}()
	select {
	case <-requestsDone:
	case <-time.After(2 * time.Second):
		t.Fatal("a handler bound to the server lifetime kept HTTP shutdown waiting")
	}
}
