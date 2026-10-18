package agent

import (
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"
)

func TestConflictSignalsOnlyCompareTypedSingletonFacts(t *testing.T) {
	language := deriveConflictSignals("User prefers German", conflictRawFact)
	theme := deriveConflictSignals("User prefers dark theme", conflictRawFact)
	if len(language) != 1 || len(theme) != 1 || language[0].Key == theme[0].Key {
		t.Fatalf("unrelated preferences conflict: %v %v", language, theme)
	}
	for _, text := range []string{"User uses Docker", "User uses Proxmox", "User speaks English", "User speaks German", "User prefers German and English", "User prefers tea", "Server is in the rack", "NAS listens on port 80", "NAS listens on port 8443", "NAS runs on port 80", "NAS port is 8443", "NAS port 80"} {
		if got := deriveConflictSignals(text, conflictRawFact); len(got) != 0 {
			t.Fatalf("multi-valued fact nominated: %q: %v", text, got)
		}
	}
	for _, port := range []string{"80", "8443"} {
		if got := deriveConflictSignals("NAS management endpoint default port is "+port, conflictRawFact); len(got) != 1 || got[0].Key != "nas management endpoint|default_port" || got[0].Value != port {
			t.Fatalf("explicit default port lost: %v", got)
		}
	}
}

func TestMemoryRecencyUsesSharedTimestampFormats(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for _, value := range []string{"2026-10-18 12:00:00", "2026-10-18T12:00:00Z", "2026-10-18T14:00:00+02:00", "2026-10-18T12:00:00.000000000Z", "2026-10-19T12:00:00Z"} {
		got := memoryRecencyBonus(memory.MemoryMeta{LastEventAt: value, LastAccessed: value}, now)
		if got != .5 {
			t.Fatalf("timestamp=%q bonus=%v", value, got)
		}
	}
}

func TestPromptCompactionKeepsWholeFactsAndReferenceIDs(t *testing.T) {
	long := strings.Repeat("complete fact with qualifications ", 30)
	got := completeMemoryForPrompt(long, "doc-123", 100)
	if !strings.Contains(got, "[memory:doc-123]") || !strings.Contains(got, "omitted") || len(got) > 100 {
		t.Fatalf("lost teaser: %q", got)
	}
	id := strings.Repeat("x", 200)
	got = buildAvailableMemoryIndex([]rankedMemory{{text: "fact", docID: id}}, 80)
	if strings.Contains(got, "[memory:") || !strings.Contains(got, "omitted") || len(got) > 80 {
		t.Fatalf("partial ID emitted: %q", got)
	}
	got = appendAvailableContextIndex("- [memory:one] fact", "- [kg:"+id+"] fact", 80)
	if strings.Contains(got, "[kg:") || !strings.Contains(got, "omitted") || len(got) > 80 {
		t.Fatalf("partial appended ID: %q", got)
	}
}
