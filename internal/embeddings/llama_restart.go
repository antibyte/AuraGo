package embeddings

import (
	"context"
	"fmt"
)

// llamaServerOps is the part of a llama.cpp embedder that the shared restart
// policy needs. The native process and the Docker sidecar implement it with
// their own start and stop mechanics.
type llamaServerOps struct {
	// name is used in error messages, for example "llama-server".
	name string
	// running reports whether the server is believed to be up.
	running func() bool
	// start brings a server that is not running up again.
	start func(ctx context.Context) error
	embed func(ctx context.Context, texts []string) ([][]float32, error)
	// stop takes a server that just failed out of service so that the next
	// attempt restarts it.
	stop func()
}

// embedWithRestarts runs one embedding request with at most one restart of the
// server in between. Restarts are paid from budget, a sliding window.
//
// An attempt that failed because the caller's context ended says nothing about
// the server: it is returned at once, the server is neither stopped nor
// restarted and no budget is spent. A restart is never begun for a context
// that is already done.
func embedWithRestarts(ctx context.Context, budget *restartBudget, ops llamaServerOps, texts []string) ([][]float32, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !ops.running() {
			if !budget.take() {
				return nil, fmt.Errorf("%s restart limit reached (%d per %s): %w",
					ops.name, maxProcessRestarts, processRestartWindow, restartCause(lastErr))
			}
			if err := ops.start(ctx); err != nil {
				if isCallerError(ctx, err) {
					budget.release()
					return nil, err
				}
				lastErr = err
				continue
			}
		}
		vectors, err := ops.embed(ctx, texts)
		if err == nil {
			return vectors, nil
		}
		if isCallerError(ctx, err) {
			return nil, err
		}
		lastErr = err
		ops.stop()
	}
	return nil, fmt.Errorf("%s embedding failed after restart: %w", ops.name, restartCause(lastErr))
}

// restartCause keeps the %w verb valid when no attempt has failed yet (the
// budget was already empty when the call arrived).
func restartCause(err error) error {
	if err == nil {
		return fmt.Errorf("no restart left from earlier failures")
	}
	return err
}
