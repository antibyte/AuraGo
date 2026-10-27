package dockerstacks

import (
	"context"
	"time"
)

// queryTimeout bounds a journal read.
const queryTimeout = 10 * time.Second

// activeRunByID returns the live run of an operation, if it has not finished.
func (s *Service) activeRunByID(id string) *opRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs[id]
}

// Operation returns one operation. A running one is answered from memory (with
// the output so far); a finished one from the journal.
func (s *Service) Operation(ctx context.Context, id string) (Operation, error) {
	if run := s.activeRunByID(id); run != nil {
		return run.snapshot(), nil
	}
	return s.journal.get(ctx, id)
}

// Operations lists journal entries newest first. Their Output is not filled in;
// Operation(id) has it.
func (s *Service) Operations(ctx context.Context, f OperationFilter) ([]Operation, error) {
	ops, err := s.journal.list(ctx, f)
	if err != nil {
		return nil, err
	}
	// A running operation is newer in memory than in its row.
	for i := range ops {
		if !ops[i].Done() {
			if run := s.activeRunByID(ops[i].ID); run != nil {
				live := run.snapshot()
				live.Output = ""
				ops[i] = live
			}
		}
	}
	return ops, nil
}

// Subscribe returns the current state of an operation, with the output so far,
// and a channel of the events that follow. The channel is closed after the done
// event, on cancel, or when the subscriber falls 256 events behind (the client
// reconnects for a fresh snapshot). For a finished operation it returns the
// snapshot and an already closed channel.
func (s *Service) Subscribe(id string) (Operation, <-chan Event, func(), error) {
	if run := s.activeRunByID(id); run != nil {
		if op, events, cancel, ok := run.subscribe(); ok {
			return op, events, cancel, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	op, err := s.journal.get(ctx, id)
	if err != nil {
		return Operation{}, nil, nil, err
	}
	closed := make(chan Event)
	close(closed)
	return op, closed, func() {}, nil
}
