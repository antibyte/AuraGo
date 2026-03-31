package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestDispatchExecListToolsClarifiesBuiltinSkills(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := tools.NewManifest(filepath.Join(tmpDir, "tools"))
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	out := dispatchExec(
		context.Background(),
		ToolCall{Action: "list_tools"},
		&DispatchContext{
			Cfg:      &config.Config{},
			Logger:   logger,
			Manifest: manifest,
		},
	)

	for _, snippet := range []string{
		"list_tools' ONLY lists custom reusable Python tools",
		"virustotal_scan",
		"list_skills",
		"Do NOT assume an integration is unavailable",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("expected list_tools output to contain %q, got:\n%s", snippet, out)
		}
	}
}
