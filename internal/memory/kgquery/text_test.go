package kgquery

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFitEntriesKeepsCompleteUnicodeReferencesWithinBudget(t *testing.T) {
	first := "- [memory:ä-one] a complete fact"
	second := "- [kg:" + strings.Repeat("界", 30) + "] another fact"
	for _, budget := range []int{1, 20, 32, 64, 180} {
		got := FitEntries([]string{first, second}, "\n", budget)
		if len(got) > budget || !utf8.ValidString(got) {
			t.Fatalf("budget %d: invalid output %q", budget, got)
		}
		for _, line := range strings.Split(got, "\n") {
			if line != "" && line != first && line != second && line != "[additional entries omitted]" {
				t.Fatalf("partial entry at budget %d: %q", budget, line)
			}
		}
	}
}
