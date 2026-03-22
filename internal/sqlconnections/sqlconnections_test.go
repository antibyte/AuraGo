package sqlconnections

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_sql_connections.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	return db, func() {
		db.Close()
		os.Remove(dbPath)
	}
}

func TestInitDB(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Table should exist
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sql_connections'").Scan(&name)
	if err != nil {
		t.Fatalf("expected sql_connections table to exist: %v", err)
	}
}

func TestCreateAndGetByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	id, err := Create(db, "testconn", "postgres", "localhost", 5432, "testdb", "A test DB",
		true, false, false, false, "vault-key-1", "disable")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	rec, err := GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if rec.Name != "testconn" {
		t.Errorf("expected name 'testconn', got %q", rec.Name)
	}
	if rec.Driver != "postgres" {
		t.Errorf("expected driver 'postgres', got %q", rec.Driver)
	}
	if rec.Host != "localhost" {
		t.Errorf("expected host 'localhost', got %q", rec.Host)
	}
	if rec.Port != 5432 {
		t.Errorf("expected port 5432, got %d", rec.Port)
	}
	if rec.DatabaseName != "testdb" {
		t.Errorf("expected database 'testdb', got %q", rec.DatabaseName)
	}
	if rec.Description != "A test DB" {
		t.Errorf("expected description, got %q", rec.Description)
	}
	if !rec.AllowRead {
		t.Error("expected AllowRead=true")
	}
	if rec.AllowWrite || rec.AllowChange || rec.AllowDelete {
		t.Error("expected write/change/delete to be false")
	}
	if rec.VaultSecretID != "vault-key-1" {
		t.Errorf("expected vault key, got %q", rec.VaultSecretID)
	}
}

func TestGetByName(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := Create(db, "byname", "mysql", "db.local", 3306, "appdb", "desc",
		true, true, false, false, "", "disable")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	rec, err := GetByName(db, "byname")
	if err != nil {
		t.Fatalf("GetByName failed: %v", err)
	}
	if rec.Driver != "mysql" {
		t.Errorf("expected driver 'mysql', got %q", rec.Driver)
	}
	if !rec.AllowWrite {
		t.Error("expected AllowWrite=true")
	}
}

func TestGetByName_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := GetByName(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent connection")
	}
}

func TestList(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	Create(db, "alpha", "postgres", "host1", 5432, "db1", "", true, false, false, false, "", "")
	Create(db, "beta", "mysql", "host2", 3306, "db2", "", true, true, false, false, "", "")
	Create(db, "gamma", "sqlite", "", 0, "/tmp/test.db", "", true, true, true, true, "", "")

	list, err := List(db)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(list))
	}
	// Should be ordered by name
	if list[0].Name != "alpha" || list[1].Name != "beta" || list[2].Name != "gamma" {
		t.Errorf("unexpected order: %s, %s, %s", list[0].Name, list[1].Name, list[2].Name)
	}
}

func TestUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	id, _ := Create(db, "original", "postgres", "host", 5432, "db", "desc",
		true, false, false, false, "vault-1", "disable")

	err := Update(db, id, "renamed", "mysql", "newhost", 3306, "newdb", "updated desc",
		true, true, true, false, "vault-2", "require")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	rec, _ := GetByID(db, id)
	if rec.Name != "renamed" {
		t.Errorf("expected name 'renamed', got %q", rec.Name)
	}
	if rec.Driver != "mysql" {
		t.Errorf("expected driver 'mysql', got %q", rec.Driver)
	}
	if rec.Host != "newhost" {
		t.Errorf("expected host 'newhost', got %q", rec.Host)
	}
	if !rec.AllowChange {
		t.Error("expected AllowChange=true after update")
	}
	if rec.AllowDelete {
		t.Error("expected AllowDelete=false after update")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := Update(db, "nonexistent-id", "x", "postgres", "", 0, "", "",
		true, false, false, false, "", "")
	if err == nil {
		t.Fatal("expected error for nonexistent id")
	}
}

func TestDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	id, _ := Create(db, "todelete", "sqlite", "", 0, "/tmp/d.db", "",
		true, false, false, false, "", "")

	err := Delete(db, id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = GetByID(db, id)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestDelete_NotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := Delete(db, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent id")
	}
}

func TestCreateDuplicateName(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := Create(db, "dup", "postgres", "", 0, "db", "", true, false, false, false, "", "")
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	_, err = Create(db, "dup", "mysql", "", 0, "db2", "", true, false, false, false, "", "")
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestCreateInvalidDriver(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := Create(db, "bad", "oracle", "", 0, "db", "", true, false, false, false, "", "")
	if err == nil {
		t.Fatal("expected error for unsupported driver")
	}
}

func TestCreateEmptyName(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := Create(db, "", "postgres", "", 0, "db", "", true, false, false, false, "", "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestMarshalUnmarshalCredentials(t *testing.T) {
	data, err := MarshalCredentials("admin", "secret123")
	if err != nil {
		t.Fatalf("MarshalCredentials failed: %v", err)
	}

	user, pass, err := UnmarshalCredentials(data)
	if err != nil {
		t.Fatalf("UnmarshalCredentials failed: %v", err)
	}
	if user != "admin" || pass != "secret123" {
		t.Errorf("expected admin/secret123, got %s/%s", user, pass)
	}
}

func TestUnmarshalCredentials_Invalid(t *testing.T) {
	_, _, err := UnmarshalCredentials("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
