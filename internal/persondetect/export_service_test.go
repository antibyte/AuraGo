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

// ProcessOneForTest analyzes the next queued frame; false when none was queued.
// The frames it stores and the alert of an event start are written in the
// background: WaitStorageForTest waits for them.
func (s *Service) ProcessOneForTest(ctx context.Context) bool { return s.processOne(ctx) }

// SweepForTest ends events whose last hit is at least the end-after time before
// now; pass a time at or after the last hit.
func (s *Service) SweepForTest(now time.Time) { s.sweep(func() time.Time { return now }) }

// RunRetentionForTest runs one retention pass with the current settings.
func (s *Service) RunRetentionForTest(ctx context.Context) (RetentionResult, error) {
	return s.runRetention(ctx)
}

// NoteWriteErrorForTest feeds a frame write error to the storage bookkeeping.
func (s *Service) NoteWriteErrorForTest(err error) { s.noteWriteError(err) }

// RetentionRequestedForTest reports and clears a pending immediate retention run.
func (s *Service) RetentionRequestedForTest() bool {
	select {
	case <-s.retentionNow:
		return true
	default:
		return false
	}
}

// WaitStorageForTest waits until every queued frame write and alert dispatch is done.
func (s *Service) WaitStorageForTest(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		s.mu.Lock()
		idle := len(s.storeQueues) == 0
		s.mu.Unlock()
		if idle {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

// SetThumbnailerForTest replaces makeThumbnail for frames wider than ThumbnailWidth.
func (s *Service) SetThumbnailerForTest(build func([]byte) ([]byte, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.thumbnailer = build
}

// SetSweepIntervalForTest sets the sweep period of the detection loop; call it
// before the first Configure.
func (s *Service) SetSweepIntervalForTest(d time.Duration) { s.sweepEvery = d }

// SweepsForTest is the number of sweeps run so far.
func (s *Service) SweepsForTest() int64 { return s.sweeps.Load() }

// SetArmModeForTest sets the arm mode in memory only: no store write, no arm log.
func (s *Service) SetArmModeForTest(mode ArmMode) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.arm = armCache{mode: mode, armed: mode == ArmOn}
}

// SetStoreReadOnlyForTest makes every write to detections.db fail (PRAGMA
// query_only on the store's single connection) or allows writes again.
func (s *Service) SetStoreReadOnlyForTest(readOnly bool) error {
	st := s.storeRef()
	if st == nil {
		return ErrUnavailable
	}
	value := "0"
	if readOnly {
		value = "1"
	}
	_, err := st.db.Exec(`PRAGMA query_only = ` + value)
	return err
}
