package prompts

import (
	"strings"

	"aurago/internal/personabooster"
)

// maxPersonaPhaseBytes caps the variable part of the PERSONA PHASE body (header
// line plus catalog variant). The catalog validator limits a variant to 700
// runes, not bytes: 700 four-byte runes plus two 60-rune names and the header
// sentence stay below 3.5 KB. The fixed Go lines after it are not counted and
// never cut.
const maxPersonaPhaseBytes = 4096

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
// validated catalog text and fixed constants; no model output reaches it. The
// trailing fixed lines (personabooster.WildLine at wild intensity and the
// closing personabooster.LimitsLine) are kept verbatim; only the text before
// them is capped.
func buildTrustedPersonaPhase(flags *ContextFlags) string {
	if flags == nil || !flags.PersonaBoosterEnabled {
		return ""
	}
	text := strings.NewReplacer("<external_data>", "", "</external_data>", "").Replace(stripPersonaHeadingLines(flags.PersonaPhase))
	lines := nonEmptyLines(text)
	var fixed []string
	if n := len(lines); n > 0 && lines[n-1] == personabooster.LimitsLine {
		fixed, lines = []string{personabooster.LimitsLine}, lines[:n-1]
		if n := len(lines); n > 0 && lines[n-1] == personabooster.WildLine {
			fixed, lines = []string{personabooster.WildLine, personabooster.LimitsLine}, lines[:n-1]
		}
	}
	parts := make([]string, 0, 1+len(fixed))
	if head := truncateWithEllipsis(strings.Join(lines, "\n"), maxPersonaPhaseBytes); head != "" {
		parts = append(parts, head)
	}
	return strings.Join(append(parts, fixed...), "\n")
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
