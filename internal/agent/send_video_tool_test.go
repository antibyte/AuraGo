package agent

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestHandleSendVideoCopiesLocalVideoForChat(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	workspaceDir := filepath.Join(tmp, "workspace")
	videoSrcDir := filepath.Join(workspaceDir, "clips")
	if err := os.MkdirAll(videoSrcDir, 0755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	sourcePath := filepath.Join(videoSrcDir, "demo.mp4")
	if err := os.WriteFile(sourcePath, []byte("fake mp4"), 0644); err != nil {
		t.Fatalf("write source video: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = workspaceDir

	raw := handleSendVideo(sendMediaArgs{Path: "clips/demo.mp4", Title: "Demo clip"}, cfg, slog.Default(), nil)
	raw = stringsTrimToolOutput(raw)

	var result struct {
		Status    string `json:"status"`
		WebPath   string `json:"web_path"`
		LocalPath string `json:"local_path"`
		Title     string `json:"title"`
		MimeType  string `json:"mime_type"`
		Filename  string `json:"filename"`
		Format    string `json:"format"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, raw)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success", result.Status)
	}
	if result.Title != "Demo clip" || result.MimeType != "video/mp4" || result.Format != "mp4" {
		t.Fatalf("unexpected metadata: %+v", result)
	}
	if result.WebPath == "" || result.Filename == "" {
		t.Fatalf("expected web path and filename: %+v", result)
	}
	if filepath.Dir(result.LocalPath) != filepath.Join(dataDir, "generated_videos") {
		t.Fatalf("local path dir = %q, want generated_videos", filepath.Dir(result.LocalPath))
	}
	if _, err := os.Stat(result.LocalPath); err != nil {
		t.Fatalf("expected copied video at %s: %v", result.LocalPath, err)
	}
}

func TestHandleSendVideoAcceptsGeneratedVideoWebPath(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	workspaceDir := filepath.Join(tmp, "workspace")
	videoDir := filepath.Join(dataDir, "generated_videos")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		t.Fatalf("mkdir generated video dir: %v", err)
	}
	videoPath := filepath.Join(videoDir, "video_test.mp4")
	if err := os.WriteFile(videoPath, []byte("fake mp4"), 0644); err != nil {
		t.Fatalf("write generated video: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = workspaceDir

	raw := handleSendVideo(sendMediaArgs{Path: "/files/generated_videos/video_test.mp4", Title: "Generated clip"}, cfg, slog.Default(), nil)
	raw = stringsTrimToolOutput(raw)

	var result struct {
		Status    string `json:"status"`
		WebPath   string `json:"web_path"`
		LocalPath string `json:"local_path"`
		Title     string `json:"title"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, raw)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success; raw=%s", result.Status, raw)
	}
	if result.WebPath != "/files/generated_videos/video_test.mp4" {
		t.Fatalf("web_path = %q, want generated video web path", result.WebPath)
	}
	if result.LocalPath != videoPath {
		t.Fatalf("local_path = %q, want %q", result.LocalPath, videoPath)
	}
	if result.Title != "Generated clip" {
		t.Fatalf("title = %q, want Generated clip", result.Title)
	}
}

func TestEmitMediaSSEEventsSendsGeneratedVideoEvent(t *testing.T) {
	broker := &captureBroker{}
	emitMediaSSEEvents(broker, "generate_video", `Tool Output: {"status":"ok","web_path":"/files/generated_videos/video_123.mp4","filename":"video_123.mp4","format":"mp4","provider":"minimax","model":"MiniMax-Hailuo-2.3"}`, t.TempDir())

	if len(broker.events) != 1 {
		t.Fatalf("events = %d, want 1", len(broker.events))
	}
	if broker.events[0].event != "video" {
		t.Fatalf("event = %q, want video", broker.events[0].event)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(broker.events[0].message), &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if payload["path"] != "/files/generated_videos/video_123.mp4" || payload["mime_type"] != "video/mp4" {
		t.Fatalf("unexpected video payload: %+v", payload)
	}
}

func TestEmitMediaSSEEventsSendsManualVideoEvent(t *testing.T) {
	broker := &captureBroker{}
	emitMediaSSEEvents(broker, "send_video", `Tool Output: {"status":"success","web_path":"/files/generated_videos/manual.webm","title":"Manual","mime_type":"video/webm","filename":"manual.webm","format":"webm"}`, t.TempDir())

	if len(broker.events) != 1 || broker.events[0].event != "video" {
		t.Fatalf("events = %+v, want one video event", broker.events)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(broker.events[0].message), &payload); err != nil {
		t.Fatalf("unmarshal event payload: %v", err)
	}
	if payload["title"] != "Manual" || payload["mime_type"] != "video/webm" {
		t.Fatalf("unexpected video payload: %+v", payload)
	}
}

type captureBroker struct {
	events []capturedEvent
}

type capturedEvent struct {
	event   string
	message string
}

func (b *captureBroker) Send(event, message string) {
	b.events = append(b.events, capturedEvent{event: event, message: message})
}

func (b *captureBroker) SendJSON(string) {}

func (b *captureBroker) SendLLMStreamDelta(string, string, string, int, string) {}

func (b *captureBroker) SendLLMStreamDone(string) {}

func (b *captureBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {}

func (b *captureBroker) SendThinkingBlock(string, string, string) {}

func stringsTrimToolOutput(raw string) string {
	return strings.TrimPrefix(raw, "Tool Output: ")
}

var _ FeedbackBroker = (*captureBroker)(nil)
