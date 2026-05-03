package launchpad

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "launchpad.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected database file to exist")
	}
}

func TestCreateAndGetByID(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "lp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	link := LaunchpadLink{
		Title:       "Test App",
		URL:         "https://example.com",
		Description: "A test link",
		Category:    "Testing",
		Tags:        []string{"test", "demo"},
		SortOrder:   1,
	}

	id, err := Create(db, link)
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
	if got.Title != "Test App" {
		t.Errorf("Expected title 'Test App', got '%s'", got.Title)
	}
	if got.URL != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got '%s'", got.URL)
	}
	if got.Description != "A test link" {
		t.Errorf("Expected description 'A test link', got '%s'", got.Description)
	}
	if got.Category != "Testing" {
		t.Errorf("Expected category 'Testing', got '%s'", got.Category)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "test" || got.Tags[1] != "demo" {
		t.Errorf("Expected tags [test demo], got %v", got.Tags)
	}
	if got.SortOrder != 1 {
		t.Errorf("Expected sort_order 1, got %d", got.SortOrder)
	}
}

func TestCreateValidation(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "lp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := Create(db, LaunchpadLink{Title: "", URL: "https://example.com"}); err == nil {
		t.Error("Expected error for empty title")
	}
	if _, err := Create(db, LaunchpadLink{Title: "Test", URL: ""}); err == nil {
		t.Error("Expected error for empty URL")
	}
	if _, err := Create(db, LaunchpadLink{Title: "Test", URL: "ftp://example.com"}); err == nil {
		t.Error("Expected error for non-http URL")
	}
}

func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "lp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := Create(db, LaunchpadLink{Title: "Old", URL: "https://old.com", Category: "OldCat"})

	link := LaunchpadLink{
		ID:          id,
		Title:       "New",
		URL:         "https://new.com",
		Description: "Updated",
		Category:    "NewCat",
		SortOrder:   5,
	}
	if err := Update(db, link); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := GetByID(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "New" {
		t.Errorf("Expected title 'New', got '%s'", got.Title)
	}
	if got.URL != "https://new.com" {
		t.Errorf("Expected URL 'https://new.com', got '%s'", got.URL)
	}
	if got.Category != "NewCat" {
		t.Errorf("Expected category 'NewCat', got '%s'", got.Category)
	}
	if got.SortOrder != 5 {
		t.Errorf("Expected sort_order 5, got %d", got.SortOrder)
	}

	// Update non-existent
	link.ID = "nonexistent"
	if err := Update(db, link); err == nil {
		t.Error("Expected error for non-existent ID")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "lp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, _ := Create(db, LaunchpadLink{Title: "ToDelete", URL: "https://delete.com"})

	iconPath, err := Delete(db, id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if iconPath != "" {
		t.Errorf("Expected empty iconPath, got '%s'", iconPath)
	}

	if _, err := GetByID(db, id); err == nil {
		t.Error("Expected error after deleting link")
	}
}

func TestListAndCategories(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := InitDB(filepath.Join(tmpDir, "lp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	Create(db, LaunchpadLink{Title: "A", URL: "https://a.com", Category: "Cat1", SortOrder: 2})
	Create(db, LaunchpadLink{Title: "B", URL: "https://b.com", Category: "Cat2", SortOrder: 1})
	Create(db, LaunchpadLink{Title: "C", URL: "https://c.com", Category: "Cat1", SortOrder: 3})

	all, err := List(db, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("Expected 3 links, got %d", len(all))
	}
	// Should be sorted by sort_order ascending
	if all[0].Title != "B" || all[1].Title != "A" || all[2].Title != "C" {
		t.Errorf("Unexpected sort order: %v", []string{all[0].Title, all[1].Title, all[2].Title})
	}

	cat1, err := List(db, "Cat1")
	if err != nil {
		t.Fatal(err)
	}
	if len(cat1) != 2 {
		t.Errorf("Expected 2 links in Cat1, got %d", len(cat1))
	}

	cats, err := ListCategories(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(cats))
	}
}

func TestValidateURL(t *testing.T) {
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://example.com", true},
		{"http://localhost:8080", true},
		{"", false},
		{"ftp://example.com", false},
		{"javascript:alert(1)", false},
	}
	for _, c := range cases {
		err := validateURL(c.url)
		if c.ok && err != nil {
			t.Errorf("Expected URL %q to be valid, got error: %v", c.url, err)
		}
		if !c.ok && err == nil {
			t.Errorf("Expected URL %q to be invalid", c.url)
		}
	}
}
