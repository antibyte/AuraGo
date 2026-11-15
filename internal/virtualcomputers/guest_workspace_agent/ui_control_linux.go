//go:build linux

package main

import (
	"context"
	"sync"
)

// uiControl fences RPCs already in transit as well as queued and active input.
// The host changes the token before acknowledging a human takeover.
type uiControl struct {
	mu      sync.Mutex
	serial  sync.Mutex
	token   string
	blocked bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func (u *uiControl) enter(ctx context.Context, token string, input bool) (context.Context, func(), error) {
	u.serial.Lock()
	u.mu.Lock()
	if ctx.Err() != nil || token != u.token || (input && u.blocked) {
		u.mu.Unlock()
		u.serial.Unlock()
		return nil, nil, &rpcError{Code: "desktop_control_changed", Message: "observe again after workspace control is returned"}
	}
	callCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	u.cancel, u.done = cancel, done
	u.mu.Unlock()
	return callCtx, func() {
		cancel()
		u.mu.Lock()
		u.cancel, u.done = nil, nil
		close(done)
		u.mu.Unlock()
		u.serial.Unlock()
	}, nil
}

func (u *uiControl) change(ctx context.Context, token string, blocked bool) error {
	u.mu.Lock()
	u.token, u.blocked = token, blocked
	cancel, done := u.cancel, u.done
	if cancel != nil {
		cancel()
	}
	u.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func browserInput(op string) bool {
	switch op {
	case "inspect", "list_tabs", "screenshot", "list_downloads", "wait":
		return false
	}
	return true
}
