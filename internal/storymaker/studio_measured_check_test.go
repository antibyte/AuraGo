package storymaker

import (
	"context"
	"slices"
	"testing"
)

// measuredRig runs the check on a story whose compile stub reports the given compile issues and layout scenes, and decides the
// outcome of the report the way the job does.
func measuredRig(t *testing.T, issues []Issue, layout []LayoutScene) (CheckReport, string, []AgentCheck) {
	t.Helper()
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{Available: true, DurationMS: 8000, StoryboardScenes: 2, Layout: layout, Issues: issues}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	outcome, checks := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	return report, outcome, checks
}

func reportCodes(list []Issue) []string {
	var out []string
	for _, is := range list {
		out = append(out, is.Code)
	}
	return out
}

func measuredIssue(code string) Issue {
	return Issue{Severity: SeverityWarning, SceneID: "s1", Path: "/scenes/0", Code: code, Message: code}
}

func measuredLayout() []LayoutScene { return []LayoutScene{layoutScene("s1", 0), layoutScene("s2", 1)} }

// The probe did not reach every frame: the finding is an open check, listed but never blocking.
func TestCheckListsFlashUnverifiedAsAnOpenCheck(t *testing.T) {
	report, outcome, checks := measuredRig(t, []Issue{measuredIssue(CodeFlashUnverified)}, measuredLayout())
	if !slices.Contains(reportCodes(report.Environment), CodeFlashUnverified) {
		t.Fatalf("environment = %v, want flash_unverified", reportCodes(report.Environment))
	}
	for _, list := range [][]Issue{report.Issues, report.Advice} {
		if slices.Contains(reportCodes(list), CodeFlashUnverified) {
			t.Fatalf("flash_unverified is only an open check: issues %v, advice %v", reportCodes(report.Issues), reportCodes(report.Advice))
		}
	}
	if !slices.Contains(checks, AgentCheck{Code: CodeFlashUnverified, SceneID: "s1"}) {
		t.Errorf("checks = %+v, want flash_unverified listed", checks)
	}
	if outcome != OutcomeReadyUnverified {
		t.Errorf("outcome = %q, want ready_unverified", outcome)
	}
	if HasErrors(report.Environment) {
		t.Error("flash_unverified must not be an error")
	}
}

// Contrast, safe-zone and slow-hook findings are advice: they never block and never count as open checks.
func TestCheckKeepsTheAdviceCodesAsAdvice(t *testing.T) {
	inZone := layoutScene("s2", 1)
	inZone.Texts = []LayoutText{{ID: "title", Box: LayoutBox{0.3, 0.01, 0.4, 0.1}}} // in the top margin, inside the frame
	issues := []Issue{measuredIssue(CodeTextContrast), measuredIssue(CodeSlowHook)}
	report, outcome, checks := measuredRig(t, issues, []LayoutScene{layoutScene("s1", 0), inZone})
	for _, code := range []string{CodeTextContrast, CodeSlowHook, CodeTextUnsafe} {
		if !slices.Contains(reportCodes(report.Advice), code) {
			t.Errorf("advice = %v, want %s", reportCodes(report.Advice), code)
		}
		if slices.Contains(reportCodes(report.Issues), code) || slices.Contains(reportCodes(report.Environment), code) {
			t.Errorf("%s must be advice only", code)
		}
		for _, c := range checks {
			if c.Code == code {
				t.Errorf("%s must not be listed as an open check", code)
			}
		}
	}
	if outcome == OutcomeIncomplete {
		t.Errorf("advice must not block: %s", outcome)
	}
	if HasErrors(report.Advice) {
		t.Error("advice must not be an error")
	}
}

// A text outside the frame and a flash are checks to fix: they go to the model's issues and, left over, make the job
// incomplete. Neither is an error, so a person's export is never refused.
func TestCheckBlocksOnTextOffframeAndFlash(t *testing.T) {
	out := layoutScene("s2", 1)
	out.Texts = []LayoutText{{ID: "title", Box: LayoutBox{0.7, 0.4, 0.4, 0.1}}}
	for _, tc := range []struct {
		name   string
		code   string
		issues []Issue
		layout []LayoutScene
	}{
		{"text_offframe", CodeTextOffframe, nil, []LayoutScene{layoutScene("s1", 0), out}},
		{"flash", CodeFlash, []Issue{measuredIssue(CodeFlash)}, measuredLayout()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report, outcome, checks := measuredRig(t, tc.issues, tc.layout)
			if !slices.Contains(reportCodes(report.Issues), tc.code) {
				t.Fatalf("issues = %v, want %s", reportCodes(report.Issues), tc.code)
			}
			if outcome != OutcomeIncomplete {
				t.Errorf("outcome = %q, want incomplete", outcome)
			}
			if !slices.ContainsFunc(checks, func(c AgentCheck) bool { return c.Code == tc.code }) {
				t.Errorf("checks = %+v, want %s listed", checks, tc.code)
			}
			if HasErrors(report.Issues) {
				t.Errorf("%s must not be an error: %+v", tc.code, report.Issues)
			}
		})
	}
}
