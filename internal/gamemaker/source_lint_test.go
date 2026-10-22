package gamemaker

import "testing"

func lintCodes(findings []LintFinding) map[string]bool {
	codes := map[string]bool{}
	for _, finding := range findings {
		codes[finding.Code] = true
	}
	return codes
}

func TestLintSourceReportsStructuralDefects(t *testing.T) {
	plan := &GamePlan{
		Stages: []string{"grove", "sanctuary"},
		Assets: []PlanAsset{{Role: "player"}, {Role: "enemy"}, {Role: "treasure"}},
	}
	source := `class Main extends GameScene {
  step(dt) { this.body(0, 0, 10, 10); }
  load() { return fetch("https://cdn.example.test/asset.png"); }
}`
	findings := LintSource(plan, source)
	codes := lintCodes(findings)
	for _, want := range []string{"missing_setup", "missing_player", "missing_levels", "external_resource", "unused_role"} {
		if !codes[want] {
			t.Errorf("lint missed %s: %+v", want, findings)
		}
	}
	for _, finding := range findings {
		if finding.Code == "external_resource" && finding.Line <= 0 {
			t.Errorf("external resource has no line: %+v", finding)
		}
	}
}

func TestLintSourceAcceptsACompleteMain(t *testing.T) {
	plan := &GamePlan{
		Stages: []string{"grove", "sanctuary"},
		Assets: []PlanAsset{{Role: "player"}, {Role: "enemy"}},
	}
	source := `class Main extends GameScene {
  setup() {
    this.player = this.body(48, 48, 16, 16, 0xffffff, false, "player");
    this.configureLevels([{ id: "grove" }, { id: "sanctuary" }]);
    const foe = this.body(120, 48, 16, 16, 0xff0000, false, "enemy");
    return foe;
  }
  step(dt) { super.step(dt); }
}`
	if findings := LintSource(plan, source); len(findings) != 0 {
		t.Fatalf("complete source produced findings: %+v", findings)
	}
}

func TestLintSourceStaysBounded(t *testing.T) {
	plan := &GamePlan{}
	assets := make([]PlanAsset, 0, 20)
	for i := 0; i < 20; i++ {
		assets = append(assets, PlanAsset{Role: "role_missing_" + string(rune('a'+i))})
	}
	plan.Assets = assets
	findings := LintSource(plan, "class Main extends GameScene { setup() { this.player = null; } }")
	if len(findings) > lintMaxFindings {
		t.Fatalf("lint findings unbounded: %d", len(findings))
	}
}

func TestLintSourceIgnoresComments(t *testing.T) {
	plan := &GamePlan{Assets: []PlanAsset{{Role: "player"}}}
	source := "// setup() and this.player are documented here\nclass Main extends GameScene { setup() { this.player = this.body(1,1,1,1,0); } }"
	if findings := LintSource(plan, source); len(findings) != 0 {
		t.Fatalf("comment-only tokens produced findings: %+v", findings)
	}
}
