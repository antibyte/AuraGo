package server

import "aurago/internal/config"

// injectCYDCreatorDefaults shows the loaded cyd_creator values on the config
// page when the YAML omits them (pattern injectGameMakerDefaults); values the
// file already has are kept.
func injectCYDCreatorDefaults(rawCfg map[string]interface{}, cfg *config.Config) {
	if cfg == nil {
		return
	}
	c := cfg.CYDCreator
	section, ok := rawCfg["cyd_creator"].(map[string]interface{})
	if !ok {
		section = make(map[string]interface{})
		rawCfg["cyd_creator"] = section
	}
	setDefaultBool(section, "enabled", c.Enabled)
	for key, value := range map[string]string{"builder_image": c.BuilderImage, "model": c.Model} {
		if _, exists := section[key]; !exists {
			section[key] = value
		}
	}
	setDefaultInt(section, "build_timeout_seconds", c.BuildTimeoutSeconds)
	setDefaultInt(section, "cold_build_timeout_seconds", c.ColdBuildTimeoutSeconds)
	setDefaultInt(section, "memory_mb", c.MemoryMB)
	if _, exists := section["cpu_cores"]; !exists {
		section["cpu_cores"] = c.CPUCores
	}
	setDefaultInt(section, "idle_stop_minutes", c.IdleStopMinutes)
	setDefaultInt(section, "max_repair_attempts", c.MaxRepairAttempts)
	setDefaultInt(section, "job_timeout_minutes", c.JobTimeoutMinutes)
	setDefaultInt(section, "keep_builds_per_project", c.KeepBuildsPerProject)
	ota, ok := section["ota"].(map[string]interface{})
	if !ok {
		ota = make(map[string]interface{})
		section["ota"] = ota
	}
	setDefaultBool(ota, "enabled", c.OTA.Enabled)
	setDefaultInt(ota, "heartbeat_seconds", c.OTA.HeartbeatSeconds)
	if _, exists := ota["public_url"]; !exists {
		ota["public_url"] = c.OTA.PublicURL
	}
}
