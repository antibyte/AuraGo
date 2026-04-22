package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func browserAutomationTestConfig(t *testing.T, sidecarURL string) *config.Config {
	t.Helper()

	workspaceDir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	cfg := &config.Config{}
	cfg.Server.MasterKey = strings.Repeat("a", 64)
	cfg.Directories.WorkspaceDir = workspaceDir
	cfg.BrowserAutomation.Enabled = true
	cfg.Tools.BrowserAutomation.Enabled = true
	cfg.BrowserAutomation.URL = sidecarURL
	cfg.BrowserAutomation.Mode = "sidecar"
	cfg.BrowserAutomation.AllowedDownloadDir = "browser_downloads"
	cfg.BrowserAutomation.ScreenshotsDir = "browser_screenshots"
	cfg.BrowserAutomation.AllowFileUploads = true
	cfg.BrowserAutomation.AllowFileDownloads = true
	cfg.BrowserAutomation.Viewport.Width = 1280
	cfg.BrowserAutomation.Viewport.Height = 720
	return cfg
}

func decodeBrowserAutomationResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", raw, err)
	}
	return parsed
}

func TestExecuteBrowserAutomationDisabled(t *testing.T) {
	cfg := &config.Config{}

	result := decodeBrowserAutomationResult(t, ExecuteBrowserAutomation(context.Background(), cfg, BrowserAutomationRequest{
		Operation: "current_state",
	}, nil))

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "disabled") {
		t.Fatalf("message = %q, want disabled error", result["message"])
	}
}

func TestExecuteBrowserAutomationReadOnlyBlocksMutation(t *testing.T) {
	cfg := browserAutomationTestConfig(t, "http://127.0.0.1:7331")
	cfg.BrowserAutomation.ReadOnly = true

	result := decodeBrowserAutomationResult(t, ExecuteBrowserAutomation(context.Background(), cfg, BrowserAutomationRequest{
		Operation: "click",
		SessionID: "ba_123",
		Selector:  "#submit",
	}, nil))

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "read-only") {
		t.Fatalf("message = %q, want read-only error", result["message"])
	}
}

func TestExecuteBrowserAutomationRejectsUploadOutsideWorkspace(t *testing.T) {
	cfg := browserAutomationTestConfig(t, "http://127.0.0.1:7331")

	result := decodeBrowserAutomationResult(t, ExecuteBrowserAutomation(context.Background(), cfg, BrowserAutomationRequest{
		Operation: "upload_file",
		SessionID: "ba_123",
		Selector:  "input[type=file]",
		FilePath:  "../escape.txt",
	}, nil))

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "inside workspace") {
		t.Fatalf("message = %q, want workspace validation error", result["message"])
	}
}

func TestExecuteBrowserAutomationMapsScreenshotAndDownloads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/automation" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"success",
			"operation":"screenshot",
			"session_id":"ba_123",
			"url":"https://example.com",
			"title":"Example",
			"screenshot_rel_path":"browser_screenshots/shot.png",
			"download_rel_path":"ba_123/report.pdf",
			"downloads":[{"name":"report.pdf","rel_path":"ba_123/report.pdf"}]
		}`))
	}))
	defer srv.Close()

	cfg := browserAutomationTestConfig(t, srv.URL)

	result := decodeBrowserAutomationResult(t, ExecuteBrowserAutomation(context.Background(), cfg, BrowserAutomationRequest{
		Operation:  "screenshot",
		SessionID:  "ba_123",
		OutputPath: "browser_screenshots/shot.png",
	}, nil))

	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("status = %q, want success", got)
	}
	screenshotPath, _ := result["screenshot_path"].(string)
	if !strings.HasSuffix(filepath.ToSlash(screenshotPath), "browser_screenshots/shot.png") {
		t.Fatalf("screenshot_path = %q, want browser_screenshots/shot.png suffix", screenshotPath)
	}
	if got, _ := result["screenshot_web_path"].(string); got != "/files/browser_screenshots/shot.png" {
		t.Fatalf("screenshot_web_path = %q, want /files/browser_screenshots/shot.png", got)
	}
	downloadedFile, _ := result["downloaded_file"].(string)
	if !strings.HasSuffix(filepath.ToSlash(downloadedFile), "browser_downloads/ba_123/report.pdf") {
		t.Fatalf("downloaded_file = %q, want browser_downloads/ba_123/report.pdf suffix", downloadedFile)
	}
	if got, _ := result["downloaded_file_web_path"].(string); got != "/files/browser_downloads/ba_123/report.pdf" {
		t.Fatalf("downloaded_file_web_path = %q, want /files/browser_downloads/ba_123/report.pdf", got)
	}
	downloads, ok := result["downloads"].([]interface{})
	if !ok || len(downloads) != 1 {
		t.Fatalf("downloads = %#v, want one entry", result["downloads"])
	}
	entry, ok := downloads[0].(map[string]interface{})
	if !ok {
		t.Fatalf("downloads[0] = %#v, want object", downloads[0])
	}
	if got, _ := entry["web_path"].(string); got != "/files/browser_downloads/ba_123/report.pdf" {
		t.Fatalf("downloads[0].web_path = %q, want /files/browser_downloads/ba_123/report.pdf", got)
	}
}

func TestBrowserAutomationHealthReadsSidecarEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("X-AuraGo-Sidecar-Token"); got == "" {
			t.Fatal("expected X-AuraGo-Sidecar-Token header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","message":"ok","sessions":1}`))
	}))
	defer srv.Close()

	cfg := browserAutomationTestConfig(t, srv.URL)
	result := BrowserAutomationHealth(context.Background(), cfg)

	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("status = %q, want success", got)
	}
	if got, _ := result["sessions"].(float64); got != 1 {
		t.Fatalf("sessions = %v, want 1", got)
	}
}

func TestBrowserAutomationHealthReturnsErrorPayloadForNonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"error","retryable":false,"message":"browser unavailable"}`))
	}))
	defer srv.Close()

	cfg := browserAutomationTestConfig(t, srv.URL)
	result := BrowserAutomationHealth(context.Background(), cfg)

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if retryable, _ := result["retryable"].(bool); retryable {
		t.Fatalf("retryable = true, want false")
	}
}

func TestBrowserAutomationSidecarRequestRetriesTransientFailures(t *testing.T) {
	originalClient := browserAutomationHTTPClient
	t.Cleanup(func() {
		browserAutomationHTTPClient = originalClient
	})

	attempts := 0
	browserAutomationHTTPClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if got := req.Header.Get("X-AuraGo-Sidecar-Token"); got != "test-token" {
				t.Fatalf("X-AuraGo-Sidecar-Token = %q, want test-token", got)
			}
			if attempts == 1 {
				return nil, errors.New("connection refused")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":"success","message":"ok"}`)),
			}, nil
		}),
	}

	result, err := browserAutomationSidecarRequest(context.Background(), BrowserAutomationSidecarConfig{
		URL:       "http://127.0.0.1:7331",
		AuthToken: "test-token",
	}, map[string]interface{}{"operation": "current_state"})
	if err != nil {
		t.Fatalf("browserAutomationSidecarRequest() error = %v", err)
	}
	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("status = %q, want success", got)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestBrowserAutomationAuthTokenUsesMasterKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.MasterKey = strings.Repeat("b", 64)

	token1 := browserAutomationAuthToken(cfg)
	token2 := browserAutomationAuthToken(cfg)
	if token1 == "" {
		t.Fatal("expected auth token to be derived from master key")
	}
	if token1 != token2 {
		t.Fatalf("expected deterministic auth token, got %q and %q", token1, token2)
	}
}

func TestBrowserAutomationManagedURLHost(t *testing.T) {
	tests := []struct {
		name            string
		raw             string
		containerName   string
		runningInDocker bool
		want            string
	}{
		{name: "empty url", raw: "", want: ""},
		{name: "loopback ipv4", raw: "http://127.0.0.1:7331", want: "127.0.0.1"},
		{name: "loopback localhost", raw: "http://localhost:7331", want: "localhost"},
		{name: "loopback ipv6", raw: "http://[::1]:7331", want: "::1"},
		{name: "docker service host", raw: "http://browser-automation:7331", runningInDocker: true, want: "browser-automation"},
		{name: "custom managed container host", raw: "http://aurago_browser_automation:7331", containerName: "aurago_browser_automation", runningInDocker: true, want: "aurago_browser_automation"},
		{name: "docker host ignored outside docker", raw: "http://browser-automation:7331", want: ""},
		{name: "remote host", raw: "https://remote.example.com:7331", runningInDocker: true, want: ""},
	}

	for _, tt := range tests {
		if got := browserAutomationManagedURLHost(tt.raw, tt.containerName, tt.runningInDocker); got != tt.want {
			t.Fatalf("%s: browserAutomationManagedURLHost(%q, %q, %v) = %q, want %q", tt.name, tt.raw, tt.containerName, tt.runningInDocker, got, tt.want)
		}
	}
}

func TestBrowserAutomationEffectiveContainerName(t *testing.T) {
	tests := []struct {
		name        string
		cfg         BrowserAutomationSidecarConfig
		managedHost string
		want        string
	}{
		{
			name:        "loopback keeps default managed name",
			cfg:         BrowserAutomationSidecarConfig{},
			managedHost: "127.0.0.1",
			want:        browserAutomationContainerName,
		},
		{
			name:        "docker service host reuses service name for default config",
			cfg:         BrowserAutomationSidecarConfig{ContainerName: browserAutomationContainerName},
			managedHost: "browser-automation",
			want:        "browser-automation",
		},
		{
			name:        "custom container name stays untouched",
			cfg:         BrowserAutomationSidecarConfig{ContainerName: "custom-browser-sidecar"},
			managedHost: "browser-automation",
			want:        "custom-browser-sidecar",
		},
	}

	for _, tt := range tests {
		if got := browserAutomationEffectiveContainerName(tt.cfg, tt.managedHost); got != tt.want {
			t.Fatalf("%s: browserAutomationEffectiveContainerName(...) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
