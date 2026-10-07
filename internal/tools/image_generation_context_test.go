package tools

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/fileutil"
)

type imageAuditTransport func(*http.Request) (*http.Response, error)

func (f imageAuditTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGenerateImageContextCancelsProviderAndRejectsLatePublication(t *testing.T) {
	previous := imageGenHTTPClient
	defer func() { imageGenHTTPClient = previous }()
	started := make(chan struct{})
	imageGenHTTPClient = &http.Client{Transport: imageAuditTransport(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	dir := t.TempDir()
	cfg := ImageGenConfig{ProviderType: "openai", BaseURL: "https://image.invalid/v1", DataDir: dir}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := GenerateImageContext(ctx, cfg, "fixture", ImageGenOptions{}); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("provider ignored cancellation")
	}
	imageGenHTTPClient = &http.Client{Transport: imageAuditTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a2n8AAAAASUVORK5CYII="}]}`))}, nil
	})}
	ctx = fileutil.WithPublicationGate(context.Background(), func(func() error) error { return context.Canceled })
	if _, err := GenerateImageContext(ctx, cfg, "fixture", ImageGenOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("late publication: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "generated_images"))
	if len(entries) != 0 {
		t.Fatal("cancelled image or temporary file was published")
	}
}
