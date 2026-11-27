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

// A scene whose camera the player does not make caption-safe is skipped by the caption rule; the other scenes keep it. Both
// fix texts name the wide framing as the alternative.
func TestLayoutIssuesCaptionRuleSkipsExemptScenes(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	exempt, other := layoutScene("s1", 0), layoutScene("s2", 1)
	for _, sc := range []*LayoutScene{&exempt, &other} {
		sc.Caption = &caption
		sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.25, 0.40, 0.12, 0.48}}}
		sc.Props = []LayoutProp{{ID: "lock", Item: "prop.lock", Box: LayoutBox{0.45, 0.70, 0.12, 0.18}}}
	}
	got := LayoutIssues([]LayoutScene{exempt, other}, LayoutOptions{CaptionRule: true, CaptionExempt: map[string]bool{"s1": true}})
	if !slices.Equal(layoutCodes(got), []string{CodeCaptionOverlap, CodeCaptionOverlap}) {
		t.Fatalf("issues = %v", layoutCodes(got))
	}
	for _, is := range got {
		if is.SceneID != "s2" {
			t.Errorf("the exempt scene s1 must have no caption finding: %+v", is)
		}
		if !strings.Contains(is.Fix, `Or give the scene the camera framing "wide".`) {
			t.Errorf("the fix names the wide framing: %q", is.Fix)
		}
	}
	if got := LayoutIssues([]LayoutScene{exempt}, LayoutOptions{CaptionRule: true, CaptionExempt: map[string]bool{"s1": true}}); len(got) != 0 {
		t.Fatalf("an exempt scene has no caption finding: %v", layoutCodes(got))
	}
	if got := LayoutIssues([]LayoutScene{exempt}, LayoutOptions{CaptionRule: true, CaptionExempt: map[string]bool{"s9": true}}); len(got) != 2 {
		t.Fatalf("another scene's exemption does not count: %v", layoutCodes(got))
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
	want := []string{CodeOffscreen, CodeTextOffframe, CodeActorPropOverlap} // the text has its own code, the prop keeps offscreen
	slices.Sort(want)
	have := layoutCodes(got)
	slices.Sort(have)
	if !slices.Equal(have, want) {
		t.Fatalf("issues = %v, want %v", have, want)
	}
}

// A text that sticks out of the frame by more than 1 percent is text_offframe, with the text and the scene named; a text that
// is inside, or out by a hair, is not.
func TestLayoutIssuesTextOffframe(t *testing.T) {
	sc := layoutScene("s3", 2)
	sc.Texts = []LayoutText{
		{ID: "title", Box: LayoutBox{0.7, 0.2, 0.35, 0.1}},  // a seventh outside to the right
		{ID: "fine", Box: LayoutBox{0.3, 0.2, 0.3, 0.1}},    // inside
		{ID: "hair", Box: LayoutBox{0.7, 0.2, 0.3005, 0.1}}, // 0.17 percent outside
	}
	var got []Issue
	for _, is := range LayoutIssues([]LayoutScene{sc}, LayoutOptions{}) {
		if is.Code != CodeTextUnsafe { // a text at the frame edge may also lie in the margin; that is another rule
			got = append(got, is)
		}
	}
	if len(got) != 1 {
		t.Fatalf("issues = %v, want one text_offframe", layoutCodes(got))
	}
	is := got[0]
	if is.Code != CodeTextOffframe || is.Severity != SeverityWarning || is.SceneID != "s3" || is.Path != "/scenes/2" {
		t.Errorf("issue = %+v", is)
	}
	if !strings.Contains(is.Message, "title") || !strings.Contains(is.Message, "s3") || !strings.Contains(is.Fix, `"center"`) {
		t.Errorf("message %q, fix %q", is.Message, is.Fix)
	}
	if IssueLayout(is.Code) || !IssueModelFixable(is) {
		t.Errorf("text_offframe is model-fixable and not a layout code")
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

func TestLayoutIssuesSkipStaticForCodeScenes(t *testing.T) {
	code := LayoutScene{ID: "c", Index: 1, HoldS: 2, Code: true, Static: true, Actors: []LayoutActor{}, Props: []LayoutProp{}, Texts: []LayoutText{}}
	if got := LayoutIssues([]LayoutScene{code}, LayoutOptions{}); len(got) != 0 {
		t.Fatalf("a code scene reported %v", layoutCodes(got))
	}
	code.Code = false
	if got := layoutCodes(LayoutIssues([]LayoutScene{code}, LayoutOptions{})); !slices.Equal(got, []string{CodeSceneStatic}) {
		t.Fatalf("a storyboard scene that is static reported %v", got)
	}
}

func TestLayoutIssuesCodeSceneGetsNoCaptionFindings(t *testing.T) {
	caption := LayoutBox{0.2, 0.8, 0.6, 0.1} // inside the bottom margin of 9:16
	sc := layoutScene("c", 0)
	sc.Caption, sc.Code = &caption, true
	opts := LayoutOptions{CaptionRule: true, Aspect: "9:16"}
	if got := LayoutIssues([]LayoutScene{sc}, opts); len(got) != 0 {
		t.Fatalf("a code scene reported %v", layoutCodes(got))
	}
	sc.Code = false
	if got := layoutCodes(LayoutIssues([]LayoutScene{sc}, opts)); !slices.Equal(got, []string{CodeTextUnsafe}) {
		t.Fatalf("a storyboard scene reported %v", got)
	}
}

func TestLayoutIssuesForBlocks(t *testing.T) {
	caption := LayoutBox{0.2, 0.794, 0.6, 0.126}
	sc := layoutScene("s1", 0)
	sc.Caption = &caption
	sc.Actors = []LayoutActor{{ID: "anna", Box: LayoutBox{0.5, 0.2, 0.1, 0.4}}}
	sc.Blocks = []LayoutBlock{
		{ID: "covered", Kind: "chart", Layout: "bar", Label: 0.03, Box: LayoutBox{0.3, 0.5, 0.3, 0.4}},
		{ID: "outside", Kind: "list", Label: 0.03, Box: LayoutBox{0.8, 0.1, 0.4, 0.3}},
		{ID: "tiny", Kind: "headline", Label: 0.03, Box: LayoutBox{0.1, 0.1, 0.3, 0.05}},
		{ID: "hidden", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.45, 0.2, 0.2, 0.3}},
		{ID: "label", Kind: "diagram", Layout: "flow", Label: 0.02, Box: LayoutBox{0.05, 0.55, 0.3, 0.2}},
		{ID: "fine", Kind: "diagram", Layout: "flow", Label: 0.03, Box: LayoutBox{0.05, 0.05, 0.3, 0.4}},
	}
	got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{})
	want := []string{CodeCaptionOverlap, CodeOffscreen, CodeBlockTooSmall, CodeActorPropOverlap, CodeBlockTooSmall}
	if !slices.Equal(layoutCodes(got), want) {
		t.Fatalf("issues = %v, want %v", layoutCodes(got), want)
	}
	for _, is := range got {
		if is.Severity != SeverityWarning || is.Fix == "" || !IssueLayout(is.Code) || !IssueModelFixable(is) || strings.Contains(is.Fix, "api.safe()") {
			t.Errorf("a storyboard block finding is a warning with a patch_scene fix: %+v", is)
		}
	}
	if !strings.Contains(got[0].Message, "covered") || !strings.Contains(got[2].Fix, `"full"`) {
		t.Errorf("messages: %+v", got)
	}
	// The caption rule holds without composition v2, and a code scene names api.safe().
	sc.Code = true
	for _, is := range LayoutIssues([]LayoutScene{sc}, LayoutOptions{}) {
		if !strings.Contains(is.Fix, "api.safe()") || !strings.Contains(is.Fix, "put_code") {
			t.Errorf("a code scene fix names api.safe(): %+v", is)
		}
	}
	sc.Blocks = []LayoutBlock{{ID: "ok", Kind: "chart", Label: 0.03, Box: LayoutBox{0.1, 0.1, 0.5, 0.5}}}
	sc.Actors = nil
	if got := LayoutIssues([]LayoutScene{sc}, LayoutOptions{}); len(got) != 0 {
		t.Fatalf("a block inside the safe area with label 0.03: %v", layoutCodes(got))
	}
}
