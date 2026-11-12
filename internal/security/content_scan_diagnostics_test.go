package security

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/sashabaranov/go-openai"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestContentScanReportsFixedFailureCodesWithoutContent(t *testing.T) {
	const marker = "PRIVATE_MAIL_CONTENT_MUST_NOT_BE_LOGGED"
	for _, tc := range []struct {
		name, input, verdict, code string
		finish                     openai.FinishReason
	}{
		{"oversized", strings.Repeat("x", 29185), "safe 0 ordinary", "content_scan_input_limit", openai.FinishReasonStop},
		{"invalid_utf8", marker + "\xff", "safe 0 ordinary", "content_scan_invalid_utf8", openai.FinishReasonStop},
		{"malformed", marker, "safe", "content_scan_invalid_verdict", openai.FinishReasonStop},
		{"truncated", marker, "safe 0 ordinary", "content_scan_output_limit", openai.FinishReasonLength},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			g := contentScanTestGuardian(t, func(w http.ResponseWriter, r *http.Request) { writeContentScanVerdict(w, tc.verdict, tc.finish) })
			g.logger = slog.New(slog.NewTextHandler(&logs, nil))
			result, err := g.EvaluateContentStrict(context.Background(), "email", tc.input)
			if err == nil || err.Error() != tc.code || result.Decision != DecisionQuarantine {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !strings.Contains(logs.String(), tc.code) || strings.Contains(logs.String(), marker) {
				t.Fatalf("unsafe or missing diagnostic: %s", logs.String())
			}
			if g.cache.Size() != 0 {
				t.Fatal("incomplete verdict cached")
			}
		})
	}
}

func TestContentScanCompletionBudgetIncludesReasoning(t *testing.T) {
	g := contentScanTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.MaxTokens < 4096 || req.MaxTokens > 8192 {
			writeContentScanVerdict(w, "", openai.FinishReasonLength)
			return
		}
		writeContentScanVerdict(w, "safe 0 ordinary mail", openai.FinishReasonStop)
	})
	result, err := g.EvaluateContentStrict(context.Background(), "email", "fixture mail")
	if err != nil || result.Decision != DecisionAllow {
		t.Fatalf("reasoning budget truncated verdict: %+v %v", result, err)
	}
}
