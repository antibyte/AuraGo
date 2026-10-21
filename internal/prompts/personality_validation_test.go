package prompts

import (
	"strings"
	"testing"
)

func TestPersonalityMetadataRejectsUnknownAndInvalidFields(t *testing.T) {
	for _, source := range []string{
		"meta:\n  volatility: 5", "meta:\n  volatiltiy: 1", "meta:\n  empathy_bias: .nan", "meta:\n  conflict_response: panic",
		"meta:\n  anchor_traits:\n    imaginary: .5", "meta:\n  decay_resistance:\n    curiosity: 3",
		"meta:\n  thresholds:\n    high_curiosity: 4", "meta:\n  thresholds:\n    high_curioisty: .7", "unknown_metadata: true",
	} {
		t.Run(strings.ReplaceAll(source, "\n", " "), func(t *testing.T) {
			if err := ValidatePersonalitySource("---\nid: custom\n" + source + "\n---\nBody"); err == nil {
				t.Fatal("accepted invalid metadata")
			}
		})
	}
	for _, source := range []string{"Plain Markdown persona", "---\nid: custom\nmeta:\n  volatility: 0\n  empathy_bias: 0\n  thresholds:\n    high_curiosity: .75\n---\nBody"} {
		if err := ValidatePersonalitySource(source); err != nil {
			t.Fatal(err)
		}
	}
}
