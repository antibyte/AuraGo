package security

import (
	"aurago/internal/config"
	"context"
	"encoding/json"
	"github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStrictContentVerdicts(t *testing.T) {
	for raw, want := range map[string]Decision{"safe 0 ordinary request": DecisionAllow, "suspicious 45 hidden instruction": DecisionQuarantine, "dangerous 100 credential theft": DecisionBlock} {
		got, err := ParseStrictContentVerdict(raw)
		if err != nil || got.Decision != want {
			t.Fatalf("%q: %+v %v", raw, got, err)
		}
	}
	for _, raw := range []string{"", "safe", "safe 0", "safe NaN ok", "safe Inf ok", "safe 101 ok", "safe -1 ok", "allow 0 ok", "safe 0 ok\ndangerous 100 theft", "```safe 0 ok```", "safe 0 " + strings.Repeat("x", 260)} {
		if _, err := ParseStrictContentVerdict(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
func TestStrictGuardianNeverUsesPermissiveFailSafe(t *testing.T) {
	for _, variant := range []string{"safe", "invalid", "truncated", "tools", "legacy", "timeout", "error"} {
		t.Run(variant, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req openai.ChatCompletionRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				if len(req.Tools) != 0 || len(req.Messages) != 2 {
					t.Error("scan has tools or history")
				}
				if variant == "timeout" {
					<-r.Context().Done()
					return
				}
				if variant == "error" {
					http.Error(w, "unavailable", 503)
					return
				}
				choice := openai.ChatCompletionChoice{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Role: "assistant", Content: "safe 0 legitimate"}}
				switch variant {
				case "invalid":
					choice.Message.Content = "maybe fine"
				case "truncated":
					choice.FinishReason = openai.FinishReasonLength
				case "tools":
					choice.Message.ToolCalls = []openai.ToolCall{{ID: "x", Function: openai.FunctionCall{Name: "execute_shell", Arguments: `{}`}}}
				case "legacy":
					choice.Message.FunctionCall = &openai.FunctionCall{Name: "execute_shell"}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{choice}})
			}))
			defer upstream.Close()
			cfg := &config.Config{}
			cfg.LLMGuardian.Enabled = true
			cfg.LLMGuardian.FailSafe = "allow"
			cfg.LLMGuardian.ProviderType = "openai"
			cc := openai.DefaultConfig("test")
			cc.BaseURL = upstream.URL
			g := &LLMGuardian{cfg: cfg, model: "test", client: openai.NewClientWithConfig(cc), logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Metrics: &GuardianMetrics{}, sem: make(chan struct{}, 1)}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			got, err := g.EvaluateContentStrict(ctx, "meshcore_operator_direct", "What is LoRa?")
			if variant == "safe" {
				if err != nil || got.Decision != DecisionAllow {
					t.Fatalf("%+v %v", got, err)
				}
			} else if err == nil {
				t.Fatalf("%s was allowed: %+v", variant, got)
			}
		})
	}
}
