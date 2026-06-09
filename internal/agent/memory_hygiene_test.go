package agent

import (
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestBuildMemoryReflectionReviewIssueTriggersOnActionableCuratorCounts(t *testing.T) {
	issue, ok := buildMemoryReflectionReviewIssue("recent", memory.MemoryCuratorDryRun{
		StaleCandidates:     30,
		VerificationBacklog: 75,
		Contradictions:      1,
	})
	if !ok {
		t.Fatal("expected memory reflection review issue")
	}
	if issue.Fingerprint != "memory_reflect|recent|curator_review" {
		t.Fatalf("fingerprint = %q, want stable memory reflection fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "contradictions=1") || !strings.Contains(issue.Detail, "verification_backlog=75") {
		t.Fatalf("issue detail = %q, want curator counts", issue.Detail)
	}
}

func TestBuildMemoryReflectionReviewIssueSkipsNoise(t *testing.T) {
	if _, ok := buildMemoryReflectionReviewIssue("recent", memory.MemoryCuratorDryRun{StaleCandidates: 2}); ok {
		t.Fatal("unexpected issue for low curator noise")
	}
}

func TestBuildKnowledgeGraphSparseIssueRequiresCoreFacts(t *testing.T) {
	if _, ok := buildKnowledgeGraphSparseIssue(nil, 0, 0); ok {
		t.Fatal("unexpected issue without core facts")
	}
	issue, ok := buildKnowledgeGraphSparseIssue([]string{"User: Andi", "Agent: Nova"}, 1, 0)
	if !ok {
		t.Fatal("expected sparse KG issue with core facts")
	}
	if issue.Fingerprint != "memory_maintenance|kg_sparse|core_facts_present" {
		t.Fatalf("fingerprint = %q, want stable sparse KG fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "core_facts=2") || !strings.Contains(issue.Detail, "nodes=1") {
		t.Fatalf("issue detail = %q, want KG counts", issue.Detail)
	}
}

func TestBuildCoreMemoryReviewIssueFlagsTestFacts(t *testing.T) {
	issue, ok := buildCoreMemoryReviewIssue([]string{"This is a test fact", "User: Andi"})
	if !ok {
		t.Fatal("expected core memory review issue for test fact")
	}
	if issue.Fingerprint != "memory_maintenance|core_memory_review|low_signal" {
		t.Fatalf("fingerprint = %q, want stable core memory review fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "test fact") {
		t.Fatalf("issue detail = %q, want test fact detail", issue.Detail)
	}
}
