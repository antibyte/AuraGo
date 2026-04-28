package agent

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestHandleSendImageAcceptsGeneratedImageWebPath(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	workspaceDir := filepath.Join(tmp, "workspace")
	imageDir := filepath.Join(dataDir, "generated_images")
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		t.Fatalf("mkdir generated image dir: %v", err)
	}
	imagePath := filepath.Join(imageDir, "img_test.png")
	if err := os.WriteFile(imagePath, []byte("fake png"), 0644); err != nil {
		t.Fatalf("write generated image: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.DataDir = dataDir
	cfg.Directories.WorkspaceDir = workspaceDir

	raw := handleSendImage(sendMediaArgs{
		Path:    "/files/generated_images/img_test.png",
		Caption: "Generated mood",
	}, cfg, slog.Default())
	raw = stringsTrimToolOutput(raw)

	var result struct {
		Status    string `json:"status"`
		WebPath   string `json:"web_path"`
		LocalPath string `json:"local_path"`
		Markdown  string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, raw)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success; raw=%s", result.Status, raw)
	}
	if result.WebPath != "/files/generated_images/img_test.png" {
		t.Fatalf("web_path = %q, want generated image web path", result.WebPath)
	}
	if result.LocalPath != imagePath {
		t.Fatalf("local_path = %q, want %q", result.LocalPath, imagePath)
	}
	if result.Markdown != "![Generated mood](/files/generated_images/img_test.png)" {
		t.Fatalf("markdown = %q", result.Markdown)
	}
}
