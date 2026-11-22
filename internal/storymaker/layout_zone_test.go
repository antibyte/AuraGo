package storymaker

import (
	"slices"
	"strings"
	"testing"
)

// The 9:16 zone is the union of the platform overlays; every other aspect keeps the stage's 5 percent margin.
func TestSafeZone(t *testing.T) {
	if got, want := SafeZone("9:16"), (LayoutZone{Top: 0.115, Right: 0.130, Bottom: 0.219, Left: 0.056}); got != want {
		t.Errorf("9:16 zone = %+v, want %+v", got, want)
	}
	margin := LayoutZone{Top: 0.05, Right: 0.05, Bottom: 0.05, Left: 0.05}
	for _, aspect := range []string{"16:9", "1:1", "", "4:5"} {
		if got := SafeZone(aspect); got != margin {
			t.Errorf("%q zone = %+v, want %+v", aspect, got, margin)
		}
	}
}

// A text or the caption with more than 2 percent of its box inside the zone is text_unsafe, in the zone of the story's aspect.
func TestLayoutIssuesTextUnsafe(t *testing.T) {
	scene := func() LayoutScene {
		sc := layoutScene("s2", 1)
		sc.Texts = []LayoutText{
			{ID: "top", Box: LayoutBox{0.3, 0.02, 0.4, 0.1}},    // 16:9: three tenths of the box sit above the 5 percent margin
			{ID: "low", Box: LayoutBox{0.3, 0.70, 0.4, 0.1}},    // 9:16: one fifth of the box sits in the bottom zone (0.781)
			{ID: "mid", Box: LayoutBox{0.3, 0.40, 0.4, 0.1}},    // safe everywhere
			{ID: "edge", Box: LayoutBox{0.3, 0.0495, 0.4, 0.1}}, // 16:9: only half a percent of the box is in the margin
		}
		return sc
	}
	only := func(aspect string) []Issue {
		return LayoutIssues([]LayoutScene{scene()}, LayoutOptions{Aspect: aspect})
	}
	ids := func(issues []Issue) []string {
		var out []string
		for _, is := range issues {
			if is.Code != CodeTextUnsafe {
				t.Errorf("unexpected %s", is.Code)
			}
			for _, id := range []string{"top", "low", "mid", "edge"} {
				if strings.Contains(is.Message, "text "+id+" ") {
					out = append(out, id)
				}
			}
		}
		return out
	}
	if got := ids(only("16:9")); !slices.Equal(got, []string{"top"}) {
		t.Errorf("16:9 unsafe texts = %v, want [top]", got)
	}
	// 9:16: the top zone is 0.115, so "top" and "edge" are unsafe there, and "low" joins them.
	if got := ids(only("9:16")); !slices.Equal(got, []string{"top", "low", "edge"}) {
		t.Errorf("9:16 unsafe texts = %v, want [top low edge]", got)
	}
	is := only("16:9")[0]
	if is.Severity != SeverityWarning || is.SceneID != "s2" || is.Path != "/scenes/1" || !strings.Contains(is.Message, "s2") || is.Fix == "" {
		t.Errorf("issue = %+v", is)
	}
	if IssueLayout(is.Code) || IssueModelFixable(is) || !IssueAdvisory(is.Code) {
		t.Errorf("text_unsafe is advice, not a layout code")
	}
}

// A text that already sticks out of the frame gets text_offframe only; the caption is checked in every aspect.
func TestLayoutIssuesTextUnsafeSkipsOffframeAndChecksTheCaption(t *testing.T) {
	sc := layoutScene("s1", 0)
	sc.Texts = []LayoutText{{ID: "wide", Box: LayoutBox{0.7, 0.4, 0.4, 0.1}}} // out of the frame and in the right zone
	if got := layoutCodes(LayoutIssues([]LayoutScene{sc}, LayoutOptions{})); !slices.Equal(got, []string{CodeTextOffframe}) {
		t.Fatalf("issues = %v, want only text_offframe", got)
	}
	sc.Texts = nil
	caption := LayoutBox{0.2, 0.70, 0.6, 0.126} // bottom at 0.826
	sc.Caption = &caption
	if got := layoutCodes(LayoutIssues([]LayoutScene{sc}, LayoutOptions{Aspect: "9:16"})); !slices.Equal(got, []string{CodeTextUnsafe}) {
		t.Errorf("9:16 caption issues = %v, want [text_unsafe]", got)
	}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{Aspect: "16:9"})
	if len(got) != 0 {
		t.Errorf("16:9 caption at 0.826 is inside the 5 percent margin, got %v", layoutCodes(got))
	}
	low := LayoutBox{0.2, 0.84, 0.6, 0.126} // bottom at 0.966
	sc.Caption = &low
	got = LayoutIssues([]LayoutScene{sc}, LayoutOptions{Aspect: "16:9"})
	if len(got) != 1 || got[0].Code != CodeTextUnsafe || !strings.Contains(got[0].Message, "caption") || !strings.Contains(got[0].Message, "s1") {
		t.Errorf("16:9 caption in the margin: %+v", got)
	}
}

// The check hands the story's aspect to the layout rules.
func TestStoryLayoutOptionsCarryTheAspect(t *testing.T) {
	var s Story
	s.Format.Aspect = "9:16"
	if got := StoryLayoutOptions(&s).Aspect; got != "9:16" {
		t.Errorf("Aspect = %q, want 9:16", got)
	}
}
