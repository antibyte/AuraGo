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

// stanceHintInTrustedPersonaState reports whether the system prompt carries the
// hint inside the "### PERSONA STATE" section, which holds no <external_data>.
func stanceHintInTrustedPersonaState(req openai.ChatCompletionRequest, hint string) bool {
	for _, m := range req.Messages {
		if m.Role != openai.ChatMessageRoleSystem {
			continue
		}
		_, section, found := strings.Cut(m.Content, "### PERSONA STATE\n")
		if !found {
			continue
		}
		section, _, _ = strings.Cut(section, "\n\n#") // up to the next section heading
		if strings.Contains(section, hint) && !strings.Contains(section, "<external_data>") {
			return true
		}
	}
	return false
}

const innerVoiceTestThought = "I feel calm and curious" // the prompt drops the trailing period

// runInnerVoiceLoop seeds a "caution" thought and runs the loop over the scripted
// model responses. It returns the requests the model saw and the stance hint.
func runInnerVoiceLoop(t *testing.T, sessionID string, responses ...openai.ChatCompletionResponse) ([]openai.ChatCompletionRequest, string) {
	t.Helper()
	hint := innerVoiceStanceHint("caution")
	if hint == "" {
		t.Fatal("caution must select a stance hint")
	}
	ResetInnerVoiceState()
	t.Cleanup(ResetInnerVoiceState)
	applyInnerVoiceResult(sessionID, innerVoiceTestThought+".", "caution", 0.8)

	client := &gameMakerCacheClient{circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: responses}}
	run := gameMakerCacheRun(t, client)
	run.IsMission = false
	run.StableSystemPrompt = false
	run.MessageSource = "web_chat"
	run.SessionID = sessionID
	// stop_process is disabled in the test config: calling it is a tool error.
	run.AllowedTools = append(run.AllowedTools, "stop_process")
	run.NativeToolSchemas = append(run.NativeToolSchemas, openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: "stop_process", Description: "Stop a managed process.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"pid": map[string]any{"type": "integer"}}},
	}})
	cfg := run.Config
	cfg.Personality.Engine, cfg.Personality.EngineV2 = true, true
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.InnerVoice.DecayTurns = 3
	cfg.Personality.InnerVoice.DecayMaxAgeSecs = 300

	if _, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(run), run, false, NoopBroker{}); err != nil {
		t.Fatalf("loop: %v", err)
	}
	return client.requests, hint
}

func requireInnerVoiceCarried(t *testing.T, name string, req openai.ChatCompletionRequest, hint string) {
	t.Helper()
	if !systemPromptsMention(req, "inner=") || !systemPromptsMention(req, innerVoiceTestThought) {
		t.Fatalf("%s must carry the seeded inner voice", name)
	}
	if !stanceHintInTrustedPersonaState(req, hint) {
		t.Fatalf("%s must carry the stance hint in the trusted persona state, outside external data", name)
	}
}

// The injection gate is evaluated for every loop iteration. Successful tool
// rounds keep the thought and its stance hint, so the final answer of a tool
// turn sees them.
func TestInnerVoiceInjectionIsGatedPerLoopIteration(t *testing.T) {
	malformed := gameMakerCacheToolResponse("malformed", "status")
	malformed.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"operation":`
	requests, hint := runInnerVoiceLoop(t, "inner-voice-gate-session",
		malformed, gameMakerCacheToolResponse("second", "status"), gameMakerCacheFinalResponse())
	if len(requests) != 3 {
		t.Fatalf("requests=%d, want the recovery round, the tool round and the final answer", len(requests))
	}
	for i, name := range []string{"the first request", "the request after the recovery round", "the request after a successful tool result"} {
		requireInnerVoiceCarried(t, name, requests[i], hint)
	}
}

// During active tool-error recovery (consecutive errors > 0) the next prompt
// stays procedural: no thought and no stance hint.
func TestInnerVoiceIsWithheldAfterAToolError(t *testing.T) {
	failing := gameMakerCacheToolResponse("failing", "status")
	failing.Choices[0].Message.ToolCalls[0].Function = openai.FunctionCall{Name: "stop_process", Arguments: `{"pid":1}`}
	requests, hint := runInnerVoiceLoop(t, "inner-voice-error-session", failing, gameMakerCacheFinalResponse())
	if len(requests) != 2 {
		t.Fatalf("requests=%d, want the failing tool round and the final answer", len(requests))
	}
	requireInnerVoiceCarried(t, "the first request", requests[0], hint)
	if systemPromptsMention(requests[1], "inner=") || systemPromptsMention(requests[1], innerVoiceTestThought) || systemPromptsMention(requests[1], hint) {
		t.Fatal("the request after a failed tool call must carry neither the inner voice nor the stance hint")
	}
}
