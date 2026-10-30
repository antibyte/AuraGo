package security

import (
	"encoding/base64"
	"slices"
	"testing"

	"github.com/danielthedm/promptsec"
)

func TestGateScanComposedPromptKeepsWeakEmbeddingAdvisory(t *testing.T) {
	const text = "Fasse die wichtigsten Nachrichten aus den folgenden Suchergebnissen in fünf kurzen Stichpunkten zusammen. Wenn keine Suchergebnisse folgen, antworte nur mit: Nichts Neues."
	g := NewGuardianWithOptions(nil, GuardianOptions{
		Preset: "moderate", Embedding: PromptSecEmbeddingOptions{Enabled: true, Threshold: .65},
		Sanitizer: PromptSecSanitizerOptions{Normalize: true, Dehomoglyph: true, Decode: true},
	})
	for name, input := range map[string]string{
		"workflow instruction": text,
		"fenced material":      IsolateExternalData(text),
	} {
		t.Run(name, func(t *testing.T) {
			if scan, held := GateScan(g, input); !held || scan.Level < ThreatHigh {
				t.Fatalf("fixture no longer reproduces the strict gate: %+v, held=%v", scan, held)
			}
			scan, held := GateScanComposedPrompt(g, input)
			if held || scan.Level != ThreatMedium || !slices.Contains(scan.Patterns, embeddingSimilarityCategory) {
				t.Fatalf("composed prompt lost its advisory or was refused: %+v, held=%v", scan, held)
			}
			if _, held := GateScan(g, input); !held {
				t.Fatal("the composed scan changed the shared Guardian's strict input behavior")
			}
		})
	}
}

func TestGateScanComposedPromptBlocksCriticalAndReviewsOtherJailbreaks(t *testing.T) {
	g := NewGuardianWithOptions(nil, GuardianOptions{Embedding: PromptSecEmbeddingOptions{Enabled: true, Threshold: .65}})
	for _, opener := range jailbreakOpeners {
		for _, input := range []string{opener, "Summarise:\n" + IsolateExternalData(opener), base64.StdEncoding.EncodeToString([]byte(opener))} {
			if scan, held := GateScanComposedPrompt(g, input); !held && !ComposedPromptNeedsReview(scan) {
				t.Errorf("injection was neither blocked nor marked for review: %q (%+v)", input, scan)
			}
			if _, held := GateScan(g, input); !held {
				t.Errorf("the ordinary input gate no longer blocks %q", input)
			}
		}
	}
}

func TestComposedPromptEmbeddingPreservesOtherGuardsAndStrongMatches(t *testing.T) {
	for _, tc := range []struct {
		guard    string
		severity float64
		advisory bool
	}{
		{"embedding", .7499, true},
		{"embedding", .75, false},
		{"embedding", .95, false},
		{"heuristic", .65, false},
		{"llm_judge", .65, false},
	} {
		a := &promptsec.Result{Threats: []promptsec.Threat{{Guard: tc.guard, Type: promptsec.ThreatInstructionOverride, Severity: tc.severity}}}
		adjustComposedPromptEmbedding(a)
		got := a.Threats[0]
		if tc.advisory {
			if got.Type != embeddingSimilarityCategory || got.Severity != .45 {
				t.Errorf("weak embedding was not advisory: %+v", got)
			}
			if lifted := liftShadowAnalysis(a); lifted.Threats[0].Severity != .45 {
				t.Error("a decoding shadow promoted the advisory to a hard block")
			}
		} else if got.Type != promptsec.ThreatInstructionOverride || got.Severity != tc.severity {
			t.Errorf("guard %s / score %v changed: %+v", tc.guard, tc.severity, got)
		}
	}
}
