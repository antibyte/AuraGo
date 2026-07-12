package tools

import (
	"context"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

func ExecuteVirtualComputers(ctx context.Context, cfg *config.Config, args map[string]interface{}) string {
	return virtualcomputers.ExecuteTool(ctx, virtualcomputers.FromAuraConfig(cfg), args)
}
