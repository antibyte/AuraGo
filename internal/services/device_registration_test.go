package services

import (
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/inventory"
	"aurago/internal/security"
)

func TestRegisterDeviceRejectsPrivateKeyPath(t *testing.T) {
	db, err := inventory.InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	vault, err := security.NewVault("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}

	_, err = RegisterDevice(db, vault, "server-1", "server", "192.168.1.10", 22, "root", "", filepath.Join(t.TempDir(), "id_rsa"), "desc", nil, "")
	if err == nil {
		t.Fatal("RegisterDevice accepted private_key_path; expected error")
	}
	if !strings.Contains(err.Error(), "private_key_path") {
		t.Fatalf("error = %q, want private_key_path guidance", err.Error())
	}

	devices, listErr := inventory.ListAllDevices(db)
	if listErr != nil {
		t.Fatalf("ListAllDevices: %v", listErr)
	}
	if len(devices) != 0 {
		t.Fatalf("device was registered despite private_key_path rejection: %#v", devices)
	}
}
