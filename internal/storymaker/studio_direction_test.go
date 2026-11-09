package storymaker

import (
	"slices"
	"testing"
)

func TestDirectionRoles(t *testing.T) {
	want := map[int][]string{
		1: {RoleHook},
		2: {RoleHook, RoleClose},
		3: {RoleHook, RoleHow, RoleClose},
		7: {RoleHook, RoleProblem, RoleSolution, RoleHow, RoleHow, RoleBenefit, RoleClose},
	}
	for n, roles := range want {
		if got := directionRoles(n); !slices.Equal(got, roles) {
			t.Errorf("directionRoles(%d) = %v, want %v", n, got, roles)
		}
	}
}

// Every suggested value is a member of a closed set or a spot that every environment has.
func TestDirectionStylesUseValidNames(t *testing.T) {
	cat := loadReal(t)
	for role, style := range directionStyles {
		if !slices.Contains(PoseNames, style.pose) || !slices.Contains(GestureNames, style.gesture) || !slices.Contains([]string{"wide", "medium"}, style.framing) {
			t.Errorf("%s: %+v holds an invalid name", role, style)
		}
	}
	for _, entry := range directionEntries {
		if !slices.Contains([]string{"pop", "fade", "slide"}, entry) {
			t.Errorf("entry %q is not accepted by the validator", entry)
		}
	}
	for _, id := range cat.Names("environment") {
		spots := cat.SpotNames(id)
		for _, spot := range []string{"left", "right", "center"} {
			if !slices.Contains(spots, spot) {
				t.Errorf("%s lacks the spot %s", id, spot)
			}
		}
	}
	for _, choice := range craftGestureChoices {
		if !slices.Contains(GestureNames, choice) {
			t.Errorf("craftGestureChoices holds %q, which is not a gesture", choice)
		}
	}
}

func TestDirectionPlanForTheRealFirewallRun(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-firewall-run")
	plan := DirectionPlan(&s, "", cat)
	if len(plan) != 7 || plan[0].Role != RoleHook || plan[6].Role != RoleClose {
		t.Fatalf("plan = %+v", plan)
	}
	envs := map[string]bool{}
	for _, d := range plan {
		if d.Environment == "" || d.Environment == plainStageID {
			t.Errorf("a scene is planned on %q", d.Environment)
		}
		envs[d.Environment] = true
	}
	if len(envs) != 3 {
		t.Errorf("a film of 7 scenes gets 3 environments, got %v", envs)
	}
	if plan[2].Environment != "env.abstract-tech" {
		t.Errorf("a story about a firewall puts its solution on abstract-tech, got %s", plan[2].Environment)
	}
	if plan[3].Environment != plan[2].Environment || plan[0].Environment != plan[1].Environment || plan[5].Environment != plan[6].Environment {
		t.Errorf("an act keeps one environment: %+v", plan)
	}
	if plan[0].Actor != "tobias" || plan[1].Actor != "laura" {
		t.Errorf("two cast members alternate: %s, %s", plan[0].Actor, plan[1].Actor)
	}
	if plan[0].ActorAt != "left" || plan[1].ActorAt != "right" {
		t.Errorf("the actor spot alternates: %s, %s", plan[0].ActorAt, plan[1].ActorAt)
	}
	if again := DirectionPlan(&s, "", cat); !slices.EqualFunc(plan, again, func(a, b SceneDirection) bool { return a == b }) {
		t.Error("the plan is not deterministic")
	}
}

func TestDirectionPlanDialogueStoryHasNoActorSuggestion(t *testing.T) {
	cat := loadReal(t)
	s := loadAgentFixture(t, "agent-varied")
	for _, d := range DirectionPlan(&s, "dialogue_story", cat) {
		if d.Actor != "" {
			t.Fatalf("a dialogue_story follows its speaker turns, got actor %q", d.Actor)
		}
	}
}

func TestDirectionPlanWithoutCatalogOrCastIsSafe(t *testing.T) {
	s := &Story{Title: "x", Scenes: []Scene{{ID: "s1", Narration: "a"}, {ID: "s2", Narration: "b"}}}
	plan := DirectionPlan(s, "", nil)
	if len(plan) != 2 || plan[0].Environment != "" || plan[0].Actor != "" {
		t.Fatalf("plan = %+v", plan)
	}
	if DirectionPlan(nil, "", nil) != nil || DirectionPlan(&Story{}, "", nil) != nil {
		t.Fatal("an empty story has no plan")
	}
}

func TestTagMatchesWholeWordsAndLongPrefixes(t *testing.T) {
	words := []string{"datenpakete", "tv", "firewall"}
	for tag, want := range map[string]bool{"daten": true, "paket": false, "firewall": true, "tv": true, "fire": true, "tvs": false} {
		if got := tagMatches(words, tag); got != want {
			t.Errorf("tagMatches(%q) = %v, want %v", tag, got, want)
		}
	}
}
