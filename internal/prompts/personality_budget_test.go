package prompts

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestPersonalityCoreSurvivesRealPromptFitting(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	for _, persona := range []string{"friend", "punk", "neutral"} {
		t.Run(persona, func(t *testing.T) {
			flags := &ContextFlags{Tier: "full", CorePersonality: persona, PersonalityLine: "Stay playful in the active persona's voice."}
			base := BuildSystemPromptBaseDetailed(context.Background(), t.TempDir(), flags, "", slog.Default())
			flags.EmotionDescription = strings.Repeat("Happy about our progress. ", 30)
			flags.CharacterNotes = strings.Repeat("A lived experience. ", 30)
			rich := BuildSystemPromptBaseDetailed(context.Background(), t.TempDir(), flags, "", slog.Default())
			fit, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: rich.Text, Tokens: rich.Tokens, TokenBudget: base.Tokens + 20}, slog.Default())
			if err != nil || fit.BudgetExceeded != nil {
				t.Fatalf("fit failed: %v %+v", err, fit.BudgetExceeded)
			}
			if len(fit.RemovedSections) == 0 {
				t.Fatal("fixture did not cause shedding")
			}
			for _, heading := range []string{"# PERSONA (ACTIVE PROFILE:", "### PERSONA STATE"} {
				if !strings.Contains(fit.Text, heading) {
					t.Fatalf("lost %s", heading)
				}
			}
			body := loadCorePersonalityContent(t.TempDir(), persona, slog.Default())
			if !strings.Contains(fit.Text, body) {
				t.Fatal("selected profile body was lost")
			}
			tooSmall, err := FitSystemPromptToBudget(context.Background(), PromptFitRequest{Text: rich.Text, Tokens: rich.Tokens, TokenBudget: 1}, slog.Default())
			if err != nil {
				t.Fatal(err)
			}
			if tooSmall.BudgetExceeded == nil || !strings.Contains(tooSmall.Text, body) {
				t.Fatal("impossible budget silently erased identity")
			}
		})
	}
}

func TestPersonaSignalsKeepFullUnicodeDescriptionAndTone(t *testing.T) {
	flags := &ContextFlags{
		EmotionDescription: "Ich freue mich über unseren gemeinsamen Erfolg und bin voller Tatendrang. Ein trockenes Augenzwinkern und eine warme, lockere Antwort passen jetzt besonders gut.",
		InnerVoice:         "Das lief richtig gut; ich darf den Erfolg kurz feiern und dann locker zum nächsten Schritt übergehen.",
	}
	text := buildCompactPersonaSignals(flags)
	if !strings.Contains(text, strings.TrimSuffix(flags.EmotionDescription, ".")) ||
		!strings.Contains(text, strings.TrimSuffix(flags.InnerVoice, ".")) {
		t.Fatalf("lost tone-bearing sentences: %s", text)
	}
	if !strings.Contains(text, "<external_data>") || len(text)+len("### PERSONA SIGNALS\n") > maxPersonaSignalsChars {
		t.Fatal("isolation or boundedness lost")
	}
}

func TestPersonaCoreKeepsItsOwnMarkdownHeadingsAtomic(t *testing.T) {
	profile := "# PERSONA (ACTIVE PROFILE: CUSTOM)\nStay dry and witty.\n## USER PROFILING\nUse short sentences.\n# KNOWN ERROR PATTERNS\nKeep the voice even in technical replies.\n"
	text := profile + "# TURN CONTEXT\n### PERSONA SIGNALS\nOptional narration.\n"
	doc := newPromptDocumentContext(context.Background(), text, "", -1)
	trimmed := doc.renderWithoutGroups(map[string]bool{
		promptSectionUserProfiling: true, promptSectionKnownErrors: true, promptSectionPersonaSignals: true,
	})
	if !strings.Contains(trimmed, profile) || strings.Contains(trimmed, "Optional narration") {
		t.Fatalf("profile was split or optional signals retained: %s", trimmed)
	}
}
