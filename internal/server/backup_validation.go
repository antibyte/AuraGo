package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func agoIsAllowedRestorePath(clean string) bool {
	slash := filepath.ToSlash(filepath.Clean(clean))
	switch slash {
	case "config.yaml", "data", "prompts", "agent_workspace", "agent_workspace/skills", "agent_workspace/tools", "agent_workspace/workdir":
		return true
	}
	for _, prefix := range []string{
		"data/",
		"prompts/",
		"agent_workspace/skills/",
		"agent_workspace/tools/",
		"agent_workspace/workdir/",
	} {
		if strings.HasPrefix(slash, prefix) {
			return true
		}
	}
	return false
}

func agoConfigContainsPlaintextSecret(data []byte) bool {
	var root interface{}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}
	return agoYAMLNodeContainsSecret("", root)
}

func agoYAMLNodeContainsSecret(key string, value interface{}) bool {
	if agoIsSensitiveConfigKey(key) && agoSecretValueIsSet(value) {
		return true
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for childKey, childValue := range typed {
			if agoYAMLNodeContainsSecret(childKey, childValue) {
				return true
			}
		}
	case map[interface{}]interface{}:
		for childKey, childValue := range typed {
			if agoYAMLNodeContainsSecret(fmt.Sprint(childKey), childValue) {
				return true
			}
		}
	case []interface{}:
		for _, childValue := range typed {
			if agoYAMLNodeContainsSecret(key, childValue) {
				return true
			}
		}
	}
	return false
}

func agoIsSensitiveConfigKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	if normalized == "" || normalized == "token_budget" || strings.HasPrefix(normalized, "max_token") {
		return false
	}
	exact := map[string]bool{
		"api_key":        true,
		"apikey":         true,
		"access_key":     true,
		"secret_key":     true,
		"client_secret":  true,
		"bot_token":      true,
		"access_token":   true,
		"refresh_token":  true,
		"bearer_token":   true,
		"password":       true,
		"password_hash":  true,
		"session_secret": true,
		"shared_key":     true,
		"private_key":    true,
		"deploy_key":     true,
		"auth_key":       true,
		"user_key":       true,
		"master_key":     true,
	}
	if exact[normalized] {
		return true
	}
	return strings.HasSuffix(normalized, "_api_key") ||
		strings.HasSuffix(normalized, "_token") ||
		strings.HasSuffix(normalized, "_password") ||
		strings.HasSuffix(normalized, "_secret")
}

func agoSecretValueIsSet(value interface{}) bool {
	switch typed := value.(type) {
	case string:
		v := strings.TrimSpace(typed)
		if v == "" || strings.Contains(v, "••") {
			return false
		}
		lower := strings.ToLower(v)
		if lower == "changeme" || lower == "change-me" || lower == "placeholder" || lower == "none" || lower == "null" {
			return false
		}
		if strings.HasPrefix(v, "${") || strings.HasPrefix(v, "{{") || strings.HasPrefix(lower, "vault:") {
			return false
		}
		return true
	case nil:
		return false
	case map[string]interface{}, map[interface{}]interface{}, []interface{}:
		return false
	default:
		return fmt.Sprint(value) != ""
	}
}
