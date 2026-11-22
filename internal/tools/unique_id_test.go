package tools

import (
	"testing"
	"time"
)

func TestFreeNanoIDSkipsTakenIDs(t *testing.T) {
	now := time.Unix(0, 100)
	taken := map[string]bool{"job_100": true, "job_101": true, "job_103": true}
	isTaken := func(id string) bool { return taken[id] }

	if got := freeNanoID("job_", now, isTaken); got != "job_102" {
		t.Fatalf("freeNanoID with taken prefix range = %q, want job_102", got)
	}
	if got := freeNanoID("other_", now, isTaken); got != "other_100" {
		t.Fatalf("freeNanoID without collision = %q, want other_100", got)
	}
}

func TestProcessUniqueNanoIncreasesWhenClockRepeats(t *testing.T) {
	// Windows clocks can return the same instant for back-to-back calls.
	frozen := time.Unix(1_700_000_000, 0)
	previous := int64(0)
	for i := 0; i < 5; i++ {
		got := processUniqueNano(frozen)
		if got < frozen.UnixNano() {
			t.Fatalf("call #%d = %d, want at least %d", i, got, frozen.UnixNano())
		}
		if got <= previous {
			t.Fatalf("call #%d = %d, want more than %d", i, got, previous)
		}
		previous = got
	}
}
