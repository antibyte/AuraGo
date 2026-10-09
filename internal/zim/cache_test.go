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
	if _, err := c.get(1, load); err != nil || loads != 2 {
		t.Fatalf("after clear: loads = %d, err = %v, want a fresh load", loads, err)
	}
}

func TestNewClusterCacheDefaultBudget(t *testing.T) {
	if got := newClusterCache(0).maxBytes; got != DefaultClusterCacheBytes {
		t.Fatalf("maxBytes = %d, want %d", got, DefaultClusterCacheBytes)
	}
}
