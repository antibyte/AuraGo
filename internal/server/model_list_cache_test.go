package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestModelListCacheSharesRefreshAndExpires(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	c := &modelListCache{parent: context.Background(), client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		if r.URL.String() != "https://openrouter.ai/api/v1/models" {
			t.Errorf("unexpected URL %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
	})}}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			c.serveHTTP(w, httptest.NewRequest("GET", "/", nil))
			if w.Code != 200 || w.Body.String() != `{"data":[]}` {
				t.Errorf("response: %d %s", w.Code, w.Body)
			}
		}()
	}
	<-started
	close(release)
	wg.Wait()
	if calls.Load() != 1 || time.Until(c.expires) < 14*time.Minute {
		t.Fatalf("calls=%d expires=%v", calls.Load(), c.expires)
	}
	c.mu.Lock()
	c.expires = time.Time{}
	c.mu.Unlock()
	c.serveHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if calls.Load() != 2 {
		t.Fatal("expired result did not refresh")
	}
}

func TestModelListCacheRejectsAndCachesInvalidResponses(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		status     int
		length     int64
	}{
		{"malformed", `{`, 200, -1},
		{"too large", strings.Repeat(" ", maxModelListBytes+1), 200, -1},
		{"declared too large", `{}`, 200, maxModelListBytes + 1},
		{"upstream failure", `{}`, 503, -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			c := &modelListCache{parent: context.Background(), client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: tt.status, ContentLength: tt.length, Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})}}
			for i := 0; i < 2; i++ {
				w := httptest.NewRecorder()
				c.serveHTTP(w, httptest.NewRequest("GET", "/", nil))
				if w.Code != 502 || strings.Contains(w.Body.String(), tt.body) && len(tt.body) > 2 {
					t.Fatalf("response: %d", w.Code)
				}
			}
			if calls.Load() != 1 || time.Until(c.expires) > 30*time.Second || time.Until(c.expires) < 25*time.Second {
				t.Fatal("negative cache not retained")
			}
		})
	}
}

func TestModelListCacheWaiterAndShutdownCancellation(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	defer stop()
	started, ended := make(chan struct{}), make(chan struct{})
	c := &modelListCache{parent: parent, client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		defer close(ended)
		if d, ok := r.Context().Deadline(); !ok || time.Until(d) > 15*time.Second {
			t.Error("missing fetch deadline")
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}}
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		c.serveHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	}()
	<-started
	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("cancelled waiter blocked")
	}
	select {
	case <-ended:
		t.Fatal("waiter cancelled shared fetch")
	default:
	}
	stop()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel fetch")
	}
	w := httptest.NewRecorder()
	c.serveHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 502 {
		t.Fatalf("shutdown response %d", w.Code)
	}
}
