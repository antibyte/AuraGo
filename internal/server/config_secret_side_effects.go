package server

import (
	"strings"

	"aurago/internal/security"
	"aurago/internal/tsnetnode"
)

// configSecretSideEffects carries the secrets of a PUT /api/config patch whose
// vault POST counterpart (handleSetVaultSecret) does more than write the vault.
type configSecretSideEffects struct {
	mqttPassword string
	tsnetAuthKey string
}

// patchSecretString returns the trimmed string at path when it is a real new
// value (not empty, not the mask).
func patchSecretString(patch map[string]interface{}, path ...string) (string, bool) {
	current := patch
	for _, part := range path[:len(path)-1] {
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return "", false
		}
		current = next
	}
	raw, ok := current[path[len(path)-1]].(string)
	if !ok {
		return "", false
	}
	value := strings.TrimSpace(raw)
	if value == "" || value == draftConfigMask {
		return "", false
	}
	return value, true
}

// prepareConfigSecretSideEffects validates the secrets of a patch that need
// more than a vault write. It must run BEFORE anything is written: an invalid
// tsnet auth key rejects the whole save (HTTP 400) with nothing persisted.
func prepareConfigSecretSideEffects(patch map[string]interface{}) (configSecretSideEffects, error) {
	var effects configSecretSideEffects
	if value, ok := patchSecretString(patch, "tailscale", "tsnet", "auth_key"); ok {
		if err := validateTsNetAuthKey(value); err != nil {
			return effects, err
		}
		security.RegisterSensitive(value)
		effects.tsnetAuthKey = value
	}
	if value, ok := patchSecretString(patch, "mqtt", "password"); ok {
		security.RegisterSensitive(value)
		effects.mqttPassword = value
	}
	return effects, nil
}

// applyConfigSecretSideEffects runs AFTER the config was written and the reload
// attempted, so a rejected save never leaves side effects behind. The caller
// holds CfgSaveMu and no CfgMu.
//
// A successful reload already republishes the vault-backed MQTT password and
// the tsnet auth key with the new snapshot (ApplyVaultSecrets) and hands that
// snapshot to the tsnet manager. What it does not do is flag the nodes whose
// credential changed (vault POST: applyTsNetCredentialMutation) and, when the
// reload failed, republish the MQTT password.
func (s *Server) applyConfigSecretSideEffects(effects configSecretSideEffects, reloaded bool) error {
	if effects.mqttPassword != "" && !reloaded {
		if err := s.refreshMQTTPassword(); err != nil {
			return err
		}
	}
	if effects.tsnetAuthKey != "" && s.TsNetManager != nil {
		for _, node := range []tsnetnode.NodeID{tsnetnode.NodeMain, tsnetnode.NodeManifest, tsnetnode.NodeSpaceAgent} {
			s.TsNetManager.MarkCredentialChanged(node)
		}
	}
	return nil
}
