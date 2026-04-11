package agent

import "aurago/internal/prompts"

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

func (c *tokenCountCache) Count(text string) int {
	if text == "" {
		return 0
	}
	if v, ok := c.counts[text]; ok {
		return v
	}
	v := prompts.CountTokens(text)
	c.counts[text] = v
	c.order = append(c.order, text)

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
