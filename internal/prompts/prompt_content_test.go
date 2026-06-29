package prompts

import (
	"strings"
	"testing"

	promptsembed "aurago/prompts"
)

func TestPromptContentIdentityAndToolAvailabilityWording(t *testing.T) {
	identityData, err := promptsembed.FS.ReadFile("identity.md")
	if err != nil {
		t.Fatalf("read identity: %v", err)
	}
	identity := string(identityData)
	for _, want := range []string{
		"Your default name is AuraGo",
		"user's naming preferences",
	} {
		if !strings.Contains(identity, want) {
			t.Fatalf("identity prompt missing %q", want)
		}
	}

	rulesData, err := promptsembed.FS.ReadFile("rules.md")
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	rules := string(rulesData)
	if !strings.Contains(rules, "when the tool is available") {
		t.Fatal("rules prompt should gate tool-specific instructions on tool availability")
	}
}
