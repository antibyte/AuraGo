package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"

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

// A turn that ends inside an error streak reports its real error count to the
// helper batch and neither reserves nor requests an inner voice.
func TestTurnEndingInAnErrorStreakReportsErrorsAndRequestsNoInnerVoice(t *testing.T) {
	const sessionID = "inner-voice-error-streak-session"
	const text = "Please stop process 1."
	const helperResponse = `{"memory_analysis":{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]},"activity_digest":{"intent":"Stop","user_goal":"Stop a process","actions_taken":["Tried"],"outcomes":["Failed"],"important_points":[],"pending_items":[],"importance":1,"entities":[]},"personality_analysis":{"mood_analysis":{"user_sentiment":"neutral","agent_appropriate_response_mood":"focused","relationship_delta":0.0,"trait_deltas":{},"user_profile_updates":[]},"emotion_state":{"description":"I feel steady.","primary_mood":"focused","secondary_mood":"","valence":0.1,"arousal":0.3,"confidence":0.8,"cause":"a failed tool call","recommended_response_style":"calm"}}}`

	oldFailures, oldPause := v2FailCount.Swap(0), v2PausedUntil.Swap(0)
	ResetGlobalHelperLLMManager()
	ResetInnerVoiceState()
	t.Cleanup(func() {
		v2FailCount.Store(oldFailures)
		v2PausedUntil.Store(oldPause)
		ResetGlobalHelperLLMManager()
		ResetInnerVoiceState()
	})

	failing := gameMakerCacheToolResponse("failing", "status")
	failing.Choices[0].Message.ToolCalls[0].Function = openai.FunctionCall{Name: "stop_process", Arguments: `{"pid":1}`}
	mainClient := &gameMakerCacheClient{circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{failing, gameMakerCacheFinalResponse()}}}
	run, _, cleanup := newPromptPipelineTestRunConfig(t, sessionID, "web_chat")
	t.Cleanup(cleanup)
	run.LLMClient = mainClient
	run.AllowedTools = []string{"stop_process"} // disabled in the test config: calling it is a tool error
	run.NativeToolSchemas = []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: "stop_process", Description: "Stop a managed process.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"pid": map[string]any{"type": "integer"}}},
	}}}
	cfg := run.Config
	cfg.LLM.UseNativeFunctions = true
	cfg.Personality.Engine, cfg.Personality.EngineV2 = true, true
	cfg.Personality.CorePersonality = "punk"
	cfg.Personality.EmotionSynthesizer.Enabled = true
	cfg.Personality.EmotionSynthesizer.MinIntervalSecs = 60
	cfg.Personality.EmotionSynthesizer.TriggerOnMoodChange = true
	cfg.Personality.InnerVoice.Enabled = true
	cfg.Personality.InnerVoice.MinIntervalSecs = 60
	cfg.Personality.InnerVoice.MaxPerSession = 20
	cfg.Personality.InnerVoice.DecayTurns = 3
	cfg.Personality.InnerVoice.DecayMaxAgeSecs = 300
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "openai"
	cfg.LLM.HelperBaseURL = "https://helper.invalid/v1"
	cfg.LLM.HelperAPIKey = "k"
	cfg.LLM.HelperResolvedModel = "helper-model"
	if cfg.Budget.Enabled || run.BudgetTracker != nil {
		t.Fatal("fixture must model disabled budgeting: no tracker")
	}

	helper := &recordingHelperClient{response: helperResponse}
	manager := newHelperLLMManager(cfg, run.Logger)
	if manager == nil {
		t.Fatal("helper manager unavailable")
	}
	manager.client = helper

	sideEffects := NewAsyncTaskGroup(context.Background())
	t.Cleanup(func() { _ = sideEffects.Shutdown(5 * time.Second) })
	run.SideEffects = sideEffects
	run.HumanInput = memory.NewHumanInput(text, sessionID)
	request := openai.ChatCompletionRequest{Model: cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: text}}}
	if _, err := ExecuteAgentLoop(context.Background(), request, run, false, NoopBroker{}); err != nil {
		t.Fatal(err)
	}
	if len(mainClient.requests) != 2 {
		t.Fatalf("main requests=%d, want the failing tool round and the final answer", len(mainClient.requests))
	}

	// Do not Shutdown here: it cancels the group context of the running analysis.
	var requests []openai.ChatCompletionRequest
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if requests = helper.recorded(); len(requests) > 0 {
			break
		}
	}
	if len(requests) != 1 {
		t.Fatalf("helper requests=%d, want the single batched turn request", len(requests))
	}
	var prompt strings.Builder
	for _, message := range requests[0].Messages {
		prompt.WriteString(message.Content)
		prompt.WriteString("\n")
	}
	if !strings.Contains(prompt.String(), "Errors: 1") {
		t.Fatalf("batched turn request must report the real error count:\n%s", prompt.String())
	}
	if strings.Contains(prompt.String(), "INNER VOICE REQUESTED") {
		t.Fatal("a turn that ends inside an error streak must not request an inner voice")
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
