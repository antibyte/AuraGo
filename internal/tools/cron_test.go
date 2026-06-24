package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCronManagerDeniesMutationWithoutRuntimePolicy(t *testing.T) {
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	mgr := NewCronManager(tempSystemTaskDir(t))
	t.Cleanup(func() { _ = mgr.Close() })

	result, err := mgr.ManageSchedule("add", "job-1", "0 * * * *", "run cleanup", "en")
	if err != nil {
		t.Fatalf("ManageSchedule add returned unexpected error: %v", err)
	}
	if !strings.Contains(result, "scheduler is disabled") {
		t.Fatalf("ManageSchedule add = %s, want scheduler permission denial", result)
	}
}

func TestCronManagerReadOnlyRuntimePolicyAllowsListOnly(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{SchedulerEnabled: true, SchedulerReadOnly: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	mgr := NewCronManager(tempSystemTaskDir(t))
	t.Cleanup(func() { _ = mgr.Close() })

	listResult, err := mgr.ManageSchedule("list", "", "", "", "en")
	if err != nil {
		t.Fatalf("ManageSchedule list returned unexpected error: %v", err)
	}
	if !strings.Contains(listResult, `"status": "success"`) {
		t.Fatalf("ManageSchedule list = %s, want success", listResult)
	}

	addResult, err := mgr.ManageSchedule("add", "job-1", "0 * * * *", "run cleanup", "en")
	if err != nil {
		t.Fatalf("ManageSchedule add returned unexpected error: %v", err)
	}
	if !strings.Contains(addResult, "scheduler mutation is disabled") {
		t.Fatalf("ManageSchedule add = %s, want readonly mutation denial", addResult)
	}
}

func TestCronManagerDisabledRuntimePolicyAllowsListOnly(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{SchedulerEnabled: false})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	dir := tempSystemTaskDir(t)
	seed := NewCronManager(dir)
	if err := seed.store.save(systemTaskNamespaceCron, []CronJob{
		{ID: "disabled-runtime-job", CronExpr: "0 * * * *", TaskPrompt: "run later"},
	}); err != nil {
		t.Fatalf("seed cron store: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed cron manager: %v", err)
	}

	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })
	if err := mgr.Start(func(string) {}); err != nil {
		t.Fatalf("Start with scheduler disabled: %v", err)
	}

	statuses := mgr.GetJobsWithRuntimeStatus()
	if len(statuses) != 1 {
		t.Fatalf("runtime status count = %d, want 1", len(statuses))
	}
	if statuses[0].Registered {
		t.Fatalf("disabled scheduler registered job: %+v", statuses[0])
	}
	if !strings.Contains(statuses[0].LastError, "scheduler disabled by configuration") {
		t.Fatalf("LastError = %q, want scheduler disabled message", statuses[0].LastError)
	}

	listResult, err := mgr.ManageSchedule("list", "", "", "", "en")
	if err != nil {
		t.Fatalf("ManageSchedule list returned unexpected error: %v", err)
	}
	if !strings.Contains(listResult, "scheduler disabled by configuration") {
		t.Fatalf("ManageSchedule list = %s, want disabled runtime status", listResult)
	}

	addResult, err := mgr.ManageSchedule("add", "job-1", "0 * * * *", "run cleanup", "en")
	if err != nil {
		t.Fatalf("ManageSchedule add returned unexpected error: %v", err)
	}
	if !strings.Contains(addResult, "scheduler is disabled") {
		t.Fatalf("ManageSchedule add = %s, want scheduler permission denial", addResult)
	}
}

func TestCronManagerPersistsToSQLiteStore(t *testing.T) {
	dir := tempSystemTaskDir(t)
	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })
	mgr.callback = func(prompt string) {}

	if _, err := mgr.ManageSchedule("add", "job-1", "0 * * * *", "run cleanup", "en"); err != nil {
		t.Fatalf("ManageSchedule add: %v", err)
	}

	target := filepath.Join(dir, systemTaskStoreFile)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected sqlite system task store to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "crontab.json.tmp")); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover temp file, got err=%v", err)
	}

	var jobs []CronJob
	loaded, err := mgr.store.load(systemTaskNamespaceCron, &jobs)
	if err != nil {
		t.Fatalf("load persisted cron jobs: %v", err)
	}
	if !loaded {
		t.Fatal("expected persisted cron jobs in sqlite store")
	}
	if len(jobs) != 1 || jobs[0].ID != "job-1" {
		data, _ := json.Marshal(jobs)
		t.Fatalf("unexpected persisted jobs: %s", data)
	}
}

func TestCronManagerMigratesLegacyJSON(t *testing.T) {
	dir := tempSystemTaskDir(t)
	legacy := filepath.Join(dir, "crontab.json")
	if err := os.WriteFile(legacy, []byte(`[{"id":"legacy-job","cron_expr":"0 0 * * *","task_prompt":"legacy prompt"}]`), 0o644); err != nil {
		t.Fatalf("write legacy cron json: %v", err)
	}

	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })
	if err := mgr.Start(func(string) {}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	jobs := mgr.GetJobs()
	if len(jobs) != 1 || jobs[0].ID != "legacy-job" {
		t.Fatalf("unexpected migrated jobs: %+v", jobs)
	}

	var stored []CronJob
	loaded, err := mgr.store.load(systemTaskNamespaceCron, &stored)
	if err != nil {
		t.Fatalf("load migrated cron jobs: %v", err)
	}
	if !loaded || len(stored) != 1 || stored[0].ID != "legacy-job" {
		t.Fatalf("unexpected store contents after migration: %+v", stored)
	}
	if data, err := os.ReadFile(legacy); err != nil || !strings.Contains(string(data), "legacy-job") {
		t.Fatalf("expected legacy file to remain readable, err=%v data=%q", err, string(data))
	}
}

func TestCronManagerStartReportsPersistedScheduleRegistrationFailures(t *testing.T) {
	dir := tempSystemTaskDir(t)
	seed := NewCronManager(dir)
	if err := seed.store.save(systemTaskNamespaceCron, []CronJob{
		{ID: "good-job", CronExpr: "0 * * * *", TaskPrompt: "run good job"},
		{ID: "bad-job", CronExpr: "not a cron expression", TaskPrompt: "run bad job"},
	}); err != nil {
		t.Fatalf("seed cron store: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed cron manager: %v", err)
	}

	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })

	err := mgr.Start(func(string) {})
	if err == nil {
		t.Fatal("Start returned nil, want registration error for bad persisted cron job")
	}
	if !strings.Contains(err.Error(), "bad-job") {
		t.Fatalf("Start error = %v, want bad job id", err)
	}
	if !strings.Contains(err.Error(), "not a cron expression") {
		t.Fatalf("Start error = %v, want invalid cron expression", err)
	}
	if _, ok := mgr.cronEntryIDs["good-job"]; !ok {
		t.Fatal("valid persisted cron job was not registered")
	}
	if _, ok := mgr.cronEntryIDs["bad-job"]; ok {
		t.Fatal("invalid persisted cron job should not have a registered cron entry")
	}
}

func TestCronManagerListReportsRuntimeRegistrationStatus(t *testing.T) {
	dir := tempSystemTaskDir(t)
	seed := NewCronManager(dir)
	if err := seed.store.save(systemTaskNamespaceCron, []CronJob{
		{ID: "good-job", CronExpr: "0 * * * *", TaskPrompt: "run good job"},
		{ID: "bad-job", CronExpr: "not a cron expression", TaskPrompt: "run bad job"},
	}); err != nil {
		t.Fatalf("seed cron store: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed cron manager: %v", err)
	}

	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })
	if err := mgr.Start(func(string) {}); err == nil {
		t.Fatal("Start returned nil, want registration error for bad persisted cron job")
	}

	result, err := mgr.ManageSchedule("list", "", "", "", "en")
	if err != nil {
		t.Fatalf("ManageSchedule list: %v", err)
	}
	var payload struct {
		Status string `json:"status"`
		Jobs   []struct {
			ID         string `json:"id"`
			Registered bool   `json:"registered"`
			LastError  string `json:"last_error"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("parse list result: %v\n%s", err, result)
	}
	statusByID := map[string]struct {
		registered bool
		lastError  string
	}{}
	for _, job := range payload.Jobs {
		statusByID[job.ID] = struct {
			registered bool
			lastError  string
		}{registered: job.Registered, lastError: job.LastError}
	}
	if !statusByID["good-job"].registered {
		t.Fatalf("good job runtime status = %+v, want registered", statusByID["good-job"])
	}
	if statusByID["bad-job"].registered {
		t.Fatalf("bad job runtime status = %+v, want not registered", statusByID["bad-job"])
	}
	if !strings.Contains(statusByID["bad-job"].lastError, "not a cron expression") {
		t.Fatalf("bad job last_error = %q, want parse error", statusByID["bad-job"].lastError)
	}
}

func TestCronManagerAcceptsSecondsFieldExpressions(t *testing.T) {
	mgr := NewCronManager(tempSystemTaskDir(t))
	t.Cleanup(func() { _ = mgr.Close() })
	mgr.callback = func(prompt string) {}
	result, err := mgr.ManageSchedule("add", "job-seconds", "0 */15 * * * *", "run every fifteen minutes", "en")
	if err != nil {
		t.Fatalf("ManageSchedule add with seconds field: %v", err)
	}
	if !strings.Contains(result, `"status": "success"`) {
		t.Fatalf("expected success response, got %s", result)
	}
}

func TestCronManagerIdempotentAddReplacesDisabledJob(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	mgr := NewCronManager(tempSystemTaskDir(t))
	t.Cleanup(func() { _ = mgr.Close() })

	if _, err := mgr.ManageSchedule("add", "job-disabled", "0 8 * * *", "old prompt", "en"); err != nil {
		t.Fatalf("ManageSchedule add old job: %v", err)
	}
	if _, err := mgr.ManageSchedule("disable", "job-disabled", "", "", "en"); err != nil {
		t.Fatalf("ManageSchedule disable old job: %v", err)
	}
	if _, err := mgr.ManageSchedule("add", "job-disabled", "0 9 * * *", "new prompt", "en"); err != nil {
		t.Fatalf("ManageSchedule add replacement: %v", err)
	}

	jobs := mgr.GetJobs()
	if len(jobs) != 1 {
		t.Fatalf("job count after replacing disabled job = %d, want 1: %+v", len(jobs), jobs)
	}
	if jobs[0].Disabled || jobs[0].CronExpr != "0 9 * * *" || jobs[0].TaskPrompt != "new prompt" {
		t.Fatalf("replacement job = %+v, want enabled new job", jobs[0])
	}
	statuses := mgr.GetJobsWithRuntimeStatus()
	if len(statuses) != 1 || !statuses[0].Registered {
		t.Fatalf("runtime statuses after replacement = %+v, want one registered job", statuses)
	}
}

func TestCronManagerRefreshRuntimePermissionsReregistersValidJobs(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{SchedulerEnabled: false})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	dir := tempSystemTaskDir(t)
	seed := NewCronManager(dir)
	if err := seed.store.save(systemTaskNamespaceCron, []CronJob{
		{ID: "good-job", CronExpr: "0 * * * *", TaskPrompt: "run good job"},
		{ID: "bad-job", CronExpr: "not a cron expression", TaskPrompt: "run bad job"},
	}); err != nil {
		t.Fatalf("seed cron store: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed cron manager: %v", err)
	}

	mgr := NewCronManager(dir)
	t.Cleanup(func() { _ = mgr.Close() })
	if err := mgr.Start(func(string) {}); err != nil {
		t.Fatalf("Start with scheduler disabled: %v", err)
	}

	ConfigureRuntimePermissions(RuntimePermissions{SchedulerEnabled: true})
	err := mgr.RefreshRuntimePermissions()
	if err == nil {
		t.Fatal("RefreshRuntimePermissions returned nil, want bad persisted cron error")
	}

	statusByID := map[string]CronJobRuntimeStatus{}
	for _, status := range mgr.GetJobsWithRuntimeStatus() {
		statusByID[status.ID] = status
	}
	if !statusByID["good-job"].Registered {
		t.Fatalf("good job status = %+v, want registered", statusByID["good-job"])
	}
	if statusByID["bad-job"].Registered {
		t.Fatalf("bad job status = %+v, want not registered", statusByID["bad-job"])
	}
	if !strings.Contains(statusByID["bad-job"].LastError, "not a cron expression") {
		t.Fatalf("bad job LastError = %q, want parse error", statusByID["bad-job"].LastError)
	}
}

func TestCronManagerRemovesDisabledJobs(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	mgr := NewCronManager(tempSystemTaskDir(t))
	t.Cleanup(func() { _ = mgr.Close() })

	if _, err := mgr.ManageSchedule("add", "disabled-job", "0 8 * * *", "run disabled job", "en"); err != nil {
		t.Fatalf("ManageSchedule add: %v", err)
	}
	if _, err := mgr.ManageSchedule("disable", "disabled-job", "", "", "en"); err != nil {
		t.Fatalf("ManageSchedule disable: %v", err)
	}
	result, err := mgr.ManageSchedule("remove", "disabled-job", "", "", "en")
	if err != nil {
		t.Fatalf("ManageSchedule remove disabled job: %v", err)
	}
	if !strings.Contains(result, `"status": "success"`) {
		t.Fatalf("remove disabled job result = %s, want success", result)
	}
	if jobs := mgr.GetJobs(); len(jobs) != 0 {
		t.Fatalf("jobs after disabled remove = %+v, want none", jobs)
	}
}
