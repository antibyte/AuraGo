package flows

import (
	"context"
	"time"
)

// Clock abstracts time for the engine, waits and timers.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// RealClock returns the wall clock.
func RealClock() Clock { return realClock{} }

// ToolInvoker runs one AuraGo tool outside the LLM loop (implemented in internal/server).
type ToolInvoker interface {
	InvokeTool(ctx context.Context, req ToolRequest) (ToolResponse, error)
}

// ToolRequest is one tool call made by a flow node.
type ToolRequest struct {
	FlowID       string
	RunID        string
	NodeID       string
	Mode         RunMode
	Tool         string
	Args         map[string]any
	AllowedTools []string
}

// ToolResponse is the raw tool result.
type ToolResponse struct {
	Output  string
	IsError bool
	Status  string
}

// LLMStepper performs one LLM completion for AI nodes (implemented in internal/server).
type LLMStepper interface {
	Step(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

// LLMRequest is one AI step. When JSONSchema is set the response must be a JSON object.
type LLMRequest struct {
	FlowID     string
	RunID      string
	NodeID     string
	System     string
	Prompt     string
	Model      string
	JSONSchema map[string]any
	MaxTokens  int
}

// LLMResponse is the result of an AI step.
type LLMResponse struct {
	Text         string
	JSON         map[string]any
	Model        string
	InputTokens  int
	OutputTokens int
}

// SecretReader resolves vault references of secret_ref parameters (implemented in internal/server).
// Implementations must register returned values with the output scrubber.
type SecretReader interface {
	ReadSecret(key string) (string, error)
}

// Services bundles what nodes may use. Any field may be nil; helpers fall back to defaults.
type Services struct {
	Tools    ToolInvoker
	LLM      LLMStepper
	Secrets  SecretReader
	Clock    Clock
	Location *time.Location
}

func (s *Services) clock() Clock {
	if s == nil || s.Clock == nil {
		return realClock{}
	}
	return s.Clock
}

// Now returns the current time of the configured clock.
func (s *Services) Now() time.Time { return s.clock().Now() }

// Loc returns the configured location or time.Local.
func (s *Services) Loc() *time.Location {
	if s != nil && s.Location != nil {
		return s.Location
	}
	return time.Local
}

// Sleep waits for d or until ctx is done.
func (s *Services) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.clock().After(d):
		return nil
	}
}
