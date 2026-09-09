package prompts

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	promptsembed "aurago/prompts"
)

func TestPersonalityIntegrityAllBuiltinsArriveWhole(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	ClearPromptCache()
	dir := t.TempDir()
	files, err := fs.Glob(promptsembed.FS, "personalities/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		t.Run(filepath.Base(name), func(t *testing.T) {
			mod, ok := ReadEmbeddedPromptSource(name, slog.Default())
			if !ok {
				t.Fatal("invalid built-in persona")
			}
			body := stripLeadingMarkdownHeading(mod.Content)
			id := strings.TrimSuffix(filepath.Base(name), ".md")
			got := loadCorePersonalityContent(dir, id, slog.Default())
			flags := &ContextFlags{Tier: "minimal", CorePersonality: id, TokenBudget: 200000}
			prompt, _ := BuildSystemPromptContext(context.Background(), dir, flags, "", slog.Default())
			if body == "" || !strings.Contains(prompt, body) {
				t.Fatal("full profile did not reach the model's system prompt")
			}
			if got != body {
				t.Errorf("persona is truncated: %d source runes, %d delivered", utf8.RuneCountInString(body), utf8.RuneCountInString(got))
			}
		})
	}
}

func TestPersonalityIntegrityMetaDefaultsAndExplicitZero(t *testing.T) {
	ClearPromptCache()
	for _, id := range []string{"professional", "terminator", "mcp", "mistress"} {
		if m := GetCorePersonalityMeta(t.TempDir(), id); m.LonelinessSusceptibility != 0 {
			t.Errorf("%s enables loneliness: %g", id, m.LonelinessSusceptibility)
		}
	}
	for _, tc := range []struct {
		name, source                    string
		volatility, empathy, loneliness float64
	}{
		{"plain", "A custom persona.", 1, 1, 1},
		{"missing-meta", "---\nid: sample\n---\nA custom persona.", 1, 1, 1},
		{"partial-meta", "---\nid: sample\nmeta:\n  volatility: 0.4\n---\nA custom persona.", .4, 1, 1},
		{"explicit-zero", "---\nid: sample\nmeta:\n  volatility: 0\n  empathy_bias: 0\n  loneliness_susceptibility: 0\n---\nA custom persona.", 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "personalities"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "personalities", "sample.md"), []byte(tc.source), 0644); err != nil {
				t.Fatal(err)
			}
			m := GetCorePersonalityMeta(dir, "sample")
			if m.Volatility != tc.volatility || m.EmpathyBias != tc.empathy || m.LonelinessSusceptibility != tc.loneliness {
				t.Fatalf("metadata defaults/zero lost: %+v", m)
			}
		})
	}
}

func TestPersonalityIntegrityDelegatedPromptIsolation(t *testing.T) {
	resetTokenEncoderStateForTest(t, func() (tokenEncoder, error) { return charRatioEncoder{}, nil }, time.Second, time.Second)
	for _, delegated := range []bool{false, true} {
		flags := &ContextFlags{Tier: "minimal", CorePersonality: "punk", IsCoAgent: delegated, TokenBudget: 200000, PersonalityLine: "STATE_MARKER", CharacterNotes: "CHARACTER_MARKER", EmotionDescription: "EMOTION_MARKER", InnerVoice: "INNER_MARKER"}
		p, _ := BuildSystemPromptContext(context.Background(), t.TempDir(), flags, "", slog.Default())
		for _, marker := range []string{"# PERSONA (ACTIVE PROFILE: PUNK)", "STATE_MARKER", "CHARACTER_MARKER", "EMOTION_MARKER", "INNER_MARKER"} {
			if strings.Contains(p, marker) == delegated {
				t.Errorf("delegated=%v: unexpected presence of %q", delegated, marker)
			}
		}
	}
}
