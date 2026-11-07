package gamemaker

import (
	"strings"
	"testing"
)

func TestCompactRuntimeReferenceKeepsChecksAndPickupFeedback(t *testing.T) {
	got := CompactRuntimeReference(phaserRuntimeReference, 2000)
	if len(got) > 2000 {
		t.Fatalf("compact reference exceeds its budget: %d bytes", len(got))
	}
	for _, required := range []string{"feedback('pickup'", "Checks:", "this.state:", "pickup_events"} {
		if !strings.Contains(got, required) {
			t.Fatalf("compact reference dropped %q", required)
		}
	}
	if !strings.Contains(got, compactRuntimeReferenceTail) {
		t.Fatal("compact reference did not name the omitted lines")
	}
	for _, line := range strings.Split(strings.TrimSuffix(got, compactRuntimeReferenceTail), "\n") {
		if !strings.Contains(phaserRuntimeReference, line) {
			t.Fatalf("compact reference invented or mangled a line: %q", line)
		}
	}
}

func TestCompactRuntimeReferenceKeepsCompleteShortReference(t *testing.T) {
	short := "Entry: short.\nChecks: score."
	if got := CompactRuntimeReference(short, 2000); got != short {
		t.Fatalf("short reference changed: %q", got)
	}
}

func TestCompactRuntimeReferenceNeverSplitsALine(t *testing.T) {
	long := strings.Repeat("x", 40) + "\nChecks: feedback('pickup', item) counts."
	const budget = len(compactRuntimeReferenceTail) + 80
	got := CompactRuntimeReference(long, budget)
	if len(got) > budget {
		t.Fatalf("tiny budget exceeded: %d bytes", len(got))
	}
	if !strings.Contains(got, "Checks: feedback('pickup', item) counts.") {
		t.Fatal("priority line was lost to a preceding long line")
	}
}
