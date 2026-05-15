package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchServicesHomepageExecRejectsEmptyCommandWithGuidance(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.AllowContainerManagement = true

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:    "homepage",
		Operation: "exec",
		Params: map[string]interface{}{
			"command": "   ",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchServices to handle homepage")
	}
	for _, want := range []string{"command is required", "Do not retry", "list_files", "build"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected homepage exec empty-command guidance to contain %q, got:\n%s", want, out)
		}
	}
}
