package dockerutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestVersionTransportNegotiatesBeforeMutations(t *testing.T) {
	for _, tc := range []struct{ server, minimum, want string }{
		{"1.43", "1.24", "v1.43"}, {"1.55", "1.40", "v1.45"}, {"1.44", "1.44", "v1.44"},
		{"1.24", "1.12", ""}, {"1.55", "1.46", ""}, {"../invalid", "1.24", ""},
	} {
		t.Run(tc.server+"-"+tc.minimum, func(t *testing.T) {
			var probes, calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/version" {
					probes.Add(1)
					fmt.Fprintf(w, `{"ApiVersion":%q,"MinAPIVersion":%q}`, tc.server, tc.minimum)
					return
				}
				calls.Add(1)
				if r.URL.Path != "/"+tc.want+"/containers/create" {
					t.Errorf("versioned path = %s", r.URL.Path)
				}
				w.WriteHeader(201)
			}))
			defer s.Close()
			client := NewClient("tcp://"+strings.TrimPrefix(s.URL, "http://"), time.Second)
			defer client.CloseIdleConnections()
			for i := 0; i < 2; i++ {
				_, err := client.DoJSON(context.Background(), "POST", "/containers/create", map[string]string{"Image": "fixture"}, nil)
				if (err == nil) != (tc.want != "") {
					t.Fatalf("negotiation: %v", err)
				}
			}
			if tc.want == "" && calls.Load() != 0 {
				t.Fatal("mutation before valid negotiation")
			}
			if tc.want != "" && (calls.Load() != 2 || probes.Load() != 1) {
				t.Fatalf("calls=%d probes=%d", calls.Load(), probes.Load())
			}
		})
	}
}

func TestVersionTransportCancellationDoesNotCacheFailure(t *testing.T) {
	var count atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			if count.Add(1) == 1 {
				<-r.Context().Done()
				return
			}
			fmt.Fprint(w, `{"ApiVersion":"1.43","MinAPIVersion":"1.24"}`)
			return
		}
		w.WriteHeader(204)
	}))
	defer s.Close()
	c := NewClient("tcp://"+strings.TrimPrefix(s.URL, "http://"), time.Second)
	defer c.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := c.DoJSON(ctx, "POST", "/containers/create", nil, nil); err == nil {
		t.Fatal("cancelled negotiation succeeded")
	}
	if _, err := c.DoJSON(context.Background(), "POST", "/containers/create", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestVersionTransportRenegotiatesAfterEngineRejectsVersion(t *testing.T) {
	var downgraded atomic.Bool
	var probes, v145, v144 atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api := "1.45"
		if downgraded.Load() {
			api = "1.44"
		}
		switch r.URL.Path {
		case "/version":
			probes.Add(1)
			fmt.Fprintf(w, `{"ApiVersion":%q,"MinAPIVersion":"1.24"}`, api)
			return
		case "/v1.45/containers/create":
			v145.Add(1)
		case "/v1.44/containers/create":
			v144.Add(1)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Path != "/v"+api+"/containers/create" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"message":"client version is too new. Maximum supported API version is %s"}`, api)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()
	client := NewClient("tcp://"+strings.TrimPrefix(s.URL, "http://"), time.Second)
	defer client.CloseIdleConnections()
	create := func() (int, error) {
		return client.DoJSON(context.Background(), "POST", "/containers/create", map[string]string{"Image": "fixture"}, nil)
	}
	if _, err := create(); err != nil {
		t.Fatalf("create before downgrade: %v", err)
	}
	downgraded.Store(true) // the Engine restarted with an older release
	if code, err := create(); err == nil || code != http.StatusBadRequest {
		t.Fatalf("create with stale version = %d, %v; want the Engine's 400 returned unchanged", code, err)
	}
	if _, err := create(); err != nil {
		t.Fatalf("create after renegotiation: %v", err)
	}
	if probes.Load() != 2 || v145.Load() != 2 || v144.Load() != 1 {
		t.Fatalf("probes=%d v1.45 creates=%d v1.44 creates=%d; want 2, 2, 1 (the rejected create is never resent)", probes.Load(), v145.Load(), v144.Load())
	}
}

func TestVersionTransportRenegotiatesAfterTransportFailure(t *testing.T) {
	var downgraded, dropNext atomic.Bool
	var probes, creates, v144 atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			probes.Add(1)
			api := "1.45"
			if downgraded.Load() {
				api = "1.44"
			}
			fmt.Fprintf(w, `{"ApiVersion":%q,"MinAPIVersion":"1.24"}`, api)
			return
		}
		creates.Add(1)
		if dropNext.CompareAndSwap(true, false) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}
		if r.URL.Path == "/v1.44/containers/create" {
			v144.Add(1)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()
	client := NewClient("tcp://"+strings.TrimPrefix(s.URL, "http://"), time.Second)
	defer client.CloseIdleConnections()
	create := func() (int, error) {
		return client.DoJSON(context.Background(), "POST", "/containers/create", map[string]string{"Image": "fixture"}, nil)
	}
	if _, err := create(); err != nil {
		t.Fatalf("create before restart: %v", err)
	}
	downgraded.Store(true)
	dropNext.Store(true) // the Engine restarts while the next request is in flight
	if code, err := create(); err == nil || code != 0 {
		t.Fatalf("create on a dropped connection = %d, %v; want a transport error", code, err)
	}
	if _, err := create(); err != nil {
		t.Fatalf("create after renegotiation: %v", err)
	}
	if probes.Load() != 2 || creates.Load() != 3 || v144.Load() != 1 {
		t.Fatalf("probes=%d creates=%d v1.44 creates=%d; want 2, 3, 1 (the dropped create is never resent)", probes.Load(), creates.Load(), v144.Load())
	}
}

func TestVersionTransportKeepsVersionAfterCallerCancellation(t *testing.T) {
	var probes, creates atomic.Int32
	arrived := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			probes.Add(1)
			fmt.Fprint(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`)
			return
		}
		if creates.Add(1) == 1 {
			close(arrived)
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()
	client := NewClient("tcp://"+strings.TrimPrefix(s.URL, "http://"), 5*time.Second)
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-arrived: // the create is in flight; now the caller gives up
			cancel()
		case <-ctx.Done():
		}
	}()
	if _, err := client.DoJSON(ctx, "POST", "/containers/create", nil, nil); err == nil {
		t.Fatal("cancelled create succeeded")
	}
	if _, err := client.DoJSON(context.Background(), "POST", "/containers/create", nil, nil); err != nil {
		t.Fatalf("create after cancellation: %v", err)
	}
	if probes.Load() != 1 || creates.Load() != 2 {
		t.Fatalf("probes=%d creates=%d; want 1, 2 (caller cancellation keeps the negotiated version)", probes.Load(), creates.Load())
	}
}

func TestVersionTransportKeepsVersionAfterClientTimeout(t *testing.T) {
	var probes, creates atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			probes.Add(1)
			fmt.Fprint(w, `{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`)
			return
		}
		if creates.Add(1) == 1 {
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer s.Close()
	// Client.Timeout arms the legacy request-cancel timer next to the context
	// deadline, so the base transport can report "request canceled" while the
	// context still reads nil. That is a timeout, not a stale version.
	client := NewClient("tcp://"+strings.TrimPrefix(s.URL, "http://"), 150*time.Millisecond)
	defer client.CloseIdleConnections()
	if _, err := client.DoJSON(context.Background(), "POST", "/containers/create", nil, nil); err == nil {
		t.Fatal("create past the client timeout succeeded")
	}
	if _, err := client.DoJSON(context.Background(), "POST", "/containers/create", nil, nil); err != nil {
		t.Fatalf("create after client timeout: %v", err)
	}
	if probes.Load() != 1 || creates.Load() != 2 {
		t.Fatalf("probes=%d creates=%d; want 1, 2 (a client timeout keeps the negotiated version)", probes.Load(), creates.Load())
	}
}
