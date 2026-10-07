package flows

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeClock is a manually advanced Clock for tests.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeWaiter
}

type fakeWaiter struct {
	at time.Time
	ch chan time.Time
}

func newFakeClock(now time.Time) *fakeClock { return &fakeClock{now: now} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- c.now
		return ch
	}
	c.waiters = append(c.waiters, fakeWaiter{at: c.now.Add(d), ch: ch})
	return ch
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	kept := c.waiters[:0]
	for _, w := range c.waiters {
		if !w.at.After(c.now) {
			w.ch <- c.now
			continue
		}
		kept = append(kept, w)
	}
	clear(c.waiters[len(kept):]) // drop the stale tail so fired channels can be collected
	c.waiters = kept
}

// pending counts the After channels that have not fired yet. It also counts
// abandoned ones: a Sleep cancelled through its context leaves its channel here
// until Advance passes its deadline.
func (c *fakeClock) pending() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.waiters)
}

// WaitForWaiters blocks until at least n After channels are pending (see
// pending). Abandoned channels of cancelled sleeps count too, so after a
// cancelled sleep a wait for the next sleeper can be satisfied by a stale entry;
// advance the clock past the abandoned deadline first when that matters.
func (c *fakeClock) WaitForWaiters(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for c.pending() < n {
		if time.Now().After(deadline) {
			t.Fatalf("expected %d clock waiters, have %d", n, c.pending())
		}
		time.Sleep(time.Millisecond)
	}
}

// newTestRegistry returns a registry with the logic nodes and small test node types.
func newTestRegistry(t *testing.T) *Registry {
	t.Helper()
	reg := NewRegistry()
	if err := RegisterLogicNodes(reg); err != nil {
		t.Fatalf("RegisterLogicNodes: %v", err)
	}
	echo := func(_ context.Context, in ExecInput) (ExecResult, error) {
		return ExecResult{Output: map[string]any{"value": in.Params["value"]}}, nil
	}
	reg.MustRegister(&NodeDef{Type: "test.trigger", Category: "trigger", Trigger: true})
	reg.MustRegister(&NodeDef{Type: "test.untrusted_trigger", Category: "trigger", Trigger: true, UntrustedOutput: true})
	reg.MustRegister(&NodeDef{Type: "test.echo", Category: "test",
		Params:       []ParamSpec{{Name: "value", Kind: ParamText, Templatable: true}},
		OutputFields: []FieldSpec{{Name: "value", Type: "any", Primary: true}},
		Execute:      echo})
	reg.MustRegister(&NodeDef{Type: "test.required", Category: "test",
		Params: []ParamSpec{
			{Name: "text", Kind: ParamText, Required: true},
			{Name: "mode", Kind: ParamSelect, Default: "a"},
			{Name: "extra", Kind: ParamText, Required: true, VisibleIf: &Visibility{Param: "mode", Equals: []string{"b"}}},
		},
		Execute: echo})
	reg.MustRegister(&NodeDef{Type: "test.fail", Category: "test",
		Execute: func(context.Context, ExecInput) (ExecResult, error) {
			return ExecResult{}, NewNodeError("TEST_FAILED", "boom")
		}})
	reg.MustRegister(&NodeDef{Type: "test.sink", Category: "test",
		Params:  []ParamSpec{{Name: "command", Kind: ParamText, Templatable: true, SensitiveSink: true}},
		Effects: []Effect{EffectRunsCode},
		Execute: echo})
	reg.MustRegister(&NodeDef{Type: "test.web", Category: "test", UntrustedOutput: true,
		Execute: func(context.Context, ExecInput) (ExecResult, error) {
			return ExecResult{Output: map[string]any{"text": "<web>"}}, nil
		}})
	reg.MustRegister(&NodeDef{Type: "test.unavailable", Category: "test",
		AvailabilityFunc: func() Availability { return Availability{State: NeedsSetupState, ConfigSection: "telegram"} },
		Execute:          echo})
	return reg
}

// testNodeID returns a valid, deterministic node id for index i.
func testNodeID(i int) string {
	return "n_aaaaa" + string(rune('a'+(i/676)%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+i%26))
}

type flowBuilder struct {
	f *Flow
}

func newFlow(name string) *flowBuilder {
	return &flowBuilder{f: &Flow{Schema: SchemaVersion, ID: "flow_test", Kind: KindFlow, Name: name}}
}

// node adds a node and returns its id. The key doubles as label.
func (b *flowBuilder) node(key, typ string, params map[string]any) string {
	id := testNodeID(len(b.f.Nodes) + 1)
	b.f.Nodes = append(b.f.Nodes, Node{ID: id, Key: key, Type: typ, TypeVersion: 1, Label: key, Params: params})
	return id
}

// edge connects from:port to the default input of to.
func (b *flowBuilder) edge(from, port, to string) string {
	id := fmt.Sprintf("e_%d", len(b.f.Edges)+1)
	b.f.Edges = append(b.f.Edges, Edge{ID: id, Source: PortRef{Node: from, Port: port}, Target: PortRef{Node: to, Port: PortIn}})
	return id
}

func (b *flowBuilder) build() *Flow {
	b.f.Normalize()
	return b.f
}
