package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestAgentHTTPServerKeepsSSEAliveThroughMiddleware(t *testing.T) {
	b := NewSSEBroadcaster()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Server.HTTPS.Enabled = true
	handler := gzipMiddleware(accessLogMiddleware(logger, securityHeadersMiddleware(s.httpsTransitionMiddleware(b), true, false), false))
	srv := httptest.NewUnstartedServer(newAgentHTTPServer("", handler).Handler)
	srv.Config.WriteTimeout = 500 * time.Millisecond
	srv.StartTLS()
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Exercise both response-writer wrappers (the /events path skips them).
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	go func() {
		for i := 0; i < 30; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(50 * time.Millisecond):
				b.Send("test", "alive")
			}
		}
		b.Send("test", "stream-complete")
	}()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "stream-complete") {
			return
		}
	}
	t.Fatalf("active SSE ended at the server write timeout: %v", scanner.Err())
}

// sseChain builds the production middleware chain around h and serves it with
// the real 40-minute httpstream budget; the global write timeout cannot rescue it.
func sseChain(t *testing.T, tlsMode bool, h http.Handler) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{Cfg: &config.Config{}}
	var inner http.Handler
	if tlsMode {
		s.Cfg.Server.HTTPS.Enabled = true
		inner = s.httpsTransitionMiddleware(h)
	} else {
		inner = h
	}
	chain := s.trackHTTP(newAgentHTTPServer("", gzipMiddleware(accessLogMiddleware(logger, securityHeadersMiddleware(inner, tlsMode, false), false))).Handler)
	srv := httptest.NewUnstartedServer(chain)
	srv.Config.WriteTimeout = 30 * time.Second
	if tlsMode {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)
	return srv
}

func TestSSEHandlerDeadlineDropsStalledClientThroughMiddleware(t *testing.T) {
	for _, tlsMode := range []bool{true, false} {
		name := "http"
		if tlsMode {
			name = "tls"
		}
		t.Run(name, func(t *testing.T) {
			done := make(chan error, 1)
			frame := []byte("data: " + strings.Repeat("x", 64<<10) + "\n\n")
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				rc := http.NewResponseController(w)
				for {
					if err := rc.SetWriteDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
						if errors.Is(err, http.ErrNotSupported) {
							done <- fmt.Errorf("SetWriteDeadline did not reach the connection: %w", err)
						} else {
							// A stream that already failed reports its write error here.
							done <- nil
						}
						return
					}
					if _, err := w.Write(frame); err != nil {
						done <- nil
						return
					}
					if err := rc.Flush(); err != nil {
						done <- nil
						return
					}
				}
			})
			srv := sseChain(t, tlsMode, h)
			var conn net.Conn
			var err error
			if tlsMode {
				cfg := srv.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
				conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", srv.Listener.Addr().String(), cfg)
			} else {
				conn, err = net.DialTimeout("tcp", srv.Listener.Addr().String(), 2*time.Second)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			raw := conn
			if tc, ok := conn.(*tls.Conn); ok {
				raw = tc.NetConn()
			}
			if err := raw.(*net.TCPConn).SetReadBuffer(1024); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(conn, "GET /api/test/stream HTTP/1.1\r\nHost: example.com\r\nAccept-Encoding: gzip\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			// crypto/tls allows five seconds to send close_notify after a failed write.
			case <-time.After(7 * time.Second):
				t.Fatal("stalled SSE client kept its stream past the handler deadline")
			}
		})
	}
}

func TestSSEHandlerDeadlineKeepsActiveClientThroughMiddleware(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		send := func(msg string) bool {
			if err := rc.SetWriteDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
				return false
			}
			_, err := io.WriteString(w, "data: "+msg+"\n\n")
			return err == nil && rc.Flush() == nil
		}
		for i := 0; i < 30; i++ {
			if !send("alive") {
				return
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
		send("stream-complete")
	})
	srv := sseChain(t, true, h)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/test/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "stream-complete") {
			return
		}
	}
	t.Fatalf("active SSE was dropped before completion: %v", scanner.Err())
}
