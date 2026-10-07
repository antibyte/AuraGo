package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func buildErrorHost(t *testing.T, status int, body string, hold bool) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
		if hold {
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
		}
	})
}

func TestBuildImageWaitErrorBodies(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	const image = "aurago/code-studio:latest"
	build := func(ctx context.Context, host string) error {
		return BuildImageWait(ctx, DockerConfig{Host: host}, image, "Dockerfile", []byte("FROM alpine\n"), nil, nil)
	}
	t.Run("engine message keeps today's text", func(t *testing.T) {
		err := build(context.Background(), buildErrorHost(t, http.StatusForbidden, `{"message":"build denied"}`, false))
		if err == nil || err.Error() != "build image "+image+": HTTP 403: build denied" {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("empty body keeps today's text", func(t *testing.T) {
		err := build(context.Background(), buildErrorHost(t, http.StatusInternalServerError, "", false))
		if err == nil || err.Error() != "build image "+image+": HTTP 500: " {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("a body of control bytes never reaches the error", func(t *testing.T) {
		err := build(context.Background(), buildErrorHost(t, http.StatusInternalServerError, "\x1b\x00\x07\n", false))
		if err == nil || err.Error() != "build image "+image+": HTTP 500: " {
			t.Fatalf("err = %q, want no raw control bytes", err)
		}
	})
	t.Run("an endless error body is not awaited", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		started := time.Now()
		err := build(ctx, buildErrorHost(t, http.StatusInternalServerError, strings.Repeat("x", 70<<10), true))
		want := "build image " + image + ": HTTP 500: " + strings.Repeat("x", 500) + "..."
		if err == nil || err.Error() != want {
			t.Fatalf("err = %v, want %q", err, want)
		}
		if elapsed := time.Since(started); elapsed > 3*time.Second {
			t.Fatalf("waited %s for the rest of the error body", elapsed)
		}
	})
}
