package tools

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
			"minimax": {Name: "minimax", Command: "uvx", Enabled: true},
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
			"minimax": {Name: "minimax", Command: "uvx", Enabled: true},
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
