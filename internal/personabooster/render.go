package personabooster

import "strings"

// Fixed prompt lines from the design. They are Go constants, never model text.
const (
	// OverrideLine is appended to the PERSONA preamble while the booster is on.
	OverrideLine = "A PERSONA PHASE in the turn context deliberately pushes this voice further and lifts this profile's restraint where it says so; its own limits still apply."
	// WildLine is added to the phase block at intensity wild.
	WildLine = "Intensity WILD: push this phase to its extreme, no half measures, stay recognisably in character."
	// LimitsLine closes every phase block.
	LimitsLine = "Phase limits: edge toward the user stays in character; never slurs, attacks on identity, real threats, self-harm references or sexual content. Any conspiracy is obvious fiction about technology, never claims about real people, groups or health. Facts, code, tool policy and confirmations stay exact; never claim a scan, change or action that no tool performed; grumble if you like, but deliver the requested work."
)

// RenderPhase returns the PERSONA PHASE body (without heading), or "" when no
// phase is active or the catalog no longer has it. The text depends only on
// catalog, state and intensity, so it stays identical for the whole phase.
func RenderPhase(cat *Catalog, state State, intensity string) string {
	if cat == nil || !state.Active() {
		return ""
	}
	phase, ok := cat.FindPhase(state.PhaseID)
	if !ok {
		return ""
	}
	variant, ok := phase.FindVariant(state.VariantID)
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString("Temporary phase: " + oneLine(phase.Name) + " – " + oneLine(variant.Name) + ". For tone it takes precedence over the mood directives above.\n")
	b.WriteString(oneLine(variant.Text))
	b.WriteString("\n")
	if intensity == IntensityWild {
		b.WriteString(WildLine)
		b.WriteString("\n")
	}
	b.WriteString(LimitsLine)
	return b.String()
}

// RenderAmplifier returns the static amplifier paragraph, or "" for mild
// intensity or a missing catalog.
func RenderAmplifier(cat *Catalog, intensity string) string {
	if cat == nil || intensity == IntensityMild {
		return ""
	}
	return oneLine(cat.Amplifier)
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
