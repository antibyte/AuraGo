package gamemaker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSourceEditsSaveWithoutBuildingAndPreserveStaleFiles(t *testing.T) {
	s := newTestService(t)
	project := createTestProject(t, s, "2d")
	s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
		if run.Stage != "building" {
			return errors.New("unexpected repair")
		}
		bad, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/main.ts", "const broken = ;", "")
		if err != nil || !bad.Written || bad.Build.OK || len(bad.Build.Diagnostics) != 0 {
			t.Errorf("write must save without compiling: %+v %v", bad, err)
		}
		read, err := s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 0, 0)
		if err != nil || read.SHA256 != bad.SHA256 {
			t.Fatalf("read: %+v %v", read, err)
		}
		if _, err = s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", "const broken = ;", "const fixed = 1;", "wrong"); err == nil {
			t.Error("stale edit accepted")
		}
		if got, _ := s.ReadJobFile(ctx, run.Job.ID, "src/main.ts"); !strings.Contains(got, "const broken = ;") {
			t.Error("conflict mutated file")
		}
		good, err := s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", "const broken = ;", "const fixed = 1;", read.SHA256)
		if err != nil || good.Build.OK || good.SHA256 == read.SHA256 {
			t.Errorf("replace: %+v %v", good, err)
		}
		if _, err = s.ReplaceJobFile(ctx, run.Job.ID, "src/main.ts", "", "anything", good.SHA256); err == nil {
			t.Error("empty target accepted")
		}
		for _, path := range []string{"../escape.ts", "vendor/runtime.js", ".aurago/game-plan.json", "dist/game.js"} {
			if _, err = s.WriteJobFileChecked(ctx, run.Job.ID, path, "", ""); err == nil {
				t.Errorf("path writable: %s", path)
			}
		}
		if _, err = s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 1, 300); err == nil {
			t.Error("unbounded range accepted")
		}
		full, err := s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", 1, 240)
		if err != nil || full.EndLine != full.TotalLines || full.SHA256 != good.SHA256 || !strings.Contains(full.Content, "const fixed = 1;") {
			t.Errorf("bounded range must stop at EOF and retain the full digest: %+v %v", full, err)
		}
		if _, err = s.ReadJobFileRange(ctx, run.Job.ID, "src/main.ts", full.TotalLines+1, full.TotalLines+2); err == nil {
			t.Error("range starting past EOF accepted")
		}
		return errors.New("checked edits")
	}))
	job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitJob(t, s, job.ID); done.Error != "checked edits" {
		t.Fatal(done.Error)
	}
}
