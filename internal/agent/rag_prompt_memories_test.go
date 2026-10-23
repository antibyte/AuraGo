package agent

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// ragTestWords builds deterministic, whitespace-separated text of at least n
// bytes (cut at exactly n bytes) whose words are unique, so prefixes are easy
// to assert on.
func ragTestWords(prefix string, n int) string {
	var b strings.Builder
	for i := 0; b.Len() < n; i++ {
		fmt.Fprintf(&b, "%s%03d ", prefix, i)
	}
	return strings.TrimSpace(b.String()[:n])
}

func TestFilterShortQueryRAGMemoriesDropsAllLowScores(t *testing.T) {
	ranked := []rankedMemory{
		{text: "old error message", docID: "a", score: 0.49},
		{text: "another old error", docID: "b", score: 0.31},
	}
	got := filterShortQueryRAGMemories("versuche es erneut", ranked)
	if len(got) != 0 {
		t.Fatalf("short query with only low-score memories must serve nothing, got %+v", got)
	}
}

func TestFilterShortQueryRAGMemoriesKeepsOnlyStrongEntries(t *testing.T) {
	ranked := []rankedMemory{
		{text: "weak", docID: "a", score: 0.40},
		{text: "strong", docID: "b", score: 0.50},
		{text: "weaker", docID: "c", score: 0.10},
	}
	got := filterShortQueryRAGMemories("backup status", ranked)
	if len(got) != 1 || got[0].docID != "b" {
		t.Fatalf("got %+v, want only docID b (score >= 0.50)", got)
	}
}

func TestFilterShortQueryRAGMemoriesLeavesLongMessagesUntouched(t *testing.T) {
	ranked := []rankedMemory{
		{text: "weak", docID: "a", score: 0.10},
		{text: "strong", docID: "b", score: 0.90},
	}
	msg := strings.Repeat("x", 40) // 40 bytes is no longer "short"
	got := filterShortQueryRAGMemories(msg, ranked)
	if len(got) != len(ranked) || got[0].docID != "a" || got[1].docID != "b" {
		t.Fatalf("long message must keep every entry in order, got %+v", got)
	}
	if got := filterShortQueryRAGMemories("hi", nil); len(got) != 0 {
		t.Fatalf("empty input must stay empty, got %+v", got)
	}
}

func TestCompleteMemoryForPromptExcerptsOverlongMemory(t *testing.T) {
	text := ragTestWords("fact", 1000)
	const maxBytes = aggressiveAutoRAGMemoryChars

	got := completeMemoryForPrompt(text, "doc-42", maxBytes)

	if len(got) > maxBytes {
		t.Fatalf("entry is %d bytes, want <= %d: %q", len(got), maxBytes, got)
	}
	if !strings.HasPrefix(got, text[:100]) {
		t.Fatalf("entry must start with the beginning of the memory, got %q", got)
	}
	if !strings.Contains(got, "[memory:doc-42]") || !strings.Contains(got, "recall_memory") {
		t.Fatalf("entry lost the recall pointer: %q", got)
	}
	if !strings.HasSuffix(got, " … [memory:doc-42] (truncated; use recall_memory for the full text)") {
		t.Fatalf("entry must end with the truncation pointer, got %q", got)
	}
	if strings.Contains(got, "content omitted") {
		t.Fatalf("an excerpt must not carry the omission pointer: %q", got)
	}
	// The cut must fall on a whitespace boundary, not inside a word.
	excerpt := strings.TrimSuffix(got, " … [memory:doc-42] (truncated; use recall_memory for the full text)")
	if !strings.HasPrefix(text, excerpt) || !strings.HasPrefix(text[len(excerpt):], " ") {
		t.Fatalf("excerpt is not cut at a word boundary: %q", excerpt)
	}
	if len(excerpt) < 150 {
		t.Fatalf("excerpt keeps only %d bytes, want most of the available space", len(excerpt))
	}
}

func TestCompleteMemoryForPromptExcerptCutsOnRuneBoundary(t *testing.T) {
	// No whitespace at all and every rune is 2 bytes: the cut has to fall back
	// to a rune boundary.
	text := strings.Repeat("ä", 600)
	got := completeMemoryForPrompt(text, "doc-7", 300)
	if !utf8.ValidString(got) {
		t.Fatalf("excerpt is not valid UTF-8: %q", got)
	}
	if len(got) > 300 {
		t.Fatalf("entry is %d bytes, want <= 300", len(got))
	}
	if !strings.HasPrefix(got, "äääää") || !strings.Contains(got, "[memory:doc-7]") {
		t.Fatalf("unexpected excerpt: %q", got)
	}
}

func TestCompleteMemoryForPromptFallsBackToPointerWhenCapIsTiny(t *testing.T) {
	text := ragTestWords("fact", 1000)
	got := completeMemoryForPrompt(text, "doc-123", 100)
	if got != "[memory:doc-123] (content omitted; use recall_memory)" {
		t.Fatalf("tiny cap must keep the bare pointer, got %q", got)
	}
}

func TestCompleteMemoryForPromptWithoutDocIDKeepsOldBehaviour(t *testing.T) {
	text := ragTestWords("fact", 1000)
	got := completeMemoryForPrompt(text, "", 200)
	if len(got) > 200 || strings.Contains(got, "[memory:") || strings.Contains(got, "recall_memory") {
		t.Fatalf("without a docID no pointer may be emitted, got %q", got)
	}
	if short := completeMemoryForPrompt("  small fact  ", "doc-1", 200); short != "small fact" {
		t.Fatalf("fitting memory must be returned sanitized and whole, got %q", short)
	}
}

func TestBuildAggressiveRAGPromptEntriesPacksAllServedInOrder(t *testing.T) {
	served := []rankedMemory{
		{text: "first short memory", docID: "mem-1", score: 0.9},
		{text: "second short memory", docID: "mem-2", score: 0.8},
		{text: "third short memory", docID: "mem-3", score: 0.7},
	}
	entries := buildAggressiveRAGPromptEntries(served, false, nil)
	if len(entries) != 3 {
		t.Fatalf("entries len = %d, want 3: %+v", len(entries), entries)
	}
	for i, want := range []string{"mem-1", "mem-2", "mem-3"} {
		if entries[i].docID != want || entries[i].text != served[i].text {
			t.Fatalf("entry %d = %+v, want docID %s with its full text", i, entries[i], want)
		}
	}
	if got := len(joinedPromptEntries(entries)); got > aggressiveRetrievedMemoriesChars {
		t.Fatalf("joined entries are %d bytes, want <= %d", got, aggressiveRetrievedMemoriesChars)
	}
}

func TestBuildAggressiveRAGPromptEntriesRespectsTotalBudget(t *testing.T) {
	served := make([]rankedMemory, 0, 6)
	for i := 0; i < 6; i++ {
		served = append(served, rankedMemory{
			text:  ragTestWords(fmt.Sprintf("m%d_", i), 1000),
			docID: fmt.Sprintf("mem-%d", i),
			score: 0.9 - float64(i)/100,
		})
	}
	entries := buildAggressiveRAGPromptEntries(served, false, nil)
	if len(entries) < 2 || len(entries) >= len(served) {
		t.Fatalf("entries len = %d, want some but not all of %d served memories", len(entries), len(served))
	}
	if got := len(joinedPromptEntries(entries)); got > aggressiveRetrievedMemoriesChars {
		t.Fatalf("joined entries are %d bytes, want <= %d", got, aggressiveRetrievedMemoriesChars)
	}
	for i, entry := range entries {
		if entry.docID != served[i].docID {
			t.Fatalf("entry %d docID = %q, want %q (order must be kept)", i, entry.docID, served[i].docID)
		}
		if len(entry.text) > aggressiveAutoRAGMemoryChars {
			t.Fatalf("entry %d is %d bytes, want <= %d", i, len(entry.text), aggressiveAutoRAGMemoryChars)
		}
		if !strings.Contains(entry.text, "[memory:"+entry.docID+"]") {
			t.Fatalf("entry %d is long and must carry its recall pointer: %q", i, entry.text)
		}
	}
	// The last kept entry is squeezed by the remaining budget, not by its own cap.
	last := entries[len(entries)-1]
	if len(last.text) >= aggressiveAutoRAGMemoryChars {
		t.Fatalf("last entry (%d bytes) should be shrunk to the remaining budget", len(last.text))
	}
}

func TestBuildAggressiveRAGPromptEntriesDeepDetailsOnlyForFirst(t *testing.T) {
	served := []rankedMemory{
		{text: "short", docID: "mem-1", score: 0.95},
		{text: "second compact memory", docID: "mem-2", score: 0.90},
	}
	var fetched []string
	entries := buildAggressiveRAGPromptEntries(served, true, func(id string) (string, error) {
		fetched = append(fetched, id)
		return strings.Repeat("full verified detail ", 80), nil
	})
	if len(fetched) != 1 || fetched[0] != "mem-1" {
		t.Fatalf("full text fetched for %v, want only mem-1", fetched)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2: %+v", len(entries), entries)
	}
	if !strings.HasPrefix(entries[0].text, "[Detailed Memory]\n") || len(entries[0].text) > aggressiveDetailedRAGMemoryChars {
		t.Fatalf("first entry must be the capped detailed memory, got %d bytes: %q", len(entries[0].text), entries[0].text)
	}
	if entries[1].text != "second compact memory" {
		t.Fatalf("second entry must stay compact, got %q", entries[1].text)
	}
}

func joinedPromptEntries(entries []promptMemoryEntry) string {
	texts := make([]string, 0, len(entries))
	for _, entry := range entries {
		texts = append(texts, entry.text)
	}
	return strings.Join(texts, "\n---\n")
}
