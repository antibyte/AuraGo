package server

import (
	"database/sql"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/sqlconnections"
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

func TestMCPBuildToolListRequiresSQLRuntimeDependencies(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.SQLConnections.Enabled = true

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	toolsWithoutRuntime := mcpBuildToolList(s)
	for _, tool := range toolsWithoutRuntime {
		if tool.Name == "sql_query" || tool.Name == "manage_sql_connections" {
			t.Fatalf("unexpected SQL tool %q without runtime dependencies", tool.Name)
		}
	}

	s.SQLConnectionsDB = &sql.DB{}
	s.SQLConnectionPool = &sqlconnections.ConnectionPool{}
	if !mcpToolAvailable(s, "sql_query") {
		t.Fatal("expected sql_query to become available once runtime dependencies exist")
	}
	if mcpToolAvailable(s, "definitely_missing_tool") {
		t.Fatal("unexpected availability for unknown MCP tool")
	}
}

func TestMCPFeatureFlagsIncludeMediaConversion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.MediaConversion.Enabled = true

	flags := mcpFeatureFlags(&Server{Cfg: cfg})
	if !flags.MediaConversionEnabled {
		t.Fatal("expected MediaConversionEnabled to be true")
	}
}

func TestMCPFeatureFlagsIncludeSendYouTubeVideo(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.SendYouTubeVideo.Enabled = true

	flags := mcpFeatureFlags(&Server{Cfg: cfg})
	if !flags.SendYouTubeVideoEnabled {
		t.Fatal("expected SendYouTubeVideoEnabled to be true")
	}
}
