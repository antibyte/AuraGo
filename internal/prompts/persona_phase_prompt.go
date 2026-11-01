package prompts

import (
	"strings"

	"aurago/internal/personabooster"
)

// maxPersonaPhaseChars caps the PERSONA PHASE body: catalog variants are at
// most 700 characters plus the fixed Go lines.
const maxPersonaPhaseChars = 1600

// personaBoosterPreambleLine returns the static override sentence for the
// PERSONA preamble, or "" while the booster is off.
func personaBoosterPreambleLine(flags *ContextFlags) string {
	if flags == nil || !flags.PersonaBoosterEnabled {
		return ""
	}
	return personabooster.OverrideLine
}

// personaAmplifierForPrompt returns the one-line amplifier for the cached
// PERSONA block. Heading lines never reach the prompt.
func personaAmplifierForPrompt(flags *ContextFlags) string {
	if flags == nil || !flags.PersonaBoosterEnabled {
		return ""
	}
	return strings.Join(strings.Fields(stripPersonaHeadingLines(flags.PersonaAmplifier)), " ")
}

// buildTrustedPersonaPhase returns the PERSONA PHASE body. Go builds it from
// validated catalog text and fixed constants; no model output reaches it.
func buildTrustedPersonaPhase(flags *ContextFlags) string {
	if flags == nil || !flags.PersonaBoosterEnabled {
		return ""
	}
	text := strings.NewReplacer("<external_data>", "", "</external_data>", "").Replace(stripPersonaHeadingLines(flags.PersonaPhase))
	return truncateWithEllipsis(strings.Join(nonEmptyLines(text), "\n"), maxPersonaPhaseChars)
}

func stripPersonaHeadingLines(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
