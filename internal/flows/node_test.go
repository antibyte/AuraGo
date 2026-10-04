package flows

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	def := &NodeDef{Type: "x.a", Category: "x"}
	if err := reg.Register(def); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if def.Version != 1 {
		t.Fatalf("Register must default Version to 1, got %d", def.Version)
	}
	if err := reg.Register(&NodeDef{Type: "x.a"}); err == nil {
		t.Fatal("duplicate type must fail")
	}
	if err := reg.Register(&NodeDef{}); err == nil {
		t.Fatal("empty type must fail")
	}
	if err := reg.Register(nil); err == nil {
		t.Fatal("nil definition must fail")
	}
	reg.MustRegister(&NodeDef{Type: "x.b"})
	if _, ok := reg.Lookup("x.b"); !ok {
		t.Fatal("Lookup x.b failed")
	}
	var types []string
	for _, d := range reg.All() {
		types = append(types, d.Type)
	}
	if !reflect.DeepEqual(types, []string{"x.a", "x.b"}) {
		t.Fatalf("All() = %v", types)
	}
	reg.Replace(&NodeDef{Type: "x.a", Category: "y"})
	if d, _ := reg.Lookup("x.a"); d.Category != "y" {
		t.Fatalf("Replace did not replace: %+v", d)
	}
	if n := reg.RemoveWhere(func(d *NodeDef) bool { return d.Category == "y" }); n != 1 {
		t.Fatalf("RemoveWhere removed %d, want 1", n)
	}
	if _, ok := reg.Lookup("x.a"); ok {
		t.Fatal("x.a must be removed")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustRegister of a duplicate must panic")
		}
	}()
	reg.MustRegister(&NodeDef{Type: "x.b"})
}

func TestNodeDefPorts(t *testing.T) {
	trigger := &NodeDef{Type: "t", Trigger: true}
	if trigger.InputPorts() != nil {
		t.Fatal("triggers have no inputs")
	}
	plain := &NodeDef{Type: "p"}
	if !reflect.DeepEqual(plain.InputPorts(), []string{PortIn}) || !reflect.DeepEqual(plain.OutputPorts(nil), []string{PortOut}) {
		t.Fatal("plain node ports")
	}
	errNode := &Node{Settings: NodeSettings{OnError: ErrorPort}}
	if !reflect.DeepEqual(plain.OutputPorts(errNode), []string{PortOut, PortError}) {
		t.Fatalf("error port = %v", plain.OutputPorts(errNode))
	}
	if plain.DefaultPort(errNode) != PortOut {
		t.Fatal("default port must skip the error port")
	}
	branch := &NodeDef{Type: "b", Outputs: []string{PortTrue, PortFalse}}
	if branch.DefaultPort(&Node{}) != PortTrue {
		t.Fatal("default port of a branch is its first port")
	}
	dyn := &NodeDef{Type: "d", OutputsFunc: func(*Node) []string { return []string{"case_1", PortDefault} }}
	if !reflect.DeepEqual(dyn.OutputPorts(&Node{}), []string{"case_1", PortDefault}) {
		t.Fatal("OutputsFunc ignored")
	}
	stop := &NodeDef{Type: "s", Outputs: []string{}}
	if len(stop.OutputPorts(&Node{})) != 0 || stop.DefaultPort(&Node{}) != "" {
		t.Fatal("a node without outputs has no default port")
	}
}

func TestOutputPortsDoesNotWriteIntoSharedSlices(t *testing.T) {
	shared := make([]string, 1, 4)
	shared[0] = "case_1"
	def := &NodeDef{Type: "d", OutputsFunc: func(*Node) []string { return shared }}
	errNode := &Node{Settings: NodeSettings{OnError: ErrorPort}}
	if got := def.OutputPorts(errNode); !reflect.DeepEqual(got, []string{"case_1", PortError}) {
		t.Fatalf("OutputPorts = %v", got)
	}
	if got := shared[:2]; got[1] != "" {
		t.Fatalf("OutputPorts wrote into the slice returned by OutputsFunc: %v", got)
	}
}

func TestRegistryConcurrentUse(t *testing.T) {
	reg := NewRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			typ := fmt.Sprintf("c.%d", i)
			for j := 0; j < 100; j++ {
				reg.Replace(&NodeDef{Type: typ, Category: "c"})
				reg.Lookup(typ)
				reg.All()
				reg.RemoveWhere(func(d *NodeDef) bool { return d.Type == typ })
			}
		}(i)
	}
	wg.Wait()
	if n := len(reg.All()); n != 0 {
		t.Fatalf("registry must be empty, has %d definitions", n)
	}
}

func TestNodeDefTimeoutAvailabilityEffects(t *testing.T) {
	def := &NodeDef{Type: "t"}
	if def.Timeout(&Node{}) != DefaultNodeTimeout {
		t.Fatal("default timeout")
	}
	def.DefaultTimeout = time.Minute
	if def.Timeout(&Node{}) != time.Minute {
		t.Fatal("definition timeout")
	}
	if def.Timeout(&Node{Settings: NodeSettings{TimeoutSeconds: 5}}) != 5*time.Second {
		t.Fatal("node timeout overrides")
	}
	if def.Availability().State != AvailableState {
		t.Fatal("default availability")
	}
	def.AvailabilityFunc = func() Availability { return Availability{State: BlockedState, Reason: "readonly"} }
	if def.Availability().State != BlockedState {
		t.Fatal("AvailabilityFunc ignored")
	}
	def.Effects = []Effect{EffectSendsMessage}
	if !reflect.DeepEqual(def.EffectsOf(&Node{}), []Effect{EffectSendsMessage}) {
		t.Fatal("static effects")
	}
	def.EffectsFunc = func(*Node) []Effect { return []Effect{EffectDeletes} }
	if !reflect.DeepEqual(def.EffectsOf(&Node{}), []Effect{EffectDeletes}) {
		t.Fatal("EffectsFunc ignored")
	}
	def.OutputFields = []FieldSpec{{Name: "static"}}
	if got := def.FieldsOf(&Node{}); len(got) != 1 || got[0].Name != "static" {
		t.Fatalf("static fields = %+v", got)
	}
	def.OutputFieldsFunc = func(n *Node) []FieldSpec { return []FieldSpec{{Name: Stringify(n.Params["f"])}} }
	if got := def.FieldsOf(&Node{Params: map[string]any{"f": "dyn"}}); len(got) != 1 || got[0].Name != "dyn" {
		t.Fatalf("dynamic fields = %+v", got)
	}
}

func TestAsNodeError(t *testing.T) {
	if ne := asNodeError(NewNodeError("X", "bad %d", 1)); ne.Code != "X" || ne.Message != "bad 1" {
		t.Fatalf("NodeError passthrough = %+v", ne)
	}
	if ne := asNodeError(context.DeadlineExceeded); ne.Code != "FLOW_NODE_TIMEOUT" {
		t.Fatalf("deadline = %+v", ne)
	}
	if ne := asNodeError(errors.New("boom")); ne.Code != "FLOW_NODE_FAILED" || ne.Message != "boom" {
		t.Fatalf("plain error = %+v", ne)
	}
	if NewNodeError("X", "m").Error() != "X: m" {
		t.Fatal("Error() format")
	}
}

func TestServicesSleepAndDefaults(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC))
	svc := &Services{Clock: clock, Location: time.UTC}
	done := make(chan error, 1)
	go func() { done <- svc.Sleep(context.Background(), time.Minute) }()
	clock.WaitForWaiters(t, 1)
	clock.Advance(time.Minute)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Sleep: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Sleep did not return")
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { done <- svc.Sleep(ctx, time.Hour) }()
	clock.WaitForWaiters(t, 1)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Sleep = %v", err)
	}
	if err := svc.Sleep(ctx, 0); err != nil {
		t.Fatalf("zero Sleep = %v", err)
	}
	if svc.Now() != clock.Now() || svc.Loc() != time.UTC {
		t.Fatal("Now/Loc must use the configured clock and location")
	}
	var empty *Services
	if empty.Now().IsZero() || empty.Loc() != time.Local {
		t.Fatal("nil Services must fall back to the real clock and time.Local")
	}
}

func TestRunStatusTerminal(t *testing.T) {
	for _, s := range []RunStatus{RunSuccess, RunError, RunCancelled} {
		if !s.Terminal() {
			t.Errorf("%s must be terminal", s)
		}
	}
	for _, s := range []RunStatus{RunQueued, RunRunning, RunWaiting} {
		if s.Terminal() {
			t.Errorf("%s must not be terminal", s)
		}
	}
}
