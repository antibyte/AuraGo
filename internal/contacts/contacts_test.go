package contacts

import (
	"os"
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *testDBHandle {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_contacts.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	return &testDBHandle{db: db, path: dbPath}
}

type testDBHandle struct {
	db   interface{ Close() error }
	path string
}

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "contacts.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// DB file should exist
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected database file to exist")
	}
}

func TestCreateAndGetByID(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	c := Contact{
		Name:         "Alice Smith",
		Email:        "alice@example.com",
		Phone:        "+49123456",
		Mobile:       "+49170789",
		Address:      "123 Main St",
		Relationship: "friend",
		Notes:        "test note",
	}

	id, err := Create(db, c)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == "" {
		t.Fatal("Expected non-empty ID")
	}

	got, err := GetByID(db, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != "Alice Smith" {
		t.Errorf("Expected name 'Alice Smith', got '%s'", got.Name)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Expected email 'alice@example.com', got '%s'", got.Email)
	}
	if got.Relationship != "friend" {
		t.Errorf("Expected relationship 'friend', got '%s'", got.Relationship)
	}
}

func TestCreateRequiresName(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = Create(db, Contact{Email: "no-name@example.com"})
	if err == nil {
		t.Error("Expected error when creating contact without name")
	}
}

func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := Create(db, Contact{Name: "Bob"})

	err = Update(db, Contact{ID: id, Name: "Bob Updated", Email: "bob@new.com"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, _ := GetByID(db, id)
	if got.Name != "Bob Updated" {
		t.Errorf("Expected updated name, got '%s'", got.Name)
	}
	if got.Email != "bob@new.com" {
		t.Errorf("Expected updated email, got '%s'", got.Email)
	}
}

func TestUpdateNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = Update(db, Contact{ID: "nonexistent", Name: "Ghost"})
	if err == nil {
		t.Error("Expected error when updating nonexistent contact")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := Create(db, Contact{Name: "ToDelete"})
	err = Delete(db, id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = GetByID(db, id)
	if err == nil {
		t.Error("Expected error after deleting contact")
	}
}

func TestDeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = Delete(db, "nonexistent")
	if err == nil {
		t.Error("Expected error when deleting nonexistent contact")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	Create(db, Contact{Name: "Alice", Email: "alice@test.com", Relationship: "friend"})
	Create(db, Contact{Name: "Bob", Phone: "+49170", Relationship: "colleague"})
	Create(db, Contact{Name: "Charlie", Mobile: "+49171"})

	// List all
	all, err := List(db, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 contacts, got %d", len(all))
	}

	// Search by name
	results, _ := List(db, "alice")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'alice', got %d", len(results))
	}

	// Search by relationship
	results, _ = List(db, "colleague")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'colleague', got %d", len(results))
	}

	// Search matching nothing
	results, _ = List(db, "zzzzz")
	if len(results) != 0 {
		t.Errorf("Expected 0 results for 'zzzzz', got %d", len(results))
	}
}

func TestToJSON(t *testing.T) {
	c := Contact{ID: "abc", Name: "Test"}
	j := ToJSON(c)
	if j == "" || j == "{}" {
		t.Error("Expected non-empty JSON")
	}
	if len(j) < 10 {
		t.Error("JSON too short")
	}
}
