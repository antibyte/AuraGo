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

// mediaConversionArgValue returns the value that follows the given flag.
func mediaConversionArgValue(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
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

func TestExecuteMediaConversionStagesExternalProcessIO(t *testing.T) {
	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceDir, "input.wav"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldLookPath, oldRunCommand := mediaConversionLookPath, mediaConversionRunCommand
	t.Cleanup(func() {
		mediaConversionLookPath, mediaConversionRunCommand = oldLookPath, oldRunCommand
	})
	mediaConversionLookPath = func(string) (string, error) { return "ffmpeg", nil }
	mediaConversionRunCommand = func(_ context.Context, _ string, args []string) ([]byte, error) {
		input := mediaConversionArgValue(args, "-i")
		output := args[len(args)-1]
		if input == "" {
			t.Fatalf("converter received no input argument: %v", args)
		}
		if strings.HasPrefix(input, workspaceDir) || strings.HasPrefix(output, workspaceDir) {
			t.Fatalf("converter received a workspace path: %q %q", input, output)
		}
		data, err := os.ReadFile(input)
		if err != nil || string(data) != "source" {
			t.Fatalf("staged input: %q, %v", data, err)
		}
		return nil, os.WriteFile(output, []byte("converted"), 0o600)
	}
	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, mediaConversionTestConfig(true), MediaConversionRequest{
		Operation: "audio_convert", FilePath: "input.wav", OutputFormat: "mp3",
	}))
	if result["status"] != "success" || result["file"] != filepath.Join(workspaceDir, "input.mp3") {
		t.Fatalf("conversion result: %#v", result)
	}
	data, err := os.ReadFile(filepath.Join(workspaceDir, "input.mp3"))
	if err != nil || string(data) != "converted" {
		t.Fatalf("saved output: %q, %v", data, err)
	}
}

func TestExecuteMediaConversionAddsProtocolWhitelist(t *testing.T) {
	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceDir, "input.wav"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldLookPath, oldRunCommand := mediaConversionLookPath, mediaConversionRunCommand
	t.Cleanup(func() {
		mediaConversionLookPath, mediaConversionRunCommand = oldLookPath, oldRunCommand
	})
	mediaConversionLookPath = func(string) (string, error) { return "ffmpeg", nil }
	var captured []string
	mediaConversionRunCommand = func(_ context.Context, _ string, args []string) ([]byte, error) {
		captured = args
		return nil, os.WriteFile(args[len(args)-1], []byte("converted"), 0o600)
	}

	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, mediaConversionTestConfig(true), MediaConversionRequest{
		Operation: "audio_convert", FilePath: "input.wav", OutputFormat: "mp3",
	}))
	if result["status"] != "success" {
		t.Fatalf("conversion result: %#v", result)
	}
	if got := mediaConversionArgValue(captured, "-protocol_whitelist"); got != mediaConversionProtocols {
		t.Fatalf("-protocol_whitelist = %q, want %q (args: %v)", got, mediaConversionProtocols, captured)
	}
	if got := mediaConversionArgValue(captured, "-format_whitelist"); got != "wav" {
		t.Fatalf("-format_whitelist = %q, want wav (args: %v)", got, captured)
	}
	for _, flag := range []string{"-nostdin", "-hide_banner", "-loglevel"} {
		if !slicesContains(captured, flag) {
			t.Fatalf("converter arguments miss %q: %v", flag, captured)
		}
	}
}

func TestExecuteMediaConversionAudioDropsVideoStream(t *testing.T) {
	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceDir, "input.mp4"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldLookPath, oldRunCommand := mediaConversionLookPath, mediaConversionRunCommand
	t.Cleanup(func() {
		mediaConversionLookPath, mediaConversionRunCommand = oldLookPath, oldRunCommand
	})
	mediaConversionLookPath = func(string) (string, error) { return "ffmpeg", nil }
	var captured []string
	mediaConversionRunCommand = func(_ context.Context, _ string, args []string) ([]byte, error) {
		captured = args
		return nil, os.WriteFile(args[len(args)-1], []byte("converted"), 0o600)
	}

	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, mediaConversionTestConfig(true), MediaConversionRequest{
		Operation: "audio_convert", FilePath: "input.mp4", OutputFormat: "mp3",
	}))
	if result["status"] != "success" {
		t.Fatalf("conversion result: %#v", result)
	}
	if !slicesContains(captured, "-vn") {
		t.Fatalf("audio conversion must drop the video stream: %v", captured)
	}
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func TestResolveConversionTargetRejectsSameFile(t *testing.T) {
	_, err := resolveConversionTarget(t.TempDir(), "/tmp/input.mp3", "/tmp/input.mp3", "", "audio")
	if err == nil {
		t.Fatal("expected error when input and output are the same file")
	}
	if !strings.Contains(err.Error(), "must be different from input file") {
		t.Fatalf("error = %q, want different-from-input error", err.Error())
	}
}

func TestResolveConversionTargetRejectsImageMagickPseudoProtocols(t *testing.T) {
	for _, output := range []string{
		"|sh -c whoami",
		"@payload.txt",
		"-write.png",
		"msl:payload.msl",
		"https://example.com/out.png",
		"caption:hello",
	} {
		_, err := resolveConversionTarget(t.TempDir(), "input.png", output, "png", "image")
		if err == nil {
			t.Fatalf("expected output %q to be rejected", output)
		}
	}
}

func TestResolveConversionTargetDerivesStagedExtensionFromFormat(t *testing.T) {
	staging := t.TempDir()
	target, err := resolveConversionTarget(staging, "/ws/input.wav", "out", "mp3", "audio")
	if err != nil {
		t.Fatalf("resolveConversionTarget: %v", err)
	}
	if filepath.Ext(target.StagedPath) != ".mp3" {
		t.Fatalf("staged path = %q, want a .mp3 suffix", target.StagedPath)
	}
	if filepath.Base(target.Destination) != "out.mp3" {
		t.Fatalf("destination = %q, want out.mp3", target.Destination)
	}
	if target.Format != "mp3" {
		t.Fatalf("format = %q, want mp3", target.Format)
	}
}

func TestResolveConversionTargetRejectsFormatConflict(t *testing.T) {
	_, err := resolveConversionTarget(t.TempDir(), "/ws/in.mp4", "out.mp4", "mp3", "audio")
	if err == nil {
		t.Fatal("expected a conflict error")
	}
	if !strings.Contains(err.Error(), "out.mp4") || !strings.Contains(err.Error(), "mp3") {
		t.Fatalf("error must name both values, got %q", err)
	}
}

func TestResolveConversionTargetRejectsUnsupportedFormat(t *testing.T) {
	_, err := resolveConversionTarget(t.TempDir(), "/ws/in.wav", "", "txt", "audio")
	if err == nil {
		t.Fatal("expected an allowlist error")
	}
	if !strings.Contains(err.Error(), "mp3") {
		t.Fatalf("error must list the supported formats, got %q", err)
	}
}

func TestResolveConversionTargetDerivesFormatFromOutputFile(t *testing.T) {
	target, err := resolveConversionTarget(t.TempDir(), "/ws/in.wav", "song.mp3", "", "audio")
	if err != nil {
		t.Fatalf("resolveConversionTarget: %v", err)
	}
	if target.Format != "mp3" {
		t.Fatalf("format = %q, want mp3", target.Format)
	}
	if filepath.Ext(target.StagedPath) != ".mp3" {
		t.Fatalf("staged path = %q, want a .mp3 suffix", target.StagedPath)
	}
}

func TestExecuteMediaConversionOutputWithoutExtension(t *testing.T) {
	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceDir, "input.wav"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldLookPath, oldRunCommand := mediaConversionLookPath, mediaConversionRunCommand
	t.Cleanup(func() {
		mediaConversionLookPath, mediaConversionRunCommand = oldLookPath, oldRunCommand
	})
	mediaConversionLookPath = func(string) (string, error) { return "ffmpeg", nil }
	var stagedOutput string
	mediaConversionRunCommand = func(_ context.Context, _ string, args []string) ([]byte, error) {
		stagedOutput = args[len(args)-1]
		if filepath.Ext(stagedOutput) == "" {
			t.Fatalf("converter received an output without an extension: %v", args)
		}
		return nil, os.WriteFile(stagedOutput, []byte("converted"), 0o600)
	}

	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, mediaConversionTestConfig(true), MediaConversionRequest{
		Operation: "audio_convert", FilePath: "input.wav", OutputFile: "out", OutputFormat: "mp3",
	}))
	if result["status"] != "success" {
		t.Fatalf("conversion result: %#v", result)
	}
	if result["file"] != filepath.Join(workspaceDir, "out.mp3") {
		t.Fatalf("file = %v, want out.mp3", result["file"])
	}
	if got, _ := result["output_format"].(string); got != "mp3" {
		t.Fatalf("output_format = %q, want mp3", got)
	}
}

func TestExecuteMediaConversionRejectsFormatConflict(t *testing.T) {
	workspaceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceDir, "input.wav"), []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, mediaConversionTestConfig(true), MediaConversionRequest{
		Operation: "audio_convert", FilePath: "input.wav", OutputFile: "out.mp4", OutputFormat: "mp3",
	}))
	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "conflicts") {
		t.Fatalf("message = %q, want a conflict error", result["message"])
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

	// A file above the cap must be rejected with the limit in the message.
	oversized := filepath.Join(workspaceDir, "big.png")
	big := make([]byte, 2*1024*1024)
	if err := os.WriteFile(oversized, big, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.MaxFileSizeMB = 1
	result = decodeMediaConversionResult(t, ExecuteMediaConversion(workspaceDir, cfg, MediaConversionRequest{
		Operation: "info",
		FilePath:  "big.png",
	}))
	if got, _ := result["status"].(string); got != "error" {
		t.Fatalf("oversized file: status = %q, want error", got)
	}
	if !strings.Contains(result["message"].(string), "1 MB") {
		t.Fatalf("message = %q, want the size limit", result["message"])
	}

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
