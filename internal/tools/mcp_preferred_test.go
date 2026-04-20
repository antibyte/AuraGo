package tools

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestCallPreferredMCPWebSearchUsesConfiguredSelection(t *testing.T) {
	oldList := listPreferredMCPTools
	oldCall := callPreferredMCPTool
	defer func() {
		listPreferredMCPTools = oldList
		callPreferredMCPTool = oldCall
	}()

	listPreferredMCPTools = func(serverName string, logger *slog.Logger) ([]MCPToolInfo, error) {
		return []MCPToolInfo{{
			Server: serverName,
			Name:   "web_search",
			InputSchema: map[string]interface{}{
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
					"limit": map[string]interface{}{"type": "integer"},
				},
			},
		}}, nil
	}

	var gotArgs map[string]interface{}
	callPreferredMCPTool = func(serverName, toolName string, arguments map[string]interface{}, logger *slog.Logger) (string, error) {
		gotArgs = arguments
		return `{"status":"success"}`, nil
	}

	cfg := &config.Config{}
	cfg.Agent.AllowMCP = true
	cfg.MCP.Enabled = true
	cfg.MCP.PreferredCapabilities.WebSearch = config.MCPPreferredToolSelection{Server: "minimax", Tool: "web_search"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	result, used, err := CallPreferredMCPWebSearch(cfg, "latest ai news", 5, "", "", logger)
	if err != nil {
		t.Fatalf("CallPreferredMCPWebSearch error = %v", err)
	}
	if !used {
		t.Fatal("expected preferred MCP search to be used")
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if gotArgs["query"] != "latest ai news" {
		t.Fatalf("query arg = %#v, want %q", gotArgs["query"], "latest ai news")
	}
	if gotArgs["limit"] != 5 {
		t.Fatalf("limit arg = %#v, want 5", gotArgs["limit"])
	}
}

func TestCallPreferredMCPVisionUsesResolvedFilePath(t *testing.T) {
	oldList := listPreferredMCPTools
	oldCall := callPreferredMCPTool
	defer func() {
		listPreferredMCPTools = oldList
		callPreferredMCPTool = oldCall
	}()

	listPreferredMCPTools = func(serverName string, logger *slog.Logger) ([]MCPToolInfo, error) {
		return []MCPToolInfo{{
			Server: serverName,
			Name:   "vision_tool",
			InputSchema: map[string]interface{}{
				"properties": map[string]interface{}{
					"image_path": map[string]interface{}{"type": "string"},
					"prompt":     map[string]interface{}{"type": "string"},
				},
			},
		}}, nil
	}

	var gotArgs map[string]interface{}
	callPreferredMCPTool = func(serverName, toolName string, arguments map[string]interface{}, logger *slog.Logger) (string, error) {
		gotArgs = arguments
		return `{"status":"success"}`, nil
	}

	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	imagePath := filepath.Join(workspaceDir, "image.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowMCP = true
	cfg.MCP.Enabled = true
	cfg.MCP.PreferredCapabilities.Vision = config.MCPPreferredToolSelection{Server: "minimax", Tool: "vision_tool"}
	cfg.Directories.WorkspaceDir = workspaceDir

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, used, err := CallPreferredMCPVision(cfg, "image.png", "Describe this image", logger)
	if err != nil {
		t.Fatalf("CallPreferredMCPVision error = %v", err)
	}
	if !used {
		t.Fatal("expected preferred MCP vision to be used")
	}

	gotPath, _ := gotArgs["image_path"].(string)
	if !strings.HasSuffix(filepath.ToSlash(gotPath), "/image.png") {
		t.Fatalf("image_path = %q, want resolved workspace file", gotPath)
	}
	if gotArgs["prompt"] != "Describe this image" {
		t.Fatalf("prompt arg = %#v, want %q", gotArgs["prompt"], "Describe this image")
	}
}
