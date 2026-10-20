package config

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
)

// Legacy keys never imply the helper provider's identity. Ambiguous values are
// escrowed separately, without an agent grant or an active provider binding.
func migratePersonalityLegacySecrets(root map[string]interface{}, vault SecretReadWriter, log *slog.Logger) bool {
	changed := false
	for _, path := range [][3]string{{"personality", "v2_api_key", "v2_provider"}, {"agent", "personality_v2_api_key", "personality_v2_provider"}} {
		section, _ := root[path[0]].(map[string]interface{})
		if section == nil {
			continue
		}
		raw, present := section[path[1]]
		if !present {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			if log != nil {
				log.Warn("[Config] Legacy personality credential remains in YAML because its value is not a string", "field", path[0]+"."+path[1])
			}
			continue
		}
		if strings.TrimSpace(value) == "" {
			delete(section, path[1])
			changed = true
			continue
		}
		ref, _ := section[path[2]].(string)
		urlKey := "v2_url"
		if path[0] == "agent" {
			urlKey = "personality_v2_url"
		}
		legacyURL, _ := section[urlKey].(string)
		matches := 0
		providerID := ""
		providers, _ := root["providers"].([]interface{})
		for _, item := range providers {
			provider, _ := item.(map[string]interface{})
			id, _ := provider["id"].(string)
			base, _ := provider["base_url"].(string)
			if ref != "" && id == ref && (legacyURL == "" || strings.TrimRight(base, "/") == strings.TrimRight(legacyURL, "/")) {
				matches++
				providerID = id
			}
		}
		saved := false
		if matches == 1 {
			saved = storePersonalitySecretWithoutOverwrite(vault, "provider_"+providerID+"_api_key", value)
		}
		if !saved {
			sum := sha256.Sum256([]byte(value))
			slot := fmt.Sprintf("personality_legacy_%s_%s_%x", path[0], path[1], sum[:16])
			saved = storePersonalitySecretWithoutOverwrite(vault, slot, value)
		}
		if saved {
			delete(section, path[1])
			changed = true
		} else if log != nil {
			log.Warn("[Config] Legacy personality credential remains in YAML because Vault storage failed", "field", path[0]+"."+path[1])
		}
	}
	return changed
}

func storePersonalitySecretWithoutOverwrite(vault SecretReadWriter, key, value string) bool {
	existing, err := vault.ReadSecret(key)
	if err == nil && existing == value {
		return true
	}
	if err == nil && existing != "" {
		return false
	}
	if atomic, ok := vault.(interface {
		WriteUserSecret(string, string, bool) error
	}); ok {
		return atomic.WriteUserSecret(key, value, false) == nil
	}
	// Generic stores cannot safely infer absence from a failed read.
	if err != nil {
		return false
	}
	return vault.WriteSecret(key, value) == nil
}

// ValidatePersonalityLegacyPatch runs before any Vault or file mutation.
func ValidatePersonalityLegacyPatch(patch map[string]interface{}) error {
	for _, path := range [][2]string{{"personality", "v2_api_key"}, {"agent", "personality_v2_api_key"}} {
		if section, ok := patch[path[0]].(map[string]interface{}); ok {
			if _, present := section[path[1]]; present {
				return fmt.Errorf("%s.%s is retired; save credentials through the provider settings", path[0], path[1])
			}
		}
		if _, present := patch[path[0]+"."+path[1]]; present {
			return fmt.Errorf("legacy personality credentials are retired; use the provider settings")
		}
	}
	return nil
}
