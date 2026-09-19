package server

import (
	"aurago/internal/agent"
	"aurago/internal/config"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestDisclosureRegressionRunningDispatchSeesPermissionRevocation(t *testing.T) {
	root := t.TempDir()
	old := &config.Config{}
	old.Directories.WorkspaceDir = root
	old.Agent.AllowFilesystemWrite = true
	s := &Server{Cfg: old}
	s.initConfigSnapshot()
	running := &agent.DispatchContext{Cfg: s.ConfigSnapshot(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	next := *old
	next.Agent.AllowFilesystemWrite = false
	s.replaceConfigSnapshot(&next)
	path := filepath.Join(root, "review-revocation.txt")
	tc := agent.ToolCall{Action: "filesystem", Operation: "write_file", FilePath: path, Content: "review fixture"}
	result := agent.DispatchToolCallResult(context.Background(), &tc, running, "write fixture")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("file created after permission revoked; currentGate=%v resultStatus=%s", s.ConfigSnapshot().Agent.AllowFilesystemWrite, result.Status)
	}
	if !result.IsError {
		t.Fatalf("expected permission denial, received %+v", result)
	}
}
