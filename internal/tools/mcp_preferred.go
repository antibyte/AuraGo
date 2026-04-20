package tools

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

var (
	listPreferredMCPTools = MCPListTools
	callPreferredMCPTool  = MCPCallTool
)

func hasPreferredMCPSelection(sel config.MCPPreferredToolSelection) bool {
	return strings.TrimSpace(sel.Server) != "" && strings.TrimSpace(sel.Tool) != ""
}

func findPreferredMCPTool(sel config.MCPPreferredToolSelection, logger *slog.Logger) (*MCPToolInfo, error) {
	if !hasPreferredMCPSelection(sel) {
		return nil, nil
	}
	tools, err := listPreferredMCPTools(strings.TrimSpace(sel.Server), logger)
	if err != nil {
		return nil, err
	}
	for i := range tools {
		if strings.EqualFold(strings.TrimSpace(tools[i].Name), strings.TrimSpace(sel.Tool)) {
			return &tools[i], nil
		}
	}
	return nil, fmt.Errorf("preferred MCP tool %q not found on server %q", sel.Tool, sel.Server)
}

func mcpInputProperties(tool *MCPToolInfo) map[string]interface{} {
	if tool == nil || len(tool.InputSchema) == 0 {
		return nil
	}
	props, _ := tool.InputSchema["properties"].(map[string]interface{})
	return props
}

func setFirstMatchingMCPArg(args map[string]interface{}, props map[string]interface{}, value interface{}, names ...string) bool {
	if args == nil || len(names) == 0 {
		return false
	}
	if len(props) == 0 {
		args[names[0]] = value
		return true
	}
	for _, name := range names {
		if _, ok := props[name]; ok {
			args[name] = value
			return true
		}
	}
	return false
}

func buildPreferredMCPWebSearchArgs(tool *MCPToolInfo, query string, count int, country, lang string) (map[string]interface{}, error) {
	args := make(map[string]interface{})
	props := mcpInputProperties(tool)

	if !setFirstMatchingMCPArg(args, props, query, "query", "q", "search_query", "search", "keywords", "keyword", "text", "prompt", "topic") {
		return nil, fmt.Errorf("preferred MCP search tool %q does not expose a recognized query input", tool.Name)
	}
	if count > 0 {
		setFirstMatchingMCPArg(args, props, count, "count", "max_results", "limit", "num_results", "top_k")
	}
	if country != "" {
		setFirstMatchingMCPArg(args, props, country, "country", "country_code", "region")
	}
	if lang != "" {
		setFirstMatchingMCPArg(args, props, lang, "lang", "language", "locale")
	}
	return args, nil
}

func buildPreferredMCPVisionArgs(tool *MCPToolInfo, resolvedPath, prompt string) (map[string]interface{}, error) {
	args := make(map[string]interface{})
	props := mcpInputProperties(tool)

	hasImageInput := false
	if setFirstMatchingMCPArg(args, props, resolvedPath, "file_path", "path", "image_path", "image_file", "local_path", "input_path", "image", "image_url", "url") {
		hasImageInput = true
	}
	if !hasImageInput {
		raw, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("read image for MCP fallback: %w", err)
		}
		encoded := base64.StdEncoding.EncodeToString(raw)
		if setFirstMatchingMCPArg(args, props, encoded, "image_base64", "base64_image", "base64", "image_data", "data") {
			hasImageInput = true
		}
	}
	if !hasImageInput {
		return nil, fmt.Errorf("preferred MCP vision tool %q does not expose a recognized image input", tool.Name)
	}

	setFirstMatchingMCPArg(args, props, prompt, "prompt", "question", "query", "instruction", "text")
	setFirstMatchingMCPArg(args, props, filepath.Base(resolvedPath), "filename", "file_name", "name")
	return args, nil
}

// CallPreferredMCPWebSearch routes a web search request through a user-selected MCP tool when configured.
func CallPreferredMCPWebSearch(cfg *config.Config, query string, count int, country, lang string, logger *slog.Logger) (string, bool, error) {
	if cfg == nil || !cfg.Agent.AllowMCP || !cfg.MCP.Enabled {
		return "", false, nil
	}
	selection := cfg.MCP.PreferredCapabilities.WebSearch
	if !hasPreferredMCPSelection(selection) {
		return "", false, nil
	}
	toolInfo, err := findPreferredMCPTool(selection, logger)
	if err != nil {
		return "", true, err
	}
	args, err := buildPreferredMCPWebSearchArgs(toolInfo, query, count, country, lang)
	if err != nil {
		return "", true, err
	}
	result, err := callPreferredMCPTool(selection.Server, selection.Tool, args, logger)
	if err != nil {
		return "", true, err
	}
	return result, true, nil
}

// CallPreferredMCPVision routes an image analysis request through a user-selected MCP tool when configured.
func CallPreferredMCPVision(cfg *config.Config, filePath, prompt string, logger *slog.Logger) (string, bool, error) {
	if cfg == nil || !cfg.Agent.AllowMCP || !cfg.MCP.Enabled {
		return "", false, nil
	}
	selection := cfg.MCP.PreferredCapabilities.Vision
	if !hasPreferredMCPSelection(selection) {
		return "", false, nil
	}
	toolInfo, err := findPreferredMCPTool(selection, logger)
	if err != nil {
		return "", true, err
	}
	resolvedPath, err := ResolveToolInputPath(filePath, cfg)
	if err != nil {
		return "", true, err
	}
	args, err := buildPreferredMCPVisionArgs(toolInfo, resolvedPath, prompt)
	if err != nil {
		return "", true, err
	}
	result, err := callPreferredMCPTool(selection.Server, selection.Tool, args, logger)
	if err != nil {
		return "", true, err
	}
	return result, true, nil
}
