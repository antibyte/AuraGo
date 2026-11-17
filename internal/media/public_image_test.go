package media

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type imagePolicyTransport func(*http.Request) (*http.Response, error)

func (f imagePolicyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAnswerImageDownloadsRejectPrivateTargets(t *testing.T) {
	for _, url := range []string{"http://127.0.0.1/image.png", "http://10.0.0.1/image.png", "http://169.254.169.254/image.png", "http://[::1]/image.png", "http://[::ffff:127.0.0.1]/image.png"} {
		dir := t.TempDir()
		if _, err := DownloadAndSanitizeImageContext(context.Background(), url, dir); err == nil || !strings.Contains(err.Error(), "blocked") {
			t.Fatalf("private image URL not rejected: %s: %v", url, err)
		}
		if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
			t.Fatalf("rejected URL left an image: %v, %v", entries, err)
		}
	}
}

func TestAnswerImageRedirectsRecheckPrivateTargets(t *testing.T) {
	original := httpClientPublicImage
	client := *original
	client.Transport = imagePolicyTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "93.184.216.34" {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"http://127.0.0.1/private.png"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		return original.Transport.RoundTrip(r)
	})
	httpClientPublicImage = &client
	t.Cleanup(func() { httpClientPublicImage = original })
	if _, err := SaveURLToDir("http://93.184.216.34/image.png", t.TempDir()); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("private redirect was not rejected: %v", err)
	}
}
