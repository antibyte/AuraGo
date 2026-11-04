package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

// restoreHasFlowsDatabase verifies the staged database will be used by the
// configuration that starts after the restore, not just the current config.
func restoreHasFlowsDatabase(cfg *config.Config, entries []agoImportEntry) bool {
	if cfg == nil || cfg.ConfigPath == "" {
		return false
	}
	configPath, err := filepath.Abs(cfg.ConfigPath)
	if err != nil {
		return false
	}
	flowPath := config.FlowsDBPath(cfg)
	configStaged := false
	for _, entry := range entries {
		if entry.isDir || !sameRestorePath(entry.dest, configPath) {
			continue
		}
		configStaged = true
		data, err := os.ReadFile(entry.stage)
		if err != nil {
			return false
		}
		var raw struct {
			SQLite struct {
				GameMakerPath string `yaml:"game_maker_path"`
			} `yaml:"sqlite"`
		}
		if yaml.Unmarshal(data, &raw) != nil {
			return false
		}
		gm := raw.SQLite.GameMakerPath
		if gm == "" {
			gm = "./data/game_maker.db"
		}
		loaded, err := config.Load(entry.stage)
		if err != nil {
			return false
		}
		stagedGM := gm
		if !filepath.IsAbs(stagedGM) {
			stagedGM = filepath.Join(filepath.Dir(entry.stage), stagedGM)
		}
		if !sameRestorePath(stagedGM, loaded.SQLite.GameMakerPath) {
			return false
		}
		if !filepath.IsAbs(gm) {
			gm = filepath.Join(filepath.Dir(configPath), gm)
		}
		flowPath = filepath.Join(filepath.Dir(gm), config.FlowsDBFilename)
		break
	}
	if !configStaged || flowPath == "" {
		return false
	}
	for _, entry := range entries {
		if !entry.isDir && sameRestorePath(entry.dest, flowPath) {
			return true
		}
	}
	return false
}

func restoreContainsFlowsDatabase(entries []agoImportEntry) bool {
	for _, entry := range entries {
		if !entry.isDir && strings.EqualFold(filepath.Base(entry.dest), config.FlowsDBFilename) {
			return true
		}
	}
	return false
}

func sameRestorePath(a, b string) bool {
	a, aErr := filepath.Abs(a)
	b, bErr := filepath.Abs(b)
	if aErr != nil || bErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
