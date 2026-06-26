package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"aurago/internal/config"
	"aurago/internal/setup"
)

// bootstrapIfNeeded inspects the install directory for resources.dat and runs
// setup.Run if the config is missing or invalid AND setup is needed. Returns
// the reloaded config or an error.
//
// Must be called WITH the application file lock held — see main().
func bootstrapIfNeeded(installDir, configFile string, logger *slog.Logger) (*config.Config, error) {
	resPath := filepath.Join(installDir, "resources.dat")
	if _, statErr := os.Stat(resPath); statErr != nil {
		logger.Warn("resources.dat not found in install directory — bootstrapping from local defaults", "path", resPath)
	} else {
		logger.Info("Running automatic setup from resources.dat")
	}
	if setupErr := setup.Run(logger); setupErr != nil {
		return nil, fmt.Errorf("auto-setup failed: %w", setupErr)
	}
	reloaded, err := config.Load(configFile)
	if err != nil {
		return nil, fmt.Errorf("reload config after auto-setup: %w", err)
	}
	return reloaded, nil
}
