package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const defaultSideEffectShutdownTimeout = 30 * time.Second

// AsyncTaskGroup tracks cancellable background side effects that must drain
// before memory databases are closed.
type AsyncTaskGroup struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

func NewAsyncTaskGroup(parent context.Context) *AsyncTaskGroup {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &AsyncTaskGroup{ctx: ctx, cancel: cancel}
}

func (g *AsyncTaskGroup) Go(fn func(context.Context)) bool {
	if g == nil || fn == nil {
		return false
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return false
	}
	ctx := g.ctx
	g.wg.Add(1)
	g.mu.Unlock()

	go func() {
		defer g.wg.Done()
		fn(ctx)
	}()
	return true
}

func (g *AsyncTaskGroup) Shutdown(timeout time.Duration) error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	if !g.closed {
		g.closed = true
		g.cancel()
	}
	g.mu.Unlock()

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	if timeout <= 0 {
		<-done
		return nil
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("async side effects did not stop within %s", timeout)
	}
}

var defaultSideEffects = struct {
	mu    sync.Mutex
	group *AsyncTaskGroup
}{
	group: NewAsyncTaskGroup(context.Background()),
}

func sideEffectsFromRunConfig(runCfg RunConfig) *AsyncTaskGroup {
	if runCfg.SideEffects != nil {
		return runCfg.SideEffects
	}
	defaultSideEffects.mu.Lock()
	defer defaultSideEffects.mu.Unlock()
	if defaultSideEffects.group == nil {
		defaultSideEffects.group = NewAsyncTaskGroup(context.Background())
	}
	return defaultSideEffects.group
}

func shutdownDefaultSideEffects(timeout time.Duration) error {
	defaultSideEffects.mu.Lock()
	group := defaultSideEffects.group
	defaultSideEffects.group = NewAsyncTaskGroup(context.Background())
	defaultSideEffects.mu.Unlock()
	return group.Shutdown(timeout)
}
