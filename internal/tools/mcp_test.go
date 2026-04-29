package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandMCPPathValueExpandsHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := expandMCPPathValue("~/aurago/mcp")
	want := filepath.Join(home, "aurago", "mcp")
	if got != want {
		t.Fatalf("expandMCPPathValue() = %q, want %q", got, want)
	}
}

func TestNormalizeMCPEnvExpandsPathLikeValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := normalizeMCPEnv(map[string]string{
		"MINIMAX_MCP_BASE_PATH": "~/aurago/mcp",
		"MINIMAX_API_HOST":      "https://api.minimax.io",
	})

	if got["MINIMAX_MCP_BASE_PATH"] != filepath.Join(home, "aurago", "mcp") {
		t.Fatalf("MINIMAX_MCP_BASE_PATH = %q", got["MINIMAX_MCP_BASE_PATH"])
	}
	if got["MINIMAX_API_HOST"] != "https://api.minimax.io" {
		t.Fatalf("MINIMAX_API_HOST = %q", got["MINIMAX_API_HOST"])
	}
}

func TestNormalizeMCPArgsExpandsPathLikeValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := normalizeMCPArgs([]string{"--state-dir", "~/aurago/mcp"})
	if got[1] != filepath.Join(home, "aurago", "mcp") {
		t.Fatalf("normalizeMCPArgs()[1] = %q", got[1])
	}
}

func TestResolveMCPCommandPathFallsBackToUserLocalBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", "")

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	commandName := "uvx"
	commandPath := filepath.Join(localBin, commandName)
	if err := os.WriteFile(commandPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got := resolveMCPCommandPath(commandName)
	if got != commandPath {
		t.Fatalf("resolveMCPCommandPath() = %q, want %q", got, commandPath)
	}
}

func TestResolveMCPLaunchArgsAndEnvInterpolatesSecretsAndWorkdirs(t *testing.T) {
	server := MCPServerConfig{
		Name:             "minimax",
		Command:          "uvx",
		Args:             []string{"--base", "{{workdir.output}}"},
		Env:              map[string]string{"API_KEY": "{{api-token}}", "BASE": "{{workdir}}"},
		HostWorkdir:      filepath.Join(t.TempDir(), "minimax"),
		ContainerWorkdir: "/workspace",
		Secrets:          map[string]string{"api-token": "secret-value"},
	}

	args, env, err := resolveMCPLaunchArgsAndEnv(server, false)
	if err != nil {
		t.Fatalf("resolveMCPLaunchArgsAndEnv(local) error = %v", err)
	}
	if got, want := args[1], filepath.Join(server.HostWorkdir, "output"); got != want {
		t.Fatalf("local arg = %q, want %q", got, want)
	}
	if got := env["API_KEY"]; got != "secret-value" {
		t.Fatalf("local API_KEY = %q, want secret-value", got)
	}
	if got := env["BASE"]; got != server.HostWorkdir {
		t.Fatalf("local BASE = %q, want %q", got, server.HostWorkdir)
	}

	args, env, err = resolveMCPLaunchArgsAndEnv(server, true)
	if err != nil {
		t.Fatalf("resolveMCPLaunchArgsAndEnv(docker) error = %v", err)
	}
	if got, want := args[1], "/workspace/output"; filepath.ToSlash(got) != want {
		t.Fatalf("docker arg = %q, want %q", got, want)
	}
	if got := env["BASE"]; filepath.ToSlash(got) != "/workspace" {
		t.Fatalf("docker BASE = %q, want /workspace", got)
	}
}

func TestNormalizeMCPResultTextMapsContainerPathsToHostPaths(t *testing.T) {
	hostDir := filepath.Join(t.TempDir(), "minimax")
	got := normalizeMCPResultText(`{"output_path":"/workspace/output/test.mp3"}`, hostDir, "/workspace")
	want := filepath.Join(hostDir, "output", "test.mp3")
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v; got=%q", err, got)
	}
	if decoded["output_path"] != want {
		t.Fatalf("output_path = %q, want %q", decoded["output_path"], want)
	}
}

func TestNormalizeMCPResultTextDoesNotReplacePartialPaths(t *testing.T) {
	// Ensure /work is NOT replaced when it appears inside /workspace
	hostDir := filepath.Join(t.TempDir(), "app")
	got := normalizeMCPResultText(`{"path":"/workspace-old/file.txt"}`, hostDir, "/workspace")
	// /workspace-old should remain untouched because /workspace is a prefix of /workspace-old
	// but the path does not start with /workspace/ (it starts with /workspace-old/)
	var decoded map[string]string
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v; got=%q", err, got)
	}
	// Since /workspace-old does not start with /workspace/, it should be unchanged
	if decoded["path"] != "/workspace-old/file.txt" {
		t.Fatalf("path = %q, want %q", decoded["path"], "/workspace-old/file.txt")
	}
}

func TestMCPManagerListServersReturnsEmptySlice(t *testing.T) {
	mgr := &MCPManager{conns: map[string]*mcpConn{}}
	servers := mgr.ListServers()
	if servers == nil {
		t.Fatal("ListServers() returned nil, want empty slice")
	}
	if len(servers) != 0 {
		t.Fatalf("len(ListServers()) = %d, want 0", len(servers))
	}
}

func TestMCPManagerListToolsReturnsEmptySlice(t *testing.T) {
	mgr := &MCPManager{conns: map[string]*mcpConn{}}
	tools := mgr.ListTools("")
	if tools == nil {
		t.Fatal("ListTools() returned nil, want empty slice")
	}
	if len(tools) != 0 {
		t.Fatalf("len(ListTools()) = %d, want 0", len(tools))
	}
}

func TestMCPManagerListServersReconnectsConfiguredServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	oldStart := startManagedMCPConn
	oldClose := closeManagedMCPConn
	t.Cleanup(func() {
		startManagedMCPConn = oldStart
		closeManagedMCPConn = oldClose
	})

	startCalls := 0
	startManagedMCPConn = func(srv MCPServerConfig, _ *slog.Logger) (*mcpConn, error) {
		startCalls++
		return &mcpConn{
			name:  srv.Name,
			tools: []MCPToolInfo{{Server: srv.Name, Name: "tts"}},
			ready: true,
		}, nil
	}
	closeManagedMCPConn = func(conn *mcpConn) {}

	mgr := &MCPManager{
		conns: map[string]*mcpConn{},
		configs: map[string]MCPServerConfig{
			"minimax": {Name: "minimax", Command: "uvx", Enabled: true, AllowedTools: []string{"tts"}},
		},
		logger: logger,
	}

	servers := mgr.ListServers()
	if startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", startCalls)
	}
	if len(servers) != 1 {
		t.Fatalf("len(servers) = %d, want 1", len(servers))
	}
	if servers[0]["name"] != "minimax" {
		t.Fatalf("server name = %v, want minimax", servers[0]["name"])
	}
	if servers[0]["tool_count"] != 1 {
		t.Fatalf("tool_count = %v, want 1", servers[0]["tool_count"])
	}
}

func TestMCPManagerCallToolReconnectsAfterTransportFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	oldStart := startManagedMCPConn
	oldInvoke := invokeMCPConnTool
	oldClose := closeManagedMCPConn
	t.Cleanup(func() {
		startManagedMCPConn = oldStart
		invokeMCPConnTool = oldInvoke
		closeManagedMCPConn = oldClose
	})

	var (
		startCalls int
		connA      = &mcpConn{name: "minimax", ready: true}
		connB      = &mcpConn{name: "minimax", ready: true}
	)
	startManagedMCPConn = func(srv MCPServerConfig, _ *slog.Logger) (*mcpConn, error) {
		startCalls++
		if startCalls == 1 {
			return connA, nil
		}
		return connB, nil
	}
	invokeMCPConnTool = func(conn *mcpConn, toolName string, arguments map[string]interface{}) (string, error) {
		if conn == connA {
			return "", fmt.Errorf("tools/call: read from stdout: EOF")
		}
		if conn == connB {
			return "ok", nil
		}
		return "", fmt.Errorf("unexpected conn")
	}
	closeManagedMCPConn = func(conn *mcpConn) {}

	mgr := &MCPManager{
		conns: map[string]*mcpConn{},
		configs: map[string]MCPServerConfig{
			"minimax": {Name: "minimax", Command: "uvx", Enabled: true, AllowedTools: []string{"tts"}},
		},
		logger: logger,
	}

	got, err := mgr.CallTool("minimax", "tts", map[string]interface{}{"text": "Hallo"})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("CallTool() = %q, want ok", got)
	}
	if startCalls != 2 {
		t.Fatalf("startCalls = %d, want 2", startCalls)
	}
}

func TestMCPManagerCallToolEnforcesAllowedTools(t *testing.T) {
	mgr := &MCPManager{
		configs: map[string]MCPServerConfig{
			"safe": {Name: "safe", AllowedTools: []string{"allowed_tool"}},
		},
		conns:  map[string]*mcpConn{},
		logger: slog.Default(),
	}

	if _, err := mgr.CallTool("safe", "blocked_tool", nil); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("CallTool error = %v, want allowlist denial", err)
	}
}

func TestMCPManagerCallToolBlocksDestructiveToolsWithoutToggle(t *testing.T) {
	mgr := &MCPManager{
		configs: map[string]MCPServerConfig{
			"safe": {Name: "safe", AllowedTools: []string{"delete_database"}},
		},
		conns:  map[string]*mcpConn{},
		logger: slog.Default(),
	}

	if _, err := mgr.CallTool("safe", "delete_database", nil); err == nil || !strings.Contains(err.Error(), "allow_destructive") {
		t.Fatalf("CallTool error = %v, want destructive toggle denial", err)
	}
}

func TestMCPManagerListToolsOnlyExposesAllowedTools(t *testing.T) {
	mgr := &MCPManager{
		configs: map[string]MCPServerConfig{
			"safe": {Name: "safe", Enabled: true, Command: "uvx", AllowedTools: []string{"allowed_tool"}},
		},
		conns: map[string]*mcpConn{
			"safe": {
				name:  "safe",
				ready: true,
				tools: []MCPToolInfo{
					{Server: "safe", Name: "allowed_tool"},
					{Server: "safe", Name: "blocked_tool"},
				},
			},
		},
		logger: slog.Default(),
	}

	got := mgr.ListTools("safe")
	if len(got) != 1 || got[0].Name != "allowed_tool" {
		t.Fatalf("ListTools() = %+v, want only allowed_tool", got)
	}
}

func TestMCPManagerListToolsHidesDestructiveToolsWithoutToggle(t *testing.T) {
	mgr := &MCPManager{
		configs: map[string]MCPServerConfig{
			"safe": {Name: "safe", Enabled: true, Command: "uvx", AllowedTools: []string{"delete_database"}},
		},
		conns: map[string]*mcpConn{
			"safe": {
				name:  "safe",
				ready: true,
				tools: []MCPToolInfo{
					{Server: "safe", Name: "delete_database"},
				},
			},
		},
		logger: slog.Default(),
	}

	got := mgr.ListTools("safe")
	if len(got) != 0 {
		t.Fatalf("ListTools() = %+v, want destructive tool hidden", got)
	}
}
