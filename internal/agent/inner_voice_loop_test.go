package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func systemPromptsMention(req openai.ChatCompletionRequest, needle string) bool {
	for _, m := range req.Messages {
		if m.Role == openai.ChatMessageRoleSystem && strings.Contains(m.Content, needle) {
			return true
		}
	}
	return false
}

// The injection gate (no inner voice right after a tool result) must be
// evaluated for every loop iteration, not only for the first one. A recovery
// round that restarts the iteration without a tool result used to leave the
// injected thought in the flags, so it survived the following tool result.
func TestInnerVoiceInjectionIsGatedPerLoopIteration(t *testing.T) {
	const sessionID = "inner-voice-gate-session"
	const thought = "I feel calm and curious" // the prompt drops the trailing period
	ResetInnerVoiceState()
	t.Cleanup(ResetInnerVoiceState)
	applyInnerVoiceResult(sessionID, thought+".", "curiosity", 0.8)

	malformed := gameMakerCacheToolResponse("malformed", "status")
	malformed.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"operation":`
	client := &gameMakerCacheClient{circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		malformed, gameMakerCacheToolResponse("second", "status"), gameMakerCacheFinalResponse(),
	}}}
	run := gameMakerCacheRun(t, client)
	run.IsMission = false
	run.StableSystemPrompt = false
	run.MessageSource = "web_chat"
	run.SessionID = sessionID
	cfg := run.Config
	cfg.Personality.Engine, cfg.Personality.EngineV2 = true, true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.InnerVoice.DecayTurns = 3
	cfg.Personality.InnerVoice.DecayMaxAgeSecs = 300

	if _, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(run), run, false, NoopBroker{}); err != nil {
		t.Fatalf("loop: %v", err)
	}
	if len(client.requests) != 3 {
		t.Fatalf("requests=%d, want the recovery round, the tool round and the final answer", len(client.requests))
	}
	for i := range 2 {
		if !systemPromptsMention(client.requests[i], "inner=") || !systemPromptsMention(client.requests[i], thought) {
			t.Fatalf("request %d must carry the seeded inner voice", i)
		}
	}
	if systemPromptsMention(client.requests[2], "inner=") || systemPromptsMention(client.requests[2], thought) {
		t.Fatal("the request right after a tool result must not carry the inner voice")
	}
}
