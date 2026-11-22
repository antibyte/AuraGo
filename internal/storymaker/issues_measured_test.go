package storymaker

import "testing"

// The hard measured codes are model-fixable warnings that block an outcome; the advice codes never do. None of them is an
// error, so a person's export is never refused for them.
func TestMeasuredCodesClassification(t *testing.T) {
	for _, code := range []string{CodeTextOffframe, CodeFlash} {
		is := Issue{Severity: SeverityWarning, Code: code, SceneID: "s1"}
		if !IssueModelFixable(is) {
			t.Errorf("%s must be model-fixable", code)
		}
		if issueNeverBlocks(code) || !HasBlockingIssue([]Issue{is}) {
			t.Errorf("%s must block while left", code)
		}
		if IssueAdvisory(code) {
			t.Errorf("%s is not advice", code)
		}
		if HasErrors([]Issue{is}) {
			t.Errorf("%s must not be an error", code)
		}
		outcome, checks := DecideOutcome(OutcomeInput{
			Progress: StudioProgress{ScenesTotal: 2, ScenesDone: 2},
			Check:    CheckReport{Compile: "ok", StoryboardScenes: 2, LayoutScenes: 2, Issues: []Issue{is}},
		})
		if outcome != OutcomeIncomplete {
			t.Errorf("%s left: outcome = %q, want incomplete", code, outcome)
		}
		if len(checks) != 1 || checks[0].Code != code || checks[0].SceneID != "s1" {
			t.Errorf("%s left: checks = %+v", code, checks)
		}
	}
	for _, code := range []string{CodeTextContrast, CodeTextUnsafe, CodeSlowHook} {
		is := Issue{Severity: SeverityWarning, Code: code, SceneID: "s1"}
		if !IssueAdvisory(code) {
			t.Errorf("%s must be advice", code)
		}
		if HasErrors([]Issue{is}) {
			t.Errorf("%s must not be an error", code)
		}
	}
	for _, code := range []string{CodeTextOffframe, CodeTextUnsafe, CodeFlash} {
		if IssueLayout(code) {
			t.Errorf("%s is not a layout code", code)
		}
	}
}
