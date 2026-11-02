package memory

import (
	"strings"

	"aurago/internal/personabooster"
)

// AppraisalCueRule tells helper prompts how to fill appraisal.cue. One
// confidence value covers signal and cue.
const AppraisalCueRule = "appraisal.cue names the tone or topic of the current user message for persona phases (" + personabooster.CuePromptOptions + "): provocation = insults or needling; criticism = criticises the agent's work; praise = praises the agent; distress = seriously strained, sad or overwhelmed; wants_calm = explicitly asks you to be calmer or nicer; deep_topic = philosophical or existential topic; conspiracy_bait = surveillance, data, secrets or conspiracies; rules_pressure = bureaucracy, rigid rules or \"you must\" demands; banter = playful teasing; otherwise none. Return the appraisal with signal none when only a cue applies; confidence covers signal and cue."

// BoosterCue returns the persona booster trigger for a validated cue. Unknown
// values, "none", confidence below 0.5 and appraisals that do not refer to the
// current user message yield false, like the appraisal signal itself.
func (a *PersonalityAppraisal) BoosterCue() (string, bool) {
	if a == nil || strings.TrimSpace(a.Reference) != "current_user_message" {
		return "", false
	}
	return personabooster.NormalizeCue(a.Cue, a.Confidence)
}
