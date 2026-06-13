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

func TestDispatchShellRejectsEmptyCommand(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Directories.WorkspaceDir = t.TempDir()

	out := dispatchShell(ToolCall{
		Action: "execute_shell",
		Params: map[string]interface{}{
			"command": "   ",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, want := range []string{"[EXECUTION ERROR]", "'command' is required", "execute_shell"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected empty command guidance to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDispatchShellRejectsVirtualDesktopWorkspacePathInDesktopChat(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Directories.WorkspaceDir = t.TempDir()

	out := dispatchShell(ToolCall{
		Action: "execute_shell",
		Params: map[string]interface{}{
			"command": "wc -l /home/aurago/aurago/agent_workspace/virtual_desktop/Apps/space-invaders/game.js",
		},
	}, &DispatchContext{
		Cfg:           cfg,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		MessageSource: "virtual_desktop_chat",
	})

	for _, want := range []string{"virtual desktop workspace", "virtual_desktop", "read_file"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected virtual desktop guidance to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDispatchShellRejectsRelativeVirtualDesktopPathInDesktopChat(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Directories.WorkspaceDir = t.TempDir()

	out := dispatchShell(ToolCall{
		Action: "execute_shell",
		Params: map[string]interface{}{
			"command": "cat Apps/space-invaders/game.js",
		},
	}, &DispatchContext{
		Cfg:           cfg,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		MessageSource: "virtual_desktop_chat",
	})

	for _, want := range []string{"virtual desktop workspace", "virtual_desktop", "read_file"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected relative virtual desktop guidance to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDispatchShellRejectsHomepageDataPathInDesktopChat(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Directories.WorkspaceDir = t.TempDir()

	out := dispatchShell(ToolCall{
		Action: "execute_shell",
		Params: map[string]interface{}{
			"command": "cat /home/aurago/aurago/data/homepage/space-invaders/game.js",
		},
	}, &DispatchContext{
		Cfg:           cfg,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		MessageSource: "virtual_desktop_chat",
	})

	for _, want := range []string{"homepage workspace", "homepage", "not a Virtual Desktop file"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected homepage guidance to contain %q, got:\n%s", want, out)
		}
	}
}
