package server

import (
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
