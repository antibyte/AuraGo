package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

// reviewStep is what one withReview call left behind: the link in the job document, the sheet the summary still carries and the
// sheet the project holds.
type reviewStep struct {
	review       *storymaker.AgentReview
	summarySheet []byte
	stored       []byte
	err          error
}

// TestStorymakerWithReviewKeepsTheLinkTruthful runs withReview inside a real agent job (the override gives it a live handle) and
// reads the job document after each check: a sheet is saved and linked, a check without a sheet or with a failed save clears the
// link, and a check that errors leaves it alone.
func TestStorymakerWithReviewKeepsTheLinkTruthful(t *testing.T) {
	s, _, writeToken := storymakerAPIFixture(t)
	sheet := []byte("\x89PNG\r\n\x1a\nsheet")
	withSheet := func(scenes int) func(context.Context, string) (storymaker.CompileSummary, error) {
		return func(context.Context, string) (storymaker.CompileSummary, error) {
			return storymaker.CompileSummary{Available: true, Sheet: sheet, Stills: make([]storymaker.StillInfo, scenes)}, nil
		}
	}
	withoutSheet := func(context.Context, string) (storymaker.CompileSummary, error) {
		return storymaker.CompileSummary{Available: true, Issues: []storymaker.Issue{{Severity: storymaker.SeverityError, Code: storymaker.CodeCompileFailed}}}, nil
	}
	failed := func(context.Context, string) (storymaker.CompileSummary, error) {
		return storymaker.CompileSummary{}, errors.New("the check was cancelled")
	}

	var rt *storymakerRuntime
	steps := make(chan []reviewStep, 1)
	s.storymakerAgentRunOverride = func(ctx context.Context, job render.Job, _ any, h *render.Handle) (render.Output, error) {
		j := &storymakerAgentJob{rt: rt, job: job, h: h, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		var got []reviewStep
		run := func(projectID string, inner func(context.Context, string) (storymaker.CompileSummary, error)) {
			summary, err := j.withReview(inner)(ctx, projectID)
			step := reviewStep{summarySheet: summary.Sheet, err: err}
			if info, _, ok := rt.jobByID(job.ID); ok && info.Agent != nil {
				step.review = info.Agent.Review
			}
			if r, _, openErr := rt.projects.OpenReviewSheet(ctx, job.ProjectID); openErr == nil {
				step.stored, _ = io.ReadAll(r)
				_ = r.Close()
			}
			got = append(got, step)
		}
		run(job.ProjectID, withSheet(3))
		run(job.ProjectID, failed)
		run(job.ProjectID, withoutSheet)
		run(job.ProjectID, withSheet(5))
		run("../evil", withSheet(5)) // the save is refused
		steps <- got
		return render.Output{}, nil
	}
	id := createStorymakerProject(t, s, writeToken).Project.ID
	var err error
	if rt, err = s.storymakerRuntime(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := rt.agent.Enqueue(render.EnqueueRequest{ProjectID: id, Kind: render.KindAgent, Coalesce: true}); err != nil {
		t.Fatal(err)
	}
	var got []reviewStep
	select {
	case got = <-steps:
	case <-time.After(20 * time.Second):
		t.Fatal("the agent job did not run")
	}

	want := []struct {
		name   string
		review *storymaker.AgentReview
	}{
		{"a sheet is saved and linked", &storymaker.AgentReview{Sheet: storymaker.ReviewSheetName, Scenes: 3}},
		{"a check that errors leaves the link", &storymaker.AgentReview{Sheet: storymaker.ReviewSheetName, Scenes: 3}},
		{"a check without a sheet clears it", nil},
		{"a new sheet is linked again", &storymaker.AgentReview{Sheet: storymaker.ReviewSheetName, Scenes: 5}},
		{"a failed save clears it", nil},
	}
	if len(got) != len(want) {
		t.Fatalf("%d steps ran, want %d", len(got), len(want))
	}
	for i, w := range want {
		step := got[i]
		if (step.review == nil) != (w.review == nil) || (w.review != nil && *step.review != *w.review) {
			t.Errorf("%s: review = %+v, want %+v", w.name, step.review, w.review)
		}
		if step.summarySheet != nil {
			t.Errorf("%s: the summary keeps %d bytes of the sheet", w.name, len(step.summarySheet))
		}
		if (i == 1) != (step.err != nil) {
			t.Errorf("%s: error = %v", w.name, step.err)
		}
	}
	if string(got[0].stored) != string(sheet) {
		t.Errorf("the sheet is not in the project after the first check: %q", got[0].stored)
	}
}

func TestStorymakerReviewSheetRoute(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	id := createStorymakerProject(t, s, writeToken).Project.ID
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	path := "/projects/" + id + "/review/contact-sheet.png"

	w := storymakerRequest(s, readToken, http.MethodGet, path, nil, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("no sheet yet: %d %s", w.Code, w.Body.String())
	}
	data := []byte("\x89PNG\r\n\x1a\nsheet")
	if err := rt.projects.SaveReviewSheet(context.Background(), id, data); err != nil {
		t.Fatal(err)
	}
	w = storymakerRequest(s, readToken, http.MethodGet, path, nil, nil)
	if w.Code != http.StatusOK || w.Body.String() != string(data) {
		t.Fatalf("GET = %d, %d bytes", w.Code, w.Body.Len())
	}
	for header, want := range map[string]string{
		"Content-Type": "image/png", "X-Content-Type-Options": "nosniff", "Content-Security-Policy": "sandbox; default-src 'none'", "Cache-Control": "private, no-cache",
	} {
		if got := w.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if w := storymakerRequest(s, readToken, http.MethodGet, "/projects/"+id+"/review/other.png", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("another file name = %d", w.Code)
	}
	if w := storymakerRequest(s, writeToken, http.MethodPost, path, nil, nil); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST = %d, Allow %q", w.Code, w.Header().Get("Allow"))
	}
}
