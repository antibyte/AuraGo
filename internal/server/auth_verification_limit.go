package server

import (
	"context"
	"time"
)

var adminVerifications = make(chan struct{}, 4)
var throttledAdminVerifications = make(chan struct{}, 1)

// The account budget limits verification throughput; only the attacking IP
// receives a hard lockout. Valid credentials from another IP remain usable.
func beginAdminVerification(ctx context.Context, ipKey, accountKey string) (func(), bool) {
	gate := adminVerifications
	if IsLockedOut(accountKey) {
		gate = throttledAdminVerifications
	}
	select {
	case gate <- struct{}{}:
	default:
		return nil, false
	}
	release := func() { <-gate }
	if delay := LoginBackoffDelay(ipKey, accountKey); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			release()
			return nil, false
		case <-timer.C:
		}
	}
	return release, true
}
