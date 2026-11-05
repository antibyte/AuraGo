package flows

import (
	"context"
	"sync"
)

// blockAdmissionTracker contains at most the four admissions allowed by Runner.
// Its lock never protects a bridge, database, service or flow-lock call.
type blockAdmissionTracker struct {
	mu     sync.Mutex
	active map[*blockAdmission]struct{}
}

type blockAdmission struct {
	flowID, missionID string
	mode              RunMode
	cancel            context.CancelFunc
}

func (t *blockAdmissionTracker) add(flowID, missionID string, mode RunMode, cancel context.CancelFunc) *blockAdmission {
	a := &blockAdmission{flowID: flowID, missionID: missionID, mode: mode, cancel: cancel}
	t.mu.Lock()
	if t.active == nil {
		t.active = make(map[*blockAdmission]struct{})
	}
	t.active[a] = struct{}{}
	t.mu.Unlock()
	return a
}

func (t *blockAdmissionTracker) remove(a *blockAdmission) {
	t.mu.Lock()
	delete(t.active, a)
	t.mu.Unlock()
}

// transfer orders admission against revocation. A successful transfer is the
// point at which a run becomes an ordinary accepted run.
func (t *blockAdmissionTracker) transfer(a *blockAdmission, ctx context.Context, accept func()) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	delete(t.active, a)
	accept()
	return nil
}

func (t *blockAdmissionTracker) cancelMatching(match func(*blockAdmission) bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for a := range t.active {
		if match(a) {
			a.cancel()
		}
	}
}

// InvalidateFlowAdmissions is safe for a synchronous Mission Control callback
// while its own lock is held: it only cancels bounded in-flight admissions.
func (s *Service) InvalidateFlowAdmissions(missionID string) {
	if missionID == "" {
		return
	}
	s.runner.blockAdmissionTokens.cancelMatching(func(a *blockAdmission) bool {
		return a.missionID == missionID && a.mode != ModeTest
	})
}
