package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLLMResponseTimeoutTracksStreamProgress(t *testing.T) {
	for _, mode := range []string{"stream", "stalled", "json", "headers", "cancel", "close"} {
		t.Run(mode, func(t *testing.T) {
			done := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(done)
				if mode == "headers" {
					<-r.Context().Done()
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if mode == "json" {
					w.Header().Set("Content-Type", "application/json")
				}
				w.(http.Flusher).Flush()
				for range 7 {
					fmt.Fprint(w, "data: progress\n\n")
					w.(http.Flusher).Flush()
					if mode == "stalled" || mode == "close" {
						<-r.Context().Done()
						return
					}
					select {
					case <-r.Context().Done():
						return
					case <-time.After(80 * time.Millisecond):
					}
				}
			}))
			defer server.Close()
			client := buildLLMHTTPClient(nil, "openai", "", server.URL)
			client.Transport.(*responseTimeoutTransport).timeout = 250 * time.Millisecond
			defer client.CloseIdleConnections()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if mode == "cancel" {
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 120*time.Millisecond)
				defer stop()
			}
			req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
			if mode != "json" {
				req.Header.Set("Accept", "text/event-stream")
			}
			response, err := client.Do(req)
			if err == nil {
				if mode == "close" {
					err = response.Body.Close()
				} else {
					_, err = io.ReadAll(response.Body)
					response.Body.Close()
				}
			}
			if mode == "stream" || mode == "close" {
				if err != nil {
					t.Fatalf("active stream or explicit close failed: %v", err)
				}
			} else if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected bounded timeout, got %v", err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("response left a running request after completion/cancellation")
			}
		})
	}
}
