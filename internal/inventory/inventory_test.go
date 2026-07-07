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

func TestInventoryHandlesLegacyNullFieldsAndBadTags(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "inventory.db")
	legacyDB, err := dbutil.Open(dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := legacyDB.Exec(`
		CREATE TABLE devices (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			ip_address TEXT,
			port INTEGER NOT NULL DEFAULT 22,
			username TEXT,
			vault_secret_id TEXT,
			credential_id TEXT,
			description TEXT,
			tags TEXT,
			mac_address TEXT
		);
		INSERT INTO devices (id, name, type, port, ip_address, username, vault_secret_id, credential_id, description, tags, mac_address)
		VALUES ('legacy-1', 'Legacy Server', 'server', 22, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
	`); err != nil {
		legacyDB.Close()
		t.Fatalf("seed legacy db: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	devices, err := ListAllDevices(db)
	if err != nil {
		t.Fatalf("ListAllDevices with legacy NULLs: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices = %d, want 1", len(devices))
	}
	got := devices[0]
	if got.Protocol != "ssh" {
		t.Fatalf("protocol = %q, want ssh", got.Protocol)
	}
	if got.IPAddress != "" || got.Username != "" || got.VaultSecretID != "" || got.CredentialID != "" || got.Description != "" || got.MACAddress != "" {
		t.Fatalf("legacy NULL fields were not normalized: %#v", got)
	}
	if got.Tags == nil || len(got.Tags) != 0 {
		t.Fatalf("tags = %#v, want empty slice", got.Tags)
	}

	if _, err := db.Exec(`UPDATE devices SET tags = 'not-json' WHERE id = 'legacy-1'`); err != nil {
		t.Fatalf("set invalid legacy tags: %v", err)
	}
	got, err = GetDeviceByID(db, "legacy-1")
	if err != nil {
		t.Fatalf("GetDeviceByID with invalid tags: %v", err)
	}
	if got.Tags == nil || len(got.Tags) != 0 {
		t.Fatalf("tags with invalid JSON = %#v, want empty slice", got.Tags)
	}
	devices, err = QueryDevices(db, "", "", "Legacy")
	if err != nil {
		t.Fatalf("QueryDevices with invalid tags: %v", err)
	}
	if len(devices) != 1 || len(devices[0].Tags) != 0 {
		t.Fatalf("QueryDevices tags = %#v, want one device with empty tags", devices)
	}
}

func TestInventorySupportsProtocolNone(t *testing.T) {
	t.Parallel()

	db, err := InitDB(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	id, err := CreateDevice(db, "Registry Only", "printer", "none", "192.168.1.90", 0, "", "", "", "", nil, "")
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	got, err := GetDeviceByID(db, id)
	if err != nil {
		t.Fatalf("GetDeviceByID: %v", err)
	}
	if got.Protocol != "none" {
		t.Fatalf("protocol = %q, want none", got.Protocol)
	}
}
