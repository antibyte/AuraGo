package agent

import (
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestImageGenerationToolResultIncludesLocalPath(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), "data")

	payload := imageGenerationToolResultPayload(cfg, &tools.ImageGenResult{
		Filename:   "img_test.jpeg",
		WebPath:    "/files/generated_images/img_test.jpeg",
		Markdown:   "![Generated Image](/files/generated_images/img_test.jpeg)",
		Model:      "demo-model",
		Provider:   "demo-provider",
		Size:       "1024x1024",
		DurationMs: 42,
	}, "cat in car", "funny cat in car")

	wantLocalPath := filepath.Join(cfg.Directories.DataDir, "generated_images", "img_test.jpeg")
	if payload["local_path"] != wantLocalPath {
		t.Fatalf("local_path = %#v, want %q", payload["local_path"], wantLocalPath)
	}
	if payload["web_path"] != "/files/generated_images/img_test.jpeg" {
		t.Fatalf("web_path = %#v", payload["web_path"])
	}
}
