package server

import (
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestStoreEggSharedKeyRequiresVault(t *testing.T) {
	s := &Server{}
	err := s.storeEggSharedKey("nest-1", strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("expected error when vault is unavailable")
	}
}

func TestStoreEggSharedKeyWritesVault(t *testing.T) {
	vault, err := security.NewVault(strings.Repeat("b", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	s := &Server{Vault: vault}
	key := strings.Repeat("c", 64)

	if err := s.storeEggSharedKey("nest-1", key); err != nil {
		t.Fatalf("storeEggSharedKey: %v", err)
	}

	got, err := vault.ReadSecret("egg_shared_nest-1")
	if err != nil {
		t.Fatalf("ReadSecret: %v", err)
	}
	if got != key {
		t.Fatalf("stored key = %q, want %q", got, key)
	}
}
