package agent

import (
	"context"
	"testing"

	"aurago/internal/tools"
	"github.com/sashabaranov/go-openai"
)

func TestMailInnerVoiceClearsAtActualTurnCompletion(t *testing.T) {
	for _, tc := range []struct {
		name               string
		trusted, cancelled bool
	}{
		{"completed-mail", true, false}, {"cancelled-mail", true, true}, {"untrusted-mail-prefix", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			session := tools.MailEventSessionID(tc.name)
			run, _, cleanup := newPromptPipelineTestRunConfig(t, session, "web_chat")
			defer cleanup()
			run.SuppressTurnSideEffects = true
			run.LLMClient = &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Role: "assistant", Content: "Done."}}}}}}
			applyInnerVoiceResult(session, "transient event thought", "focus", 0.8)
			applyInnerVoiceResult("unrelated-user-session", "retained user thought", "focus", 0.8)
			defer ResetInnerVoiceSession(session)
			defer ResetInnerVoiceSession("unrelated-user-session")
			ctx := context.Background()
			if tc.trusted {
				ctx = tools.WithMailNotificationPolicy(ctx, t.TempDir(), session, false)
			}
			if tc.cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			_, err := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: run.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "Finish this event."}}}, run, false, NoopBroker{})
			if !tc.cancelled && err != nil {
				t.Fatal(err)
			}
			if tc.cancelled && err == nil {
				t.Fatal("cancelled run unexpectedly succeeded")
			}
			globalInnerVoiceStore.mu.Lock()
			_, exists := globalInnerVoiceStore.states[session]
			_, otherExists := globalInnerVoiceStore.states["unrelated-user-session"]
			globalInnerVoiceStore.mu.Unlock()
			if exists == tc.trusted || !otherExists {
				t.Fatalf("cleanup authority: event retained=%v other retained=%v", exists, otherExists)
			}
		})
	}
}
