package server

import (
	"slices"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/storymaker"
)

func TestStorymakerProfilesNamesToolsAndTextGuide(t *testing.T) {
	names := agent.StorymakerNames{
		Poses:        []string{"stand", "wave"},
		Gestures:     []string{"point"},
		Expressions:  []string{"neutral"},
		Environments: []string{"env.office"},
		Characters:   []string{"char.office-woman-1"},
	}
	headings := map[string]string{
		storymaker.PhaseOutline: "## Outline",
		storymaker.PhaseCast:    "## Cast",
		storymaker.PhaseFill:    "## Fill one scene",
		storymaker.PhaseCheck:   "## Check and fix",
		storymaker.PhaseRevise:  "## Revise",
	}
	for phase, heading := range headings {
		for _, compact := range []bool{false, true} {
			for _, text := range []bool{false, true} {
				profile, err := storymakerAgentProfile(phase, compact, text, names)
				if err != nil {
					t.Fatalf("%s compact=%v text=%v: %v", phase, compact, text, err)
				}
				wantName, house, otherHouse := "storymaker/v1/"+phase+"/normal", "## House style", "## House rules"
				if compact {
					wantName, house, otherHouse = "storymaker/v1/"+phase+"/compact", "## House rules", "## House style"
				}
				if text {
					wantName += "/text"
				}
				if profile.Name() != wantName {
					t.Fatalf("profile name = %q, want %q", profile.Name(), wantName)
				}

				var gotTools []string
				for _, tool := range profile.Tools() {
					gotTools = append(gotTools, tool.Function.Name)
				}
				phaseTools := storymaker.PhaseTools(phase)
				wantTools := slices.Clone(phaseTools)
				slices.Sort(gotTools)
				slices.Sort(wantTools)
				if !slices.Equal(gotTools, wantTools) {
					t.Fatalf("%s tools = %v, want %v", wantName, gotTools, wantTools)
				}

				system := profile.SystemPrompt()
				if strings.Contains(system, otherHouse) {
					t.Fatalf("%s holds %s", wantName, otherHouse)
				}
				ground, houseAt, workflowAt := strings.Index(system, "## Ground rules"), strings.Index(system, house), strings.Index(system, heading)
				if ground < 0 || houseAt <= ground || workflowAt <= houseAt {
					t.Fatalf("%s has its sections out of order: %d, %d, %d", wantName, ground, houseAt, workflowAt)
				}

				guideAt := strings.Index(system, "## Calling tools")
				if !text {
					if guideAt >= 0 {
						t.Fatalf("%s holds the text-mode guide", wantName)
					}
					continue
				}
				if guideAt <= workflowAt {
					t.Fatalf("%s must end with the text-mode guide", wantName)
				}
				for _, line := range []string{"Poses: stand, wave", "Gestures: point", "Expressions: neutral", "Transitions: cut, fade, slide-left, wipe"} {
					if !strings.Contains(system, line) {
						t.Fatalf("%s lacks %q", wantName, line)
					}
				}
				for _, example := range storymakerToolExamples[phase] {
					if !strings.Contains(system, example) {
						t.Fatalf("%s lacks the example %s", wantName, example)
					}
					allowed := slices.ContainsFunc(phaseTools, func(tool string) bool {
						return strings.Contains(example, `"action":"`+tool+`"`)
					})
					if !allowed {
						t.Fatalf("%s shows a tool outside the phase: %s", wantName, example)
					}
				}
			}
		}
	}
	if _, err := storymakerAgentProfile("unknown", false, false, names); err == nil {
		t.Fatal("an unknown phase must be refused")
	}
}
