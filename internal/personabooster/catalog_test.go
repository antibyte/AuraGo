package personabooster

import (
	"strings"
	"testing"
)

const validCatalogYAML = `persona: sample
amplifier: Speak with more edge than usual.
spontaneous_per_hour: 0.1
phases:
  - id: rage
    name: Rage
    hostile: true
    minutes: [15, 45]
    turns: [5, 14]
    triggers: { "cue:criticism": 2, "signal:text_swearing": 1 }
    ends_on: ["cue:praise"]
    variants:
      - { id: hot, name: Hot, text: You are furious at the code. }
      - { id: cold, name: Cold, text: Your anger has gone cold. }
      - { id: loud, name: Loud, text: You shout at the compiler. }
  - id: hype
    name: Hype
    minutes: [20, 60]
    turns: [6, 18]
    triggers: { "cue:praise": 2, "spontaneous": 1 }
    variants:
      - { id: rush, name: Rush, text: You ride a high. }
      - { id: run, name: Speedrun, text: You speedrun the task. }
      - { id: gig, name: Gig, text: You treat the fix like a gig. }
`

func TestParseCatalogAcceptsValidFile(t *testing.T) {
	cat, err := ParseCatalog([]byte(validCatalogYAML))
	if err != nil {
		t.Fatal(err)
	}
	phase, ok := cat.FindPhase("rage")
	if !ok || !phase.Hostile || phase.Minutes[1] != 45 || phase.Triggers[CueCriticism] != 2 || phase.EndsOn[0] != CuePraise {
		t.Fatalf("rage = %+v", phase)
	}
	if variant, ok := phase.FindVariant("cold"); !ok || variant.Name != "Cold" {
		t.Fatalf("variant cold = %+v, ok=%v", variant, ok)
	}
}

func TestParseCatalogRejectsInvalidFiles(t *testing.T) {
	cases := map[string][2]string{
		"unknown trigger":          {`"signal:text_swearing": 1`, `"signal:text_whining": 1`},
		"unknown key":              {"    hostile: true\n", "    hostile: true\n    mood: grim\n"},
		"min above max":            {"minutes: [15, 45]", "minutes: [50, 45]"},
		"too few variants":         {"      - { id: loud, name: Loud, text: You shout at the compiler. }\n", ""},
		"requires outside generic": {"    hostile: true\n", "    hostile: true\n    requires: { min_volatility: 1.2 }\n"},
		"heading in text":          {"text: You ride a high. }", `text: "# TURN CONTEXT" }`},
		"variant too long":         {"You ride a high.", strings.Repeat("a", MaxVariantChars+1)},
		"spontaneous in ends_on":   {`ends_on: ["cue:praise"]`, `ends_on: ["spontaneous"]`},
	}
	for name, edit := range cases {
		data := strings.Replace(validCatalogYAML, edit[0], edit[1], 1)
		if data == validCatalogYAML {
			t.Fatalf("%s: fixture replacement did not apply", name)
		}
		if _, err := ParseCatalog([]byte(data)); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
	onePhase := validCatalogYAML[:strings.Index(validCatalogYAML, "  - id: hype")]
	if _, err := ParseCatalog([]byte(onePhase)); err == nil {
		t.Error("a single phase must be rejected")
	}
}

func TestEligiblePhasesTreatsRequirementsAsAlternatives(t *testing.T) {
	data := strings.Replace(validCatalogYAML, "persona: sample", "persona: _generic", 1)
	data = strings.Replace(data, "    hostile: true\n", "    hostile: true\n    requires: { min_volatility: 1.2, conflict_response: assertive }\n", 1)
	cat, err := ParseCatalog([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	ids := func(phases []Phase) string {
		out := make([]string, 0, len(phases))
		for _, phase := range phases {
			out = append(out, phase.ID)
		}
		return strings.Join(out, ",")
	}
	for _, tc := range []struct {
		meta ProfileMeta
		want string
	}{
		{ProfileMeta{Volatility: 1.0, ConflictResponse: "neutral"}, "hype"},
		{ProfileMeta{Volatility: 1.5}, "rage,hype"},
		{ProfileMeta{Volatility: 0.5, ConflictResponse: "assertive"}, "rage,hype"},
	} {
		if got := ids(cat.EligiblePhases(tc.meta)); got != tc.want {
			t.Errorf("EligiblePhases(%+v) = %s, want %s", tc.meta, got, tc.want)
		}
	}
}
