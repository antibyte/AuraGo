package invasion

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "invasion_test.db")
}

func TestInitDB(t *testing.T) {
	dbPath := tempDB(t)
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Verify tables exist
	for _, table := range []string{"nests", "eggs"} {
		var count int
		err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Fatalf("failed to check table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s not found", table)
		}
	}
}

func TestNestCRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	// Create
	nest := NestRecord{
		Name:       "Test Server",
		Notes:      "Integration test nest",
		AccessType: "ssh",
		Host:       "192.168.1.100",
		Port:       22,
		Username:   "deploy",
		Active:     true,
	}
	id, err := CreateNest(db, nest)
	if err != nil {
		t.Fatalf("CreateNest: %v", err)
	}
	if id == "" {
		t.Fatal("CreateNest returned empty ID")
	}

	// Read
	got, err := GetNest(db, id)
	if err != nil {
		t.Fatalf("GetNest: %v", err)
	}
	if got.Name != "Test Server" {
		t.Errorf("name = %q, want %q", got.Name, "Test Server")
	}
	if got.AccessType != "ssh" {
		t.Errorf("access_type = %q, want %q", got.AccessType, "ssh")
	}
	if got.Host != "192.168.1.100" {
		t.Errorf("host = %q, want %q", got.Host, "192.168.1.100")
	}
	if !got.Active {
		t.Error("expected active = true")
	}
	if got.CreatedAt == "" {
		t.Error("created_at should be set")
	}

	// Update
	got.Name = "Updated Server"
	got.Notes = "Updated notes"
	if err := UpdateNest(db, got); err != nil {
		t.Fatalf("UpdateNest: %v", err)
	}
	updated, _ := GetNest(db, id)
	if updated.Name != "Updated Server" {
		t.Errorf("updated name = %q, want %q", updated.Name, "Updated Server")
	}
	if updated.Notes != "Updated notes" {
		t.Errorf("updated notes = %q, want %q", updated.Notes, "Updated notes")
	}

	// Toggle active
	if err := ToggleNestActive(db, id, false); err != nil {
		t.Fatalf("ToggleNestActive: %v", err)
	}
	toggled, _ := GetNest(db, id)
	if toggled.Active {
		t.Error("expected active = false after toggle")
	}

	// List
	nests, err := ListNests(db)
	if err != nil {
		t.Fatalf("ListNests: %v", err)
	}
	if len(nests) != 1 {
		t.Errorf("ListNests count = %d, want 1", len(nests))
	}

	// ListActive (should be 0 since we toggled off)
	activeNests, err := ListActiveNests(db)
	if err != nil {
		t.Fatalf("ListActiveNests: %v", err)
	}
	if len(activeNests) != 0 {
		t.Errorf("ListActiveNests count = %d, want 0", len(activeNests))
	}

	// GetByName
	byName, err := GetNestByName(db, "updated server") // case-insensitive
	if err != nil {
		t.Fatalf("GetNestByName: %v", err)
	}
	if byName.ID != id {
		t.Errorf("GetNestByName ID = %q, want %q", byName.ID, id)
	}

	// Delete
	if err := DeleteNest(db, id); err != nil {
		t.Fatalf("DeleteNest: %v", err)
	}
	_, err = GetNest(db, id)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestEggCRUD(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	// Create
	egg := EggRecord{
		Name:        "Analytics Agent",
		Description: "Processes analytics data",
		Personality: "analytical",
		Model:       "gpt-4o-mini",
		Provider:    "openrouter",
		Active:      true,
	}
	id, err := CreateEgg(db, egg)
	if err != nil {
		t.Fatalf("CreateEgg: %v", err)
	}
	if id == "" {
		t.Fatal("CreateEgg returned empty ID")
	}

	// Read
	got, err := GetEgg(db, id)
	if err != nil {
		t.Fatalf("GetEgg: %v", err)
	}
	if got.Name != "Analytics Agent" {
		t.Errorf("name = %q, want %q", got.Name, "Analytics Agent")
	}
	if got.Model != "gpt-4o-mini" {
		t.Errorf("model = %q, want %q", got.Model, "gpt-4o-mini")
	}
	if !got.Active {
		t.Error("expected active = true")
	}

	// Update
	got.Description = "Updated description"
	got.Model = "claude-3.5-sonnet"
	if err := UpdateEgg(db, got); err != nil {
		t.Fatalf("UpdateEgg: %v", err)
	}
	updated, _ := GetEgg(db, id)
	if updated.Description != "Updated description" {
		t.Errorf("description = %q, want %q", updated.Description, "Updated description")
	}
	if updated.Model != "claude-3.5-sonnet" {
		t.Errorf("model = %q, want %q", updated.Model, "claude-3.5-sonnet")
	}

	// Toggle
	if err := ToggleEggActive(db, id, false); err != nil {
		t.Fatalf("ToggleEggActive: %v", err)
	}
	toggled, _ := GetEgg(db, id)
	if toggled.Active {
		t.Error("expected active = false")
	}

	// List
	eggs, err := ListEggs(db)
	if err != nil {
		t.Fatalf("ListEggs: %v", err)
	}
	if len(eggs) != 1 {
		t.Errorf("ListEggs count = %d, want 1", len(eggs))
	}

	// Delete
	if err := DeleteEgg(db, id); err != nil {
		t.Fatalf("DeleteEgg: %v", err)
	}
	_, err = GetEgg(db, id)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	if err := DeleteNest(db, "nonexistent"); err == nil {
		t.Error("expected error deleting non-existent nest")
	}
	if err := DeleteEgg(db, "nonexistent"); err == nil {
		t.Error("expected error deleting non-existent egg")
	}
}

func TestNestEggAssignment(t *testing.T) {
	db, err := InitDB(tempDB(t))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	eggID, _ := CreateEgg(db, EggRecord{Name: "Worker", Active: true})
	nestID, _ := CreateNest(db, NestRecord{Name: "Server", AccessType: "ssh", Host: "10.0.0.1", Port: 22, Active: true})

	// Assign egg to nest
	nest, _ := GetNest(db, nestID)
	nest.EggID = eggID
	if err := UpdateNest(db, nest); err != nil {
		t.Fatalf("assign egg: %v", err)
	}

	updated, _ := GetNest(db, nestID)
	if updated.EggID != eggID {
		t.Errorf("egg_id = %q, want %q", updated.EggID, eggID)
	}
}
