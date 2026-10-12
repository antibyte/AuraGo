package persondetect

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"
)

// modelStep is what trackerModel decided for one observation.
type modelStep struct {
	ended, started, updated bool
	endedAt, startedAt      time.Time
	store                   []time.Time
	stats                   EventStats
}

// trackerModel restates the event rules naively. It keeps every frame since the
// last end, sweep or clock break and decides each step from that log, where the
// tracker keeps running counters: the three-frame window is the last three log
// entries, the aggregates are recomputed from the hit frames of the event.
type trackerModel struct {
	endAfter time.Duration
	log      []AnalyzedFrame // frames of the current epoch
	open     bool
	hits     []AnalyzedFrame // hit frames of the open event
	stored   []time.Time     // frames the open event asked to store
}

func (m *trackerModel) lastHit() time.Time { return m.hits[len(m.hits)-1].At }

func (m *trackerModel) aggregate() EventStats {
	zones := map[string]bool{}
	var s EventStats
	for _, f := range m.hits {
		s.MaxScore = max(s.MaxScore, f.Score)
		s.MaxPersons = max(s.MaxPersons, len(f.Hits))
		for _, zone := range f.Zones {
			zones[zone] = true
		}
	}
	s.Zones = make([]string, 0, len(zones))
	for zone := range zones {
		s.Zones = append(s.Zones, zone)
	}
	sort.Strings(s.Zones)
	return s
}

func (m *trackerModel) finish() time.Time {
	at := m.lastHit()
	m.open, m.hits, m.stored, m.log = false, nil, nil, nil
	return at
}

func (m *trackerModel) observe(f AnalyzedFrame) (out modelStep) {
	if m.open {
		if gap := f.At.Sub(m.lastHit()); gap < 0 || gap >= m.endAfter {
			out.ended, out.endedAt = true, m.finish()
		}
	} else if n := len(m.log); n > 0 && f.At.Before(m.log[n-1].At) {
		m.log = nil // the clock stepped back: older frames are no part of the window
	}
	m.log = append(m.log, f)
	if m.open {
		if !f.IsHit() {
			return out
		}
		before := m.aggregate()
		m.hits = append(m.hits, f)
		changed := !reflect.DeepEqual(before, m.aggregate())
		if len(m.stored) < 20 && f.At.Sub(m.stored[len(m.stored)-1]) >= 5*time.Second {
			m.stored = append(m.stored, f.At)
			out.store = []time.Time{f.At}
			changed = true
		}
		if changed {
			out.updated, out.stats = true, m.aggregate()
		}
		return out
	}
	window := m.log
	if len(window) > 3 {
		window = window[len(window)-3:]
	}
	var inWindow []AnalyzedFrame
	for _, w := range window {
		if w.IsHit() {
			inWindow = append(inWindow, w)
		}
	}
	if !f.IsHit() || (len(inWindow) < 2 && f.Score < 0.8) {
		return out
	}
	if len(inWindow) > 2 {
		inWindow = inWindow[len(inWindow)-2:]
	}
	m.open, m.hits = true, inWindow
	for _, w := range inWindow {
		m.stored = append(m.stored, w.At)
		out.store = append(out.store, w.At)
	}
	out.started, out.startedAt = true, inWindow[0].At
	out.updated, out.stats = true, m.aggregate()
	return out
}

func (m *trackerModel) expire(now time.Time) (time.Time, bool) {
	if !m.open {
		return time.Time{}, false
	}
	if gap := now.Sub(m.lastHit()); gap >= 0 && gap < m.endAfter {
		return time.Time{}, false
	}
	return m.finish(), true
}

func (m *trackerModel) close() (time.Time, bool) {
	if !m.open {
		return time.Time{}, false
	}
	return m.finish(), true
}

func randomTrackGap(r *rand.Rand, endAfter time.Duration) time.Duration {
	switch n := r.Intn(20); {
	case n == 0: // the clock steps back
		return -time.Duration(1+r.Intn(120000)) * time.Millisecond
	case n < 3: // right at the thresholds
		edges := []time.Duration{
			EventFrameInterval - time.Millisecond, EventFrameInterval, EventFrameInterval + time.Millisecond,
			endAfter - time.Millisecond, endAfter, endAfter + time.Millisecond,
		}
		return edges[r.Intn(len(edges))]
	case n < 5: // a slow camera
		return time.Duration(7000+r.Intn(30000)) * time.Millisecond
	case n < 7: // the same millisecond
		return 0
	default:
		return time.Duration(500+r.Intn(5000)) * time.Millisecond
	}
}

func randomTrackFrame(r *rand.Rand, at time.Time, pHit float64) AnalyzedFrame {
	f := AnalyzedFrame{At: at}
	if r.Float64() < 0.15 { // a box the zones filter out
		f.Boxes = []Box{{X1: 0.1, Y1: 0.1, X2: 0.2, Y2: 0.9, Score: 0.9}}
	}
	if r.Float64() >= pHit {
		return f
	}
	scores := []float64{0.5, 0.6, 0.79, 0.8, 0.9, 0.99}
	for n := 1 + r.Intn(3); n > 0; n-- {
		box := Box{X1: 0.4, Y1: 0.3, X2: 0.5, Y2: 0.9, Score: scores[r.Intn(len(scores))]}
		f.Boxes = append(f.Boxes, box)
		f.Hits = append(f.Hits, box)
		f.Score = max(f.Score, box.Score)
	}
	for _, zone := range []string{"A", "B", "C"} {
		if r.Intn(2) == 0 {
			f.Zones = append(f.Zones, zone)
		}
	}
	return f
}

func sameTrackTimes(got []time.Time, want []time.Time) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if !got[i].Equal(want[i]) {
			return false
		}
	}
	return true
}

// TestTrackerMatchesNaiveModel feeds seeded random frame sequences (gaps around
// the thresholds, clock steps back, sweeps, closes, end-after changes) to the
// tracker and to trackerModel and compares every step. It also checks that an
// event never stores more than EventMaxFrames frames and never ends before it
// started.
func TestTrackerMatchesNaiveModel(t *testing.T) {
	for seed := int64(1); seed <= 8; seed++ {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			for run := 0; run < 40; run++ {
				runTrackerModel(t, r, run)
			}
		})
	}
}

func runTrackerModel(t *testing.T, r *rand.Rand, run int) {
	t.Helper()
	endChoices := []time.Duration{EventEndAfter, EventEndAfterFor(15 * time.Second), EventEndAfterFor(30 * time.Second)}
	endAfter := endChoices[r.Intn(len(endChoices))]
	tr := newTracker()
	tr.SetEndAfter(endAfter)
	m := &trackerModel{endAfter: endAfter}
	pHit := []float64{0.25, 0.5, 0.8}[r.Intn(3)]
	now := trackT0
	var eventStart time.Time
	eventStored := 0
	finished := func(where string, at time.Time) {
		t.Helper()
		if at.Before(eventStart) {
			t.Fatalf("run %d %s: event ended at %v before it started at %v", run, where, at, eventStart)
		}
	}
	for i := 0; i < 150; i++ {
		switch roll := r.Intn(100); {
		case roll < 85:
			now = now.Add(randomTrackGap(r, endAfter))
			f := randomTrackFrame(r, now, pHit)
			got, want := tr.Observe(f), m.observe(f)
			gotStore := make([]time.Time, 0, len(got.Store))
			for _, stored := range got.Store {
				gotStore = append(gotStore, stored.At)
			}
			if got.Ended != want.ended || got.Started != want.started || got.Updated != want.updated ||
				!got.EndedAt.Equal(want.endedAt) || !got.StartedAt.Equal(want.startedAt) ||
				!sameTrackTimes(gotStore, want.store) || !reflect.DeepEqual(got.Stats, want.stats) {
				t.Fatalf("run %d step %d (frame at %v, hits %d, score %v):\n tracker %+v\n model   %+v", run, i, f.At, len(f.Hits), f.Score, got, want)
			}
			if got.Ended {
				finished("observe", got.EndedAt)
			}
			if got.Started {
				eventStart, eventStored = got.StartedAt, 0
			}
			if eventStored += len(got.Store); eventStored > EventMaxFrames {
				t.Fatalf("run %d step %d: %d frames stored, cap %d", run, i, eventStored, EventMaxFrames)
			}
			if got.Updated && got.Stats.Zones == nil {
				t.Fatalf("run %d step %d: updated stats with nil zones", run, i)
			}
		case roll < 97:
			at := now.Add(time.Duration(r.Intn(50000)-5000) * time.Millisecond)
			gotAt, gotOK := tr.Expire(at)
			wantAt, wantOK := m.expire(at)
			if gotOK != wantOK || !gotAt.Equal(wantAt) {
				t.Fatalf("run %d step %d: expire(%v) = %v %v, model %v %v", run, i, at, gotAt, gotOK, wantAt, wantOK)
			}
			if gotOK {
				finished("expire", gotAt)
			}
		case roll < 99:
			gotAt, gotOK := tr.Close()
			wantAt, wantOK := m.close()
			if gotOK != wantOK || !gotAt.Equal(wantAt) {
				t.Fatalf("run %d step %d: close = %v %v, model %v %v", run, i, gotAt, gotOK, wantAt, wantOK)
			}
			if gotOK {
				finished("close", gotAt)
			}
		default:
			endAfter = endChoices[r.Intn(len(endChoices))]
			tr.SetEndAfter(endAfter)
			m.endAfter = endAfter
		}
		if tr.Open() != m.open {
			t.Fatalf("run %d step %d: open = %v, model %v", run, i, tr.Open(), m.open)
		}
	}
}
