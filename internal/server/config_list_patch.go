package server

import (
	"fmt"
	"log/slog"
	"strings"
)

// completeConfigListPaths are lists whose entries the config UI edits per index.
// A save replaces them wholesale (mergeConfigArrayValue), so the UI sends them
// complete; a sparse or partial copy would delete every entry and field it
// leaves out. go2rtc.streams has its own stricter check.
var completeConfigListPaths = []string{
	"a2a.server.skills",
	"a2a.client.remote_agents",
	"three_d_printers.elegoo_centauri_carbon.printers",
	"three_d_printers.klipper.printers",
}

// validateCompleteConfigListPatch rejects a list from completeConfigListPaths
// that is not a list of complete entries. Every entry the UI renders carries
// its id field, so an entry without one is a partial copy. An explicit empty
// list still removes every entry.
func validateCompleteConfigListPatch(patch map[string]interface{}) error {
	for _, path := range completeConfigListPaths {
		raw, present := configPatchValue(patch, path)
		if !present {
			continue
		}
		var items []interface{}
		switch list := raw.(type) {
		case []interface{}:
			items = list
		case map[string]interface{}:
			converted, dense := numericKeyedMapToSlice(list)
			if !dense {
				return fmt.Errorf("%s must be a list", path)
			}
			items = converted
		default:
			return fmt.Errorf("%s must be a list", path)
		}
		for index, item := range items {
			entry, ok := item.(map[string]interface{})
			if !ok || len(entry) == 0 {
				return fmt.Errorf("%s must list every entry: entry %d is empty", path, index+1)
			}
			if _, ok := entry["id"].(string); !ok {
				return fmt.Errorf("%s must list every entry: entry %d has no id", path, index+1)
			}
		}
	}
	return nil
}

func configPatchValue(patch map[string]interface{}, path string) (interface{}, bool) {
	keys := strings.Split(path, ".")
	current := patch
	for _, key := range keys[:len(keys)-1] {
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}
	value, present := current[keys[len(keys)-1]]
	return value, present
}

// extractA2ARemoteAgentSecretsToVault removes the per-agent credentials from a
// remote agent list before it replaces the saved one. The UI sends the masked
// placeholders the server reported; a real value is stored in the Vault under
// the agent id, as a2aSaveVault does.
func extractA2ARemoteAgentSecretsToVault(items []interface{}, vault configSecretWriter, logger *slog.Logger) error {
	var firstErr error
	for _, item := range items {
		agent, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		for _, field := range []string{"api_key", "bearer_token"} {
			raw, exists := agent[field]
			if !exists {
				continue
			}
			delete(agent, field)
			value, ok := raw.(string)
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			if value == "" || value == "••••••••" {
				continue
			}
			id, _ := agent["id"].(string)
			id = strings.TrimSpace(id)
			if id == "" {
				if firstErr == nil {
					firstErr = fmt.Errorf("credential 'a2a.client.remote_agents.%s' cannot be saved: agent id is required", field)
				}
				continue
			}
			if configSecretWriterMissing(vault) {
				if firstErr == nil {
					firstErr = fmt.Errorf("credential 'a2a.client.remote_agents.%s.%s' cannot be saved: no vault configured (AURAGO_MASTER_KEY required)", id, field)
				}
				continue
			}
			vaultKey := "a2a_remote_" + id + "_" + field
			if err := vault.WriteSecret(vaultKey, value); err != nil {
				if logger != nil {
					logger.Error("[Config] Failed to write A2A remote agent credential to vault", "key", vaultKey, "error", err)
				}
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to save credential %s for remote agent %q to vault: %w", field, id, err)
				}
				continue
			}
			if logger != nil {
				logger.Info("[Config] A2A remote agent credential accepted for Vault storage", "key", vaultKey)
			}
		}
	}
	return firstErr
}
