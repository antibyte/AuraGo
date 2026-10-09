package zim

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fakeCluster(size int) *clusterData { return &clusterData{data: make([]byte, size)} }

func TestClusterCacheEvictsLeastRecentlyUsed(t *testing.T) {
	c := newClusterCache(2 * (1000 + clusterOverheadBytes))
	loads := map[uint32]int{}
	load := func(idx uint32) func() (*clusterData, error) {
		return func() (*clusterData, error) { loads[idx]++; return fakeCluster(1000), nil }
	}
	for _, idx := range []uint32{1, 2, 1, 3, 1, 2} {
		if _, err := c.get(idx, load(idx)); err != nil {
			t.Fatal(err)
		}
	}
	// 1 stays hot; 2 is evicted by 3 and loaded again at the end.
	if loads[1] != 1 || loads[2] != 2 || loads[3] != 1 {
		t.Fatalf("loads = %v, want 1:1 2:2 3:1", loads)
	}
	if n, used := c.usage(); n != 2 || used != 2*(1000+clusterOverheadBytes) {
		t.Fatalf("usage = %d clusters / %d bytes", n, used)
	}
}

func TestClusterCacheServesOversizedClustersUncached(t *testing.T) {
	c := newClusterCache(100)
	var loads atomic.Int32
	for range 2 {
		if _, err := c.get(7, func() (*clusterData, error) { loads.Add(1); return fakeCluster(1000), nil }); err != nil {
			t.Fatal(err)
		}
	}
	if loads.Load() != 2 {
		t.Fatalf("loads = %d, want 2 (oversized cluster must not be cached)", loads.Load())
	}
}

func TestClusterCacheDoesNotCacheErrors(t *testing.T) {
	c := newClusterCache(0)
	boom := errors.New("boom")
	if _, err := c.get(1, func() (*clusterData, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want boom", err)
	}
	cd, err := c.get(1, func() (*clusterData, error) { return fakeCluster(10), nil })
	if err != nil || cd == nil {
		t.Fatalf("retry = %v, %v", cd, err)
	}
}

func TestClusterCacheSharesConcurrentLoads(t *testing.T) {
	c := newClusterCache(0)
	var loads atomic.Int32
	release := make(chan struct{})
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			_, err := c.get(5, func() (*clusterData, error) {
				loads.Add(1)
				<-release
				return fakeCluster(10), nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if loads.Load() != 1 {
		t.Fatalf("loads = %d, want 1", loads.Load())
	}
}

// Loads of different clusters run in parallel only up to
// maxConcurrentClusterLoads; the others wait for a slot and then load too.
func TestClusterCacheBoundsConcurrentLoads(t *testing.T) {
	c := newClusterCache(0)
	const callers = 4 * maxConcurrentClusterLoads
	var running, peak, loads atomic.Int32
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := range callers {
		wg.Go(func() {
			_, err := c.get(uint32(i), func() (*clusterData, error) {
				now := running.Add(1)
				for {
					old := peak.Load()
					if now <= old || peak.CompareAndSwap(old, now) {
						break
					}
				}
				<-release
				running.Add(-1)
				loads.Add(1)
				return fakeCluster(10), nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	deadline := time.Now().Add(5 * time.Second)
	for running.Load() < maxConcurrentClusterLoads && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	// Give any caller that ignores the bound time to start its load.
	time.Sleep(50 * time.Millisecond)
	if got := running.Load(); got != maxConcurrentClusterLoads {
		t.Fatalf("running loads = %d, want %d", got, maxConcurrentClusterLoads)
	}
	close(release)
	wg.Wait()
	if got := peak.Load(); got != maxConcurrentClusterLoads {
		t.Fatalf("peak concurrent loads = %d, want %d", got, maxConcurrentClusterLoads)
	}
	if got := loads.Load(); got != callers {
		t.Fatalf("loads = %d, want %d (every waiting caller must load eventually)", got, callers)
	}
	if got := len(c.loadSlots); got != 0 {
		t.Fatalf("occupied load slots after all loads = %d, want 0", got)
	}
}

// A failing or panicking load gives its slot back.
func TestClusterCacheReleasesLoadSlotsOnFailure(t *testing.T) {
	c := newClusterCache(0)
	boom := errors.New("boom")
	for i := range 2 * maxConcurrentClusterLoads {
		if _, err := c.get(uint32(i), func() (*clusterData, error) { return nil, boom }); !errors.Is(err, boom) {
			t.Fatalf("error = %v, want boom", err)
		}
	}
	for i := range 2 * maxConcurrentClusterLoads {
		func() {
			defer func() { _ = recover() }()
			_, _ = c.get(uint32(100+i), func() (*clusterData, error) { panic("load panicked") })
		}()
	}
	if got := len(c.loadSlots); got != 0 {
		t.Fatalf("occupied load slots = %d, want 0", got)
	}
	if _, err := c.get(1000, func() (*clusterData, error) { return fakeCluster(10), nil }); err != nil {
		t.Fatalf("load after failures = %v", err)
	}
}

func TestClusterCacheClearDropsEntries(t *testing.T) {
	c := newClusterCache(0)
	var loads int
	load := func() (*clusterData, error) { loads++; return fakeCluster(10), nil }
	for range 2 {
		if _, err := c.get(1, load); err != nil {
			t.Fatal(err)
		}
	}
	if n, used := c.usage(); loads != 1 || n != 1 || used != 10+clusterOverheadBytes {
		t.Fatalf("before clear: loads = %d, usage = %d clusters / %d bytes", loads, n, used)
	}
	c.clear()
	if n, used := c.usage(); n != 0 || used != 0 {
		t.Fatalf("after clear: usage = %d clusters / %d bytes, want empty", n, used)
	}
	// A closed cache still serves clusters but no longer retains them.
	for range 2 {
		if _, err := c.get(1, load); err != nil {
			t.Fatal(err)
		}
	}
	if n, used := c.usage(); loads != 3 || n != 0 || used != 0 {
		t.Fatalf("after clear: loads = %d, usage = %d clusters / %d bytes, want 3 uncached loads", loads, n, used)
	}
}

func TestClusterCacheDoesNotRetainLoadFinishingAfterClear(t *testing.T) {
	c := newClusterCache(0)
	started := make(chan struct{})
	release := make(chan struct{})
	type result struct {
		cd  *clusterData
		err error
	}
	done := make(chan result, 1)
	go func() {
		cd, err := c.get(9, func() (*clusterData, error) {
			close(started)
			<-release
			return fakeCluster(100), nil
		})
		done <- result{cd, err}
	}()
	<-started
	c.clear() // the archive is closed while the load is in flight
	close(release)
	res := <-done
	if res.err != nil || res.cd == nil || len(res.cd.data) != 100 {
		t.Fatalf("in-flight load = %v, %v; the caller must still get its cluster", res.cd, res.err)
	}
	if n, used := c.usage(); n != 0 || used != 0 {
		t.Fatalf("usage after clear = %d clusters / %d bytes, want empty (late load must not be re-added)", n, used)
	}
}

func TestNewClusterCacheDefaultBudget(t *testing.T) {
	if got := newClusterCache(0).maxBytes; got != DefaultClusterCacheBytes {
		t.Fatalf("maxBytes = %d, want %d", got, DefaultClusterCacheBytes)
	}
}
