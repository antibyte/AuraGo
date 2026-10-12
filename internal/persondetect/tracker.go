package persondetect

import (
	"sort"
	"time"
)

// AnalyzedFrame is one detector result for a camera.
type AnalyzedFrame struct {
	At            time.Time
	JPEG          []byte
	Width, Height int
	Boxes         []Box    // every box the detector returned for the camera's threshold
	Hits          []Box    // boxes that count (threshold and zones)
	Zones         []string // zones hit in this frame, sorted
	Score         float64  // highest hit score, 0 without hits
}

// IsHit reports whether the frame contains at least one counting person.
func (f AnalyzedFrame) IsHit() bool { return len(f.Hits) > 0 }

// EventStats are the aggregates of an open event. Zones is never nil.
type EventStats struct {
	MaxScore   float64
	MaxPersons int
	Zones      []string
}

// trackStep is what one observation asks the service to persist, in this order:
// end the open event, start a new one, store frames, update the aggregates.
type trackStep struct {
	Ended     bool
	EndedAt   time.Time
	Started   bool
	StartedAt time.Time
	Store     []AnalyzedFrame
	Updated   bool
	Stats     EventStats
}

type pendingHit struct {
	seq   int
	frame AnalyzedFrame
}

// tracker is the per-camera event state machine. It is not safe for concurrent use.
type tracker struct {
	seq        int
	recent     []bool
	pending    []pendingHit
	open       bool
	startedAt  time.Time
	lastHit    time.Time
	lastStored time.Time
	stored     int
	maxScore   float64
	maxPersons int
	zones      map[string]struct{}
}

func newTracker() *tracker { return &tracker{} }

// Open reports whether an event is open.
func (t *tracker) Open() bool { return t.open }

// Observe feeds one analyzed frame.
func (t *tracker) Observe(f AnalyzedFrame) trackStep {
	var step trackStep
	if t.open && f.At.Sub(t.lastHit) >= EventEndAfter {
		step.Ended, step.EndedAt = true, t.lastHit
		t.reset()
	}
	t.seq++
	t.recent = append(t.recent, f.IsHit())
	if len(t.recent) > EventStartWindow {
		t.recent = t.recent[len(t.recent)-EventStartWindow:]
	}
	if t.open {
		if !f.IsHit() {
			return step
		}
		t.lastHit = f.At
		t.absorb(f)
		step.Updated, step.Stats = true, t.stats()
		if t.stored < EventMaxFrames && f.At.Sub(t.lastStored) >= EventFrameInterval {
			step.Store = []AnalyzedFrame{f}
			t.stored++
			t.lastStored = f.At
		}
		return step
	}
	kept := t.pending[:0]
	for _, p := range t.pending {
		if t.seq-p.seq < EventStartWindow {
			kept = append(kept, p)
		}
	}
	t.pending = kept
	if !f.IsHit() {
		return step
	}
	t.pending = append(t.pending, pendingHit{seq: t.seq, frame: f})
	if len(t.pending) > EventPendingHitFrames {
		t.pending = t.pending[len(t.pending)-EventPendingHitFrames:]
	}
	hits := 0
	for _, hit := range t.recent {
		if hit {
			hits++
		}
	}
	if hits < EventStartHits && f.Score < EventSingleHitScore {
		return step
	}
	t.open = true
	t.zones = map[string]struct{}{}
	t.maxScore, t.maxPersons = 0, 0
	for _, p := range t.pending {
		t.absorb(p.frame)
		step.Store = append(step.Store, p.frame)
	}
	t.startedAt = t.pending[0].frame.At
	t.lastHit = f.At
	t.lastStored = f.At
	t.stored = len(step.Store)
	t.pending = nil
	step.Started, step.StartedAt = true, t.startedAt
	step.Updated, step.Stats = true, t.stats()
	return step
}

// Expire ends the open event when now is at least EventEndAfter past the last hit.
func (t *tracker) Expire(now time.Time) (time.Time, bool) {
	if !t.open || now.Sub(t.lastHit) < EventEndAfter {
		return time.Time{}, false
	}
	at := t.lastHit
	t.reset()
	return at, true
}

// Close ends the open event at its last hit (shutdown, camera removed, event deleted).
func (t *tracker) Close() (time.Time, bool) {
	if !t.open {
		return time.Time{}, false
	}
	at := t.lastHit
	t.reset()
	return at, true
}

func (t *tracker) reset() {
	*t = tracker{seq: t.seq}
}

func (t *tracker) absorb(f AnalyzedFrame) {
	if f.Score > t.maxScore {
		t.maxScore = f.Score
	}
	if len(f.Hits) > t.maxPersons {
		t.maxPersons = len(f.Hits)
	}
	if t.zones == nil {
		t.zones = map[string]struct{}{}
	}
	for _, zone := range f.Zones {
		t.zones[zone] = struct{}{}
	}
}

func (t *tracker) stats() EventStats {
	zones := make([]string, 0, len(t.zones))
	for zone := range t.zones {
		zones = append(zones, zone)
	}
	sort.Strings(zones)
	return EventStats{MaxScore: t.maxScore, MaxPersons: t.maxPersons, Zones: zones}
}
