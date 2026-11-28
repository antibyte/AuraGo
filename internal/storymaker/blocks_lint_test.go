package storymaker

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func flowScene(id string) Scene {
	return Scene{ID: id, Beats: []Beat{{Do: "show", Diagram: &Diagram{Layout: "flow", Nodes: []DiagramNode{{ID: "a", Label: "Eins"}, {ID: "b", Label: "Zwei"}}}, At: SpotAt("full")}}}
}

func plainScene(id string) Scene {
	return Scene{ID: id, Beats: []Beat{{Do: "show", Item: "prop.lock", At: SpotAt("right")}}}
}

func codesOf(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Code+"@"+is.SceneID)
	}
	return out
}

func TestLintBlocks(t *testing.T) {
	// Five scenes and no block: one no_blocks. Four scenes are too few.
	five := &Story{Scenes: []Scene{plainScene("s1"), plainScene("s2"), plainScene("s3"), plainScene("s4"), plainScene("s5")}}
	if got := LintBlocks(five, nil); len(got) != 1 || got[0].Code != CodeNoBlocks || got[0].Path != "/scenes" {
		t.Errorf("5 scenes without a block: %v", codesOf(got))
	}
	if got := LintBlocks(&Story{Scenes: five.Scenes[:4]}, nil); len(got) != 0 {
		t.Errorf("4 scenes without a block: %v", codesOf(got))
	}

	// Three flows in a row repeat at the third; a fourth does not repeat again; a scene without a block ends the run.
	rep := &Story{Scenes: []Scene{flowScene("s1"), flowScene("s2"), flowScene("s3"), flowScene("s4"), plainScene("s5"), flowScene("s6"), flowScene("s7")}}
	got := LintBlocks(rep, nil)
	if len(got) != 1 || got[0].Code != CodeBlockRepeat || got[0].SceneID != "s3" {
		t.Errorf("three flows in a row: %v", codesOf(got))
	}

	// A code scene counts by its first block; a diagram counts by its layout.
	code := Scene{ID: "s3", Kind: "code"}
	mixed := &Story{Scenes: []Scene{flowScene("s1"), flowScene("s2"), code, plainScene("s4"), plainScene("s5")}}
	flowSrc := "export default function scene(api) {\n  api.diagram({layout:'flow', nodes: []}, {at: 'full'});\n}\n"
	if got := LintBlocks(mixed, map[string]string{"s3": flowSrc}); len(got) != 1 || got[0].Code != CodeBlockRepeat || got[0].SceneID != "s3" {
		t.Errorf("a code diagram with layout flow is a flow: %v", codesOf(got))
	}
	cycleSrc := "export default function scene(api) {\n  api . diagram(\n    { layout : \"cycle\", nodes: [] });\n}\n"
	if got := LintBlocks(mixed, map[string]string{"s3": cycleSrc}); len(got) != 0 {
		t.Errorf("a cycle between two flows is no repeat: %v", codesOf(got))
	}
	chartSrc := "export default function scene(api) {\n  api.chart({type:'bar'});\n  api.diagram({layout:'flow'});\n}\n"
	if got := LintBlocks(mixed, map[string]string{"s3": chartSrc}); len(got) != 0 {
		t.Errorf("the first block of a code scene decides: %v", codesOf(got))
	}
	handSrc := "export default function scene(api) {\n  const myapi = { diagram() {} };\n  api.onFrame(() => {});\n}\n"
	if got := LintBlocks(&Story{Scenes: []Scene{code}}, map[string]string{"s3": handSrc}); len(got) != 0 {
		t.Errorf("a code scene without api blocks: %v", codesOf(got))
	}
	if got := LintBlocks(mixed, nil); len(got) != 0 {
		t.Errorf("a code scene whose source is unknown has no block: %v", codesOf(got))
	}

	// Long labels and headlines.
	long := flowScene("s1")
	long.Beats[0].Diagram.Nodes[0].Label = "Das ist viel zu lang"
	long.Beats = append(long.Beats, Beat{Do: "show", Text: "Eins zwei drei vier fünf sechs sieben acht neun", Style: "headline", At: SpotAt("top")},
		Beat{Do: "show", Text: "Eins zwei drei vier fünf sechs sieben *acht*", Style: "headline", At: SpotAt("top")})
	got = LintBlocks(&Story{Scenes: []Scene{long}}, nil)
	if len(got) != 2 || got[0].Code != CodeLabelLong || got[0].Path != "/scenes/0/beats/0/diagram/nodes/0/label" || got[1].Code != CodeLabelLong || got[1].Path != "/scenes/0/beats/1/text" {
		t.Errorf("long labels: %v %+v", codesOf(got), got)
	}

	// Text without spaces is counted like the narration does: a run of n CJK characters is ceil(n/2) words.
	cjk := flowScene("s1")
	cjk.Beats[0].Diagram.Nodes[0].Label = "ファイアウォール機能"
	cjk.Beats = append(cjk.Beats, Beat{Do: "show", Text: "ファイアウォールはネットワークの入口で不正な通信を止めて安全を守りますよ", Style: "headline", At: SpotAt("top")},
		Beat{Do: "show", Text: "不正な通信を止める", Style: "headline", At: SpotAt("top")})
	got = LintBlocks(&Story{Scenes: []Scene{cjk}}, nil)
	if len(got) != 2 || got[0].Code != CodeLabelLong || got[0].Path != "/scenes/0/beats/0/diagram/nodes/0/label" || got[1].Code != CodeLabelLong || got[1].Path != "/scenes/0/beats/1/text" {
		t.Errorf("long CJK labels: %v %+v", codesOf(got), got)
	}

	// All of them are warnings and advice.
	all := append(LintBlocks(five, nil), LintBlocks(rep, nil)...)
	all = append(all, LintBlocks(&Story{Scenes: []Scene{long}}, nil)...)
	for _, is := range all {
		if is.Severity != SeverityWarning || !IssueAdvisory(is.Code) || strings.TrimSpace(is.Fix) == "" {
			t.Errorf("%s must be an advisory warning with a fix: %+v", is.Code, is)
		}
	}
}

func TestCheckAdvisesAboutBlocksForACreateJobOnly(t *testing.T) {
	ctx := context.Background()
	studio, ps, projectID := newStudioWriteRig(t, nil)
	saveFixture(t, ps, projectID, "agent-firewall-run")
	has := func(report CheckReport) bool {
		return slices.ContainsFunc(report.Advice, func(is Issue) bool { return is.Code == CodeNoBlocks })
	}
	report, err := studio.Check(ctx)
	if err != nil || !has(report) {
		t.Fatalf("a create job without blocks gets no_blocks advice: %v %+v", err, report.Advice)
	}
	studio.job.Kind = AgentKindRevise
	if report, err = studio.Check(ctx); err != nil || has(report) {
		t.Fatalf("a revise job gets no block advice: %v %+v", err, report.Advice)
	}
}
