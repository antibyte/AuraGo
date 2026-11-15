package ui

import (
	"strings"
	"testing"
)

// easyDragBudgetProblems lists what is wrong with a budget table. A well-formed table has:
//   - a region name that is unique and a ceiling above zero;
//   - a target that is not negative and not above its ceiling (a target of 0 means "none");
//   - a gate on every row with a target, and no gate on a row without one;
//   - for an enforced row, a target and a ceiling equal to it.
func easyDragBudgetProblems(rows []easyDragControlBudget) []string {
	var problems []string
	seen := map[string]bool{}
	for _, b := range rows {
		switch {
		case strings.TrimSpace(b.Region) == "":
			problems = append(problems, "a row has no region name")
			continue
		case seen[b.Region]:
			problems = append(problems, b.Region+": the region appears twice")
		}
		seen[b.Region] = true
		if b.Ceiling <= 0 {
			problems = append(problems, b.Region+": the ceiling must be above 0")
		}
		if b.Target < 0 {
			problems = append(problems, b.Region+": the target must not be negative")
		}
		if b.Target > b.Ceiling {
			problems = append(problems, b.Region+": the target is above the ceiling")
		}
		if b.Target > 0 && strings.TrimSpace(b.Gate) == "" {
			problems = append(problems, b.Region+": a row with a target must name the sub-project that enforces it")
		}
		if b.Target == 0 && b.Gate != "" {
			problems = append(problems, b.Region+": a row without a target must not name a gate")
		}
		if b.Enforced && (b.Target == 0 || b.Ceiling != b.Target) {
			problems = append(problems, b.Region+": an enforced row needs a target and a ceiling equal to it")
		}
	}
	return problems
}

// TestEasyDragControlBudgetTable is the T0 check of the control budget (W1b spec section 5.4): the
// table that TestEasyDragControlBudgetsBrowser reads is well formed, and it still names every region
// the browser test measures.
func TestEasyDragControlBudgetTable(t *testing.T) {
	for _, problem := range easyDragBudgetProblems(easyDragControlBudgets) {
		t.Error(problem)
	}
	have := map[string]bool{}
	for _, b := range easyDragControlBudgets {
		have[b.Region] = true
	}
	for _, region := range []string{"header", "footer", "more-menu", "home", "card"} {
		if !have[region] {
			t.Errorf("the table has no row for %q", region)
		}
	}

	broken := []easyDragControlBudget{
		{Region: "a", Ceiling: 5, Target: 6, Gate: "W1c"},
		{Region: "b", Ceiling: 5, Target: 3},
		{Region: "c", Ceiling: 5, Gate: "W1f"},
		{Region: "d", Ceiling: 5, Target: 3, Gate: "W1c", Enforced: true},
		{Region: "d", Ceiling: 0},
	}
	if got := len(easyDragBudgetProblems(broken)); got != 6 {
		t.Errorf("the check found %d problems in a table with 6, so it no longer checks", got)
	}
}
