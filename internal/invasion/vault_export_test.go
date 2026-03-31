package invasion

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/security"
)

func testVault(t *testing.T) (*security.Vault, string) {
	t.Helper()
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "vault.bin")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	keyHex := hex.EncodeToString(key)
	vault, err := security.NewVault(keyHex, vaultPath)
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	return vault, dir
}

func TestGenerateSharedKey_Length(t *testing.T) {
	key, err := GenerateSharedKey()
	if err != nil {
		t.Fatalf("GenerateSharedKey: %v", err)
	}
	// 32 bytes = 64 hex characters
	if len(key) != 64 {
		t.Errorf("key length = %d, want 64", len(key))
	}
	// Must be valid hex
	if _, err := hex.DecodeString(key); err != nil {
		t.Errorf("key is not valid hex: %v", err)
	}
}

func TestGenerateSharedKey_Unique(t *testing.T) {
	k1, _ := GenerateSharedKey()
	k2, _ := GenerateSharedKey()
	if k1 == k2 {
		t.Error("two generated keys should be different")
	}
}

func TestExportVaultForEgg_EmptyVault(t *testing.T) {
	vault, _ := testVault(t)

	data, keyHex, err := ExportVaultForEgg(vault)
	if err != nil {
		t.Fatalf("ExportVaultForEgg: %v", err)
	}
	if len(data) == 0 {
		t.Error("encrypted data should not be empty")
	}
	if len(keyHex) != 64 {
		t.Errorf("key hex length = %d, want 64", len(keyHex))
	}

	// Decrypt and verify empty map
	secrets := decryptExport(t, data, keyHex)
	if len(secrets) != 0 {
		t.Errorf("secrets map should be empty, got %d entries", len(secrets))
	}
}

func TestExportVaultForEgg_RoundTrip(t *testing.T) {
	vault, _ := testVault(t)

	// Write some secrets
	if err := vault.WriteSecret("api_key", "sk-test-123"); err != nil {
		t.Fatal(err)
	}
	if err := vault.WriteSecret("db_password", "s3cret"); err != nil {
		t.Fatal(err)
	}

	data, keyHex, err := ExportVaultForEgg(vault)
	if err != nil {
		t.Fatal(err)
	}

	secrets := decryptExport(t, data, keyHex)
	if secrets["api_key"] != "sk-test-123" {
		t.Errorf("api_key = %q", secrets["api_key"])
	}
	if secrets["db_password"] != "s3cret" {
		t.Errorf("db_password = %q", secrets["db_password"])
	}
}

func TestExportVaultForEgg_UniqueKeys(t *testing.T) {
	vault, _ := testVault(t)

	_, key1, _ := ExportVaultForEgg(vault)
	_, key2, _ := ExportVaultForEgg(vault)

	if key1 == key2 {
		t.Error("each export should produce a unique key")
	}
}

func TestExportVaultForEgg_EncryptionChanges(t *testing.T) {
	vault, _ := testVault(t)
	vault.WriteSecret("test", "value")

	data1, _, _ := ExportVaultForEgg(vault)
	data2, _, _ := ExportVaultForEgg(vault)

	// Different keys + nonces → different ciphertext
	if string(data1) == string(data2) {
		t.Error("two exports should produce different ciphertext")
	}
}

func TestExportVaultForEgg_CanCreateEggVault(t *testing.T) {
	vault, _ := testVault(t)
	vault.WriteSecret("token", "abc123")

	_, eggKeyHex, err := ExportVaultForEgg(vault)
	if err != nil {
		t.Fatal(err)
	}

	// The egg key should be usable to create a new Vault
	dir := t.TempDir()
	eggVault, err := security.NewVault(eggKeyHex, filepath.Join(dir, "egg_vault.bin"))
	if err != nil {
		t.Fatalf("egg vault creation failed: %v", err)
	}

	// Write and read-back in egg vault
	if err := eggVault.WriteSecret("hello", "world"); err != nil {
		t.Fatal(err)
	}
	val, err := eggVault.ReadSecret("hello")
	if err != nil {
		t.Fatal(err)
	}
	if val != "world" {
		t.Errorf("egg vault read = %q", val)
	}
}

// decryptExport uses the export key to decrypt the vault export blob.
func decryptExport(t *testing.T, data []byte, keyHex string) map[string]string {
	t.Helper()
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Fatalf("invalid key hex: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		t.Fatalf("data too short: %d < %d", len(data), nonceSize)
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	var secrets map[string]string
	if err := json.Unmarshal(plaintext, &secrets); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	return secrets
}

// Verify vault file is NOT leaked to the export data directory
func TestExportVaultForEgg_NoFileLeak(t *testing.T) {
	vault, dir := testVault(t)
	vault.WriteSecret("secret", "value")

	ExportVaultForEgg(vault)

	// No extra files should appear in temp dir (only vault.bin and vault.bin.lock)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if name != "vault.bin" && name != "vault.bin.lock" {
			t.Errorf("unexpected file in vault dir: %s", name)
		}
	}
}
