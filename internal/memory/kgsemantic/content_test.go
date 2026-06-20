package kgsemantic

import "testing"

func TestShouldSkipQuery(t *testing.T) {
	cases := []struct {
		query string
		want  bool
	}{
		{"", true},
		{"*", true},
		{"a", true},
		{"pi", true},
		{"pve01", false},
		{"HomeAssistant", false},
		{"long enough query", false},
	}
	for _, tc := range cases {
		if got := ShouldSkipQuery(tc.query); got != tc.want {
			t.Fatalf("ShouldSkipQuery(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func TestBuildNodeContentSkipsOperationalProperties(t *testing.T) {
	content := BuildNodeContent(NodeContent{
		ID:    "node-1",
		Label: "Router",
		Properties: map[string]string{
			"type":         "device",
			"source":       "inventory",
			"extracted_at": "2026-01-01",
			"ip":           "192.168.0.1",
		},
	})
	if content == "" {
		t.Fatal("expected non-empty semantic content")
	}
	if contains(content, "source:") || contains(content, "extracted_at:") {
		t.Fatalf("operational properties leaked into content: %q", content)
	}
}

func TestBuildEdgeContentPreservesDotSeparator(t *testing.T) {
	content := BuildEdgeContent(EdgeContent{
		Source:   "app",
		Target:   "server",
		Relation: "runs_on",
		Properties: map[string]string{
			"notes": "nightly workload",
		},
	})
	want := "runs_on. app runs_on server. notes: nightly workload"
	if content != want {
		t.Fatalf("BuildEdgeContent() = %q, want %q", content, want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}