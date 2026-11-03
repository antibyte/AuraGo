package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/httpstream"

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

func TestHTTPDrainInterruptsBlockedIO(t *testing.T) {
	for _, mode := range []string{"upload", "stream"} {
		t.Run(mode, func(t *testing.T) {
			s := &Server{}
			started := make(chan struct{})
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(started)
				if mode == "upload" {
					_, _ = io.Copy(io.Discard, r.Body)
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				payload := make([]byte, 64<<10)
				for {
					if _, err := w.Write(payload); err != nil {
						return
					}
				}
			})
			srv := httptest.NewServer(s.trackHTTP(httpstream.WithWriteTimeout(handler, time.Minute)))
			defer srv.Close()
			client, err := net.DialTimeout("tcp", srv.Listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			if err := client.(*net.TCPConn).SetReadBuffer(1024); err != nil {
				t.Fatal(err)
			}
			request := "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"
			if mode == "upload" {
				request = "POST / HTTP/1.1\r\nHost: example.com\r\nContent-Length: 1024\r\n\r\nx"
			}
			if _, err := io.WriteString(client, request); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("handler did not start")
			}

			// The client leaves its upload unfinished or never reads the stream.
			s.beginHTTPDrain()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := srv.Config.Shutdown(ctx); err != nil {
				t.Fatalf("blocked %s held HTTP shutdown: %v", mode, err)
			}
			s.httpRequests.Wait()
		})
	}
}
