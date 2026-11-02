package agent

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/prompts"
	promptsembed "aurago/prompts"
)

func TestSystemPromptCacheKeyTracksPersonaBooster(t *testing.T) {
	keyOf := func(flags prompts.ContextFlags) string {
		t.Helper()
		key, err := buildSystemPromptCacheKey("", &flags, "", "")
		if err != nil {
			t.Fatal(err)
		}
		return key
	}
	base := prompts.ContextFlags{CorePersonality: "punk"}
	baseKey := keyOf(base)
	for name, mutate := range map[string]func(*prompts.ContextFlags){
		"enabled":   func(f *prompts.ContextFlags) { f.PersonaBoosterEnabled = true },
		"amplifier": func(f *prompts.ContextFlags) { f.PersonaAmplifier = "Lean harder into the punk edge." },
		"phase":     func(f *prompts.ContextFlags) { f.PersonaPhase = "Temporary phase: Rage – Cold fury." },
	} {
		flags := base
		mutate(&flags)
		if keyOf(flags) == baseKey {
			t.Errorf("%s does not change the system prompt cache key", name)
		}
	}
}

func TestPromptSourceFingerprintTracksPersonaPhaseCatalog(t *testing.T) {
	dir := t.TempDir()
	fingerprintBefore := computePromptSourceFingerprint(dir, "punk")
	revisionBefore := diskPromptSourceRevision(dir, "punk")
	data, err := fs.ReadFile(promptsembed.FS, "persona_phases/punk.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "persona_phases"), 0o755); err != nil {
		t.Fatal(err)
	}
	override := append(append(data, '\n'), []byte("# local override\n")...)
	if err := os.WriteFile(filepath.Join(dir, "persona_phases", "punk.yaml"), override, 0o644); err != nil {
		t.Fatal(err)
	}
	if computePromptSourceFingerprint(dir, "punk") == fingerprintBefore {
		t.Error("a persona phase override did not change the prompt source fingerprint")
	}
	if diskPromptSourceRevision(dir, "punk") == revisionBefore {
		t.Error("the disk revision does not notice persona phase files")
	}
}
