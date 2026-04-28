package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchFilesystemRejectsOutsideHostWriteCanary(t *testing.T) {
	t.Parallel()

	tempRoot := t.TempDir()
	repoRoot := filepath.Join(tempRoot, "repo")
	workspaceDir := filepath.Join(repoRoot, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	outsidePath := filepath.Join(tempRoot, "outside-host.txt")
	const original = "original host content"
	if err := os.WriteFile(outsidePath, []byte(original), 0o644); err != nil {
		t.Fatalf("create outside canary file: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Directories.WorkspaceDir = workspaceDir
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	calls := []ToolCall{
		{
			Action:    "filesystem",
			Operation: "write_file",
			FilePath:  outsidePath,
			Content:   "mutated by filesystem",
		},
		{
			Action:    "file_editor",
			Operation: "str_replace",
			FilePath:  outsidePath,
			Params: map[string]interface{}{
				"old": "original",
				"new": "mutated by file editor",
			},
		},
	}

	for _, tc := range calls {
		output := dispatchFilesystem(context.Background(), tc, dc)
		if !strings.Contains(output, `"status":"error"`) {
			t.Fatalf("%s outside-host write did not return an error: %s", tc.Action, output)
		}
		if !strings.Contains(output, "absolute path outside the project root") {
			t.Fatalf("%s outside-host write returned the wrong error: %s", tc.Action, output)
		}

		got, err := os.ReadFile(outsidePath)
		if err != nil {
			t.Fatalf("read outside canary after %s: %v", tc.Action, err)
		}
		if string(got) != original {
			t.Fatalf("%s mutated outside-host file: got %q", tc.Action, string(got))
		}
	}
}
