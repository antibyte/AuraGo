package storymaker

import (
	"context"
	"testing"
)

func TestCompileReasonCodeIsClosed(t *testing.T) {
	for _, tc := range []struct {
		reason, state, want string
	}{
		{"browser_unavailable", "unavailable", "browser_unavailable"},
		{"compile_timeout", "unavailable", "compile_timeout"},
		{"no browser in tests", "unavailable", CompileReasonNone},
		{"The pinned Chromium could not be downloaded: C:\\Users\\x", "unavailable", CompileReasonNone},
		{"", "unavailable", CompileReasonNone},
		{"browser_unavailable", "ok", ""},
		{"browser_unavailable", "failed", ""},
	} {
		if got := compileReasonCode(CompileSummary{Reason: tc.reason}, tc.state); got != tc.want {
			t.Errorf("compileReasonCode(%q, %q) = %q, want %q", tc.reason, tc.state, got, tc.want)
		}
	}
}

func TestCheckReportCarriesTheCompileReason(t *testing.T) {
	studio, _, _ := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		return CompileSummary{Available: false, Reason: "browser_unavailable"}, nil
	})
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Compile != "unavailable" || report.CompileReason != "browser_unavailable" {
		t.Fatalf("compile %q reason %q", report.Compile, report.CompileReason)
	}
	// The rig's scenes are not filled, so the outcome is incomplete; only the detail of the open check is under test.
	_, checks := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 2, ScenesDone: 2}, Check: report})
	found := false
	for _, c := range checks {
		if c.Code == "compile_unverified" && c.Detail == "browser_unavailable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks %+v: compile_unverified must carry its reason", checks)
	}

	none, _, _ := newStudioWriteRig(t, nil)
	report, err = none.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.CompileReason == "" || report.CompileReason == "no browser in tests" {
		t.Fatalf("free text must not become the reason: %q", report.CompileReason)
	}
}
