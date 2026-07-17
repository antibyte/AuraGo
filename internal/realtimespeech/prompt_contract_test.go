package realtimespeech

import (
	"strings"
	"testing"
)

func TestAuraGoVoiceContractPreservesMonolithicExecution(t *testing.T) {
	contract := strings.ToLower(AuraGoSystemContract)
	required := []string{
		"one monolithic aurago system",
		"speak in the first person as aurago",
		"casual conversation and stable general knowledge directly",
		"you must call aurago_execute",
		"one short honest progress acknowledgement",
		"do not claim success before",
		"status is needs_input",
		"if completed",
		"if cancelled or error",
		"details are in the chat",
		"cancel an aurago action only",
	}
	for _, phrase := range required {
		if !strings.Contains(contract, phrase) {
			t.Errorf("voice contract is missing %q", phrase)
		}
	}

	forbiddenClaims := []string{
		"i will forward",
		"i am forwarding",
		"i will hand this off",
		"another model will",
		"the internal agent will",
	}
	for _, phrase := range forbiddenClaims {
		if strings.Contains(contract, phrase) {
			t.Errorf("voice contract contains forbidden handoff phrasing %q", phrase)
		}
	}
}
