package server

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAgentHTTPServerKeepsSSEAliveThroughMiddleware(t *testing.T) {
	b := NewSSEBroadcaster()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := gzipMiddleware(accessLogMiddleware(logger, b, false))
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
