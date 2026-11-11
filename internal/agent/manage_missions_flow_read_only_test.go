package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/tools"
)

// flows.agent.read_only: run and run_now on a flow mission are refused and never reach the
// flow hooks, while list and history still work. Dispatch reads cfg on every call, so turning
// the switch off on the same dispatch context restores runs without a restart.
func TestManageMissionsFlowReadOnlyRefusesRuns(t *testing.T) {
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

	f.dc.Cfg.Flows.Agent.ReadOnly = true
	for _, op := range []string{"run", "run_now"} {
		out := f.call(t, map[string]interface{}{"operation": op, "id": f.flowID})
		status, message := mm1c09Decode(t, out)
		if status != "error" || !strings.Contains(message, "flows.agent.read_only") || !strings.Contains(out, `"code":"flow_read_only"`) {
			t.Fatalf("%s with read_only on: %q", op, out)
		}
	}
	if got := f.hooks.runs(); len(got) != 0 {
		t.Fatalf("a refused run reached the flow hooks: %v", got)
	}
	for label, params := range map[string]map[string]interface{}{
		"list":    {"operation": "list", "id": f.flowID},
		"history": {"operation": "history", "id": f.flowID},
	} {
		out := f.call(t, params)
		if strings.Contains(out, `"status":"error"`) || strings.Contains(out, "flow_read_only") {
			t.Fatalf("%s must still work with read_only on: %q", label, out)
		}
		want := f.flowID
		if label == "history" {
			want = runID
		}
		if !strings.Contains(out, want) {
			t.Fatalf("%s must contain %s: %q", label, want, out)
		}
	}

	f.dc.Cfg.Flows.Agent.ReadOnly = false
	out := f.call(t, map[string]interface{}{"operation": "run_now", "id": f.flowID})
	if status, _ := mm1c09Decode(t, out); status != "success" {
		t.Fatalf("run_now with read_only off: %q", out)
	}
	if got := f.hooks.runs(); len(got) != 1 {
		t.Fatalf("run_now with read_only off must start one flow run: %v", got)
	}
}
