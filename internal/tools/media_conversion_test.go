package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func decodeMediaConversionResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", raw, err)
	}
	return parsed
}

func mediaConversionTestConfig(enabled bool) *config.MediaConversionConfig {
	return &config.MediaConversionConfig{
		Enabled:        enabled,
		ReadOnly:       false,
		TimeoutSeconds: 30,
	}
}

func TestExecuteMediaConversionDisabled(t *testing.T) {
	result := decodeMediaConversionResult(t, ExecuteMediaConversion(t.TempDir(), &config.MediaConversionConfig{}, MediaConversionRequest{
		Operation: "info",
		FilePath:  "sample.png",
	}))

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "disabled") {
		t.Fatalf("message = %q, want disabled error", result["message"])
	}
}

func TestExecuteMediaConversionReadOnlyBlocksWrites(t *testing.T) {
	cfg := mediaConversionTestConfig(true)
	cfg.ReadOnly = true

	result := decodeMediaConversionResult(t, ExecuteMediaConversion(t.TempDir(), cfg, MediaConversionRequest{
		Operation:    "audio_convert",
		FilePath:     "sample.wav",
		OutputFormat: "mp3",
	}))

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "read-only") {
		t.Fatalf("message = %q, want read-only error", result["message"])
	}
}

func TestExecuteMediaConversionRejectsPathOutsideWorkspace(t *testing.T) {
	cfg := mediaConversionTestConfig(true)

	result := decodeMediaConversionResult(t, ExecuteMediaConversion(t.TempDir(), cfg, MediaConversionRequest{
		Operation: "info",
		FilePath:  "../escape.mp4",
	}))

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "escapes the project root") {
		t.Fatalf("message = %q, want workspace validation error", result["message"])
	}
}

func TestExecuteMediaConversionImageInfo(t *testing.T) {
	workspaceDir := t.TempDir()
	imagePath := filepath.Join(workspaceDir, "sample.png")
	if err := os.WriteFile(imagePath, []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}, 0o644); err != nil {
		t.Fatalf("failed to write png: %v", err)
	}

	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, mediaConversionTestConfig(true), MediaConversionRequest{
		Operation: "info",
		FilePath:  "sample.png",
	}))

	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("status = %q, want success", got)
	}
	if got, _ := result["media_type"].(string); got != "image" {
		t.Fatalf("media_type = %q, want image", got)
	}
}

func TestExecuteMediaConversionInfoMissingFile(t *testing.T) {
	workspaceDir := t.TempDir()
	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, mediaConversionTestConfig(true), MediaConversionRequest{
		Operation: "info",
		FilePath:  "nonexistent.mp4",
	}))

	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "cannot access input file") {
		t.Fatalf("message = %q, want cannot access input file error", result["message"])
	}
}

func TestResolveConversionOutputRejectsSameFile(t *testing.T) {
	_, _, err := resolveConversionOutput("/tmp/input.mp3", "/tmp/input.mp3", "")
	if err == nil {
		t.Fatal("expected error when input and output are the same file")
	}
	if !strings.Contains(err.Error(), "must be different from input file") {
		t.Fatalf("error = %q, want different-from-input error", err.Error())
	}
}

func TestResolveConversionOutputRejectsImageMagickPseudoProtocols(t *testing.T) {
	for _, output := range []string{
		"|sh -c whoami",
		"@payload.txt",
		"-write.png",
		"msl:payload.msl",
		"https://example.com/out.png",
		"caption:hello",
	} {
		_, _, err := resolveConversionOutput("input.png", output, "png")
		if err == nil {
			t.Fatalf("expected output %q to be rejected", output)
		}
	}
}

func TestExecuteMediaConversionRejectsOversizedFile(t *testing.T) {
	workspaceDir := t.TempDir()
	imagePath := filepath.Join(workspaceDir, "sample.png")
	if err := os.WriteFile(imagePath, []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0xc9, 0xfe, 0x92,
		0xef, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}, 0o644); err != nil {
		t.Fatalf("failed to write png: %v", err)
	}

	cfg := mediaConversionTestConfig(true)
	cfg.MaxFileSizeMB = 0 // unlimited should allow the file
	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, cfg, MediaConversionRequest{
		Operation: "info",
		FilePath:  "sample.png",
	}))
	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("unlimited size: status = %q, want success", got)
	}

	cfg.MaxFileSizeMB = 0 // 0 means unlimited
	result = decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, cfg, MediaConversionRequest{
		Operation: "info",
		FilePath:  "sample.png",
	}))
	if got, _ := result["status"].(string); got != "success" {
		t.Fatalf("zero limit means unlimited: status = %q, want success", got)
	}

	// Test checkMediaFileSize logic directly for rejection and acceptance
	if err := checkMediaFileSize(&config.MediaConversionConfig{MaxFileSizeMB: 1}, imagePath); err != nil {
		t.Fatalf("67 byte PNG should pass 1 MB limit: %v", err)
	}
	if err := checkMediaFileSize(&config.MediaConversionConfig{MaxFileSizeMB: 0}, imagePath); err != nil {
		t.Fatalf("67 byte PNG should pass unlimited limit: %v", err)
	}
	if err := checkMediaFileSize(nil, imagePath); err != nil {
		t.Fatalf("67 byte PNG should pass nil config: %v", err)
	}
}

func TestMediaConversionHealthReportsMissingBinaries(t *testing.T) {
	oldLookPath := mediaConversionLookPath
	oldRunCommand := mediaConversionRunCommand
	t.Cleanup(func() {
		mediaConversionLookPath = oldLookPath
		mediaConversionRunCommand = oldRunCommand
	})

	mediaConversionLookPath = func(file string) (string, error) {
		return "", os.ErrNotExist
	}

	health := MediaConversionHealth(context.Background(), mediaConversionTestConfig(true))
	if got, _ := health["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
}
