package homedash

import "context"

// PrefetchCatalogs fetches the stale catalogs of the active sources of ids concurrently
// (CatalogTimeout each, outside the lock), so that a following Subscriber.Set snapshots real
// entities instead of placeholders. It adds no interest and starts no source. Invalid ids and
// unknown or inactive sources are skipped.
func (h *Hub) PrefetchCatalogs(ctx context.Context, ids []EntityID) {
	now := h.opts.Now()
	seen := map[SourceID]bool{}
	var jobs []catalogJob
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	for _, id := range ids {
		src, _, err := ParseEntityID(string(id))
		if err != nil || seen[src] {
			continue
		}
		seen[src] = true
		rt := h.rts[src]
		if rt == nil || !rt.active || rt.src == nil || !h.catalogStale(rt, now) {
			continue
		}
		jobs = append(jobs, h.catalogJobLocked(rt))
	}
	h.mu.Unlock()
	h.fetchCatalogs(ctx, jobs)
}

// LookupEntities returns the cataloged entities among ids with their current state and the hub
// gates applied (as Entity does). Stale catalogs are fetched first (PrefetchCatalogs); unknown ids
// are absent from the map. It never warms or starts a source.
func (h *Hub) LookupEntities(ctx context.Context, ids []EntityID) map[EntityID]Entity {
	h.PrefetchCatalogs(ctx, ids)
	out := make(map[EntityID]Entity, len(ids))
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return out
	}
	for _, id := range ids {
		src, _, err := ParseEntityID(string(id))
		if err != nil {
			continue
		}
		rt := h.rts[src]
		if rt == nil || !rt.active {
			continue
		}
		e, ok := rt.catalog[id]
		if !ok {
			continue
		}
		if st, ok := h.states[id]; ok {
			e.State = st
		}
		out[id] = h.viewLocked(e)
	}
	return out
}
