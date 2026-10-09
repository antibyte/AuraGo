package zim

import (
	"container/list"
	"strconv"
	"sync"

	"golang.org/x/sync/singleflight"
)

// DefaultClusterCacheBytes is the decompressed-cluster budget when
// Options.ClusterCacheBytes is 0.
const DefaultClusterCacheBytes = 64 << 20

// clusterOverheadBytes approximates the bookkeeping cost of one cache entry.
const clusterOverheadBytes = 256

// clusterCache is a byte-bounded LRU of decompressed clusters. The mutex only
// guards the map and list; loads run outside it, deduplicated per cluster by
// singleflight.
type clusterCache struct {
	maxBytes int64

	mu    sync.Mutex
	lru   *list.List // front = most recently used
	items map[uint32]*list.Element
	used  int64
	// closed is set by clear: a load that finishes afterwards is served but
	// never re-adds its cluster to the emptied cache.
	closed bool

	group singleflight.Group
}

type cacheEntry struct {
	idx  uint32
	cd   *clusterData
	cost int64
}

func newClusterCache(maxBytes int64) *clusterCache {
	if maxBytes <= 0 {
		maxBytes = DefaultClusterCacheBytes
	}
	return &clusterCache{maxBytes: maxBytes, lru: list.New(), items: make(map[uint32]*list.Element)}
}

// get returns the cached cluster or loads it once, even under concurrent
// callers. Load errors are not cached.
func (c *clusterCache) get(idx uint32, load func() (*clusterData, error)) (*clusterData, error) {
	if cd := c.lookup(idx); cd != nil {
		return cd, nil
	}
	v, err, _ := c.group.Do(strconv.FormatUint(uint64(idx), 10), func() (any, error) {
		if cd := c.lookup(idx); cd != nil {
			return cd, nil
		}
		cd, err := load()
		if err != nil {
			return nil, err
		}
		c.add(idx, cd)
		return cd, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*clusterData), nil
}

func (c *clusterCache) lookup(idx uint32) *clusterData {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[idx]
	if !ok {
		return nil
	}
	c.lru.MoveToFront(el)
	return el.Value.(*cacheEntry).cd
}

func (c *clusterCache) add(idx uint32, cd *clusterData) {
	cost := int64(len(cd.data)) + clusterOverheadBytes
	if cost > c.maxBytes {
		return // larger than the whole budget: serve uncached
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	if el, ok := c.items[idx]; ok {
		c.lru.MoveToFront(el)
		return
	}
	for c.used+cost > c.maxBytes {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		ent := c.lru.Remove(oldest).(*cacheEntry)
		delete(c.items, ent.idx)
		c.used -= ent.cost
	}
	c.items[idx] = c.lru.PushFront(&cacheEntry{idx: idx, cd: cd, cost: cost})
	c.used += cost
}

// clear empties the cache and stops it from caching anything further. It is
// called when the archive is closed; clusters still being loaded are returned
// to their callers but not retained.
func (c *clusterCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lru.Init()
	clear(c.items)
	c.used = 0
	c.closed = true
}

// usage reports the number of cached clusters and their accounted bytes.
func (c *clusterCache) usage() (int, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items), c.used
}
