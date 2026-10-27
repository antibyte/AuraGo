package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/memory"
	"aurago/internal/prompts"
)

// The RETRIEVED MEMORIES header explains the trust labels once. It must name
// every label that is not user or app written, so a new agent-side label fails
// here until the note carries it.
func TestMemoryHeaderNamesEveryTrustLabelTheAgentWrites(t *testing.T) {
	var labels []string
	for _, meta := range []memory.MemoryMeta{
		{},
		{SourceType: "memory_analysis"},
		{SourceType: "consolidation"},
		{SourceType: "user"},
		{SourceType: "user", VerificationStatus: "confirmed"},
		{SourceType: "system"},
	} {
		text := memoryTextWithTrust("fact", meta)
		label, _, ok := strings.Cut(strings.TrimPrefix(text, "[trust:"), "]")
		if !ok || !strings.HasPrefix(text, "[trust:") {
			t.Fatalf("memoryTextWithTrust(%+v) = %q, want a [trust:label] prefix", meta, text)
		}
		labels = append(labels, label)
	}

	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if _, err := stm.AddModelCoreFact("User prefers short answers", "session-test", 10, true); err != nil {
		t.Fatalf("AddModelCoreFact: %v", err)
	}
	facts, err := stm.GetCoreMemoryFacts()
	if err != nil || len(facts) != 1 {
		t.Fatalf("GetCoreMemoryFacts = %v, %v", facts, err)
	}
	labels = append(labels, facts[0].Trust)

	flags := prompts.ContextFlags{Tier: "full", TokenBudget: 12000, RetrievedMemories: "[trust:agent-extracted] fact"}
	prompt, _ := prompts.BuildSystemPrompt(t.TempDir(), &flags, "", slog.Default())
	header := prompt[strings.Index(prompt, "[trust:label] says who wrote an entry"):]
	header, _, _ = strings.Cut(header, "\n")
	for _, label := range labels {
		if strings.HasPrefix(label, "user-") || strings.HasPrefix(label, "system-") {
			continue
		}
		if !strings.Contains(header, label) {
			t.Fatalf("memory header does not explain %q: %s", label, header)
		}
	}
}
