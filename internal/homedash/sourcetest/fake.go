// Package sourcetest provides a scripted homedash.Source for hub tests.
package sourcetest

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"aurago/internal/homedash"
)

// FakeSource is a scripted homedash.Source.
type FakeSource struct {
	Panics        int           // the first Panics runs panic
	CommandGate   chan struct{} // when set, Command waits until it is closed
	CommandErr    error
	CommandResult homedash.CommandResult

	id                       homedash.SourceID
	mu                       sync.Mutex
	catalog                  []homedash.Entity
	sink                     homedash.Sink
	runs, running, inCommand int
	interest                 []string
	commands                 []homedash.Command
}

// New builds a fake source with a fixed catalog.
func New(id homedash.SourceID, entities ...homedash.Entity) *FakeSource {
	return &FakeSource{id: id, catalog: entities}
}

func (f *FakeSource) ID() homedash.SourceID { return f.id }

func (f *FakeSource) Run(ctx context.Context, sink homedash.Sink) error {
	f.mu.Lock()
	f.runs++
	n := f.runs
	f.running++
	f.sink = sink
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.running--
		if f.sink == sink {
			f.sink = nil
		}
		f.mu.Unlock()
	}()
	if n <= f.Panics {
		panic("sourcetest: scripted panic")
	}
	sink.Status(homedash.SourceStatus{Source: f.id, State: homedash.StatusLive})
	<-ctx.Done()
	return nil
}

func (f *FakeSource) SetInterest(natives []string) {
	f.mu.Lock()
	f.interest = slices.Clone(natives)
	f.mu.Unlock()
}

func (f *FakeSource) Catalog(context.Context) ([]homedash.Entity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.catalog), nil
}

func (f *FakeSource) Command(ctx context.Context, cmd homedash.Command) (homedash.CommandResult, error) {
	f.mu.Lock()
	f.commands = append(f.commands, cmd)
	f.inCommand++
	gate := f.CommandGate
	f.mu.Unlock()
	defer func() { f.mu.Lock(); f.inCommand--; f.mu.Unlock() }()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return homedash.CommandResult{}, ctx.Err()
		}
	}
	return f.CommandResult, f.CommandErr
}

// Push sends updates through the running sink; false when the source is not running.
func (f *FakeSource) Push(updates ...homedash.StateUpdate) bool {
	f.mu.Lock()
	sink := f.sink
	f.mu.Unlock()
	if sink == nil {
		return false
	}
	sink.Update(updates...)
	return true
}

func (f *FakeSource) Runs() int      { f.mu.Lock(); defer f.mu.Unlock(); return f.runs }
func (f *FakeSource) Running() int   { f.mu.Lock(); defer f.mu.Unlock(); return f.running }
func (f *FakeSource) InCommand() int { f.mu.Lock(); defer f.mu.Unlock(); return f.inCommand }
func (f *FakeSource) Interest() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.interest)
}
func (f *FakeSource) Commands() []homedash.Command {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.commands)
}

// Entity builds a catalog entity whose native id is also its name.
func Entity(id homedash.EntityID, kind homedash.Kind, caps ...homedash.CapabilitySpec) homedash.Entity {
	src, native, _ := homedash.ParseEntityID(string(id))
	if caps == nil {
		caps = []homedash.CapabilitySpec{}
	}
	return homedash.Entity{ID: id, Source: src, Kind: kind, Name: native, Capabilities: caps, State: homedash.State{Available: true}}
}

// RecordingSink records what a source sends (adapter tests).
type RecordingSink struct {
	mu       sync.Mutex
	Catalogs [][]homedash.Entity
	Updates  []homedash.StateUpdate
	Statuses []homedash.SourceStatus
}

func (r *RecordingSink) Catalog(e []homedash.Entity) {
	r.mu.Lock()
	r.Catalogs = append(r.Catalogs, e)
	r.mu.Unlock()
}

func (r *RecordingSink) Update(u ...homedash.StateUpdate) {
	r.mu.Lock()
	r.Updates = append(r.Updates, u...)
	r.mu.Unlock()
}

func (r *RecordingSink) Status(s homedash.SourceStatus) {
	r.mu.Lock()
	r.Statuses = append(r.Statuses, s)
	r.mu.Unlock()
}

// WaitFor polls cond every 5 ms until it holds or timeout passes.
func WaitFor(t testing.TB, timeout time.Duration, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
