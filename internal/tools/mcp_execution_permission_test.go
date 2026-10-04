package tools

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPStdioChecksPermissionsBeforeProcessOrWorkspaceCreation(t *testing.T) {
	previous, configured := CurrentRuntimePermissionsForTest()
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})
	ConfigureRuntimePermissions(RuntimePermissions{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, runtime := range []string{"local", "docker"} {
		dir := filepath.Join(t.TempDir(), "must-not-exist")
		_, err := startMCPServerConnection(context.Background(), MCPServerConfig{
			Name: "fixture", Command: "must-not-execute", DockerImage: "must-not-pull", Runtime: runtime, HostWorkdir: dir, AllowLocalFallback: true,
		}, logger)
		if err == nil || !strings.Contains(err.Error(), "shell") {
			t.Fatalf("runtime=%s missing shell denial: %v", runtime, err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatal("disabled MCP created a workspace")
		}
	}
}

func TestMCPStdioSnapshotCannotWidenLiveExecutionRights(t *testing.T) {
	previous, configured := CurrentRuntimePermissionsForTest()
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})
	ConfigureRuntimePermissions(RuntimePermissions{})
	perms := RuntimePermissions{AllowShell: true, AllowUnsafeHostExecution: true, DockerEnabled: true}
	_, err := startMCPServerConnection(context.Background(), MCPServerConfig{
		Name: "fixture", Command: "must-not-execute", MCPEnabled: true, ExecutionPermissions: &perms,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "shell") {
		t.Fatalf("snapshot overrode live shell revocation: %v", err)
	}
}

func TestMCPStdioRejectsSandboxControlOverride(t *testing.T) {
	_, err := newMCPConn("fixture", "must-not-execute", nil, map[string]string{"AURAGO_SBX_RW": "/"}, slog.New(slog.NewTextHandler(io.Discard, nil)), "local", t.TempDir(), "")
	if err == nil || !strings.Contains(err.Error(), "sandbox controls") {
		t.Fatalf("sandbox environment override accepted: %v", err)
	}
}
