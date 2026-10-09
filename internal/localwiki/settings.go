package localwiki

import (
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

// SettingsFromConfig resolves the manager settings from a loaded config. An
// empty language follows agent.system_language; in Docker the storage
// directory is always <directories.data_dir>/wikipedia, inside AuraGo's data
// mount, so free-space checks measure the right filesystem.
func SettingsFromConfig(cfg *config.Config) Settings {
	if cfg == nil {
		return Settings{Variant: VariantNoPic}
	}
	section := cfg.LocalWikipedia
	locked := cfg.Runtime.IsDocker
	dir := strings.TrimSpace(section.DataDir)
	if locked || dir == "" {
		dir = filepath.Join(cfg.Directories.DataDir, "wikipedia")
	}
	return Settings{
		Enabled:        section.Enabled,
		AgentAccess:    section.AgentAccess,
		Language:       ResolveLanguage(section.Language, cfg.Agent.SystemLanguage),
		SystemLanguage: ResolveLanguage("", cfg.Agent.SystemLanguage),
		Variant:        normalizeVariant(section.Variant),
		DataDir:        filepath.Clean(dir),
		DataDirLocked:  locked,
		UpdateCheck:    section.UpdateCheck,
	}
}

func normalizeVariant(raw string) Variant {
	if Variant(strings.ToLower(strings.TrimSpace(raw))) == VariantMaxi {
		return VariantMaxi
	}
	return VariantNoPic
}
