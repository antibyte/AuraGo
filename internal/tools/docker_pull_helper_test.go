package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/dockerutil"
)

const helperTestImage = "ghcr.io/example/app:latest"

func TestPullDockerImageStreamReportsStatusAsTypedError(t *testing.T) {
	var pulls atomic.Int32
	host := pullOnlyDockerHost(t, http.StatusNotFound, `{"message":"manifest unknown"}`, &pulls)
	err := pullDockerImageStream(context.Background(), DockerConfig{Host: host}, helperTestImage)
	var pullErr *dockerPullError
	if !errors.As(err, &pullErr) || pullErr.StatusCode != http.StatusNotFound || pullErr.Message != "manifest unknown" || pullErr.Stream {
		t.Fatalf("err = %#v, want a 404 *dockerPullError carrying the Engine message", err)
	}
	if got := err.Error(); got != "pull image "+helperTestImage+": HTTP 404: manifest unknown" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestPullDockerImageStreamReportsStreamFailures(t *testing.T) {
	t.Run("error event after 200", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, pullStreamFailure, &pulls)
		err := pullDockerImageStream(context.Background(), DockerConfig{Host: host}, helperTestImage)
		var pullErr *dockerPullError
		var event *dockerutil.JSONMessageError
		if !errors.As(err, &pullErr) || pullErr.StatusCode != 0 || !pullErr.Stream || !errors.As(err, &event) {
			t.Fatalf("err = %#v, want a stream *dockerPullError wrapping the error event", err)
		}
		if got := err.Error(); got != "pull image "+helperTestImage+": failed to register layer: no space left on device" {
			t.Fatalf("Error() = %q", got)
		}
	})
	t.Run("stream cut inside a message", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, `{"status":"Downloading"}`+"\n"+`{"status":"Downlo`, &pulls)
		if err := pullDockerImageStream(context.Background(), DockerConfig{Host: host}, helperTestImage); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v, want a cut-stream error", err)
		}
	})
	t.Run("clean stream", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, pullStreamSuccess, &pulls)
		if err := pullDockerImageStream(context.Background(), DockerConfig{Host: host}, helperTestImage); err != nil || pulls.Load() != 1 {
			t.Fatalf("err = %v, pulls = %d; want nil and one pull", err, pulls.Load())
		}
	})
	t.Run("another 2xx status drains the stream", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusCreated, pullStreamSuccess, &pulls)
		if err := pullDockerImageStream(context.Background(), DockerConfig{Host: host}, helperTestImage); err != nil {
			t.Fatalf("err = %v, want nil for a clean stream behind a 2xx status", err)
		}
	})
}

func TestPullDockerImageStreamHonoursItsContext(t *testing.T) {
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"Pulling fs layer"}`+"\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := pullDockerImageStream(ctx, DockerConfig{Host: host}, helperTestImage)
	if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("err = %v, want the caller's deadline", err)
	}
}

func TestPullDockerImageStreamLeavesPermissionsToCallers(t *testing.T) {
	configureDockerSecurityTestPermissions(t, true)
	var pulls atomic.Int32
	host := pullOnlyDockerHost(t, http.StatusOK, pullStreamSuccess, &pulls)
	if err := pullDockerImageStream(context.Background(), DockerConfig{Host: host}, helperTestImage); err != nil || pulls.Load() != 1 {
		t.Fatalf("err = %v, pulls = %d; the helper must not apply runtime permissions (each caller keeps its own gate)", err, pulls.Load())
	}
}

func TestPullDockerImageQuerySendsTheGivenQuery(t *testing.T) {
	var rawQuery atomic.Value
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		rawQuery.Store(r.URL.RawQuery)
		_, _ = io.WriteString(w, pullStreamSuccess)
	})
	cfg := DockerConfig{Host: host}
	if err := pullDockerImageQuery(context.Background(), cfg, "ollama/ollama", url.Values{"fromImage": {"ollama/ollama"}, "tag": {"latest"}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := rawQuery.Load().(string); got != "fromImage=ollama%2Follama&tag=latest" {
		t.Fatalf("raw query = %q", got)
	}
	if err := pullDockerImageStream(context.Background(), cfg, helperTestImage); err != nil {
		t.Fatal(err)
	}
	if got, _ := rawQuery.Load().(string); got != "fromImage=ghcr.io%2Fexample%2Fapp%3Alatest" {
		t.Fatalf("raw query = %q, want today's fromImage escaping", got)
	}
}

func TestDetachedPullContextIgnoresCallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	<-ctx.Done()
	detached := detachedPullContext(ctx)
	if detached.Err() != nil {
		t.Fatalf("detached context error = %v, want nil", detached.Err())
	}
	if _, ok := detached.Deadline(); ok {
		t.Fatal("detached context kept the caller's deadline")
	}
	//nolint:staticcheck // a nil context must not panic
	if detachedPullContext(nil) == nil {
		t.Fatal("detachedPullContext(nil) = nil")
	}
}

// Pins: the four existing pull sites keep their exact texts.
func TestDockerPullSiteTextsStayUnchanged(t *testing.T) {
	const notFound = `{"message":"manifest unknown"}`
	t.Run("PullImageWait and PullImageForce", func(t *testing.T) {
		for name, pull := range map[string]func(DockerConfig) error{
			"wait":  func(cfg DockerConfig) error { return PullImageWait(context.Background(), cfg, helperTestImage, nil) },
			"force": func(cfg DockerConfig) error { return PullImageForce(context.Background(), cfg, helperTestImage, nil) },
		} {
			var pulls atomic.Int32
			err := pull(DockerConfig{Host: pullOnlyDockerHost(t, http.StatusNotFound, notFound, &pulls)})
			if err == nil || err.Error() != "pull image "+helperTestImage+": HTTP 404: manifest unknown" {
				t.Fatalf("%s 404: %v", name, err)
			}
			err = pull(DockerConfig{Host: pullOnlyDockerHost(t, http.StatusOK, pullStreamFailure, &pulls)})
			if err == nil || err.Error() != "pull image "+helperTestImage+": failed to register layer: no space left on device" {
				t.Fatalf("%s stream: %v", name, err)
			}
		}
	})
	t.Run("DockerPullImageContext", func(t *testing.T) {
		var pulls atomic.Int32
		for _, tc := range []struct {
			status int
			body   string
			want   string
		}{
			{http.StatusOK, pullStreamFailure, "Failed to pull image: failed to register layer: no space left on device"},
			{http.StatusNotFound, notFound, "Docker error (HTTP 404): manifest unknown"},
			{http.StatusNotFound, "", "Docker error (HTTP 404)"},
		} {
			result := decodeDockerToolResult(t, DockerPullImageContext(context.Background(), DockerConfig{Host: pullOnlyDockerHost(t, tc.status, tc.body, &pulls)}, helperTestImage))
			if result["status"] != "error" || result["message"] != tc.want {
				t.Fatalf("result = %#v, want %q", result, tc.want)
			}
		}
	})
	t.Run("container update", func(t *testing.T) {
		for _, tc := range []struct {
			status int
			body   string
			want   string
		}{
			{http.StatusOK, pullStreamFailure, "Failed to pull latest image for manifestdotbuild/manifest:5: failed to register layer: no space left on device"},
			{http.StatusNotFound, notFound, "Failed to pull latest image for manifestdotbuild/manifest:5: HTTP 404: manifest unknown"},
			{http.StatusBadGateway, "", "Failed to pull latest image for manifestdotbuild/manifest:5: HTTP 502: Bad Gateway"},
		} {
			host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
				switch {
				case r.Method == http.MethodGet && path == "/containers/demo/json":
					writeDockerJSON(w, http.StatusOK, map[string]interface{}{
						"Id": "old-container-id", "Name": "/manifest",
						"State":  map[string]interface{}{"Running": true},
						"Config": map[string]interface{}{"Image": "manifestdotbuild/manifest:5"},
					})
				case r.Method == http.MethodPost && path == "/images/create":
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
				default:
					t.Errorf("unexpected Docker request after the failed pull: %s %s", r.Method, path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			})
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(DockerUpdateContainerImage(context.Background(), DockerConfig{Host: host}, "demo", nil)), &result); err != nil {
				t.Fatal(err)
			}
			if result["message"] != tc.want {
				t.Fatalf("message = %v, want %q", result["message"], tc.want)
			}
		}
	})
}
