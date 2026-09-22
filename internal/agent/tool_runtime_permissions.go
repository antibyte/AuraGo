package agent

import (
	"aurago/internal/config"
	"aurago/internal/tools"
)

func configureToolRuntimePermissions(cfg *config.Config) {
	if cfg == nil {
		return
	}
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))
}
