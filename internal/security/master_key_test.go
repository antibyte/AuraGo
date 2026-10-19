package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadMasterKeyFilePreservesExistingKey(t *testing.T) {
	key := strings.Repeat("5a", 32)
	path := filepath.Join(t.TempDir(), "existing.key")
	original := []byte(key + "\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadMasterKeyFile(path)
	if err != nil || got != key {
		t.Fatalf("key read failed: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != string(original) {
		t.Fatal("existing key file changed")
	}
	for _, invalid := range []string{"", strings.Repeat("z", 64), strings.Repeat("a", 63), strings.Repeat("a", 258)} {
		if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadMasterKeyFile(path); err == nil {
			t.Fatal("invalid key accepted")
		}
	}
}
