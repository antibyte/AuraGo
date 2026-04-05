package warnings

import (
	"sync"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	total, unack := r.Count()
	if total != 0 || unack != 0 {
		t.Fatalf("expected 0/0, got %d/%d", total, unack)
	}
}

func TestAdd(t *testing.T) {
	r := NewRegistry()
	w := Warning{ID: "test1", Severity: SeverityWarning, Title: "Test", Description: "desc", Category: CategorySystem}
	r.Add(w)

	total, unack := r.Count()
	if total != 1 || unack != 1 {
		t.Fatalf("expected 1/1, got %d/%d", total, unack)
	}

	all := r.Warnings()
	if len(all) != 1 || all[0].ID != "test1" {
		t.Fatalf("unexpected warnings: %+v", all)
	}
}

func TestAddDedup(t *testing.T) {
	r := NewRegistry()
	w := Warning{ID: "dup", Severity: SeverityInfo, Title: "Dup"}
	r.Add(w)
	r.Add(w)

	total, _ := r.Count()
	if total != 1 {
		t.Fatalf("expected 1 (dedup), got %d", total)
	}
}

func TestAddTimestamp(t *testing.T) {
	r := NewRegistry()
	r.Add(Warning{ID: "ts", Severity: SeverityInfo, Title: "Time"})

	all := r.Warnings()
	if all[0].Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
}

func TestAcknowledge(t *testing.T) {
	r := NewRegistry()
	r.Add(Warning{ID: "a1", Severity: SeverityWarning, Title: "W1"})
	r.Add(Warning{ID: "a2", Severity: SeverityCritical, Title: "W2"})

	if !r.Acknowledge("a1") {
		t.Fatal("Acknowledge returned false for existing warning")
	}
	if r.Acknowledge("nonexistent") {
		t.Fatal("Acknowledge returned true for nonexistent warning")
	}

	total, unack := r.Count()
	if total != 2 || unack != 1 {
		t.Fatalf("expected 2/1, got %d/%d", total, unack)
	}

	un := r.Unacknowledged()
	if len(un) != 1 || un[0].ID != "a2" {
		t.Fatalf("unexpected unacknowledged: %+v", un)
	}
}

func TestAcknowledgeAll(t *testing.T) {
	r := NewRegistry()
	r.Add(Warning{ID: "b1", Severity: SeverityWarning, Title: "W1"})
	r.Add(Warning{ID: "b2", Severity: SeverityCritical, Title: "W2"})

	r.AcknowledgeAll()
	_, unack := r.Count()
	if unack != 0 {
		t.Fatalf("expected 0 unacknowledged, got %d", unack)
	}
}

func TestRemove(t *testing.T) {
	r := NewRegistry()
	r.Add(Warning{ID: "r1", Severity: SeverityInfo, Title: "R"})

	if !r.Remove("r1") {
		t.Fatal("Remove returned false for existing warning")
	}
	if r.Remove("r1") {
		t.Fatal("Remove returned true for already-removed warning")
	}

	total, _ := r.Count()
	if total != 0 {
		t.Fatalf("expected 0 after remove, got %d", total)
	}

	// Can re-add after removal
	r.Add(Warning{ID: "r1", Severity: SeverityInfo, Title: "R again"})
	total, _ = r.Count()
	if total != 1 {
		t.Fatalf("expected 1 after re-add, got %d", total)
	}
}

func TestOnNewWarningCallback(t *testing.T) {
	r := NewRegistry()
	var received []Warning
	var mu sync.Mutex

	r.OnNewWarning = func(w Warning) {
		mu.Lock()
		received = append(received, w)
		mu.Unlock()
	}

	r.Add(Warning{ID: "cb1", Severity: SeverityWarning, Title: "CB"})
	r.Add(Warning{ID: "cb1", Severity: SeverityWarning, Title: "CB"}) // dup — no callback

	// Callback runs in goroutine; wait briefly
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(received))
	}
	if received[0].ID != "cb1" {
		t.Fatalf("expected cb1, got %s", received[0].ID)
	}
}

func TestWarningsReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.Add(Warning{ID: "cp1", Severity: SeverityInfo, Title: "Copy"})

	all := r.Warnings()
	all[0].Title = "Modified"

	orig := r.Warnings()
	if orig[0].Title != "Copy" {
		t.Fatal("Warnings() did not return a copy")
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Add(Warning{ID: "c" + string(rune('a'+n%26)), Severity: SeverityInfo, Title: "Concurrent"})
			r.Count()
			r.Warnings()
			r.Unacknowledged()
		}(i)
	}

	wg.Wait()
	total, _ := r.Count()
	if total == 0 {
		t.Fatal("expected some warnings after concurrent adds")
	}
}
