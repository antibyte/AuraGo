package homedash_test

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func TestSubscriberSetReportsDuplicateRejectionOnce(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, nil, fa)
	sub, err := h.Attach(session("s1"))
	if err != nil {
		t.Fatal(err)
	}
	acc, rej, err := sub.Set(ids("bad id", "bad id", "fa:x", "fa:x"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(acc, ids("fa:x")) {
		t.Errorf("accepted %v", acc)
	}
	if want := []homedash.Rejection{{ID: "bad id", Reason: homedash.RejectInvalidID}}; !slices.Equal(rej, want) {
		t.Errorf("rejected %v, want %v", rej, want)
	}
}

func TestSubscriberSetPublishesOneSnapshotBatch(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, nil, fa)
	sub, err := h.Attach(session("s1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sub.Set(ids("fa:a", "fa:b", "fa:c")); err != nil {
		t.Fatal(err)
	}
	b := sub.Drain()
	if len(b.Snapshot) != 3 {
		t.Fatalf("snapshot has %d entities, want 3", len(b.Snapshot))
	}
	if len(b.Sources) != 1 {
		t.Errorf("sources %v", b.Sources)
	}
}

func TestSubscriberSetSnapshotIsNeverSplitByDrain(t *testing.T) {
	fa := sourcetest.New("fa")
	h := newTestHub(t, nil, fa)
	const n = 20
	for round := 0; round < 30; round++ {
		sub, err := h.Attach(session(fmt.Sprintf("s%d", round)))
		if err != nil {
			t.Fatal(err)
		}
		list := make([]string, n)
		for i := range list {
			list[i] = fmt.Sprintf("fa:e%d", i)
		}
		stop := make(chan struct{})
		var wg sync.WaitGroup
		var mu sync.Mutex
		var sizes []int
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if b := sub.Drain(); len(b.Snapshot) > 0 {
					mu.Lock()
					sizes = append(sizes, len(b.Snapshot))
					mu.Unlock()
				}
				select {
				case <-stop:
					return
				default:
				}
			}
		}()
		if _, _, err := sub.Set(ids(list...)); err != nil {
			t.Fatal(err)
		}
		close(stop)
		wg.Wait()
		if b := sub.Drain(); len(b.Snapshot) > 0 {
			sizes = append(sizes, len(b.Snapshot))
		}
		if len(sizes) != 1 || sizes[0] != n {
			t.Fatalf("round %d: snapshot batch sizes %v, want [%d]", round, sizes, n)
		}
		sub.Close()
	}
}
