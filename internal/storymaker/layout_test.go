package storymaker

import (
	"slices"
	"strings"
	"testing"
)

func layoutScene(id string, index int) LayoutScene {
	return LayoutScene{ID: id, Index: index, HoldS: 1, Actors: []LayoutActor{}, Props: []LayoutProp{}, Texts: []LayoutText{}}
}

func layoutCodes(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Code)
	}
	return out
}

func TestLayoutIssuesCaptionOverlap(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	sc := layoutScene("s1", 0)
	sc.Caption = &caption
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.25, 0.40, 0.12, 0.48}}} // feet at 0.88: inside the band
	sc.Props = []LayoutProp{{ID: "lock", Item: "prop.lock", Box: LayoutBox{0.45, 0.70, 0.12, 0.18}}}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: true})
	if !slices.Equal(layoutCodes(got), []string{CodeCaptionOverlap, CodeCaptionOverlap}) {
		t.Fatalf("issues = %v", layoutCodes(got))
	}
	for _, is := range got {
		if is.Severity != SeverityWarning || is.SceneID != "s1" || is.Path != "/scenes/0" || is.Fix == "" || !IssueLayout(is.Code) || !IssueModelFixable(is) {
			t.Errorf("a layout finding is a warning on the scene with a fix: %+v", is)
		}
	}

	// Feet above the band, or no caption on screen, or a v1 film (rule off): nothing.
	sc.Actors[0].Box = LayoutBox{0.25, 0.28, 0.12, 0.48} // feet at 0.76
	sc.Props[0].Box = LayoutBox{0.45, 0.58, 0.12, 0.18}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: true}); len(got) != 0 {
		t.Fatalf("feet above the band: %v", layoutCodes(got))
	}
	sc.Actors[0].Box = LayoutBox{0.25, 0.40, 0.12, 0.48}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: false}); slices.Contains(layoutCodes(got), CodeCaptionOverlap) {
		t.Fatalf("a v1 film is not held to the caption rule: %v", layoutCodes(got))
	}
	sc.Caption = nil
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{CaptionRule: true}); len(got) != 0 {
		t.Fatalf("no caption on screen: %v", layoutCodes(got))
	}
}

func TestLayoutIssuesHeroPropTooSmall(t *testing.T) {
	sc := layoutScene("s2", 1)
	sc.Props = []LayoutProp{
		{ID: "small", Item: "prop.key", Hero: true, Scale: 1, Box: LayoutBox{0.5, 0.6, 0.05, 0.08}},
		{ID: "other", Item: "prop.lock", Hero: false, Scale: 1, Box: LayoutBox{0.2, 0.6, 0.05, 0.06}},
	}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	if !slices.Equal(layoutCodes(got), []string{CodePropTooSmall}) {
		t.Fatalf("only the hero counts: %v", layoutCodes(got))
	}
	if got[0].SceneID != "s2" || got[0].Fix == "" {
		t.Fatalf("issue = %+v", got[0])
	}
	sc.Props[0].Box = LayoutBox{0.5, 0.5, 0.1, 0.13}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); len(got) != 0 {
		t.Fatalf("13 percent of the height is big enough: %v", layoutCodes(got))
	}
}

// A scale of 2 is the most a beat takes. When even that does not reach the target height, the fix must not ask for a scale
// that can never clear the finding.
func TestLayoutIssuesHeroPropBeyondTheScaleRange(t *testing.T) {
	prop := func(height float64) Issue {
		sc := layoutScene("s2", 1)
		sc.Props = []LayoutProp{{ID: "key", Item: "prop.key", Hero: true, Scale: 1, Box: LayoutBox{0.5, 0.6, 0.05, height}}}
		got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
		if !slices.Equal(layoutCodes(got), []string{CodePropTooSmall}) {
			t.Fatalf("height %v: issues = %v", height, layoutCodes(got))
		}
		return got[0]
	}
	// 0.05 at scale 1 needs 4.0: scale 2 is not enough, the fix names another spot or a bigger item.
	far := prop(0.05)
	if !strings.Contains(far.Fix, "Even scale 2 is not enough") || !strings.Contains(far.Fix, "nearer spot") || !strings.Contains(far.Fix, "storymaker_catalog") || strings.Contains(far.Fix, `"scale":`) {
		t.Errorf("a prop that scale 2 cannot fix: %q", far.Fix)
	}
	// 0.1 at scale 1 needs exactly 2.0: the old text with the scale applies.
	edge := prop(0.1)
	if !strings.Contains(edge.Fix, `"scale":2.0`) || strings.Contains(edge.Fix, "Even scale 2") {
		t.Errorf("a prop that scale 2 reaches: %q", edge.Fix)
	}
}

func TestLayoutIssuesOffscreenAndOverlap(t *testing.T) {
	sc := layoutScene("s3", 2)
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.30, 0.30, 0.12, 0.45}}}
	sc.Props = []LayoutProp{
		{ID: "gone", Item: "prop.lock", Box: LayoutBox{0.9, 0.5, 0.2, 0.2}},               // half outside to the right
		{ID: "cover", Item: "prop.shield", Box: LayoutBox{0.32, 0.4, 0.10, 0.10}},         // inside the actor box
		{ID: "behind", Item: "prop.key", Back: true, Box: LayoutBox{0.32, 0.4, 0.1, 0.1}}, // a back prop may overlap
	}
	sc.Texts = []LayoutText{{ID: "t", Box: LayoutBox{0.7, 0.2, 0.35, 0.1}}} // sticks out of the frame
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	want := []string{CodeOffscreen, CodeOffscreen, CodeActorPropOverlap}
	slices.Sort(want)
	have := layoutCodes(got)
	slices.Sort(have)
	if !slices.Equal(have, want) {
		t.Fatalf("issues = %v, want %v", have, want)
	}
}

func TestLayoutIssuesStaticSceneAndMissingArt(t *testing.T) {
	sc := layoutScene("s4", 3)
	sc.Static = true
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.3, 0.3, 0.1, 0.4}}}
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); !slices.Equal(layoutCodes(got), []string{CodeSceneStatic}) {
		t.Fatalf("issues = %v", layoutCodes(got))
	}
	sc.Missing = 1
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); slices.Contains(layoutCodes(got), CodeSceneStatic) == false {
		t.Fatalf("a missing box does not hide a static scene")
	}
	if LayoutIssues(nil, LayoutOptions{CaptionRule: true}) != nil {
		t.Fatal("no layout, no findings")
	}
}
