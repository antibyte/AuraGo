package kgquality

import "testing"

func TestIsGenericEntityToken(t *testing.T) {
	tests := map[string]bool{
		"png":                true,
		".jpeg":              true,
		"rgba8":              true,
		"x86_64":             true,
		"attachment_folders": true,
		"server":             false,
		"caddy_server":       false,
		"agodesk":            false,
	}
	for input, want := range tests {
		if got := IsGenericEntity(input); got != want {
			t.Fatalf("IsGenericEntity(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFileNodeIDIsStableAndPathBacked(t *testing.T) {
	left := FileNodeID(`/home/aurago/aurago/data/documents/test_pdf.pdf`)
	right := FileNodeID(`\home\aurago\aurago\data\documents\test_pdf.pdf`)
	if left == "" || left != right {
		t.Fatalf("FileNodeID should be stable across separators, got %q and %q", left, right)
	}
	if len(left) != len("file_")+12 {
		t.Fatalf("FileNodeID length = %d, want %d", len(left), len("file_")+12)
	}
}

func TestLowConfidenceCoMention(t *testing.T) {
	policy := DefaultPolicy()
	if !LowConfidenceCoMention("co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}, policy) {
		t.Fatal("pending weight=1 co-mention should be low confidence")
	}
	if !LowConfidenceCoMention("co_mentioned_with", map[string]string{"source": "activity_turn", "weight": "1"}, policy) {
		t.Fatal("activity_turn weight below threshold should be low confidence")
	}
	if LowConfidenceCoMention("co_mentioned_with", map[string]string{"source": "activity_turn", "weight": "15"}, policy) {
		t.Fatal("promoted high-weight co-mention should not be low confidence")
	}
	if LowConfidenceCoMention("uses", map[string]string{"source": "manual"}, policy) {
		t.Fatal("semantic manual edge should not be treated as low-confidence co-mention")
	}
}
