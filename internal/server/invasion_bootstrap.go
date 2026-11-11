package server

import (
	"encoding/json"
	"fmt"

	"aurago/internal/config"
	"aurago/internal/invasion"
	"aurago/internal/security"
)

func (s *Server) eggBootstrap(cfg *config.Config, egg invasion.EggRecord, sharedKey, exportKey string, vaultData []byte) ([]byte, error) {
	secrets := map[string]string{"provider_main_api_key": ""}
	if sharedKey != "" {
		secrets["egg_shared_key"] = sharedKey
	}
	if egg.InheritLLM {
		secrets["provider_main_api_key"] = cfg.LLM.APIKey
	} else if egg.APIKeyRef != "" {
		if s.Vault == nil {
			return nil, fmt.Errorf("Egg provider Vault unavailable")
		}
		key, err := s.Vault.ReadSecret(egg.APIKeyRef)
		if err != nil {
			return nil, fmt.Errorf("read Egg provider secret: %w", err)
		}
		secrets["provider_main_api_key"] = key
	}
	for _, value := range secrets {
		security.RegisterSensitive(value)
	}
	security.RegisterSensitive(exportKey)
	bootstrap := invasion.EggBootstrap{Version: invasion.EggSecurityPolicyVersion, Secrets: secrets}
	if len(vaultData) > 0 {
		bootstrap.ExportKey, bootstrap.VaultData = exportKey, vaultData
	}
	data, err := json.Marshal(bootstrap)
	if err != nil {
		return nil, fmt.Errorf("encode private Egg bootstrap: %w", err)
	}
	if len(data) > 10<<20 {
		return nil, fmt.Errorf("private Egg bootstrap exceeds 10 MiB")
	}
	return data, nil
}

func (s *Server) restoreEggPermissions(nest invasion.NestRecord, data []byte, patch invasion.SafeConfigPatch) ([]byte, error) {
	completeReview := patch.AllowedTools != nil && patch.AllowFilesystemWrite != nil && patch.AllowNetworkRequests != nil && patch.AllowUnsandboxedShell != nil
	if nest.AppliedConfigRev == "" || (nest.DesiredConfigRev != "" && nest.DesiredConfigRev != nest.AppliedConfigRev) {
		if !completeReview {
			return nil, fmt.Errorf("Egg permissions need explicit review before reconfiguration")
		}
		return data, nil
	}
	revision, err := invasion.GetSafeConfigRevision(s.InvasionDB, nest.AppliedConfigRev)
	if err != nil {
		return nil, err
	}
	if revision.PermissionsJSON == "" && completeReview {
		return data, nil
	}
	return invasion.ApplyEggPermissions(data, revision.PermissionsJSON)
}
