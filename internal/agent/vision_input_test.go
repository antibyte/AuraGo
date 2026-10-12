package agent

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/tools"
)

var visionInputJPEG = []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0xff, 0xd9}

func newVisionInputFixture(t *testing.T) (*DispatchContext, string, string, *int) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = filepath.Join(root, "agent_workspace", "workdir")
	cfg.Directories.DataDir = filepath.Join(root, "data")
	for _, dir := range []string{cfg.Directories.WorkspaceDir, cfg.Directories.DataDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	db, err := tools.InitMediaRegistryDB(filepath.Join(root, "media.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	path := filepath.Join(cfg.Directories.DataDir, "camera.jpg")
	if err := os.WriteFile(path, visionInputJPEG, 0o600); err != nil {
		t.Fatal(err)
	}
	webPath := "/files/camera.jpg"
	registerVisionInput(t, db, path, webPath, "image")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		wantImage := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(visionInputJPEG)
		if !bytes.Contains(body, []byte(wantImage)) {
			t.Errorf("provider did not receive the snapshot bytes: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"camera analysis succeeded"}}],"usage":{"prompt_tokens":11,"completion_tokens":13}}`)
	}))
	t.Cleanup(server.Close)
	cfg.Vision.APIKey = "vision-fixture-key"
	cfg.Vision.BaseURL = server.URL
	cfg.Budget.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &DispatchContext{Cfg: cfg, Logger: logger, MediaRegistryDB: db, BudgetTracker: budget.NewTracker(cfg, logger, t.TempDir())}, path, webPath, &calls
}

func registerVisionInput(t *testing.T, db *sql.DB, path, webPath, mediaType string) {
	t.Helper()
	if _, _, err := tools.RegisterMedia(db, tools.MediaItem{FilePath: path, WebPath: webPath, Filename: filepath.Base(path), MediaType: mediaType, SourceTool: "go2rtc"}); err != nil {
		t.Fatal(err)
	}
}

func TestRegisteredVisionInputReachesNativeProvider(t *testing.T) {
	dc, path, webPath, calls := newVisionInputFixture(t)
	// Reproduce the old failure and retain the general workspace boundary.
	if _, _, _, err := tools.AnalyzeImageWithPrompt(path, "Describe it", dc.Cfg); err == nil || !strings.Contains(err.Error(), "path traversal denied") {
		t.Fatalf("ordinary workspace input should reject data path: %v", err)
	}
	workspace := dc.Cfg.Directories.WorkspaceDir
	for _, input := range []string{path, webPath} {
		out, handled := dispatchServices(context.Background(), ToolCall{Action: "analyze_image", Params: map[string]interface{}{"file_path": input}}, dc)
		if !handled || !strings.Contains(out, "camera analysis succeeded") {
			t.Fatalf("input %q: handled=%v, output=%s", input, handled, out)
		}
	}
	if *calls != 2 || dc.Cfg.Directories.WorkspaceDir != workspace {
		t.Fatalf("calls=%d; workspace mutated=%v", *calls, dc.Cfg.Directories.WorkspaceDir != workspace)
	}
	if got := dc.BudgetTracker.GetStatus().Models[tools.DefaultVisionModel].Calls; got != 2 {
		t.Fatalf("budget calls=%d, want 2", got)
	}
}

func TestRegisteredVisionInputDeniesUntrustedPaths(t *testing.T) {
	for _, kind := range []string{"unregistered", "deleted", "outside", "traversal", "not-image", "wrong-type", "directory", "oversized", "symlink-escape"} {
		t.Run(kind, func(t *testing.T) {
			dc, path, webPath, calls := newVisionInputFixture(t)
			input := webPath
			switch kind {
			case "unregistered":
				if _, err := dc.MediaRegistryDB.Exec("DELETE FROM media_items"); err != nil {
					t.Fatal(err)
				}
				input = path
			case "deleted":
				if _, err := dc.MediaRegistryDB.Exec("UPDATE media_items SET deleted = 1"); err != nil {
					t.Fatal(err)
				}
			case "outside":
				outside := filepath.Join(t.TempDir(), "outside.jpg")
				if err := os.WriteFile(outside, visionInputJPEG, 0o600); err != nil {
					t.Fatal(err)
				}
				registerVisionInput(t, dc.MediaRegistryDB, outside, "/files/outside.jpg", "image")
				input = "/files/outside.jpg"
			case "traversal":
				input = "/files/../camera.jpg"
			case "not-image":
				if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "wrong-type":
				if _, err := dc.MediaRegistryDB.Exec("UPDATE media_items SET media_type = 'document'"); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				if err := os.Truncate(path, (50<<20)+1); err != nil {
					t.Fatal(err)
				}
			case "symlink-escape":
				outside := filepath.Join(t.TempDir(), "outside.jpg")
				if err := os.WriteFile(outside, visionInputJPEG, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			out, handled := dispatchServices(context.Background(), ToolCall{Action: "analyze_image", Params: map[string]interface{}{"file_path": input}}, dc)
			var payload struct {
				Status string `json:"status"`
			}
			if !handled || json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &payload) != nil || payload.Status != "error" || *calls != 0 {
				t.Fatalf("denial failed: handled=%v calls=%d output=%s", handled, *calls, out)
			}
		})
	}
}

func TestRegisteredVisionInputSupportsMCPAndCleansUp(t *testing.T) {
	previous := dispatchPreferredMCPVision
	t.Cleanup(func() { dispatchPreferredMCPVision = previous })
	for _, fail := range []bool{false, true} {
		dc, original, webPath, calls := newVisionInputFixture(t)
		var staged string
		dispatchPreferredMCPVision = func(_ context.Context, cfg *config.Config, path, _ string, _ *slog.Logger) (string, bool, error) {
			staged = path
			file, err := tools.OpenToolInputFile(path, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			data, err := io.ReadAll(file)
			if err != nil || path == original || !bytes.Equal(data, visionInputJPEG) {
				t.Fatalf("invalid private image: path=%s err=%v", path, err)
			}
			if fail {
				return "", true, assertiveTestError("MCP failed")
			}
			return "MCP analysis succeeded", true, nil
		}
		out, _ := dispatchServices(context.Background(), ToolCall{Action: "analyze_image", Params: map[string]interface{}{"file_path": webPath}}, dc)
		want := "MCP analysis succeeded"
		if fail {
			want = "camera analysis succeeded"
		}
		if !strings.Contains(out, want) || (!fail && *calls != 0) {
			t.Fatalf("output=%s calls=%d", out, *calls)
		}
		if _, err := os.Stat(filepath.Dir(staged)); !os.IsNotExist(err) {
			t.Fatalf("private input not removed: %v", err)
		}
	}
}

func TestGo2RTCSnapshotAnalysisUsesManagedImage(t *testing.T) {
	dc, _, _, calls := newVisionInputFixture(t)
	// Force Snapshot to persist a new dated go2rtc artifact instead of deduping
	// against the generic registered-image fixture.
	if _, err := dc.MediaRegistryDB.Exec("DELETE FROM media_items"); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/go2rtc/proxy/api":
			_, _ = io.WriteString(w, `{"version":"1.9.14"}`)
		case "/api/go2rtc/proxy/api/streams":
			_, _ = io.WriteString(w, `{"aurago_driveway":{"producers":[{}],"consumers":[]}}`)
		case "/api/go2rtc/proxy/api/frame.jpeg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(visionInputJPEG)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	dc.Cfg.Go2RTC = config.Go2RTCConfig{Enabled: true, AgentAccess: true, URL: upstream.URL, APIHostPort: port, APIPassword: "camera-fixture-password", Streams: []config.Go2RTCStreamConfig{{ID: "driveway", Enabled: true, Source: "rtsp://camera.local/live"}}}
	manager := tools.NewGo2RTCManager(dc.Cfg, nil, dc.MediaRegistryDB, dc.Logger)
	if _, err := manager.Test(context.Background()); err != nil {
		t.Fatal(err)
	}
	previous := tools.DefaultGo2RTCManager()
	tools.SetDefaultGo2RTCManager(manager)
	t.Cleanup(func() { tools.SetDefaultGo2RTCManager(previous) })
	status, _ := dispatchPlatform(context.Background(), ToolCall{Action: "go2rtc", Operation: "stream_status", Params: map[string]interface{}{"stream_id": "driveway"}}, dc)
	if !strings.Contains(status, `"reachable":false`) || !strings.Contains(status, "reachability_note") {
		t.Fatalf("idle status=%s", status)
	}
	out, handled := dispatchPlatform(context.Background(), ToolCall{Action: "go2rtc", Operation: "analyze_snapshot", Params: map[string]interface{}{"stream_id": "driveway"}}, dc)
	if !handled || !strings.Contains(out, `"analysis":"camera analysis succeeded"`) || !strings.Contains(out, "/files/go2rtc/snapshots/") || *calls != 1 {
		t.Fatalf("output=%s calls=%d", out, *calls)
	}
	if got := dc.BudgetTracker.GetStatus().Models[tools.DefaultVisionModel].Calls; got != 1 {
		t.Fatalf("budget calls=%d", got)
	}
}

func TestThreeDPrinterAnalysisUsesManagedImage(t *testing.T) {
	dc, _, _, calls := newVisionInputFixture(t)
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(visionInputJPEG)
	}))
	t.Cleanup(imageServer.Close)
	wsURL, closeWS := mockAgentThreeDPrinterCameraURLServer(t, imageServer.URL+"/snapshot.jpg")
	t.Cleanup(closeWS)
	cameraCfg := agentThreeDPrinterConfig(t, wsURL)
	dc.Cfg.ThreeDPrinters = cameraCfg.ThreeDPrinters
	runtimeCfg := tools.BuildThreeDPrinterRuntimeConfig(dc.Cfg)
	if out := tools.ExecuteThreeDPrinter(context.Background(), runtimeCfg, tools.ThreeDPrinterRequest{Operation: "enable_camera"}); !strings.Contains(out, `"status":"ok"`) {
		t.Fatal(out)
	}
	out := handleThreeDPrinterAnalyzeCamera(context.Background(), dc.Cfg, runtimeCfg, threeDPrinterArgs{PrinterID: "lab"}, dc.BudgetTracker)
	if !strings.Contains(out, `"analysis":"camera analysis succeeded"`) || *calls != 1 {
		t.Fatalf("output=%s calls=%d", out, *calls)
	}
}
