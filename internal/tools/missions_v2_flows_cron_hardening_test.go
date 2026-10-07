package tools

import (
	"strings"
	"testing"
)

// ff1PersistedCronJobs reads the cron jobs as the crontab store holds them, the list an
// older AuraGo loads at its start.
func ff1PersistedCronJobs(t *testing.T, dir string) []CronJob {
	t.Helper()
	store, err := newSystemTaskStore(dir)
	if err != nil {
		t.Fatalf("open system task store: %v", err)
	}
	defer func() { _ = store.release() }()
	var jobs []CronJob
	if _, err := store.load(systemTaskNamespaceCron, &jobs); err != nil {
		t.Fatalf("load persisted cron jobs: %v", err)
	}
	return jobs
}

// ff1PersistedFlowJobs returns the ids of persisted cron jobs with the flow source.
func ff1PersistedFlowJobs(t *testing.T, dir string) []string {
	t.Helper()
	var ids []string
	for _, job := range ff1PersistedCronJobs(t, dir) {
		if job.Source == flowCronSource {
			ids = append(ids, job.ID)
		}
	}
	return ids
}

// FF1: a flow schedule is a runtime-only cron job. The crontab store never holds it, so an
// older AuraGo, which hands every job it does not know to its agent, never runs it; the
// dashboard and the agent's cron tools still see it (GetJobs) and leave it alone.
func TestFF1FlowScheduleIsNotPersisted(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	if _, err := cronMgr.ManageSchedule("add", "ff1_agent_job", "0 6 * * *", "ff1 agent job", ""); err != nil {
		t.Fatal(err)
	}
	mm := NewMissionManagerV2(dir, cronMgr)
	id := publishTestFlow(t, mm, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "* * * * *"})
	jobID := flowCronJobID(id, "n_aaaaaaaa")

	job, ok := c07CronJob(cronMgr, jobID)
	if !ok || job.Source != flowCronSource || job.CronExpr != "* * * * *" {
		t.Fatalf("flow schedule job in memory = %+v (found %v)", job, ok)
	}
	if !IsFlowCronJob(cronMgr, jobID) || !FlowOwnsCronJob(mm, cronMgr, jobID) {
		t.Fatal("the dashboard and agent protections must see the runtime flow job")
	}
	if out, _ := cronMgr.ManageSchedule("list", "", "", "", ""); !ff1ContainsAll(out, jobID, `"source":"flow"`) {
		t.Fatalf("cron list does not show the flow job: %s", out)
	}
	if got := ff1PersistedFlowJobs(t, dir); len(got) != 0 {
		t.Fatalf("persisted flow cron jobs = %v", got)
	}
	// A later save (any change to another job) keeps it out of the store too.
	if _, err := cronMgr.ManageSchedule("add", "ff1_agent_job_2", "0 7 * * *", "ff1 agent job 2", ""); err != nil {
		t.Fatal(err)
	}
	persisted := ff1PersistedCronJobs(t, dir)
	if len(persisted) != 2 || ff1HasJob(persisted, jobID) || !ff1HasJob(persisted, "ff1_agent_job") || !ff1HasJob(persisted, "ff1_agent_job_2") {
		t.Fatalf("persisted cron jobs = %+v", persisted)
	}
	// Disabling the flow removes the runtime job.
	if err := mm.SetFlowMissionEnabled(id, false); err != nil {
		t.Fatal(err)
	}
	if hasCronJob(cronMgr, jobID) {
		t.Fatal("a disabled flow keeps its schedule job")
	}
}

// FF1: after a restart (a new cron manager and mission manager on the same data
// directory) the flow schedule is registered again from the flow mission's triggers.
func TestFF1FlowScheduleIsRegisteredAfterARestart(t *testing.T) {
	dir := tempSystemTaskDir(t)
	setupCron := NewCronManager(dir)
	setup := NewMissionManagerV2(dir, setupCron)
	id := publishTestFlow(t, setup, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"})
	jobID := flowCronJobID(id, "n_aaaaaaaa")
	if err := setupCron.Close(); err != nil {
		t.Fatal(err)
	}

	cronMgr := NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	if hasCronJob(cronMgr, jobID) {
		t.Fatal("the cron manager loaded a flow schedule job from its store")
	}
	mm := NewMissionManagerV2(dir, cronMgr)
	if err := mm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(mm.Stop)
	job, ok := c07CronJob(cronMgr, jobID)
	if !ok || job.Source != flowCronSource || job.CronExpr != "0 7 * * *" {
		t.Fatalf("flow schedule after restart = %+v (found %v)", job, ok)
	}
	if _, scheduled := cronMgr.NextRun(jobID); !scheduled {
		t.Fatal("the flow schedule job is not scheduled after the restart")
	}
	if got := ff1PersistedFlowJobs(t, dir); len(got) != 0 {
		t.Fatalf("persisted flow cron jobs after restart = %v", got)
	}
}

// FF1: flow cron jobs that an earlier build persisted are taken out of the store at Start;
// a current one keeps running from memory, a stale one is removed.
func TestFF1StartRemovesPersistedFlowCronJobs(t *testing.T) {
	dir := tempSystemTaskDir(t)
	setupCron := NewCronManager(dir)
	setup := NewMissionManagerV2(dir, setupCron)
	id := publishTestFlow(t, setup, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"})
	current, stale := flowCronJobID(id, "n_aaaaaaaa"), flowCronJobID(id, "n_bbbbbbbb")
	// What an earlier branch build wrote: persisted jobs with the flow source.
	for jobID, expr := range map[string]string{current: "0 7 * * *", stale: "0 9 * * *"} {
		if _, err := setupCron.ManageScheduleWithSource("add", jobID, expr, "EasyDrag flow trigger", "", flowCronSource); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := setupCron.ManageSchedule("add", "ff1_agent_job", "0 6 * * *", "ff1 agent job", ""); err != nil {
		t.Fatal(err)
	}
	if got := ff1PersistedFlowJobs(t, dir); len(got) != 2 {
		t.Fatalf("seeded persisted flow jobs = %v", got)
	}
	if err := setupCron.Close(); err != nil {
		t.Fatal(err)
	}

	cronMgr := NewCronManager(dir)
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	if err := mm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(mm.Stop)
	if got := ff1PersistedFlowJobs(t, dir); len(got) != 0 {
		t.Fatalf("persisted flow cron jobs after Start = %v", got)
	}
	if !ff1HasJob(ff1PersistedCronJobs(t, dir), "ff1_agent_job") {
		t.Fatal("Start dropped a persisted job of another source")
	}
	if job, ok := c07CronJob(cronMgr, current); !ok || job.Source != flowCronSource || job.CronExpr != "0 7 * * *" {
		t.Fatalf("current flow schedule after Start = %+v (found %v)", job, ok)
	}
	if hasCronJob(cronMgr, stale) {
		t.Fatal("a stale persisted flow job survived Start")
	}
}

// FF1: the cron manager's runtime-only jobs. AddRuntimeJob keeps a job out of every save,
// Start keeps runtime jobs added before it, and a regular add under the same id makes the
// job persisted again.
func TestFF1CronRuntimeJobs(t *testing.T) {
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	out, err := cronMgr.AddRuntimeJob("ff1_runtime", "0 5 * * *", "ff1 runtime", "ff1")
	if err != nil || !ff1ContainsAll(out, `"status": "success"`) {
		t.Fatalf("AddRuntimeJob = %s, %v", out, err)
	}
	if out, err := cronMgr.AddRuntimeJob("ff1_bad", "not a cron expression", "x", "ff1"); err != nil || !ff1ContainsAll(out, `"status": "error"`) {
		t.Fatalf("AddRuntimeJob with a bad expression = %s, %v", out, err)
	}
	if _, err := cronMgr.ManageSchedule("add", "ff1_persisted", "0 6 * * *", "ff1 persisted", ""); err != nil {
		t.Fatal(err)
	}
	if err := cronMgr.Start(func(string) {}); err != nil {
		t.Fatalf("cron start: %v", err)
	}
	if !hasCronJob(cronMgr, "ff1_runtime") || !hasCronJob(cronMgr, "ff1_persisted") {
		t.Fatalf("jobs after Start = %+v", cronMgr.GetJobs())
	}
	if _, scheduled := cronMgr.NextRun("ff1_runtime"); !scheduled {
		t.Fatal("a runtime job added before Start is not scheduled")
	}
	persisted := ff1PersistedCronJobs(t, dir)
	if ff1HasJob(persisted, "ff1_runtime") || !ff1HasJob(persisted, "ff1_persisted") {
		t.Fatalf("persisted = %+v", persisted)
	}
	// A regular add under the same id persists the job.
	if _, err := cronMgr.ManageScheduleWithSource("add", "ff1_runtime", "0 5 * * *", "ff1 runtime", "", "ff1"); err != nil {
		t.Fatal(err)
	}
	if !ff1HasJob(ff1PersistedCronJobs(t, dir), "ff1_runtime") {
		t.Fatal("a regular add did not persist the job")
	}
	// And a runtime add under a persisted id takes it out of the store.
	if _, err := cronMgr.AddRuntimeJob("ff1_persisted", "0 6 * * *", "ff1 persisted", ""); err != nil {
		t.Fatal(err)
	}
	if ff1HasJob(ff1PersistedCronJobs(t, dir), "ff1_persisted") {
		t.Fatal("a runtime add left the persisted copy in the store")
	}
	if _, err := cronMgr.ManageSchedule("remove", "ff1_persisted", "", "", ""); err != nil || hasCronJob(cronMgr, "ff1_persisted") {
		t.Fatalf("remove of a runtime job: %v", err)
	}
}

func ff1HasJob(jobs []CronJob, id string) bool {
	for _, job := range jobs {
		if job.ID == id {
			return true
		}
	}
	return false
}

func ff1ContainsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
