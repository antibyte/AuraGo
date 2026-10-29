package agent

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/memory"
	"aurago/internal/prompts"
)

var followUpSectionHeaders = []string{"[Pending Follow-Ups]", "[Recent Day Anchors]", "[Last 72h Episodes]"}

// ragFollowUpTestSections renders the three follow-up sections with content
// long enough to hit their 230 byte cap, i.e. the largest they can get.
func ragFollowUpTestSections() []string {
	return formatFollowUpPromptSections(
		[]memory.EpisodicMemory{{
			EventDate:    "2026-10-24",
			Title:        "Renew router certificate",
			Summary:      strings.Repeat("renew the certificate before it expires ", 12),
			TriggerQuery: "certificate",
		}},
		[]string{strings.Repeat("anchor line ", 40)},
		[]string{strings.Repeat("episode line ", 40)},
	)
}

func ragLongServedMemories(n int) []rankedMemory {
	served := make([]rankedMemory, 0, n)
	for i := 0; i < n; i++ {
		served = append(served, rankedMemory{
			text:  ragTestWords(fmt.Sprintf("mem%d_", i), 1000),
			docID: fmt.Sprintf("mem-%d", i),
			score: 0.9 - float64(i)/100,
		})
	}
	return served
}

func TestFormatFollowUpPromptSectionsKeepsHeadersAndCaps(t *testing.T) {
	sections := ragFollowUpTestSections()
	if len(sections) != len(followUpSectionHeaders) {
		t.Fatalf("sections = %d, want %d: %q", len(sections), len(followUpSectionHeaders), sections)
	}
	for i, header := range followUpSectionHeaders {
		if !strings.HasPrefix(sections[i], header) {
			t.Fatalf("section %d = %q, want header %q", i, sections[i], header)
		}
		if len(sections[i]) > followUpPromptSectionChars {
			t.Fatalf("section %d is %d bytes, want <= %d", i, len(sections[i]), followUpPromptSectionChars)
		}
	}
	if got := formatFollowUpPromptSections(nil, nil, nil); len(got) != 0 {
		t.Fatalf("no input must give no sections, got %q", got)
	}
}

// With several essential memories (or the deep-details top memory plus a
// second one) the follow-up sections used to be appended behind a section that
// was already full, so the final fit dropped them. The packer has to leave their
// room free, and the final fit then has nothing to cut.
func TestRAGPackingLeavesRoomForFollowUpSections(t *testing.T) {
	deepFull := func(string) (string, error) { return strings.Repeat("full verified detail ", 80), nil }
	tests := []struct {
		name      string
		served    int
		deep      bool
		getFull   func(string) (string, error)
		wantAtLst int // at least this many memories must still be packed
	}{
		{name: "three long essential memories", served: 3, wantAtLst: 2},
		{name: "six long essential memories", served: 6, wantAtLst: 2},
		{name: "two essential memories with deep details", served: 2, deep: true, getFull: deepFull, wantAtLst: 1},
		{name: "one essential memory with deep details", served: 1, deep: true, getFull: deepFull, wantAtLst: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			served := ragLongServedMemories(tt.served)
			sections := ragFollowUpTestSections()

			entries := buildAggressiveRAGPromptEntriesWithin(served, tt.deep, tt.getFull, ragMemoryBudgetAfterSections(sections))
			if len(entries) < tt.wantAtLst {
				t.Fatalf("packed %d memories, want at least %d", len(entries), tt.wantAtLst)
			}
			texts := make([]string, 0, len(entries))
			for _, entry := range entries {
				texts = append(texts, entry.text)
			}
			assembled := appendPromptSections(strings.Join(texts, ragPromptEntrySeparator), sections)

			flags := &prompts.ContextFlags{RetrievedMemories: assembled}
			applyAggressivePromptContextBudgets(flags)

			for _, header := range followUpSectionHeaders {
				if !strings.Contains(flags.RetrievedMemories, header) {
					t.Fatalf("section %q was cut from the prompt:\n%s", header, flags.RetrievedMemories)
				}
			}
			if strings.Contains(flags.RetrievedMemories, "[additional entries omitted]") {
				t.Fatalf("final fit still cut entries:\n%s", flags.RetrievedMemories)
			}
			if flags.RetrievedMemories != assembled {
				t.Fatalf("final fit changed the packed section\nbefore: %q\nafter:  %q", assembled, flags.RetrievedMemories)
			}
			if len(flags.RetrievedMemories) > aggressiveRetrievedMemoriesChars {
				t.Fatalf("section is %d bytes, want <= %d", len(flags.RetrievedMemories), aggressiveRetrievedMemoriesChars)
			}
			// Every memory that is counted as used is whole in the final text, and
			// the memories come before the follow-up sections.
			firstSection := strings.Index(flags.RetrievedMemories, followUpSectionHeaders[0])
			for _, entry := range entries {
				at := strings.Index(flags.RetrievedMemories, entry.text)
				if at < 0 {
					t.Fatalf("memory %s is not (completely) in the final text:\n%s", entry.docID, flags.RetrievedMemories)
				}
				if at > firstSection {
					t.Fatalf("memory %s stands behind the follow-up sections", entry.docID)
				}
			}
		})
	}
}

func TestAppendPromptSectionsKeepsOrderAndSeparators(t *testing.T) {
	if got := appendPromptSections("", []string{"a", "b"}); got != "a\n---\nb" {
		t.Fatalf("got %q, want sections joined without leading separator", got)
	}
	if got := appendPromptSections("mem", []string{"a", "b"}); got != "mem\n---\na\n---\nb" {
		t.Fatalf("got %q, want memories first, then the sections", got)
	}
	if got := appendPromptSections("mem", nil); got != "mem" {
		t.Fatalf("got %q, want the memories untouched", got)
	}
}

func TestRAGMemoryBudgetAfterSections(t *testing.T) {
	if got := ragMemoryBudgetAfterSections(nil); got != aggressiveRetrievedMemoriesChars {
		t.Fatalf("no sections: budget = %d, want the full %d", got, aggressiveRetrievedMemoriesChars)
	}
	sections := ragFollowUpTestSections()
	reserved := 0
	for _, section := range sections {
		reserved += len(ragPromptEntrySeparator) + len(section)
	}
	if got, want := ragMemoryBudgetAfterSections(sections), aggressiveRetrievedMemoriesChars-reserved; got != want {
		t.Fatalf("budget = %d, want %d (separator and section of each follow-up reserved)", got, want)
	}
	if got := ragMemoryBudgetAfterSections([]string{strings.Repeat("x", 2*aggressiveRetrievedMemoriesChars)}); got != 0 {
		t.Fatalf("oversized sections: budget = %d, want 0", got)
	}
}

// A memory that the packer drops never reached the prompt, so it must not be
// counted as used: no access bump, no usage record, no entry in the used-ID set.
func TestRecordPackedRAGMemoriesCountsOnlyPackedMemories(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	served := ragLongServedMemories(6)
	for _, item := range served {
		if err := stm.UpsertMemoryMeta(item.docID); err != nil {
			t.Fatalf("UpsertMemoryMeta(%s): %v", item.docID, err)
		}
	}
	entries := buildAggressiveRAGPromptEntriesWithin(served, false, nil, ragMemoryBudgetAfterSections(ragFollowUpTestSections()))
	if len(entries) < 1 || len(entries) >= len(served) {
		t.Fatalf("packed %d of %d memories, the scenario needs some but not all", len(entries), len(served))
	}

	used := map[string]int{}
	recordPackedRAGMemories(stm, "sess-1", used, served, entries)

	packed := map[string]bool{}
	for _, entry := range entries {
		packed[entry.docID] = true
	}
	for _, item := range served {
		meta, err := stm.GetMemoryMeta(item.docID)
		if err != nil {
			t.Fatalf("GetMemoryMeta(%s): %v", item.docID, err)
		}
		wantAccess, wantUsed := 0, 0
		if packed[item.docID] {
			wantAccess, wantUsed = 1, 1
		}
		if meta.AccessCount != wantAccess {
			t.Errorf("%s: access count = %d, want %d (packed=%v)", item.docID, meta.AccessCount, wantAccess, packed[item.docID])
		}
		if used[item.docID] != wantUsed {
			t.Errorf("%s: used-ID count = %d, want %d (packed=%v)", item.docID, used[item.docID], wantUsed, packed[item.docID])
		}
	}
	stats, err := stm.GetMemoryUsageStats(1, 10)
	if err != nil {
		t.Fatalf("GetMemoryUsageStats: %v", err)
	}
	if stats.DistinctMemories != len(entries) || stats.RetrievedEvents != len(entries) {
		t.Fatalf("usage log has %d memories / %d retrieved events, want %d packed memories", stats.DistinctMemories, stats.RetrievedEvents, len(entries))
	}
}

func TestRecordPackedRAGMemoriesWithoutStoreStillMarksUsedIDs(t *testing.T) {
	served := []rankedMemory{{text: "a", docID: "mem-a", score: 0.9}, {text: "b", docID: "mem-b", score: 0.8}, {text: "c", score: 0.7}}
	entries := []promptMemoryEntry{{docID: "mem-a", text: "a"}, {text: "c"}}
	used := map[string]int{}
	recordPackedRAGMemories(nil, "sess-1", used, served, entries)
	if len(used) != 1 || used["mem-a"] != 1 {
		t.Fatalf("used = %v, want only mem-a (mem-b was not packed, the entry without docID cannot be tracked)", used)
	}
}

func TestBuildFollowUpPromptSectionsReadsPendingActions(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.UpsertPendingEpisodicAction("2026-04-01", "Help with Nextcloud", "Prepare Docker Compose setup", "nextcloud docker", "sess-1", 3, nil); err != nil {
		t.Fatalf("UpsertPendingEpisodicAction: %v", err)
	}

	sections, pending := buildFollowUpPromptSections(stm, "please help with nextcloud")
	if len(pending) != 1 || len(sections) == 0 {
		t.Fatalf("sections = %q, pending = %d, want the pending action and its section", sections, len(pending))
	}
	if want := "[Pending Follow-Ups]\n- 2026-04-01 | Help with Nextcloud — Prepare Docker Compose setup | trigger: nextcloud docker"; sections[0] != want {
		t.Fatalf("pending section = %q, want %q", sections[0], want)
	}
	if sections, pending := buildFollowUpPromptSections(nil, "nextcloud"); sections != nil || pending != nil {
		t.Fatalf("no store must give nothing, got %q / %v", sections, pending)
	}
	if sections, pending := buildFollowUpPromptSections(stm, ""); sections != nil || pending != nil {
		t.Fatalf("empty message must give nothing, got %q / %v", sections, pending)
	}
}
