package server

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestMCPEffectiveAllowedToolsUsesVSCodePreset(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCPServer.VSCodeDebugBridge = true

	got := mcpEffectiveAllowedTools(cfg)
	joined := strings.Join(got, ",")
	for _, want := range []string{"ask_aurago", "execute_shell", "api_request", "query_memory"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("effective allowed tools missing %q: %v", want, got)
		}
	}
}

func TestBuildVSCodeBridgeConfigSnippet(t *testing.T) {
	snippet, err := buildVSCodeBridgeConfigSnippet("https://aurago.example/mcp")
	if err != nil {
		t.Fatalf("buildVSCodeBridgeConfigSnippet: %v", err)
	}
	for _, want := range []string{
		`"type": "http"`,
		`"url": "https://aurago.example/mcp"`,
		`${input:aurago-mcp-token}`,
	} {
		if !strings.Contains(snippet, want) {
			t.Fatalf("snippet missing %q:\n%s", want, snippet)
		}
	}
}

func TestBuildCursorBridgeConfigSnippet(t *testing.T) {
	snippet, err := buildCursorBridgeConfigSnippet("https://aurago.example/mcp")
	if err != nil {
		t.Fatalf("buildCursorBridgeConfigSnippet: %v", err)
	}
	for _, want := range []string{
		`"mcpServers"`,
		`"url": "https://aurago.example/mcp"`,
		`${env:AURAGO_MCP_TOKEN}`,
	} {
		if !strings.Contains(snippet, want) {
			t.Fatalf("snippet missing %q:\n%s", want, snippet)
		}
	}
}

func TestBuildClaudeDesktopBridgeConfigSnippet(t *testing.T) {
	snippet, err := buildClaudeDesktopBridgeConfigSnippet("https://aurago.example/mcp")
	if err != nil {
		t.Fatalf("buildClaudeDesktopBridgeConfigSnippet: %v", err)
	}
	for _, want := range []string{
		`"mcpServers"`,
		`"type": "http"`,
		`${AURAGO_MCP_TOKEN}`,
	} {
		if !strings.Contains(snippet, want) {
			t.Fatalf("snippet missing %q:\n%s", want, snippet)
		}
	}
}

func TestMCPBuildToolListIncludesAskAuraGoWhenBridgeEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.MCPServer.VSCodeDebugBridge = true

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	tools := mcpBuildToolList(s)
	for _, tool := range tools {
		if tool.Name == "ask_aurago" {
			return
		}
	}

	t.Fatalf("ask_aurago not found in MCP tool list")
}
