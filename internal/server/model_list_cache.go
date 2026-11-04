package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

const maxModelListBytes = 8 << 20

// One handler owns both the public setup route and the authenticated browser.
type modelListCache struct {
	parent  context.Context
	client  *http.Client
	mu      sync.Mutex
	body    []byte
	status  int
	expires time.Time
	pending chan struct{}
}

func (c *modelListCache) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.mu.Lock()
	if !time.Now().Before(c.expires) {
		if c.pending == nil {
			c.pending = make(chan struct{})
			go c.refresh()
		}
		pending := c.pending
		c.mu.Unlock()
		select {
		case <-r.Context().Done():
			return
		case <-pending:
		}
		c.mu.Lock()
	}
	body, status := c.body, c.status
	c.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func (c *modelListCache) refresh() {
	ctx, cancel := context.WithTimeout(c.parent, 15*time.Second)
	defer cancel()
	body := []byte(`{"available":false,"reason":"Failed to fetch a valid OpenRouter model list"}`)
	status, ttl := http.StatusBadGateway, 30*time.Second
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models", nil)
	if err == nil {
		resp, err := c.client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK && resp.ContentLength <= maxModelListBytes {
				data, err := io.ReadAll(io.LimitReader(resp.Body, maxModelListBytes+1))
				if err == nil && len(data) <= maxModelListBytes && json.Valid(data) && ctx.Err() == nil {
					body, status, ttl = data, http.StatusOK, 15*time.Minute
				}
			}
		}
	}
	c.mu.Lock()
	c.body, c.status, c.expires = body, status, time.Now().Add(ttl)
	close(c.pending)
	c.pending = nil
	c.mu.Unlock()
}
