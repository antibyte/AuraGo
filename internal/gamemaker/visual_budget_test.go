package gamemaker

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestVisualRepairSharesTechnicalRepairAllowance(t *testing.T) {
	for failures := 0; failures <= 4; failures++ {
		t.Run(fmt.Sprint(failures), func(t *testing.T) {
			s := newTestService(t)
			s.validationFailures["job"] = failures
			repairs := 0
			runner := visualRunner(func(_ context.Context, run JobRun) error {
				if run.Stage == "visual" {
					run.Result.Visual = VisualReview{Status: "reviewed", Findings: []VisualFinding{{Severity: "defect", Confidence: .99, Observation: "Overlapping HUD"}}}
					return nil
				}
				repairs++
				return errors.New("stop before fixture edits")
			})
			result := s.reviewAndRepairVisual(context.Background(), runner, JobRun{Job: Job{ID: "job"}}, BuildResult{OK: true}, "full")
			if failures < 3 && repairs != 1 {
				t.Fatalf("remaining repair allowance was lost: repairs=%d", repairs)
			}
			if failures >= 3 && (repairs != 0 || !result.OK || len(result.Visual.Findings) != 1) {
				t.Fatalf("exhausted visual repair changed a passing result: repairs=%d, result=%+v", repairs, result)
			}
		})
	}
}
