package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/config"
	"aurago/internal/security"
	"gopkg.in/yaml.v3"
)

// Validate the merged candidate before extracting any secrets into the Vault.
func validateAIGatewayConfigPatch(raw, patch map[string]interface{}) error {
	_, gatewayChanged := patch["ai_gateway"]
	_, providersChanged := patch["providers"]
	if !gatewayChanged && !providersChanged {
		return nil
	}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("invalid gateway configuration")
	}
	var candidate map[string]interface{}
	if err = yaml.Unmarshal(data, &candidate); err != nil {
		return fmt.Errorf("invalid gateway configuration")
	}
	candidate = normalizeConfigYAMLMap(candidate)
	deepMerge(candidate, patch, "")
	data, err = yaml.Marshal(candidate)
	if err != nil {
		return fmt.Errorf("invalid gateway configuration")
	}
	var cfg config.Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid gateway configuration")
	}
	if gatewayChanged {
		if err = config.ValidateAIGatewayConfig(&cfg); err != nil {
			return err
		}
		metadata, _ := json.Marshal(cfg.AIGateway.Metadata)
		if security.Scrub(string(metadata)) != string(metadata) {
			return fmt.Errorf("AI Gateway metadata must not contain credentials")
		}
		if section, ok := patch["ai_gateway"].(map[string]interface{}); ok {
			if token, ok := section["token"].(string); ok && token != "" {
				for key, value := range cfg.AIGateway.Metadata {
					if strings.Contains(value, token) || strings.Contains(key, token) {
						return fmt.Errorf("AI Gateway metadata must not contain credentials")
					}
				}
			}
		}
	}
	if providersChanged {
		for _, provider := range cfg.Providers {
			if err = config.ValidateGatewayProvider(provider); err != nil {
				return err
			}
		}
	}
	return nil
}
