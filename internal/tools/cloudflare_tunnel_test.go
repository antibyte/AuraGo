package tools

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"
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

func TestCloudflareTunnelReadOnlyBlocksDirectMutations(t *testing.T) {
	cfg := CloudflareTunnelConfig{ReadOnly: true}

	tests := map[string]string{
		"start":        CloudflareTunnelStart(cfg, nil, nil, nil),
		"stop":         CloudflareTunnelStop(cfg, nil, nil),
		"restart":      CloudflareTunnelRestart(cfg, nil, nil, nil),
		"quick tunnel": CloudflareTunnelQuickTunnel(cfg, nil, nil, 8080),
		"install":      CloudflareTunnelInstall(cfg, nil),
	}

	for name, got := range tests {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, `"status":"error"`) || !strings.Contains(strings.ToLower(got), "read-only") {
				t.Fatalf("expected read-only error, got %s", got)
			}
		})
	}
}

func TestTokenTunnelArgsUseRemoteManagedRun(t *testing.T) {
	args := tokenTunnelArgs()
	if want := []string{"tunnel", "run"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("tokenTunnelArgs() = %#v, want %#v", args, want)
	}
	for _, arg := range args {
		if arg == "--url" {
			t.Fatalf("token tunnel args must not include --url: %#v", args)
		}
	}
}

func TestQuickTunnelOriginURLRespectsExplicitPort(t *testing.T) {
	cfg := CloudflareTunnelConfig{
		WebUIPort:      8080,
		HTTPSEnabled:   true,
		HTTPSPort:      8443,
		LoopbackPort:   18080,
		HomepagePort:   3000,
		ExposeWebUI:    true,
		ExposeHomepage: true,
	}

	got, noTLS := quickTunnelOriginURL(cfg, "localhost", 9000)
	if got != "http://localhost:9000" {
		t.Fatalf("quickTunnelOriginURL explicit port = %q, want http://localhost:9000", got)
	}
	if noTLS {
		t.Fatal("quickTunnelOriginURL explicit HTTP port noTLS = true, want false")
	}
}

func TestQuickTunnelOriginURLPrefersLoopbackForDefaultPort(t *testing.T) {
	cfg := CloudflareTunnelConfig{
		WebUIPort:    8080,
		LoopbackPort: 18080,
	}

	got, noTLS := quickTunnelOriginURL(cfg, "localhost", 0)
	if got != "http://127.0.0.1:18080" {
		t.Fatalf("quickTunnelOriginURL loopback = %q, want http://127.0.0.1:18080", got)
	}
	if noTLS {
		t.Fatal("quickTunnelOriginURL loopback noTLS = true, want false")
	}
}

func TestQuickTunnelOriginURLUsesHTTPSWithoutLoopback(t *testing.T) {
	cfg := CloudflareTunnelConfig{
		WebUIPort:      8080,
		HTTPSEnabled:   true,
		HTTPSPort:      8443,
		LoopbackPort:   0,
		ExposeWebUI:    true,
		ExposeHomepage: false,
	}

	got, noTLS := quickTunnelOriginURL(cfg, "localhost", 0)
	if got != "https://localhost:8443" {
		t.Fatalf("quickTunnelOriginURL https = %q, want https://localhost:8443", got)
	}
	if !noTLS {
		t.Fatal("quickTunnelOriginURL https noTLS = false, want true")
	}
}
