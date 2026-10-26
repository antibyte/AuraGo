package gamemaker

import (
	"context"
	"strings"
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
