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
		{"browser_crashed", "unavailable", "browser_crashed"},
		{"compile_setup", "unavailable", "compile_setup"},
		{"stale_story", "unavailable", CompileReasonNone}, // code-like words of other origins are not reasons either
		{"invalid_story", "unavailable", CompileReasonNone},
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

// A compile that was unavailable is kept for the story version, so a second check does not ask again. When the browser
// becomes ready the server calls ForgetCompile, and the next check compiles for the same version.
func TestStudioCheckForgetCompileLetsAReadyBrowserBeSeenForTheSameStory(t *testing.T) {
	calls := 0
	studio, _, _ := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		calls++
		if calls == 1 {
			return CompileSummary{Available: false, Reason: "browser_unavailable"}, nil
		}
		return CompileSummary{Available: true}, nil
	})
	check := func() CheckReport {
		t.Helper()
		report, err := studio.Check(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	if report := check(); report.Compile != "unavailable" {
		t.Fatalf("first check: compile %q, want unavailable", report.Compile)
	}
	if report := check(); report.Compile != "unavailable" || calls != 1 {
		t.Fatalf("second check of the same story: compile %q after %d compiles, want the cached unavailable after 1", report.Compile, calls)
	}
	studio.ForgetCompile()
	if report := check(); report.Compile != "ok" || report.CompileReason != "" || calls != 2 {
		t.Fatalf("after ForgetCompile: compile %q reason %q after %d compiles, want ok after 2", report.Compile, report.CompileReason, calls)
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
