package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/cydcreator"
)

// cydScriptRunner is a cydcreator.Runner for handler tests.
type cydScriptRunner func(ctx context.Context, run cydcreator.JobRun) error

func (f cydScriptRunner) RunCYDJob(ctx context.Context, run cydcreator.JobRun) error {
	return f(ctx, run)
}

func TestCYDCreatorJobAPI(t *testing.T) {
	f := newCYDTestServer(t)
	f.svc.SetRunner(cydScriptRunner(func(ctx context.Context, run cydcreator.JobRun) error {
		switch run.Phase {
		case cydcreator.PhasePlanning:
			_, err := f.svc.SubmitPlan(ctx, run.Job.ID, []byte(`{"summary":"Clock","screens":["home"],"wifi":"optional"}`))
			return err
		case cydcreator.PhaseCoding:
			if _, err := f.svc.WriteJobFile(ctx, run.Job.ID, "src/app/app_main.cpp", "// clock\n"); err != nil {
				return err
			}
			return f.svc.FinishJob(ctx, run.Job.ID, "A clock.")
		}
		return nil
	}))
	p := f.createProject(t, "Jobs")
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", strings.NewReader(`{"prompt":"x","mode":"new","extra":1}`), nil),
		http.StatusBadRequest, cydcreator.CodeBadRequest)
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", strings.NewReader(`{"prompt":"x","mode":"rewrite"}`), nil),
		http.StatusBadRequest, cydcreator.CodeBadRequest)

	var started struct {
		Job cydcreator.Job `json:"job"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", `{"prompt":"A clock","mode":"new","resume":false,"attachments":[{"kind":"log","text":"boot ok"}]}`,
		nil, http.StatusAccepted, &started)
	if started.Job.ID == "" || started.Job.Mode != "new" {
		t.Fatalf("started = %+v", started.Job)
	}
	waitCYDServerJob(t, f, started.Job.ID, cydcreator.JobAwaitingConfirmation)
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", strings.NewReader(`{"prompt":"y","mode":"change"}`), nil),
		http.StatusConflict, cydcreator.CodeBusy)
	layout, _, _, _ := f.svc.GetLayout(context.Background(), p.ID)
	body, _ := json.Marshal(map[string]any{"project": layout})
	wantCYDError(t, f.do(f.admin, http.MethodPut, "projects/"+p.ID+"/layout", strings.NewReader(string(body)), map[string]string{"If-Match": "1"}),
		http.StatusLocked, cydcreator.CodeProjectLocked)

	var got struct {
		Job cydcreator.Job `json:"job"`
	}
	f.json(t, f.read, http.MethodGet, "jobs/"+started.Job.ID, "", nil, http.StatusOK, &got)
	if got.Job.Status != cydcreator.JobAwaitingConfirmation || got.Job.Plan == nil || got.Job.Plan.Summary != "Clock" {
		t.Fatalf("job = %+v", got.Job)
	}
	f.json(t, f.admin, http.MethodPost, "jobs/"+started.Job.ID+"/confirm", `{}`, nil, http.StatusOK, &got)
	done := waitCYDServerJob(t, f, started.Job.ID, cydcreator.JobSucceeded, cydcreator.JobFailed)
	if done.Status != cydcreator.JobSucceeded {
		t.Fatalf("job = %+v", done)
	}
	// The final status is stored before the "job" revision, the chat summary
	// and the release of the agent slot; a cancel before the release still
	// reaches the finishing run. JobFinished holds until the run leaves the slot.
	waitCYD(t, "the released agent slot", func() bool { return !f.svc.JobFinished(started.Job.ID) })
	wantCYDError(t, f.do(f.admin, http.MethodPost, "jobs/"+started.Job.ID+"/confirm", strings.NewReader(`{}`), nil),
		http.StatusConflict, cydcreator.CodeJobState)
	wantCYDError(t, f.do(f.admin, http.MethodPost, "jobs/"+started.Job.ID+"/cancel", nil, nil), http.StatusConflict, cydcreator.CodeJobState)
	wantCYDError(t, f.do(f.read, http.MethodGet, "jobs/ccj_missing", nil, nil), http.StatusNotFound, cydcreator.CodeJobNotFound)

	var jobs struct {
		Jobs []cydcreator.Job `json:"jobs"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/jobs", "", nil, http.StatusOK, &jobs)
	var messages struct {
		Messages []cydcreator.Message `json:"messages"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/messages", "", nil, http.StatusOK, &messages)
	if len(jobs.Jobs) != 1 || len(messages.Messages) != 3 || len(messages.Messages[0].Attachments) != 1 {
		t.Fatalf("jobs = %+v, messages = %+v", jobs.Jobs, messages.Messages)
	}
	var revisions struct {
		Revisions []cydcreator.Revision `json:"revisions"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/revisions", "", nil, http.StatusOK, &revisions)
	if len(revisions.Revisions) != 2 {
		t.Fatalf("revisions = %+v", revisions.Revisions)
	}
	var diff cydcreator.RevisionDiffResult
	f.json(t, f.read, http.MethodGet, "revisions/"+revisions.Revisions[0].ID+"/diff", "", nil, http.StatusOK, &diff)
	if len(diff.Files) != 1 || diff.Files[0].Status != "modified" {
		t.Fatalf("diff = %+v", diff)
	}
	var restored struct {
		Version  int64               `json:"version"`
		Revision cydcreator.Revision `json:"revision"`
	}
	f.json(t, f.admin, http.MethodPost, "revisions/"+revisions.Revisions[1].ID+"/restore", "", nil, http.StatusOK, &restored)
	if restored.Revision.Reason != cydcreator.RevisionRestore || restored.Version == 0 {
		t.Fatalf("restore = %+v", restored)
	}
	if w := f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/draft/discard", nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("discard = %d %s", w.Code, w.Body.String())
	}
}

func TestCYDCreatorJobBodiesAreCappedAndStrict(t *testing.T) {
	f := newCYDTestServer(t)
	p := f.createProject(t, "Bodies")
	for _, route := range []struct {
		path  string
		limit int
	}{
		{"projects/" + p.ID + "/jobs", cydJobBodyLimit},
		{"jobs/ccj_x1/confirm", cydJobBodyLimit},
		{"jobs/ccj_x1/cancel", cydSmallBodyLimit},
		{"projects/" + p.ID + "/draft/discard", cydSmallBodyLimit},
		{"revisions/ccr_x1/restore", cydSmallBodyLimit},
	} {
		padded := `{"prompt":"` + strings.Repeat("a", route.limit) + `"}`
		wantCYDError(t, f.do(f.admin, http.MethodPost, route.path, strings.NewReader(padded), nil), http.StatusRequestEntityTooLarge, cydcreator.CodeBadRequest)
		wantCYDError(t, f.do(f.admin, http.MethodPost, route.path, strings.NewReader(`{}{}`), nil), http.StatusBadRequest, cydcreator.CodeBadRequest)
	}
}

func TestCYDCreatorJobCancelWorksWhileReadOnly(t *testing.T) {
	f := newCYDTestServer(t)
	started := make(chan struct{})
	f.svc.SetRunner(cydScriptRunner(func(ctx context.Context, run cydcreator.JobRun) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}))
	p := f.createProject(t, "Cancel")
	var out struct {
		Job cydcreator.Job `json:"job"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", `{"prompt":"x","mode":"change"}`, nil, http.StatusAccepted, &out)
	<-started
	f.s.Cfg.VirtualDesktop.ReadOnly = true
	defer func() { f.s.Cfg.VirtualDesktop.ReadOnly = false }()
	wantCYDError(t, f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", strings.NewReader(`{"prompt":"y","mode":"change"}`), nil),
		http.StatusForbidden, cydcreator.CodeReadOnly)
	if w := f.do(f.read, http.MethodPost, "jobs/"+out.Job.ID+"/cancel", nil, nil); w.Code != http.StatusForbidden {
		t.Fatalf("read token cancel = %d", w.Code)
	}
	if w := f.do(f.admin, http.MethodPost, "jobs/"+out.Job.ID+"/cancel", nil, nil); w.Code != http.StatusAccepted {
		t.Fatalf("cancel = %d %s", w.Code, w.Body.String())
	}
	if done := waitCYDServerJob(t, f, out.Job.ID, cydcreator.JobCancelled); done.Status != cydcreator.JobCancelled {
		t.Fatalf("job = %+v", done)
	}
}
