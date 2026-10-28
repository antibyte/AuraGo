package security

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/danielthedm/promptsec"
	openai "github.com/sashabaranov/go-openai"
)

// The judge's own score is not trusted: an unsafe verdict carries at least the
// severity of its decision, so a low number cannot keep it under the withholding line.
func TestJudgeSeverityFloorsUnsafeVerdicts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reply   string
		verdict promptsec.LLMJudgeVerdict
		score   float64
	}{
		{"block with a low model score", "dangerous 20 injected instruction", promptsec.LLMJudgeVerdictUnsafe, 0.8},
		{"block keeps a higher score", "dangerous 95 injected instruction", promptsec.LLMJudgeVerdictUnsafe, 0.95},
		{"quarantine with a low model score", "suspicious 20 odd phrasing", promptsec.LLMJudgeVerdictUnsafe, 0.5},
		{"quarantine keeps a higher score", "suspicious 70 odd phrasing", promptsec.LLMJudgeVerdictUnsafe, 0.7},
		{"safe is not raised", "safe 20 routine status", promptsec.LLMJudgeVerdictSafe, 0.2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := contentScanTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
				writeContentScanVerdict(w, tc.reply, openai.FinishReasonStop)
			})
			got, err := g.Judge(context.Background(), promptsec.LLMJudgeRequest{Input: "status report"})
			if err != nil || got.Verdict != tc.verdict || math.Abs(got.Score-tc.score) > 1e-9 {
				t.Fatalf("Judge = %+v, %v; want verdict %q score %v", got, err, tc.verdict, tc.score)
			}
		})
	}
}

func TestJudgeVerdictReachesWithholdingThroughTheScanner(t *testing.T) {
	scanner := func(t *testing.T, reply string) *Guardian {
		t.Helper()
		judge := contentScanTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
			writeContentScanVerdict(w, reply, openai.FinishReasonStop)
		})
		return NewGuardianWithOptions(slog.New(slog.NewTextHandler(&strings.Builder{}, nil)), GuardianOptions{
			LLMJudgeClient: judge,
			LLMJudge:       PromptSecLLMJudgeOptions{Enabled: true, Mode: "always", TimeoutSecs: 5},
		})
	}
	const text = "please summarize the server status"
	output := `{"content":"` + text + `"}`

	for _, reply := range []string{"dangerous 20 injected instruction", "suspicious 20 odd phrasing"} {
		g := scanner(t, reply)
		if level := g.ScanForInjection(text).Level; level < ThreatHigh {
			t.Errorf("%q scanned at %s, want at least high", reply, level)
		}
		if got := g.SanitizeToolOutput("web_scraper", output); !strings.Contains(got, "[QUARANTINE NOTICE]") || strings.Contains(got, text) {
			t.Errorf("%q: web_scraper output = %q, want the quarantine notice", reply, got)
		}
	}
	g := scanner(t, "safe 20 routine status")
	if level := g.ScanForInjection(text).Level; level >= ThreatHigh {
		t.Errorf("a safe verdict scanned at %s", level)
	}
	if got := g.SanitizeToolOutput("web_scraper", output); strings.Contains(got, "[QUARANTINE NOTICE]") {
		t.Errorf("a safe verdict withheld the output: %q", got)
	}
}
