package homedash

import (
	"math"
	"testing"
	"time"
)

func TestHistoryRingDownsamplesAndEvictsLRU(t *testing.T) {
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	hs := newHistoryStore(4, 2, time.Minute)
	hs.observe("fa:a", 9, base) // no ring yet: ignored
	hs.touch("fa:a", base)
	for i, v := range []float64{1, 2, 3} {
		hs.observe("fa:a", v, base.Add(time.Duration(i)*20*time.Second))
	}
	if got := hs.series("fa:a", base, base.Add(time.Hour), 100); len(got) != 1 || got[0].V != 3 {
		t.Fatalf("one bucket, last value wins: %+v", got)
	}
	for i := 1; i <= 6; i++ {
		hs.observe("fa:a", float64(i*10), base.Add(time.Duration(i)*time.Minute))
	}
	got := hs.series("fa:a", base, base.Add(time.Hour), 100)
	if len(got) != 4 || got[0].V != 30 || got[3].V != 60 {
		t.Fatalf("ring keeps the last 4 points: %+v", got)
	}
	if got := hs.series("fa:a", base, base.Add(time.Hour), 2); len(got) != 2 || got[0].V != 35 || got[1].V != 55 {
		t.Fatalf("downsampled: %+v", got)
	}
	hs.touch("fa:b", base.Add(10*time.Minute))
	hs.touch("fa:c", base.Add(11*time.Minute))
	if got := hs.series("fa:a", base, base.Add(time.Hour), 10); got != nil {
		t.Fatal("least recently used ring not evicted")
	}
}

func TestHistoryRingRejectsNonFiniteAndDegenerateConfig(t *testing.T) {
	base := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	hs := newHistoryStore(4, 2, time.Minute)
	hs.touch("fa:a", base)
	hs.observe("fa:a", math.NaN(), base)
	hs.observe("fa:a", math.Inf(1), base.Add(time.Minute))
	hs.observe("fa:a", math.Inf(-1), base.Add(2*time.Minute))
	if got := hs.series("fa:a", base, base.Add(time.Hour), 10); len(got) != 0 {
		t.Fatalf("non-finite samples stored: %+v", got)
	}
	zero := newHistoryStore(0, 0, time.Minute) // must neither loop nor store
	zero.touch("fa:a", base)
	zero.observe("fa:a", 1, base)
	if got := zero.series("fa:a", base, base.Add(time.Hour), 10); got != nil {
		t.Fatalf("zero-sized store kept a ring: %+v", got)
	}
}
