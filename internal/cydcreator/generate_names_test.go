package cydcreator

import "testing"

// TestValidateGeneratedSymbolCollisions covers the names plan 1c reserves so
// generated C symbols cannot collide: ui_screen_t / ui_transition_t are
// types, ui_img_<asset id> are image descriptors, and asset ids must be C
// identifier tails.
func TestValidateGeneratedSymbolCollisions(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(p *Project)
		code   string
		path   string
	}{
		{"type name screen_t", func(p *Project) { w := lbl("w_1"); w.Name = "screen_t"; addTo(p, 0, w) },
			IssueNameInvalid, "screens/0/widgets/0/name"},
		{"type name transition_t", func(p *Project) { w := lbl("w_1"); w.Name = "transition_t"; addTo(p, 0, w) },
			IssueNameInvalid, "screens/0/widgets/0/name"},
		{"image symbol of an asset", func(p *Project) { w := lbl("w_1"); w.Name = "img_a_sun"; addTo(p, 0, w) },
			IssueNameInvalid, "screens/0/widgets/0/name"},
		{"asset id with a dash", func(p *Project) { p.Assets[0].ID = "a-sun" }, IssuePropInvalid, "assets/0/id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validFixture(t)
			tc.mutate(&p)
			issues := Validate(p, ValidateOptions{})
			if !issueAt(issues, tc.code, tc.path) {
				t.Fatalf("want %s at %s, got:\n%s", tc.code, tc.path, fmtIssues(issues))
			}
		})
	}
	// Names that merely start with img_ stay valid when no asset matches.
	p := validFixture(t)
	w := lbl("w_1")
	w.Name = "img_1"
	addTo(&p, 0, w)
	if issues := Validate(p, ValidateOptions{}); len(issues) != 0 {
		t.Fatalf("img_1 must be a valid name:\n%s", fmtIssues(issues))
	}
}
