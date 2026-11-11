package invasion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestEggBootstrapPreservesVaultAndRetiresStaleInput(t *testing.T) {
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("unrelated", "bootstrap-retained-fixture"); err != nil {
		t.Fatal(err)
	}
	source, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "source.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := source.WriteSecret("selected", "bootstrap-selected-fixture"); err != nil {
		t.Fatal(err)
	}
	exported, exportKey, err := ExportVaultForEgg(source, []string{"selected"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(EggBootstrap{Version: 1, ExportKey: exportKey, VaultData: exported, Secrets: map[string]string{"egg_shared_key": strings.Repeat("c", 64), "provider_main_api_key": "bootstrap-provider-fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, EggBootstrapFile)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ApplyEggBootstrap(dir, vault); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"unrelated": "bootstrap-retained-fixture", "selected": "bootstrap-selected-fixture", "provider_main_api_key": "bootstrap-provider-fixture", "egg_shared_key": strings.Repeat("c", 64)} {
		got, err := vault.ReadSecret(key)
		if err != nil || got != want {
			t.Fatalf("secret %s was not merged correctly", key)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("bootstrap was not removed")
	}
	rotated := strings.Repeat("d", 64)
	if err := vault.WriteSecret("egg_shared_key", rotated); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ApplyEggBootstrap(dir, vault); err != nil {
		t.Fatal(err)
	}
	if got, _ := vault.ReadSecret("egg_shared_key"); got != rotated {
		t.Fatal("stale bootstrap overwrote a rotated key")
	}
}

func TestEggBootstrapRejectsInvalidInputBeforeVaultWrites(t *testing.T) {
	for _, raw := range []string{`{"version":0,"secrets":{}}`, `{"version":1,"secrets":{"egg_shared_key":"invalid"}}`, `{"version":1,"secrets":{}} {}`, `{"version":1,"unknown":"field"}`} {
		t.Run(raw, func(t *testing.T) {
			dir := t.TempDir()
			vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(dir, "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, EggBootstrapFile), []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			if err := ApplyEggBootstrap(dir, vault); err == nil {
				t.Fatal("invalid bootstrap accepted")
			}
			if _, err := os.Stat(filepath.Join(dir, "vault.bin")); !os.IsNotExist(err) {
				t.Fatal("invalid bootstrap changed the Vault")
			}
		})
	}
}
