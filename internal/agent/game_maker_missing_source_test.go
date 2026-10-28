package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"
)

func TestGameMakerExistingSourcesAreBoundedSortedAndTextOnly(t *testing.T) {
	files := []string{"vendor/three.js", "assets/builtin/a.glb", "src/zeta.ts", "src/main.ts", "src/model.glb", "game.json"}
	for i := range 14 {
		files = append(files, fmt.Sprintf("src/part%02d.ts", i))
	}
	got := gameMakerExistingSources(files)
	if !strings.HasPrefix(got, "existing src files: src/main.ts, src/part00.ts, src/part01.ts") {
		t.Fatalf("list is not sorted: %s", got)
	}
	// 16 text sources; only the first 12 are named and the rest are counted.
	if strings.Count(got, "src/") != 12 || !strings.HasSuffix(got, "(+4 more)") {
		t.Fatalf("list is not bounded: %s", got)
	}
	for _, unwanted := range []string{"vendor/", "assets/", ".glb", "game.json"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("list names %s: %s", unwanted, got)
		}
	}
	if gameMakerExistingSources([]string{"game.json", "assets/x.png"}) != "" {
		t.Fatal("a project without sources produced a list")
	}
}

func TestGameMakerMissingSourceReadNamesExistingFilesOnlyForBoundJobs(t *testing.T) {
	root := t.TempDir()
	s, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(s)
	defer gamemaker.SetDefaultService(previous)
	s.SetSkillStatus(nil, true)
	project, err := s.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Orchard", Dimension: "2d", Description: "Collect apples in an orchard"})
	if err != nil {
		t.Fatal(err)
	}
	message := func(output string) string {
		t.Helper()
		var value map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &value); err != nil {
			t.Fatalf("tool output is not JSON: %v: %s", err, output)
		}
		text, _ := value["message"].(string)
		return text
	}
	answers := map[string]string{}
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, gamemaker.ExampleGamePlan(project))
		}
		if _, err := s.WriteJobFileChecked(ctx, run.Job.ID, "src/zz-helpers.ts", "export const helper = 1;", ""); err != nil {
			return err
		}
		read := ToolCall{Action: "game_maker_file", Params: map[string]any{"operation": "read", "path": "src/three-common.ts", "job_id": run.Job.ID}}
		bound, _ := dispatchGameMaker(gamemaker.WithJobContext(ctx, run.Job.ID), read, nil)
		unbound, _ := dispatchGameMaker(ctx, read, nil)
		search := ToolCall{Action: "game_maker_file", Params: map[string]any{"operation": "search", "path": "src/three-common.ts", "query": "x"}}
		boundSearch, _ := dispatchGameMaker(gamemaker.WithJobContext(ctx, run.Job.ID), search, nil)
		answers["bound"], answers["unbound"], answers["search"] = message(bound), message(unbound), message(boundSearch)
		return errors.New("fixture complete")
	}))
	job, err := s.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; {
		finished, err := s.GetJob(context.Background(), job.ID)
		if err == nil && (finished.Status == "failed" || finished.Status == "ready") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fixture job did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	for _, name := range []string{"bound", "search"} {
		if !strings.Contains(answers[name], "list_files") || !strings.Contains(answers[name], "existing src files: ") ||
			!strings.Contains(answers[name], "src/main.ts") || !strings.Contains(answers[name], "src/zz-helpers.ts") {
			t.Errorf("%s missing-file answer does not name the existing sources: %q", name, answers[name])
		}
	}
	if !strings.Contains(answers["unbound"], "list_files") || strings.Contains(answers["unbound"], "existing src files") {
		t.Errorf("unbound answer = %q", answers["unbound"])
	}
}
