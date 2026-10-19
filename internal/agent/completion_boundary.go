package agent

import (
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// IncompleteCompletionError preserves provider finish diagnostics without making
// partial text or tool arguments executable. It carries no provider content.
type IncompleteCompletionError struct {
	FinishReason  openai.FinishReason
	ReasoningOnly bool
	HasToolCalls  bool
}

func (e *IncompleteCompletionError) Error() string {
	return fmt.Sprintf("model response incomplete (%s); output was not applied", e.FinishReason)
}

func incompleteCompletion(reason openai.FinishReason, content, reasoning string, hasToolCalls bool) error {
	return &IncompleteCompletionError{
		FinishReason: reason, HasToolCalls: hasToolCalls,
		ReasoningOnly: !hasToolCalls && strings.TrimSpace(content) == "" && reasoning != "",
	}
}
