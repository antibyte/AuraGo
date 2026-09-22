package server

import (
	"fmt"

	"aurago/internal/config"
	"aurago/internal/mqtt"

	"gopkg.in/yaml.v3"
)

// validateMQTTConfigPatch checks the candidate before any credential or file writes.
// Unmarshal into a fresh value so rejected edits cannot mutate a published snapshot.
func validateMQTTConfigPatch(raw, patch map[string]interface{}) error {
	section, changed := patch["mqtt"]
	if !changed {
		return nil
	}
	data, err := yaml.Marshal(map[string]interface{}{"mqtt": raw["mqtt"]})
	if err != nil {
		return fmt.Errorf("mqtt: encode current settings: %w", err)
	}
	var candidate map[string]interface{}
	if err := yaml.Unmarshal(data, &candidate); err != nil {
		return fmt.Errorf("mqtt: decode current settings: %w", err)
	}
	deepMerge(candidate, map[string]interface{}{"mqtt": section}, "")
	data, err = yaml.Marshal(candidate)
	if err != nil {
		return fmt.Errorf("mqtt: encode candidate settings: %w", err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("mqtt: invalid settings: %w", err)
	}
	if err := mqtt.ValidateConfig(&cfg); err != nil {
		return fmt.Errorf("mqtt: %w", err)
	}
	return nil
}
