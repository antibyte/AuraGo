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

	if err := verifyCloudflaredChecksum("", tmp.Name()); err == nil {
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

func TestQuickTunnelOriginRejectsUnmanagedDefaults(t *testing.T) {
	cfg := CloudflareTunnelConfig{WebUIPort: 8080, LoopbackPort: 18080, HTTPSEnabled: true, HTTPSPort: 8443, HomepagePort: 3000}
	for _, port := range []int{0, 2375, 9000} {
		if got := quickTunnelOriginURL(cfg, "localhost", port); got != "" {
			t.Fatalf("unmanaged origin: %s", got)
		}
	}
}

func TestCloudflareQuickStartAndDirectUseManagedOrigin(t *testing.T) {
	for _, method := range []string{"start", "quick", "restart"} {
		t.Run(method, func(t *testing.T) {
			cfg := fixtureHomepageQuickConfig(t)
			cfg.WebUIPort = 8080
			cfg.LoopbackPort = 18080
			args := captureNativeQuickTunnelArgs(t, cfg, func(cfg CloudflareTunnelConfig, registry *ProcessRegistry, logger *slog.Logger) string {
				if method == "start" {
					return CloudflareTunnelStart(cfg, nil, registry, logger)
				}
				if method == "restart" {
					return CloudflareTunnelRestart(cfg, nil, registry, logger)
				}
				return CloudflareTunnelQuickTunnel(cfg, registry, logger, 0)
			})
			got := argAfter(args, "--url")
			if !strings.HasPrefix(got, "http://127.0.0.1:") || got == "http://127.0.0.1:18080" || got == "http://127.0.0.1:8080" {
				t.Fatalf("unmanaged target: %q", got)
			}
			if argAfter(args, "--http-host-header") == "" {
				t.Fatal("managed origin binding missing")
			}
		})
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
	tunnelLifecycleMu.Lock()
	defer tunnelLifecycleMu.Unlock()
	tunnelPolicy = nil
	clearCloudflareState()
	tunnelMu.Lock()
	defer tunnelMu.Unlock()
	tunnelPID = 0
	tunnelExit = nil
	tunnelDockerHost = ""
	tunnelContainerID = ""
	tunnelAuth = ""
	tunnelWarnings = nil
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
