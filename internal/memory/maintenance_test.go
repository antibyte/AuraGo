package memory

import (
	"database/sql"
	"io"
	"log/slog"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSyncExternalSourcesImportsInventoryDevices(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	inventoryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open inventory db: %v", err)
	}
	t.Cleanup(func() { _ = inventoryDB.Close() })

	if _, err := inventoryDB.Exec(`
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
			mac_address TEXT,
			protocol TEXT NOT NULL DEFAULT 'ssh'
		);
		INSERT INTO devices (id, name, type, ip_address, port, description, tags)
		VALUES ('nas-1', 'NAS', 'storage', '192.168.1.10', 22, 'Backup target', 'backup,storage');
	`); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	if err := kg.SyncExternalSources(inventoryDB, logger); err != nil {
		t.Fatalf("SyncExternalSources: %v", err)
	}

	node, err := kg.GetNode("dev_nas-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.Label != "NAS" {
		t.Fatalf("node label = %q, want NAS", node.Label)
	}
	if node.Properties["source"] != "inventory_sync" {
		t.Fatalf("source = %q, want inventory_sync", node.Properties["source"])
	}
}

func TestSyncExternalSourcesRemovesStaleInventoryDevices(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	inventoryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open inventory db: %v", err)
	}
	t.Cleanup(func() { _ = inventoryDB.Close() })

	if _, err := inventoryDB.Exec(`
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
			mac_address TEXT,
			protocol TEXT NOT NULL DEFAULT 'ssh'
		);
		INSERT INTO devices (id, name, type, ip_address, port, description, tags)
		VALUES
			('nas-1', 'NAS', 'storage', '192.168.1.10', 22, 'Backup target', 'backup,storage'),
			('old-pc', 'Old PC', 'workstation', '192.168.1.20', 22, 'Retired host', 'legacy');
	`); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	if err := kg.SyncExternalSources(inventoryDB, logger); err != nil {
		t.Fatalf("initial SyncExternalSources: %v", err)
	}
	if _, err := kg.GetNode("dev_old-pc"); err != nil {
		t.Fatalf("expected dev_old-pc before removal: %v", err)
	}

	if _, err := inventoryDB.Exec(`DELETE FROM devices WHERE id = 'old-pc'`); err != nil {
		t.Fatalf("delete stale inventory device: %v", err)
	}
	if err := kg.SyncExternalSources(inventoryDB, logger); err != nil {
		t.Fatalf("second SyncExternalSources: %v", err)
	}

	if _, err := kg.GetNode("dev_old-pc"); err == nil {
		t.Fatal("expected dev_old-pc to be removed from knowledge graph")
	}
	if _, err := kg.GetNode("dev_nas-1"); err != nil {
		t.Fatalf("expected dev_nas-1 to remain: %v", err)
	}
}

func TestSyncExternalSourcesPreservesNonInventoryDevNodes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	if err := kg.AddNode("dev_manual", "Manual Device", map[string]string{
		"type":   "device",
		"source": "manual",
	}); err != nil {
		t.Fatalf("AddNode manual device: %v", err)
	}

	inventoryDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open inventory db: %v", err)
	}
	t.Cleanup(func() { _ = inventoryDB.Close() })

	if _, err := inventoryDB.Exec(`
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
			mac_address TEXT,
			protocol TEXT NOT NULL DEFAULT 'ssh'
		);
	`); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	if err := kg.SyncExternalSources(inventoryDB, logger); err != nil {
		t.Fatalf("SyncExternalSources: %v", err)
	}
	if _, err := kg.GetNode("dev_manual"); err != nil {
		t.Fatalf("manual dev_ node should be preserved: %v", err)
	}
}