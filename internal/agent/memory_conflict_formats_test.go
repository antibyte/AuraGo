package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestConflictFormatsPreserveMultilineFact(t *testing.T) {
	raw := "User prefers Emacs\n\nThis applies to the editor."
	want := normalizeConflictText(raw, conflictRawFact)
	for _, document := range []string{"Editor preference\n\n" + raw,
		"[preference:workflow] " + raw + "\n\nsource:memory_analysis",
		"[preference:workflow] " + raw + "\n\nsource:memory_analysis session:old"} {
		if got := normalizeConflictText(document, conflictStoredDocument); got != want {
			t.Fatalf("stored=%q raw=%q", got, want)
		}
	}
	for _, raw := range []string{"[custom] User prefers Vim", "[Similarity: evidence] User prefers Vim", "User prefers Vim\n\nsource:memory_analysis session:human text"} {
		if got := normalizeConflictText(raw, conflictRawFact); got != strings.Join(strings.Fields(raw), " ") {
			t.Fatalf("raw erased: %q", got)
		}
	}
	stm, _ := newMemorySafetyStore(t)
	for _, id := range []string{"old", "new"} {
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	v := &memorySafetyVector{archiveFilterVectorDB: archiveFilterVectorDB{byQuery: map[string][]memory.SearchResult{
		"user|preference": {{DocID: "old", Text: "Editor preference\n\nUser prefers Vim", Similarity: .99}},
	}}, stored: map[string]string{"new": "[preference:workflow] " + raw + "\n\nsource:memory_analysis"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := detectMemoryConflictsForDocIDsWithContext(t.Context(), logger, stm, v, []string{"new"}, raw, conflictRawFact); err != nil {
		t.Fatal(err)
	}
	first, _ := stm.GetOpenMemoryConflicts(10)
	if err := detectMemoryConflictsForDocIDsWithContext(t.Context(), logger, stm, v, []string{"new"}, v.stored["new"], conflictStoredDocument); err != nil {
		t.Fatal(err)
	}
	if err := detectMemoryConflictsForDocIDsWithContext(t.Context(), logger, stm, v, []string{"new"}, "", conflictStoredDocument); err != nil {
		t.Fatal(err)
	}
	last, err := stm.GetOpenMemoryConflicts(10)
	if err != nil || len(first) != 1 || len(last) != 1 || first[0].LeftValue != last[0].LeftValue || first[0].RightValue != last[0].RightValue {
		t.Fatalf("immediate=%+v scanned=%+v err=%v", first, last, err)
	}
}
