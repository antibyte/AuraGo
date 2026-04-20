package tools

import (
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
