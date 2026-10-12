package persondetect

import (
	"reflect"
	"testing"
	"time"
)

func TestTrackerEndedEventResetsTheStartWindow(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.9))
	tr.Observe(trackHit(10*time.Second, 0.6))
	s := tr.Observe(trackHit(40*time.Second, 0.6))
	if !s.Ended || !s.EndedAt.Equal(trackT0.Add(10*time.Second)) || s.Started || tr.Open() {
		t.Fatalf("step = %+v, want the end at 10 s and no start: the hits of the ended event are not part of the next window", s)
	}
	s = tr.Observe(trackHit(42*time.Second, 0.6))
	if !s.Started || !s.StartedAt.Equal(trackT0.Add(40*time.Second)) || len(s.Store) != 2 {
		t.Fatalf("step = %+v, want a start at 40 s from the two new hits", s)
	}
}

func TestTrackerStrongStartDropsHitsOutsideTheWindow(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.6))
	tr.Observe(trackEmpty(2 * time.Second))
	tr.Observe(trackEmpty(4 * time.Second))
	s := tr.Observe(trackHit(6*time.Second, 0.85))
	if !s.Started || len(s.Store) != 1 || !s.StartedAt.Equal(trackT0.Add(6*time.Second)) {
		t.Fatalf("step = started %v store %d startedAt %v, want one stored frame at 6 s: the hit three frames back left the window", s.Started, len(s.Store), s.StartedAt)
	}
}

func TestTrackerIgnoresBoxesWithoutHits(t *testing.T) {
	ignored := func(at time.Duration) AnalyzedFrame {
		return AnalyzedFrame{At: trackT0.Add(at), JPEG: []byte{1}, Boxes: []Box{{X1: 0.1, Y1: 0.1, X2: 0.2, Y2: 0.9, Score: 0.9}}}
	}
	tr := newTracker()
	for i := 0; i < 4; i++ {
		if s := tr.Observe(ignored(time.Duration(i) * time.Second)); s.Started || s.Updated || len(s.Store) != 0 {
			t.Fatalf("boxes without hits, frame %d: step = %+v", i, s)
		}
	}
	if tr.Open() {
		t.Fatal("boxes without hits opened an event")
	}
	if s := tr.Observe(trackHit(10*time.Second, 0.9)); !s.Started {
		t.Fatalf("step = %+v, want a start", s)
	}
	if s := tr.Observe(ignored(20 * time.Second)); s.Updated || len(s.Store) != 0 || s.Ended {
		t.Fatalf("boxes without hits extended the event: %+v", s)
	}
	s := tr.Observe(trackEmpty(40 * time.Second))
	if !s.Ended || !s.EndedAt.Equal(trackT0.Add(10*time.Second)) {
		t.Fatalf("step = %+v, want the end at the last hit (10 s), not at the ignored boxes", s)
	}
}

func TestTrackerCapsStoredFramesWithTwoStartFrames(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.6))
	first := tr.Observe(trackHit(time.Second, 0.6))
	if !first.Started || len(first.Store) != 2 {
		t.Fatalf("step = %+v, want a start storing two frames", first)
	}
	stored := len(first.Store)
	for sec := 2; sec <= 200; sec++ {
		stored += len(tr.Observe(trackHit(time.Duration(sec)*time.Second, 0.6)).Store)
	}
	if stored != EventMaxFrames {
		t.Fatalf("stored %d frames, want %d: the start frames count towards the cap", stored, EventMaxFrames)
	}
}

func TestTrackerBackwardClockStepEndsTheOpenEvent(t *testing.T) {
	startEvent := func() (*tracker, trackStep) {
		tr := newTracker()
		tr.Observe(trackHit(0, 0.6))
		return tr, tr.Observe(trackHit(2*time.Second, 0.6))
	}
	lastHit := trackT0.Add(2 * time.Second)
	t.Run("a person right after the step", func(t *testing.T) {
		tr, start := startEvent()
		if !start.Started {
			t.Fatalf("step = %+v, want a start", start)
		}
		s := tr.Observe(trackHit(2*time.Second-120*time.Second, 0.95))
		if !s.Ended || !s.EndedAt.Equal(lastHit) || !s.Started || !s.StartedAt.Equal(trackT0.Add(-118*time.Second)) || len(s.Store) != 1 {
			t.Fatalf("step = %+v, want the old event ended at its last hit and a new one started at -118 s", s)
		}
		if s.EndedAt.Before(start.StartedAt) {
			t.Fatalf("the event ended at %v before it started at %v", s.EndedAt, start.StartedAt)
		}
	})
	t.Run("empty frames after the step", func(t *testing.T) {
		tr, _ := startEvent()
		base := -118 * time.Second
		ends := 0
		for i := 1; i <= 30; i++ {
			s := tr.Observe(trackEmpty(base + time.Duration(2*i)*time.Second))
			if s.Ended {
				ends++
				if i != 1 || !s.EndedAt.Equal(lastHit) {
					t.Fatalf("frame %d: step = %+v, want the end at the last hit on the first frame after the step", i, s)
				}
			}
		}
		if ends != 1 || tr.Open() {
			t.Fatalf("ends = %d open = %v, want one end and no open event", ends, tr.Open())
		}
		s := tr.Observe(trackHit(base+62*time.Second, 0.95))
		if s.Ended || !s.Started || !s.StartedAt.Equal(trackT0.Add(base+62*time.Second)) {
			t.Fatalf("step = %+v, want a new event for the person after the step", s)
		}
	})
	t.Run("a sweep before the last hit", func(t *testing.T) {
		tr, _ := startEvent()
		at, ok := tr.Expire(trackT0.Add(-60 * time.Second))
		if !ok || !at.Equal(lastHit) || tr.Open() {
			t.Fatalf("expire = %v %v open = %v, want the end at the last hit", at, ok, tr.Open())
		}
	})
}

func TestTrackerEndTimeIsNeverBeforeTheStart(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(100*time.Second, 0.6))
	tr.Observe(trackEmpty(102 * time.Second))
	// The clock steps back: the hit at 100 s is from before the step and no part of this window.
	if s := tr.Observe(trackHit(50*time.Second, 0.6)); s.Started {
		t.Fatalf("step = %+v, want no start from a hit before and a hit after a clock step", s)
	}
	s := tr.Observe(trackHit(52*time.Second, 0.6))
	if !s.Started || !s.StartedAt.Equal(trackT0.Add(50*time.Second)) || len(s.Store) != 2 {
		t.Fatalf("step = %+v, want a start at 50 s from the hits after the step", s)
	}
	at, ok := tr.Close()
	if !ok || at.Before(s.StartedAt) {
		t.Fatalf("close = %v %v, started at %v: an event never ends before it starts", at, ok, s.StartedAt)
	}
}

func TestEventEndAfterForScalesWithTheInterval(t *testing.T) {
	cases := []struct {
		interval time.Duration
		want     time.Duration
	}{
		{-time.Second, EventEndAfter},
		{0, EventEndAfter},
		{2 * time.Second, EventEndAfter},
		{12500 * time.Millisecond, EventEndAfter},
		{12600 * time.Millisecond, 30200 * time.Millisecond},
		{15 * time.Second, 35 * time.Second},
		{30 * time.Second, 65 * time.Second},
	}
	for _, c := range cases {
		if got := EventEndAfterFor(c.interval); got != c.want {
			t.Errorf("EventEndAfterFor(%v) = %v, want %v", c.interval, got, c.want)
		}
	}
}

func TestTrackerSlowCameraKeepsOneEventWhilePersonStaysInView(t *testing.T) {
	tr := newTracker()
	tr.SetEndAfter(EventEndAfterFor(30 * time.Second))
	spacing := 30*time.Second + 300*time.Millisecond
	starts, ends := 0, 0
	for i := 0; i < 20; i++ { // 10 minutes
		s := tr.Observe(trackHit(time.Duration(i)*spacing, 0.6))
		if s.Started {
			starts++
		}
		if s.Ended {
			ends++
		}
	}
	if starts != 1 || ends != 0 || !tr.Open() {
		t.Fatalf("starts = %d ends = %d open = %v, want one event that stays open", starts, ends, tr.Open())
	}
}

func TestTrackerSlowCameraSurvivesOneMissedDetection(t *testing.T) {
	spacing := 15*time.Second + 200*time.Millisecond
	tr := newTracker()
	tr.SetEndAfter(EventEndAfterFor(15 * time.Second))
	tr.Observe(trackHit(0, 0.9))
	tr.Observe(trackEmpty(spacing))
	s := tr.Observe(trackHit(2*spacing, 0.6))
	if s.Ended || s.Started || !tr.Open() {
		t.Fatalf("step = %+v open = %v, want the same event after one missed detection", s, tr.Open())
	}
	// Two missed detections are still the end of the event.
	tr.Observe(trackEmpty(3 * spacing))
	tr.Observe(trackEmpty(4 * spacing))
	s = tr.Observe(trackHit(5*spacing, 0.6))
	if !s.Ended || !s.EndedAt.Equal(trackT0.Add(2*spacing)) {
		t.Fatalf("step = %+v, want the end at the last hit after two missed detections", s)
	}
}

func TestTrackerEndAfterIsConfigurable(t *testing.T) {
	tr := newTracker()
	tr.Observe(trackHit(0, 0.9))
	if _, ok := tr.Expire(trackT0.Add(EventEndAfter)); !ok {
		t.Fatal("a new tracker did not use EventEndAfter")
	}
	tr.Observe(trackHit(100*time.Second, 0.9))
	tr.SetEndAfter(time.Minute)
	if _, ok := tr.Expire(trackT0.Add(159 * time.Second)); ok {
		t.Fatal("expired 59 s after the last hit with an end of 60 s")
	}
	if at, ok := tr.Expire(trackT0.Add(160 * time.Second)); !ok || !at.Equal(trackT0.Add(100*time.Second)) {
		t.Fatalf("expire = %v %v, want the end at the last hit", at, ok)
	}
	tr.SetEndAfter(0)
	tr.Observe(trackHit(200*time.Second, 0.9))
	if _, ok := tr.Expire(trackT0.Add(230 * time.Second)); !ok {
		t.Fatal("SetEndAfter(0) did not restore EventEndAfter")
	}
	// The setting survives the end of an event.
	tr.SetEndAfter(time.Minute)
	tr.Observe(trackHit(300*time.Second, 0.9))
	tr.Observe(trackEmpty(330 * time.Second)) // 30 s after the hit: still the same event
	if !tr.Open() {
		t.Fatal("an event ended after 30 s with an end of 60 s")
	}
	tr.Observe(trackEmpty(360 * time.Second)) // 60 s after the hit: ended
	if tr.Open() {
		t.Fatal("an event stayed open 60 s after its last hit")
	}
	tr.Observe(trackHit(400*time.Second, 0.9))
	if _, ok := tr.Expire(trackT0.Add(450 * time.Second)); ok {
		t.Fatal("the end-after setting was lost when an event ended")
	}
}

func TestTrackerReportsUpdatedOnlyWhenSomethingChanged(t *testing.T) {
	tr := newTracker()
	step := func(at time.Duration, f AnalyzedFrame, wantUpdated bool, wantStored int, what string) trackStep {
		t.Helper()
		f.At = trackT0.Add(at)
		s := tr.Observe(f)
		if s.Updated != wantUpdated || len(s.Store) != wantStored {
			t.Fatalf("%s: updated = %v stored = %d, want %v and %d", what, s.Updated, len(s.Store), wantUpdated, wantStored)
		}
		return s
	}
	step(0, trackHit(0, 0.8, "Porch"), true, 1, "start")
	step(time.Second, trackHit(0, 0.7, "Porch"), false, 0, "lower score, same zone")
	step(2*time.Second, trackHit(0, 0.8, "Porch"), false, 0, "equal score")
	if s := step(3*time.Second, trackHit(0, 0.9, "Porch"), true, 0, "better score"); s.Stats.MaxScore != 0.9 {
		t.Fatalf("stats = %+v", s.Stats)
	}
	if s := step(4*time.Second, trackHit(0, 0.5, "Driveway"), true, 0, "new zone"); !reflect.DeepEqual(s.Stats.Zones, []string{"Driveway", "Porch"}) {
		t.Fatalf("stats = %+v", s.Stats)
	}
	two := trackHit(0, 0.5, "Porch")
	two.Hits = append(two.Hits, two.Hits[0])
	if s := step(4500*time.Millisecond, two, true, 0, "second person"); s.Stats.MaxPersons != 2 {
		t.Fatalf("stats = %+v", s.Stats)
	}
	step(5*time.Second, trackHit(0, 0.5, "Porch"), true, 1, "a stored frame changes the frame count")
	step(6*time.Second, trackHit(0, 0.5, "Porch"), false, 0, "nothing new")
	step(7*time.Second, trackEmpty(0), false, 0, "frame without hit")
}

func TestTrackerPendingBufferDoesNotRetainDroppedFrames(t *testing.T) {
	tr := newTracker()
	hit := trackHit(0, 0.6)
	hit.JPEG = []byte{1, 2, 3}
	tr.Observe(hit)
	tr.Observe(trackEmpty(2 * time.Second))
	tr.Observe(trackEmpty(4 * time.Second))
	tr.Observe(trackEmpty(6 * time.Second)) // the hit at 0 s has left the three-frame window
	if len(tr.pending) != 0 {
		t.Fatalf("pending = %d entries, want none", len(tr.pending))
	}
	for i, p := range tr.pending[:cap(tr.pending)] {
		if p.frame.JPEG != nil || p.frame.Boxes != nil || p.frame.Hits != nil {
			t.Fatalf("dropped entry %d still references its frame", i)
		}
	}
}

func TestTrackerStepBackInsideTheNoHitTailIsAClockBreak(t *testing.T) {
	// The event's last hit is at 0 s and empty frames follow up to 28 s. A step back of 20 s
	// then reads 30 s as 10 s: that is later than the last hit, so only the previous frame
	// (28 s) shows the step.
	const stepBack = 20 * time.Second
	openWithTail := func() *tracker {
		tr := newTracker()
		tr.Observe(trackHit(0, 0.9))
		for s := 2; s <= 28; s += 2 {
			tr.Observe(trackEmpty(time.Duration(s) * time.Second))
		}
		return tr
	}
	t.Run("empty frames, then a new person", func(t *testing.T) {
		tr := openWithTail()
		ends := 0
		for real := 30; real <= 44; real += 2 {
			s := tr.Observe(trackEmpty(time.Duration(real)*time.Second - stepBack))
			if s.Ended {
				ends++
				if real != 30 || !s.EndedAt.Equal(trackT0) {
					t.Fatalf("real %d s: step = %+v, want the end at the last hit on the first frame after the step", real, s)
				}
			}
		}
		if ends != 1 || tr.Open() {
			t.Fatalf("ends = %d open = %v, want one end and no open event", ends, tr.Open())
		}
		s := tr.Observe(trackHit(46*time.Second-stepBack, 0.95))
		if s.Ended || !s.Started || !s.StartedAt.Equal(trackT0.Add(26*time.Second)) {
			t.Fatalf("step = %+v, want a new event for the person 46 real seconds after the last hit", s)
		}
	})
	t.Run("a person on the first frame after the step", func(t *testing.T) {
		tr := openWithTail()
		s := tr.Observe(trackHit(30*time.Second-stepBack, 0.95))
		if !s.Ended || !s.EndedAt.Equal(trackT0) || !s.Started || !s.StartedAt.Equal(trackT0.Add(10*time.Second)) || len(s.Store) != 1 {
			t.Fatalf("step = %+v, want the old event ended at 0 s and a new one started at 10 s", s)
		}
	})
}

func TestTrackerStartWindowIsBoundedByAge(t *testing.T) {
	cases := []struct {
		name      string
		endAfter  time.Duration // 0 keeps the default
		gap       time.Duration
		between   bool // an empty frame between the hits
		wantStart bool
	}{
		{"within the end time", 0, 29999 * time.Millisecond, false, true},
		{"exactly the end time", 0, EventEndAfter, false, false},
		{"minutes apart", 0, 600 * time.Second, false, false},
		{"an empty frame in between", 0, 36 * time.Second, true, false},
		{"a longer end time admits 60 s", time.Duration(65) * time.Second, 60 * time.Second, false, true},
		{"a longer end time has a limit too", time.Duration(65) * time.Second, 65 * time.Second, false, false},
		{"the default end time rejects 60 s", 0, 60 * time.Second, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := newTracker()
			if c.endAfter > 0 {
				tr.SetEndAfter(c.endAfter)
			}
			tr.Observe(trackHit(0, 0.6))
			if c.between {
				tr.Observe(trackEmpty(c.gap / 2))
			}
			s := tr.Observe(trackHit(c.gap, 0.6))
			if s.Started != c.wantStart {
				t.Fatalf("hits %v apart: step = %+v, want started = %v", c.gap, s, c.wantStart)
			}
			if c.wantStart {
				if len(s.Store) != 2 || !s.StartedAt.Equal(trackT0) {
					t.Fatalf("step = %+v, want a start at 0 s storing both hits", s)
				}
				return
			}
			// The late hit is the first of a new window: the next one starts the event, without the old hit.
			s = tr.Observe(trackHit(c.gap+time.Second, 0.6))
			if !s.Started || len(s.Store) != 2 || !s.StartedAt.Equal(trackT0.Add(c.gap)) {
				t.Fatalf("step = %+v, want a start at %v storing the two recent hits", s, c.gap)
			}
		})
	}
}

func TestTrackerStrongStartDropsHitsOlderThanTheEndTime(t *testing.T) {
	cases := []struct {
		gap        time.Duration
		wantStored int
	}{
		{40 * time.Second, 1},
		{EventEndAfter, 1},
		{EventEndAfter - time.Millisecond, 2},
	}
	for _, c := range cases {
		tr := newTracker()
		tr.Observe(trackHit(0, 0.6))
		s := tr.Observe(trackHit(c.gap, 0.85))
		wantStart := trackT0.Add(c.gap)
		if c.wantStored == 2 {
			wantStart = trackT0
		}
		if !s.Started || len(s.Store) != c.wantStored || !s.StartedAt.Equal(wantStart) {
			t.Fatalf("strong hit %v after a weak one: started %v store %d startedAt %v, want %d stored from %v", c.gap, s.Started, len(s.Store), s.StartedAt, c.wantStored, wantStart)
		}
	}
}
