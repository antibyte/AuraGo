package server

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/tools"
)

// newLocalWikipediaManager builds the passive Local Wikipedia manager; Start
// loads the installed edition with the server lifetime. The manager builds its
// own HTTP client, which trusts only the Kiwix catalog host.
func newLocalWikipediaManager(cfg *config.Config, logger *slog.Logger) *localwiki.Manager {
	manager := localwiki.NewManager(localwiki.Deps{
		Logger:          logger,
		IsSensitivePath: localWikipediaSensitivePath(cfg.Directories.DataDir),
	})
	manager.Configure(localwiki.SettingsFromConfig(cfg))
	return manager
}

// localWikipediaSensitivePath refuses system locations (the Docker bind
// denylist via tools.IsSensitiveHostDirectory) and AuraGo's data directory
// root, whose state.json belongs to AuraGo itself.
func localWikipediaSensitivePath(dataDir string) func(string) bool {
	root := strings.TrimSpace(dataDir)
	return func(dir string) bool {
		if tools.IsSensitiveHostDirectory(dir) {
			return true
		}
		return root != "" && sameLocalPath(dir, root)
	}
}

func sameLocalPath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// validateLocalWikipediaSettings checks a candidate storage directory on
// native installs: absolute, no system location, not AuraGo's data root.
// Docker ignores the field because the directory is fixed in the data volume.
func validateLocalWikipediaSettings(s *Server, candidate config.LocalWikipediaConfig, rt config.Runtime) error {
	if rt.IsDocker {
		return nil
	}
	dir := strings.TrimSpace(candidate.DataDir)
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("local_wikipedia.data_dir must be an absolute path")
	}
	dataDir := ""
	if current := s.ConfigSnapshot(); current != nil {
		dataDir = current.Directories.DataDir
	}
	if localWikipediaSensitivePath(dataDir)(dir) {
		return fmt.Errorf("local_wikipedia.data_dir must not be a system directory or AuraGo's data directory")
	}
	return nil
}

// injectLocalWikipediaDefaults shows the loader defaults for keys an older
// config.yaml does not contain yet (agent_access and update_check default on).
func injectLocalWikipediaDefaults(rawCfg map[string]interface{}, cfg *config.Config) {
	if cfg == nil {
		return
	}
	section, ok := rawCfg["local_wikipedia"].(map[string]interface{})
	if !ok {
		section = make(map[string]interface{})
		rawCfg["local_wikipedia"] = section
	}
	setDefaultBool(section, "enabled", cfg.LocalWikipedia.Enabled)
	setDefaultBool(section, "agent_access", cfg.LocalWikipedia.AgentAccess)
	setDefaultBool(section, "update_check", cfg.LocalWikipedia.UpdateCheck)
	for key, value := range map[string]string{
		"language": cfg.LocalWikipedia.Language,
		"variant":  cfg.LocalWikipedia.Variant,
		"data_dir": cfg.LocalWikipedia.DataDir,
	} {
		if _, exists := section[key]; !exists {
			section[key] = value
		}
	}
}
