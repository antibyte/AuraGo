package persondetect

import (
	"context"
	"log/slog"
	"time"
)

// NewManualServiceForTest creates a service whose Configure starts no camera,
// detection, arming or retention goroutines; tests drive single steps instead.
// Detector setups (and the database open that precedes them) still run in the
// background.
func NewManualServiceForTest(deps Deps) *Service {
	s := NewService(deps)
	s.manual = true
	return s
}

// FetchOnceForTest runs one frame request of a camera and returns the next delay.
func (s *Service) FetchOnceForTest(ctx context.Context, cameraID string) time.Duration {
	return s.fetchOnce(ctx, cameraID)
}

// QueuedFramesForTest is the number of cameras with a frame waiting for analysis.
func (s *Service) QueuedFramesForTest() int { return s.slots.queued() }

// WaitBackgroundForTest waits for the spawned background work of a manual service.
func (s *Service) WaitBackgroundForTest(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// SetStoreOpenerForTest replaces OpenEventStore; call it before the first Configure.
func (s *Service) SetStoreOpenerForTest(open func(dataDir string, logger *slog.Logger) (*EventStore, error)) {
	s.storeOpener = open
}

// StoreErrForTest is the error an API call that needs the database gets now (nil
// when the database is open).
func (s *Service) StoreErrForTest() error {
	_, err := s.apiStore()
	return err
}

// HandleDetectErrorForTest applies a Detect error of the current detector to a
// camera, as the detection loop does.
func (s *Service) HandleDetectErrorForTest(ctx context.Context, cameraID string, err error) {
	s.mu.Lock()
	det := s.det
	s.mu.Unlock()
	s.handleDetectError(ctx, det, cameraID, err)
}

// FailDetectorForTest takes the current detector out of service as after an
// ErrDetectorFailed; it returns the delay of the scheduled setup, false when no
// detector was ready.
func (s *Service) FailDetectorForTest(err error) (time.Duration, bool) {
	s.mu.Lock()
	det := s.det
	s.mu.Unlock()
	return s.detectorFailed(det, err)
}

// SetDetectorRetryForTest sets the delays of the automatic detector setups after
// a detector failed: base for the first, doubling up to limit.
func (s *Service) SetDetectorRetryForTest(base, limit time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.detRetryBase, s.detRetryMax = base, limit
}
