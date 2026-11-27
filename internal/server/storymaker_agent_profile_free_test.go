package server

import (
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/storymaker"
)

func TestStorymakerFreeProfilesTellTheFreeStory(t *testing.T) {
	names := agent.StorymakerNames{Poses: []string{"stand"}, Gestures: []string{"wave"}, Expressions: []string{"neutral"}}
	for _, compact := range []bool{false, true} {
		for _, text := range []bool{false, true} {
			for _, phase := range []string{storymaker.PhaseOutline, storymaker.PhaseFill, storymaker.PhaseCheck, storymaker.PhaseRevise} {
				profile, err := storymakerAgentProfileFor(storymaker.ModeFree, phase, compact, text, false, names)
				if err != nil {
					t.Fatalf("%s compact=%v text=%v: %v", phase, compact, text, err)
				}
				prompt := profile.SystemPrompt()
				if !strings.Contains(prompt, "storymaker_story") || !strings.Contains(prompt, "no catalog") {
					t.Errorf("%s: the free prompt lacks its ground rules", phase)
				}
				for _, banned := range []string{"## Cast", "char.", "prop.", "env.", "Poses:", "Gestures:", "environment_guide"} {
					if strings.Contains(prompt, banned) {
						t.Errorf("%s compact=%v text=%v: the free prompt contains %q", phase, compact, text, banned)
					}
				}
			}
		}
	}
	if _, err := storymakerAgentProfileFor(storymaker.ModeFree, storymaker.PhaseCast, false, false, false, names); err == nil {
		t.Error("a free project has no cast phase")
	}
	fill, _ := storymakerAgentProfileFor(storymaker.ModeFree, storymaker.PhaseFill, false, false, false, names)
	if !strings.Contains(fill.SystemPrompt(), "## Fill one scene") || !strings.Contains(fill.SystemPrompt(), "put_code") {
		t.Error("the free fill workflow must keep its heading and name put_code")
	}
	if !strings.Contains(fill.Name(), "storymaker/v6/free/fill/") {
		t.Errorf("profile name = %q", fill.Name())
	}
	if prompt := fill.SystemPrompt(); !strings.Contains(prompt, "code_rev") || !strings.Contains(prompt, "api_card") || strings.Contains(prompt, "before the first scene") {
		t.Error("the free fill prompt must name code_rev and api_card and must not send the model to the reference first")
	}
	catalog, _ := storymakerAgentProfile(storymaker.PhaseFill, false, false, names)
	if strings.Contains(catalog.SystemPrompt(), "no catalog") || strings.Contains(catalog.Name(), "/free/") {
		t.Error("the catalog profile must not change")
	}
	nudge := storymakerRecoveryNudgeFor(storymaker.ModeFree, storymaker.PhaseFill)(false)
	if !strings.Contains(nudge, "put_code") || !strings.Contains(nudge, "code_rev") || strings.Contains(nudge, "search") {
		t.Errorf("free fill nudge = %q", nudge)
	}
}
