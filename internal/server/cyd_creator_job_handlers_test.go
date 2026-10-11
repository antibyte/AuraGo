package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
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
	// The final status is stored before the "job" revision and the chat
	// summary; JobFinished holds until the run has written both and left the
	// agent slot, so the revision and message counts below are complete.
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

func TestCYDCreatorJobRequestsAreValidated(t *testing.T) {
	f := newCYDTestServer(t)
	f.svc.SetRunner(cydScriptRunner(func(ctx context.Context, run cydcreator.JobRun) error { return nil }))
	p := f.createProject(t, "Requests")
	start := func(prompt string, attachments ...string) *httptest.ResponseRecorder {
		list := []map[string]string{}
		for _, text := range attachments {
			list = append(list, map[string]string{"kind": "log", "text": text})
		}
		body, err := json.Marshal(map[string]any{"prompt": prompt, "mode": "change", "attachments": list})
		if err != nil {
			t.Fatal(err)
		}
		return f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", strings.NewReader(string(body)), nil)
	}
	wantCYDError(t, start("x", "a", "b", "c", "d", "e"), http.StatusBadRequest, cydcreator.CodeBadRequest)
	wantCYDError(t, start("x", strings.Repeat("a", cydcreator.MaxAttachmentBytes+1)), http.StatusBadRequest, cydcreator.CodeBadRequest)
	wantCYDError(t, f.do(f.read, http.MethodGet, "revisions/ccr_missing/diff", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)
	wantCYDError(t, f.do(f.admin, http.MethodPost, "revisions/ccr_missing/restore", nil, nil), http.StatusNotFound, cydcreator.CodeNotFound)

	// The largest request the service accepts fits the body cap even when
	// every character is a six-byte JSON escape.
	w := start(strings.Repeat("\x01", cydcreator.MaxPromptRunes), strings.Repeat("\x01", cydcreator.MaxAttachmentBytes))
	var started struct {
		Job cydcreator.Job `json:"job"`
	}
	if w.Code != http.StatusAccepted || json.Unmarshal(w.Body.Bytes(), &started) != nil {
		t.Fatalf("largest accepted request = %d %s", w.Code, w.Body.String())
	}
	waitCYDServerJob(t, f, started.Job.ID, cydcreator.JobSucceeded, cydcreator.JobFailed)
}

func TestCYDCreatorConfirmWithAnEditedPromptPlansAgain(t *testing.T) {
	f := newCYDTestServer(t)
	var plannings atomic.Int32
	f.svc.SetRunner(cydScriptRunner(func(ctx context.Context, run cydcreator.JobRun) error {
		if run.Phase != cydcreator.PhasePlanning {
			return nil
		}
		plannings.Add(1)
		_, err := f.svc.SubmitPlan(ctx, run.Job.ID, []byte(`{"summary":"Clock","screens":["home"],"wifi":"optional"}`))
		return err
	}))
	p := f.createProject(t, "Replan")
	var out struct {
		Job cydcreator.Job `json:"job"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", `{"prompt":"A clock","mode":"new"}`, nil, http.StatusAccepted, &out)
	waitCYDServerJob(t, f, out.Job.ID, cydcreator.JobAwaitingConfirmation)
	var confirmed struct {
		Job cydcreator.Job `json:"job"`
	}
	f.json(t, f.admin, http.MethodPost, "jobs/"+out.Job.ID+"/confirm", `{"prompt":"A bigger clock"}`, nil, http.StatusOK, &confirmed)
	if confirmed.Job.Status != cydcreator.JobRunning || confirmed.Job.Plan != nil {
		t.Fatalf("confirmed = %+v", confirmed.Job)
	}
	waitCYD(t, "the second plan", func() bool {
		j, err := f.svc.GetJob(context.Background(), out.Job.ID)
		return err == nil && j.Status == cydcreator.JobAwaitingConfirmation && plannings.Load() == 2
	})
	var messages struct {
		Messages []cydcreator.Message `json:"messages"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/messages", "", nil, http.StatusOK, &messages)
	if !slices.ContainsFunc(messages.Messages, func(m cydcreator.Message) bool {
		return m.Role == cydcreator.MessageRoleUser && m.Text == "A bigger clock"
	}) {
		t.Fatalf("messages = %+v", messages.Messages)
	}
	f.json(t, f.admin, http.MethodPost, "jobs/"+out.Job.ID+"/cancel", "", nil, http.StatusAccepted, nil)
	waitCYDServerJob(t, f, out.Job.ID, cydcreator.JobCancelled)
}

func TestCYDCreatorDraftDiscardAndRestoreGates(t *testing.T) {
	f := newCYDTestServer(t)
	wrote := make(chan struct{})
	f.svc.SetRunner(cydScriptRunner(func(ctx context.Context, run cydcreator.JobRun) error {
		if _, err := f.svc.WriteJobFile(ctx, run.Job.ID, "src/app/extra.cpp", "int x;\n"); err != nil {
			return err
		}
		close(wrote)
		<-ctx.Done()
		return ctx.Err()
	}))
	p := f.createProject(t, "Draft")
	var out struct {
		Job cydcreator.Job `json:"job"`
	}
	f.json(t, f.admin, http.MethodPost, "projects/"+p.ID+"/jobs", `{"prompt":"x","mode":"change"}`, nil, http.StatusAccepted, &out)
	<-wrote
	f.json(t, f.admin, http.MethodPost, "jobs/"+out.Job.ID+"/cancel", "", nil, http.StatusAccepted, nil)
	waitCYDServerJob(t, f, out.Job.ID, cydcreator.JobCancelled)
	var project struct {
		Project cydcreator.ProjectSummary `json:"project"`
	}
	waitCYD(t, "the draft", func() bool {
		f.json(t, f.read, http.MethodGet, "projects/"+p.ID, "", nil, http.StatusOK, &project)
		return project.Project.HasDraft
	})
	if w := f.do(f.admin, http.MethodPost, "projects/"+p.ID+"/draft/discard", nil, nil); w.Code != http.StatusNoContent {
		t.Fatalf("discard = %d %s", w.Code, w.Body.String())
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID, "", nil, http.StatusOK, &project)
	if project.Project.HasDraft {
		t.Fatalf("project after discard = %+v", project.Project)
	}

	var revisions struct {
		Revisions []cydcreator.Revision `json:"revisions"`
	}
	f.json(t, f.read, http.MethodGet, "projects/"+p.ID+"/revisions", "", nil, http.StatusOK, &revisions)
	if len(revisions.Revisions) != 1 {
		t.Fatalf("revisions = %+v", revisions.Revisions)
	}
	restore := "revisions/" + revisions.Revisions[0].ID + "/restore"
	cydLockProjectForTest(t, f, p.ID, "ccj_other")
	locked := wantCYDError(t, f.do(f.admin, http.MethodPost, restore, nil, nil), http.StatusLocked, cydcreator.CodeProjectLocked)
	if locked.Details["job_id"] != "ccj_other" {
		t.Fatalf("locked details = %v", locked.Details)
	}
	cydLockProjectForTest(t, f, p.ID, "")
	f.s.Cfg.VirtualDesktop.ReadOnly = true
	wantCYDError(t, f.do(f.admin, http.MethodPost, restore, nil, nil), http.StatusForbidden, cydcreator.CodeReadOnly)
	f.s.Cfg.VirtualDesktop.ReadOnly = false
	f.json(t, f.admin, http.MethodPost, restore, "", nil, http.StatusOK, nil)
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
