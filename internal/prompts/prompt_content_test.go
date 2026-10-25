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

// The root tools_*.md files and mission_preparation.md are kept out of the
// module system (isExcludedRootPromptModule). The mandatory tag overrides every
// other inclusion rule, so none of them may carry it: a future loader change
// that stops excluding them would otherwise put them into every system prompt.
func TestExcludedRootPromptsCarryNoMandatoryTag(t *testing.T) {
	entries, err := promptsembed.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || !isExcludedRootPromptModule(name) {
			continue
		}
		data, err := promptsembed.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		module, _, err := ParsePromptSource(name, string(data), nil)
		if err != nil {
			t.Errorf("parse %s: %v", name, err)
			continue
		}
		checked++
		for _, tag := range module.Metadata.Tags {
			if tag == "mandatory" {
				t.Errorf("%s is excluded from the prompt modules but carries the tag %q", name, tag)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no excluded root prompt was checked")
	}
}
