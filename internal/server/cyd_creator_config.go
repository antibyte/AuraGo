package server

import (
	"errors"
	"math"
	"strings"

	"aurago/internal/config"

	"gopkg.in/yaml.v3"
)

// cydCreatorSectionFromRaw decodes the cyd_creator section of a raw
// config.yaml map (a missing or undecodable section is the zero value). Both
// save paths read it before merging the patch, as the baseline of
// validateCYDCreatorSave.
func cydCreatorSectionFromRaw(raw map[string]interface{}) config.CYDCreatorConfig {
	var section config.CYDCreatorConfig
	data, err := yaml.Marshal(raw["cyd_creator"])
	if err != nil {
		return section
	}
	if err := yaml.Unmarshal(data, &section); err != nil {
		return config.CYDCreatorConfig{}
	}
	return section
}

// validateCYDCreatorSave validates the cyd_creator values a config save
// changes, compared with config.yaml before the save (previous), like
// validateLocalWikipediaSave: config.Load already clamps a hand-edited
// out-of-range value, replaces an invalid builder_image and ignores an
// unusable public_url, so a value the save leaves as it was is not checked
// again and never blocks saving other sections, while every value a save sets
// is. The cold-versus-warm timeout rule runs when either timeout changed.
func validateCYDCreatorSave(previous, candidate config.CYDCreatorConfig) error {
	changed := candidate
	if strings.TrimSpace(previous.BuilderImage) == strings.TrimSpace(candidate.BuilderImage) {
		changed.BuilderImage = ""
	}
	buildSame := previous.BuildTimeoutSeconds == candidate.BuildTimeoutSeconds
	coldSame := previous.ColdBuildTimeoutSeconds == candidate.ColdBuildTimeoutSeconds
	if buildSame {
		changed.BuildTimeoutSeconds = 0
	}
	if coldSame {
		changed.ColdBuildTimeoutSeconds = 0
	}
	if previous.MemoryMB == candidate.MemoryMB {
		changed.MemoryMB = 0
	}
	if previous.CPUCores == candidate.CPUCores || (math.IsNaN(previous.CPUCores) && math.IsNaN(candidate.CPUCores)) {
		changed.CPUCores = 0
	}
	if previous.IdleStopMinutes == candidate.IdleStopMinutes {
		changed.IdleStopMinutes = 0
	}
	if previous.MaxRepairAttempts == candidate.MaxRepairAttempts {
		changed.MaxRepairAttempts = 0
	}
	if previous.JobTimeoutMinutes == candidate.JobTimeoutMinutes {
		changed.JobTimeoutMinutes = 0
	}
	if previous.KeepBuildsPerProject == candidate.KeepBuildsPerProject {
		changed.KeepBuildsPerProject = 0
	}
	if previous.OTA.HeartbeatSeconds == candidate.OTA.HeartbeatSeconds {
		changed.OTA.HeartbeatSeconds = 0
	}
	if strings.TrimSpace(previous.OTA.PublicURL) == strings.TrimSpace(candidate.OTA.PublicURL) {
		changed.OTA.PublicURL = ""
	}
	if err := config.ValidateCYDCreatorConfig(changed); err != nil {
		return err
	}
	if (!buildSame || !coldSame) && candidate.BuildTimeoutSeconds != 0 && candidate.ColdBuildTimeoutSeconds != 0 &&
		candidate.ColdBuildTimeoutSeconds < candidate.BuildTimeoutSeconds {
		return errors.New("cyd_creator.cold_build_timeout_seconds must be at least build_timeout_seconds")
	}
	return nil
}

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
