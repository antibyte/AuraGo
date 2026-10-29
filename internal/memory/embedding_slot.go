package memory

import (
	"context"
	"sync"
)

// embedSlot admits one store-side provider call at a time, as a mutex would, but
// a waiter gives up as soon as its own context ends instead of staying blocked
// behind a slow holder. The zero value is ready to use.
type embedSlot struct {
	once sync.Once
	held chan struct{} // one token while the slot is taken
}

func (s *embedSlot) channel() chan struct{} {
	s.once.Do(func() { s.held = make(chan struct{}, 1) })
	return s.held
}

// acquire takes the slot, or returns the context error when ctx ends first. A
// caller whose ctx has ended never takes the slot.
func (s *embedSlot) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.channel() <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// release frees a slot taken by acquire.
func (s *embedSlot) release() { <-s.channel() }
