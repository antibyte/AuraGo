package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/evomap"
	"aurago/internal/security"
	"gopkg.in/yaml.v3"
)

// registerEvomapNode owns the complete transaction. Keep network work outside
// CfgMu; CfgSaveMu serializes it with all other configuration/Vault writers.
func (s *Server) registerEvomapNode(ctx context.Context, expected config.EvomapConfig) (string, string, bool, error) {
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Evomap.Enabled || cfg.Evomap.ReadOnly || expected.ReadOnly {
		return "", "", false, fmt.Errorf("EvoMap registration requires an enabled writable integration")
	}
	if cfg.Evomap != expected {
		return "", "", false, fmt.Errorf("EvoMap configuration changed; retry from the current configuration")
	}
	if s.Vault == nil || strings.TrimSpace(cfg.ConfigPath) == "" {
		return "", "", false, fmt.Errorf("EvoMap registration requires Vault and a saved configuration")
	}
	if err := ctx.Err(); err != nil {
		return "", "", false, err
	}
	original, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		return "", "", false, fmt.Errorf("read EvoMap configuration: %w", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(original, &raw); err != nil {
		return "", "", false, fmt.Errorf("parse EvoMap configuration: %w", err)
	}
	section, ok := raw["evomap"].(map[string]interface{})
	if !ok {
		return "", "", false, fmt.Errorf("saved EvoMap configuration is missing")
	}
	client, err := newEvomapServerClient(cfg.Evomap)
	if err != nil {
		return "", "", false, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, time.Duration(max(5, min(60, cfg.Evomap.RequestTimeoutSeconds)))*time.Second)
	defer cancel()
	result, err := client.RegisterNode(requestCtx, evomap.RegisterRequest{Capabilities: []string{"status", "fetch_capsules", "get_asset", "kg_query"}, Metadata: map[string]interface{}{"client": "aurago"}})
	if err != nil {
		return "", "", false, fmt.Errorf("register EvoMap node (remote outcome may be uncertain): %w", err)
	}
	nodeID := strings.TrimSpace(result.NodeID)
	if nodeID == "" {
		return "", "", false, fmt.Errorf("EvoMap registration returned no node ID")
	}
	security.RegisterSensitive(result.NodeSecret)
	section["node_id"] = nodeID
	output, err := yaml.Marshal(raw)
	if err != nil {
		return "", "", false, fmt.Errorf("encode EvoMap configuration: %w", err)
	}
	var mutations []vaultMutation
	if strings.TrimSpace(result.NodeSecret) != "" {
		mutations = append(mutations, vaultMutation{key: "evomap_node_secret", value: result.NodeSecret})
	}
	snapshots, err := applyVaultMutations(s.Vault, mutations)
	if err != nil {
		return "", "", false, fmt.Errorf("store registered EvoMap node secret: %w", err)
	}
	if err := config.WriteFileAtomic(cfg.ConfigPath, output, 0o600); err != nil {
		return "", "", false, errors.Join(fmt.Errorf("save registered EvoMap node: %w", err), restoreVaultSecrets(s.Vault, snapshots))
	}
	next, err := config.Load(cfg.ConfigPath)
	if err != nil {
		return "", "", false, errors.Join(fmt.Errorf("reload EvoMap configuration: %w", err), config.WriteFileAtomic(cfg.ConfigPath, original, 0o600), restoreVaultSecrets(s.Vault, snapshots))
	}
	next.ConfigPath = cfg.ConfigPath
	next.ApplyVaultSecrets(s.Vault)
	next.ApplyOAuthTokens(s.Vault)
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()
	return nodeID, result.ClaimURL, strings.TrimSpace(next.Evomap.NodeSecret) != "", nil
}
