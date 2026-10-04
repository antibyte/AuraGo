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
