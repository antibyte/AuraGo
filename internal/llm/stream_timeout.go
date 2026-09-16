package llm

import (
	"context"
	"time"
)

type streamAttemptTimeoutKey struct{}

// WithStreamAttemptTimeout gives an isolated workflow its configured call
// deadline without changing the retry timeout of concurrent chats. Parent
// cancellation and deadlines still bound every attempt.
func WithStreamAttemptTimeout(ctx context.Context, timeout time.Duration) context.Context {
	if timeout <= 0 {
		return ctx
	}
	return context.WithValue(ctx, streamAttemptTimeoutKey{}, timeout)
}

func streamAttemptTimeout(ctx context.Context) time.Duration {
	if timeout, ok := ctx.Value(streamAttemptTimeoutKey{}).(time.Duration); ok && timeout > 0 {
		return timeout
	}
	return perAttemptTimeout()
}
