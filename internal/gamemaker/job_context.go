package gamemaker

import "context"

type jobContextKey struct{}

// WithJobContext binds an isolated Studio run to a server-selected job. Never
// populate this context from model arguments or a user-supplied session name.
func WithJobContext(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobContextKey{}, jobID)
}

func JobIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(jobContextKey{}).(string)
	return id
}
