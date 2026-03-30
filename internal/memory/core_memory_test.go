package memory

import (
	"strings"
	"testing"
)

// ── Core Memory CRUD ──────────────────────────────────────────────────────────

func TestCoreMemory_AddAndGet(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("The sky is blue")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Fact != "The sky is blue" {
		t.Errorf("unexpected fact text: %q", facts[0].Fact)
	}
	if facts[0].ID != id {
		t.Errorf("expected ID %d, got %d", id, facts[0].ID)
	}
}

func TestCoreMemory_Update(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("original fact")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	if err := stm.UpdateCoreMemoryFact(id, "updated fact"); err != nil {
		t.Fatalf("UpdateCoreMemoryFact: %v", err)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 1 || facts[0].Fact != "updated fact" {
		t.Errorf("expected 'updated fact', got %v", facts)
	}
}

func TestCoreMemory_UpdateNonExistent(t *testing.T) {
	stm := newTestProfileDB(t)

	err := stm.UpdateCoreMemoryFact(99999, "does not matter")
	if err == nil {
		t.Error("expected error when updating non-existent fact")
	}
}

func TestCoreMemory_Delete(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("to be deleted")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	if err := stm.DeleteCoreMemoryFact(id); err != nil {
		t.Fatalf("DeleteCoreMemoryFact: %v", err)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts after delete, got %d", len(facts))
	}
}

func TestCoreMemory_DeleteNonExistent(t *testing.T) {
	stm := newTestProfileDB(t)

	err := stm.DeleteCoreMemoryFact(99999)
	if err == nil {
		t.Error("expected error when deleting non-existent fact")
	}
}

func TestCoreMemory_FactExists(t *testing.T) {
	stm := newTestProfileDB(t)

	if stm.CoreMemoryFactExists("missing") {
		t.Error("expected false for non-existent fact")
	}

	if _, err := stm.AddCoreMemoryFact("present"); err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	if !stm.CoreMemoryFactExists("present") {
		t.Error("expected true for existing fact")
	}

	// Exact match only — a substring should not match.
	if stm.CoreMemoryFactExists("pres") {
		t.Error("CoreMemoryFactExists should do exact match, not substring")
	}
}

func TestCoreMemory_FindByFact(t *testing.T) {
	stm := newTestProfileDB(t)

	id, err := stm.AddCoreMemoryFact("findable fact")
	if err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	found, err := stm.FindCoreMemoryIDByFact("findable fact")
	if err != nil {
		t.Fatalf("FindCoreMemoryIDByFact: %v", err)
	}
	if found != id {
		t.Errorf("expected ID %d, got %d", id, found)
	}

	// Non-existent fact → error.
	if _, err := stm.FindCoreMemoryIDByFact("ghost"); err == nil {
		t.Error("expected error for non-existent fact")
	}
}

func TestCoreMemory_Count(t *testing.T) {
	stm := newTestProfileDB(t)

	count, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	for i := 0; i < 3; i++ {
		if _, err := stm.AddCoreMemoryFact(strings.Repeat("fact", i+1)); err != nil {
			t.Fatalf("AddCoreMemoryFact #%d: %v", i, err)
		}
	}

	count, err = stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount after inserts: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestCoreMemory_TruncatesLongFact(t *testing.T) {
	stm := newTestProfileDB(t)

	longFact := strings.Repeat("x", maxCoreMemoryFactLen+100)
	id, err := stm.AddCoreMemoryFact(longFact)
	if err != nil {
		t.Fatalf("AddCoreMemoryFact (long): %v", err)
	}

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if len(facts[0].Fact) > maxCoreMemoryFactLen {
		t.Errorf("stored fact exceeds maxCoreMemoryFactLen: len=%d", len(facts[0].Fact))
	}
	_ = id
}

func TestCoreMemory_GetEmptyReturnsSlice(t *testing.T) {
	stm := newTestProfileDB(t)

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		t.Fatalf("GetCoreMemoryFacts on empty DB: %v", err)
	}
	// Must return non-nil empty slice (not nil) so callers can range safely.
	if facts == nil {
		t.Error("GetCoreMemoryFacts should return non-nil slice even when empty")
	}
}
