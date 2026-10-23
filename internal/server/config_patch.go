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
	if rejectWebAuthnConfigPatch(s.ConfigSnapshot(), patch) {
		return nil, fmt.Errorf("change WebAuthn policy through /api/auth/webauthn/settings")
	}
	if err := config.ValidatePersonalityLegacyPatch(patch); err != nil {
		return nil, err
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
	// The cyd_creator values before the merge: the save validates only the
	// values it changes (validateCYDCreatorSave).
	previousCYDCreator := cydCreatorSectionFromRaw(rawCfg)

	providerMutations, err := prepareSetupProviders(s, patch)
	if err != nil {
		return nil, err
	}

	// Deep merge the patch into existing config.
	deepMerge(rawCfg, patch, "")
	rawCfg = normalizeConfigYAMLMap(rawCfg)

	// Write back atomically.
	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	validateCfg := *s.ConfigSnapshot().Clone()
	if err := config.ValidateCloudflareTunnelPortYAML(out); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(out, &validateCfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	if _, providersChanged := patch["providers"]; providersChanged {
		if err := validateProviderEntries(s.Cfg, &validateCfg); err != nil {
			return nil, err
		}
	}
	if err := validateNewProviderReferences(s.Cfg, &validateCfg); err != nil {
		return nil, err
	}
	// config.Load rejects invalid detection settings, so AuraGo would not start.
	if err := validateGo2RTCDetectionSettings(validateCfg.Go2RTC); err != nil {
		return nil, err
	}
	if err := config.ValidateCloudflareTunnelConfig(&validateCfg); err != nil {
		return nil, err
	}
	if err := config.ValidateActionPolicy(validateCfg.Agent.ActionPolicy); err != nil {
		return nil, err
	}
	if err := config.ValidateLocalLLMConfig(&validateCfg); err != nil {
		return nil, err
	}
	if err := config.NormalizeVideoStudioConfig(&validateCfg.VideoStudio); err != nil {
		return nil, err
	}
	if err := validateCYDCreatorSave(previousCYDCreator, validateCfg.CYDCreator); err != nil {
		return nil, err
	}
	if err := config.NormalizeNewspaperConfig(&validateCfg.Newspaper); err != nil {
		return nil, err
	}
	if err := config.ValidateLocalMusicConfig(&validateCfg); err != nil {
		return nil, err
	}
	if err := config.ValidateMediaConversionConfig(&validateCfg.Tools.MediaConversion); err != nil {
		return nil, err
	}
	if (s.Cfg.MusicGeneration.Enabled && s.Cfg.UsesLocalMusic() || localMusicStopPending(s)) && (!validateCfg.Docker.Enabled || validateCfg.Docker.ReadOnly) {
		return nil, fmt.Errorf("disable local music and wait until stopped before disabling Docker mutations")
	}
	// Same invariant as /api/config and Start: a remotely reachable listener
	// never runs without login unless the unsafe exception is explicit.
	if err := validateRemoteAuthExposure(&validateCfg); err != nil {
		return nil, err
	}
	_, cfExplicit := patch["cloudflare_tunnel"]
	cfFinish, cfErr := prepareCloudflareConfigSave(s, &validateCfg, cfExplicit)
	if cfErr != nil {
		return nil, cfErr
	}
	defer cfFinish(false)
	secretMutations, err := prepareConfigSecrets(patch, s.Logger)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return nil, err
	}
	deepMerge(rawCfg, patch, "")
	out, err = yaml.Marshal(normalizeConfigYAMLMap(rawCfg))
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	mutations := append(providerMutations, secretMutations...)
	reloaded, err := saveProviderConfig(s, configPath, data, out, mutations)
	if err != nil {
		return nil, err
	}
	cfFinish(true)
	return reloaded, nil
}
