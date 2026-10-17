package memory

import (
	"context"
	"sync"
	"time"
)

// Waiting behind another embedding writer must respect the same deadline as I/O.
func lockMemoryMutex(ctx context.Context, mu *sync.Mutex) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if mu.TryLock() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}
