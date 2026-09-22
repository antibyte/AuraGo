package server

import "aurago/internal/config"

// Headless eggs do not run integration bots. Keep this gate on every update,
// including Vault mutations, rather than only on the initial startup path.
func mqttRuntimeSnapshot(cfg *config.Config) *config.Config {
	if cfg == nil || !cfg.EggMode.Enabled {
		return cfg
	}
	copyCfg := *cfg
	copyCfg.MQTT.Enabled = false
	return &copyCfg
}
