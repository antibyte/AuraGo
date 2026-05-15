package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchShellRejectsHomepageWorkspacePath(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Directories.WorkspaceDir = t.TempDir()

	out := dispatchShell(ToolCall{
		Action: "execute_shell",
		Params: map[string]interface{}{
			"command": "cd /workspace/ordo-fragmentis && npm run build",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, want := range []string{"homepage container workspace", "homepage", "project_dir"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected homepage workspace guidance to contain %q, got:\n%s", want, out)
		}
	}
}
