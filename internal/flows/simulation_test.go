package flows

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type diffCountSummary struct {
	FirstPublish                                         bool
	AddedNodes, RemovedNodes, ChangedNodes, ChangedEdges int
}

func TestSimulationFixturesAndTemplate(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Fixture safety")
	start := b.node("start", TypeTriggerManual, nil)
	prev := start
	fixtures := map[string]SimulationFixture{}
	for i, typ := range []string{TypeHTTPRequest, TypeFileWrite, TypeTelegram, TypeHomeAssistant, TypeAIStep, TypeWait} {
		key := []string{"http", "file", "notify", "home", "ai", "wait"}[i]
		id := b.node(key, typ, nil)
		b.edge(prev, PortOut, id)
		fixtures[id] = SimulationFixture{Output: map[string]any{}}
		prev = id
	}
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	spy := &fakeTools{}
	s.services.Tools = spy
	result, err := s.Simulate(ctx, rec.ID, SimulationRequest{Fixtures: fixtures})
	if err != nil || result.Status != RunSuccess || spy.count() != 0 {
		t.Fatalf("fixture simulation: %+v %v calls=%d", result, err, spy.count())
	}
	for id := range fixtures {
		bad := map[string]SimulationFixture{id: {Ports: []string{"missing"}}}
		if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{Fixtures: bad}); !errors.Is(err, ErrInvalidSimulationFixture) {
			t.Fatalf("invalid port %s: %v", id, err)
		}
	}
	httpID := b.f.Nodes[1].ID
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{Fixtures: map[string]SimulationFixture{httpID: {Output: map[string]any{"status_code": "not a number"}}}}); !errors.Is(err, ErrInvalidSimulationFixture) {
		t.Fatalf("invalid output type: %v", err)
	}
	for _, info := range DescribeTemplates(s.reg) {
		if info.ID == "local_first_result" && (!info.LocalOnly || !info.EffectsKnown || len(info.Effects) != 0) {
			t.Fatalf("local metadata: %+v", info)
		}
	}
	local, err := TemplateFlow("local_first_result", func(k string) string { return k })
	if err != nil {
		t.Fatal(err)
	}
	localRec, err := s.CreateFlow(ctx, CreateRequest{Import: local})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	localResult, err := s.Simulate(ctx, localRec.ID, SimulationRequest{At: at})
	if err != nil || localResult.Status != RunSuccess || len(localResult.Steps) != 3 || !localResult.StartedAt.Equal(at) {
		t.Fatalf("local recipe: %+v %v", localResult, err)
	}
}

func TestSimulationFileFixtures(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("File fixture")
	start := b.node("start", TypeTriggerManual, nil)
	pdf := b.node("pdf", TypePDFCreate, map[string]any{"content": "fixture"})
	b.edge(start, PortOut, pdf)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	spy := &fakeTools{}
	s.services.Tools = spy
	for _, tc := range []struct {
		name  string
		file  any
		valid bool
	}{
		{"file reference", FileRef("/simulated/report.pdf", "", "", "/files/report.pdf", 123), true},
		{"path object", map[string]any{"path": "/simulated/report.pdf"}, true},
		{"path string", "/simulated/report.pdf", true},
		{"missing path", map[string]any{"$type": "file"}, false},
		{"wrong path type", map[string]any{"path": 123}, false},
		{"wrong marker", map[string]any{"path": "report.pdf", "$type": "other"}, false},
		{"wrong metadata type", map[string]any{"path": "report.pdf", "mime": false}, false},
		{"negative size", map[string]any{"path": "report.pdf", "size": -1}, false},
		{"fractional size", map[string]any{"path": "report.pdf", "size": 1.5}, false},
		{"wrong value type", []any{"report.pdf"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := s.Simulate(ctx, rec.ID, SimulationRequest{Fixtures: map[string]SimulationFixture{
				pdf: {Output: map[string]any{"file": tc.file}},
			}})
			if !tc.valid {
				if !errors.Is(err, ErrInvalidSimulationFixture) {
					t.Fatalf("expected invalid fixture, got %+v, %v", res, err)
				}
				return
			}
			if err != nil || res.Status != RunSuccess {
				t.Fatalf("file fixture: %+v, %v", res, err)
			}
		})
	}
	if spy.count() != 0 {
		t.Fatalf("simulation invoked tools %d times", spy.count())
	}
}

func TestDiffDetailsRedactsAndBounds(t *testing.T) {
	live := simpleFlow("before")
	draft, _ := live.Clone()
	draft.Nodes[1].Params["headers"] = map[string]any{"Authorization": "Bearer sensitive"}
	draft.Nodes[1].Params["url"] = "https://user:sensitive@example.com/sensitive?token=sensitive#sensitive"
	draft.Nodes[1].Params["password"] = "sensitive"
	draft.Nodes[1].Params["note"] = "Bearer sensitive"
	d := DiffFlows(live, draft)
	encoded, err := json.Marshal(d)
	if err != nil || strings.Contains(string(encoded), "sensitive") || !strings.Contains(string(encoded), "https://example.com") || len(d.Details) == 0 {
		t.Fatalf("diff leaked a secret: %s, %v", encoded, err)
	}
}

func diffCounts(d DiffSummary) diffCountSummary {
	return diffCountSummary{d.FirstPublish, d.AddedNodes, d.RemovedNodes, d.ChangedNodes, d.ChangedEdges}
}

func TestSimulationIsIsolated(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	ctx := context.Background()
	b := newFlow("Simulation")
	start := b.node("start", TypeTriggerManual, nil)
	web := b.node("web", TypeHTTPRequest, map[string]any{"url": "https://example.com/"})
	set := b.node("result", TypeSet, map[string]any{"fields": []any{map[string]any{"name": "answer", "value": "{{web.body}}"}}})
	b.edge(start, PortOut, web)
	b.edge(web, PortOut, set)
	rec, err := s.CreateFlow(ctx, CreateRequest{Import: b.build()})
	if err != nil {
		t.Fatal(err)
	}
	var called atomic.Int32
	for _, typ := range []string{TypeHTTPRequest, TypeSet} {
		def, _ := s.reg.Lookup(typ)
		copyDef := *def
		copyDef.Execute = func(context.Context, ExecInput) (ExecResult, error) { called.Add(1); return ExecResult{}, nil }
		copyDef.OutputsFunc = func(*Node) []string { called.Add(1); return []string{PortOut} }
		s.reg.Replace(&copyDef)
	}
	missing, err := s.Simulate(ctx, rec.ID, SimulationRequest{})
	if err != nil || missing.Status != RunError || missing.ErrorCode != "FLOW_SIMULATION_FIXTURE_REQUIRED" {
		t.Fatalf("missing fixture: %+v, %v", missing, err)
	}
	req := SimulationRequest{Fixtures: map[string]SimulationFixture{web: {Output: map[string]any{"body": "example"}}}}
	res, err := s.Simulate(ctx, rec.ID, req)
	if err != nil || res.Status != RunSuccess || res.Mode != ModeSimulation {
		t.Fatalf("simulation: %+v, %v", res, err)
	}
	var found bool
	for _, step := range res.Steps {
		if step.NodeID == set && step.Output["answer"] == "example" {
			found = true
		}
	}
	if !found || called.Load() != 0 || len(bridge.startedRuns()) != 0 {
		t.Fatalf("isolation: found=%v calls=%d", found, called.Load())
	}
	runs, err := s.Runs(ctx, rec.ID, RunFilter{})
	if err != nil || len(runs) != 0 {
		t.Fatalf("simulation persisted runs: %v, %v", runs, err)
	}
	blocked := s.engine.Execute(ctx, RunRequest{Mode: ModeSimulation, Flow: rec.Draft, TriggerNode: start}, nil)
	if blocked.Status != RunError || called.Load() != 0 {
		t.Fatalf("production engine accepted simulation: %+v", blocked)
	}
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision + 1}); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("revision: %v", err)
	}
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{Fixtures: map[string]SimulationFixture{web: {ErrorCode: "arbitrary"}}}); !errors.Is(err, ErrInvalidSimulationFixture) {
		t.Fatalf("invalid fixture: %v", err)
	}
	if _, err := s.Simulate(ctx, rec.ID, SimulationRequest{TriggerData: map[string]any{"large": strings.Repeat("x", MaxStoredOutputBytes)}}); !errors.Is(err, ErrTestDataTooLarge) {
		t.Fatalf("oversized data: %v", err)
	}
}

func TestSimulationRejectsUnknownNodesBeforeExecution(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	var called atomic.Int32
	s.reg.MustRegister(&NodeDef{Type: "tool.unknown", Execute: func(context.Context, ExecInput) (ExecResult, error) {
		called.Add(1)
		return ExecResult{}, nil
	}})
	for _, tc := range []struct {
		name                string
		disabled, connected bool
	}{
		{"disabled", true, true}, {"unreachable", false, false}, {"continue", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newFlow(tc.name)
			start := b.node("start", TypeTriggerManual, nil)
			unknown := b.node("unknown", "tool.unknown", nil)
			b.f.Nodes[1].Settings.Disabled = tc.disabled
			b.f.Nodes[1].Settings.OnError = ErrorContinue
			if tc.connected {
				b.edge(start, PortOut, unknown)
			}
			rec, err := s.CreateFlow(context.Background(), CreateRequest{Import: b.build()})
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Simulate(context.Background(), rec.ID, SimulationRequest{})
			var validation *ValidationError
			if !errors.As(err, &validation) || findIssue(validation.Issues, IssueNodeTypeUnknown, unknown) == nil {
				t.Fatalf("unknown node was not refused: %v", err)
			}
		})
	}
	if called.Load() != 0 {
		t.Fatalf("unknown node executed %d times", called.Load())
	}
}
