package tools

import (
	"context"
	"errors"
	"sync"
)

// StoreAccess is server-owned. A managed legacy config cannot silently fall
// back to a manual connection when the Store has not verified its binding.
type StoreAccess func(context.Context, string, string, bool) (context.Context, func(), error)

var storeAccess struct {
	sync.RWMutex
	acquire StoreAccess
}

func SetStoreAccess(acquire StoreAccess) {
	storeAccess.Lock()
	storeAccess.acquire = acquire
	storeAccess.Unlock()
}

func AcquireStoreAccess(ctx context.Context, app, base string, mutation bool) (context.Context, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	storeAccess.RLock()
	acquire := storeAccess.acquire
	storeAccess.RUnlock()
	if acquire == nil {
		return ctx, func() {}, errors.New("Store integration is not connected")
	}
	return acquire(ctx, app, base, mutation)
}
