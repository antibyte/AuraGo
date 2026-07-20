package config

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aurago/internal/networkshares"
)

// NetworkSharesCapabilities is the single policy/runtime gate used by all
// agent-facing tool transports.
type NetworkSharesCapabilities struct {
	Enabled     bool
	AllowCreate bool
	AllowUpdate bool
	AllowDelete bool
}

// ComputeNetworkSharesCapabilities combines saved policy with the passive host
// probe. It intentionally exposes no mutation when no enabled protocol is
// writable.
func ComputeNetworkSharesCapabilities(cfg *Config) NetworkSharesCapabilities {
	if cfg == nil {
		return NetworkSharesCapabilities{}
	}
	enabled := cfg.NetworkShares.Enabled && cfg.Runtime.NetworkShares.Usable
	writable := enabled && !cfg.NetworkShares.ReadOnly &&
		((cfg.NetworkShares.SMB.Enabled && cfg.Runtime.NetworkShares.SMB.Writable) ||
			(cfg.NetworkShares.NFS.Enabled && cfg.Runtime.NetworkShares.NFS.Writable))
	return NetworkSharesCapabilities{
		Enabled:     enabled,
		AllowCreate: writable && cfg.NetworkShares.AllowCreate,
		AllowUpdate: writable && cfg.NetworkShares.AllowUpdate,
		AllowDelete: writable && cfg.NetworkShares.AllowDelete,
	}
}

// NormalizeNetworkSharesConfig validates durable policy without requiring roots
// or operating-system backends to be available during config loading.
func NormalizeNetworkSharesConfig(cfg *NetworkSharesConfig) error {
	if cfg == nil {
		return nil
	}
	if err := networkshares.ValidateConfiguredRoots(cfg.AllowedRoots); err != nil {
		return err
	}
	for index := range cfg.AllowedRoots {
		cfg.AllowedRoots[index] = filepath.Clean(strings.TrimSpace(cfg.AllowedRoots[index]))
	}

	principals, err := normalizeNetworkShareValues(cfg.SMB.AllowedPrincipals, true, "network_shares.smb.allowed_principals")
	if err != nil {
		return err
	}
	cfg.SMB.AllowedPrincipals = principals

	seenClients := make(map[string]struct{}, len(cfg.NFS.AllowedClients))
	clients := make([]string, 0, len(cfg.NFS.AllowedClients))
	for _, raw := range cfg.NFS.AllowedClients {
		client, ok := networkshares.CanonicalClient(raw)
		if !ok {
			return fmt.Errorf("network_shares.nfs.allowed_clients contains invalid address or CIDR %q", raw)
		}
		if _, duplicate := seenClients[client]; duplicate {
			return fmt.Errorf("network_shares.nfs.allowed_clients contains duplicate client %q", raw)
		}
		seenClients[client] = struct{}{}
		clients = append(clients, client)
	}
	sort.Strings(clients)
	cfg.NFS.AllowedClients = clients
	return nil
}

func normalizeNetworkShareValues(values []string, caseInsensitive bool, field string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("%s entries must not be empty", field)
		}
		key := value
		if caseInsensitive || runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%s contains duplicate entry %q", field, raw)
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

// NetworkSharesOptions maps persisted policy and runtime restrictions into the
// host subsystem. SudoPassword is intentionally supplied separately from Vault.
func NetworkSharesOptions(cfg *Config, sudoPassword string) networkshares.Options {
	if cfg == nil {
		return networkshares.Options{}
	}
	return networkshares.Options{
		Enabled:              cfg.NetworkShares.Enabled,
		ReadOnly:             cfg.NetworkShares.ReadOnly,
		AllowCreate:          cfg.NetworkShares.AllowCreate,
		AllowUpdate:          cfg.NetworkShares.AllowUpdate,
		AllowDelete:          cfg.NetworkShares.AllowDelete,
		AllowedRoots:         append([]string(nil), cfg.NetworkShares.AllowedRoots...),
		SMBEnabled:           cfg.NetworkShares.SMB.Enabled,
		SMBAllowGuest:        cfg.NetworkShares.SMB.AllowGuest,
		SMBAllowedPrincipals: append([]string(nil), cfg.NetworkShares.SMB.AllowedPrincipals...),
		NFSEnabled:           cfg.NetworkShares.NFS.Enabled,
		NFSAllowedClients:    append([]string(nil), cfg.NetworkShares.NFS.AllowedClients...),
		IsDocker:             cfg.Runtime.IsDocker,
		SudoEnabled:          cfg.Agent.SudoEnabled,
		SudoUnrestricted:     cfg.Agent.SudoUnrestricted,
		NoNewPrivileges:      cfg.Runtime.NoNewPrivileges,
		ProtectSystemStrict:  cfg.Runtime.ProtectSystemStrict,
		SudoPassword:         sudoPassword,
	}
}
