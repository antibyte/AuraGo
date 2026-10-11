package cydcreator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStartJobValidatesTheRequest(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: "rewrite", Prompt: "x"}), CodeBadRequest)
	wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeNew}), CodeBadRequest)
	wantCode(t, mustStartErr(env, "ccp_missing", StartJobInput{Mode: JobModeNew, Prompt: "x"}), CodeNotFound)
	wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeChange, Resume: true}), CodeJobState)

	layout, v, _, _ := env.svc.GetLayout(ctx, p.ID)
	layout.Screens[0].Widgets = append(layout.Screens[0].Widgets, Widget{ID: "w_bad", Type: "label", Name: "Bad Name", X: 0, Y: 0, W: 50, H: 20})
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, v, OriginUser); err != nil {
		t.Fatal(err)
	}
	e := wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeGenerateCode}), CodeInvalidLayout)
	if issues, _ := e.Details["issues"].([]Issue); len(issues) == 0 {
		t.Fatalf("details = %+v", e.Details)
	}
	env.svc.SetRunner(nil)
	wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"}), CodeDisabled)
}

func mustStartErr(env *testEnv, projectID string, in StartJobInput) error {
	_, err := env.svc.StartJob(context.Background(), projectID, in)
	return err
}

func TestNewJobPlansWaitsForConfirmationAndPublishes(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()

	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeNew, Prompt: "Build a weather display"})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != JobQueued || job.Mode != JobModeNew || job.Phase != PhaseQueued {
		t.Fatalf("started job = %+v", job)
	}
	waiting := waitJob(t, env, job.ID, JobAwaitingConfirmation)
	if waiting.Phase != PhasePlanning || waiting.Plan == nil || waiting.Plan.Summary != "Weather display with two screens" || waiting.StartedAt == "" {
		t.Fatalf("waiting job = %+v", waiting)
	}
	if got := runner.phaseList(); !slices.Equal(got, []string{PhasePlanning}) {
		t.Fatalf("phases before confirmation = %v", got)
	}
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if summary.LockedByJob != job.ID {
		t.Fatalf("project lock while waiting = %q", summary.LockedByJob)
	}
	caps := env.svc.Capabilities(ctx)
	if caps.ActiveJob == nil || caps.ActiveJob.JobID != job.ID || caps.ActiveJob.Status != JobAwaitingConfirmation {
		t.Fatalf("active job = %+v", caps.ActiveJob)
	}
	other, _ := env.svc.CreateProject(ctx, CreateProjectInput{Name: "Other", Board: "esp32-2432s028r"})
	wantCode(t, mustStartErr(env, other.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"}), CodeBusy)

	confirmed, err := env.svc.ConfirmJob(ctx, job.ID, "")
	if err != nil || confirmed.Status != JobRunning {
		t.Fatalf("confirm = %+v, %v", confirmed, err)
	}
	done := waitJob(t, env, job.ID, JobSucceeded, JobFailed)
	if done.Status != JobSucceeded || done.Phase != PhaseDone || done.BuildID == "" || done.FinishedAt == "" {
		t.Fatalf("finished job = %+v", done)
	}
	if got := runner.phaseList(); !slices.Equal(got, []string{PhasePlanning, PhaseLayout, PhaseCoding}) {
		t.Fatalf("phases = %v", got)
	}
	if layoutRun := runner.at(1); layoutRun.Plan == nil || layoutRun.Plan.Summary == "" {
		t.Fatalf("layout phase without plan: %+v", layoutRun)
	}
	if src := readProjectFile(t, env, p, "src/app/app_main.cpp"); !strings.Contains(src, "// agent") {
		t.Fatalf("published app file = %q", src)
	}
	layout, version, _, _ := env.svc.GetLayout(ctx, p.ID)
	if _, _, ok := layout.FindWidget("lbl_temp"); !ok || version != p.Version+1 {
		t.Fatalf("published layout v%d = %+v", version, layout.Screens[0].Widgets)
	}
	summary, _ = env.svc.GetProject(ctx, p.ID)
	if summary.LockedByJob != "" || summary.HasDraft {
		t.Fatalf("project after success = %+v", summary)
	}
	revs, _ := env.svc.ListRevisions(ctx, p.ID)
	if len(revs) != 2 || revs[0].Reason != RevisionJob || revs[0].BuildID != done.BuildID || revs[1].Reason != RevisionBeforeJob || revs[0].JobID != job.ID {
		t.Fatalf("revisions = %+v", revs)
	}
	msgs, _ := env.svc.ListMessages(ctx, p.ID)
	var roles []string
	for _, m := range msgs {
		roles = append(roles, m.Role+":"+m.Text)
	}
	if !slices.Equal(roles, []string{"user:Build a weather display", "agent:Weather display with two screens", "agent:Shows the temperature."}) {
		t.Fatalf("messages = %v", roles)
	}
	got := drain(events)
	types := eventTypes(got)
	for _, want := range []string{EventLockChanged, EventJobStatus, EventJobStep, EventJobMessage, EventBuildStatus, EventProjectChanged} {
		if !slices.Contains(types, want) {
			t.Fatalf("missing %s in %v", want, types)
		}
	}
	var origins []string
	for _, ev := range got {
		if ev.Type == EventProjectChanged {
			var d struct{ Origin, What string }
			_ = json.Unmarshal(ev.Data, &d)
			origins = append(origins, d.Origin+"/"+d.What)
		}
	}
	if !slices.Contains(origins, "job:"+job.ID+"/layout") || !slices.Contains(origins, "job:"+job.ID+"/files") {
		t.Fatalf("project_changed = %v", origins)
	}
	last := got[len(got)-1]
	if last.Type != EventJobMessage {
		t.Fatalf("last event = %s", last.Type)
	}
}

func TestConfirmJobWithAnEditedPromptPlansAgain(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	ctx := context.Background()
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeNew, Prompt: "first idea"})
	waitJob(t, env, job.ID, JobAwaitingConfirmation)
	if _, err := env.svc.ConfirmJob(ctx, job.ID, "second idea with a clock"); err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobAwaitingConfirmation)
	if got := runner.phaseList(); !slices.Equal(got, []string{PhasePlanning, PhasePlanning}) || runner.last().Prompt != "second idea with a clock" {
		t.Fatalf("phases = %v, prompt = %q", got, runner.last().Prompt)
	}
	msgs, _ := env.svc.ListMessages(ctx, p.ID)
	if msgs[len(msgs)-2].Text != "second idea with a clock" {
		t.Fatalf("messages = %+v", msgs)
	}
	wantCode(t, confirmErr(env, "ccj_missing"), CodeJobNotFound)
	if err := env.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, confirmErr(env, job.ID), CodeJobState)
}

func confirmErr(env *testEnv, jobID string) error {
	_, err := env.svc.ConfirmJob(context.Background(), jobID, "")
	return err
}

func TestPlanningWithoutAPlanFails(t *testing.T) {
	env, _, p := newJobEnv(t)
	job, _ := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeNew, Prompt: "x"})
	done := waitJob(t, env, job.ID, JobFailed)
	if !strings.Contains(done.ErrorMessage, "without submitting a plan") {
		t.Fatalf("job = %+v", done)
	}
	summary, _ := env.svc.GetProject(context.Background(), p.ID)
	if summary.LockedByJob != "" || summary.HasDraft {
		t.Fatalf("project = %+v", summary)
	}
}

func compileError(req BuildRequest) BuildOutput {
	return BuildOutput{ExitCode: BuildExitCompileError, Log: "/work/" + req.BuildID + "/src/app/app_main.cpp:4:3: error: 'foo' was not declared in this scope\n" +
		"/work/" + req.BuildID + "/src/generated/ui.cpp:9:1: warning: unused variable 'x'\n"}
}

func TestChangeJobRepairsUntilTheBuildIsGreen(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	builds := 0
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		builds++
		if builds == 1 {
			return compileError(req), nil
		}
		return successOutput(req), nil
	}
	runner.on(PhaseRepairing, func(ctx context.Context, run JobRun) error {
		if len(run.Diagnostics) != 1 || run.Diagnostics[0].Generated || run.BuildErrorCode != CodeBuildFailed || run.RepairAttempt != 1 {
			return errors.New("unexpected repair input")
		}
		_, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/app_main.cpp", "#include \"ui.h\"\nvoid app_setup() {}\nvoid app_loop() {}\n// fixed\n")
		return err
	})
	job, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "Add a clock"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, job.ID, JobSucceeded, JobFailed)
	if done.Status != JobSucceeded || done.RepairAttempt != 1 {
		t.Fatalf("job = %+v", done)
	}
	if got := runner.phaseList(); !slices.Equal(got, []string{PhaseCoding, PhaseRepairing}) {
		t.Fatalf("phases = %v", got)
	}
	if src := readProjectFile(t, env, p, "src/app/app_main.cpp"); !strings.Contains(src, "// fixed") {
		t.Fatalf("published = %q", src)
	}
}

func TestRepairBudgetExhaustedKeepsTheProjectAndADraft(t *testing.T) {
	env, runner, p := newJobEnv(t, testServiceOptions{limits: ServiceLimits{MaxRepairAttempts: 1, JobTimeout: time.Minute}})
	scriptSuccess(t, env, runner)
	before := readProjectFile(t, env, p, "src/app/app_main.cpp")
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		return compileError(req), nil
	}
	job, _ := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "Add a clock"})
	done := waitJob(t, env, job.ID, JobFailed, JobSucceeded)
	if done.Status != JobFailed || done.ErrorCode != CodeBuildFailed || done.RepairAttempt != 1 || !strings.Contains(done.ErrorMessage, "after 1 repair") {
		t.Fatalf("job = %+v", done)
	}
	if got := readProjectFile(t, env, p, "src/app/app_main.cpp"); got != before {
		t.Fatalf("project changed: %q", got)
	}
	summary, _ := env.svc.GetProject(context.Background(), p.ID)
	if !summary.HasDraft || summary.LockedByJob != "" {
		t.Fatalf("project = %+v", summary)
	}
	msgs, _ := env.svc.ListMessages(context.Background(), p.ID)
	if last := msgs[len(msgs)-1]; last.Role != MessageRoleSystem || !strings.Contains(last.Text, "draft") {
		t.Fatalf("last message = %+v", last)
	}
}

func TestGeneratorErrorsAreNotRepaired(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		return BuildOutput{ExitCode: BuildExitCompileError, Log: "/work/" + req.BuildID + "/src/generated/ui.cpp:9:1: error: boom\n"}, nil
	}
	job, _ := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeGenerateCode})
	done := waitJob(t, env, job.ID, JobFailed, JobSucceeded)
	if done.ErrorCode != CodeGeneratorError || done.RepairAttempt != 0 || slices.Contains(runner.phaseList(), PhaseRepairing) {
		t.Fatalf("job = %+v, phases %v", done, runner.phaseList())
	}
	if runner.at(0).Prompt != generateCodePrompt {
		t.Fatalf("generate_code prompt = %q", runner.at(0).Prompt)
	}
}

func TestAgentBuildIsReusedWhenNothingChanged(t *testing.T) {
	env, runner, p := newJobEnv(t)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int x = 1;\n"); err != nil {
			return err
		}
		res, err := env.svc.BuildForJob(ctx, run.Job.ID)
		if err != nil || res.Status != BuildSucceeded || !strings.Contains(res.NextAction, "cyd_creator_finish") {
			return errors.New("agent build failed")
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "done")
	})
	job, _ := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	done := waitJob(t, env, job.ID, JobSucceeded, JobFailed)
	if done.Status != JobSucceeded || env.builder.requestCount() != 1 {
		t.Fatalf("job = %+v, builds = %d", done, env.builder.requestCount())
	}
}

func TestCancelRunningJobKeepsADraft(t *testing.T) {
	env, runner, p := newJobEnv(t)
	started := make(chan struct{})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int y;\n"); err != nil {
			return err
		}
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	ctx := context.Background()
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	<-started
	if err := env.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, job.ID, JobCancelled)
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if done.ErrorMessage != "" || !summary.HasDraft || summary.LockedByJob != "" {
		t.Fatalf("job = %+v, project = %+v", done, summary)
	}
	wantCode(t, env.svc.CancelJob(ctx, job.ID), CodeJobState)
	wantCode(t, env.svc.CancelJob(ctx, "ccj_missing"), CodeJobNotFound)
	// The lock is gone: user writes work again.
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/user.cpp", []byte("int z;\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
}

func TestCancelAwaitingJobLeavesNoDraft(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	ctx := context.Background()
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeNew, Prompt: "x"})
	waitJob(t, env, job.ID, JobAwaitingConfirmation)
	if err := env.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	done, _ := env.svc.GetJob(ctx, job.ID)
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if done.Status != JobCancelled || summary.HasDraft || summary.LockedByJob != "" || dirExists(env.svc.stagingPath(job.ID)) {
		t.Fatalf("job = %+v, project = %+v", done, summary)
	}
}

// A job stores its terminal status before its run leaves the agent slot. A
// cancel in that finishing window is CYD_JOB_STATE and leaves the run alone;
// the window is reproduced by storing the terminal status while the run
// still holds the slot.
func TestCancelInTheFinishingWindowIsAJobStateError(t *testing.T) {
	env, runner, p := newJobEnv(t)
	started, release := make(chan struct{}), make(chan struct{})
	releaseRun := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseRun) // before the service closes (cleanups run last-in first-out)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		close(started)
		<-release
		return ctx.Err()
	})
	ctx := context.Background()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	waitClosed(t, started, "the coding round")
	run := env.svc.activeRun()
	if run == nil || run.jobID != job.ID {
		t.Fatalf("active run = %+v", run)
	}
	if _, err := env.svc.db.Exec(`UPDATE cc_jobs SET status=? WHERE id=?`, JobSucceeded, job.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, env.svc.CancelJob(ctx, job.ID), CodeJobState)
	if run.ctx.Err() != nil {
		t.Fatalf("the finishing run was cancelled: %v", context.Cause(run.ctx))
	}
	// Still running: the cancel reaches the run again.
	if _, err := env.svc.db.Exec(`UPDATE cc_jobs SET status=? WHERE id=?`, JobRunning, job.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	releaseRun()
	if done := waitJob(t, env, job.ID, JobCancelled, JobSucceeded, JobFailed); done.Status != JobCancelled {
		t.Fatalf("job = %+v", done)
	}
}

func TestProjectIsLockedWhileAJobRuns(t *testing.T) {
	env, runner, p := newJobEnv(t)
	release := make(chan struct{})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		<-release
		return nil
	})
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	ev := collectEvents(t, events, 1, 2*time.Second, EventLockChanged)[0]
	if string(ev.Data) != `{"locked_by_job":"`+job.ID+`"}` {
		t.Fatalf("lock_changed = %s", ev.Data)
	}
	layout, v, _, _ := env.svc.GetLayout(ctx, p.ID)
	_, _, err := env.svc.PutLayout(ctx, p.ID, layout, v, OriginUser)
	wantCode(t, err, CodeProjectLocked)
	_, err = env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	wantCode(t, err, CodeProjectLocked)
	wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "y"}), CodeBusy)
	close(release)
	waitJob(t, env, job.ID, JobSucceeded, JobFailed)
	ev = collectEvents(t, events, 1, 2*time.Second, EventLockChanged)[0]
	if string(ev.Data) != `{"locked_by_job":""}` {
		t.Fatalf("lock release = %s", ev.Data)
	}
}

func TestJobTimeoutFailsTheJob(t *testing.T) {
	env, runner, p := newJobEnv(t, testServiceOptions{limits: ServiceLimits{JobTimeout: 150 * time.Millisecond}})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		<-ctx.Done()
		return ctx.Err()
	})
	job, _ := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	done := waitJob(t, env, job.ID, JobFailed)
	if !strings.Contains(done.ErrorMessage, "time limit") {
		t.Fatalf("job = %+v", done)
	}
}

// The job's timers are fakes here (review fix): with real 300 ms timers a slow
// write on a loaded machine crossed the deadline and the test flaked.
func TestProductiveJobGetsOneGraceWindow(t *testing.T) {
	env, runner, p := newJobEnv(t, testServiceOptions{limits: ServiceLimits{JobTimeout: 30 * time.Second}})
	timers := useFakeTimers(env)
	wrote := make(chan struct{})
	ended := make(chan error, 1)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/progress.cpp", "int p;\n"); err != nil {
			return err
		}
		close(wrote)
		<-ctx.Done()
		ended <- context.Cause(ctx)
		return ctx.Err()
	})
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	job, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	budget := timers.next(t)
	waitClosed(t, wrote, "the agent's write")
	budget.fire()
	grace := timers.next(t)
	if budget.d != 30*time.Second || grace.d != 15*time.Second {
		t.Fatalf("timers = %v then %v, want 30s then 15s", budget.d, grace.d)
	}
	select {
	case cause := <-ended:
		t.Fatalf("the job ended inside its grace window: %v", cause)
	default:
	}
	grace.fire()
	select {
	case cause := <-ended:
		if !errors.Is(cause, errJobTimeout) {
			t.Fatalf("cause = %v", cause)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the job did not end after its grace window")
	}
	done := waitJob(t, env, job.ID, JobFailed)
	extended := false
	for _, ev := range drain(events) {
		if ev.Type == EventJobStep && strings.Contains(string(ev.Data), "the job gets 15s more to finish") {
			extended = true
		}
	}
	if !extended || !strings.Contains(done.ErrorMessage, "time limit of 30s") {
		t.Fatalf("grace step %v; job = %+v", extended, done)
	}
}

func TestResumeContinuesTheDraft(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/draft.cpp", "int draft;\n"); err != nil {
			return err
		}
		return errors.New("model went away")
	})
	first, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "add draft"})
	failed := waitJob(t, env, first.ID, JobFailed)
	if !strings.Contains(failed.ErrorMessage, "model went away") {
		t.Fatalf("failed job = %+v", failed)
	}
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if !run.Resumed || run.Job.ResumeOf != first.ID || run.Prompt != continuePrompt {
			return errors.New("not a resume")
		}
		f, err := env.svc.ReadJobFile(ctx, run.Job.ID, "src/app/draft.cpp")
		if err != nil || f.Content != "int draft;\n" {
			return errors.New("draft content missing")
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "resumed")
	})
	second, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, second.ID, JobSucceeded, JobFailed)
	if done.Status != JobSucceeded || done.ResumeOf != first.ID {
		t.Fatalf("resumed job = %+v", done)
	}
	if got := readProjectFile(t, env, p, "src/app/draft.cpp"); got != "int draft;\n" {
		t.Fatalf("published draft = %q", got)
	}
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if summary.HasDraft || dirExists(env.svc.stagingPath(first.ID)) {
		t.Fatalf("old draft kept: %+v", summary)
	}
}

func TestResumeIsRefusedAfterTheProjectChanged(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		_, _ = env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/draft.cpp", "int draft;\n")
		return errors.New("stop")
	})
	first, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	waitJob(t, env, first.ID, JobFailed)
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/mine.cpp", []byte("int mine;\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
	wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeChange, Resume: true}), CodeJobState)
	if err := env.svc.DiscardDraft(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if summary.HasDraft || dirExists(env.svc.stagingPath(first.ID)) {
		t.Fatalf("draft not discarded: %+v", summary)
	}
	if err := env.svc.DiscardDraft(ctx, p.ID); err != nil {
		t.Fatalf("second discard: %v", err)
	}
}

func TestAttachmentsAreStoredAndHandedToTheAgent(t *testing.T) {
	env, runner, p := newJobEnv(t)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		return env.svc.FinishJob(ctx, run.Job.ID, "looked at the log")
	})
	log := "Guru Meditation Error\nBacktrace: 0x400d1234"
	job, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Attachments: []Attachment{{Kind: "log", Text: log}}})
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobSucceeded, JobFailed)
	run := runner.at(0)
	if run.Prompt != attachedLogPrompt || len(run.Attachments) != 1 || run.Attachments[0].Text != log {
		t.Fatalf("run = %+v", run)
	}
	msgs, _ := env.svc.ListMessages(context.Background(), p.ID)
	if msgs[0].Role != MessageRoleUser || len(msgs[0].Attachments) != 1 || msgs[0].Attachments[0].Text != log || msgs[0].JobID != job.ID {
		t.Fatalf("message = %+v", msgs[0])
	}
}

func TestUserChangesSinceTheLastJobReachTheAgent(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	ctx := context.Background()
	first, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	waitJob(t, env, first.ID, JobSucceeded)
	if runner.at(0).UserChanges != nil {
		t.Fatalf("first job user changes = %+v", runner.at(0).UserChanges)
	}
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/mine.cpp", []byte("int mine;\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
	second, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "y"})
	waitJob(t, env, second.ID, JobSucceeded)
	changes := runner.last().UserChanges
	if changes == nil || changes.LayoutChanged || len(changes.Files) != 1 || changes.Files[0] != (UserFileChange{Path: "src/app/mine.cpp", Status: "added"}) {
		t.Fatalf("user changes = %+v", changes)
	}
}

func TestLogicHooksAreTheCodingTaskList(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	layout, v, _, _ := env.svc.GetLayout(ctx, p.ID)
	layout.Screens[0].Widgets = append(layout.Screens[0].Widgets, Widget{ID: "w_btn", Type: "button", Name: "btn_refresh", X: 10, Y: 10, W: 120, H: 40,
		Events: []EventBinding{{On: "clicked", Action: Action{Type: "logic", Hint: "reload the weather"}}}})
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, v, OriginUser); err != nil {
		t.Fatal(err)
	}
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		return env.svc.FinishJob(ctx, run.Job.ID, "ok")
	})
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeGenerateCode})
	waitJob(t, env, job.ID, JobSucceeded)
	want := LogicHook{Hook: "on_btn_refresh_clicked", Widget: "btn_refresh", Event: "clicked", Screen: "home", Hint: "reload the weather"}
	if hooks := runner.at(0).Hooks; len(hooks) != 1 || hooks[0] != want {
		t.Fatalf("hooks = %+v", hooks)
	}
}

func TestAgentContextIsSavedPerJobAndInheritedByResume(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		_, _ = env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/draft.cpp", "int d;\n")
		if err := env.svc.SaveAgentContext(ctx, run.Job.ID, []byte(`[{"role":"assistant","content":"step 1"}]`)); err != nil {
			return err
		}
		return errors.New("stop")
	})
	first, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	waitJob(t, env, first.ID, JobFailed)
	wantCode(t, env.svc.SaveAgentContext(ctx, first.ID, []byte(`[]`)), CodeJobState)
	var inherited []byte
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		var err error
		inherited, err = env.svc.LoadAgentContext(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "ok")
	})
	second, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Resume: true})
	waitJob(t, env, second.ID, JobSucceeded)
	if string(inherited) != `[{"role":"assistant","content":"step 1"}]` {
		t.Fatalf("inherited = %s", inherited)
	}
	var n int
	_ = env.svc.db.QueryRow(`SELECT COUNT(*) FROM cc_agent_context`).Scan(&n)
	if n != 0 {
		t.Fatalf("%d agent contexts left after success", n)
	}
}

func TestListJobsNewestFirst(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	ctx := context.Background()
	a, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "a"})
	waitJob(t, env, a.ID, JobSucceeded)
	b, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "b"})
	waitJob(t, env, b.ID, JobSucceeded)
	jobs, err := env.svc.ListJobs(ctx, p.ID)
	if err != nil || len(jobs) != 2 || jobs[0].ID != b.ID || jobs[1].ID != a.ID {
		t.Fatalf("jobs = %+v, %v", jobs, err)
	}
	if _, err := env.svc.ListJobs(ctx, "ccp_missing"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("missing project: %v", err)
	}
}

// The tests below pin rules the orchestrator owes plan 2 and the Task 1/2
// reviews: lock and fence refusal, lock event order, terminal job writes,
// publication details, tooling failures, job_step text and shutdown.

func TestStartJobRefusesLockedAndFencedProjects(t *testing.T) {
	env, _, p := newJobEnv(t)
	ctx := context.Background()
	env.lockForJob(t, p.ID, "ccj_foreign")
	e := wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"}), CodeBusy)
	if e.Details["job_id"] != "ccj_foreign" {
		t.Fatalf("locked project: details = %+v", e.Details)
	}
	env.lockForJob(t, p.ID, "")
	env.svc.setPublicationFence(p.ID, "publish_ccj_old")
	e = wantCode(t, mustStartErr(env, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"}), CodeBusy)
	if e.Details["reason"] != "publication_review" {
		t.Fatalf("fenced project: details = %+v", e.Details)
	}
	jobs, _ := env.svc.ListJobs(ctx, p.ID)
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if len(jobs) != 0 || summary.LockedByJob != "" || env.svc.Capabilities(ctx).ActiveJob != nil || env.svc.activeRun() != nil {
		t.Fatalf("a refused start left state behind: jobs %+v, project %+v", jobs, summary)
	}
}

func TestLockEventsFollowTheLockChangesAcrossJobs(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	var want []string
	for _, prompt := range []string{"first", "second"} {
		job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: prompt})
		if err != nil {
			t.Fatal(err)
		}
		waitJob(t, env, job.ID, JobSucceeded)
		want = append(want, job.ID, "")
	}
	lockValues := func(list []Event) []string {
		var out []string
		for _, ev := range list {
			if ev.Type == EventLockChanged {
				var d struct {
					LockedByJob string `json:"locked_by_job"`
				}
				_ = json.Unmarshal(ev.Data, &d)
				out = append(out, d.LockedByJob)
			}
		}
		return out
	}
	if got := lockValues(drain(events)); !slices.Equal(got, want) {
		t.Fatalf("live lock_changed = %q, want %q", got, want)
	}
	stored, err := env.svc.EventsAfter(ctx, p.ID, 0, 500)
	if err != nil {
		t.Fatal(err)
	}
	if got := lockValues(stored); !slices.Equal(got, want) {
		t.Fatalf("stored lock_changed = %q, want %q", got, want)
	}
}

func TestTerminalJobWriteIsRetried(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	// Three failed UPDATEs to status 'failed': more than one saveJob call
	// (two attempts) absorbs. RAISE(FAIL) keeps the counter update.
	for _, q := range []string{
		`CREATE TABLE test_fail_terminal(n INTEGER NOT NULL)`,
		`INSERT INTO test_fail_terminal(n) VALUES(3)`,
		`CREATE TRIGGER test_fail_terminal_t BEFORE UPDATE ON cc_jobs
			WHEN NEW.status='failed' AND (SELECT n FROM test_fail_terminal) > 0
			BEGIN UPDATE test_fail_terminal SET n=n-1; SELECT RAISE(FAIL, 'injected'); END`,
	} {
		if _, err := env.svc.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error { return errors.New("stop") })
	first, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, first.ID, JobFailed)
	var left int
	_ = env.svc.db.QueryRow(`SELECT n FROM test_fail_terminal`).Scan(&left)
	if left != 0 {
		t.Fatalf("%d injected failures left: the terminal write was not retried", left)
	}
	scriptSuccess(t, env, runner)
	second, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "y"})
	if err != nil {
		t.Fatalf("the agent slot stayed blocked: %v", err)
	}
	waitJob(t, env, second.ID, JobSucceeded)
}

func TestPublicationMirrorsOnlyValidLayoutMetaAndCommitsTheReceipt(t *testing.T) {
	lenient := func(in GenerateInput) (GenerateOutput, error) {
		return GenerateOutput{Files: map[string][]byte{"platformio.ini": []byte("[env]\n")}}, nil
	}
	env, runner, p := newJobEnv(t, testServiceOptions{generate: lenient})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		// A hand-edited working copy: a valid new name and an unknown theme.
		path := filepath.Join(env.svc.stagingPath(run.Job.ID), ProjectFile)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		layout, err := ParseProject(data)
		if err != nil {
			return err
		}
		layout.Name, layout.Theme = "Neuer Name", "no_such_theme"
		if data, err = MarshalProject(layout); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o640); err != nil {
			return err
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "renamed")
	})
	ctx := context.Background()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, env, job.ID, JobSucceeded, JobFailed); done.Status != JobSucceeded {
		t.Fatalf("job = %+v", done)
	}
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if summary.Name != "Neuer Name" || summary.Theme != "night" || summary.Board != p.Board {
		t.Fatalf("project row = %+v", summary)
	}
	var receipt string
	if err := env.svc.db.QueryRow(`SELECT publish_receipt FROM cc_projects WHERE id=?`, p.ID).Scan(&receipt); err != nil || receipt != receiptPublish+"_"+job.ID {
		t.Fatalf("publish_receipt = %q, %v", receipt, err)
	}
}

func TestPublicationKeepsTheAssetEntryRule(t *testing.T) {
	env, runner, p := newJobEnv(t)
	seedLayoutAsset(t, env, p.ID, Asset{ID: "a_sun", File: "assets/sun.png", W: 16, H: 16})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		// A hand-edited working copy that changes a stored asset entry.
		path := filepath.Join(env.svc.stagingPath(run.Job.ID), ProjectFile)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		layout, err := ParseProject(data)
		if err != nil {
			return err
		}
		layout.Assets[0].W = 32
		if data, err = MarshalProject(layout); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o640); err != nil {
			return err
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "done")
	})
	ctx := context.Background()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, job.ID, JobSucceeded, JobFailed)
	layout, _, _, _ := env.svc.GetLayout(ctx, p.ID)
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if done.Status != JobFailed || done.ErrorCode != CodeInvalidLayout || layout.Assets[0].W != 16 || summary.LockedByJob != "" || !summary.HasDraft {
		t.Fatalf("job = %+v, assets %+v, project %+v", done, layout.Assets, summary)
	}
}

func TestPublicationIsAnnouncedWhenTheJobRevisionFails(t *testing.T) {
	env, runner, p := newJobEnv(t)
	blobs := env.svc.blobsDir()
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int e;\n"); err != nil {
			return err
		}
		// The before_job revision is stored; now the blob store breaks.
		if err := os.Rename(blobs, blobs+".moved"); err != nil {
			return err
		}
		if err := os.WriteFile(blobs, []byte("not a directory"), 0o640); err != nil {
			return err
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "done")
	})
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, env, job.ID, JobSucceeded, JobFailed); done.Status != JobSucceeded {
		t.Fatalf("job = %+v", done)
	}
	if got := readProjectFile(t, env, p, "src/app/extra.cpp"); got != "int e;\n" {
		t.Fatalf("published = %q", got)
	}
	if revs, _ := env.svc.ListRevisions(ctx, p.ID); len(revs) != 1 || revs[0].Reason != RevisionBeforeJob {
		t.Fatalf("revisions = %+v", revs)
	}
	types := eventTypes(drain(events))
	tail := []string{EventProjectChanged, EventProjectChanged, EventLockChanged, EventJobStatus, EventJobMessage}
	if len(types) < len(tail) || !slices.Equal(types[len(types)-len(tail):], tail) {
		t.Fatalf("events = %v", types)
	}
}

func TestPublicationRetriesWhileAProjectFileIsOpen(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to rename a directory with an open file inside")
	}
	env, runner, p := newJobEnv(t)
	opened := make(chan *os.File, 1)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		f, err := os.Open(filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "app_main.cpp"))
		if err != nil {
			return err
		}
		opened <- f
		return env.svc.FinishJob(ctx, run.Job.ID, "done")
	})
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	var f *os.File
	select {
	case f = <-opened:
	case <-time.After(10 * time.Second):
		t.Fatal("the coding round did not open the file")
	}
	defer f.Close()
	// The file stays open until the publication reports the open file and
	// waits; then the next attempt can succeed.
	deadline := time.After(20 * time.Second)
	for retried := false; !retried; {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("the subscription ended")
			}
			retried = ev.Type == EventJobStep && strings.Contains(string(ev.Data), "A project file is open")
		case <-deadline:
			t.Fatal("no retry step while a project file was open")
		}
	}
	_ = f.Close()
	if done := waitJob(t, env, job.ID, JobSucceeded, JobFailed); done.Status != JobSucceeded {
		t.Fatalf("job = %+v", done)
	}
}

func TestToolingFailuresAreNotRepaired(t *testing.T) {
	appError := func(req BuildRequest) string {
		return "/work/" + req.BuildID + "/src/app/app_main.cpp:4:3: error: 'foo' was not declared in this scope\n"
	}
	cases := []struct {
		name  string
		build func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error)
	}{
		{"exit 2", func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
			return BuildOutput{ExitCode: BuildExitToolingError, Log: appError(req)}, nil
		}},
		{"unknown exit code", func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
			return BuildOutput{ExitCode: 7, Log: appError(req)}, nil
		}},
		{"builder error", func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
			return BuildOutput{Log: appError(req)}, errors.New("the build container died")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, runner, p := newJobEnv(t)
			scriptSuccess(t, env, runner)
			env.builder.build = c.build
			ctx := context.Background()
			job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
			if err != nil {
				t.Fatal(err)
			}
			done := waitJob(t, env, job.ID, JobFailed, JobSucceeded)
			if done.Status != JobFailed || done.ErrorCode != CodeBuildFailed || done.RepairAttempt != 0 ||
				slices.Contains(runner.phaseList(), PhaseRepairing) || !strings.Contains(done.ErrorMessage, "cannot fix") {
				t.Fatalf("job = %+v, phases %v", done, runner.phaseList())
			}
			b, _ := env.svc.GetBuild(ctx, done.BuildID)
			for _, d := range b.Diagnostics {
				if d.Severity == SeverityError {
					t.Fatalf("an error diagnostic survived a tooling failure: %+v", b.Diagnostics)
				}
			}
		})
	}
}

func TestPlanStepCarriesNoModelText(t *testing.T) {
	env, runner, p := newJobEnv(t)
	scriptSuccess(t, env, runner)
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeNew, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobAwaitingConfirmation)
	planStep, planMessage := false, false
	for _, ev := range drain(events) {
		data := string(ev.Data)
		switch ev.Type {
		case EventJobStep:
			if strings.Contains(data, "Weather display") {
				t.Fatalf("job_step carries the model's plan summary: %s", data)
			}
			if strings.Contains(data, "Plan submitted: 2 screen(s), 1 widget(s)") {
				planStep = true
			}
		case EventJobMessage:
			if strings.Contains(data, "Weather display with two screens") {
				planMessage = true
			}
		}
	}
	if !planStep || !planMessage {
		t.Fatalf("plan step %v, plan message %v", planStep, planMessage)
	}
}

// jobGoroutines returns the stacks of orchestrator and job timer goroutines.
func jobGoroutines() string {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	var out []string
	for _, g := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(g, "cydcreator.(*Service).executeJob") || strings.Contains(g, "cydcreator.newJobContext") {
			out = append(out, g)
		}
	}
	return strings.Join(out, "\n\n")
}

func TestCloseInterruptsTheRunningJobAndLeavesNoGoroutine(t *testing.T) {
	env, runner, p := newJobEnv(t)
	started := make(chan struct{})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int e;\n"); err != nil {
			return err
		}
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	ctx := context.Background()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	closed := make(chan error, 1)
	go func() { closed <- env.svc.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not stop the running job")
	}
	deadline := time.Now().Add(2 * time.Second)
	for stacks := jobGoroutines(); stacks != ""; stacks = jobGoroutines() {
		if time.Now().After(deadline) {
			t.Fatalf("job goroutines left after Close:\n%s", stacks)
		}
		time.Sleep(10 * time.Millisecond)
	}
	after := reopenService(t, env)
	done, _ := after.svc.GetJob(ctx, job.ID)
	summary, _ := after.svc.GetProject(ctx, p.ID)
	if done.Status != JobInterrupted || summary.LockedByJob != "" || !summary.HasDraft {
		t.Fatalf("after restart: job %+v, project %+v", done, summary)
	}
}

func TestNewJobContextGrantsOneGraceWindow(t *testing.T) {
	for _, productive := range []bool{true, false} {
		t.Run(fmt.Sprintf("productive=%v", productive), func(t *testing.T) {
			timers := newFakeTimers()
			var wg sync.WaitGroup
			var extensions []time.Duration
			ctx, cancel := newJobContext(context.Background(), &wg, timers.newTimer, time.Minute,
				func() bool { return productive }, func(d time.Duration) { extensions = append(extensions, d) })
			defer cancel(nil)
			budget := timers.next(t)
			if budget.d != time.Minute || ctx.Err() != nil {
				t.Fatalf("budget timer %v, ctx %v", budget.d, ctx.Err())
			}
			budget.fire()
			if productive {
				grace := timers.next(t)
				if grace.d != 30*time.Second || ctx.Err() != nil {
					t.Fatalf("grace timer %v, ctx %v", grace.d, ctx.Err())
				}
				grace.fire()
			}
			waitClosed(t, ctx.Done(), "the timeout")
			waitGroupDone(t, &wg)
			want := 0
			if productive {
				want = 1
			}
			if !errors.Is(context.Cause(ctx), errJobTimeout) || len(extensions) != want {
				t.Fatalf("cause %v, extensions %v", context.Cause(ctx), extensions)
			}
			select {
			case ft := <-timers.started:
				t.Fatalf("a further timer (%v) was started", ft.d)
			default:
			}
		})
	}
}

func TestNewJobContextStopsWithItsContext(t *testing.T) {
	timers := newFakeTimers()
	var wg sync.WaitGroup
	never := func() bool { t.Error("productive was asked after the context ended"); return true }
	noGrace := func(time.Duration) { t.Error("a grace window was granted after the context ended") }
	ctx, cancel := newJobContext(context.Background(), &wg, timers.newTimer, time.Minute, never, noGrace)
	timers.next(t)
	cancel(errJobCancelled)
	waitGroupDone(t, &wg)
	if !errors.Is(context.Cause(ctx), errJobCancelled) {
		t.Fatalf("cause = %v", context.Cause(ctx))
	}
	parent, stop := context.WithCancel(context.Background())
	ctx, cancel = newJobContext(parent, &wg, timers.newTimer, time.Minute, never, noGrace)
	defer cancel(nil)
	timers.next(t)
	stop() // Close cancels the service context
	waitGroupDone(t, &wg)
	if !errors.Is(context.Cause(ctx), context.Canceled) {
		t.Fatalf("cause after the parent ended = %v", context.Cause(ctx))
	}
}

// blockingBuild makes every build wait until it is cancelled and announces
// its start.
func blockingBuild(started chan<- struct{}) func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
	return func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		started <- struct{}{}
		<-ctx.Done()
		return BuildOutput{}, ctx.Err()
	}
}

func TestCancelDuringTheOrchestratorBuildCancelsTheJob(t *testing.T) {
	env, runner, p := newJobEnv(t)
	started := make(chan struct{}, 1)
	env.builder.build = blockingBuild(started)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		_, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int y;\n")
		return err
	})
	ctx := context.Background()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	waitClosed(t, started, "the build")
	if err := env.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, job.ID, JobCancelled, JobFailed, JobInterrupted)
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if done.Status != JobCancelled || done.ErrorCode != "" || done.ErrorMessage != "" || !summary.HasDraft {
		t.Fatalf("job = %+v, project = %+v", done, summary)
	}
	if b, _ := env.svc.GetBuild(ctx, done.BuildID); done.BuildID != "" && b.Status != BuildCancelled {
		t.Fatalf("build = %+v", b)
	}
}

func TestTimeoutDuringTheOrchestratorBuildReportsTheTimeLimit(t *testing.T) {
	env, runner, p := newJobEnv(t, testServiceOptions{limits: ServiceLimits{JobTimeout: 30 * time.Second}})
	timers := useFakeTimers(env)
	started := make(chan struct{}, 1)
	env.builder.build = blockingBuild(started)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error { return nil }) // no change: no grace window
	job, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	budget := timers.next(t)
	waitClosed(t, started, "the build")
	budget.fire()
	done := waitJob(t, env, job.ID, JobFailed, JobCancelled, JobInterrupted)
	if done.Status != JobFailed || done.ErrorCode != "" || done.ErrorMessage != "The job exceeded its time limit of 30s." {
		t.Fatalf("job = %+v", done)
	}
}

func TestOrchestratorFailuresStayInTheServerLog(t *testing.T) {
	env, runner, p := newJobEnv(t)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		// The working copy vanishes: the build step fails with a path error.
		return os.RemoveAll(env.svc.stagingPath(run.Job.ID))
	})
	job, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, job.ID, JobFailed, JobSucceeded)
	if done.Status != JobFailed || done.ErrorCode != CodeInternal || strings.Contains(done.ErrorMessage, env.dir) ||
		strings.Contains(done.ErrorMessage, stagingDirName) || strings.Contains(done.ErrorMessage, "agent stopped") {
		t.Fatalf("job = %+v", done)
	}
}

// The agent's reason reaches the chat through ServiceConfig.Scrub (a second
// safeguard behind the runner's generic texts), and every phase learns how
// much of the segment's time budget is left.
func TestAgentStopReasonIsScrubbedAndPhasesSeeTheirTimeBudget(t *testing.T) {
	env, runner, p := newJobEnv(t, testServiceOptions{limits: ServiceLimits{JobTimeout: 30 * time.Minute}})
	env.svc.scrub = func(s string) string { return strings.ReplaceAll(s, "sk-secret-123", "[redacted]") }
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		return errors.New("the provider said sk-secret-123")
	})
	job, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, job.ID, JobFailed, JobSucceeded)
	if done.Status != JobFailed || done.ErrorMessage != "The agent stopped: the provider said [redacted]" {
		t.Fatalf("job = %+v", done)
	}
	if got := runner.last().TimeRemaining; got <= 0 || got > 30*time.Minute {
		t.Fatalf("time remaining = %s", got)
	}
}

func TestToolsRefuseAnEndingJob(t *testing.T) {
	env, runner, p := newJobEnv(t)
	started := make(chan struct{})
	var late error
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		close(started)
		<-ctx.Done()
		// A tool call that was already on its way when the job was cancelled.
		_, late = env.svc.WriteJobFile(context.Background(), run.Job.ID, "src/app/late.cpp", "int l;\n")
		return ctx.Err()
	})
	ctx := context.Background()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	waitClosed(t, started, "the coding round")
	if err := env.svc.CancelJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobCancelled)
	if e := wantCode(t, late, CodeJobState); !strings.Contains(e.Message, "ending") {
		t.Fatalf("late write = %v", e)
	}
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if summary.HasDraft {
		t.Fatal("the refused write became a draft")
	}
}

func TestConfirmAndCancelOfAWaitingJobDoNotRace(t *testing.T) {
	env, runner, p := newJobEnv(t)
	runner.on(PhasePlanning, func(ctx context.Context, run JobRun) error {
		_, err := env.svc.SubmitPlan(ctx, run.Job.ID, []byte(testPlanJSON))
		return err
	})
	// A confirmed run that the cancel does not reach would wait here forever.
	runner.on(PhaseLayout, func(ctx context.Context, run JobRun) error {
		<-ctx.Done()
		return ctx.Err()
	})
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeNew, Prompt: fmt.Sprintf("idea %d", i)})
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		waitJob(t, env, job.ID, JobAwaitingConfirmation)
		var wg sync.WaitGroup
		var confirmErr, cancelErr error
		gate := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-gate; _, confirmErr = env.svc.ConfirmJob(ctx, job.ID, "") }()
		go func() { defer wg.Done(); <-gate; cancelErr = env.svc.CancelJob(ctx, job.ID) }()
		close(gate)
		wg.Wait()
		if cancelErr != nil || (confirmErr != nil && ErrorCode(confirmErr) != CodeJobState) {
			t.Fatalf("round %d: confirm %v, cancel %v", i, confirmErr, cancelErr)
		}
		if done := waitJob(t, env, job.ID, JobCancelled, JobFailed, JobSucceeded); done.Status != JobCancelled {
			t.Fatalf("round %d: job = %+v", i, done)
		}
		if summary, _ := env.svc.GetProject(ctx, p.ID); summary.LockedByJob != "" {
			t.Fatalf("round %d: project still locked: %+v", i, summary)
		}
	}
}

func TestFencedPublicationKeepsTheFailureReason(t *testing.T) {
	env, runner, p := newJobEnv(t)
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		if _, err := env.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int f;\n"); err != nil {
			return err
		}
		return env.svc.FinishJob(ctx, run.Job.ID, "done")
	})
	// The build was accepted; before the publication the project gets fenced
	// (an unresolved swap of another write).
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		env.svc.setPublicationFence(p.ID, "publish_ccj_review")
		return successOutput(req), nil
	}
	ctx := context.Background()
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	job, err := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	done := waitJob(t, env, job.ID, JobSucceeded, JobFailed)
	if done.Status != JobFailed || done.ErrorCode != CodeBusy || done.ErrorReason != "publication_review" {
		t.Fatalf("job = %+v", done)
	}
	if jobs, _ := env.svc.ListJobs(ctx, p.ID); len(jobs) != 1 || jobs[0].ErrorReason != "publication_review" {
		t.Fatalf("listed jobs = %+v", jobs)
	}
	announced := false
	for _, ev := range drain(events) {
		if ev.Type == EventJobStatus && strings.Contains(string(ev.Data), `"status":"failed"`) {
			announced = strings.Contains(string(ev.Data), `"error_code":"CYD_BUSY"`) &&
				strings.Contains(string(ev.Data), `"error_reason":"publication_review"`)
		}
	}
	if !announced {
		t.Fatal("the failed job_status event lacks error_code CYD_BUSY and error_reason publication_review")
	}
	if _, err := os.Stat(filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "extra.cpp")); !os.IsNotExist(err) {
		t.Fatalf("the fenced publication changed the project: %v", err)
	}
	// Jobs without a reason keep the field out of the JSON.
	if data, err := json.Marshal(Job{ID: "ccj_plain", Status: JobFailed}); err != nil || strings.Contains(string(data), "error_reason") {
		t.Fatalf("job JSON = %s (%v)", data, err)
	}
}

func TestJobErrorReasonIsAnIdentifierOnly(t *testing.T) {
	env, _, _ := newJobEnv(t)
	run := &jobRun{ctx: context.Background()}
	cases := []struct {
		details map[string]any
		want    string
	}{
		{map[string]any{"reason": "file_open"}, "file_open"},
		{map[string]any{"reason": "summary must have at most 1000 characters"}, ""},
		{map[string]any{"reason": 7}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		end := env.svc.classifyJobError(run, NewError(CodeBusy, "busy", c.details))
		if end.status != JobFailed || end.code != CodeBusy || end.reason != c.want || end.message != "busy" {
			t.Errorf("details %v: ending = %+v, want reason %q", c.details, end, c.want)
		}
	}
}
