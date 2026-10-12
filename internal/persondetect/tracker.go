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
	Boxes         []Box    // every box the detector returned for the camera's threshold; not persisted: events store Hits only (S2 Task 5 decision)
	Hits          []Box    // boxes that count (threshold and zones)
	Zones         []string // zones hit in this frame, sorted
	Score         float64  // highest hit score, 0 without hits
}

// IsHit reports whether the frame contains at least one counting person.
func (f AnalyzedFrame) IsHit() bool { return len(f.Hits) > 0 }

// EventStats are the aggregates of an open event. Zones is never nil.
//
// MaxScore is the highest score of every analyzed hit of the event, stored or
// not. The event's cover is the best STORED frame, and frames are stored at most
// once per EventFrameInterval, so max_score can exceed the score of every stored
// frame.
type EventStats struct {
	MaxScore   float64
	MaxPersons int
	Zones      []string
}

// trackStep is what one observation asks the service to persist, in this order:
// end the open event, start a new one, store frames, update the aggregates.
// Updated is true only when the aggregates changed or a frame is stored (the
// event row has something new), not on every hit frame.
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
	endAfter   time.Duration
	seq        int
	lastAt     time.Time // At of the previous observed frame, to notice a clock step back
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

// newTracker returns a tracker that ends events EventEndAfter after the last
// hit. SetEndAfter adapts that to the camera's analysis interval.
func newTracker() *tracker { return &tracker{endAfter: EventEndAfter} }

// SetEndAfter sets the time without a hit after which an event ends (see
// EventEndAfterFor); d <= 0 restores EventEndAfter. It applies to an open event
// at once and survives the end of an event.
func (t *tracker) SetEndAfter(d time.Duration) {
	if d <= 0 {
		d = EventEndAfter
	}
	t.endAfter = d
}

// Open reports whether an event is open.
func (t *tracker) Open() bool { return t.open }

// Observe feeds one analyzed frame.
//
// f.At is wall-clock time: the service clock truncates timestamps to
// milliseconds UTC, which strips Go's monotonic reading, so a step of the system
// clock (NTP, a manual change, a VM restore) is visible here. A frame earlier
// than the last hit of the open event (a negative gap) is therefore a clock
// break, not "a hit 30 s ago": the open event ends at its last hit and the
// frame is a fresh candidate for the next one. With no event open, a frame
// earlier than the previous one empties the start window, so hits from before
// and after a step never start an event together.
func (t *tracker) Observe(f AnalyzedFrame) trackStep {
	var step trackStep
	if t.open {
		if gap := f.At.Sub(t.lastHit); gap < 0 || gap >= t.endAfter {
			step.Ended, step.EndedAt = true, t.lastHit
			t.reset()
		}
	} else if f.At.Before(t.lastAt) {
		t.reset()
	}
	t.lastAt = f.At
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
		changed := t.absorb(f)
		if t.stored < EventMaxFrames && f.At.Sub(t.lastStored) >= EventFrameInterval {
			step.Store = []AnalyzedFrame{f}
			t.stored++
			t.lastStored = f.At
			changed = true
		}
		if changed {
			step.Updated, step.Stats = true, t.stats()
		}
		return step
	}
	t.prunePending()
	if !f.IsHit() {
		return step
	}
	t.pending = append(t.pending, pendingHit{seq: t.seq, frame: f})
	if drop := len(t.pending) - EventPendingHitFrames; drop > 0 {
		n := copy(t.pending, t.pending[drop:])
		clear(t.pending[n:])
		t.pending = t.pending[:n]
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

// prunePending drops the pending hits that left the start window and clears
// their slots, so the buffer does not keep their JPEGs alive.
func (t *tracker) prunePending() {
	kept := t.pending[:0]
	for _, p := range t.pending {
		if t.seq-p.seq < EventStartWindow {
			kept = append(kept, p)
		}
	}
	clear(t.pending[len(kept):])
	t.pending = kept
}

// Expire ends the open event when now is at least the end-after time past the
// last hit, or before it (the clock stepped back). The event ends at its last hit.
func (t *tracker) Expire(now time.Time) (time.Time, bool) {
	if !t.open {
		return time.Time{}, false
	}
	if gap := now.Sub(t.lastHit); gap >= 0 && gap < t.endAfter {
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

// reset forgets the event, the start window and the pending hits (their buffer
// is dropped with them). It keeps the sequence number, the end-after time and
// the last frame time.
func (t *tracker) reset() {
	*t = tracker{endAfter: t.endAfter, seq: t.seq, lastAt: t.lastAt}
}

// absorb adds a hit frame to the aggregates and reports whether any changed.
func (t *tracker) absorb(f AnalyzedFrame) (changed bool) {
	if f.Score > t.maxScore {
		t.maxScore = f.Score
		changed = true
	}
	if len(f.Hits) > t.maxPersons {
		t.maxPersons = len(f.Hits)
		changed = true
	}
	if t.zones == nil {
		t.zones = map[string]struct{}{}
	}
	for _, zone := range f.Zones {
		if _, known := t.zones[zone]; !known {
			t.zones[zone] = struct{}{}
			changed = true
		}
	}
	return changed
}

func (t *tracker) stats() EventStats {
	zones := make([]string, 0, len(t.zones))
	for zone := range t.zones {
		zones = append(zones, zone)
	}
	sort.Strings(zones)
	return EventStats{MaxScore: t.maxScore, MaxPersons: t.maxPersons, Zones: zones}
}
