package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchCloudHandlesDisabledHuggingFace(t *testing.T) {
	out, handled := dispatchCloud(context.Background(), ToolCall{
		Action:    "huggingface",
		Operation: "search_models",
		Query:     "bert",
	}, &DispatchContext{Cfg: &config.Config{}, Logger: slog.Default()})

	if !handled {
		t.Fatal("expected huggingface action to be handled")
	}
	if !strings.Contains(out, "Hugging Face integration is not enabled") {
		t.Fatalf("output = %s", out)
	}
}

func TestDispatchCloudBlocksHuggingFaceWriteInReadOnly(t *testing.T) {
	out, handled := dispatchCloud(context.Background(), ToolCall{
		Action:    "huggingface",
		Operation: "create_repo",
		Name:      "owner/repo",
	}, &DispatchContext{
		Cfg: &config.Config{HuggingFace: config.HuggingFaceConfig{
			Enabled:     true,
			ReadOnly:    true,
			AllowWrites: true,
		}},
		Logger: slog.Default(),
	})

	if !handled {
		t.Fatal("expected huggingface action to be handled")
	}
	if !strings.Contains(out, "read-only") {
		t.Fatalf("output = %s", out)
	}
}
