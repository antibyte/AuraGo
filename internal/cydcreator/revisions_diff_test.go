package cydcreator

import (
	"fmt"
	"strings"
	"testing"
)

func numbered(from, to int, change map[int]string) string {
	var b strings.Builder
	for i := from; i <= to; i++ {
		line := fmt.Sprintf("line %d", i)
		if c, ok := change[i]; ok {
			line = c
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func TestUnifiedDiff(t *testing.T) {
	old := numbered(1, 10, nil)
	got := unifiedDiff("src/app/a.cpp", old, numbered(1, 10, map[int]string{5: "five"}), false, false)
	want := "--- a/src/app/a.cpp\n+++ b/src/app/a.cpp\n@@ -2,7 +2,7 @@\n line 2\n line 3\n line 4\n-line 5\n+five\n line 6\n line 7\n line 8\n"
	if got != want {
		t.Fatalf("modify:\n%s\nwant:\n%s", got, want)
	}
	if got := unifiedDiff("src/app/x.cpp", "", "a\nb\n", true, false); got != "--- /dev/null\n+++ b/src/app/x.cpp\n@@ -0,0 +1,2 @@\n+a\n+b\n" {
		t.Fatalf("added:\n%s", got)
	}
	if got := unifiedDiff("src/app/x.cpp", "a\nb\n", "", false, true); got != "--- a/src/app/x.cpp\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-a\n-b\n" {
		t.Fatalf("deleted:\n%s", got)
	}
	if got := unifiedDiff("p", old, old, false, false); got != "" {
		t.Fatalf("identical: %q", got)
	}
	two := unifiedDiff("p", numbered(1, 30, nil), numbered(1, 30, map[int]string{3: "three", 27: "twenty-seven"}), false, false)
	if strings.Count(two, "@@ ") != 2 || !strings.Contains(two, "@@ -1,6 +1,6 @@") || !strings.Contains(two, "@@ -24,7 +24,7 @@") {
		t.Fatalf("two hunks:\n%s", two)
	}
	inserted := unifiedDiff("p", "a\nc\n", "a\nb\nc\n", false, false)
	if inserted != "--- a/p\n+++ b/p\n@@ -1,2 +1,3 @@\n a\n+b\n c\n" {
		t.Fatalf("insert:\n%s", inserted)
	}
}

func TestUnifiedDiffLineEndings(t *testing.T) {
	if got := unifiedDiff("p", "a\nb", "a\nb\n", false, false); got != "--- a/p\n+++ b/p\n@@ -1,2 +1,2 @@\n a\n-b\n\\ No newline at end of file\n+b\n" {
		t.Fatalf("final newline added:\n%s", got)
	}
	if got := unifiedDiff("p", "a\n", "a", false, false); got != "--- a/p\n+++ b/p\n@@ -1,1 +1,1 @@\n-a\n+a\n\\ No newline at end of file\n" {
		t.Fatalf("final newline removed:\n%s", got)
	}
	if got := unifiedDiff("p", "x", "", false, true); got != "--- a/p\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-x\n\\ No newline at end of file\n" {
		t.Fatalf("deleted without newline:\n%s", got)
	}
	if got := unifiedDiff("p", "a\r\nb\r\n", "a\nb\n", false, false); got != "--- a/p\n+++ b/p\n@@ -1,2 +1,2 @@\n-a\r\n-b\r\n+a\n+b\n" {
		t.Fatalf("CRLF to LF:\n%q", got)
	}
	if got := unifiedDiff("p", "a\r\nb\r\n", "a\r\nb\r\n", false, false); got != "" {
		t.Fatalf("identical CRLF text: %q", got)
	}
}

func TestUnifiedDiffSpendsTheCellBudget(t *testing.T) {
	old, changed := "x\na\ny\n", "z\na\nw\n"
	budget := 100
	if got := unifiedDiffBudget("p", old, changed, false, false, &budget); !strings.Contains(got, "\n a\n") || budget != 91 {
		t.Fatalf("with budget: %q, %d cells left", got, budget)
	}
	budget = 8
	if got := unifiedDiffBudget("p", old, changed, false, false, &budget); strings.Contains(got, "\n a\n") || budget != 8 {
		t.Fatalf("over budget: %q, %d cells left", got, budget)
	}
}
