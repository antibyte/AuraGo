package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestDesignSettingsCorrectionReachesBuilding(t *testing.T) {
	for _, base := range []string{"three", "minimal", "platformer"} {
		for _, correction := range []string{`{"features":["Run forever and dodge obstacles"]}`, `{"settings":null}`} {
			t.Run(base+correction, func(t *testing.T) {
				s := newTestService(t)
				dimension := "2d"
				if base == "three" {
					dimension = "3d"
				}
				project := createTestProject(t, s, dimension)
				s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
					if run.Stage == "building" {
						if run.Plan.Template != base || run.Plan.Gameplay != nil || run.Plan.Objective != "Endless runner with obstacles" || run.Plan.Scope[0] != "Run forever and dodge obstacles" || run.Plan.Mechanics["lives"] != float64(3) {
							t.Errorf("correction discarded intent or retained invalid settings: %+v", run.Plan)
						}
						return errors.New("settings corrected")
					}
					initial := fmt.Sprintf(`{"base":%q,"objective":"Endless runner with obstacles","features":["Run forever and dodge obstacles"],"mechanics":{"lives":3},"settings":{"goal":5,"speed":5,"duration":120}}`, base)
					if err := s.SetDesignJSON(ctx, run.Job.ID, []byte(initial)); err == nil || !strings.Contains(err.Error(), "settings") {
						t.Fatalf("explicit unsupported settings must be rejected: %v", err)
					}
					if plan, _ := s.GetPlan(ctx, run.Job.ID); plan != nil {
						t.Fatal("rejected settings published a plan")
					}
					return s.SetDesignJSON(ctx, run.Job.ID, []byte(correction))
				}))
				job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
				if err != nil {
					t.Fatal(err)
				}
				if done := waitJob(t, s, job.ID); done.Error != "settings corrected" {
					t.Fatalf("documented correction failed: %s", done.Error)
				}
			})
		}
	}
}

func TestDesignSettingsFollowCorrectedBase(t *testing.T) {
	for _, correction := range []string{`{"base":"three"}`, `{"base":"three","settings":null}`} {
		s := newTestService(t)
		project := Project{Dimension: "3d"}
		_, err := s.expandDesign(context.Background(), "draft", project, []byte(`{"base":"exploration","objective":"Explore freely","features":["Custom mechanics"],"settings":{"goal":7,"speed":8,"duration":0}}`))
		if err != nil {
			t.Fatal(err)
		}
		data, err := s.expandDesign(context.Background(), "draft", project, []byte(correction))
		if err != nil {
			t.Fatal(err)
		}
		var plan GamePlan
		if err = json.Unmarshal(data, &plan); err != nil {
			t.Fatal(err)
		}
		if err = s.checkPlan(project, plan); err != nil || plan.Template != "three" || plan.Gameplay != nil {
			t.Fatalf("base correction retained guided settings: %+v, %v", plan.Gameplay, err)
		}
	}
}

func TestDesignSettingsGuidedCorrectionsRetainValues(t *testing.T) {
	for _, correction := range []string{`{"features":["New feature"]}`, `{"settings":null}`} {
		s := newTestService(t)
		project := Project{Dimension: "3d"}
		_, err := s.expandDesign(context.Background(), "draft", project, []byte(`{"base":"exploration","objective":"Explore freely","features":["Custom mechanics"],"settings":{"goal":7,"speed":8,"duration":0}}`))
		if err != nil {
			t.Fatal(err)
		}
		data, err := s.expandDesign(context.Background(), "draft", project, []byte(correction))
		if err != nil {
			t.Fatal(err)
		}
		var plan GamePlan
		if err = json.Unmarshal(data, &plan); err != nil {
			t.Fatal(err)
		}
		if err = s.checkPlan(project, plan); err != nil || plan.Gameplay == nil || *plan.Gameplay != (GameSettings{Goal: 7, Speed: 8, Duration: 0}) {
			t.Fatalf("guided correction lost explicit values: %+v, %v", plan.Gameplay, err)
		}
	}
}
