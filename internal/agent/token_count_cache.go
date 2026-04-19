package agent

import (
	"aurago/internal/prompts"
	"hash/fnv"
	"strconv"
)

type tokenCountCache struct {
	maxEntries int
	order      []string
	counts     map[string]int
}

func newTokenCountCache(maxEntries int) *tokenCountCache {
	if maxEntries <= 0 {
		maxEntries = 2048
	}
	return &tokenCountCache{
		maxEntries: maxEntries,
		counts:     make(map[string]int, maxEntries),
	}
}

// cacheHashKey produces a compact, collision-resistant hash key from text and model.
// This avoids storing the full text as a map key, which caused significant memory
// bloat when caching token counts for large tool outputs (50-100KB+).
func cacheHashKey(text, model string) string {
	h := fnv.New64a()
	h.Write([]byte(text))
	h.Write([]byte(model))
	return strconv.FormatUint(h.Sum64(), 36)
}

func (c *tokenCountCache) Count(text, model string) int {
	if text == "" {
		return 0
	}
	key := cacheHashKey(text, model)
	if v, ok := c.counts[key]; ok {
		return v
	}
	v := prompts.CountTokensForModel(text, model)
	c.counts[key] = v
	c.order = append(c.order, key)

	// Coarse eviction: when the cache grows too large, drop the oldest half.
	if len(c.counts) > c.maxEntries {
		drop := len(c.order) / 2
		for i := 0; i < drop; i++ {
			delete(c.counts, c.order[i])
		}
		c.order = c.order[drop:]
	}
	return v
}
