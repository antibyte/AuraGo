package server

import (
	"aurago/internal/config"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// applyConfigPatch reads the YAML config at s.Cfg.ConfigPath, deep-merges patch
// into it, extracts any sensitive credentials to the vault, writes back
// atomically, and returns the reloaded config in-memory.
//
// Currently used by /api/setup (the unauthenticated first-run wizard). It is
// designed as the future consolidation point for /api/config update as well —
// that handler still has its own inline implementation with extra pre-write
// validation that will be migrated separately.
//
// The patch is mutated: sensitive credential fields are stripped (moved to the
// vault), and `_setup_profile_id` is consumed and removed.
//
// Hot-reload of subsystems (budget tracker, LLM client, VectorDB) is the
// caller's responsibility — see handleSetupSave for the setup-specific path.
func applyConfigPatch(s *Server, patch map[string]interface{}) (*config.Config, error) {
	if s == nil {
		return nil, fmt.Errorf("applyConfigPatch: nil server")
	}
	if s.Cfg == nil {
		return nil, fmt.Errorf("applyConfigPatch: server has no config")
	}
	configPath := s.Cfg.ConfigPath
	if configPath == "" {
		return nil, fmt.Errorf("applyConfigPatch: config path not set")
	}

	// Apply any setup-profile-specific defaults (no-op for non-profile patches).
	applySetupProfileConfigPatch(patch, s)

	// Read current config file.
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	rawCfg = normalizeConfigYAMLMap(rawCfg)

	// Move any sensitive credentials into the vault before merging.
	if s.Vault != nil {
		if err := extractSecretsToVault(patch, s.Vault, s.Logger); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("[Config] Some credentials could not be saved to vault", "error", err)
			}
		}
	}

	// Deep merge the patch into existing config.
	deepMerge(rawCfg, patch, "")
	rawCfg = normalizeConfigYAMLMap(rawCfg)

	// Write back atomically.
	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Reload from disk and resolve vault secrets.
	reloaded, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("reload config: %w", err)
	}
	reloaded.ConfigPath = configPath
	reloaded.ApplyVaultSecrets(s.Vault)
	reloaded.ResolveProviders()
	return reloaded, nil
}