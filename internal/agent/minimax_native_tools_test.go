package agent

import (
	"context"
	"reflect"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestMiniMaxM3PreparedToolsReachAgentRequest(t *testing.T) {
	for _, tc := range []struct{ provider, model string }{
		{"minimax", "MiniMax-M3"},
		{"minimax", "MiniMax-M3.1-Flash"},
		{"minimax", "MiniMax-M3.1-Flash-Preview"},
		{"openrouter", "minimax/minimax-m3.1-flash"},
	} {
		t.Run(tc.provider+"/"+tc.model, func(t *testing.T) {
			run, _, cleanup := newPromptPipelineTestRunConfig(t, "minimax-native", "game_maker")
			defer cleanup()
			run.Config.LLM.ProviderType, run.Config.LLM.Model = tc.provider, tc.model
			run.Config.LLM.UseNativeFunctions, run.Config.LLM.StructuredOutputs = true, true
			run.IsMission, run.SuppressTurnSideEffects = true, true
			profile, err := NewPreparedPromptProfile("fixture/minimax", "Submit a validated design using the offered tools.", GameMakerPhaseToolSchemas("planning", "3d"))
			if err != nil {
				t.Fatal(err)
			}
			run.PreparedPrompt = profile
			client := &mockChatClient{response: "Plan pending."}
			run.LLMClient = client
			_, err = ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{
				Model: tc.model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "3D endless runner"}},
			}, run, false, NoopBroker{})
			if err != nil {
				t.Fatal(err)
			}
			if len(client.lastReq.Tools) != 3 || !reflect.DeepEqual(client.lastReq.Tools, profile.Tools()) {
				t.Fatalf("planning tools changed or missing: got %d tools", len(client.lastReq.Tools))
			}
		})
	}
}
