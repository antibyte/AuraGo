package inventory

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/dbutil"
)

func TestInventory(t *testing.T) {
	dbPath := "test_infrastructure.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer db.Close()

	device := DeviceRecord{
		ID:            "srv-1",
		Name:          "web-01",
		Type:          "server",
		Protocol:      "ssh",
		Port:          80,
		Username:      "admin",
		VaultSecretID: "vault/web-01",
		CredentialID:  "cred-1",
		Tags:          []string{"production", "web"},
	}

	// Test Add
	err = AddDevice(db, device)
	if err != nil {
		t.Fatalf("Failed to add device: %v", err)
	}

	// Test Get
	retrieved, err := GetDeviceByID(db, "srv-1")
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}
	if retrieved.Name != "web-01" {
		t.Errorf("Expected name web-01, got %s", retrieved.Name)
	}
	if retrieved.CredentialID != "cred-1" {
		t.Errorf("Expected credential_id cred-1, got %s", retrieved.CredentialID)
	}
	if len(retrieved.Tags) != 2 || retrieved.Tags[0] != "production" {
		t.Errorf("Tags mismatch: %v", retrieved.Tags)
	}

	// Test List by Tag
	devices, err := ListDevicesByTag(db, "web")
	if err != nil {
		t.Fatalf("Failed to list devices by tag: %v", err)
	}
	if len(devices) != 1 {
		t.Errorf("Expected 1 device with tag 'web', got %d", len(devices))
	}

	devices, err = ListDevicesByTag(db, "nonexistent")
	if err != nil {
		t.Fatalf("Failed to list devices by tag: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices with tag 'nonexistent', got %d", len(devices))
	}
}

func TestInitDBCreatesQueryIndexes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "inventory.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	var indexCount int
	if err := db.QueryRow(`
		SELECT count(*) FROM sqlite_master
		WHERE type = 'index' AND tbl_name = 'devices' AND name IN ('idx_devices_type', 'idx_devices_name_ci')
	`).Scan(&indexCount); err != nil {
		t.Fatalf("count indexes: %v", err)
	}
	if indexCount != 2 {
		t.Fatalf("index count = %d, want 2", indexCount)
	}

	version, err := dbutil.GetUserVersion(db)
	if err != nil {
		t.Fatalf("GetUserVersion: %v", err)
	}
	if version != 4 {
		t.Fatalf("schema version = %d, want 4", version)
	}
}
