package cydcreator

import (
	"fmt"
	"strings"
)

// The repository has no in-process line diff (internal/tools shells out to
// git diff, which the service cannot depend on), so revisions carry their own
// small unified diff: common prefix and suffix are trimmed, the rest is an LCS
// table bounded by maxDiffCells (and by the caller's cell budget); larger
// changes become one delete+insert hunk.

const (
	diffContext   = 3
	maxDiffCells  = 4_000_000
	maxDiffOutput = 64 << 10
	noNewlineMark = "\\ No newline at end of file\n"
)

type diffOp struct {
	kind byte // ' ' keep, '-' delete, '+' insert
	text string
}

// splitLines splits text after every "\n" and keeps the terminators, so a
// last line without "\n" differs from the same text with one (and "\r\n"
// from "\n"). A final newline does not produce an empty last line.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// diffLines returns the edit script from a to b. An LCS table is used only
// while its cell count fits maxDiffCells and *budget, which it then reduces.
func diffLines(a, b []string, budget *int) []diffOp {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	am, bm := a[pre:len(a)-suf], b[pre:len(b)-suf]
	ops := make([]diffOp, 0, len(a)+len(bm))
	for _, l := range a[:pre] {
		ops = append(ops, diffOp{' ', l})
	}
	if cells := len(am) * len(bm); cells == 0 || cells > min(maxDiffCells, *budget) {
		for _, l := range am {
			ops = append(ops, diffOp{'-', l})
		}
		for _, l := range bm {
			ops = append(ops, diffOp{'+', l})
		}
	} else {
		*budget -= cells
		ops = append(ops, lcsDiff(am, bm)...)
	}
	for _, l := range a[len(a)-suf:] {
		ops = append(ops, diffOp{' ', l})
	}
	return ops
}

// lcsDiff is the classic longest-common-subsequence edit script; dp[i][j] is
// the LCS length of a[i:] and b[j:].
func lcsDiff(a, b []string) []diffOp {
	n, m := len(a), len(b)
	w := m + 1
	dp := make([]int32, (n+1)*w)
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				dp[i*w+j] = dp[(i+1)*w+j+1] + 1
			case dp[(i+1)*w+j] >= dp[i*w+j+1]:
				dp[i*w+j] = dp[(i+1)*w+j]
			default:
				dp[i*w+j] = dp[i*w+j+1]
			}
		}
	}
	ops := make([]diffOp, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case dp[(i+1)*w+j] >= dp[i*w+j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}

// unifiedDiff renders the change of path from oldText to newText with three
// lines of context. added/deleted files use /dev/null on the missing side, a
// last line without newline is followed by "\ No newline at end of file".
// Identical texts return "". The result is capped at 64 KiB.
func unifiedDiff(path, oldText, newText string, added, deleted bool) string {
	cells := maxDiffCells
	return unifiedDiffBudget(path, oldText, newText, added, deleted, &cells)
}

// unifiedDiffBudget is unifiedDiff with a shared LCS cell budget (one
// RevisionDiff call spends it over all of its files).
func unifiedDiffBudget(path, oldText, newText string, added, deleted bool, budget *int) string {
	ops := diffLines(splitLines(oldText), splitLines(newText), budget)
	type span struct{ start, end int }
	var hunks []span
	for i, op := range ops {
		if op.kind == ' ' {
			continue
		}
		start, end := max(0, i-diffContext), min(len(ops)-1, i+diffContext)
		if n := len(hunks); n > 0 && start <= hunks[n-1].end+1 {
			hunks[n-1].end = end
			continue
		}
		hunks = append(hunks, span{start, end})
	}
	if len(hunks) == 0 {
		return ""
	}
	var b strings.Builder
	from, to := "a/"+path, "b/"+path
	if added {
		from = "/dev/null"
	}
	if deleted {
		to = "/dev/null"
	}
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", from, to)
	oldLine, newLine, pos := 1, 1, 0
	for _, h := range hunks {
		for ; pos < h.start; pos++ {
			oldLine++
			newLine++
		}
		oldCount, newCount := 0, 0
		for _, op := range ops[h.start : h.end+1] {
			if op.kind != '+' {
				oldCount++
			}
			if op.kind != '-' {
				newCount++
			}
		}
		oldStart, newStart := oldLine, newLine
		if oldCount == 0 {
			oldStart--
		}
		if newCount == 0 {
			newStart--
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", oldStart, oldCount, newStart, newCount)
		for _, op := range ops[h.start : h.end+1] {
			b.WriteByte(op.kind)
			b.WriteString(op.text)
			if !strings.HasSuffix(op.text, "\n") {
				b.WriteString("\n" + noNewlineMark)
			}
			if op.kind != '+' {
				oldLine++
			}
			if op.kind != '-' {
				newLine++
			}
		}
		pos = h.end + 1
		if b.Len() > maxDiffOutput {
			break
		}
	}
	out := b.String()
	if len(out) > maxDiffOutput {
		cut := strings.LastIndexByte(out[:maxDiffOutput], '\n') + 1
		out = out[:cut] + "… diff truncated …\n"
	}
	return out
}
