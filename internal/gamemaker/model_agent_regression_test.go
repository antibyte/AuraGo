package gamemaker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestModelJobRejectsImportedButUnimplementedStarter(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "3d")
	repairs := 0
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage == "planning" {
			plan := ExampleGamePlan(project)
			plan.Assets = []PlanAsset{
				{Role: "arms", PackID: ModelPackID, Version: "1.0.0", AssetID: "fps-arms-modern", Scale: 1, Collider: "none"},
				{Role: "weapon", PackID: ModelPackID, Version: "1.0.0", AssetID: "fps-rifle", Scale: 1, Collider: "none"},
			}
			return s.SetPlan(ctx, run.Job.ID, plan)
		}
		if run.Stage == "repair" {
			repairs++
			if len(run.Diagnostics) != 1 || !strings.Contains(run.Diagnostics[0].Message, "No implementation was written") {
				t.Errorf("repair did not receive implementation failure: %+v", run.Diagnostics)
			}
		}
		packs, err := s.importedJobPacks(ctx, run.Job.ID)
		if err != nil {
			return err
		}
		if len(packs) != 1 || len(packs[0].AssetIDs) != 2 || !strings.Contains(packs[0].ThreeExample, "loadAsset") || packs[0].Manifests["fps-arms-modern"] == "" || packs[0].Manifests["fps-rifle"] == "" {
			t.Errorf("existing model import lost its usable example: %+v", packs)
		}
		// Reproduce the reported agent: imports/reads only, then normal completion.
		return nil
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{Prompt: "FPS shooter"})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitJob(t, s, job.ID)
	if finished.Status != "failed" || finished.ResultRevision != 0 || repairs == 0 {
		t.Fatalf("unimplemented model game was published or skipped repair: %+v repairs=%d", finished, repairs)
	}
	revisions, err := s.ListRevisions(context.Background(), project.ID)
	if err != nil || len(revisions) != 0 {
		t.Fatalf("unexpected published revision: %v, %v", revisions, err)
	}
}

func TestModelDescriptionKeepsExampleBeforeLargeMetadata(t *testing.T) {
	s := newTestService(t)
	detail, err := s.DescribeAsset(ModelPackID, "fps-arms-modern", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	end := strings.Index(text, "releaseAsset")
	if !strings.HasPrefix(text, `{"example":`) || end < 0 || end > 1400 {
		t.Fatal("bounded tool output hides the complete loading/disposal example")
	}
}
