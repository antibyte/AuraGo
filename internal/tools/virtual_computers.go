package tools

import (
	"context"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

func ExecuteVirtualComputers(ctx context.Context, cfg *config.Config, args map[string]interface{}) string {
	return virtualcomputers.ExecuteTool(ctx, virtualcomputers.FromAuraConfig(cfg), args)
}

func ExecuteVirtualWorkspace(ctx context.Context, cfg *config.Config, identity virtualcomputers.WorkspaceIdentity, args map[string]interface{}) string {
	return virtualcomputers.ExecuteWorkspaceTool(ctx, virtualcomputers.FromAuraConfig(cfg), identity, args)
}

func ExecuteVirtualBrowser(ctx context.Context, cfg *config.Config, identity virtualcomputers.WorkspaceIdentity, args map[string]interface{}) string {
	return virtualcomputers.ExecuteBrowserTool(ctx, virtualcomputers.FromAuraConfig(cfg), identity, args)
}
