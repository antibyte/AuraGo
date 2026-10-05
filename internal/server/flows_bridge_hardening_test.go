package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/planner"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// c15Env is a server with Mission Control, a mission history, a planner database and a
// desktop hub whose events the test reads.
type c15Env struct {
	s      *Server
	bridge flowMissionBridge
	hist   *sql.DB
	events <-chan desktop.Event
}

func c15NewEnv(t *testing.T) *c15Env {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Tools.Missions.Enabled = true
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(cfg))
	mm := tools.NewMissionManagerV2(dir, nil)
	t.Cleanup(mm.Stop)
	hist, err := tools.InitMissionHistoryDB(filepath.Join(dir, "history.db"))
	if err != nil {
		t.Fatalf("history db: %v", err)
	}
	t.Cleanup(func() { _ = hist.Close() })
	mm.SetHistoryDB(hist)
	plannerDB, err := planner.InitDB(filepath.Join(dir, "planner.db"))
	if err != nil {
		t.Fatalf("planner db: %v", err)
	}
	t.Cleanup(func() { _ = plannerDB.Close() })
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), MissionManagerV2: mm,
		PlannerDB: plannerDB, DesktopHub: hub}
	return &c15Env{s: s, bridge: flowMissionBridge{s: s}, hist: hist, events: events}
}

// c15Mission creates an enabled flow mission with a manual trigger.
func (e *c15Env) c15Mission(t *testing.T, flowID, name string) string {
	t.Helper()
	id, err := e.bridge.CreateFlowMission(flowID, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.bridge.SyncFlowMission(id, name, []flows.TriggerBinding{{NodeID: "n_aaaaaaaa", Kind: flows.BindingManual}}); err != nil {
		t.Fatal(err)
	}
	if err := e.bridge.SetFlowMissionEnabled(id, true); err != nil {
		t.Fatal(err)
	}
	return id
}

// c15Drain returns the desktop events sent so far.
func (e *c15Env) c15Drain() []desktop.Event {
	var out []desktop.Event
	for {
		select {
		case ev := <-e.events:
			out = append(out, ev)
		default:
			return out
		}
	}
}

func c15Count(events []desktop.Event, typ string) int {
	n := 0
	for _, ev := range events {
		if ev.Type == typ {
			n++
		}
	}
	return n
}

func (e *c15Env) c15Issues(t *testing.T) []planner.OperationalIssueListItem {
	t.Helper()
	page, err := planner.ListOperationalIssues(e.s.PlannerDB, planner.OperationalIssueListFilter{Status: "all", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	return page.Items
}

func (e *c15Env) c15Run(t *testing.T, id string) *tools.MissionRun {
	t.Helper()
	run, err := tools.GetMissionRun(e.hist, id)
	if err != nil {
		t.Fatalf("history run %s: %v", id, err)
	}
	return run
}

// c15LogBuffer collects log output from any goroutine.
type c15LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *c15LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *c15LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func c15Logger(buf *c15LogBuffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// A run that never started (cancelled while queued, or ended by a shutdown) leaves the
// mission's status, counters and history alone, records no planner issue and sends no
// notification; open editors are still told to refresh.
func TestC15UnstartedRunLeavesMissionControlAlone(t *testing.T) {
	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15aaaaaaa", "Bericht")
	histID := e.bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_c15started1", FlowID: "flow_c15aaaaaaa", TriggerType: "manual"})
	if histID == "" {
		t.Fatal("the started run has no history entry")
	}
	before, _ := e.s.MissionManagerV2.Get(id)
	e.c15Drain()
	for _, res := range []flows.RunResult{
		{Status: flows.RunCancelled, ErrorCode: "FLOW_CANCELLED", ErrorMessage: "the run was stopped"},
		{Status: flows.RunError, ErrorCode: "FLOW_SHUTDOWN", ErrorMessage: "the server stopped"},
	} {
		e.bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, FlowName: "Bericht", NotifyOnError: "desktop",
			Record: flows.RunRecord{ID: "run_c15queued01", FlowID: "flow_c15aaaaaaa"}, Result: res, Outputs: map[string]any{}})
	}
	after, _ := e.s.MissionManagerV2.Get(id)
	if after.Status != tools.MissionStatusRunning || after.RunCount != before.RunCount ||
		after.LastResult != before.LastResult || after.LastOutput != before.LastOutput {
		t.Fatalf("mission changed by unstarted runs: before %+v, after %+v", before, after)
	}
	if run := e.c15Run(t, histID); run.Status != "running" {
		t.Fatalf("the started run's history entry = %q", run.Status)
	}
	if issues := e.c15Issues(t); len(issues) != 0 {
		t.Fatalf("planner issues = %+v", issues)
	}
	events := e.c15Drain()
	if c15Count(events, "notification") != 0 || c15Count(events, "flows_changed") != 2 {
		t.Fatalf("events = %+v", events)
	}
	// The running slot of the started run is intact: its end releases it and counts.
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, HistoryID: histID, FlowName: "Bericht", Started: true,
		Record: flows.RunRecord{ID: "run_c15started1", FlowID: "flow_c15aaaaaaa"}, Result: flows.RunResult{Status: flows.RunSuccess}})
	if m, _ := e.s.MissionManagerV2.Get(id); m.Status != tools.MissionStatusIdle || m.RunCount != before.RunCount+1 {
		t.Fatalf("mission after the started run = %+v", m)
	}
}

// A started run whose flow and mission are gone (no mission id, no name) still completes
// its history entry, records a planner issue named after the flow id and notifies.
func TestC15StartedRunWithoutMission(t *testing.T) {
	e := c15NewEnv(t)
	histID, err := tools.RecordMissionStart(e.hist, "mission_c15gone", "Gone", "manual", "{}")
	if err != nil {
		t.Fatal(err)
	}
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{Started: true, HistoryID: histID,
		Record: flows.RunRecord{ID: "run_c15nomission", FlowID: "flow_c15gone0001"},
		Result: flows.RunResult{Status: flows.RunError, ErrorCode: "FLOW_TOOL_ERROR", ErrorMessage: "kaputt"}})
	if run := e.c15Run(t, histID); run.Status != "error" || !strings.Contains(run.ErrorMsg, "kaputt") {
		t.Fatalf("history entry = %+v", run)
	}
	issues := e.c15Issues(t)
	if len(issues) != 1 || issues[0].Source != "flow" || !strings.Contains(issues[0].Title, "Flow flow_c15gone0001 failed") {
		t.Fatalf("planner issues = %+v", issues)
	}
	if n := c15Count(e.c15Drain(), "notification"); n != 1 {
		t.Fatalf("notifications = %d", n)
	}
	// Without even a history entry nothing is left to record but the issue.
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{Started: true,
		Record: flows.RunRecord{ID: "run_c15nomission2", FlowID: "flow_c15gone0001"}, Result: flows.RunResult{Status: flows.RunSuccess}})
	if issues := e.c15Issues(t); len(issues) != 1 || issues[0].Status != "done" {
		t.Fatalf("planner issues after a success = %+v", issues)
	}
}

// The planner issue title carries at most 80 runes of the flow's name.
func TestC15PlannerIssueTitleIsBounded(t *testing.T) {
	e := c15NewEnv(t)
	name := strings.Repeat("Ü", 300)
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{Started: true, FlowName: name, NotifyOnError: "off",
		Record: flows.RunRecord{ID: "run_c15longname1", FlowID: "flow_c15longname"},
		Result: flows.RunResult{Status: flows.RunError, ErrorMessage: "kaputt"}})
	issues := e.c15Issues(t)
	if len(issues) != 1 {
		t.Fatalf("planner issues = %+v", issues)
	}
	if n := strings.Count(issues[0].Title, "Ü"); n != flowNotifyNameRunes {
		t.Fatalf("the title holds %d runes of the name: %q", n, issues[0].Title)
	}
}

// Secrets are scrubbed in the parsed values, keys included, so a secret with a quote and a
// backslash (escaped in JSON, where the text scrubber cannot see it) is still removed, and
// the structure survives the redaction.
func TestC15ScrubWalksValues(t *testing.T) {
	secret := `c15"se\cret-value`
	release := security.RegisterScopedSensitiveExact(secret)
	t.Cleanup(release)
	redacted := security.RedactedText("")
	escaped, _ := json.Marshal(secret) // "c15\"se\\cret-value"
	escapedText := strings.Trim(string(escaped), `"`)

	in := map[string]any{
		"node":  map[string]any{"text": "token " + secret + " end", "list": []any{secret, 1.0, true, nil}},
		secret:  "a key holds it",
		"plain": "short",
		"typed": map[string]string{"x": secret},
	}
	// The JSON text scrub of the plan could not find it.
	if raw, _ := json.Marshal(in); !strings.Contains(security.Scrub(string(raw)), escapedText) {
		t.Fatal("the text scrubber now finds escaped secrets; the walk is still right but the test premise changed")
	}
	out := scrubFlowMap(in)
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), escapedText) || strings.Contains(string(raw), secret) {
		t.Fatalf("secret left in %s", raw)
	}
	node := out["node"].(map[string]any)
	if node["text"] != "token "+redacted+" end" || !reflect.DeepEqual(node["list"], []any{redacted, 1.0, true, nil}) {
		t.Fatalf("node = %+v", node)
	}
	if out[redacted] != "a key holds it" || out["plain"] != "short" || !reflect.DeepEqual(out["typed"], map[string]any{"x": redacted}) {
		t.Fatalf("out = %+v", out)
	}
	if in["node"].(map[string]any)["text"] != "token "+secret+" end" || in[secret] != "a key holds it" {
		t.Fatal("the input was modified")
	}
	if scrubFlowMap(nil) != nil {
		t.Fatal("a nil map must stay nil")
	}

	// The bridge applies it to run outputs and to trigger data.
	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15secret01", "Secret")
	histID := e.bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_c15secret001", FlowID: "flow_c15secret01", TriggerType: "webhook",
		TriggerData: map[string]any{"raw": `{"pw":"` + secret + `"}`, "payload": map[string]any{"pw": secret}}})
	run := e.c15Run(t, histID)
	if strings.Contains(run.TriggerData, escapedText) || strings.Contains(run.TriggerData, secret) || !json.Valid([]byte(run.TriggerData)) {
		t.Fatalf("history trigger data = %s", run.TriggerData)
	}
	e.bridge.FlowRunFinished(flows.RunFinishedInfo{MissionID: id, HistoryID: histID, FlowName: "Secret", Started: true,
		Record: flows.RunRecord{ID: "run_c15secret001", FlowID: "flow_c15secret01"}, Result: flows.RunResult{Status: flows.RunSuccess},
		Outputs: map[string]any{"mail": map[string]any{"body": "pw " + secret}}})
	m, _ := e.s.MissionManagerV2.Get(id)
	if strings.Contains(m.LastOutput, escapedText) || strings.Contains(m.LastOutput, secret) || !strings.Contains(m.LastOutput, redacted) {
		t.Fatalf("last output = %s", m.LastOutput)
	}
	if out := e.c15Run(t, histID).Output; !json.Valid([]byte(out)) || strings.Contains(out, escapedText) {
		t.Fatalf("history output = %s", out)
	}
}

// The walk stops at flowScrubMaxDepth: deeper values are dropped, never passed on unscrubbed.
func TestC15ScrubWalkIsDepthBounded(t *testing.T) {
	secret := "c15-deep-secret-value"
	release := security.RegisterScopedSensitiveExact(secret)
	t.Cleanup(release)
	var v any = secret
	for range flowScrubMaxDepth + 5 {
		v = map[string]any{"a": v}
	}
	got := scrubFlowValue(v)
	depth := 0
	for {
		m, ok := got.(map[string]any)
		if !ok {
			break
		}
		depth++
		got = m["a"]
	}
	if depth != flowScrubMaxDepth || got != nil {
		t.Fatalf("kept %d levels, then %v", depth, got)
	}
}

// The output text Mission Control gets is bounded (16 KiB, cut at a rune boundary), so a
// 1 MiB output neither reaches the mission nor the history in full.
func TestC15MissionOutputIsBounded(t *testing.T) {
	big := strings.Repeat("ä", 512<<10) // 1 MiB
	info := flows.RunFinishedInfo{Result: flows.RunResult{Status: flows.RunSuccess}, Outputs: map[string]any{"doc": map[string]any{"text": big}}}
	result, out := flowRunOutcome(info)
	if result != tools.MissionResultSuccess || len(out) > flowMissionOutputMaxBytes || !strings.HasSuffix(out, flowCutMarker) ||
		!utf8.ValidString(out) || !strings.HasPrefix(out, `{"doc":{"text":"ää`) {
		t.Fatalf("output: %d bytes, valid %v, starts %q", len(out), utf8.ValidString(out), flowBoundRunes(out, 20))
	}
	failed := flows.RunFinishedInfo{Result: flows.RunResult{Status: flows.RunError, ErrorCode: "FLOW_X", ErrorMessage: big}}
	if _, msg := flowRunOutcome(failed); len(msg) > flowMissionOutputMaxBytes || !strings.HasPrefix(msg, "FLOW_X: ää") {
		t.Fatalf("error output: %d bytes", len(msg))
	}

	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15bigoutpt", "Big")
	histID := e.bridge.FlowRunStarted(id, flows.RunRecord{ID: "run_c15bigoutput", FlowID: "flow_c15bigoutpt", TriggerType: "manual"})
	info.MissionID, info.HistoryID, info.Started = id, histID, true
	info.Record = flows.RunRecord{ID: "run_c15bigoutput", FlowID: "flow_c15bigoutpt"}
	e.bridge.FlowRunFinished(info)
	m, _ := e.s.MissionManagerV2.Get(id)
	if m.LastResult != tools.MissionResultSuccess || len(m.LastOutput) > 600 {
		t.Fatalf("mission: result %q, last output %d bytes", m.LastResult, len(m.LastOutput))
	}
	if run := e.c15Run(t, histID); run.Status != "success" || len(run.Output) > 2000 {
		t.Fatalf("history: %q, %d bytes", run.Status, len(run.Output))
	}
}

// Trigger settings decode strictly: an unknown key, a key in other case or a value of the
// wrong type fails with a short message instead of dropping a filter, and only trigger
// types that Mission Control starts flows for pass.
func TestC15TriggerConfigsAreStrict(t *testing.T) {
	mission := func(trigger string, cfg map[string]any) []flows.TriggerBinding {
		return []flows.TriggerBinding{{NodeID: "n_aaaaaaaa", Kind: flows.BindingMission, MissionTrigger: trigger, Config: cfg}}
	}
	long := strings.Repeat("k", 500)
	for _, tc := range []struct {
		name     string
		bindings []flows.TriggerBinding
		want     string
	}{
		{"unknown key", mission("webhook", map[string]any{"webhook_id": "h1", "foo": 1.0}), `trigger n_aaaaaaaa: unknown setting "foo"`},
		{"other case", mission("webhook", map[string]any{"Webhook_ID": "h1"}), `trigger n_aaaaaaaa: unknown setting "Webhook_ID"`},
		{"long key", mission("webhook", map[string]any{long: "x"}), `trigger n_aaaaaaaa: unknown setting "` + strings.Repeat("k", flowNameEchoRunes) + `…"`},
		{"wrong type", mission("webhook", map[string]any{"min_interval_seconds": "soon"}), `trigger n_aaaaaaaa: the setting "min_interval_seconds" has the wrong type`},
		{"fraction", mission("mqtt_message", map[string]any{"mqtt_topic": "a", "mqtt_min_interval_seconds": 1.5}), `the setting "mqtt_min_interval_seconds" has the wrong type`},
		{"egg event", mission("egg_hatched", nil), `waits for the event "egg_hatched", which Mission Control cannot start flows for`},
		{"flow-only type", mission("schedule", nil), `waits for the event "schedule"`},
		{"unknown type", mission("bogus", nil), `waits for the event "bogus"`},
		{"long node id", []flows.TriggerBinding{{NodeID: strings.Repeat("n", 500), Kind: "bogus"}}, `trigger ` + strings.Repeat("n", flowNameEchoRunes) + `… has an unknown binding "bogus"`},
	} {
		_, err := flowTriggerSpecs(tc.bindings)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
		if err != nil && utf8.RuneCountInString(err.Error()) > 200 {
			t.Errorf("%s: the error is not bounded: %d runes", tc.name, utf8.RuneCountInString(err.Error()))
		}
	}
	// MissionManagerV2.SyncFlowMission checks only node ids and would store any trigger
	// type, which is why the bridge checks the type.
	e := c15NewEnv(t)
	id := e.c15Mission(t, "flow_c15strict01", "Strict")
	if err := e.s.MissionManagerV2.SyncFlowMission(id, "Strict", []tools.FlowTriggerSpec{{NodeID: "n_aaaaaaaa", TriggerType: "bogus"}}); err != nil {
		t.Fatalf("SyncFlowMission validates trigger types now (%v); the bridge's check may be documented as a second line", err)
	}
	if err := e.bridge.SyncFlowMission(id, "Strict", mission("bogus", nil)); err == nil {
		t.Fatal("the bridge must refuse an unknown trigger type")
	}
}

// c15ParamSample returns a non-empty example value for a parameter.
func c15ParamSample(p flows.ParamSpec) any {
	switch {
	case len(p.Options) > 0 && p.Kind == flows.ParamMultiSelect:
		return []any{p.Options[0].Value}
	case len(p.Options) > 0:
		return p.Options[len(p.Options)-1].Value
	}
	switch p.Kind {
	case flows.ParamNumber:
		return 90.0
	case flows.ParamBool:
		return true
	case flows.ParamDateTime:
		return "2026-12-24T18:00:00Z"
	case flows.ParamCron:
		return "0 7 * * *"
	case flows.ParamJSON:
		return map[string]any{"c15": true}
	default:
		return "c15-" + p.Name
	}
}

// c15ParamVariants returns the parameter sets the catalog test binds: the defaults plus a
// sample for every required parameter without a default; the defaults plus a sample of
// every parameter; and that full set once per option of each parameter with options.
func c15ParamVariants(def *flows.NodeDef) []map[string]any {
	minimal, full := map[string]any{}, map[string]any{}
	for _, p := range def.Params {
		if p.Default != nil {
			minimal[p.Name] = p.Default
		} else if p.Required {
			minimal[p.Name] = c15ParamSample(p)
		}
		full[p.Name] = c15ParamSample(p)
	}
	variants := []map[string]any{minimal, full}
	for _, p := range def.Params {
		if p.Kind == flows.ParamMultiSelect {
			continue
		}
		for _, o := range p.Options {
			v := map[string]any{}
			for k, val := range full {
				v[k] = val
			}
			v[p.Name] = o.Value
			variants = append(variants, v)
		}
	}
	return variants
}

func c15JSONValue(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Every Mission Control trigger of the node catalog, bound by the flows package itself
// from default and sample parameters, decodes into tools.TriggerConfig with every value it
// sets, and the catalog uses exactly the trigger types the bridge accepts.
func TestC15CatalogTriggerConfigsDecode(t *testing.T) {
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, flows.StaticEnv{}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	missionTypes := map[string]bool{}
	seen := map[tools.TriggerType]bool{}
	for _, def := range reg.All() {
		if !def.Trigger {
			continue
		}
		for _, params := range c15ParamVariants(def) {
			node := flows.Node{ID: "n_aaaaaaaa", Key: "trigger", Type: def.Type, Params: params}
			bindings, err := flows.BindTriggers(&flows.Flow{Nodes: []flows.Node{node}}, reg, time.UTC, now)
			if err != nil || len(bindings) != 1 {
				t.Fatalf("%s %v: bindings %+v, err %v", def.Type, params, bindings, err)
			}
			b := bindings[0]
			if b.Kind != flows.BindingMission {
				break // manual, schedule and date/time do not depend on the variant
			}
			missionTypes[def.Type] = true
			specs, err := flowTriggerSpecs(bindings)
			if err != nil {
				t.Fatalf("%s %v: %v", def.Type, b.Config, err)
			}
			seen[specs[0].TriggerType] = true
			got, _ := c15JSONValue(t, specs[0].TriggerConfig).(map[string]any)
			for k, v := range b.Config {
				want := c15JSONValue(t, v)
				if want == nil || want == "" || want == false || want == 0.0 {
					if _, present := got[k]; present {
						t.Errorf("%s: empty %s came through as %v", def.Type, k, got[k])
					}
					continue
				}
				if !reflect.DeepEqual(got[k], want) {
					t.Errorf("%s: %s = %v in TriggerConfig, want %v (config %v)", def.Type, k, got[k], want, b.Config)
				}
			}
			for k := range got {
				if _, ok := b.Config[k]; !ok {
					t.Errorf("%s: TriggerConfig has %s, which the binding did not set", def.Type, k)
				}
			}
		}
	}
	if len(missionTypes) != 10 {
		t.Fatalf("mission trigger node types = %v, want 10", missionTypes)
	}
	if len(seen) != len(flowMissionTriggerTypes) {
		t.Fatalf("the catalog uses %v; the bridge accepts %v", seen, flowMissionTriggerTypes)
	}
}

// Hook errors are logged by kind: a flow that is gone at Debug, a timeout and anything
// else at Warn, the error text bounded.
func TestC15HookErrorsAreLoggedByKind(t *testing.T) {
	logs := &c15LogBuffer{}
	h := flowMissionHooks{s: &Server{Logger: c15Logger(logs)}}
	h.logError("gone", "m1", fmt.Errorf("lookup: %w", flows.ErrNotFound))
	h.logError("busy", "m2", fmt.Errorf("lock: %w", context.DeadlineExceeded))
	h.logError("other", "m3", errors.New(strings.Repeat("x", 5000)))
	out := logs.String()
	for _, want := range []string{`level=DEBUG msg="gone: the flow is gone already" mission_id=m1`,
		`level=WARN msg="busy: the flow stayed busy" mission_id=m2 timeout=2m0s`, `level=WARN msg=other mission_id=m3`} {
		if !strings.Contains(out, want) {
			t.Errorf("logs lack %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "x") > flowErrorRunes+10 {
		t.Fatalf("the error text is not bounded: %d", strings.Count(out, "x"))
	}
}

// The hooks reach the flow service: Mission Control's delete removes the flow, and a
// switch of a mission no flow holds is no error.
func TestC15HooksFollowTheFlowService(t *testing.T) {
	e := c15NewEnv(t)
	logs := &c15LogBuffer{}
	e.s.Logger = c15Logger(logs)
	store, err := flows.OpenStore(filepath.Join(t.TempDir(), "flows.db"), e.s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, flows.StaticEnv{}); err != nil {
		t.Fatal(err)
	}
	svc := flows.NewService(store, reg, nil, e.bridge, flows.ServiceConfig{}, e.s.Logger)
	t.Cleanup(func() { _ = svc.Shutdown(context.Background()) })
	e.s.Flows = svc
	hooks := flowMissionHooks{s: e.s}

	rec, err := svc.CreateFlow(context.Background(), flows.CreateRequest{Name: "Hooked"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := e.s.MissionManagerV2.Get(rec.MissionID); !ok {
		t.Fatal("the flow has no mission")
	}
	hooks.FlowEnabledChanged(rec.MissionID, false)
	hooks.FlowEnabledChanged("mission_c15nobody", true)
	if err := e.s.MissionManagerV2.DeleteFlowMission(rec.MissionID); err != nil {
		t.Fatal(err)
	}
	e.c15Drain()
	hooks.FlowMissionDeleted(rec.MissionID)
	if _, err := svc.GetFlow(context.Background(), rec.ID); !errors.Is(err, flows.ErrNotFound) {
		t.Fatalf("the flow survived its mission: %v", err)
	}
	if c15Count(e.c15Drain(), "flows_changed") != 1 {
		t.Fatal("open editors were not told about the delete")
	}
	if strings.Contains(logs.String(), "level=WARN") {
		t.Fatalf("unexpected warnings:\n%s", logs.String())
	}
}
