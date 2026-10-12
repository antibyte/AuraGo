package persondetect

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func slotJob(camera string, n byte) frameJob {
	return frameJob{cameraID: camera, jpeg: []byte{n}}
}

func TestFrameSlotsNewestWinsAndRoundRobin(t *testing.T) {
	slots := newFrameSlots()
	slots.setOrder([]string{"a", "b", "c"})
	slots.put(slotJob("a", 1))
	slots.put(slotJob("a", 2))
	slots.put(slotJob("c", 1))
	if slots.queued() != 2 {
		t.Fatalf("queued = %d, want one slot per camera", slots.queued())
	}
	expect := func(camera string, n byte) {
		t.Helper()
		job, ok := slots.take()
		if !ok || job.cameraID != camera || job.jpeg[0] != n {
			t.Fatalf("take = %+v %v, want %s#%d", job, ok, camera, n)
		}
	}
	expect("a", 2)
	expect("c", 1)
	if _, ok := slots.take(); ok {
		t.Fatal("empty slots returned a frame")
	}
	slots.put(slotJob("a", 3))
	slots.put(slotJob("b", 1))
	slots.put(slotJob("c", 2))
	expect("a", 3)
	expect("b", 1)
	expect("c", 2)
	if slots.put(slotJob("x", 1)) {
		t.Fatal("a frame of an unknown camera was queued")
	}
	slots.put(slotJob("b", 2))
	slots.setOrder([]string{"a", "c"})
	if _, ok := slots.take(); ok {
		t.Fatal("frames of removed cameras must be dropped")
	}
}

func TestFetchBackoffDoublesUpToSixtySeconds(t *testing.T) {
	for _, tc := range []struct {
		interval time.Duration
		failures int
		want     time.Duration
	}{
		{2 * time.Second, 0, 2 * time.Second},
		{2 * time.Second, 1, 4 * time.Second},
		{2 * time.Second, 2, 8 * time.Second},
		{2 * time.Second, 5, 60 * time.Second},
		{2 * time.Second, 40, 60 * time.Second},
		{30 * time.Second, 1, 60 * time.Second},
		{0, 1, 4 * time.Second},
	} {
		if got := FetchBackoff(tc.interval, tc.failures); got != tc.want {
			t.Fatalf("FetchBackoff(%v, %d) = %v, want %v", tc.interval, tc.failures, got, tc.want)
		}
	}
}

func TestTimingAverageUsesTheLastTwenty(t *testing.T) {
	var avg timingAverage
	if w, c := avg.average(); w != 0 || c != 0 {
		t.Fatalf("empty average = %v %v", w, c)
	}
	avg.add(10, 20)
	avg.add(30, 40)
	if w, c := avg.average(); w != 20 || c != 30 {
		t.Fatalf("average of two = %v %v", w, c)
	}
	for i := 0; i < 5; i++ {
		avg.add(1000, 1000)
	}
	for i := 0; i < TimingAverageWindow; i++ {
		avg.add(50, 100)
	}
	if w, c := avg.average(); w != 50 || c != 100 {
		t.Fatalf("window average = %v %v, want only the last %d samples", w, c, TimingAverageWindow)
	}
}

func TestAnalysisRateCountsTheLastMinute(t *testing.T) {
	var rate analysisRate
	t0 := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	rate.add(t0)
	rate.add(t0.Add(30 * time.Second))
	rate.add(t0.Add(59 * time.Second))
	if got := rate.perMinute(t0.Add(60 * time.Second)); got != 2 {
		t.Fatalf("perMinute = %v, want 2", got)
	}
	if got := rate.perMinute(t0.Add(3 * time.Minute)); got != 0 {
		t.Fatalf("perMinute after a quiet period = %v", got)
	}
}

func TestFetchBackoffNeverShortensAnInterval(t *testing.T) {
	for _, tc := range []struct {
		interval time.Duration
		failures int
		want     time.Duration
	}{
		{MaxFetchBackoff, 0, MaxFetchBackoff},
		{MaxFetchBackoff, 1, MaxFetchBackoff},
		{90 * time.Second, 0, 90 * time.Second},
		{90 * time.Second, 1, 90 * time.Second},
		{90 * time.Second, 5, 90 * time.Second},
		{time.Duration(1) << 62, 0, time.Duration(1) << 62},
		{time.Duration(1) << 62, 1, time.Duration(1) << 62},
		{time.Duration(1) << 62, 40, time.Duration(1) << 62},
		{time.Duration(math.MaxInt64), 3, time.Duration(math.MaxInt64)},
		{time.Nanosecond, math.MaxInt, MaxFetchBackoff},
	} {
		got := FetchBackoff(tc.interval, tc.failures)
		if got != tc.want {
			t.Fatalf("FetchBackoff(%v, %d) = %v, want %v", tc.interval, tc.failures, got, tc.want)
		}
		if got < tc.interval || got <= 0 {
			t.Fatalf("FetchBackoff(%v, %d) = %v: shorter than the interval or not positive", tc.interval, tc.failures, got)
		}
	}
}

func TestAnalysisRateRestartsAfterABackwardClockStep(t *testing.T) {
	var rate analysisRate
	t0 := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ { // one analysis every 2 s for a minute
		rate.add(t0.Add(time.Duration(2*i) * time.Second))
	}
	end := t0.Add(59 * time.Second)
	if got := rate.perMinute(end); got != 30 {
		t.Fatalf("perMinute before the step = %v, want 30", got)
	}

	// The wall clock jumps back one hour. The analyses from before the step must not
	// be counted as recent: the true rate at the stepped clock is what happens after it.
	stepped := end.Add(-time.Hour)
	rate.add(stepped)
	if got := rate.perMinute(stepped); got != 1 {
		t.Fatalf("perMinute right after the step = %v, want 1 (only the new analysis)", got)
	}
	for i := 1; i < 30; i++ {
		rate.add(stepped.Add(time.Duration(2*i) * time.Second))
	}
	if got := rate.perMinute(stepped.Add(59 * time.Second)); got != 30 {
		t.Fatalf("perMinute a minute after the step = %v, want 30", got)
	}
	if got := rate.perMinute(stepped.Add(59 * time.Minute)); got != 0 {
		t.Fatalf("perMinute after a quiet hour = %v, want 0", got)
	}
}

func TestAnalysisRateKeepsEntriesWithEqualTimestamps(t *testing.T) {
	var rate analysisRate
	t0 := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	rate.add(t0)
	rate.add(t0) // same millisecond: not a clock step
	rate.add(t0.Add(time.Second))
	if got := rate.perMinute(t0.Add(time.Second)); got != 3 {
		t.Fatalf("perMinute = %v, want 3 (equal timestamps are not a backward step)", got)
	}
}

func fillSlots(slots *frameSlots, cameras ...string) {
	for _, id := range cameras {
		slots.put(slotJob(id, 1))
	}
}

func drainSlots(slots *frameSlots) []string {
	var out []string
	for {
		job, ok := slots.take()
		if !ok {
			return out
		}
		out = append(out, job.cameraID)
	}
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFrameSlotsSetOrderKeepsTheFramesOfRemainingCameras(t *testing.T) {
	slots := newFrameSlots()
	slots.setOrder([]string{"a", "b", "c"})
	slots.put(slotJob("a", 11))
	slots.put(slotJob("b", 12))
	slots.put(slotJob("c", 13))
	slots.setOrder([]string{"c", "a", "d"})
	if slots.queued() != 2 {
		t.Fatalf("queued = %d, want the frames of c and a only", slots.queued())
	}
	got := map[string]byte{}
	for {
		job, ok := slots.take()
		if !ok {
			break
		}
		got[job.cameraID] = job.jpeg[0]
	}
	if len(got) != 2 || got["a"] != 11 || got["c"] != 13 {
		t.Fatalf("frames after setOrder = %v, want a#11 and c#13 unchanged", got)
	}
	// A camera that is removed and added again starts empty.
	slots.put(slotJob("a", 21))
	slots.setOrder([]string{"b", "d"})
	slots.setOrder([]string{"a", "b"})
	if slots.queued() != 0 {
		t.Fatalf("queued = %d after removing and re-adding a, want 0", slots.queued())
	}
}

func TestFrameSlotsReorderContinuesAfterTheLastServedCamera(t *testing.T) {
	slots := newFrameSlots()
	slots.setOrder([]string{"a", "b", "c"})
	fillSlots(slots, "a", "b", "c")
	if job, _ := slots.take(); job.cameraID != "a" {
		t.Fatalf("first take = %q, want a", job.cameraID)
	}
	fillSlots(slots, "a")
	slots.setOrder([]string{"c", "a", "b"})
	// a was served last: service continues after a in the new order, a comes last.
	if got := drainSlots(slots); !equalIDs(got, []string{"b", "c", "a"}) {
		t.Fatalf("after reordering to [c a b] the takes are %v, want [b c a]: the camera just served must not be served again first", got)
	}
}

func TestFrameSlotsRemovingACameraKeepsTheServiceOrder(t *testing.T) {
	slots := newFrameSlots()
	slots.setOrder([]string{"a", "b", "c", "d"})
	fillSlots(slots, "a", "b", "c", "d")
	slots.take() // a
	slots.take() // b
	fillSlots(slots, "a", "b")
	slots.setOrder([]string{"b", "c", "d"}) // a is removed, b was served last
	if got := drainSlots(slots); !equalIDs(got, []string{"c", "d", "b"}) {
		t.Fatalf("takes after removing a = %v, want [c d b]: c must not be skipped", got)
	}
}

func TestFrameSlotsRemovedLastServedCameraRestartsFromTheStart(t *testing.T) {
	slots := newFrameSlots()
	slots.setOrder([]string{"a", "b", "c", "d"})
	fillSlots(slots, "a", "b", "c", "d")
	slots.take() // a
	slots.take() // b
	fillSlots(slots, "a")
	slots.setOrder([]string{"a", "c", "d"}) // b, served last, is removed
	if got := drainSlots(slots); !equalIDs(got, []string{"a", "c", "d"}) {
		t.Fatalf("takes after removing the last served camera = %v, want [a c d]", got)
	}
}

func TestFrameSlotsForgetARemovedLastServedCamera(t *testing.T) {
	slots := newFrameSlots()
	slots.setOrder([]string{"a", "b", "c"})
	fillSlots(slots, "a", "b", "c")
	slots.take() // a
	slots.setOrder([]string{"b", "c"})
	slots.setOrder([]string{"b", "a", "c"}) // a is back, but it is no longer remembered as served
	fillSlots(slots, "a")
	if got := drainSlots(slots); !equalIDs(got, []string{"b", "a", "c"}) {
		t.Fatalf("takes = %v, want [b a c]: a removed camera must not be remembered", got)
	}
}

// Every camera with a waiting frame is served within n-1 takes of other cameras,
// whatever the cameras that are always busy do.
func TestFrameSlotsStarvationBound(t *testing.T) {
	order := []string{"a", "b", "c", "d", "e"}
	for rare := range order {
		for warm := 0; warm < 2*len(order); warm++ {
			slots := newFrameSlots()
			slots.setOrder(order)
			busy := func() {
				for i, id := range order {
					if i != rare {
						slots.put(slotJob(id, 1))
					}
				}
			}
			for i := 0; i < warm; i++ {
				busy()
				if _, ok := slots.take(); !ok {
					t.Fatal("no frame while warming up")
				}
			}
			slots.put(slotJob(order[rare], 1))
			waited := 0
			for {
				busy()
				job, ok := slots.take()
				if !ok {
					t.Fatal("no frame")
				}
				if job.cameraID == order[rare] {
					break
				}
				waited++
				if waited > len(order)-1 {
					t.Fatalf("camera %s (position %d, warm-up %d) waited %d takes, more than n-1 = %d", order[rare], rare, warm, waited, len(order)-1)
				}
			}
		}
	}
}

// The producers, the consumer and a reordering goroutine run together (meant for
// -race): a camera's frames are never served out of order, and a poke after a put is
// never lost, in the shape of the service's detection loop.
func TestFrameSlotsConcurrentPutTakeSetOrder(t *testing.T) {
	const puts = 8000
	const final = 1 << 30
	cameras := []string{"a", "b", "c", "d", "e", "f"}
	slots := newFrameSlots()
	slots.setOrder(cameras)
	seqJob := func(camera string, seq int) frameJob {
		return frameJob{cameraID: camera, jpeg: []byte{byte(seq), byte(seq >> 8), byte(seq >> 16), byte(seq >> 24)}}
	}
	seqOf := func(job frameJob) int {
		return int(job.jpeg[0]) | int(job.jpeg[1])<<8 | int(job.jpeg[2])<<16 | int(job.jpeg[3])<<24
	}
	wake := make(chan struct{}, 1)
	poke := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	stop := make(chan struct{})
	consumerDone := make(chan struct{})
	var mu sync.Mutex
	lastSeen := map[string]int{}
	var failed atomic.Bool
	go func() {
		defer close(consumerDone)
		for {
			select {
			case <-stop:
				return
			case <-wake:
				for {
					job, ok := slots.take()
					if !ok {
						break
					}
					seq := seqOf(job)
					mu.Lock()
					prev, seen := lastSeen[job.cameraID]
					if seen && seq <= prev {
						failed.Store(true)
					}
					lastSeen[job.cameraID] = seq
					mu.Unlock()
				}
			}
		}
	}()

	var producers sync.WaitGroup
	for _, id := range cameras {
		producers.Add(1)
		go func(id string) {
			defer producers.Done()
			for seq := 1; seq <= puts; seq++ {
				if slots.put(seqJob(id, seq)) {
					poke()
				}
			}
		}(id)
	}
	producers.Add(1)
	go func() {
		defer producers.Done()
		for i := 0; i < 800; i++ {
			ids := append([]string(nil), cameras...)
			for j := range ids {
				k := (i + j) % len(ids)
				ids[j], ids[k] = ids[k], ids[j]
			}
			slots.setOrder(ids)
			_ = slots.queued()
		}
	}()
	producers.Wait()

	for _, id := range cameras {
		if !slots.put(seqJob(id, final)) {
			t.Fatalf("final frame of %s refused", id)
		}
		poke()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		served := 0
		for _, id := range cameras {
			if lastSeen[id] == final {
				served++
			}
		}
		mu.Unlock()
		if served == len(cameras) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("lost wake-up: only %d of %d final frames served, queued = %d", served, len(cameras), slots.queued())
		}
		time.Sleep(time.Millisecond)
	}
	close(stop)
	<-consumerDone
	if failed.Load() {
		t.Fatal("a camera's frames were served out of order (an older frame after a newer one)")
	}
}
