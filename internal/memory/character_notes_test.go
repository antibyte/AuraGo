package memory

import (
	"strings"
	"testing"
	"time"
)

func TestInsertCharacterNoteAndPromptFormat(t *testing.T) {
	stm := newTestPersonalityDB(t)
	if _, err := stm.InsertCharacterNote(CharacterNote{
		Category:   CharacterNoteCategoryHabit,
		Text:       "I verify changes with one concrete check before calling the work done.",
		Confidence: 0.8,
		Source:     CharacterNoteSourceReflection,
	}); err != nil {
		t.Fatalf("InsertCharacterNote: %v", err)
	}
	got := stm.FormatCharacterNotesForPrompt(6)
	if !strings.Contains(got, "I verify changes") {
		t.Fatalf("prompt notes = %q", got)
	}
}

func TestDeletedCharacterNoteDoesNotReturn(t *testing.T) {
	stm := newTestPersonalityDB(t)
	note, err := stm.InsertCharacterNote(CharacterNote{
		Category: CharacterNoteCategoryValue,
		Text:     "I stay warm and loyal with this user and check in on how they are doing.",
		Source:   CharacterNoteSourceReflection,
	})
	if err != nil {
		t.Fatalf("InsertCharacterNote: %v", err)
	}
	if err := stm.DeleteCharacterNote(note.ID); err != nil {
		t.Fatalf("DeleteCharacterNote: %v", err)
	}
	_, err = stm.InsertCharacterNote(CharacterNote{
		Category: CharacterNoteCategoryValue,
		Text:     "I stay warm and loyal with this user and check in on how they are doing.",
		Source:   CharacterNoteSourceReflection,
	})
	if err == nil || !strings.Contains(err.Error(), "deleted by the user") {
		t.Fatalf("reinsert error = %v, want deleted-by-user", err)
	}
	if notes, _ := stm.ListCharacterNotes(); len(notes) != 0 {
		t.Fatalf("notes = %#v, want empty after delete", notes)
	}
}

func TestDecaySkipsProtectedCharacterNotes(t *testing.T) {
	stm := newTestPersonalityDB(t)
	note, err := stm.InsertCharacterNote(CharacterNote{
		Category:   CharacterNoteCategoryHabit,
		Text:       "I keep a short checklist after every homelab change.",
		Confidence: 0.4,
		Source:     CharacterNoteSourceReflection,
	})
	if err != nil {
		t.Fatalf("InsertCharacterNote: %v", err)
	}
	if err := stm.SetCharacterNoteProtected(note.ID, true); err != nil {
		t.Fatalf("SetCharacterNoteProtected: %v", err)
	}
	if _, err := stm.db.Exec(`UPDATE character_notes SET updated_at = ?`, time.Now().Add(-40*24*time.Hour).UTC().Format("2006-01-02 15:04:05")); err != nil {
		t.Fatalf("age note: %v", err)
	}
	deleted, err := stm.DecayUnprotectedCharacterNotes(time.Now())
	if err != nil {
		t.Fatalf("DecayUnprotectedCharacterNotes: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0 for protected note", deleted)
	}
}

func TestNarrativeVisibleDefaultOff(t *testing.T) {
	stm := newTestPersonalityDB(t)
	if stm.NarrativeVisible() {
		t.Fatal("narrative should default off")
	}
	if err := stm.SetNarrativeVisible(true); err != nil {
		t.Fatalf("SetNarrativeVisible: %v", err)
	}
	if !stm.NarrativeVisible() {
		t.Fatal("expected narrative visible after toggle")
	}
}
