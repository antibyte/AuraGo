package embeddings

import (
	"context"
	"errors"
)

var (
	// ErrInvalidInput marks an embedding request that the caller got wrong
	// (for example an empty text list). It says nothing about backend health,
	// so it never counts as a backend failure and never triggers a fallback or
	// a re-benchmark.
	ErrInvalidInput = errors.New("invalid embedding input")

	// ErrRestartRequired is returned by Rebenchmark while a scheduled embedding
	// reindex waits for a process restart. Benchmarking in that state could
	// activate a backend of another vector space before the reindex has run.
	ErrRestartRequired = errors.New("a local embedding restart is pending; restart AuraGo before running another benchmark")

	// errBackendUnresponsive marks a llama.cpp server that kept running into
	// its callers' deadlines and then failed a liveness probe. The callers'
	// deadline errors describe the server in that case, so it wraps one but is
	// not a caller error.
	errBackendUnresponsive = errors.New("embedding backend unresponsive")
)

// isCallerError reports whether err was caused by the caller rather than by an
// embedding backend: an invalid request, or the caller's own context ending.
// A context error only counts while ctx itself is done; the same error from a
// healthy caller context (for example an internal HTTP client timeout) still
// describes a failing backend.
func isCallerError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	// Checked before the context: the wrapped deadline error is the caller's.
	if errors.Is(err, errBackendUnresponsive) {
		return false
	}
	if errors.Is(err, ErrInvalidInput) {
		return true
	}
	if ctx == nil || ctx.Err() == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
