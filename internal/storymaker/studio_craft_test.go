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

// Two gestures that both pass the limit are flagged on the same scene and the same path. dedupeCheckIssues must keep both.
func TestFilmCraftKeepsEveryGestureFinding(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for i := range s.Scenes {
		sc := &s.Scenes[i]
		var beats []Beat
		for _, b := range sc.Beats {
			if b.Do != "gesture" {
				beats = append(beats, b)
			}
		}
		sc.Beats = append(beats, Beat{Do: "gesture", Who: "anna", Gesture: "point"}, Beat{Do: "gesture", Who: "anna", Gesture: "wave"})
	}
	countGestures := func(list []Issue) (n int) {
		for _, is := range list {
			if is.Code == CodeGestureMonotony {
				n++
			}
		}
		return n
	}
	found := LintFilmCraft(&s, "", cat)
	var gestures []Issue
	for _, is := range found {
		if is.Code == CodeGestureMonotony {
			gestures = append(gestures, is)
		}
	}
	if len(gestures) != 2 || gestures[0].SceneID != gestures[1].SceneID || gestures[0].Path != gestures[1].Path {
		t.Fatalf("the setup must give two gesture findings on one scene and path, got %s", dump(gestures))
	}
	if got := countGestures(dedupeCheckIssues(found)); got != 2 {
		t.Fatalf("dedupeCheckIssues kept %d gesture findings, want 2", got)
	}
	if got := countGestures(dedupeCheckIssues(append(slices.Clone(found), found...))); got != 2 {
		t.Fatalf("the same findings twice must collapse to 2, got %d", got)
	}
}

// A code scene has no environment, no framing, no gesture and no actors. It is not counted and never named in a fix.
func TestFilmCraftLeavesCodeScenesOut(t *testing.T) {
	cat := loadReal(t)
	onStage := func(id, narration string) Scene {
		return Scene{
			ID: id, Environment: "env.office", Narration: narration, Speaker: "anna",
			Actors: []Actor{{Cast: "anna", At: SpotAt("left"), Pose: "stand"}},
		}
	}
	s := Story{
		Title: "Firewall",
		Cast: []CastMember{
			{ID: "anna", Name: "Anna", Character: "char.office-woman-1"},
			{ID: "ben", Name: "Ben", Character: "char.office-man-1"},
		},
		Scenes: []Scene{
			onStage("s1", "Eine Firewall schuetzt das Netz."),
			{ID: "s2", Kind: "code", Narration: "Die Firewall im Code."},
			onStage("s3", "Sie prueft jedes Paket."),
			onStage("s4", "Sie sperrt Fremde aus."),
			onStage("s5", "Sie meldet jeden Versuch."),
			onStage("s6", "So bleibt das Netz sicher."),
		},
	}
	got := LintFilmCraft(&s, "", cat)

	var unused, environment *Issue
	for i := range got {
		switch got[i].Code {
		case CodeCastUnused:
			unused = &got[i]
		case CodeEnvironmentMonotony:
			environment = &got[i]
		}
	}
	if unused == nil || !strings.Contains(unused.Message, "ben") {
		t.Fatalf("ben never appears and must be flagged, got %s", dump(got))
	}
	if unused.SceneID == "s2" || unused.SceneID == "" {
		t.Fatalf("the fix must name a storyboard scene, not the code scene: %+v", *unused)
	}
	if environment == nil || !strings.Contains(environment.Message, "All 5 scenes play in 1 environment") {
		t.Fatalf("the code scene's empty environment is no environment and no scene of the count, got %s", dump(got))
	}

	plan := DirectionPlan(&s, "", cat)
	if d := plan[1]; d.Role == "" || d.Actor != "" || d.HeroProp != "" || d.ActorAt != "" || d.PropAt != "" {
		t.Fatalf("a code scene keeps its role and gets no actor, prop or spot: %+v", d)
	}
	if plan[2].Actor != "anna" || plan[2].ActorAt == "" {
		t.Fatalf("the storyboard scenes keep the rotation over all scenes: %+v", plan[2])
	}

	// Four scenes with one code scene are three storyboard scenes: too short for the craft lint.
	short := s
	short.Scenes = s.Scenes[:4]
	if got := LintFilmCraft(&short, "", cat); len(got) != 0 {
		t.Fatalf("3 storyboard scenes are exempt: %s", dump(got))
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
