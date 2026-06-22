package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndGetHistoryEntry(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	projectID, _, _ := RegisterProject(db, HomepageProject{Name: "HistoryTest", Framework: "astro"})

	id, err := AddHomepageHistoryEntry(db, projectID, "decision", "Use dark hero with single CTA", "homepage_file", []string{"design", "hero"})
	if err != nil {
		t.Fatalf("AddHomepageHistoryEntry failed: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	entry, err := GetHomepageHistoryEntry(db, id)
	if err != nil {
		t.Fatalf("GetHomepageHistoryEntry failed: %v", err)
	}
	if entry.Content != "Use dark hero with single CTA" {
		t.Errorf("content = %q, want %q", entry.Content, "Use dark hero with single CTA")
	}
	if entry.EntryType != "decision" {
		t.Errorf("entry_type = %q, want decision", entry.EntryType)
	}
	if entry.Source != "homepage_file" {
		t.Errorf("source = %q, want homepage_file", entry.Source)
	}
	if len(entry.Tags) != 2 || entry.Tags[0] != "design" || entry.Tags[1] != "hero" {
		t.Errorf("tags = %v, want [design hero]", entry.Tags)
	}
}

func TestListHistoryEntries(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	projectID, _, _ := RegisterProject(db, HomepageProject{Name: "ListHistoryTest", Framework: "next"})

	AddHomepageHistoryEntry(db, projectID, "note", "First note", "", nil)
	AddHomepageHistoryEntry(db, projectID, "decision", "Second decision", "", nil)

	entries, total, err := ListHomepageHistoryEntries(db, projectID, "", nil, 10, 0)
	if err != nil {
		t.Fatalf("ListHomepageHistoryEntries failed: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(entries) != 2 {
		t.Errorf("entries count = %d, want 2", len(entries))
	}
	// Newest first
	if entries[0].EntryType != "decision" {
		t.Errorf("newest entry_type = %q, want decision", entries[0].EntryType)
	}

	entries, total, _ = ListHomepageHistoryEntries(db, projectID, "note", nil, 10, 0)
	if total != 1 {
		t.Errorf("filtered total = %d, want 1", total)
	}
}

func TestSearchHistoryEntries(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	projectID, _, _ := RegisterProject(db, HomepageProject{Name: "SearchHistoryTest", Framework: "html"})

	AddHomepageHistoryEntry(db, projectID, "note", "Hero section looks great", "", nil)
	AddHomepageHistoryEntry(db, projectID, "decision", "Footer will have three columns", "", nil)

	entries, total, err := SearchHomepageHistoryEntries(db, projectID, "Hero", "", nil, 10, 0)
	if err != nil {
		t.Fatalf("SearchHomepageHistoryEntries failed: %v", err)
	}
	if total != 1 {
		t.Errorf("search total = %d, want 1", total)
	}
	if len(entries) != 1 || entries[0].Content != "Hero section looks great" {
		t.Errorf("unexpected search result: %+v", entries)
	}
}

func TestUpdateAndDeleteHistoryEntry(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	projectID, _, _ := RegisterProject(db, HomepageProject{Name: "UpdateHistoryTest", Framework: "vue"})

	id, _ := AddHomepageHistoryEntry(db, projectID, "note", "Original note", "", nil)

	if err := UpdateHomepageHistoryEntry(db, id, "Updated note", []string{"edited"}); err != nil {
		t.Fatalf("UpdateHomepageHistoryEntry failed: %v", err)
	}

	entry, _ := GetHomepageHistoryEntry(db, id)
	if entry.Content != "Updated note" {
		t.Errorf("content after update = %q, want Updated note", entry.Content)
	}
	if len(entry.Tags) != 1 || entry.Tags[0] != "edited" {
		t.Errorf("tags after update = %v, want [edited]", entry.Tags)
	}

	if err := DeleteHomepageHistoryEntry(db, id); err != nil {
		t.Fatalf("DeleteHomepageHistoryEntry failed: %v", err)
	}

	_, err = GetHomepageHistoryEntry(db, id)
	if err == nil {
		t.Error("expected error after deleting entry")
	}
}

func TestDispatchHomepageHistory(t *testing.T) {
	db, err := InitHomepageRegistryDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	projectID, _, _ := RegisterProject(db, HomepageProject{Name: "DispatchHistoryTest", Framework: "svelte"})

	result := DispatchHomepageHistory(db, "add_history", 0, projectID, "milestone", "Initial setup complete", "", "homepage_project", []string{"setup"}, 0, 0)
	if !strings.Contains(result, `"status":"success"`) {
		t.Errorf("add_history result = %s", result)
	}

	result = DispatchHomepageHistory(db, "list_history", 0, projectID, "", "", "", "", nil, 10, 0)
	if !strings.Contains(result, `"total":1`) {
		t.Errorf("list_history result = %s", result)
	}

	result = DispatchHomepageHistory(db, "search_history", 0, projectID, "", "", "setup", "", nil, 10, 0)
	if !strings.Contains(result, `"total":1`) {
		t.Errorf("search_history result = %s", result)
	}
}
