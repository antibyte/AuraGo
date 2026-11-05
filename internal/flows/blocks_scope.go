package flows

import "sync"

// blockRunScope shares the parent run's budgets with all concurrent child frames.
// Existing ordinary runs and private durable checkpoints keep their local counters.
type blockRunScope struct {
	mu              sync.Mutex
	outputBytes     int
	storedBytes     int
	history         *childHistoryBudget
	historyScrubber Scrubber
}

func (s *blockRunScope) reserveOutput(size int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if size < 0 || size > MaxRunOutputBytes-s.outputBytes {
		return false
	}
	s.outputBytes += size
	return true
}

func (s *blockRunScope) reserveStored(size int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if size < 0 || size > MaxStoredRunBytes-s.storedBytes {
		return false
	}
	s.storedBytes += size
	return true
}
