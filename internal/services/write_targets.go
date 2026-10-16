package services

import "sync"

type WriteTarget struct {
	Run      string `json:"run"`
	Lineage  string `json:"-"`
	Project  string `json:"project"`
	Resource string `json:"resource"`
	Symbol   string `json:"symbol,omitempty"`
}

// WriteTargets is advisory. Project locks and tool permissions remain authoritative.
type WriteTargets struct {
	mu     sync.Mutex
	active map[string][]WriteTarget
}

func (w *WriteTargets) Note(target WriteTarget) []WriteTarget {
	if w == nil || target.Run == "" || target.Project == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active == nil {
		w.active = map[string][]WriteTarget{}
	}
	var conflicts []WriteTarget
	for run, targets := range w.active {
		if run == target.Run {
			continue
		}
		for _, other := range targets {
			if target.Lineage != "" && target.Lineage == other.Lineage {
				continue
			}
			if target.Project == other.Project && (target.Resource == "" || other.Resource == "" || target.Resource == other.Resource) && (target.Symbol == "" || other.Symbol == "" || target.Symbol == other.Symbol) {
				conflicts = append(conflicts, other)
			}
		}
	}
	for _, old := range w.active[target.Run] {
		if old == target {
			return conflicts
		}
	}
	if len(w.active) < 1024 && len(w.active[target.Run]) < 256 {
		w.active[target.Run] = append(w.active[target.Run], target)
	}
	return conflicts
}

func (w *WriteTargets) Release(run string) {
	if w != nil {
		w.mu.Lock()
		delete(w.active, run)
		w.mu.Unlock()
	}
}
func (w *WriteTargets) Snapshot(project string) []WriteTarget {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	out := []WriteTarget{}
	for _, targets := range w.active {
		for _, t := range targets {
			if t.Project == project {
				out = append(out, t)
			}
		}
	}
	return out
}
func (w *WriteTargets) Clear() {
	if w != nil {
		w.mu.Lock()
		w.active = nil
		w.mu.Unlock()
	}
}
