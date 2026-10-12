package tools

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"aurago/internal/config"
)

var (
	listPreferredMCPTools = MCPListToolsContext
	callPreferredMCPTool  = MCPCallTool
)

func hasPreferredMCPSelection(sel config.MCPPreferredToolSelection) bool {
	return strings.TrimSpace(sel.Server) != "" && strings.TrimSpace(sel.Tool) != ""
}

func findPreferredMCPTool(ctx context.Context, sel config.MCPPreferredToolSelection, logger *slog.Logger) (*MCPToolInfo, error) {
	if !hasPreferredMCPSelection(sel) {
		return nil, nil
	}
	tools, err := listPreferredMCPTools(ctx, strings.TrimSpace(sel.Server), logger)
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
	return extractMCPInputProperties(tool.InputSchema)
}

func extractMCPInputProperties(schema map[string]interface{}) map[string]interface{} {
	if len(schema) == 0 {
		return nil
	}
	if props, _ := schema["properties"].(map[string]interface{}); len(props) > 0 {
		return props
	}
	for _, key := range []string{"inputSchema", "input_schema", "schema", "parameters"} {
		nested, _ := schema[key].(map[string]interface{})
		if props := extractMCPInputProperties(nested); len(props) > 0 {
			return props
		}
	}
	return nil
}

func summarizeMCPInputSchema(schema map[string]interface{}) string {
	props := extractMCPInputProperties(schema)
	if len(props) == 0 {
		return "no properties found"
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
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

// CallPreferredMCPWebSearch routes a web search request through a user-selected MCP tool when configured.
func CallPreferredMCPWebSearch(ctx context.Context, cfg *config.Config, query string, count int, country, lang string, logger *slog.Logger) (string, bool, error) {
	if cfg == nil || !cfg.Agent.AllowMCP || !cfg.MCP.Enabled {
		return "", false, nil
	}
	selection := cfg.MCP.PreferredCapabilities.WebSearch
	if !hasPreferredMCPSelection(selection) {
		return "", false, nil
	}
	toolInfo, err := findPreferredMCPTool(ctx, selection, logger)
	if err != nil {
		return "", true, err
	}
	args, err := buildPreferredMCPWebSearchArgs(toolInfo, query, count, country, lang)
	if err != nil {
		return "", true, err
	}
	result, err := callDiscoveredMCPTool(ctx, toolInfo, selection, args, logger)
	if err != nil {
		return "", true, err
	}
	return result, true, nil
}

// CallPreferredMCPVision routes an image analysis request through a user-selected MCP tool when configured.
func CallPreferredMCPVision(ctx context.Context, cfg *config.Config, filePath, prompt string, logger *slog.Logger) (string, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil || !cfg.Agent.AllowMCP || !cfg.MCP.Enabled {
		return "", false, nil
	}
	selection := cfg.MCP.PreferredCapabilities.Vision
	if !hasPreferredMCPSelection(selection) {
		return "", false, nil
	}
	toolInfo, err := findPreferredMCPTool(ctx, selection, logger)
	if err != nil {
		return "", true, err
	}
	resolvedPath, err := ResolveToolInputPath(filePath, cfg)
	if err != nil {
		return "", true, err
	}
	args, cleanup, err := buildPreferredMCPVisionArgs(ctx, toolInfo, resolvedPath, prompt)
	if err != nil {
		return "", true, err
	}
	defer cleanup()
	if err := ctx.Err(); err != nil {
		return "", true, err
	}
	result, err := callDiscoveredMCPTool(ctx, toolInfo, selection, args, logger)
	if err != nil {
		return "", true, err
	}
	return result, true, nil
}

func callDiscoveredMCPTool(ctx context.Context, tool *MCPToolInfo, selection config.MCPPreferredToolSelection, args map[string]interface{}, logger *slog.Logger) (string, error) {
	if tool.manager != nil {
		return tool.manager.callTool(ctx, strings.TrimSpace(selection.Server), strings.TrimSpace(selection.Tool), args, tool.connection)
	}
	return callPreferredMCPTool(ctx, selection.Server, selection.Tool, args, logger)
}
