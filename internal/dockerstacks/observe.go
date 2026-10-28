package dockerstacks

import "slices"

// Observe registers fn for every operation state change: queued, running,
// health_wait and the final succeeded, failed or interrupted. It is never
// called for output or progress. The Operation handed to fn has an empty
// Output and is read-only (its time pointers are shared). fn runs on the
// goroutine that changed the state, possibly with the service's and the
// operation's locks held: it must not block and must not call back into the
// Service. The returned func
// unregisters fn and may be called more than once.
func (s *Service) Observe(fn func(Operation)) (cancel func()) {
	if fn == nil {
		return func() {}
	}
	s.obsMu.Lock()
	if s.observers == nil {
		s.observers = map[int]func(Operation){}
	}
	s.nextObserver++
	id := s.nextObserver
	s.observers[id] = fn
	s.obsMu.Unlock()
	return func() {
		s.obsMu.Lock()
		defer s.obsMu.Unlock()
		delete(s.observers, id)
	}
}

// notify tells the observers about a state change, in registration order.
func (s *Service) notify(op Operation) {
	s.obsMu.Lock()
	if len(s.observers) == 0 {
		s.obsMu.Unlock()
		return
	}
	ids := make([]int, 0, len(s.observers))
	for id := range s.observers {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	fns := make([]func(Operation), 0, len(ids))
	for _, id := range ids {
		fns = append(fns, s.observers[id])
	}
	s.obsMu.Unlock()
	op.Output = ""
	for _, fn := range fns {
		fn(op)
	}
}
