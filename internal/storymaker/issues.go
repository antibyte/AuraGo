package storymaker

import (
	"encoding/json"
	"sort"
	"strings"
	"unicode"
)

// MaxModelIssues is the number of issues a model-facing answer carries.
const MaxModelIssues = 8

// MaxHTTPIssues caps the issues of an HTTP save answer.
const MaxHTTPIssues = 200

// CapIssues keeps at most n issues, preferring errors over warnings over minor notes, and
// returns them in their original (document) order. n <= 0 returns the issues unchanged.
func CapIssues(issues []Issue, n int) []Issue {
	if n <= 0 || len(issues) <= n {
		return issues
	}
	keep := make([]bool, len(issues))
	left := n
	for _, sev := range []string{SeverityError, SeverityWarning, SeverityMinor} {
		for i, is := range issues {
			if left == 0 {
				break
			}
			if is.Severity == sev && !keep[i] {
				keep[i] = true
				left--
			}
		}
	}
	out := make([]Issue, 0, n)
	for i, is := range issues {
		if keep[i] {
			out = append(out, is)
		}
	}
	return out
}

// HasErrors reports whether any issue is an error (renders are blocked).
func HasErrors(issues []Issue) bool {
	for _, is := range issues {
		if is.Severity == SeverityError {
			return true
		}
	}
	return false
}

// IssueModelFixable says whether the agent can fix an issue with its story tools: every error of
// Validate, scene_unfilled, and compile_failed that names a scene. Code scenes and render deadlines
// are outside its tools.
func IssueModelFixable(i Issue) bool {
	if IssueCraft(i.Code) || IssueLayout(i.Code) {
		return true
	}
	switch i.Code {
	case CodeSceneUnfilled:
		return true
	case CodeCompileFailed:
		return i.SceneID != ""
	case CodeCodeError, CodeDeadline:
		return false
	}
	return i.Severity == SeverityError
}

// IssueCraft says whether a code is a film-level craft finding: the film is valid but looks uniform. The model can fix
// it with its story tools, so IssueModelFixable is true for it and the check loop runs while rounds remain.
func IssueCraft(code string) bool {
	switch code {
	case CodeEnvironmentMonotony, CodeFramingMonotony, CodeGestureMonotony, CodeCastUnused:
		return true
	}
	return false
}

// issueNeverBlocks says whether a model-fixable code stays an open check instead of turning the outcome into
// incomplete. Craft codes and, from batch D on, layout codes never block.
func issueNeverBlocks(code string) bool {
	return IssueCraft(code) || IssueLayout(code)
}

// HasBlockingIssue reports whether any of the model-fixable issues of a report would turn the outcome into incomplete.
// Craft findings are in the list so the check loop can fix them while rounds remain, but they never block, so they do not
// earn the extra check loop after narration either.
func HasBlockingIssue(issues []Issue) bool {
	for _, is := range issues {
		if !issueNeverBlocks(is.Code) {
			return true
		}
	}
	return false
}

// IssueAdvisory says whether a lint code earns one optional fix loop. These codes never block the
// outcome and are not listed as open checks.
func IssueAdvisory(code string) bool {
	switch code {
	case CodeFirstFrameEmpty, CodeDeadStretch, CodeFadeBetweenWorlds, CodeCueLate, CodeWordsTell,
		CodeLineUnread, CodeTextTooSmall, CodeEntryMonotony, CodeOverBudget, CodeSpotOverlap:
		return true
	}
	return false
}

// exampleJSON renders {"field":"value"} for the fix text of an issue.
func exampleJSON(field, value string) string {
	b, _ := json.Marshal(map[string]string{field: value})
	return string(b)
}

// closestNames returns up to n entries of pool ranked by closeness to input: equal ignoring
// case and dashes, then one containing the other, then by edit distance. Ties keep pool order.
func closestNames(input string, pool []string, n int) []string {
	norm := func(s string) string {
		s = strings.ToLower(s)
		if i := strings.LastIndexByte(s, '.'); i >= 0 {
			s = s[i+1:]
		}
		return strings.Map(func(r rune) rune {
			if r == '-' || r == '_' || r == ' ' {
				return -1
			}
			return r
		}, s)
	}
	in := norm(input)
	type cand struct {
		name  string
		score int
		order int
	}
	cands := make([]cand, 0, len(pool))
	seen := map[string]bool{}
	for i, p := range pool {
		if seen[p] {
			continue
		}
		seen[p] = true
		c := norm(p)
		score := 0
		switch {
		case c == in:
			score = 0
		case in != "" && (strings.Contains(c, in) || strings.Contains(in, c)):
			score = 1
		default:
			score = 2 + levenshtein(in, c)
		}
		cands = append(cands, cand{p, score, i})
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score < cands[j].score })
	if len(cands) > n {
		cands = cands[:n]
	}
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.name
	}
	return out
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// splitTerms splits an asset id or name into lower-case search terms.
func splitTerms(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}
