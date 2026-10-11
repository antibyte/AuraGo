package cydcreator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// inPhase starts a job and runs fn inside the given agent phase; the phase
// then ends normally (coding phases call FinishJob, planning submits a plan).
func inPhase(t *testing.T, env *testEnv, runner *fakeRunner, p ProjectSummary, phase string, fn func(ctx context.Context, jobID string)) Job {
	t.Helper()
	runner.on(phase, func(ctx context.Context, run JobRun) error {
		fn(ctx, run.Job.ID)
		switch phase {
		case PhasePlanning:
			_, err := env.svc.SubmitPlan(ctx, run.Job.ID, []byte(testPlanJSON))
			return err
		case PhaseCoding, PhaseRepairing:
			return env.svc.FinishJob(ctx, run.Job.ID, "done")
		}
		return nil
	})
	mode := JobModeChange
	if phase == PhasePlanning {
		mode = JobModeNew
	}
	job, err := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: mode, Prompt: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return waitJob(t, env, job.ID, JobSucceeded, JobFailed, JobAwaitingConfirmation)
}

func TestPhaseToolsMatrix(t *testing.T) {
	if got := PhaseTools(PhasePlanning); !slices.Equal(got, []string{ToolReadProject, ToolAPIReference, ToolSubmitPlan}) {
		t.Fatalf("planning tools = %v", got)
	}
	if got := PhaseTools(PhaseLayout); slices.Contains(got, ToolWriteFile) || !slices.Contains(got, ToolUpdateLayout) {
		t.Fatalf("layout tools = %v", got)
	}
	if !slices.Equal(PhaseTools(PhaseCoding), PhaseTools(PhaseRepairing)) || slices.Contains(PhaseTools(PhaseCoding), ToolSubmitPlan) {
		t.Fatalf("coding tools = %v", PhaseTools(PhaseCoding))
	}
	if PhaseTools(PhaseBuilding) != nil || len(ToolNames()) != 9 {
		t.Fatalf("building tools %v, all %v", PhaseTools(PhaseBuilding), ToolNames())
	}
}

func TestToolsNeedTheRunningJobAndItsPhase(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	if _, err := env.svc.ReadProjectForJob(ctx, "ccj_none"); ErrorCode(err) != CodeJobState {
		t.Fatalf("no job: %v", err)
	}
	var errs []error
	inPhase(t, env, runner, p, PhasePlanning, func(ctx context.Context, jobID string) {
		_, err := env.svc.WriteJobFile(ctx, jobID, "src/app/x.cpp", "int x;")
		errs = append(errs, err, env.svc.FinishJob(ctx, jobID, "x"))
		_, err = env.svc.UpdateLayoutForJob(ctx, jobID, []LayoutOp{{Op: "set_theme", Theme: "clear"}})
		errs = append(errs, err)
		_, err = env.svc.ReadProjectForJob(ctx, "ccj_other")
		errs = append(errs, err)
	})
	for i, err := range errs {
		if ErrorCode(err) != CodeJobState {
			t.Errorf("call %d: err = %v, want CYD_JOB_STATE", i, err)
		}
	}
}

func TestReadProjectForJobShowsTheWorkingCopy(t *testing.T) {
	env, runner, p := newJobEnv(t)
	var view JobProjectView
	var err error
	inPhase(t, env, runner, p, PhaseCoding, func(ctx context.Context, jobID string) {
		if _, err = env.svc.WriteJobFile(ctx, jobID, "src/app/sensor.cpp", "int s;\n"); err != nil {
			return
		}
		view, err = env.svc.ReadProjectForJob(ctx, jobID)
	})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range view.Files {
		paths = append(paths, f.Path)
	}
	if view.Phase != PhaseCoding || view.Layout.Name != "Wetter" || !slices.Contains(paths, "src/app/sensor.cpp") ||
		view.Board.ID != "esp32-2432s028r" || view.Board.ScreenW != 320 || view.Board.ScreenH != 240 || view.Board.AppSlotBytes != 0x1E0000 ||
		!slices.Contains(view.Board.Features, "rgb_led") {
		t.Fatalf("view = %+v", view)
	}
}

func TestUpdateLayoutForJobChangesOnlyTheWorkingCopy(t *testing.T) {
	env, runner, p := newJobEnv(t)
	applied, release := make(chan struct{}), make(chan struct{})
	var update LayoutUpdate
	var opErr, badErr error
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		update, opErr = env.svc.UpdateLayoutForJob(ctx, run.Job.ID, []LayoutOp{
			{Op: "add_widget", Screen: "home", Widget: &Widget{Type: "button", Name: "btn_go", X: 10, Y: 10}},
			{Op: "update_widget", ID: "btn_go", Set: map[string]any{"props": map[string]any{"text": "Go"}}},
		})
		_, badErr = env.svc.UpdateLayoutForJob(ctx, run.Job.ID, []LayoutOp{{Op: "delete_widget", ID: "missing"}})
		close(applied)
		<-release
		return env.svc.FinishJob(ctx, run.Job.ID, "ok")
	})
	ctx := context.Background()
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	<-applied
	// The project is untouched until the job publishes.
	layout, _, _, _ := env.svc.GetLayout(ctx, p.ID)
	if _, _, ok := layout.FindWidget("btn_go"); ok {
		t.Fatal("the project changed before publication")
	}
	close(release)
	waitJob(t, env, job.ID, JobSucceeded)
	if opErr != nil || len(update.Results) != 2 || update.Results[0].Op != "add_widget" {
		t.Fatalf("update = %+v, %v", update, opErr)
	}
	e := wantCode(t, badErr, CodeInvalidLayout)
	if e.Details["index"] != 0 {
		t.Fatalf("details = %+v", e.Details)
	}
	layout, _, _, _ = env.svc.GetLayout(ctx, p.ID)
	si, path, ok := layout.FindWidget("btn_go")
	if !ok || layout.Screens[si].Widgets[path[0]].Props["text"] != "Go" {
		t.Fatalf("published layout = %+v", layout.Screens[0].Widgets)
	}
}

func TestJobFileToolsEnforcePathsAndLimits(t *testing.T) {
	env, runner, p := newJobEnv(t)
	type result struct {
		name string
		err  error
	}
	var results []result
	var read JobFile
	var generated JobFile
	inPhase(t, env, runner, p, PhaseCoding, func(ctx context.Context, jobID string) {
		add := func(name string, err error) { results = append(results, result{name, err}) }
		_, err := env.svc.WriteJobFile(ctx, jobID, "src/generated/ui.cpp", "x")
		add("generated", err)
		_, err = env.svc.WriteJobFile(ctx, jobID, "../evil.cpp", "x")
		add("outside", err)
		_, err = env.svc.WriteJobFile(ctx, jobID, "src/app/big.cpp", strings.Repeat("x", MaxAppFileBytes+1))
		add("too big", err)
		_, err = env.svc.WriteJobFile(ctx, jobID, "src/app/APP_MAIN.cpp", "x")
		add("case clash", err)
		_, err = env.svc.WriteJobFile(ctx, jobID, "src/app/bin.cpp", "a\x00b")
		add("binary", err)
		if _, err := env.svc.WriteJobFile(ctx, jobID, "src/app/net/http.cpp", "int h;\n"); err != nil {
			add("nested write", err)
		}
		read, err = env.svc.ReadJobFile(ctx, jobID, "src/app/net/http.cpp")
		add("read", err)
		generated, err = env.svc.ReadJobFile(ctx, jobID, "src/generated/ui.cpp")
		add("read generated", err)
		_, err = env.svc.ReadJobFile(ctx, jobID, "src/app/none.cpp")
		add("read missing", err)
		add("delete", env.svc.DeleteJobFile(ctx, jobID, "src/app/net/http.cpp"))
		add("delete again", env.svc.DeleteJobFile(ctx, jobID, "src/app/net/http.cpp"))
		add("delete generated", env.svc.DeleteJobFile(ctx, jobID, "platformio.ini"))
	})
	want := map[string]string{"generated": CodeFileReadOnly, "outside": CodeBadRequest, "too big": CodeFileLimit,
		"case clash": CodeFileConflict, "binary": CodeBadRequest, "read": "", "read generated": "", "read missing": CodeNotFound,
		"delete": "", "delete again": CodeNotFound, "delete generated": CodeFileReadOnly}
	for _, r := range results {
		if code, ok := want[r.name]; !ok || ErrorCode(r.err) != code || (code == "" && r.err != nil) {
			t.Errorf("%s: err = %v, want %q", r.name, r.err, want[r.name])
		}
	}
	if read.Content != "int h;\n" || read.SHA256 != sha256Hex([]byte("int h;\n")) || !generated.Generated || generated.Content == "" {
		t.Fatalf("read = %+v, generated = %+v", read, generated)
	}
	if dirExists(env.svc.projectDir(p.Slug) + "/src/app/net") {
		t.Fatal("the deleted file's empty directory was published")
	}
}

func TestBuildForJobHidesGeneratedDiagnostics(t *testing.T) {
	env, runner, p := newJobEnv(t)
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		return compileError(req), nil
	}
	var res JobBuildResult
	var err error
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		res, err = env.svc.BuildForJob(ctx, run.Job.ID)
		return env.svc.FinishJob(ctx, run.Job.ID, "tried")
	})
	job, _ := env.svc.StartJob(context.Background(), p.ID, StartJobInput{Mode: JobModeChange, Prompt: "x"})
	waitJob(t, env, job.ID, JobFailed, JobSucceeded)
	if err != nil || res.Status != BuildFailed || res.ErrorCode != CodeBuildFailed || len(res.Diagnostics) != 1 ||
		res.Diagnostics[0].File != "src/app/app_main.cpp" || !strings.Contains(res.NextAction, "Fix the listed errors") {
		t.Fatalf("build result = %+v, %v", res, err)
	}
	b, _ := env.svc.GetBuild(context.Background(), res.BuildID)
	if b.Origin != BuildOriginAgent || b.JobID != job.ID {
		t.Fatalf("build record = %+v", b)
	}
}

func TestSubmitPlanAndFinishValidateInput(t *testing.T) {
	env, runner, p := newJobEnv(t)
	var planErr, finishErr error
	runner.on(PhasePlanning, func(ctx context.Context, run JobRun) error {
		_, planErr = env.svc.SubmitPlan(ctx, run.Job.ID, []byte(`{"summary":"x"}`))
		if env.svc.PlanAccepted(run.Job.ID) {
			planErr = nil
		}
		_, err := env.svc.SubmitPlan(ctx, run.Job.ID, []byte(testPlanJSON))
		return err
	})
	runner.on(PhaseCoding, func(ctx context.Context, run JobRun) error {
		finishErr = env.svc.FinishJob(ctx, run.Job.ID, "   ")
		if env.svc.JobFinished(run.Job.ID) {
			finishErr = nil
		}
		env.svc.NoteAgentReply(run.Job.ID, "All done from prose.")
		return nil
	})
	ctx := context.Background()
	job, _ := env.svc.StartJob(ctx, p.ID, StartJobInput{Mode: JobModeNew, Prompt: "x"})
	waitJob(t, env, job.ID, JobAwaitingConfirmation)
	wantCode(t, planErr, CodeBadRequest)
	if _, err := env.svc.ConfirmJob(ctx, job.ID, ""); err != nil {
		t.Fatal(err)
	}
	waitJob(t, env, job.ID, JobSucceeded)
	wantCode(t, finishErr, CodeBadRequest)
	msgs, _ := env.svc.ListMessages(ctx, p.ID)
	if last := msgs[len(msgs)-1]; last.Text != "All done from prose." {
		t.Fatalf("summary from prose = %+v", last)
	}
}

// The working-copy tools apply the live project's file rules: exact-case
// paths, regular files only and the size limit on reads.
func TestJobFileToolsKeepExactCaseAndRegularFiles(t *testing.T) {
	env, runner, p := newJobEnv(t)
	ctx := context.Background()
	if _, err := env.svc.WriteFile(ctx, p.ID, "src/app/net/http.cpp", []byte("int h;\n"), "*", OriginUser); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(env.svc.projectDir(p.Slug), "src", "app", "dir.cpp"), 0o750); err != nil {
		t.Fatal(err)
	}
	errs := map[string]error{}
	inPhase(t, env, runner, p, PhaseCoding, func(ctx context.Context, jobID string) {
		_, errs["folder alias"] = env.svc.WriteJobFile(ctx, jobID, "src/app/NET/x.cpp", "int x;\n")
		_, errs["read alias"] = env.svc.ReadJobFile(ctx, jobID, "src/app/NET/http.cpp")
		errs["delete alias"] = env.svc.DeleteJobFile(ctx, jobID, "src/app/net/HTTP.cpp")
		_, errs["write dir"] = env.svc.WriteJobFile(ctx, jobID, "src/app/dir.cpp", "int d;\n")
		_, errs["read dir"] = env.svc.ReadJobFile(ctx, jobID, "src/app/dir.cpp")
		errs["delete dir"] = env.svc.DeleteJobFile(ctx, jobID, "src/app/dir.cpp")
		// A file above the limit, put into the working copy by hand.
		big := filepath.Join(env.svc.stagingPath(jobID), "src", "app", "big.cpp")
		if err := os.WriteFile(big, []byte(strings.Repeat("x", MaxAppFileBytes+1)), 0o640); err != nil {
			errs["setup"] = err
			return
		}
		_, errs["read big"] = env.svc.ReadJobFile(ctx, jobID, "src/app/big.cpp")
		_, errs["replace big"] = env.svc.WriteJobFile(ctx, jobID, "src/app/big.cpp", "int b;\n")
	})
	want := map[string]string{"folder alias": CodeFileConflict, "read alias": CodeNotFound, "delete alias": CodeNotFound,
		"write dir": CodeBadRequest, "read dir": CodeBadRequest, "delete dir": CodeBadRequest, "read big": CodeFileLimit,
		"replace big": ""}
	for name, err := range errs {
		if code, ok := want[name]; !ok || ErrorCode(err) != code || (code == "" && err != nil) {
			t.Errorf("%s: err = %v, want %q", name, err, want[name])
		}
	}
	var typed *Error
	if errors.As(errs["folder alias"], &typed) && typed.Details["existing"] != "src/app/net" {
		t.Errorf("folder alias details = %+v", typed.Details)
	}
	for _, name := range []string{"write dir", "read dir", "delete dir"} {
		if errors.As(errs[name], &typed) && typed.Details["reason"] != "not_regular" {
			t.Errorf("%s details = %+v", name, typed.Details)
		}
	}
	if got := readProjectFile(t, env, p, "src/app/big.cpp"); got != "int b;\n" {
		t.Fatalf("published big.cpp = %q", got)
	}
}

// Layout ops on the working copy keep the layout rules: a message box needs a
// button (Validate through the ApplyOps gate) and asset entries follow the
// layout save rule (checkLayoutAssets).
func TestUpdateLayoutForJobKeepsTheLayoutRules(t *testing.T) {
	env, runner, p := newJobEnv(t)
	var added, emptied, badAssets error
	inPhase(t, env, runner, p, PhaseCoding, func(ctx context.Context, jobID string) {
		_, added = env.svc.UpdateLayoutForJob(ctx, jobID, []LayoutOp{{Op: "add_widget", Screen: "home",
			Widget: &Widget{Type: "msgbox", Name: "box_info"}}})
		_, emptied = env.svc.UpdateLayoutForJob(ctx, jobID, []LayoutOp{{Op: "update_widget", ID: "box_info",
			Set: map[string]any{"props": map[string]any{"buttons": []any{}}}}})
		// A hand-edited working copy whose asset entry points at app code.
		path := filepath.Join(env.svc.stagingPath(jobID), ProjectFile)
		original, err := os.ReadFile(path)
		if err != nil {
			badAssets = err
			return
		}
		layout, err := ParseProject(original)
		if err != nil {
			badAssets = err
			return
		}
		layout.Assets = append(layout.Assets, Asset{ID: "a_bad", File: "src/app/app_main.cpp", W: 10, H: 10})
		data, err := MarshalProject(layout)
		if err == nil {
			err = os.WriteFile(path, data, 0o640)
		}
		if err != nil {
			badAssets = err
			return
		}
		_, badAssets = env.svc.UpdateLayoutForJob(ctx, jobID, []LayoutOp{{Op: "set_theme", Theme: "clear"}})
		_ = os.WriteFile(path, original, 0o640) // let the job publish
	})
	if added != nil {
		t.Fatalf("add msgbox: %v", added)
	}
	if e := wantCode(t, emptied, CodeInvalidLayout); e.Details["code"] != IssuePropInvalid {
		t.Fatalf("emptied buttons: details = %+v", e.Details)
	}
	if e := wantCode(t, badAssets, CodeInvalidLayout); e.Details["asset"] != "a_bad" {
		t.Fatalf("bad asset entry: details = %+v", e.Details)
	}
	// The refused ops changed nothing: the msgbox keeps its buttons, the theme
	// stays.
	layout, _, issues, _ := env.svc.GetLayout(context.Background(), p.ID)
	if _, _, ok := layout.FindWidget("box_info"); !ok || layout.Theme != "night" || HasErrors(issues) {
		t.Fatalf("published layout = %+v, issues %+v", layout, issues)
	}
}
