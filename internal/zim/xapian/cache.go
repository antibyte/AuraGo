package xapian

import (
	"container/list"
	"sync"
)

// defaultCacheBlocks bounds the per-database block cache (4 MiB at 8 KiB blocks).
const defaultCacheBlocks = 512

// blockCache is a small LRU of parsed blocks shared by all cursors of one
// Database. Blocks are immutable, so cached pointers may be shared freely.
type blockCache struct {
	mu    sync.Mutex
	max   int
	items map[uint32]*list.Element
	order *list.List
}

type cacheEntry struct {
	n uint32
	b *block
}

func newBlockCache(max int) *blockCache {
	if max < 16 {
		max = 16
	}
	return &blockCache{max: max, items: make(map[uint32]*list.Element), order: list.New()}
}

func (c *blockCache) get(n uint32) *block {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[n]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*cacheEntry).b
	}
	return nil
}

func (c *blockCache) put(b *block) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[b.n]; ok {
		c.order.MoveToFront(el)
		return
	}
	c.items[b.n] = c.order.PushFront(&cacheEntry{n: b.n, b: b})
	for c.order.Len() > c.max {
		el := c.order.Back()
		c.order.Remove(el)
		delete(c.items, el.Value.(*cacheEntry).n)
	}
}
