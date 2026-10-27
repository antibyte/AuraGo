package gamemaker

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPlanningFailureCodeSeparatesProtocolFromRejectedDesigns(t *testing.T) {
	cases := []struct {
		name     string
		submit   bool
		wantCode string
		wantText string
	}{
		{"never_submitted", false, "planning_protocol", "never submitted a design"},
		{"rejected_submissions", true, "planning", "charged correction budget was spent"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestService(t)
			project := createTestProject(t, s, "2d")
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				if tc.submit {
					// A rejected submission is charged; the model did reach the tool.
					_ = s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"","features":[]}`))
				}
				return nil
			}))
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			finished := waitJob(t, s, job.ID)
			if finished.Status != "failed" || finished.ErrorCode != tc.wantCode || !strings.Contains(finished.Error, tc.wantText) {
				t.Fatalf("status %s code %q error %q", finished.Status, finished.ErrorCode, finished.Error)
			}
			if tc.wantCode == "planning_protocol" && strings.Contains(finished.Error, "charged") {
				t.Fatalf("protocol failure still blames the correction budget: %q", finished.Error)
			}
		})
	}
}

// One rejected submission followed by prose-only rounds ends the loop with
// corrections left; the text must not claim the budget was spent.
func TestPlanningFailureWithCorrectionsLeftDoesNotClaimSpentBudget(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	var rounds atomic.Int32
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if rounds.Add(1) == 1 {
			_ = s.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"minimal","objective":"","features":[]}`))
		}
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if finished.Status != "failed" || finished.ErrorCode != "planning" || rounds.Load() != 3 {
		t.Fatalf("status %s code %q rounds %d error %q", finished.Status, finished.ErrorCode, rounds.Load(), finished.Error)
	}
	if strings.Contains(finished.Error, "budget was spent") || !strings.Contains(finished.Error, "without an accepted design (1 of 3 corrections used)") {
		t.Fatalf("wording does not follow the budget state: %q", finished.Error)
	}
}
