package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

// virtualComputersEnforceAgentControlGate prevents config from advertising
// workspace tools before a complete setup/repair verified boringd and both
// installed guest templates for the exact embedded asset fingerprint.
func virtualComputersEnforceAgentControlGate(s *Server, oldCfg, newCfg config.Config) error {
	if oldCfg.VirtualComputers.AgentControl.Enabled || !newCfg.VirtualComputers.AgentControl.Enabled {
		return nil
	}
	if !newCfg.VirtualComputers.Enabled {
		return fmt.Errorf("virtual_computers must be enabled before agent control")
	}
	if !newCfg.Tools.VirtualComputers.Enabled {
		return fmt.Errorf("tools.virtual_computers must be enabled before agent control")
	}
	if !virtualComputersWorkspaceSetupVerified() {
		return fmt.Errorf("workspace agent setup has not been verified for the current asset fingerprint; run Virtual Computers install/repair first")
	}
	cfg := virtualcomputers.FromAuraConfig(&newCfg)
	if strings.TrimSpace(cfg.BoringdURL) == "" {
		return fmt.Errorf("boringd URL is not configured")
	}
	if err := virtualComputersEnsureControlPlaneAccess(s, cfg); err != nil {
		return err
	}
	client, err := virtualcomputers.NewClient(virtualcomputers.ClientConfig{
		BaseURL: cfg.BoringdURL,
		Token:   cfg.BoringToken,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, err := client.WorkspaceCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("workspace capability probe failed: %w", err)
	}
	if status.ProtocolVersion != virtualcomputers.WorkspaceProtocolVersion || status.AssetFingerprint != virtualcomputers.WorkspaceAssetFingerprint() {
		return fmt.Errorf("workspace protocol or asset fingerprint does not match this AuraGo build")
	}
	return nil
}
