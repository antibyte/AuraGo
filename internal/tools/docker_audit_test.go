package tools

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

func TestDockerMutationNeverRetriesLostOrFailedResponse(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	for _, failure := range []string{"lost", "503", "short"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/version" {
					fmt.Fprint(w, `{"ApiVersion":"1.43"}`)
					return
				}
				calls.Add(1)
				if failure == "lost" {
					conn, _, _ := w.(http.Hijacker).Hijack()
					conn.Close()
					return
				}
				if failure == "short" {
					w.Header().Set("Content-Length", "100")
					fmt.Fprint(w, "short")
					return
				}
				w.WriteHeader(503)
			}))
			defer s.Close()
			_, _, _ = DockerRequest(DockerConfig{Host: "tcp://" + strings.TrimPrefix(s.URL, "http://")}, "POST", "/containers/create", `{"Image":"fixture"}`)
			if calls.Load() != 1 {
				t.Fatalf("mutation sent %d times", calls.Load())
			}
		})
	}
}

func TestDockerExecUsesCallerCancellationAndStreamingTimeout(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	var starts atomic.Int32
	release := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/version":
			fmt.Fprint(w, `{"ApiVersion":"1.43"}`)
		case strings.HasSuffix(r.URL.Path, "/exec"):
			w.WriteHeader(201)
			fmt.Fprint(w, `{"Id":"run-1"}`)
		case strings.HasSuffix(r.URL.Path, "/start"):
			starts.Add(1)
			select {
			case <-r.Context().Done():
			case <-release:
			}
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer s.Close()
	defer close(release)
	cfg := DockerConfig{Host: "tcp://" + strings.TrimPrefix(s.URL, "http://")}
	if getPullDockerClient(cfg).Timeout != 0 {
		t.Fatal("stream uses fixed client timeout")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	before := time.Now()
	_, _, err := dockerExecRawContext(ctx, cfg, "fixture", []string{"sleep", "60"}, "", nil)
	if err == nil || time.Since(before) > time.Second || starts.Load() != 1 {
		t.Fatalf("cancellation=%v starts=%d", err, starts.Load())
	}
}
