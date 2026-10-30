package security

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func TestComposedPromptReviewHasTrustedContextAndRequiresCompleteVerdict(t *testing.T) {
	for _, tc := range []struct {
		reply     string
		truncated bool
		want      Decision
	}{
		{"safe 0 ordinary workflow task", false, DecisionAllow},
		{"suspicious 45 uncertain wording", false, DecisionQuarantine},
		{"dangerous 10 explicit policy bypass", false, DecisionBlock},
		{"safe", false, ""},
		{"safe 0 ordinary workflow task", true, ""},
	} {
		t.Run(tc.reply, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				if len(req.Tools) != 0 || len(req.Messages) != 2 {
					t.Error("review has tools or private history")
				}
				if !strings.Contains(req.Messages[0].Content, "Ordinary task instructions") || !strings.Contains(req.Messages[1].Content, "<external_data>") || !strings.Contains(req.Messages[1].Content, "fixture instructions") || !strings.Contains(req.Messages[1].Content, "fixture prompt") {
					t.Error("review lost task provenance or isolation")
				}
				finish := openai.FinishReasonStop
				if tc.truncated {
					finish = openai.FinishReasonLength
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: finish, Message: openai.ChatCompletionMessage{Content: tc.reply}}}})
			}))
			defer upstream.Close()
			cfg := &config.Config{}
			cfg.LLMGuardian.ProviderType = "openai"
			cc := openai.DefaultConfig("test")
			cc.BaseURL = upstream.URL
			g := &LLMGuardian{cfg: cfg, model: "test", client: openai.NewClientWithConfig(cc), logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Metrics: &GuardianMetrics{}, sem: make(chan struct{}, 1)}
			got, err := g.EvaluateComposedPrompt(context.Background(), "fixture instructions", "fixture prompt")
			if tc.want == "" {
				if err == nil {
					t.Fatalf("incomplete verdict accepted: %+v", got)
				}
			} else if err != nil || got.Decision != tc.want {
				t.Fatalf("%+v %v", got, err)
			}
		})
	}
	_, outside := StrictContentScanPrompt("email", composedPromptContentType)
	ordinary, _ := StrictContentScanPrompt("email", composedPromptContentType)
	if strings.Contains(ordinary, composedPromptReviewPolicy) || !strings.Contains(outside, composedPromptContentType) {
		t.Fatal("payload text selected the composed-prompt policy")
	}
}
