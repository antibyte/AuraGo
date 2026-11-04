package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/restorepending"
)

func TestRestoreFlowKeysRequireMatchingDatabase(t *testing.T) {
	for _, paired := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial", true: "paired"}[paired], func(t *testing.T) {
			s, root := newPendingTestServer(t)
			s.Cfg.SQLite.GameMakerPath = filepath.Join(root, "data", "custom", "game_maker.db")
			s.Vault = newTestVault(t, filepath.Join(root, "data", "vault.bin"), "06")
			source := newTestVault(t, filepath.Join(t.TempDir(), "vault.bin"), "05")
			for _, key := range []string{flowCheckpointVaultKey, flowScenarioVaultKey} {
				if err := s.Vault.WriteSecret(key, "local-key"); err != nil {
					t.Fatal(err)
				}
				if err := source.WriteSecret(key, "backup-key"); err != nil {
					t.Fatal(err)
				}
			}
			blob, err := exportVaultSecrets(source, "pw")
			if err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{"data/state.json": []byte("restored"), "vault_secrets.enc": blob}
			if paired {
				files["data/custom/flows.db"] = makeUserVersionDB(t, 0)
				files["config.yaml"] = []byte("sqlite:\n  game_maker_path: ./data/custom/game_maker.db\n")
			}
			rec := importArchiveWithPassword(t, s, files, "pw")
			if rec.Code != 200 {
				t.Fatalf("import = %d %s", rec.Code, rec.Body.String())
			}
			var result struct {
				WarningCodes []string `json:"warning_codes"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			warned := strings.Contains(strings.Join(result.WarningCodes, " "), "flow_keys_skipped")
			if warned == paired {
				t.Fatalf("pairing warning = %v", result.WarningCodes)
			}
			if paired {
				if err := os.WriteFile(s.Cfg.ConfigPath, []byte("sqlite:\n  game_maker_path: ./data/later/game_maker.db\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			applyPendingRestoreForTest(t, s.Cfg.ConfigPath)
			restorepending.ApplyStagedVault(root, s.Vault, nil)
			if paired {
				restoredConfig, err := os.ReadFile(s.Cfg.ConfigPath)
				if err != nil || !strings.Contains(string(restoredConfig), "./data/custom/game_maker.db") {
					t.Fatal("restore did not apply the paired configuration", err)
				}
			}
			want := "local-key"
			if paired {
				want = "backup-key"
			}
			for _, key := range []string{flowCheckpointVaultKey, flowScenarioVaultKey} {
				got, err := s.Vault.ReadSecret(key)
				if err != nil || got != want {
					t.Fatal("incorrect restored flow key", err)
				}
			}
		})
	}
}

func TestRestoreFlowDatabaseAndDifferentKeysRequiresConfig(t *testing.T) {
	s, root := newPendingTestServer(t)
	s.Cfg.SQLite.GameMakerPath = filepath.Join(root, "data", "custom", "game_maker.db")
	s.Vault = newTestVault(t, filepath.Join(root, "data", "vault.bin"), "06")
	source := newTestVault(t, filepath.Join(t.TempDir(), "vault.bin"), "05")
	if err := s.Vault.WriteSecret(flowScenarioVaultKey, "local-key"); err != nil {
		t.Fatal(err)
	}
	if err := source.WriteSecret(flowScenarioVaultKey, "backup-key"); err != nil {
		t.Fatal(err)
	}
	blob, err := exportVaultSecrets(source, "pw")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"data/custom/flows.db": makeUserVersionDB(t, 0), "vault_secrets.enc": blob}
	rec := importArchiveWithPassword(t, s, files, "pw")
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "flow_key_pairing_required") {
		t.Fatalf("unpaired database = %d %s", rec.Code, rec.Body.String())
	}
	if restorepending.ReadState(root).Pending {
		t.Fatal("unsafe partial restore was staged")
	}
	got, err := s.Vault.ReadSecret(flowScenarioVaultKey)
	if err != nil || got != "local-key" {
		t.Fatal("live key changed after rejected restore", err)
	}
	if err := source.WriteSecret(flowScenarioVaultKey, "local-key"); err != nil {
		t.Fatal(err)
	}
	files["vault_secrets.enc"], err = exportVaultSecrets(source, "pw")
	if err != nil {
		t.Fatal(err)
	}
	rec = importArchiveWithPassword(t, s, files, "pw")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "flow_keys_skipped") {
		t.Fatalf("same-key partial restore = %d %s", rec.Code, rec.Body.String())
	}
}

func TestRestoreFlowDatabasePairingUsesRestoredConfig(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(stage, []byte("sqlite:\n  game_maker_path: ./data/old/../custom/game_maker.db\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ConfigPath: filepath.Join(root, "config.yaml")}
	cfg.SQLite.GameMakerPath = filepath.Join(root, "data", "current", "game_maker.db")
	entries := []agoImportEntry{{dest: cfg.ConfigPath, stage: stage}, {dest: filepath.Join(root, "data", "custom", ".", "flows.db")}}
	if !restoreHasFlowsDatabase(cfg, entries) {
		t.Fatal("normalized restored configuration did not match its database")
	}
	entries[1].dest = filepath.Join(root, "data", "current", "flows.db")
	if restoreHasFlowsDatabase(cfg, entries) {
		t.Fatal("old configuration database incorrectly accepted")
	}
}
