package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// mm1c09Hooks records the flow runs that Mission Control starts.
type mm1c09Hooks struct {
	mu      sync.Mutex
	started []string
}

func (h *mm1c09Hooks) StartFlowRun(missionID, nodeID, triggerType, triggerData string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.started = append(h.started, strings.Join([]string{missionID, nodeID, triggerType, triggerData}, "|"))
	return nil
}

func (h *mm1c09Hooks) FlowMissionDeleted(string)       {}
func (h *mm1c09Hooks) FlowEnabledChanged(string, bool) {}
func (h *mm1c09Hooks) NextFlowRun(string) (time.Time, bool) {
	return time.Time{}, false
}

func (h *mm1c09Hooks) runs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.started...)
}

// mm1c09Fixture holds a mission manager with one published, enabled flow mission.
type mm1c09Fixture struct {
	mm     *tools.MissionManagerV2
	dc     *DispatchContext
	hooks  *mm1c09Hooks
	flowID string
}

func mm1c09New(t *testing.T) *mm1c09Fixture {
	t.Helper()
	cfg := &config.Config{}
	cfg.Tools.Missions.Enabled = true
	useRuntimePermissionsForTest(t, cfg)
	mm := tools.NewMissionManagerV2(t.TempDir(), nil)
	hooks := &mm1c09Hooks{}
	mm.SetFlowHooks(hooks)
	id, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Bericht")
	if err != nil {
		t.Fatalf("CreateFlowMission: %v", err)
	}
	if err := mm.SyncFlowMission(id, "Bericht", nil); err != nil {
		t.Fatalf("SyncFlowMission: %v", err)
	}
	if err := mm.SetFlowMissionEnabled(id, true); err != nil {
		t.Fatalf("SetFlowMissionEnabled: %v", err)
	}
	return &mm1c09Fixture{
		mm:     mm,
		dc:     &DispatchContext{Cfg: cfg, Logger: slog.Default(), MissionManagerV2: mm},
		hooks:  hooks,
		flowID: id,
	}
}

func (f *mm1c09Fixture) call(t *testing.T, params map[string]interface{}) string {
	t.Helper()
	out, handled := dispatchComm(context.Background(), ToolCall{Action: "manage_missions", Params: params}, f.dc)
	if !handled {
		t.Fatalf("manage_missions was not handled: %v", params)
	}
	return out
}

func (f *mm1c09Fixture) flow(t *testing.T) tools.MissionV2 {
	t.Helper()
	m, ok := f.mm.Get(f.flowID)
	if !ok {
		t.Fatalf("flow mission %s is gone", f.flowID)
	}
	return *m
}

// mm1c09Decode parses a manage_missions reply ("Tool Output: {json}") and returns its status and
// message. The reply must be valid JSON.
func mm1c09Decode(t *testing.T, out string) (status, message string) {
	t.Helper()
	if !strings.HasPrefix(out, "Tool Output: ") {
		t.Fatalf("missing the Tool Output prefix: %q", out)
	}
	var env struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &env); err != nil {
		t.Fatalf("not valid JSON: %v: %q", err, out)
	}
	return env.Status, env.Message
}

// mm1c09RequireRefused checks the flow_mission refusal envelope.
func mm1c09RequireRefused(t *testing.T, label, out string) {
	t.Helper()
	if !strings.HasPrefix(out, "Tool Output: ") {
		t.Fatalf("%s: missing the Tool Output prefix: %q", label, out)
	}
	var env struct {
		Status  string `json:"status"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &env); err != nil {
		t.Fatalf("%s: not valid JSON: %v: %q", label, err, out)
	}
	if env.Status != "error" || env.Code != "flow_mission" || !strings.Contains(env.Message, "EasyDrag") {
		t.Fatalf("%s: not the flow_mission refusal: %q", label, out)
	}
}

func TestManageMissionsRefusesEveryChangeToAFlowMission(t *testing.T) {
	f := mm1c09New(t)
	before := f.flow(t)
	if !before.Enabled || before.Locked {
		t.Fatalf("fixture: want an enabled, unlocked flow mission, got %+v", before)
	}
	// update/edit/delete/remove exist; the others do not exist today, and a flow mission must
	// stay untouched when a later version adds them (enable, disable and lock included).
	for _, op := range []string{
		"update", "edit", "delete", "remove",
		"enable", "disable", "toggle", "activate", "deactivate", "set_enabled",
		"lock", "unlock", "set_locked",
		"pause", "resume", "set_priority", "priority", "reschedule", "publish",
		"UPDATE", "Delete",
	} {
		t.Run(op, func(t *testing.T) {
			out := f.call(t, map[string]interface{}{
				"operation": op, "id": f.flowID,
				"title": "Neu", "command": "andere Aufgabe", "cron_expr": "0 9 * * *",
				"priority": 3, "locked": true,
			})
			mm1c09RequireRefused(t, op, out)
			if after := f.flow(t); !reflect.DeepEqual(before, after) {
				t.Fatalf("%s changed the flow mission:\nbefore %+v\nafter  %+v", op, before, after)
			}
		})
	}
	if got := f.hooks.runs(); len(got) != 0 {
		t.Fatalf("a refused operation started a flow run: %v", got)
	}
}

func TestManageMissionsCannotUnlockOrDisableALockedFlowMission(t *testing.T) {
	f := mm1c09New(t)
	// Mission Control (not the agent) locks the flow.
	if err := f.mm.Update(f.flowID, &tools.MissionV2{ExecutionType: tools.ExecutionFlow, Enabled: true, Locked: true}); err != nil {
		t.Fatalf("lock through the manager: %v", err)
	}
	before := f.flow(t)
	if !before.Locked || !before.Enabled {
		t.Fatalf("fixture: want a locked, enabled flow mission, got %+v", before)
	}
	for _, params := range []map[string]interface{}{
		{"operation": "update", "id": f.flowID, "locked": false},
		{"operation": "update", "id": f.flowID, "title": "Neu"},
		{"operation": "unlock", "id": f.flowID},
		{"operation": "disable", "id": f.flowID},
		{"operation": "delete", "id": f.flowID},
	} {
		mm1c09RequireRefused(t, params["operation"].(string), f.call(t, params))
		if after := f.flow(t); !reflect.DeepEqual(before, after) {
			t.Fatalf("%v changed the flow mission: %+v", params, after)
		}
	}
}

func TestManageMissionsFlowMissionsStayReadableAndRunnable(t *testing.T) {
	f := mm1c09New(t)
	db, err := tools.InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("InitMissionHistoryDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f.mm.SetHistoryDB(db)
	runID, err := tools.RecordMissionStart(db, f.flowID, "Bericht", "manual", "")
	if err != nil {
		t.Fatalf("RecordMissionStart: %v", err)
	}
	if err := tools.RecordMissionCompletion(db, runID, "success", "fertig"); err != nil {
		t.Fatalf("RecordMissionCompletion: %v", err)
	}

	for label, params := range map[string]map[string]interface{}{
		"list":            {"operation": "list"},
		"list with id":    {"operation": "list", "id": f.flowID},
		"history":         {"operation": "history"},
		"history with id": {"operation": "history", "id": f.flowID},
	} {
		out := f.call(t, params)
		if strings.Contains(out, "flow_mission") || strings.Contains(out, `"status":"error"`) {
			t.Fatalf("%s was refused: %q", label, out)
		}
		want := f.flowID
		if strings.HasPrefix(label, "history") {
			want = runID
		}
		if !strings.Contains(out, want) {
			t.Fatalf("%s must contain %s: %q", label, want, out)
		}
	}

	for _, op := range []string{"run", "run_now"} {
		out := f.call(t, map[string]interface{}{"operation": op, "id": f.flowID})
		if !strings.Contains(out, `"status": "success"`) || strings.Contains(out, "flow_mission") {
			t.Fatalf("%s: %q", op, out)
		}
		// A flow run is not queued: the reply must say so and name the way to follow it.
		const wantMessage = "Flow run requested. EasyDrag runs flows on its own engine, not in the mission queue; use operation=history to follow it."
		if status, message := mm1c09Decode(t, out); status != "success" || message != wantMessage {
			t.Fatalf("%s: the flow run reply is wrong: %q", op, out)
		}
	}
	want := f.flowID + "||manual|" // mission, default trigger node, manual, no trigger data
	if got := f.hooks.runs(); len(got) != 2 || got[0] != want || got[1] != want {
		t.Fatalf("run must start flow runs through the hooks: got %v, want two of %q", got, want)
	}

	// Without the flow service a run is an ordinary error, not the refusal.
	f.mm.SetFlowHooks(nil)
	out := f.call(t, map[string]interface{}{"operation": "run", "id": f.flowID})
	if !strings.Contains(out, "flows are not available") || strings.Contains(out, "flow_mission") {
		t.Fatalf("run without hooks: %q", out)
	}
	f.mm.SetFlowHooks(f.hooks)
	if err := f.mm.SetFlowMissionEnabled(f.flowID, false); err != nil {
		t.Fatalf("disable through the manager: %v", err)
	}
	out = f.call(t, map[string]interface{}{"operation": "run", "id": f.flowID})
	if !strings.Contains(out, "mission is disabled") || strings.Contains(out, "flow_mission") {
		t.Fatalf("run of a disabled flow: %q", out)
	}
	if got := f.hooks.runs(); len(got) != 2 {
		t.Fatalf("a failed run reached the hooks: %v", got)
	}
}

func TestManageMissionsCreateRefusesFlowType(t *testing.T) {
	f := mm1c09New(t)
	before := len(f.mm.List())
	for _, executionType := range []string{"flow", "Flow", " FLOW "} {
		for _, op := range []string{"create", "add"} {
			out := f.call(t, map[string]interface{}{
				"operation": op, "title": "Neuer Flow", "command": "tu etwas",
				"execution_type": executionType,
			})
			mm1c09RequireRefused(t, op+" "+executionType, out)
			if strings.Contains(out, "Mission  ") {
				t.Fatalf("the creation refusal must not name a mission: %q", out)
			}
		}
	}
	if got := len(f.mm.List()); got != before {
		t.Fatalf("a refused create changed the mission count: %d, was %d", got, before)
	}
	for _, m := range f.mm.List() {
		if m.Name == "Neuer Flow" {
			t.Fatalf("a refused create made a mission: %+v", m)
		}
	}

	// Other creates keep working; a different execution_type is simply ignored.
	for _, params := range []map[string]interface{}{
		{"operation": "create", "title": "Normal", "command": "tu etwas"},
		{"operation": "add", "title": "Manuell", "command": "tu etwas", "execution_type": "manual"},
	} {
		out := f.call(t, params)
		if !strings.Contains(out, `"status":"success"`) {
			t.Fatalf("%v: %q", params, out)
		}
		// Create derives the id from the clock; a coarse Windows clock must tick between creates.
		time.Sleep(20 * time.Millisecond)
	}
	flows := 0
	for _, m := range f.mm.List() {
		switch m.Name {
		case "Normal", "Manuell":
			if m.ExecutionType != tools.ExecutionManual {
				t.Fatalf("%s: execution type %q", m.Name, m.ExecutionType)
			}
		}
		if m.ExecutionType == tools.ExecutionFlow {
			flows++
		}
	}
	if got := len(f.mm.List()); got != before+2 || flows != 1 {
		t.Fatalf("want %d missions with one flow, got %d missions and %d flows", before+2, got, flows)
	}
}

func TestManageMissionsLeavesOrdinaryMissionsAlone(t *testing.T) {
	f := mm1c09New(t)
	m := &tools.MissionV2{Name: "Alt", Prompt: "tu etwas", ExecutionType: tools.ExecutionManual}
	if err := f.mm.Create(m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if out := f.call(t, map[string]interface{}{"operation": "update", "id": m.ID, "title": "Neu"}); !strings.Contains(out, `"status": "success"`) {
		t.Fatalf("update: %q", out)
	}
	if got, ok := f.mm.Get(m.ID); !ok || got.Name != "Neu" {
		t.Fatalf("update did not apply: %+v", got)
	}
	// An operation that does not exist stays an unknown-operation error for ordinary missions.
	if out := f.call(t, map[string]interface{}{"operation": "enable", "id": m.ID}); !strings.Contains(out, "Unknown operation: enable") || strings.Contains(out, "flow_mission") {
		t.Fatalf("enable: %q", out)
	}
	// An ordinary mission still goes to the mission queue, and its reply says so.
	if out := f.call(t, map[string]interface{}{"operation": "run", "id": m.ID}); !strings.Contains(out, `"status": "success"`) || !strings.Contains(out, "background task queue") || strings.Contains(out, "Flow run requested") {
		t.Fatalf("run of an ordinary mission: %q", out)
	}
	if out := f.call(t, map[string]interface{}{"operation": "delete", "id": m.ID}); !strings.Contains(out, `"status": "success"`) {
		t.Fatalf("delete: %q", out)
	}
	if _, ok := f.mm.Get(m.ID); ok {
		t.Fatal("delete did not apply")
	}
	if _, ok := f.mm.Get(f.flowID); !ok {
		t.Fatal("the flow mission must survive")
	}
	// An id that names nothing is the usual error, not the flow refusal.
	if out := f.call(t, map[string]interface{}{"operation": "delete", "id": "mission_0"}); strings.Contains(out, "flow_mission") {
		t.Fatalf("delete of an unknown id: %q", out)
	}
}

func TestFlowMissionOperationAllowlistIsExact(t *testing.T) {
	for _, op := range []string{"list", "history", "run", "run_now"} {
		if !flowMissionOperationAllowed(op) {
			t.Errorf("%q must be allowed for flow missions", op)
		}
	}
	for _, op := range []string{
		"", "create", "add", "update", "edit", "delete", "remove", "execute",
		"enable", "disable", "lock", "unlock", "get", "show", "trigger", "List", "RUN",
	} {
		if flowMissionOperationAllowed(op) {
			t.Errorf("%q must not be allowed for flow missions", op)
		}
	}
}

func TestFlowMissionManagedOutputIsBoundedValidJSON(t *testing.T) {
	long := strings.Repeat("a", 5*1024)
	for label, id := range map[string]string{
		"ascii":     long,
		"two-byte":  strings.Repeat("ü", 5*1024),
		"four-byte": strings.Repeat("😀", 2000),
		"escapes":   strings.Repeat("\"\\\n\x00<&", 1000),
		"invalid":   strings.Repeat("\xff", 5*1024),
		"short":     "mission_123",
		"empty":     "",
	} {
		t.Run(label, func(t *testing.T) {
			out := flowMissionManagedOutput(id)
			mm1c09RequireRefused(t, label, out)
			if len(out) > 1000 {
				t.Fatalf("output is %d bytes", len(out))
			}
		})
	}

	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(flowMissionManagedOutput(long), "Tool Output: ")), &env); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(env.Message, strings.Repeat("a", missionEchoRunes)+"…") || strings.Contains(env.Message, strings.Repeat("a", missionEchoRunes+1)) {
		t.Fatalf("the echoed id must be cut to %d runes: %q", missionEchoRunes, env.Message)
	}
	short := flowMissionManagedOutput("mission_123")
	if !strings.Contains(short, "mission_123") || strings.Contains(short, "…") {
		t.Fatalf("a short id is echoed whole: %q", short)
	}
	exact := strings.Repeat("b", missionEchoRunes)
	if out := flowMissionManagedOutput(exact); !strings.Contains(out, exact) || strings.Contains(out, "…") {
		t.Fatalf("an id of exactly %d runes is echoed whole: %q", missionEchoRunes, out)
	}
}
