package gamemaker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type countingRunner struct {
	service *Service
	calls   int
	drift   *WorkspaceDrift
}

func (r *countingRunner) RunGameMakerJob(ctx context.Context, run JobRun) error {
	r.calls++
	if run.Stage == "planning" {
		r.drift = run.WorkspaceDrift
	}
	return (testRunner{service: r.service}).RunGameMakerJob(ctx, run)
}

func TestAdoptPublishesWorkspaceEditsWithoutAModel(t *testing.T) {
	service := newTestService(t)
	runner := &countingRunner{service: service}
	service.SetRunner(runner)
	project := createTestProject(t, service, "2d")
	ctx := context.Background()
	first, err := service.StartJob(ctx, project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, service, first.ID); finished.Status != "ready" {
		t.Fatalf("create status %s: %s", finished.Status, finished.Error)
	}
	calls := runner.calls
	project, err = service.GetProject(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	published := project.CurrentRevision
	if _, err := service.StartJob(ctx, project.ID, StartJobRequest{AdoptWorkspace: true}); err == nil || !strings.Contains(err.Error(), "matches the published revision") {
		t.Fatalf("empty drift error = %v", err)
	}
	if _, err := service.StartJob(ctx, project.ID, StartJobRequest{AdoptWorkspace: true, Prompt: "change the score"}); err == nil || !strings.Contains(err.Error(), "cannot include") {
		t.Fatalf("conflicting adopt error = %v", err)
	}
	if jobs := countJobs(t, service, project.ID); jobs != 1 {
		t.Fatalf("rejected adopt created jobs: %d", jobs)
	}

	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	writeDriftFile(t, root, "notes.txt", "manual edit")
	adopted, err := service.StartJob(ctx, project.ID, StartJobRequest{AdoptWorkspace: true})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, service, adopted.ID)
	if finished.Status != "ready" {
		t.Fatalf("adopt status %s code %s: %s", finished.Status, finished.ErrorCode, finished.Error)
	}
	if runner.calls != calls {
		t.Fatalf("adopt called the model %d extra times", runner.calls-calls)
	}
	if finished.Kind != "adopt" || finished.ResultRevision != published+1 {
		t.Fatalf("adopt job = %+v", finished)
	}
	if finished.RepairPasses == nil || *finished.RepairPasses != 0 || finished.ChecksPassed == nil {
		t.Fatalf("adopt outcome = passes %v checks %v", finished.RepairPasses, finished.ChecksPassed)
	}
	revisions, err := service.ListRevisions(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if revisions[0].Source != "adopt" || revisions[0].Summary != adoptPublicationSummary {
		t.Fatalf("revision source = %s %q", revisions[0].Source, revisions[0].Summary)
	}
	if _, err := os.Stat(filepath.Join(root, "notes.txt")); err != nil {
		t.Fatalf("published edit missing: %v", err)
	}
	messages, err := service.ListMessages(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if message.JobID == adopted.ID && message.Role == "user" {
			t.Fatal("adopt stored an empty user message")
		}
	}
	summary, err := service.LatestPlanSummary(ctx, project.ID)
	if err != nil || summary["template"] == "" {
		t.Fatalf("plan summary = %#v err %v", summary, err)
	}
}

func TestAdoptStopsOnACorruptPlanWithoutPublishing(t *testing.T) {
	service, project, runner, calls := publishTestGame(t)
	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	writeDriftFile(t, root, "notes.txt", "visible drift")
	writeDriftFile(t, root, ".aurago/game-plan.json", "{")
	before := project.CurrentRevision
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{AdoptWorkspace: true})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, service, job.ID)
	if finished.Status != "failed" || finished.ErrorCode != "planning" {
		t.Fatalf("status %s code %s error %s", finished.Status, finished.ErrorCode, finished.Error)
	}
	if runner.calls != calls {
		t.Fatalf("planning failure called the model")
	}
	reloaded, err := service.GetProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CurrentRevision != before {
		t.Fatalf("revision changed to %d", reloaded.CurrentRevision)
	}
	if finished.ChecksPassed != nil || finished.RepairPasses != nil {
		t.Fatalf("planning failure stored checks %+v", finished)
	}
}

func TestAdoptKeepsTheRevisionWhenValidationFails(t *testing.T) {
	service, project, runner, calls := publishTestGame(t)
	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	writeDriftFile(t, root, "src/main.ts", "<<<\n")
	before := project.CurrentRevision
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{AdoptWorkspace: true})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, service, job.ID)
	if finished.Status != "failed" || finished.ErrorCode != "validation" {
		t.Fatalf("status %s code %s error %s", finished.Status, finished.ErrorCode, finished.Error)
	}
	if runner.calls != calls {
		t.Fatal("validation failure called the model")
	}
	if finished.RepairPasses == nil || *finished.RepairPasses != 0 {
		t.Fatalf("repair passes = %v", finished.RepairPasses)
	}
	reloaded, err := service.GetProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CurrentRevision != before {
		t.Fatalf("revision changed to %d", reloaded.CurrentRevision)
	}
}

func TestEditPlanningContextNamesWorkspaceDrift(t *testing.T) {
	service, project, runner, calls := publishTestGame(t)
	root := filepath.Join(service.opts.WorkspacePath, filepath.FromSlash(project.ProjectKey))
	writeDriftFile(t, root, "notes.txt", "carry this into the next edit")
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "Mention the notebook in the score label."})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, service, job.ID)
	if finished.Status != "ready" {
		t.Fatalf("edit status %s: %s", finished.Status, finished.Error)
	}
	if runner.calls <= calls {
		t.Fatal("edit did not call the model")
	}
	if runner.drift == nil || runner.drift.AddedCount != 1 || len(runner.drift.Added) != 1 || runner.drift.Added[0] != "notes.txt" {
		t.Fatalf("planning drift = %+v", runner.drift)
	}
}

func publishTestGame(t *testing.T) (*Service, Project, *countingRunner, int) {
	t.Helper()
	service := newTestService(t)
	runner := &countingRunner{service: service}
	service.SetRunner(runner)
	project := createTestProject(t, service, "2d")
	job, err := service.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitJob(t, service, job.ID); finished.Status != "ready" {
		t.Fatalf("create status %s: %s", finished.Status, finished.Error)
	}
	project, err = service.GetProject(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	return service, project, runner, runner.calls
}

func countJobs(t *testing.T, service *Service, projectID string) int {
	t.Helper()
	var count int
	if err := service.db.QueryRow(`SELECT COUNT(*) FROM gm_jobs WHERE project_id=?`, projectID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
