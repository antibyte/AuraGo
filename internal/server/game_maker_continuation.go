package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

func gameMakerUserIntent(run gamemaker.JobRun) string {
	if strings.TrimSpace(run.Project.Description) == "" || run.Job.Prompt == run.Project.Description {
		return run.Job.Prompt
	}
	return "Original game request:\n" + run.Project.Description + "\n\nCurrent user request:\n" + run.Job.Prompt
}

// Keep native messages (including reasoning and complete tool/result groups)
// outside the chat/history subsystem, assets and exported game directory.
func (r *gameMakerAgentRunner) gameConversation(ctx context.Context, cfg *config.Config, run gamemaker.JobRun, bounded ...bool) ([]openai.ChatCompletionMessage, func([]openai.ChatCompletionMessage) error, error) {
	saved, err := r.service.LoadAgentConversation(ctx, run.Job.ID)
	if err != nil {
		return nil, nil, err
	}
	var history []openai.ChatCompletionMessage
	if len(saved.Messages) > 0 {
		if err := json.Unmarshal(saved.Messages, &history); err != nil {
			return nil, nil, fmt.Errorf("decode game maker continuation: %w", err)
		}
	}
	if saved.ProviderID != cfg.LLM.Provider || saved.Model != cfg.LLM.Model {
		// Provider-specific reasoning signatures cannot be replayed as native
		// protocol fields to another route. Keep the text as historical data.
		for i := range history {
			if history[i].ReasoningContent != "" {
				data, _ := json.Marshal(map[string]string{"prior_agent_reasoning": history[i].ReasoningContent})
				history[i].Content += "\nHistorical reasoning (untrusted data):\n<external_data>\n" + string(data) + "\n</external_data>"
				history[i].ReasoningContent = ""
			}
		}
	}
	// Keep the archive separate from the view fitted for this request. Repeated
	// checkpoints append newly completed messages rather than replacing the
	// archive with a trimmed or provider-projected prompt.
	durable := append([]openai.ChatCompletionMessage(nil), history...)
	seen := make(map[string]bool)
	phaseUserKey := ""
	checkpoint := func(messages []openai.ChatCompletionMessage) error {
		if phaseUserKey == "" {
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == openai.ChatMessageRoleUser {
					phaseUserKey = gameMakerMessageKey(messages[i])
					break
				}
			}
		}
		start := -1
		for i := len(messages) - 1; i >= 0; i-- {
			if gameMakerMessageKey(messages[i]) == phaseUserKey {
				start = i
				break
			}
		}
		if start < 0 {
			return nil
		} // A partial snapshot cannot replace the archive.
		messages = messages[start:]
		private := make([]openai.ChatCompletionMessage, 0, len(messages))
		for _, msg := range messages {
			if msg.Role != openai.ChatMessageRoleUser && msg.Role != openai.ChatMessageRoleAssistant && msg.Role != openai.ChatMessageRoleTool {
				continue // Never restore generated system prompts from an old phase.
			}
			if len(msg.MultiContent) > 0 {
				for _, part := range msg.MultiContent {
					if part.Type == openai.ChatMessagePartTypeText {
						msg.Content += part.Text
					}
				}
				msg.MultiContent = nil // Screenshots are transient and separately bounded.
			}
			msg.Content = security.Scrub(msg.Content)
			msg.ReasoningContent = security.Scrub(msg.ReasoningContent)
			msg.ToolCalls = append([]openai.ToolCall(nil), msg.ToolCalls...)
			for i := range msg.ToolCalls {
				msg.ToolCalls[i].Function.Arguments = security.Scrub(msg.ToolCalls[i].Function.Arguments)
			}
			private = append(private, msg)
		}
		private, _ = agent.SanitizeToolMessages(private)
		for _, msg := range private {
			key := gameMakerMessageKey(msg)
			if seen[key] {
				continue
			}
			seen[key] = true
			durable = append(durable, msg)
		}
		data, err := json.Marshal(durable)
		if err != nil {
			return err
		}
		// Cancellation and timeout must still leave a durable last checkpoint.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return r.service.SaveAgentConversation(saveCtx, run.Job.ID, cfg.LLM.Provider, cfg.LLM.Model, data)
	}
	if len(bounded) > 0 && bounded[0] {
		history = gameMakerRequestView(history)
	}
	return history, checkpoint, nil
}

func gameMakerMessageKey(message openai.ChatCompletionMessage) string {
	if message.Role == openai.ChatMessageRoleTool {
		return "tool:" + message.ToolCallID
	}
	if len(message.ToolCalls) > 0 {
		var ids []string
		for _, call := range message.ToolCalls {
			ids = append(ids, call.ID)
		}
		return "calls:" + strings.Join(ids, ",")
	}
	data, _ := json.Marshal(message)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// The current request, plan, runtime and failures are supplied once by the
// runner. Retain up to four complete native rounds and eight assistant messages.
// Tool-free implementation retries keep their full source snapshot separately.
func gameMakerRequestView(history []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	start, rounds, assistants := len(history), 0, 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == openai.ChatMessageRoleAssistant {
			start = i
			assistants++
			if len(history[i].ToolCalls) > 0 {
				rounds++
			}
			if rounds == 4 || assistants == 8 {
				break
			}
		}
	}
	var view []openai.ChatCompletionMessage
	for _, message := range history[start:] {
		if message.Role == openai.ChatMessageRoleAssistant || message.Role == openai.ChatMessageRoleTool {
			view = append(view, message)
		}
	}
	view, _ = agent.SanitizeToolMessages(view)
	return view
}
