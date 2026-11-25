package homedash

import (
	"context"
	"maps"
	"slices"
	"sort"
)

// SourceCatalog returns the cataloged entities of one active source as the adapter reported them,
// with their current state, sorted by id. The hub's switches (homedash.readonly,
// virtual_desktop.readonly, the source control switch) are NOT applied: capabilities carry the
// adapter's own Allowed and BlockedBy. With fetch, a stale catalog is fetched first (CatalogTimeout,
// outside the lock, through the same path as Entities); without fetch only the cached catalog is
// read. active is false when the source is unknown or not enabled or the hub is closed. It adds no
// interest and never starts the source. The result is a deep copy: callers may modify it freely.
// Templates use it; entity views for clients still go through Entities/Entity.
func (h *Hub) SourceCatalog(ctx context.Context, src SourceID, fetch bool) (list []Entity, status SourceStatus, active bool) {
	now := h.opts.Now()
	h.mu.Lock()
	rt := h.rts[src]
	if rt != nil {
		status = rt.status
	}
	if h.closed || rt == nil || !rt.active || rt.src == nil {
		h.mu.Unlock()
		return []Entity{}, status, false
	}
	var jobs []catalogJob
	if fetch && h.catalogStale(rt, now) {
		jobs = append(jobs, h.catalogJobLocked(rt))
	}
	h.mu.Unlock()
	h.fetchCatalogs(ctx, jobs)

	h.mu.Lock()
	defer h.mu.Unlock()
	rt = h.rts[src]
	if h.closed || rt == nil || !rt.active {
		return []Entity{}, status, false
	}
	list = make([]Entity, 0, len(rt.catalog))
	for id, e := range rt.catalog {
		if st, ok := h.states[id]; ok {
			e.State = st
		}
		list = append(list, cloneEntity(e))
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list, rt.status, true
}

// cloneEntity copies every slice, map and pointer reachable from e.
func cloneEntity(e Entity) Entity {
	caps := make([]CapabilitySpec, len(e.Capabilities))
	for i, c := range e.Capabilities {
		c.Min, c.Max, c.Step = cloneFloat(c.Min), cloneFloat(c.Max), cloneFloat(c.Step)
		c.Options = slices.Clone(c.Options)
		caps[i] = c
	}
	e.Capabilities = caps
	if e.Media != nil {
		m := *e.Media
		e.Media = &m
	}
	if e.State.Attributes != nil {
		e.State.Attributes = cloneAttrs(e.State.Attributes)
	}
	e.State.Value = cloneAny(e.State.Value)
	return e
}

func cloneFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneAttrs(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneAny(v)
	}
	return out
}

// cloneAny copies the JSON-shaped containers a state can carry.
func cloneAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneAttrs(t)
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = cloneAny(x)
		}
		return out
	case []string:
		return slices.Clone(t)
	case map[string]string:
		return maps.Clone(t)
	default:
		return v
	}
}
