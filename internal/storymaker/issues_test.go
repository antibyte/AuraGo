package storymaker

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCapIssues(t *testing.T) {
	var issues []Issue
	for i := 0; i < 5; i++ {
		issues = append(issues, Issue{Severity: SeverityMinor, Path: fmt.Sprintf("/m%d", i)})
	}
	issues = append(issues, Issue{Severity: SeverityError, Path: "/e1"}, Issue{Severity: SeverityWarning, Path: "/w1"}, Issue{Severity: SeverityError, Path: "/e2"})
	got := CapIssues(issues, 3)
	if len(got) != 3 || got[0].Path != "/e1" || got[1].Path != "/w1" || got[2].Path != "/e2" {
		t.Fatalf("errors first, then warnings, in document order: %+v", got)
	}
	if got := CapIssues(issues, 0); len(got) != len(issues) {
		t.Fatal("n <= 0 must keep everything")
	}
	if got := CapIssues(issues, MaxModelIssues); len(got) != MaxModelIssues {
		t.Fatalf("model cap: %d", len(got))
	}
}

func TestClosestNames(t *testing.T) {
	pool := []string{"stand", "sit", "typing", "point-left", "point-right", "wave", "think"}
	cases := []struct {
		input string
		want  []string // the best matches, in order; the rest is filler
	}{
		{"typin", []string{"typing"}},
		{"sitting", []string{"sit"}},
		{"pointing", []string{"point-left", "point-right"}},
		{"pose.wave", []string{"wave"}},
		{"Point_Left", []string{"point-left", "point-right"}},
	}
	for _, tc := range cases {
		got := closestNames(tc.input, pool, 3)
		if len(got) != 3 || !reflect.DeepEqual(got[:len(tc.want)], tc.want) {
			t.Errorf("closestNames(%q) = %v, want it to start with %v", tc.input, got, tc.want)
		}
	}
	if got := closestNames("x", nil, 5); len(got) != 0 {
		t.Errorf("empty pool: %v", got)
	}
	if got := closestNames("a", []string{"a", "a", "b"}, 5); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("duplicates: %v", got)
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) || HasErrors([]Issue{{Severity: SeverityWarning}, {Severity: SeverityMinor}}) || !HasErrors([]Issue{{Severity: SeverityMinor}, {Severity: SeverityError}}) {
		t.Fatal("HasErrors")
	}
}
