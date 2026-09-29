package agent

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"
)

// The isolated agent must be able to read what validation will drive before it
// writes code, and every validation answer must say what to do next.
func TestGameMakerToolsDiscloseChecksAndNextAction(t *testing.T) {
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
	decode := func(output string) map[string]any {
		t.Helper()
		var value map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &value); err != nil {
			t.Fatalf("tool output is not JSON: %v: %s", err, output)
		}
		return value
	}
	outputs := map[string]map[string]any{}
	s.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		bound := gamemaker.WithJobContext(ctx, run.Job.ID)
		inspect, handled := dispatchGameMaker(bound, ToolCall{Action: "game_maker_project", Params: map[string]any{"operation": "inspect"}}, nil)
		if !handled {
			return errors.New("inspect was not dispatched")
		}
		outputs[run.Stage+"_inspect"] = decode(inspect)
		if run.Stage == "planning" {
			return s.SetPlan(ctx, run.Job.ID, gamemaker.ExampleGamePlan(project))
		}
		missing, _ := dispatchGameMaker(bound, ToolCall{Action: "game_maker_file", Params: map[string]any{"operation": "read", "path": "src/missing.ts"}}, nil)
		outputs["missing"] = decode(missing)
		search, _ := dispatchGameMaker(bound, ToolCall{Action: "game_maker_file", Params: map[string]any{"operation": "search", "query": "extends GameScene"}}, nil)
		outputs["search"] = decode(search)
		// The untouched starter fails immediately, without waiting for a browser.
		validation, _ := dispatchGameMaker(bound, ToolCall{Action: "game_maker_validate", Params: map[string]any{"scope": "full"}}, nil)
		outputs["validate"] = decode(validation)
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

	planning := outputs["planning_inspect"]
	if planning == nil || planning["base_checks"] == nil || planning["validation_plan"] != nil {
		t.Fatalf("planning inspect = %+v", planning)
	}
	bases, _ := json.Marshal(planning["base_checks"])
	if !strings.Contains(string(bases), "reach item") || strings.Contains(string(bases), "fps") {
		t.Fatalf("planning base checks = %s", bases)
	}

	building := outputs["building_inspect"]
	if building == nil || building["base_checks"] != nil {
		t.Fatalf("building inspect = %+v", building)
	}
	plan, ok := building["validation_plan"].(map[string]any)
	if !ok || plan["scope"] != "full" {
		t.Fatalf("building inspect lost the validation plan: %+v", building["validation_plan"])
	}
	encoded, _ := json.Marshal(plan)
	for _, want := range []string{"required_rules", "required_restart", "required_controls", `"target":"item"`, `"target_roles"`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("validation plan lacks %s: %s", want, encoded)
		}
	}

	missing := outputs["missing"]
	message, _ := missing["message"].(string)
	if missing["status"] != "error" || !strings.Contains(message, "src/missing.ts") || !strings.Contains(message, "list_files") ||
		strings.Contains(strings.ToLower(filepath.ToSlash(message)), strings.ToLower(filepath.ToSlash(root))) {
		t.Errorf("missing file answer = %+v", missing)
	}
	search := outputs["search"]
	found, _ := json.Marshal(search["result"])
	if search["status"] != "ok" || !strings.Contains(string(found), "src/main.ts") || search["next_action"] == nil {
		t.Errorf("project-wide search = %+v", search)
	}

	validation := outputs["validate"]
	if validation == nil || validation["status"] != "error" {
		t.Fatalf("validation output = %+v", validation)
	}
	if action, _ := validation["next_action"].(string); !strings.Contains(action, "source location") {
		t.Errorf("validation lacks a next action: %+v", validation["next_action"])
	}
	if _, ok := validation["remaining_repair_passes"].(float64); !ok {
		t.Errorf("validation lacks the remaining repair allowance: %+v", validation)
	}
}
