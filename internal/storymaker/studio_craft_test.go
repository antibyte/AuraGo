package storymaker

import (
	"slices"
	"strings"
	"testing"
)

func craftCodes(issues []Issue) []string {
	var out []string
	for _, is := range issues {
		out = append(out, is.Code)
	}
	return out
}

// The real run of 2026-11-08 must trip all four film-level lints. This is the red test of the batch.
func TestFilmCraftFlagsTheRealFirewallRun(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	got := LintFilmCraft(&s, "", cat)
	for _, want := range []string{CodeEnvironmentMonotony, CodeFramingMonotony, CodeGestureMonotony, CodeCastUnused} {
		if !slices.Contains(craftCodes(got), want) {
			t.Errorf("codes = %v, missing %s", craftCodes(got), want)
		}
	}
	for _, is := range got {
		if is.Severity != SeverityWarning || is.Fix == "" || !IssueCraft(is.Code) || !IssueModelFixable(is) {
			t.Errorf("%s must be a warning with a fix, a craft code and model-fixable: %+v", is.Code, is)
		}
	}
	for _, is := range got {
		switch is.Code {
		case CodeEnvironmentMonotony:
			if !strings.Contains(is.Fix, "env.") || !strings.Contains(is.Fix, "wipe") {
				t.Errorf("the environment fix must name an environment and the transition: %s", is.Fix)
			}
		case CodeGestureMonotony:
			if is.SceneID == "" || !strings.Contains(is.Message, `"point"`) {
				t.Errorf("the gesture finding must name point and the first scene over the limit: %+v", is)
			}
		case CodeCastUnused:
			if !strings.Contains(is.Message, "laura") || is.SceneID == "" {
				t.Errorf("the cast finding must name laura and a scene: %+v", is)
			}
		}
	}
}

func TestFilmCraftPassesTheVariedStory(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	if got := LintFilmCraft(&s, "", cat); len(got) != 0 {
		t.Fatalf("the varied story must pass: %s", dump(got))
	}
}

func TestFilmCraftSkipsShortFilmsAndExemptsDialogueEnvironments(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	short := s
	short.Scenes = s.Scenes[:3]
	if got := LintFilmCraft(&short, "", cat); len(got) != 0 {
		t.Fatalf("a film of 3 scenes is exempt: %s", dump(got))
	}
	if got := LintFilmCraft(&s, "dialogue_story", cat); slices.Contains(craftCodes(got), CodeEnvironmentMonotony) {
		t.Fatalf("a dialogue_story never gets the environment lint: %v", craftCodes(got))
	}
	if got := LintFilmCraft(nil, "", cat); got != nil {
		t.Fatalf("nil story = %v", got)
	}
}

// The limit is max(2, ceil(n/3)) scenes per gesture: 7 scenes allow 3, a fourth one is the finding.
func TestFilmCraftGestureLimit(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i := range s.Scenes {
		for j := range s.Scenes[i].Beats {
			if s.Scenes[i].Beats[j].Do == "gesture" {
				s.Scenes[i].Beats[j].Gesture = "point"
			}
		}
	}
	got := LintFilmCraft(&s, "", cat)
	var gesture *Issue
	for i := range got {
		if got[i].Code == CodeGestureMonotony {
			gesture = &got[i]
		}
	}
	if gesture == nil || gesture.SceneID != "s4" {
		t.Fatalf("7 scenes with point must flag s4 (the 4th), got %+v", got)
	}
	// Three scenes with the same gesture are still inside the limit.
	for i := 3; i < len(s.Scenes); i++ {
		for j := range s.Scenes[i].Beats {
			if s.Scenes[i].Beats[j].Do == "gesture" {
				s.Scenes[i].Beats[j].Gesture = []string{"", "", "", "wave", "shrug", "present", "thumbs-up"}[i]
			}
		}
	}
	if got := LintFilmCraft(&s, "", cat); slices.Contains(craftCodes(got), CodeGestureMonotony) {
		t.Fatalf("3 uses are allowed, got %v", craftCodes(got))
	}
}

// A camera beat or a framing other than wide counts as a choice.
func TestFilmCraftFramingChoices(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	if !slices.Contains(craftCodes(LintFilmCraft(&s, "", cat)), CodeFramingMonotony) {
		t.Fatal("the run has only wide scenes and must be flagged")
	}
	withFraming := s
	withFraming.Scenes = slices.Clone(s.Scenes)
	withFraming.Scenes[2].Camera = &Camera{Framing: "medium"}
	if slices.Contains(craftCodes(LintFilmCraft(&withFraming, "", cat)), CodeFramingMonotony) {
		t.Fatal("one medium scene is a choice")
	}
	withBeat := s
	withBeat.Scenes = slices.Clone(s.Scenes)
	withBeat.Scenes[4].Beats = append(slices.Clone(s.Scenes[4].Beats), Beat{Do: "camera"})
	if slices.Contains(craftCodes(LintFilmCraft(&withBeat, "", cat)), CodeFramingMonotony) {
		t.Fatal("a camera beat is a choice")
	}
	withTarget := s
	withTarget.Scenes = slices.Clone(s.Scenes)
	withTarget.Scenes[1].Camera = &Camera{Framing: "wide", Target: "left"}
	if slices.Contains(craftCodes(LintFilmCraft(&withTarget, "", cat)), CodeFramingMonotony) {
		t.Fatal("a camera target is a choice")
	}
}

func TestFilmCraftCastUnusedIgnoresTheNarrator(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	s.Scenes[6].Speaker = "narrator"
	if got := LintFilmCraft(&s, "", cat); len(got) != 0 {
		t.Fatalf("narrator is not a cast member and anna still appears: %s", dump(got))
	}
	s.Cast = append(s.Cast, CastMember{ID: "cleo", Name: "Cleo", Character: "char.office-woman-1"})
	s.Cast = append(s.Cast, CastMember{ID: "dan", Name: "Dan", Character: "char.it-tech-1"})
	got := LintFilmCraft(&s, "", cat)
	paths := map[string]bool{}
	for _, is := range got {
		if is.Code == CodeCastUnused {
			paths[is.Path] = true
		}
	}
	if len(paths) != 2 {
		t.Fatalf("two unused members must give two findings with different paths, got %s", dump(got))
	}
}
