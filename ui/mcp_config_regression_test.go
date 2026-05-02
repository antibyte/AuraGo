package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPConfigEmptyAllowedToolsMeansAllDiscovered(t *testing.T) {
	t.Parallel()

	mcpJS, err := os.ReadFile(filepath.Join("cfg", "mcp.js"))
	if err != nil {
		t.Fatalf("read cfg/mcp.js: %v", err)
	}
	js := string(mcpJS)
	for _, marker := range []string{
		"allowedToolsCount > 0 ? String(allowedToolsCount) : t('config.mcp.card_allowed_tools_all')",
		"escapeAttr(allowedToolsLabel)",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("cfg/mcp.js missing empty-allowlist-as-all marker %q", marker)
		}
	}

	langFiles, err := filepath.Glob(filepath.Join("lang", "config", "mcp", "*.json"))
	if err != nil {
		t.Fatalf("glob mcp lang files: %v", err)
	}
	if len(langFiles) == 0 {
		t.Fatal("no MCP config language files found")
	}
	for _, path := range langFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var entries map[string]string
		if err := json.Unmarshal(content, &entries); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if strings.TrimSpace(entries["config.mcp.card_allowed_tools_all"]) == "" {
			t.Fatalf("%s missing card_allowed_tools_all translation", path)
		}
		hint := entries["config.mcp.field_allowed_tools_hint"]
		if hint == "" {
			t.Fatalf("%s missing field_allowed_tools_hint translation", path)
		}
		if strings.Contains(strings.ToLower(hint), "no tools") ||
			strings.Contains(strings.ToLower(hint), "keine tools") ||
			strings.Contains(strings.ToLower(hint), "keine werkzeuge") {
			t.Fatalf("%s still describes empty allowed_tools as no tools: %q", path, hint)
		}
	}
}

func TestMCPConfigNetworkTransportUI(t *testing.T) {
	t.Parallel()

	mcpJS, err := os.ReadFile(filepath.Join("cfg", "mcp.js"))
	if err != nil {
		t.Fatalf("read cfg/mcp.js: %v", err)
	}
	js := string(mcpJS)
	for _, marker := range []string{
		"id=\"mcp-m-transport\"",
		"value=\"streamable_http\"",
		"value=\"sse\"",
		"value=\"websocket\"",
		"/api/mcp-runtime/test-connection",
		"mcp-network-fields",
		"mcp-stdio-fields",
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("cfg/mcp.js missing network MCP UI marker %q", marker)
		}
	}

	requiredKeys := []string{
		"config.mcp.card_transport",
		"config.mcp.card_url",
		"config.mcp.card_headers",
		"config.mcp.field_transport",
		"config.mcp.transport_stdio",
		"config.mcp.transport_streamable_http",
		"config.mcp.transport_sse",
		"config.mcp.transport_websocket",
		"config.mcp.field_url",
		"config.mcp.field_headers",
		"config.mcp.headers_hint",
		"config.mcp.test_connection",
		"config.mcp.test_connection_success",
		"config.mcp.test_connection_failed",
		"config.mcp.name_command_or_url_required",
	}

	langFiles, err := filepath.Glob(filepath.Join("lang", "config", "mcp", "*.json"))
	if err != nil {
		t.Fatalf("glob mcp lang files: %v", err)
	}
	for _, path := range langFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var entries map[string]string
		if err := json.Unmarshal(content, &entries); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range requiredKeys {
			if strings.TrimSpace(entries[key]) == "" {
				t.Fatalf("%s missing %s translation", path, key)
			}
		}
	}
}
