package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchServicesAnalyzeImageFallsBackWhenPreferredMCPVisionFails(t *testing.T) {
	originalPreferred := dispatchPreferredMCPVision
	originalAnalyze := dispatchAnalyzeImageWithPrompt
	defer func() {
		dispatchPreferredMCPVision = originalPreferred
		dispatchAnalyzeImageWithPrompt = originalAnalyze
	}()

	dispatchPreferredMCPVision = func(cfg *config.Config, filePath, prompt string, logger *slog.Logger) (string, bool, error) {
		if filePath != "attachments/example.png" {
			t.Fatalf("filePath = %q, want attachments/example.png", filePath)
		}
		if prompt != "What is in this image?" {
			t.Fatalf("prompt = %q, want custom prompt", prompt)
		}
		return "", true, assertiveTestError("schema mismatch")
	}

	dispatchAnalyzeImageWithPrompt = func(filePath, prompt string, cfg *config.Config) (string, int, int, error) {
		if filePath != "attachments/example.png" {
			t.Fatalf("filePath = %q, want attachments/example.png", filePath)
		}
		if prompt != "What is in this image?" {
			t.Fatalf("prompt = %q, want custom prompt", prompt)
		}
		return `{"status":"success","analysis":"native fallback worked"}`, 0, 0, nil
	}

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action: "analyze_image",
		Params: map[string]interface{}{
			"file_path": "attachments/example.png",
			"prompt":    "What is in this image?",
		},
	}, &DispatchContext{
		Cfg:    &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchServices to handle analyze_image")
	}
	if !strings.Contains(out, `"analysis":"native fallback worked"`) {
		t.Fatalf("output = %q, want native fallback result", out)
	}
	if strings.Contains(out, "Preferred MCP vision failed") {
		t.Fatalf("output = %q, should not return preferred MCP failure", out)
	}
}

func TestDispatchServicesAnalyzeImageUsesPublicURLWithoutLoggingIt(t *testing.T) {
	originalPreferred := dispatchPreferredMCPVision
	originalAnalyzeURL := dispatchAnalyzeImageURLWithPrompt
	defer func() {
		dispatchPreferredMCPVision = originalPreferred
		dispatchAnalyzeImageURLWithPrompt = originalAnalyzeURL
	}()

	dispatchPreferredMCPVision = func(cfg *config.Config, filePath, prompt string, logger *slog.Logger) (string, bool, error) {
		t.Fatal("preferred local-file vision must not be called for image_url")
		return "", false, nil
	}
	dispatchAnalyzeImageURLWithPrompt = func(imageURL, prompt string, cfg *config.Config) (string, int, int, error) {
		if imageURL != "https://cdn.example.test/image.png?signature=secret" {
			t.Fatalf("imageURL = %q", imageURL)
		}
		return `{"status":"success","analysis":"URL worked"}`, 0, 0, nil
	}

	var logs strings.Builder
	out, ok := dispatchServices(context.Background(), ToolCall{
		Action: "analyze_image",
		Params: map[string]interface{}{
			"image_url": "https://cdn.example.test/image.png?signature=secret",
		},
	}, &DispatchContext{
		Cfg:    &config.Config{},
		Logger: slog.New(slog.NewTextHandler(&logs, nil)),
	})
	if !ok || !strings.Contains(out, `"analysis":"URL worked"`) {
		t.Fatalf("output = %q, handled = %v", out, ok)
	}
	if strings.Contains(logs.String(), "signature=secret") {
		t.Fatalf("logs leaked signed image URL: %s", logs.String())
	}
}

func TestDispatchServicesAnalyzeImageRequiresExactlyOneSource(t *testing.T) {
	for _, params := range []map[string]interface{}{
		{},
		{"file_path": "image.png", "image_url": "https://cdn.example.test/image.png"},
	} {
		out, ok := dispatchServices(context.Background(), ToolCall{
			Action: "analyze_image",
			Params: params,
		}, &DispatchContext{
			Cfg:    &config.Config{},
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
		if !ok || !strings.Contains(out, "exactly one") {
			t.Fatalf("output = %q, handled = %v", out, ok)
		}
	}
}

type assertiveTestError string

func (e assertiveTestError) Error() string {
	return string(e)
}
