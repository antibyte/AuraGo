package homedash

import (
	"context"
	"time"
)

// Sink receives a source's output. Implementations never block the source.
type Sink interface {
	// Catalog fully replaces this source's entities.
	Catalog(entities []Entity)
	// Update never blocks; the hub coalesces updates.
	Update(updates ...StateUpdate)
	// Status reports the source's runtime status.
	Status(st SourceStatus)
}

// Source is one adapter behind the hub.
type Source interface {
	ID() SourceID
	// Run runs while the hub has interest and returns on ctx.Done.
	Run(ctx context.Context, sink Sink) error
	// SetInterest is a non-blocking hint of the native ids that are subscribed; poll adapters may ignore it.
	SetInterest(natives []string)
	// Catalog works whether the source is running or not and returns at most 2000 entities.
	Catalog(ctx context.Context) ([]Entity, error)
	// Command must re-check its own integration gates.
	Command(ctx context.Context, cmd Command) (CommandResult, error)
}

// HistorySource is implemented by sources that can serve a numeric history.
type HistorySource interface {
	History(ctx context.Context, native string, from, to time.Time, points int) ([]Sample, error)
}

// SnapshotSource is implemented by sources that can serve a camera snapshot.
type SnapshotSource interface {
	Snapshot(ctx context.Context, native string, maxW, maxH int) (img []byte, contentType string, err error)
}

// Sample is one numeric history point.
type Sample struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

// Factory builds a source from the runtime configuration. Factories must not block: they run
// under the hub lock.
type Factory func(cfg RuntimeConfig) (src Source, enabled bool)

// Command is one device command after the principal and request were resolved.
type Command struct {
	Entity      EntityID
	Native      string
	Name        Capability
	Args        map[string]any
	Principal   Principal
	RequestID   string // client UUID; deduped 30 s per principal
	Confirmed   bool
	DashboardID string
	WidgetID    string
}

// CommandResult is the outcome of an accepted command.
type CommandResult struct {
	Expected   *State `json:"expected,omitempty"`
	Superseded bool   `json:"superseded,omitempty"` // a newer value for the same control replaced this one before it was sent
}
