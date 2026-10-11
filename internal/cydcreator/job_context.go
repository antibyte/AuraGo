package cydcreator

import "context"

type jobContextKey struct{}

// WithJobContext binds an agent run to a server-selected job. The agent tools
// read the job only from here; never populate it from model arguments or a
// user-supplied value.
func WithJobContext(ctx context.Context, jobID string) context.Context {
	return context.WithValue(ctx, jobContextKey{}, jobID)
}

// JobIDFromContext returns the job bound by WithJobContext or "".
func JobIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(jobContextKey{}).(string)
	return id
}
