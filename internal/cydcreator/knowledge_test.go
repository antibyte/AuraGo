package cydcreator

import (
	"context"
	"path/filepath"
	"testing"
)

func TestJobAnalysisRootFollowsActiveJob(t *testing.T) {
	env, runner, project := newJobEnv(t)
	if _, err := env.svc.JobAnalysisRoot("ccj_none"); err == nil {
		t.Fatal("inactive job exposed a root")
	}
	var root string
	var readErr error
	job := inPhase(t, env, runner, project, PhaseCoding, func(ctx context.Context, jobID string) {
		root, readErr = env.svc.JobAnalysisRoot(jobID)
		if _, err := env.svc.JobAnalysisRoot("ccj_other"); err == nil {
			readErr = NewError(CodeJobState, "other job exposed", nil)
		}
	})
	if readErr != nil || !filepath.IsAbs(root) {
		t.Fatal("active working copy unavailable", readErr)
	}
	if _, err := env.svc.JobAnalysisRoot(job.ID); err == nil {
		t.Fatal("finished job retained graph access")
	}
}
