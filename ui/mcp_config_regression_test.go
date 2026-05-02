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
