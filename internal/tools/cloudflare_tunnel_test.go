package tools

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type cloudflareRoundTripFunc func(*http.Request) (*http.Response, error)

func (f cloudflareRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestRandomHexReturnsErrorWhenRandomFails(t *testing.T) {
	prev := cloudflareRandRead
	cloudflareRandRead = func([]byte) (int, error) {
		return 0, errors.New("entropy unavailable")
	}
	defer func() { cloudflareRandRead = prev }()

	if _, err := randomHex(4); err == nil {
		t.Fatal("expected random failure to be returned")
	}
}

func TestVerifyCloudflaredChecksumFailsClosedWhenUnavailable(t *testing.T) {
	t.Parallel()

	tmp, err := os.CreateTemp(t.TempDir(), "cloudflared-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := tmp.WriteString("binary"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := &http.Client{Transport: cloudflareRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("missing")),
			Header:     make(http.Header),
		}, nil
	})}
	if err := verifyCloudflaredChecksum(client, "https://example.invalid/checksum", tmp.Name(), logger); err == nil {
		t.Fatal("expected unavailable checksum to fail closed")
	}
}

func TestCloudflaredDownloadMetadataFailsClosedWithoutChecksum(t *testing.T) {
	t.Parallel()

	_, err := cloudflaredDownloadMetadata("windows", "amd64")
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected unsupported checksum metadata error, got %v", err)
	}
}

func TestCloudflareAPIClientHasTimeout(t *testing.T) {
	if cfHTTPClient == nil {
		t.Fatal("cfHTTPClient is nil")
	}
	if cfHTTPClient.Timeout < 10*time.Second {
		t.Fatalf("cfHTTPClient.Timeout = %v, want explicit production timeout", cfHTTPClient.Timeout)
	}
}
