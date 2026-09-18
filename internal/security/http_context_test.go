package security

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPClientContextCancelsHeadersAndBody(t *testing.T) {
	for _, body := range []bool{false, true} {
		t.Run(map[bool]string{false: "headers", true: "body"}[body], func(t *testing.T) {
			reached := make(chan struct{})
			released := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if body {
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
				}
				close(reached)
				<-r.Context().Done()
				close(released)
			}))
			defer server.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := HTTPClientWithContext(server.Client(), ctx)
			result := make(chan error, 1)
			go func() {
				r, err := client.Get(server.URL)
				if err == nil {
					_, err = io.ReadAll(r.Body)
					r.Body.Close()
				}
				result <- err
			}()
			<-reached
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("HTTP operation ignored cancellation")
			}
			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("server request remained active")
			}
		})
	}
}

func TestHTTPClientContextPreservesRedirectPolicy(t *testing.T) {
	blocked := errors.New("policy stopped redirect")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/blocked", 302) }))
	defer server.Close()
	original := server.Client()
	original.CheckRedirect = func(*http.Request, []*http.Request) error { return blocked }
	client := HTTPClientWithContext(original, context.Background())
	if _, err := client.Get(server.URL); !errors.Is(err, blocked) {
		t.Fatalf("policy lost: %v", err)
	}
}
