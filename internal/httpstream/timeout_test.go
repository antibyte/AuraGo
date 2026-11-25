package httpstream

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWriteTimeoutTLS(t *testing.T) {
	for _, http2 := range []bool{false, true} {
		for _, tc := range []struct {
			name        string
			contentType string
			status      int
			streaming   bool
		}{
			{"sse", "text/event-stream; charset=utf-8", 200, true},
			{"mjpeg", "multipart/x-mixed-replace; boundary=frame", 200, true},
			{"json", "application/json", 200, false},
			{"error", "text/event-stream", 500, false},
		} {
			t.Run(fmt.Sprintf("http2=%v/%s", http2, tc.name), func(t *testing.T) {
				t.Parallel()
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", tc.contentType)
					w.WriteHeader(tc.status)
					for i := 0; i < 18; i++ {
						if _, err := io.WriteString(w, "frame\n"); err != nil {
							return
						}
						if err := http.NewResponseController(w).Flush(); err != nil {
							return
						}
						select {
						case <-r.Context().Done():
							return
						case <-time.After(100 * time.Millisecond):
						}
					}
				})
				srv := httptest.NewUnstartedServer(WithWriteTimeout(handler, time.Second))
				srv.EnableHTTP2 = http2
				srv.Config.WriteTimeout = time.Second
				srv.StartTLS()
				defer srv.Close()
				client := srv.Client()
				client.Timeout = 5 * time.Second
				resp, err := client.Get(srv.URL)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				if (resp.ProtoMajor == 2) != http2 {
					t.Fatalf("unexpected protocol %s", resp.Proto)
				}
				body, err := io.ReadAll(resp.Body)
				if tc.streaming {
					if err != nil || string(body) != strings.Repeat("frame\n", 18) {
						t.Fatalf("active stream failed: bytes=%d, error=%v", len(body), err)
					}
				} else if err == nil {
					t.Fatal("ordinary/error response escaped the absolute write timeout")
				}
			})
		}
	}
}

// stallTLSClient connects without ever reading the HTTP response.
func stallTLSClient(t *testing.T, srv *httptest.Server) {
	t.Helper()
	tlsConfig := srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", strings.TrimPrefix(srv.URL, "https://"), tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.NetConn().(*net.TCPConn).SetReadBuffer(1024); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
}

func TestWriteTimeoutStopsBlockedTLSClient(t *testing.T) {
	done := make(chan error, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		payload := bytes.Repeat([]byte("x"), 64<<10)
		for {
			if _, err := w.Write(payload); err != nil {
				_, retryErr := w.Write(payload)
				if r.Context().Err() == nil || retryErr != err {
					done <- fmt.Errorf("failed stream was not cancelled or remained writable")
				} else {
					done <- nil
				}
				return
			}
		}
	})
	srv := httptest.NewUnstartedServer(WithWriteTimeout(handler, 200*time.Millisecond))
	// A broken renewal implementation must not pass on the global timeout.
	srv.Config.WriteTimeout = 30 * time.Second
	srv.StartTLS()
	defer srv.Close()
	stallTLSClient(t, srv)
	// Complete TLS, then deliberately never read the HTTP response.
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	// crypto/tls allows five seconds to send close_notify after a failed write.
	case <-time.After(7 * time.Second):
		t.Fatal("blocked stream exceeded its write budget")
	}
}

func TestWriteTimeoutKeepsStricterHandlerDeadline(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler func() time.Time
		budget  time.Duration
	}{
		{"handler wins", func() time.Time { return time.Now().Add(200 * time.Millisecond) }, time.Minute},
		{"budget caps", func() time.Time { return time.Now().Add(time.Minute) }, 200 * time.Millisecond},
		{"zero cannot clear", func() time.Time { return time.Time{} }, 200 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan struct{}, 1)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				payload := bytes.Repeat([]byte("x"), 64<<10)
				rc := http.NewResponseController(w)
				for {
					if err := rc.SetWriteDeadline(tc.handler()); err != nil {
						t.Errorf("SetWriteDeadline: %v", err)
						done <- struct{}{}
						return
					}
					if _, err := w.Write(payload); err != nil {
						done <- struct{}{}
						return
					}
				}
			})
			srv := httptest.NewUnstartedServer(WithWriteTimeout(handler, tc.budget))
			srv.Config.WriteTimeout = 30 * time.Second
			srv.StartTLS()
			defer srv.Close()
			stallTLSClient(t, srv)
			select {
			case <-done:
			// crypto/tls allows five seconds to send close_notify after a failed write.
			case <-time.After(7 * time.Second):
				t.Fatal("blocked stream outlived the stricter deadline")
			}
		})
	}
}

func TestWriteTimeoutOrdinaryResponseKeepsHandlerExtendedDeadline(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rc := http.NewResponseController(w)
		if err := rc.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Errorf("SetWriteDeadline: %v", err)
			return
		}
		for i := 0; i < 6; i++ {
			if _, err := io.WriteString(w, "chunk\n"); err != nil {
				return
			}
			_ = rc.Flush()
			time.Sleep(250 * time.Millisecond)
		}
	})
	srv := httptest.NewUnstartedServer(WithWriteTimeout(handler, time.Minute))
	srv.Config.WriteTimeout = time.Second
	srv.StartTLS()
	defer srv.Close()
	client := srv.Client()
	client.Timeout = 8 * time.Second
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != strings.Repeat("chunk\n", 6) {
		t.Fatalf("handler-extended deadline was lost: bytes=%d, error=%v", len(body), err)
	}
}

func TestWriteTimeoutZeroHandlerDeadlineKeepsOrdinaryResponseBounded(t *testing.T) {
	done := make(chan struct{}, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
			t.Errorf("SetWriteDeadline: %v", err)
			done <- struct{}{}
			return
		}
		payload := bytes.Repeat([]byte("x"), 64<<10)
		for {
			if _, err := w.Write(payload); err != nil {
				done <- struct{}{}
				return
			}
		}
	})
	srv := httptest.NewUnstartedServer(WithWriteTimeout(handler, 200*time.Millisecond))
	srv.Config.WriteTimeout = 30 * time.Second
	srv.StartTLS()
	defer srv.Close()
	stallTLSClient(t, srv)
	select {
	case <-done:
	// crypto/tls allows five seconds to send close_notify after a failed write.
	case <-time.After(7 * time.Second):
		t.Fatal("zero handler deadline cleared the ordinary response's write bound")
	}
}

func TestWriteTimeoutPreservesWebSocketHijack(t *testing.T) {
	srv := httptest.NewServer(WithWriteTimeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		kind, data, err := conn.ReadMessage()
		if err == nil {
			_ = conn.WriteMessage(kind, data)
		}
	}), time.Second))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil || string(message) != "ping" {
		t.Fatalf("WebSocket echo failed: %q, %v", message, err)
	}
}

type cancelOnDeadlineWriter struct {
	*httptest.ResponseRecorder
	cancel        context.CancelFunc
	deadlineCalls int
	lastDeadline  time.Time
}

func (w *cancelOnDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadlineCalls++
	w.lastDeadline = deadline
	w.cancel()
	return nil
}

func TestWriteTimeoutDoesNotReviveCanceledStream(t *testing.T) {
	for _, cancelBeforeWrite := range []bool{true, false} {
		t.Run(fmt.Sprintf("cancelBeforeWrite=%v", cancelBeforeWrite), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := &cancelOnDeadlineWriter{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
			if cancelBeforeWrite {
				cancel()
			}
			handler := WithWriteTimeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if _, err := io.WriteString(w, "late frame\n"); !errors.Is(err, context.Canceled) {
					t.Errorf("canceled stream write = %v, want context.Canceled", err)
				}
			}), time.Minute)
			handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx))
			if writer.Body.Len() != 0 {
				t.Fatal("canceled stream still wrote a response")
			}
			if cancelBeforeWrite && writer.deadlineCalls != 0 {
				t.Fatal("canceled stream renewed its deadline")
			}
		})
	}
}

func TestWriteTimeoutHandlerDeadlineDoesNotReviveCanceledStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &cancelOnDeadlineWriter{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	handler := WithWriteTimeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		cancel()
		before := writer.deadlineCalls
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Minute)); !errors.Is(err, context.Canceled) {
			t.Errorf("SetWriteDeadline after cancellation = %v, want context.Canceled", err)
		}
		if writer.deadlineCalls != before {
			t.Error("canceled stream reached the underlying writer's deadline")
		}
	}), time.Minute)
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx))
}
