package gamemaker

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestResumeReplaysSelectionBeforePlanAcceptance(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	selection := []string{"road-sedan"}
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "planning" {
			return errors.New("selection replay entered building")
		}
		if !slices.Equal(run.ModelAssetIDs, selection) {
			return fmt.Errorf("model selection = %v", run.ModelAssetIDs)
		}
		if run.Job.ResumeFrom == "" {
			return errors.New("planning failed before a plan")
		}
		return errors.New("replayed pending selection")
	}))
	first, err := s.StartJob(context.Background(), project.ID, StartJobRequest{ModelAssetIDs: selection})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, first.ID)
	if !strings.Contains(finished.Error, "planning failed before a plan") || finished.PlanAccepted {
		t.Fatalf("first job = %+v", finished)
	}
	resumed, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Resume: true})
	if err != nil {
		t.Fatal(err)
	}
	finished = waitJob(t, s, resumed.ID)
	if !strings.Contains(finished.Error, "replayed pending selection") || finished.PlanAccepted {
		t.Fatalf("resume = %+v", finished)
	}
}
