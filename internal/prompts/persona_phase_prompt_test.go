package prompts

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/personabooster"
)

const testPersonaPhase = "Temporary phase: Rage – Cold fury. For tone it takes precedence over the mood directives above.\nYou speak in clipped sentences.\n" + personabooster.LimitsLine

func personaBoosterTestFlags(phase string) ContextFlags {
	return ContextFlags{
		Tier: "full", SystemLanguage: "en", CorePersonality: "punk", TokenBudget: 200000,
		PersonalityLine: "Stay sharp.", CharacterNotes: "CHARACTER_MARKER",
		PersonaBoosterEnabled: true, PersonaAmplifier: "AMPLIFIER_MARKER lean harder into the punk edge.",
		PersonaPhase: phase,
	}
}

func TestPersonaPhaseBlockSitsInTurnContextAndKeepsPrefixStable(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	dir := t.TempDir()
	withPhase := personaBoosterTestFlags(testPersonaPhase)
	withoutPhase := personaBoosterTestFlags("")
	promptA, _ := buildSystemPromptInner(dir, &withPhase, "", slog.Default())
	promptB, _ := buildSystemPromptInner(dir, &withoutPhase, "", slog.Default())
	prefixA, suffixA, okA := strings.Cut(promptA, "# TURN CONTEXT")
	prefixB, _, okB := strings.Cut(promptB, "# TURN CONTEXT")
	if !okA || !okB {
		t.Fatal("missing # TURN CONTEXT")
	}
	if prefixA != prefixB {
		t.Fatal("the persona phase changed the cached prefix")
	}
	if strings.Contains(prefixA, "### PERSONA PHASE") || !strings.Contains(suffixA, "### PERSONA PHASE\nTemporary phase: Rage – Cold fury.") {
		t.Fatalf("phase block misplaced:\n%s", promptA)
	}
	state := strings.Index(suffixA, "### PERSONA STATE")
	phase := strings.Index(suffixA, "### PERSONA PHASE")
	character := strings.Index(suffixA, "### PERSONA CHARACTER")
	if state < 0 || phase < state || character < phase {
		t.Fatalf("order must be STATE, PHASE, CHARACTER: %d %d %d", state, phase, character)
	}
	if !strings.Contains(promptSection(suffixA, "### PERSONA PHASE"), personabooster.LimitsLine) {
		t.Fatal("phase limits line missing")
	}
}

func TestPersonaAmplifierAndOverrideOnlyWithBooster(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	dir := t.TempDir()
	on := personaBoosterTestFlags(testPersonaPhase)
	off := on
	off.PersonaBoosterEnabled = false
	promptOn, _ := buildSystemPromptInner(dir, &on, "", slog.Default())
	promptOff, _ := buildSystemPromptInner(dir, &off, "", slog.Default())
	prefixOn, _, _ := strings.Cut(promptOn, "# TURN CONTEXT")
	prefixOff, _, _ := strings.Cut(promptOff, "# TURN CONTEXT")
	persona := strings.Index(prefixOn, "# PERSONA (ACTIVE PROFILE: PUNK)")
	override := strings.Index(prefixOn, personabooster.OverrideLine)
	amplifier := strings.Index(prefixOn, "AMPLIFIER_MARKER")
	if persona < 0 || override < persona || amplifier < override {
		t.Fatalf("override line and amplifier must follow the persona heading in the prefix:\n%s", prefixOn)
	}
	if strings.Contains(promptOff, personabooster.OverrideLine) || strings.Contains(promptOff, "AMPLIFIER_MARKER") || strings.Contains(promptOff, "### PERSONA PHASE") {
		t.Fatal("booster text rendered while the booster is off")
	}
	if !strings.Contains(prefixOff, "# PERSONA (ACTIVE PROFILE: PUNK)") {
		t.Fatal("persona block lost without booster")
	}
}

func TestPersonaPhaseAbsentForMissionAndCoAgent(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	dir := t.TempDir()
	for name, mutate := range map[string]func(*ContextFlags){
		"mission":  func(f *ContextFlags) { f.IsMission = true },
		"co-agent": func(f *ContextFlags) { f.IsCoAgent = true },
	} {
		flags := personaBoosterTestFlags(testPersonaPhase)
		mutate(&flags)
		prompt, _ := buildSystemPromptInner(dir, &flags, "", slog.Default())
		for _, marker := range []string{"### PERSONA PHASE", "AMPLIFIER_MARKER", personabooster.OverrideLine} {
			if strings.Contains(prompt, marker) {
				t.Errorf("%s: unexpected %q", name, marker)
			}
		}
	}
}

func TestPersonaPhaseSurvivesPromptFitting(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	dir := t.TempDir()
	lean := personaBoosterTestFlags(testPersonaPhase)
	lean.TokenBudget, lean.CharacterNotes = 0, ""
	base := BuildSystemPromptBaseDetailed(context.Background(), dir, &lean, "", slog.Default())
	rich := lean
	rich.UserProfilingEnabled = true
	rich.UserProfileSummary = strings.Repeat("Prefers concise answers. ", 60)
	rich.EmotionDescription = strings.Repeat("Happy about our progress. ", 30)
	rich.CharacterNotes = strings.Repeat("A lived experience. ", 30)
	full := BuildSystemPromptBaseDetailed(context.Background(), dir, &rich, "", slog.Default())
	fit, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: full.Text, Tokens: full.Tokens, TokenBudget: base.Tokens + 40}, slog.Default())
	if err != nil || fit.BudgetExceeded != nil {
		t.Fatalf("fit failed: %v %+v", err, fit.BudgetExceeded)
	}
	if strings.Contains(fit.Text, "## USER PROFILING") {
		t.Fatal("fixture did not shed user profiling")
	}
	if !strings.Contains(fit.Text, "### PERSONA PHASE\nTemporary phase: Rage – Cold fury.") {
		t.Fatalf("the persona phase was shed with an optional section:\n%s", fit.Text)
	}
}
