package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func TestMCPBuildToolListRequiresExplicitAllowedTools(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Tools.Memory.Enabled = true
	cfg.MCPServer.Enabled = true

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	if tools := mcpBuildToolList(s); len(tools) != 0 {
		t.Fatalf("mcpBuildToolList with empty allowlist returned %d tools, want none", len(tools))
	}
}

func TestMCPCallToolRejectsEmptyAllowedTools(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Tools.Memory.Enabled = true

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	params, err := json.Marshal(mcpCallToolParams{Name: "query_memory"})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	got, protocolErr := mcpCallTool(context.Background(), s, params)
	if protocolErr != nil {
		t.Fatalf("mcpCallTool protocol error = %v", protocolErr)
	}
	if !got.IsError {
		t.Fatal("mcpCallTool with empty allowlist succeeded, want error")
	}
	if len(got.Content) == 0 || !strings.Contains(got.Content[0].Text, "not allowed") {
		t.Fatalf("unexpected mcpCallTool error content: %#v", got.Content)
	}
}

func TestMCPCallToolRejectsToolsOutsideAllowedList(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	cfg.Tools.Memory.Enabled = true
	cfg.MCPServer.AllowedTools = []string{"ask_aurago"}

	s := &Server{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	params, err := json.Marshal(mcpCallToolParams{Name: "query_memory"})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}

	got, protocolErr := mcpCallTool(context.Background(), s, params)
	if protocolErr != nil {
		t.Fatalf("mcpCallTool protocol error = %v", protocolErr)
	}
	if !got.IsError {
		t.Fatal("mcpCallTool outside allowlist succeeded, want error")
	}
	if len(got.Content) == 0 || !strings.Contains(got.Content[0].Text, "not allowed") {
		t.Fatalf("unexpected mcpCallTool error content: %#v", got.Content)
	}
}

func TestMCPEndpointTransportAndVersionContract(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCPServer.Enabled = true
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handler := handleMCPEndpoint(s)

	t.Run("rejects foreign origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://aurago.local/mcp", strings.NewReader(`{}`))
		req.Host = "aurago.local"
		req.Header.Set("Origin", "https://evil.example")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("allows native client without origin and declines get stream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://aurago.local/mcp", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want 405", rec.Code)
		}
	})

	t.Run("accepts initialized notification", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://aurago.local/mcp", strings.NewReader(
			`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want 202", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("body = %q, want empty", rec.Body.String())
		}
	})

	for _, version := range []string{mcpProtocolVersion, mcpLegacyProtocolVersion} {
		t.Run("negotiates "+version, func(t *testing.T) {
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`, version)
			req := httptest.NewRequest(http.MethodPost, "http://aurago.local/mcp", strings.NewReader(body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp struct {
				Result mcpInitializeResult `json:"result"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Result.ProtocolVersion != version {
				t.Fatalf("protocolVersion = %q, want %q", resp.Result.ProtocolVersion, version)
			}
		})
	}

	t.Run("rejects unsupported protocol header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://aurago.local/mcp", strings.NewReader(
			`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		))
		req.Header.Set("MCP-Protocol-Version", "2099-01-01")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
}

func TestMCPCallToolUsesProtocolErrorsForInvalidRequests(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.SkillsDir = t.TempDir()
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	if _, protocolErr := mcpCallTool(context.Background(), s, json.RawMessage(`{`)); protocolErr == nil || protocolErr.Code != -32602 {
		t.Fatalf("malformed params error = %#v, want -32602", protocolErr)
	}
	params, err := json.Marshal(mcpCallToolParams{Name: "not_a_real_tool"})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	if _, protocolErr := mcpCallTool(context.Background(), s, params); protocolErr == nil || protocolErr.Code != -32602 {
		t.Fatalf("unknown tool error = %#v, want -32602", protocolErr)
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

func TestMCPFeatureFlagsIncludeVideoDownloadPermissions(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.VideoDownload.Enabled = true
	cfg.Tools.VideoDownload.AllowDownload = true
	cfg.Tools.VideoDownload.AllowTranscribe = true

	flags := mcpFeatureFlags(&Server{Cfg: cfg})
	if !flags.VideoDownloadEnabled {
		t.Fatal("expected VideoDownloadEnabled to be true")
	}
	if !flags.VideoDownloadAllowDownload {
		t.Fatal("expected VideoDownloadAllowDownload to be true")
	}
	if !flags.VideoDownloadAllowTranscribe {
		t.Fatal("expected VideoDownloadAllowTranscribe to be true")
	}

	cfg.Tools.VideoDownload.ReadOnly = true
	flags = mcpFeatureFlags(&Server{Cfg: cfg})
	if flags.VideoDownloadAllowDownload || flags.VideoDownloadAllowTranscribe {
		t.Fatal("expected read-only mode to suppress video download write permissions")
	}
}
