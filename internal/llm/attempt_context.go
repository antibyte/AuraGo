package llm

import (
	"context"
	"errors"
)

type attemptParentKey struct{}

// Only the retry owner knows whether a deadline belongs to an attempt or the
// user. Carry that identity through the transport instead of guessing by text.
func withAttemptParent(attempt, parent context.Context) context.Context {
	return context.WithValue(attempt, attemptParentKey{}, parent)
}
func providerOutcomeError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	parent, _ := ctx.Value(attemptParentKey{}).(context.Context)
	if parent != nil && parent.Err() == nil && isOwnedTransportTimeout(context.Cause(ctx)) {
		return context.Cause(ctx)
	}
	if parent != nil && parent.Err() != nil {
		return nil
	}
	if parent != nil && parent.Err() == nil && errors.Is(ctx.Err(), context.DeadlineExceeded) && IsContextError(err) {
		return &LLMError{Category: ErrCategoryTemporaryTransport, Message: "provider attempt timed out"}
	}
	if ctx.Err() != nil {
		return nil
	}
	return err
}
