package persondetect

import (
	"reflect"
	"testing"
	"time"
)

var trackT0 = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func trackHit(at time.Duration, score float64, zones ...string) AnalyzedFrame {
	b := Box{X1: 0.4, Y1: 0.3, X2: 0.5, Y2: 0.9, Score: score}
	return AnalyzedFrame{At: trackT0.Add(at), Boxes: []Box{b}, Hits: []Box{b}, Zones: zones, Score: score}
}

func trackEmpty(at time.Duration) AnalyzedFrame {
	return AnalyzedFrame{At: trackT0.Add(at)}
}

func TestTrackerStartsOnTwoHitsWithinThreeFrames(t *testing.T) {
	tr := newTracker()
	if s := tr.Observe(trackHit(0, 0.6)); s.Started {
		t.Fatal("one weak hit must not start an event")
	}
	if s := tr.Observe(trackEmpty(2 * time.Second)); s.Started {
		t.Fatal("a frame without hit started an event")
	}
	s := tr.Observe(trackHit(4*time.Second, 0.65))
	if !s.Started || !s.StartedAt.Equal(trackT0) || len(s.Store) != 2 {
		t.Fatalf("step = %+v, want a start at t0 storing both hits", s)
	}
	if !s.Store[0].At.Equal(trackT0) || !s.Store[1].At.Equal(trackT0.Add(4*time.Second)) {
		t.Fatalf("stored frames at %v and %v", s.Store[0].At, s.Store[1].At)
	}
	if s.Stats.MaxScore != 0.65 || s.Stats.MaxPersons != 1 || !tr.Open() {
		t.Fatalf("stats = %+v open = %v", s.Stats, tr.Open())
	}
}

func TestTrackerWeakHitsOutsideTheWindowDoNotStart(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.6))
	tr.Observe(trackEmpty(2 * time.Second))
	tr.Observe(trackEmpty(4 * time.Second))
	if s := tr.Observe(trackHit(6*time.Second, 0.6)); s.Started {
		t.Fatal("hits four frames apart started an event")
	}
	s := tr.Observe(trackHit(8*time.Second, 0.6))
	if !s.Started || len(s.Store) != 2 || !s.Store[0].At.Equal(trackT0.Add(6*time.Second)) {
		t.Fatalf("step = %+v, want a start storing the hits at 6 s and 8 s only", s)
	}
}

func TestTrackerSingleStrongHitStarts(t *testing.T) {
	if s := newTracker().Observe(trackHit(0, 0.8)); !s.Started || len(s.Store) != 1 {
		t.Fatalf("score 0.8 = %+v, want a start", s)
	}
	if s := newTracker().Observe(trackHit(0, 0.79)); s.Started {
		t.Fatal("score 0.79 alone must not start an event")
	}
}

func TestTrackerThrottlesAndCapsStoredFrames(t *testing.T) {
	tr := newTracker()
	stored := len(tr.Observe(trackHit(0, 0.9)).Store)
	for sec := 1; sec <= 200; sec++ {
		s := tr.Observe(trackHit(time.Duration(sec)*time.Second, 0.7))
		if len(s.Store) > 0 && sec%5 != 0 {
			t.Fatalf("frame stored at %d s, want at most one per 5 s", sec)
		}
		stored += len(s.Store)
	}
	if stored != EventMaxFrames {
		t.Fatalf("stored %d frames, want %d", stored, EventMaxFrames)
	}
}

func TestTrackerEndsThirtySecondsAfterTheLastHit(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.9))
	tr.Observe(trackHit(10*time.Second, 0.7))
	if s := tr.Observe(trackEmpty(39 * time.Second)); s.Ended {
		t.Fatal("ended after 29 s without a hit")
	}
	s := tr.Observe(trackEmpty(40 * time.Second))
	if !s.Ended || !s.EndedAt.Equal(trackT0.Add(10*time.Second)) || tr.Open() {
		t.Fatalf("step = %+v, want the end at the last hit (10 s)", s)
	}
	if s := tr.Observe(trackHit(42*time.Second, 0.6)); s.Started {
		t.Fatal("hits of the ended event helped start a new one")
	}
}

func TestTrackerLateHitEndsAndStartsAnew(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.9))
	s := tr.Observe(trackHit(60*time.Second, 0.9))
	if !s.Ended || !s.EndedAt.Equal(trackT0) || !s.Started || !s.StartedAt.Equal(trackT0.Add(60*time.Second)) {
		t.Fatalf("step = %+v, want end at t0 and a new start at 60 s", s)
	}
}

func TestTrackerAggregatesZonesAndPersons(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.9, "Porch"))
	two := trackHit(time.Second, 0.7, "Driveway")
	two.Hits = append(two.Hits, two.Hits[0])
	s := tr.Observe(two)
	if !s.Updated || s.Stats.MaxPersons != 2 || s.Stats.MaxScore != 0.9 || !reflect.DeepEqual(s.Stats.Zones, []string{"Driveway", "Porch"}) {
		t.Fatalf("stats = %+v", s.Stats)
	}
}

func TestTrackerExpireAndClose(t *testing.T) {
	tr := newTracker()
	if _, ok := tr.Close(); ok {
		t.Fatal("closing a tracker without an event reported one")
	}
	tr.Observe(trackHit(0, 0.9))
	tr.Observe(trackHit(3*time.Second, 0.9))
	if _, ok := tr.Expire(trackT0.Add(32 * time.Second)); ok {
		t.Fatal("expired 29 s after the last hit")
	}
	at, ok := tr.Expire(trackT0.Add(33 * time.Second))
	if !ok || !at.Equal(trackT0.Add(3*time.Second)) {
		t.Fatalf("expire = %v %v", at, ok)
	}
	tr.Observe(trackHit(40*time.Second, 0.9))
	if at, ok := tr.Close(); !ok || !at.Equal(trackT0.Add(40*time.Second)) {
		t.Fatalf("close = %v %v", at, ok)
	}
}

func TestTrackerStatsZonesAreNeverNil(t *testing.T) {
	s := newTracker().Observe(trackHit(0, 0.9))
	if !s.Started || s.Stats.Zones == nil || len(s.Stats.Zones) != 0 {
		t.Fatalf("zones of an event without zones = %#v, want an empty non-nil list", s.Stats.Zones)
	}
}
