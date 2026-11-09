package storymaker

import (
	"context"
	"slices"
	"testing"
)

// A compile that measured the layout of the real run: the check turns the boxes into findings, they are model-fixable and
// the summary counts decide the outcome.
func TestCheckTurnsTheLayoutIntoFindings(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		sc := layoutScene("s1", 0)
		sc.Caption = &caption
		sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.25, 0.40, 0.12, 0.48}}}
		return CompileSummary{Available: true, DurationMS: 8000, StoryboardScenes: 2, Layout: []LayoutScene{sc, layoutScene("s2", 1)}}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	report, err := studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, is := range report.Issues {
		codes = append(codes, is.Code)
	}
	if report.Compile != "ok" || report.StoryboardScenes != 2 || report.LayoutScenes != 2 {
		t.Fatalf("report counts: %+v", report)
	}
	// agent-varied is a v2-less fixture (no compose), so the caption rule is off; turn it on for the second half.
	if slices.Contains(codes, CodeCaptionOverlap) {
		t.Fatalf("a story without compose is not held to the caption rule: %v", codes)
	}
	var s Story
	raw, _, _ := ps.LoadStory(context.Background(), projectID)
	s = mustDecode(t, string(raw))
	s.Style.Compose = "v2"
	out, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.SaveStory(context.Background(), projectID, out, ""); err != nil {
		t.Fatal(err)
	}
	report, err = studio.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	codes = nil
	for _, is := range report.Issues {
		codes = append(codes, is.Code)
	}
	if !slices.Contains(codes, CodeCaptionOverlap) {
		t.Fatalf("a v2 story with the feet in the caption band must be flagged: %v", codes)
	}
	outcome, _ := DecideOutcome(OutcomeInput{Progress: StudioProgress{ScenesTotal: 7, ScenesDone: 7}, Check: report})
	if outcome == OutcomeIncomplete {
		t.Fatalf("a layout finding must not block: %s", outcome)
	}
}

// The player does not lift the feet in 9:16, so a caption overlap measured there could never be cleared: the check does not
// raise it. The same story and measurement in 16:9 does.
func TestCheckHoldsNo916FilmToTheCaptionRule(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	studio, ps, projectID := newStudioWriteRig(t, func(context.Context, string) (CompileSummary, error) {
		sc := layoutScene("s1", 0)
		sc.Caption = &caption
		sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.25, 0.40, 0.12, 0.48}}}
		return CompileSummary{Available: true, DurationMS: 8000, StoryboardScenes: 2, Layout: []LayoutScene{sc, layoutScene("s2", 1)}}, nil
	})
	saveFixture(t, ps, projectID, "agent-varied")
	codesFor := func(aspect string) []string {
		raw, _, _ := ps.LoadStory(context.Background(), projectID)
		s := mustDecode(t, string(raw))
		s.Style.Compose = "v2"
		s.Style.Captions = true
		s.Format.Aspect = aspect
		out, err := CanonicalJSON(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ps.SaveStory(context.Background(), projectID, out, ""); err != nil {
			t.Fatal(err)
		}
		report, err := studio.Check(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var codes []string
		for _, is := range report.Issues {
			codes = append(codes, is.Code)
		}
		return codes
	}
	if codes := codesFor("9:16"); slices.Contains(codes, CodeCaptionOverlap) {
		t.Fatalf("a 9:16 film is not held to the caption rule: %v", codes)
	}
	if codes := codesFor("16:9"); !slices.Contains(codes, CodeCaptionOverlap) {
		t.Fatalf("the same 16:9 film must be flagged: %v", codes)
	}
}
