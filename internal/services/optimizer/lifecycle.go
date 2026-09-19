package optimizer

import (
	"context"
	"sync"
)

var managedRuntime struct {
	sync.Mutex
	wake    chan struct{}
	desired bool
}

// SetEnabled updates the live worker; rapid toggles never overlap workers.
func SetEnabled(enabled bool) {
	managedRuntime.Lock()
	defer managedRuntime.Unlock()
	managedRuntime.desired = enabled
	if managedRuntime.wake != nil {
		select {
		case managedRuntime.wake <- struct{}{}:
		default:
		}
	}
}

func (w *OptimizerWorker) StartManaged(parent context.Context, enabled bool) func() {
	return startManagedOptimizer(parent, enabled, w.Start)
}
func startManagedOptimizer(parent context.Context, enabled bool, start func(context.Context)) func() {
	ctx, stop := context.WithCancel(parent)
	wake := make(chan struct{}, 1)
	finished := make(chan struct{})
	managedRuntime.Lock()
	managedRuntime.wake, managedRuntime.desired = wake, enabled
	managedRuntime.Unlock()
	go func() {
		defer close(finished)
		var cancel context.CancelFunc
		var done chan struct{}
		for {
			managedRuntime.Lock()
			desired := managedRuntime.desired
			managedRuntime.Unlock()
			if desired && done == nil {
				var workerCtx context.Context
				workerCtx, cancel = context.WithCancel(ctx)
				done = make(chan struct{})
				go func(completed chan struct{}) { defer close(completed); start(workerCtx) }(done)
			} else if !desired && cancel != nil {
				cancel()
			}
			select {
			case <-ctx.Done():
				if cancel != nil {
					cancel()
				}
				if done != nil {
					<-done
				}
				return
			case <-wake:
			case <-done:
				done = nil
				cancel = nil
			}
		}
	}()
	return func() { stop(); <-finished }
}
