package xapian

import (
	"container/list"
	"sync"
)

// defaultCacheBytes bounds the block data cached per Database: 512 blocks at
// libzim's 8 KiB block size, 64 at the 64 KiB maximum.
const defaultCacheBytes = 4 << 20

// blockCache is a small LRU of parsed blocks shared by all cursors of one
// Database. Blocks are immutable, so cached pointers may be shared freely.
// max counts blocks; Open derives it from defaultCacheBytes and the block size.
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
	if max < 16 { // a root-to-leaf path is at most 10 blocks
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
