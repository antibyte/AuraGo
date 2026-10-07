package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const mm1c09FlowCronExpr = "0 0 1 1 *"

// mm1c09CronFixture has a published, enabled flow with one schedule trigger, so its cron job
// "mission_<flow>__n_aaaaaaaa" exists with the flow source.
type mm1c09CronFixture struct {
	dc     *DispatchContext
	mm     *tools.MissionManagerV2
	cron   *tools.CronManager
	flowID string
	jobID  string
}

func mm1c09NewCron(t *testing.T) *mm1c09CronFixture {
	t.Helper()
	cfg := &config.Config{}
	cfg.Tools.Missions.Enabled = true
	cfg.Tools.Scheduler.Enabled = true
	useRuntimePermissionsForTest(t, cfg)
	dir := t.TempDir()
	cronMgr := tools.NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron Start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := tools.NewMissionManagerV2(dir, cronMgr)
	mm.SetFlowHooks(&mm1c09Hooks{})
	flowID, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Bericht")
	if err != nil {
		t.Fatalf("CreateFlowMission: %v", err)
	}
	spec := tools.FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: tools.FlowTriggerSchedule, Schedule: mm1c09FlowCronExpr}
	if err := mm.SyncFlowMission(flowID, "Bericht", []tools.FlowTriggerSpec{spec}); err != nil {
		t.Fatalf("SyncFlowMission: %v", err)
	}
	if err := mm.SetFlowMissionEnabled(flowID, true); err != nil {
		t.Fatalf("SetFlowMissionEnabled: %v", err)
	}
	f := &mm1c09CronFixture{
		dc: &DispatchContext{
			Cfg:              cfg,
			Logger:           slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
			MissionManagerV2: mm,
			CronManager:      cronMgr,
		},
		mm: mm, cron: cronMgr, flowID: flowID, jobID: "mission_" + flowID + "__n_aaaaaaaa",
	}
	if job, ok := f.job(f.jobID); !ok || !job.IsFlowJob() || job.CronExpr != mm1c09FlowCronExpr {
		t.Fatalf("fixture: the flow's cron job is missing or wrong: %+v", job)
	}
	return f
}

func (f *mm1c09CronFixture) job(id string) (tools.CronJob, bool) {
	for _, job := range f.cron.GetJobs() {
		if job.ID == id {
			return job, true
		}
	}
	return tools.CronJob{}, false
}

func (f *mm1c09CronFixture) call(t *testing.T, action string, params map[string]interface{}) string {
	t.Helper()
	out, handled := dispatchExec(context.Background(), ToolCall{Action: action, Params: params}, f.dc)
	if !handled {
		t.Fatalf("%s was not handled: %v", action, params)
	}
	return out
}

// mm1c09RequireCronRefused checks the flow_mission refusal envelope of a cron tool.
func mm1c09RequireCronRefused(t *testing.T, label, out string) {
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
		t.Fatalf("%s: not the flow refusal: %q", label, out)
	}
	if len(out) > 1000 {
		t.Fatalf("%s: the refusal is %d bytes", label, len(out))
	}
}

func TestCronToolsRefuseToChangeAFlowsScheduleJob(t *testing.T) {
	f := mm1c09NewCron(t)
	jobsBefore := f.cron.GetJobsWithRuntimeStatus()

	type call struct {
		label, action string
		params        map[string]interface{}
	}
	var calls []call
	for _, action := range []string{"manage_schedule", "cron_scheduler"} {
		for _, op := range []string{"add", "remove", "enable", "disable", "toggle"} {
			calls = append(calls, call{action + " " + op, action, map[string]interface{}{
				"operation": op, "id": f.jobID, "cron_expr": "* * * * *", "task_prompt": "do something else",
			}})
		}
	}
	calls = append(calls, call{"remove_cron_job", "remove_cron_job", map[string]interface{}{"id": f.jobID}})

	for _, c := range calls {
		t.Run(c.label, func(t *testing.T) {
			mm1c09RequireCronRefused(t, c.label, f.call(t, c.action, c.params))
			if got := f.cron.GetJobsWithRuntimeStatus(); !reflect.DeepEqual(got, jobsBefore) {
				t.Fatalf("%s changed the flow's cron job:\nbefore %+v\nafter  %+v", c.label, jobsBefore, got)
			}
		})
	}
	if _, ok := f.mm.NextRun(f.flowID); !ok {
		t.Fatal("the flow lost its schedule")
	}
}

func TestCronToolsRefuseToPlantAJobUnderAFlowsJobID(t *testing.T) {
	f := mm1c09NewCron(t)
	// A node the flow has no job for (yet): the id is still the flow's.
	planted := "mission_" + f.flowID + "__n_bbbbbbbb"
	if _, ok := f.job(planted); ok {
		t.Fatal("fixture: the job must be absent")
	}
	for _, action := range []string{"manage_schedule", "cron_scheduler"} {
		out := f.call(t, action, map[string]interface{}{
			"operation": "add", "id": planted, "cron_expr": "* * * * *", "task_prompt": "do something else",
		})
		mm1c09RequireCronRefused(t, action+" add", out)
	}
	if _, ok := f.job(planted); ok {
		t.Fatal("a refused add created a job")
	}
	if got := len(f.cron.GetJobs()); got != 1 {
		t.Fatalf("want only the flow's job, got %d jobs", got)
	}
}

func TestCronToolsRefuseJobsWithTheFlowSourceEvenWithoutAFlowMission(t *testing.T) {
	f := mm1c09NewCron(t)
	// A leftover flow job whose mission is gone; only its source marks it.
	orphan := "mission_gone__n_cccccccc"
	if _, err := f.cron.ManageScheduleWithSource("add", orphan, mm1c09FlowCronExpr, "EasyDrag flow trigger", "", "flow"); err != nil {
		t.Fatalf("add orphan: %v", err)
	}
	if f.mm.OwnsFlowCronJob(orphan) {
		t.Fatal("fixture: the orphan's flow mission must not exist")
	}
	// An odd id that no flow mission owns, but with the flow source.
	odd := "custom-flow-job"
	if _, err := f.cron.ManageScheduleWithSource("add", odd, mm1c09FlowCronExpr, "EasyDrag flow trigger", "", "flow"); err != nil {
		t.Fatalf("add odd: %v", err)
	}
	for _, id := range []string{orphan, odd} {
		for _, op := range []string{"remove", "disable", "add"} {
			mm1c09RequireCronRefused(t, id+" "+op, f.call(t, "manage_schedule", map[string]interface{}{
				"operation": op, "id": id, "cron_expr": "* * * * *", "task_prompt": "x",
			}))
		}
		mm1c09RequireCronRefused(t, id+" remove_cron_job", f.call(t, "remove_cron_job", map[string]interface{}{"id": id}))
		if job, ok := f.job(id); !ok || !job.IsFlowJob() || job.Disabled || job.CronExpr != mm1c09FlowCronExpr {
			t.Fatalf("%s was changed: %+v", id, job)
		}
	}
	// Without a mission manager the source still protects the job.
	f.dc.MissionManagerV2 = nil
	mm1c09RequireCronRefused(t, "no mission manager", f.call(t, "manage_schedule", map[string]interface{}{"operation": "remove", "id": odd}))
	if _, ok := f.job(odd); !ok {
		t.Fatal("the flow-source job was removed")
	}
}

func TestCronToolsListStillShowsFlowJobs(t *testing.T) {
	f := mm1c09NewCron(t)
	for _, call := range []struct {
		action string
		params map[string]interface{}
	}{
		{"manage_schedule", map[string]interface{}{"operation": "list"}},
		{"manage_schedule", map[string]interface{}{"operation": "list", "id": f.jobID}},
		{"cron_scheduler", map[string]interface{}{"operation": "list"}},
		{"list_cron_jobs", map[string]interface{}{}},
	} {
		out := f.call(t, call.action, call.params)
		if !strings.Contains(out, f.jobID) || strings.Contains(out, "flow_mission") {
			t.Fatalf("%s %v must list the flow job: %q", call.action, call.params, out)
		}
	}
}

func TestCronToolsLeaveOrdinaryJobsAlone(t *testing.T) {
	f := mm1c09NewCron(t)
	const ordinary = "daily_report"
	add := func(id string) {
		t.Helper()
		out := f.call(t, "manage_schedule", map[string]interface{}{
			"operation": "add", "id": id, "cron_expr": "0 0 9 * * *", "task_prompt": "Generate a daily summary.",
		})
		if !strings.Contains(out, `"status": "success"`) {
			t.Fatalf("add %s: %q", id, out)
		}
	}
	add(ordinary)
	if job, ok := f.job(ordinary); !ok || job.IsFlowJob() || job.Disabled {
		t.Fatalf("add: %+v", job)
	}
	for _, step := range []struct{ op, want string }{{"disable", "success"}, {"enable", "success"}} {
		out := f.call(t, "manage_schedule", map[string]interface{}{"operation": step.op, "id": ordinary})
		if !strings.Contains(out, `"status": "`+step.want+`"`) || strings.Contains(out, "flow_mission") {
			t.Fatalf("%s: %q", step.op, out)
		}
	}
	// An id that only looks like a flow job (the mission does not exist) is an ordinary job.
	lookalike := "mission_mission_0__n_aaaaaaaa"
	add(lookalike)
	if out := f.call(t, "remove_cron_job", map[string]interface{}{"id": lookalike}); strings.Contains(out, "flow_mission") {
		t.Fatalf("remove_cron_job on a lookalike: %q", out)
	}
	if _, ok := f.job(lookalike); ok {
		t.Fatal("the lookalike job must be removable")
	}
	if out := f.call(t, "manage_schedule", map[string]interface{}{"operation": "remove", "id": ordinary}); !strings.Contains(out, `"status": "success"`) {
		t.Fatalf("remove: %q", out)
	}
	// schedule_cron generates its own id and cannot collide with a flow's.
	if out := f.call(t, "schedule_cron", map[string]interface{}{"cron_expr": "0 0 9 * * *", "task_prompt": "x"}); !strings.Contains(out, `"status": "success"`) {
		t.Fatalf("schedule_cron: %q", out)
	}
	if job, ok := f.job(f.jobID); !ok || !job.IsFlowJob() {
		t.Fatalf("the flow's job changed: %+v", job)
	}
}

func TestFlowCronJobRefusalIsBoundedAndValidJSON(t *testing.T) {
	f := mm1c09NewCron(t)
	hostile := "mission_" + f.flowID + "__\t\"quote\"\\" + strings.Repeat("x", 5*1024)
	out := f.call(t, "manage_schedule", map[string]interface{}{"operation": "add", "id": hostile, "cron_expr": "* * * * *", "task_prompt": "x"})
	mm1c09RequireCronRefused(t, "hostile id", out)
	if !strings.Contains(out, "…") {
		t.Fatalf("the echoed id must be cut: %q", out)
	}
}
