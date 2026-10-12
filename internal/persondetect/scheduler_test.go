package persondetect

import (
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
