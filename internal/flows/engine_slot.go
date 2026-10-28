package flows

import (
	"context"
	"sync"
)

// SlotParker lets a node that only waits give its run's global slot back (ExecInput.Slot; nil
// without a runner slot, for example when Engine.Execute is called directly). Park gives the slot
// back as soon as no other node of the run is busy; Unpark takes one again, in turn behind runs
// that waited before, and returns the run's context error when the run ends first (cancel,
// shutdown, its time limit). A node that parks must unpark before it returns.
type SlotParker interface {
	Park()
	Unpark() error
}

// slotGate is a run's SlotParker. active counts launched nodes that are neither parked nor
// received by the coordinator, parked the nodes between Park and Unpark. The run releases its
// slot only while active is 0 and a node is parked, and takes it again in the first Unpark. mu
// guards the counters and held; acq serializes Unparks that wait for a slot, so a run never asks
// for two. Lock order: mu, then Runner.mu (through Release).
type slotGate struct {
	slot        RunSlot
	ctx         context.Context
	acq         sync.Mutex
	mu          sync.Mutex
	active      int
	parked      int
	held        bool
	dispatching bool // Ready siblings have not all been registered yet.
}

// newSlotGate returns nil without a slot; every method copes with a nil gate.
func newSlotGate(slot RunSlot, ctx context.Context) *slotGate {
	if slot == nil {
		return nil
	}
	return &slotGate{slot: slot, ctx: ctx, held: true, dispatching: true}
}

func (g *slotGate) launched() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.active++
	g.mu.Unlock()
}

func (g *slotGate) received() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.dispatching = true
	g.active--
	g.mu.Unlock()
}

// idle runs on the coordinator before it waits: a run whose busy nodes all wait releases its slot.
func (g *slotGate) idle() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.dispatching = false
	g.releaseLocked()
	g.mu.Unlock()
}

func (g *slotGate) releaseLocked() {
	if g.held && !g.dispatching && g.active == 0 && g.parked > 0 && g.slot.Release() {
		g.held = false
	}
}

func (g *slotGate) Park() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	g.parked++
	g.releaseLocked()
}

func (g *slotGate) Unpark() error {
	// Busy again before the node queues behind acq: while another woken node holds acq and
	// finishes, this one must keep the run from giving its slot back and asking for it a second time.
	g.mu.Lock()
	g.parked--
	g.active++
	g.mu.Unlock()
	g.acq.Lock()
	defer g.acq.Unlock()
	g.mu.Lock()
	if g.held {
		g.mu.Unlock()
		return nil
	}
	g.mu.Unlock()
	if err := g.slot.Acquire(g.ctx); err != nil {
		return err
	}
	g.mu.Lock()
	g.held = true
	g.mu.Unlock()
	return nil
}
