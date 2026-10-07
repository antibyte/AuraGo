package tools

import (
	"strings"
	"testing"
)

// ff1SchedulerPermissions returns the test permissions with the scheduler switched as given.
func ff1SchedulerPermissions(enabled, readOnly bool) RuntimePermissions {
	perms := defaultRuntimePermissionsForTests()
	perms.SchedulerEnabled = enabled
	perms.SchedulerReadOnly = readOnly
	return perms
}

// ff1RestartWithFlowSchedule publishes a flow with a schedule on one cron manager, closes it,
// switches the runtime permissions and starts a new cron manager and mission manager on the
// same data directory, as a restart does. It returns the new cron manager, the flow job id
// and the data directory.
func ff1RestartWithFlowSchedule(t *testing.T, perms RuntimePermissions) (*CronManager, string, string) {
	t.Helper()
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	dir := tempSystemTaskDir(t)
	setupCron := NewCronManager(dir)
	setup := NewMissionManagerV2(dir, setupCron)
	id := publishTestFlow(t, setup, FlowTriggerSpec{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"})
	if err := setupCron.Close(); err != nil {
		t.Fatal(err)
	}
	ConfigureRuntimePermissions(perms)
	cronMgr := NewCronManager(dir)
	_ = cronMgr.Start(func(string) {})
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	if err := mm.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(mm.Stop)
	return cronMgr, flowCronJobID(id, "n_aaaaaaaa"), dir
}

// ff1JobStatus returns the runtime status of a cron job.
func ff1JobStatus(t *testing.T, cronMgr *CronManager, id string) CronJobRuntimeStatus {
	t.Helper()
	for _, job := range cronMgr.GetJobsWithRuntimeStatus() {
		if job.ID == id {
			return job
		}
	}
	t.Fatalf("cron job %s is not registered", id)
	return CronJobRuntimeStatus{}
}

// FF1 review: a read-only scheduler forbids changes to the schedule, not running it. Before
// FF1 the flow's persisted job was loaded and kept firing; the runtime-only job must be
// restored at Start the same way, without the scheduler mutation permission.
func TestFF1FlowScheduleIsRestoredWithAReadOnlyScheduler(t *testing.T) {
	cronMgr, jobID, dir := ff1RestartWithFlowSchedule(t, ff1SchedulerPermissions(true, true))
	status := ff1JobStatus(t, cronMgr, jobID)
	if status.Source != flowCronSource || !status.Registered || status.LastError != "" {
		t.Fatalf("flow job after a restart with a read-only scheduler = %+v", status)
	}
	if _, scheduled := cronMgr.NextRun(jobID); !scheduled {
		t.Fatal("the flow job is not scheduled")
	}
	if got := ff1PersistedFlowJobs(t, dir); len(got) != 0 {
		t.Fatalf("the restore persisted flow jobs: %v", got)
	}
}

// FF1 review: with the scheduler off at boot the flow job is kept like a loaded persisted
// job (no engine entry, "scheduler disabled by configuration"), and a live enable
// (RefreshRuntimePermissions) registers it.
func TestFF1FlowScheduleIsRegisteredWhenTheSchedulerIsEnabledLive(t *testing.T) {
	cronMgr, jobID, _ := ff1RestartWithFlowSchedule(t, ff1SchedulerPermissions(false, false))
	status := ff1JobStatus(t, cronMgr, jobID)
	if status.Registered || status.LastError != schedulerDisabledByConfiguration {
		t.Fatalf("flow job with the scheduler off = %+v", status)
	}
	if _, scheduled := cronMgr.NextRun(jobID); scheduled {
		t.Fatal("the flow job is scheduled while the scheduler is off")
	}
	ConfigureRuntimePermissions(ff1SchedulerPermissions(true, false))
	if err := cronMgr.RefreshRuntimePermissions(); err != nil {
		t.Fatalf("RefreshRuntimePermissions: %v", err)
	}
	if status := ff1JobStatus(t, cronMgr, jobID); !status.Registered || status.LastError != "" {
		t.Fatalf("flow job after the live enable = %+v", status)
	}
	if _, scheduled := cronMgr.NextRun(jobID); !scheduled {
		t.Fatal("the flow job is not scheduled after the live enable")
	}
}

// FF1 review, decision: publishing (and switching a flow on) still needs the scheduler
// mutation permission, as the persisted add needed before FF1; a read-only scheduler
// refuses a new schedule and reports it.
func TestFF1FlowSchedulePublishStillNeedsSchedulerMutation(t *testing.T) {
	ConfigureRuntimePermissions(ff1SchedulerPermissions(true, true))
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	dir := tempSystemTaskDir(t)
	cronMgr := NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	mm := NewMissionManagerV2(dir, cronMgr)
	id, err := mm.CreateFlowMission("flow_aaaaaaaaaa", "Plan")
	if err != nil {
		t.Fatal(err)
	}
	if err := mm.SyncFlowMission(id, "Plan", []FlowTriggerSpec{{NodeID: "n_aaaaaaaa", TriggerType: FlowTriggerSchedule, Schedule: "0 7 * * *"}}); err != nil {
		t.Fatal(err)
	}
	err = mm.SetFlowMissionEnabled(id, true)
	if err == nil || !strings.Contains(err.Error(), "scheduler mutation is disabled") {
		t.Fatalf("switching a flow on with a read-only scheduler = %v", err)
	}
	if hasCronJob(cronMgr, flowCronJobID(id, "n_aaaaaaaa")) {
		t.Fatal("a read-only scheduler accepted a new flow schedule")
	}
}
