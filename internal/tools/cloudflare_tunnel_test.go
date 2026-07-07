package tools

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func TestCloudflareTunnelStartQuickUsesDefaultOriginWithoutExplicitPort(t *testing.T) {
	cfg := CloudflareTunnelConfig{
		AuthMethod:   "quick",
		Mode:         "native",
		WebUIPort:    8080,
		LoopbackPort: 18080,
	}

	args := captureNativeQuickTunnelArgs(t, cfg, func(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
		return CloudflareTunnelStart(cfg, nil, registry, logger)
	})

	if got := argAfter(args, "--url"); got != "http://127.0.0.1:18080" {
		t.Fatalf("CloudflareTunnelStart quick --url = %q, want loopback default; args=%#v", got, args)
	}
}

func TestCloudflareTunnelQuickTunnelZeroPortUsesDefaultOrigin(t *testing.T) {
	cfg := CloudflareTunnelConfig{
		Mode:         "native",
		WebUIPort:    8080,
		LoopbackPort: 18080,
	}

	args := captureNativeQuickTunnelArgs(t, cfg, func(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
		return CloudflareTunnelQuickTunnel(cfg, registry, logger, 0)
	})

	if got := argAfter(args, "--url"); got != "http://127.0.0.1:18080" {
		t.Fatalf("CloudflareTunnelQuickTunnel zero port --url = %q, want loopback default; args=%#v", got, args)
	}
}

func captureNativeQuickTunnelArgs(t *testing.T, cfg CloudflareTunnelConfig, start func(CloudflareTunnelConfig, *ProcessRegistry, *slog.Logger) string) []string {
	t.Helper()

	resetCloudflareTunnelRuntimeForTest()
	t.Cleanup(resetCloudflareTunnelRuntimeForTest)

	root := t.TempDir()
	cfg.DataDir = filepath.Join(root, "data")
	argsPath := filepath.Join(root, "cloudflared-args.txt")
	buildFakeCloudflared(t, cfg.DataDir)
	t.Setenv("CFD_ARGS_FILE", argsPath)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewProcessRegistry(logger)
	result := start(cfg, registry, logger)
	t.Cleanup(func() {
		_ = CloudflareTunnelStop(cfg, registry, logger)
	})
	if !strings.Contains(result, `"status":"ok"`) {
		t.Fatalf("quick tunnel start result = %s, want status ok", result)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(argsPath)
		if err == nil {
			return strings.Split(strings.TrimSpace(string(data)), "\n")
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fake cloudflared did not write args to %s", argsPath)
	return nil
}

func buildFakeCloudflared(t *testing.T, dataDir string) {
	t.Helper()

	binPath := cfdBinaryPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(binPath), 0o700); err != nil {
		t.Fatalf("MkdirAll fake cloudflared dir: %v", err)
	}
	srcPath := filepath.Join(t.TempDir(), "fake_cloudflared.go")
	src := `package main

import (
	"os"
	"strings"
	"time"
)

func main() {
	if argsPath := os.Getenv("CFD_ARGS_FILE"); argsPath != "" {
		_ = os.WriteFile(argsPath, []byte(strings.Join(os.Args[1:], "\n")), 0600)
	}
	time.Sleep(10 * time.Second)
}
`
	if err := os.WriteFile(srcPath, []byte(src), 0o600); err != nil {
		t.Fatalf("write fake cloudflared source: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake cloudflared: %v\n%s", err, string(out))
	}
}

func resetCloudflareTunnelRuntimeForTest() {
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	tunnelPID = 0
	tunnelMode = ""
	tunnelURL = ""
	tunnelStarted = time.Time{}
	tunnelStopping = false
}

func argAfter(args []string, key string) string {
	for i, arg := range args {
		if arg == key && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
