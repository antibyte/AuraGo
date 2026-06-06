package agent

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

func TestBuildRuntimeTTSConfigIncludesMiniMaxAndPiper(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = "data"
	cfg.TTS.Provider = "minimax"
	cfg.TTS.Language = "de"
	cfg.TTS.CacheRetentionHours = 24
	cfg.TTS.CacheMaxFiles = 10
	cfg.TTS.ElevenLabs.APIKey = "el-key"
	cfg.TTS.ElevenLabs.VoiceID = "el-voice"
	cfg.TTS.ElevenLabs.ModelID = "el-model"
	cfg.TTS.MiniMax.APIKey = "mm-key"
	cfg.TTS.MiniMax.VoiceID = "mm-voice"
	cfg.TTS.MiniMax.ModelID = "mm-model"
	cfg.TTS.MiniMax.Speed = 1.25
	cfg.TTS.Piper.Enabled = true
	cfg.TTS.Piper.ContainerPort = 10200
	cfg.TTS.Piper.Voice = "de_DE-thorsten-high"
	cfg.TTS.Piper.SpeakerID = 3

	ttsCfg := buildRuntimeTTSConfig(cfg, "")
	if ttsCfg.Provider != "minimax" {
		t.Fatalf("Provider = %q, want minimax", ttsCfg.Provider)
	}
	if ttsCfg.Language != "de" {
		t.Fatalf("Language = %q, want de", ttsCfg.Language)
	}
	if ttsCfg.MiniMax.APIKey != "mm-key" || ttsCfg.MiniMax.VoiceID != "mm-voice" || ttsCfg.MiniMax.ModelID != "mm-model" {
		t.Fatalf("MiniMax config not copied: %+v", ttsCfg.MiniMax)
	}
	if ttsCfg.MiniMax.Speed != 1.25 {
		t.Fatalf("MiniMax.Speed = %v, want 1.25", ttsCfg.MiniMax.Speed)
	}
	if ttsCfg.Piper.Port != 10200 || ttsCfg.Piper.Voice != "de_DE-thorsten-high" || ttsCfg.Piper.SpeakerID != 3 {
		t.Fatalf("Piper config not copied: %+v", ttsCfg.Piper)
	}
}

func TestIsTTSConfiguredRequiresUsableBackend(t *testing.T) {
	if isTTSConfigured(&config.Config{}) {
		t.Fatal("empty TTS config should not be considered configured")
	}

	googleCfg := &config.Config{}
	googleCfg.TTS.Provider = "google"
	if !isTTSConfigured(googleCfg) {
		t.Fatal("google provider should be considered configured")
	}

	missingKeyCfg := &config.Config{}
	missingKeyCfg.TTS.Provider = "minimax"
	if isTTSConfigured(missingKeyCfg) {
		t.Fatal("minimax without API key should not be considered configured")
	}

	minimaxCfg := &config.Config{}
	minimaxCfg.TTS.Provider = "minimax"
	minimaxCfg.TTS.MiniMax.APIKey = "mm-key"
	if !isTTSConfigured(minimaxCfg) {
		t.Fatal("minimax with API key should be considered configured")
	}

	piperCfg := &config.Config{}
	piperCfg.TTS.Piper.Enabled = true
	if !isTTSConfigured(piperCfg) {
		t.Fatal("enabled Piper should be considered configured")
	}
}

func TestResolveChromecastTargetFallsBackToDiscovery(t *testing.T) {
	orig := discoverChromecastDevices
	discoverChromecastDevices = func(logger *slog.Logger) ([]tools.ChromecastDevice, error) {
		return []tools.ChromecastDevice{
			{
				Name:         "Google-Home-Mini-b39e08d8ca5bd6baa7ed277fd1bb1437",
				FriendlyName: "Arbeitszimmer",
				Addr:         "192.168.6.130",
				Port:         8009,
			},
		}, nil
	}
	defer func() {
		discoverChromecastDevices = orig
	}()

	req := chromecastArgs{DeviceName: "Arbeitszimmer"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := resolveChromecastTarget(&req, nil, logger); err != nil {
		t.Fatalf("resolveChromecastTarget returned error: %v", err)
	}
	if req.DeviceAddr != "192.168.6.130" {
		t.Fatalf("DeviceAddr = %q, want 192.168.6.130", req.DeviceAddr)
	}
	if req.DevicePort != 8009 {
		t.Fatalf("DevicePort = %d, want 8009", req.DevicePort)
	}
}

func TestPrepareChromecastLocalMediaURLCopiesWorkspaceFileToTTSDir(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "agent_workspace", "workdir")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("MkdirAll workdir: %v", err)
	}
	src := filepath.Join(workdir, "ueberall_zuhause.mp3")
	if err := os.WriteFile(src, []byte("fake mp3"), 0o644); err != nil {
		t.Fatalf("WriteFile src: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workdir
	cfg.Directories.DataDir = dataDir
	cfg.Server.Host = "192.168.6.238"
	cfg.Chromecast.TTSPort = 8090

	req := chromecastArgs{LocalPath: "workdir/ueberall_zuhause.mp3"}
	if err := prepareChromecastLocalMediaURL(cfg, &req); err != nil {
		t.Fatalf("prepareChromecastLocalMediaURL: %v", err)
	}
	if req.URL != "http://192.168.6.238:8090/tts/ueberall_zuhause.mp3" {
		t.Fatalf("URL = %q", req.URL)
	}
	if req.ContentType != "audio/mpeg" {
		t.Fatalf("ContentType = %q, want audio/mpeg", req.ContentType)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "tts", "ueberall_zuhause.mp3")); err != nil {
		t.Fatalf("expected published file in tts dir: %v", err)
	}
}

func TestDispatchThreeDPrinterAnalyzeCameraUsesDefaultVisionBudgetModel(t *testing.T) {
	originalAnalyze := dispatchAnalyzeImageWithPrompt
	defer func() { dispatchAnalyzeImageWithPrompt = originalAnalyze }()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xff, 0xd8, 0xff, 0xdb})
	}))
	defer imageServer.Close()
	wsURL, closeWS := mockAgentThreeDPrinterCameraURLServer(t, imageServer.URL+"/snapshot.jpg")
	defer closeWS()

	dispatchAnalyzeImageWithPrompt = func(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
		if _, err := os.Stat(filePath); err != nil {
			t.Fatalf("snapshot path should exist: %v", err)
		}
		return "camera looks healthy", 11, 13, nil
	}

	cfg := agentThreeDPrinterConfig(t, wsURL)
	cfg.Budget.Enabled = true
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	out, ok := dispatchPlatform(context.Background(), ToolCall{
		Action: "three_d_printer",
		Params: map[string]interface{}{
			"operation":  "analyze_camera",
			"printer_id": "lab",
		},
	}, &DispatchContext{
		Cfg:           cfg,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		BudgetTracker: tracker,
	})
	if !ok {
		t.Fatal("expected dispatchPlatform to handle three_d_printer")
	}
	if !strings.Contains(out, `"analysis":"camera looks healthy"`) {
		t.Fatalf("unexpected output: %s", out)
	}
	status := tracker.GetStatus()
	if status.Models[tools.DefaultVisionModel].Calls != 1 {
		t.Fatalf("budget models = %#v, want one call for %q", status.Models, tools.DefaultVisionModel)
	}
}

func TestDispatchThreeDPrinterShowLiveStreamEmitsInlineStream(t *testing.T) {
	wsURL, closeWS := mockAgentThreeDPrinterCameraURLServer(t, "http://127.0.0.1:8080/video")
	defer closeWS()
	cfg := agentThreeDPrinterConfig(t, wsURL)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	broker := &captureBroker{}
	out, ok := dispatchPlatform(context.Background(), ToolCall{
		Action: "three_d_printer",
		Params: map[string]interface{}{
			"operation":  "show_live_stream",
			"printer_id": "lab",
		},
	}, &DispatchContext{Cfg: cfg, Logger: logger, Broker: broker})
	if !ok {
		t.Fatal("expected dispatchPlatform to handle three_d_printer")
	}
	if !strings.Contains(out, `"proxy_url"`) {
		t.Fatalf("expected stream URL payload, got: %s", out)
	}
	if len(broker.events) != 1 || broker.events[0].event != "live_stream" {
		t.Fatalf("broker events = %#v, want one live_stream event", broker.events)
	}
	if !strings.Contains(broker.events[0].message, `"/api/3d-printers/lab/camera/stream"`) {
		t.Fatalf("live_stream payload = %s, want proxied same-origin stream", broker.events[0].message)
	}
}

func TestDispatchCoAgentStopRequiresExplicitUserCancelRequest(t *testing.T) {
	cfg := &config.Config{}
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.MaxConcurrent = 1
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewCoAgentRegistry(1, logger)
	id, _, err := registry.RegisterWithPriority("specialist-writer", "Write a story", func() {}, 2)
	if err != nil {
		t.Fatalf("RegisterWithPriority: %v", err)
	}

	out, ok := dispatchPlatform(context.Background(), ToolCall{
		Action:    "co_agent",
		Operation: "stop",
		CoAgentID: id,
	}, &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		CoAgentRegistry: registry,
		UserContext:     "lasse den autor co agent eine scifi kurzgeschichte erstellen",
	})
	if !ok {
		t.Fatal("expected dispatchPlatform to handle co_agent stop")
	}
	if !strings.Contains(out, `"status":"blocked"`) {
		t.Fatalf("expected blocked stop without explicit user request, got: %s", out)
	}

	status, err := registry.GetStatus(id)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status["state"] != string(CoAgentRunning) {
		t.Fatalf("co-agent state = %v, want running after blocked stop", status["state"])
	}
}

func TestDispatchToolCallPropagatesUserContextToCoAgentStopGuard(t *testing.T) {
	cfg := &config.Config{}
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.MaxConcurrent = 1
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewCoAgentRegistry(1, logger)
	id, _, err := registry.RegisterWithPriority("specialist-writer", "Write a story", func() {}, 2)
	if err != nil {
		t.Fatalf("RegisterWithPriority: %v", err)
	}

	tc := ToolCall{
		Action:    "co_agent",
		Operation: "stop",
		CoAgentID: id,
	}
	out := DispatchToolCall(context.Background(), &tc, &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		CoAgentRegistry: registry,
	}, "lasse den autor co agent eine scifi kurzgeschichte erstellen")
	if !strings.Contains(out, `"status":"blocked"`) {
		t.Fatalf("expected DispatchToolCall to propagate user context and block stop, got: %s", out)
	}
	status, err := registry.GetStatus(id)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status["state"] != string(CoAgentRunning) {
		t.Fatalf("co-agent state = %v, want running after blocked stop", status["state"])
	}
}

func TestDispatchCoAgentGetResultWaitsForCompletion(t *testing.T) {
	cfg := &config.Config{}
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.MaxConcurrent = 1
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewCoAgentRegistry(1, logger)
	id, _, err := registry.RegisterWithPriority("specialist-writer", "Write a story", func() {}, 2)
	if err != nil {
		t.Fatalf("RegisterWithPriority: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		registry.Complete(id, "A compact story.", 12, 0)
	}()

	out, ok := dispatchPlatform(context.Background(), ToolCall{
		Action:    "co_agent",
		Operation: "get_result",
		CoAgentID: id,
	}, &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		CoAgentRegistry: registry,
	})
	if !ok {
		t.Fatal("expected dispatchPlatform to handle co_agent get_result")
	}
	if !strings.Contains(out, `"state":"completed"`) {
		t.Fatalf("expected completed status, got: %s", out)
	}
	if !strings.Contains(out, `"result":"A compact story."`) {
		t.Fatalf("expected result payload, got: %s", out)
	}
}

func TestDispatchCoAgentStopAllowsExplicitUserCancelRequest(t *testing.T) {
	cfg := &config.Config{}
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.MaxConcurrent = 1
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewCoAgentRegistry(1, logger)
	id, _, err := registry.RegisterWithPriority("specialist-writer", "Write a story", func() {}, 2)
	if err != nil {
		t.Fatalf("RegisterWithPriority: %v", err)
	}

	out, ok := dispatchPlatform(context.Background(), ToolCall{
		Action:    "co_agent",
		Operation: "stop",
		CoAgentID: id,
	}, &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		CoAgentRegistry: registry,
		UserContext:     "Bitte stoppe den Writer-Co-Agent jetzt.",
	})
	if !ok {
		t.Fatal("expected dispatchPlatform to handle co_agent stop")
	}
	if !strings.Contains(out, `"status": "ok"`) {
		t.Fatalf("expected explicit user stop to succeed, got: %s", out)
	}

	status, err := registry.GetStatus(id)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status["state"] != string(CoAgentCancelled) {
		t.Fatalf("co-agent state = %v, want cancelled after explicit stop", status["state"])
	}
}

func TestDispatchCoAgentStopAllRequiresExplicitUserCancelRequest(t *testing.T) {
	cfg := &config.Config{}
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.MaxConcurrent = 2
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := NewCoAgentRegistry(2, logger)
	id, _, err := registry.RegisterWithPriority("specialist-writer", "Write a story", func() {}, 2)
	if err != nil {
		t.Fatalf("RegisterWithPriority: %v", err)
	}

	out, ok := dispatchPlatform(context.Background(), ToolCall{
		Action:    "co_agent",
		Operation: "stop_all",
	}, &DispatchContext{
		Cfg:             cfg,
		Logger:          logger,
		CoAgentRegistry: registry,
		UserContext:     "schreib die Geschichte mit dem Writer-Co-Agent",
	})
	if !ok {
		t.Fatal("expected dispatchPlatform to handle co_agent stop_all")
	}
	if !strings.Contains(out, `"status":"blocked"`) {
		t.Fatalf("expected blocked stop_all without explicit user request, got: %s", out)
	}

	status, err := registry.GetStatus(id)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if status["state"] != string(CoAgentRunning) {
		t.Fatalf("co-agent state = %v, want running after blocked stop_all", status["state"])
	}
}

func TestCoAgentStopIntentDetection(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "Bitte stoppe den Writer-Co-Agent.", want: true},
		{text: "brich alle co agents ab", want: true},
		{text: "cancel the running co-agent", want: true},
		{text: "lasse den autor co agent eine scifi kurzgeschichte erstellen", want: false},
		{text: "der co-agent stoppt hoffentlich nicht", want: false},
		{text: "ist der co-agent schon gestoppt?", want: false},
	}

	for _, tt := range cases {
		if got := userExplicitlyRequestedCoAgentStop(tt.text); got != tt.want {
			t.Fatalf("userExplicitlyRequestedCoAgentStop(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func agentThreeDPrinterConfig(t *testing.T, wsURL string) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	cfg.ThreeDPrinters.Enabled = true
	cfg.ThreeDPrinters.DefaultPrinter = "lab"
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Enabled = true
	cfg.ThreeDPrinters.ElegooCentauriCarbon.Printers = []config.ElegooCentauriCarbonPrinterConfig{{
		ID:             "lab",
		URL:            wsURL,
		TimeoutSeconds: 2,
	}}
	return cfg
}

func mockAgentThreeDPrinterCameraURLServer(t *testing.T, cameraURL string) (string, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade error = %v", err)
		}
		defer conn.Close()
		var payload map[string]interface{}
		if err := conn.ReadJSON(&payload); err != nil {
			t.Fatalf("ReadJSON error = %v", err)
		}
		data := payload["Data"].(map[string]interface{})
		requestID := data["RequestID"].(string)
		if err := conn.WriteJSON(map[string]interface{}{
			"Data": map[string]interface{}{
				"RequestID": requestID,
				"Data":      map[string]interface{}{"Url": cameraURL},
			},
		}); err != nil {
			t.Fatalf("WriteJSON error = %v", err)
		}
	}))
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/websocket", server.Close
}
