package agent

import (
	"aurago/internal/config"
	"aurago/internal/tools"
	"context"
	"encoding/json"
	"log/slog"

	openai "github.com/sashabaranov/go-openai"
)

// ExecutionHooks are trusted server callbacks, never model-supplied policy.
// Nil preserves the ordinary agent's dispatch and accounting behavior.
type ExecutionHooks struct {
	OnAcquire       func(context.Context) (context.Context, context.CancelFunc, error)
	BeforeIteration func(context.Context) error
	BeforeRequest   func(context.Context, *openai.ChatCompletionRequest, int) error
	BeforeTool      func(context.Context, ToolCall) error
	HandleTool      func(context.Context, ToolCall) (string, bool)
	AfterTool       func(context.Context, ToolCall, string) string
	AfterResponse   func(openai.Usage) error
}

func executionHookError(err error) string {
	b, _ := json.Marshal(map[string]string{"status": "error", "message": err.Error()})
	return "Tool Output: " + string(b)
}

// ConfiguredToolSchemas returns the same capability-gated catalog used by chat.
func ConfiguredToolSchemas(cfg *config.Config, logger *slog.Logger) []openai.Tool {
	return BuildNativeToolSchemas(cfg.Directories.SkillsDir, tools.NewManifest(cfg.Directories.ToolsDir), buildToolFlagsFromConfig(cfg), logger)
}
