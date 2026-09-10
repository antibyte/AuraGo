package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHAContextCancellation(t *testing.T) {
	cancelled := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done(); cancelled <- struct{}{} }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	result := HAGetStatesContext(ctx, HAConfig{URL: srv.URL}, "switch")
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("cancel result: %s", result)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("HTTP request ignored cancellation")
	}
}

func TestHAContextMalformedStates(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `not json`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		result := HAGetStatesContext(context.Background(), HAConfig{URL: srv.URL}, "switch")
		srv.Close()
		if !strings.Contains(result, `"status":"error"`) {
			t.Fatalf("accepted malformed states: %s", body)
		}
	}
}
