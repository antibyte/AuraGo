package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// pullStreamFailure is an image pull that fails after the Engine already
// answered 200: progress first, then the error event.
const pullStreamFailure = `{"status":"Pulling fs layer","progressDetail":{},"id":"4f4fb700ef54"}` + "\n" +
	`{"errorDetail":{"message":"failed to register layer: no space left on device"},"error":"failed to register layer: no space left on device"}` + "\n"

const pullStreamSuccess = `{"status":"Pulling from example/app","id":"latest"}` + "\n" +
	`{"status":"Status: Downloaded newer image for ghcr.io/example/app:latest"}` + "\n"

// pullOnlyDockerHost serves an empty local image list and answers every image
// pull with status and body; any other request fails the test.
func pullOnlyDockerHost(t *testing.T, status int, body string, pulls *atomic.Int32) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/"+dockerAPIVersion+"/images/json":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/"+dockerAPIVersion+"/images/create":
			pulls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		default:
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func decodeDockerToolResult(t *testing.T, raw string) map[string]string {
	t.Helper()
	var result map[string]string
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("parse tool result: %v\n%s", err, raw)
	}
	return result
}

func TestPullImageWaitReturnsDockerStreamError(t *testing.T) {
	const image = "ghcr.io/example/app:latest"
	t.Run("error event after 200", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, pullStreamFailure, &pulls)
		err := PullImageWait(context.Background(), DockerConfig{Host: host}, image, nil)
		if err == nil || !strings.Contains(err.Error(), "no space left on device") {
			t.Fatalf("PullImageWait() error = %v, want the stream's error event", err)
		}
		if pulls.Load() != 1 {
			t.Fatalf("pull requests = %d, want 1", pulls.Load())
		}
	})
	t.Run("non-200 carries the engine message", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusNotFound, `{"message":"manifest for ghcr.io/example/app:latest not found: manifest unknown"}`, &pulls)
		err := PullImageWait(context.Background(), DockerConfig{Host: host}, image, nil)
		if err == nil || !strings.Contains(err.Error(), "HTTP 404") || !strings.Contains(err.Error(), "manifest unknown") {
			t.Fatalf("PullImageWait() error = %v, want HTTP 404 with the Engine message", err)
		}
	})
	t.Run("clean stream succeeds", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, pullStreamSuccess, &pulls)
		if err := PullImageWait(context.Background(), DockerConfig{Host: host}, image, nil); err != nil {
			t.Fatalf("PullImageWait() error = %v, want nil", err)
		}
		if pulls.Load() != 1 {
			t.Fatalf("pull requests = %d, want 1", pulls.Load())
		}
	})
}

func TestDockerUpdateContainerImageStopsOnPullStreamError(t *testing.T) {
	var touched atomic.Int32
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
		switch {
		case r.Method == http.MethodGet && path == "/containers/demo/json":
			writeDockerJSON(w, http.StatusOK, map[string]interface{}{
				"Id":     "old-container-id",
				"Name":   "/manifest",
				"State":  map[string]interface{}{"Running": true},
				"Config": map[string]interface{}{"Image": "manifestdotbuild/manifest:5"},
			})
		case r.Method == http.MethodPost && path == "/images/create":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(pullStreamFailure))
		default:
			touched.Add(1)
			t.Errorf("container changed after a failed pull: %s %s", r.Method, path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	raw := DockerUpdateContainerImage(context.Background(), DockerConfig{Host: host}, "demo", nil)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("parse result: %v\n%s", err, raw)
	}
	message, _ := result["message"].(string)
	if result["status"] != "error" ||
		!strings.Contains(message, "Failed to pull latest image for manifestdotbuild/manifest:5") ||
		!strings.Contains(message, "no space left on device") {
		t.Fatalf("result = %s, want the pull stream error before any container change", raw)
	}
	if touched.Load() != 0 {
		t.Fatalf("%d container requests after the failed pull, want 0 (no stop, rename, create or remove)", touched.Load())
	}
}

func TestDockerPullImageReportsStreamError(t *testing.T) {
	const image = "ghcr.io/example/app:latest"
	t.Run("error event after 200", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, pullStreamFailure, &pulls)
		result := decodeDockerToolResult(t, DockerPullImage(DockerConfig{Host: host}, image))
		if result["status"] != "error" || !strings.Contains(result["message"], "no space left on device") {
			t.Fatalf("DockerPullImage() = %#v, want an error naming the stream failure", result)
		}
	})
	t.Run("clean stream keeps the success result", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, pullStreamSuccess, &pulls)
		result := decodeDockerToolResult(t, DockerPullImage(DockerConfig{Host: host}, image))
		if result["status"] != "ok" || result["message"] != "Image '"+image+"' pulled successfully" {
			t.Fatalf("DockerPullImage() = %#v, want the unchanged success result", result)
		}
	})
	t.Run("non-200 keeps the Docker error result", func(t *testing.T) {
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusNotFound, `{"message":"pull access denied for example/app"}`, &pulls)
		result := decodeDockerToolResult(t, DockerPullImage(DockerConfig{Host: host}, image))
		if result["status"] != "error" || result["message"] != "Docker error (HTTP 404): pull access denied for example/app" {
			t.Fatalf("DockerPullImage() = %#v, want the unchanged Docker error result", result)
		}
	})
	t.Run("read-only denies before the pull", func(t *testing.T) {
		configureDockerSecurityTestPermissions(t, true)
		var pulls atomic.Int32
		host := pullOnlyDockerHost(t, http.StatusOK, pullStreamSuccess, &pulls)
		result := decodeDockerToolResult(t, DockerPullImage(DockerConfig{Host: host}, image))
		if result["status"] != "error" || !strings.Contains(result["message"], "docker mutation is disabled") {
			t.Fatalf("DockerPullImage() = %#v, want the read-only denial", result)
		}
		if pulls.Load() != 0 {
			t.Fatalf("pull requests = %d, want 0 under read-only", pulls.Load())
		}
	})
	t.Run("caller cancellation stops the pull", func(t *testing.T) {
		host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/"+dockerAPIVersion+"/images/create" {
				t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL.String())
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"Pulling fs layer"}` + "\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
		})
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		started := time.Now()
		result := decodeDockerToolResult(t, DockerPullImageContext(ctx, DockerConfig{Host: host}, image))
		if result["status"] != "error" || time.Since(started) > 5*time.Second {
			t.Fatalf("DockerPullImageContext() = %#v after %s, want a prompt error on cancellation", result, time.Since(started))
		}
	})
}
