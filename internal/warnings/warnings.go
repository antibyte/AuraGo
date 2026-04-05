package warnings

import (
	"sync"
	"time"
)

// Severity levels for warnings.
const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

// Category groups for warnings.
const (
	CategorySecurity    = "security"
	CategoryPerformance = "performance"
	CategorySystem      = "system"
)

// Warning represents a single system warning or health issue.
type Warning struct {
	ID           string    `json:"id"`
	Severity     string    `json:"severity"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

// Registry is a thread-safe collection of warnings with deduplication.
type Registry struct {
	mu           sync.RWMutex
	warnings     []Warning
	seen         map[string]bool
	OnNewWarning func(Warning)
}

// NewRegistry creates a new empty warnings registry.
func NewRegistry() *Registry {
	return &Registry{
		seen: make(map[string]bool),
	}
}

// Add inserts a warning into the registry. Duplicates (by ID) are silently ignored.
// If OnNewWarning is set, it is called for genuinely new warnings.
func (r *Registry) Add(w Warning) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.seen[w.ID] {
		return
	}
	if w.Timestamp.IsZero() {
		w.Timestamp = time.Now()
	}
	r.seen[w.ID] = true
	r.warnings = append(r.warnings, w)

	if r.OnNewWarning != nil {
		// Call outside the lock to avoid potential deadlocks with SSE broadcast.
		cb := r.OnNewWarning
		wCopy := w
		go cb(wCopy)
	}
}

// Warnings returns a copy of all warnings.
func (r *Registry) Warnings() []Warning {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Warning, len(r.warnings))
	copy(out, r.warnings)
	return out
}

// Unacknowledged returns only warnings that have not been acknowledged.
func (r *Registry) Unacknowledged() []Warning {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Warning
	for _, w := range r.warnings {
		if !w.Acknowledged {
			out = append(out, w)
		}
	}
	return out
}

// Acknowledge marks a single warning as acknowledged by ID.
func (r *Registry) Acknowledge(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.warnings {
		if r.warnings[i].ID == id {
			r.warnings[i].Acknowledged = true
			return true
		}
	}
	return false
}

// AcknowledgeAll marks all warnings as acknowledged.
func (r *Registry) AcknowledgeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.warnings {
		r.warnings[i].Acknowledged = true
	}
}

// Remove deletes a warning by ID.
func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, w := range r.warnings {
		if w.ID == id {
			r.warnings = append(r.warnings[:i], r.warnings[i+1:]...)
			delete(r.seen, id)
			return true
		}
	}
	return false
}

// Count returns total and unacknowledged warning counts.
func (r *Registry) Count() (total int, unacknowledged int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total = len(r.warnings)
	for _, w := range r.warnings {
		if !w.Acknowledged {
			unacknowledged++
		}
	}
	return
}
