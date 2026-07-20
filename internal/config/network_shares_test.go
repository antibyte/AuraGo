package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/networkshares"
)

func TestNetworkSharesSafeDefaultsAndRuntimeOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("agent:\n  system_language: Deutsch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.NetworkShares.Enabled || !cfg.NetworkShares.ReadOnly ||
		cfg.NetworkShares.AllowCreate || cfg.NetworkShares.AllowUpdate || cfg.NetworkShares.AllowDelete {
		t.Fatalf("unsafe network share defaults = %+v", cfg.NetworkShares)
	}
	if !cfg.NetworkShares.SMB.Enabled || !cfg.NetworkShares.NFS.Enabled {
		t.Fatalf("protocol defaults = %+v", cfg.NetworkShares)
	}
	wantLedger := filepath.Join(filepath.Dir(path), "data", "network_shares.db")
	if cfg.SQLite.NetworkSharesPath != wantLedger {
		t.Fatalf("ledger path = %q, want %q", cfg.SQLite.NetworkSharesPath, wantLedger)
	}

	cfg.Runtime.NetworkShares = networkshares.Status{Usable: true}
	availability := ComputeFeatureAvailability(cfg.Runtime, false)
	if !availability["network_shares"].Available {
		t.Fatalf("network share availability = %+v", availability["network_shares"])
	}
	options := NetworkSharesOptions(cfg, "vault-secret")
	if options.Enabled || !options.ReadOnly || options.SudoPassword != "vault-secret" {
		t.Fatalf("runtime options = %+v", options)
	}
}

func TestNetworkSharesConfigRejectsInvalidRootsAndClients(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"network_shares:\n  allowed_roots: [relative]\n",
		"network_shares:\n  allowed_roots:\n    - " + root + "\n    - " + root + "\n",
		"network_shares:\n  allowed_roots: []\n  nfs:\n    allowed_clients: ['*']\n",
		"network_shares:\n  allowed_roots: []\n  nfs:\n    allowed_clients: [192.0.2.42/24, 192.0.2.0/24]\n",
	}
	for index, content := range cases {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(content, `\`, `/`)), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("case %d unexpectedly loaded", index)
		}
	}
}

func TestNetworkSharesCapabilitiesCombinePolicyAndProtocolRuntime(t *testing.T) {
	cfg := &Config{}
	cfg.NetworkShares.Enabled = true
	cfg.NetworkShares.AllowCreate = true
	cfg.NetworkShares.AllowUpdate = true
	cfg.NetworkShares.AllowDelete = true
	cfg.NetworkShares.SMB.Enabled = true
	cfg.Runtime.NetworkShares.Usable = true
	cfg.Runtime.NetworkShares.NFS.Writable = true

	capabilities := ComputeNetworkSharesCapabilities(cfg)
	if !capabilities.Enabled {
		t.Fatal("readable integration should be enabled")
	}
	if capabilities.AllowCreate || capabilities.AllowUpdate || capabilities.AllowDelete {
		t.Fatalf("disabled NFS protocol granted mutations: %+v", capabilities)
	}

	cfg.Runtime.NetworkShares.SMB.Writable = true
	capabilities = ComputeNetworkSharesCapabilities(cfg)
	if !capabilities.AllowCreate || !capabilities.AllowUpdate || !capabilities.AllowDelete {
		t.Fatalf("writable enabled SMB protocol did not grant configured mutations: %+v", capabilities)
	}

	cfg.NetworkShares.ReadOnly = true
	capabilities = ComputeNetworkSharesCapabilities(cfg)
	if capabilities.AllowCreate || capabilities.AllowUpdate || capabilities.AllowDelete {
		t.Fatalf("read-only mode did not override granular permissions: %+v", capabilities)
	}
}
