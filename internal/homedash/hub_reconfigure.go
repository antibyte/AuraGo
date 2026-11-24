package homedash

import "time"

func (h *Hub) reconfigureLoop() {
	defer h.wg.Done()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.reconf:
			h.applyConfig()
		}
	}
}

func gatesChanged(a, b RuntimeConfig) bool {
	if a.ReadOnly != b.ReadOnly || a.DesktopReadOnly != b.DesktopReadOnly {
		return true
	}
	// Compare the union of both key sets: a source that vanishes, appears or changes its gate inputs
	// changes controlBlock for its entities.
	for id, nb := range b.Sources {
		na, ok := a.Sources[id]
		if !ok || na.Control != nb.Control || na.ConfigKey != nb.ConfigKey {
			return true
		}
	}
	for id := range a.Sources {
		if _, ok := b.Sources[id]; !ok {
			return true
		}
	}
	return false
}

// applyConfig diffs the new snapshot by fingerprint, rebuilds only changed sources (waiting for the
// old run outside the lock), restarts them on interest and re-fetches subscribed catalogs.
func (h *Hub) applyConfig() {
	h.mu.Lock()
	if h.closed || h.pending == nil {
		h.mu.Unlock()
		return
	}
	cfg, old := *h.pending, h.cfg
	h.pending, h.cfg = nil, cfg
	if h.rec != nil {
		h.rec.resize(cfg.History.RingPoints, cfg.History.RingEntities, time.Duration(cfg.History.SampleSeconds)*time.Second)
	}
	var notes []*interestNote
	for _, s := range h.subs {
		switch {
		case !cfg.active():
			notes = append(notes, h.detachLocked(s, CloseDisabled)...)
		case s.p.Kind == PrincipalKiosk && !cfg.KioskEnabled:
			notes = append(notes, h.detachLocked(s, CloseKioskDisabled)...)
		}
	}
	type change struct {
		rt   *sourceRT
		done chan struct{}
	}
	var changed []change
	for _, id := range h.order {
		rt, set := h.rts[id], cfg.Sources[id]
		want, fp := cfg.active() && set.Enabled, set.Fingerprint()
		if rt.fp == fp && rt.active == want && (rt.src != nil || !want) {
			continue
		}
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
			rt.idleGen++
		}
		stopRecheckLocked(rt)
		done := h.stopLocked(rt)
		// A rebuilt source drops its old catalog and cached states; subscribers keep their interest and
		// receive fresh data once the new catalog arrives.
		rt.src, rt.active, rt.fp, rt.catalogAt = nil, false, fp, time.Time{}
		if len(rt.catalog) > 0 && want {
			if rt.dropped == nil {
				rt.dropped = map[EntityID]struct{}{}
			}
			for eid := range rt.catalog {
				rt.dropped[eid] = struct{}{}
			}
		}
		rt.catalog, rt.failedFrom = nil, SourceStatus{}
		h.dropSourceStatesLocked(id)
		if want {
			if src, ok := callFactory(h.factories[id], cfg); ok && src != nil {
				rt.src, rt.active = src, true
			}
		}
		state := StatusDisabled
		if rt.active {
			state = StatusIdle
		} else {
			rt.dropped = nil // inactive: the disable path owns its subscribers
		}
		h.setStatusLocked(rt, state, "")
		changed = append(changed, change{rt, done})
	}
	if gatesChanged(old, cfg) {
		for _, s := range h.subs {
			for id := range s.set {
				src, _, _ := ParseEntityID(string(id))
				if rt := h.rts[src]; rt != nil {
					if e, ok := rt.catalog[id]; ok {
						e.State = h.states[id]
						s.pushEntity(h.viewLocked(e))
					}
				}
			}
		}
	}
	h.mu.Unlock()
	h.deliver(notes)
	for _, c := range changed {
		if c.done != nil {
			select {
			case <-c.done:
			case <-time.After(10 * time.Second):
				h.log.Warn("homedash: source did not stop within 10 s", "source", c.rt.id)
			}
		}
	}
	h.mu.Lock()
	notes = nil
	var jobs []catalogJob
	for _, c := range changed {
		if !c.rt.active {
			continue
		}
		notes = append(notes, h.reconcileLocked(c.rt))
		if len(c.rt.natives) > 0 {
			jobs = append(jobs, h.catalogJobLocked(c.rt))
		}
	}
	h.mu.Unlock()
	h.deliver(notes)
	h.fetchCatalogs(h.ctx, jobs)
}
