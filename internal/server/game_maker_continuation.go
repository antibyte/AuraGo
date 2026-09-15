package server

import (
	"context"
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
func (r *gameMakerAgentRunner) gameConversation(ctx context.Context, cfg *config.Config, run gamemaker.JobRun) ([]openai.ChatCompletionMessage, func([]openai.ChatCompletionMessage) error, error) {
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
	checkpoint := func(messages []openai.ChatCompletionMessage) error {
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
		data, err := json.Marshal(private)
		if err != nil {
			return err
		}
		// Cancellation and timeout must still leave a durable last checkpoint.
		saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return r.service.SaveAgentConversation(saveCtx, run.Job.ID, cfg.LLM.Provider, cfg.LLM.Model, data)
	}
	return history, checkpoint, nil
}
