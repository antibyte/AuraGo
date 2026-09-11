package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/gamemaker"

	openai "github.com/sashabaranov/go-openai"
)

func TestGameMakerPlanSurvivesAdvertisedAndFallbackTransports(t *testing.T) {
	// Exercise the provider-normalized schema, not only the raw tool definition.
	found := false
	for _, tool := range BuildNativeToolSchemas(t.TempDir(), nil, ToolFeatureFlags{GameMakerEnabled: true}, nil) {
		if tool.Function.Name == "game_maker_project" {
			found = true
			props := tool.Function.Parameters.(map[string]interface{})["properties"].(map[string]interface{})
			if props["plan"].(map[string]interface{})["type"] != "string" {
				t.Fatal("update transport coverage when the advertised plan format changes")
			}
		}
	}
	if !found {
		t.Fatal("game_maker_project is missing from the provider schema")
	}
	want := gamemaker.ExampleGamePlan(gamemaker.Project{Dimension: "2d"})
	want.Template = "blocks"
	want.Objective = "Breakout mit Power-ups, Sounds und Musik"
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	for _, transport := range []string{"native_object", "native_string", "json_string", "bracket_string", "xml_function", "xml_invoke"} {
		t.Run(transport, func(t *testing.T) {
			var tc ToolCall
			if transport == "native_object" || transport == "native_string" {
				var plan any = json.RawMessage(data)
				if transport == "native_string" {
					plan = string(data)
				}
				args, _ := json.Marshal(map[string]any{"job_id": "job-test", "operation": "set_plan", "plan": plan})
				tc = NativeToolCallToToolCall(openai.ToolCall{Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "game_maker_project", Arguments: string(args)}}, nil)
			} else if transport == "json_string" || transport == "bracket_string" {
				args, _ := json.Marshal(map[string]any{"action": "game_maker_project", "job_id": "job-test", "operation": "set_plan", "plan": string(data)})
				body := string(args)
				if transport == "bracket_string" {
					body = "[TOOL_CALL]" + body + "[/TOOL_CALL]"
				}
				tc = ParseToolCall(body)
			} else {
				body := "<parameter=job_id>job-test</parameter><parameter=operation>set_plan</parameter><parameter=plan>" + string(data) + "</parameter>"
				text := "<tool_call><function=game_maker_project>" + body + "</function></tool_call>"
				if transport == "xml_invoke" {
					text = "<function><invoke name=\"game_maker_project\">" + body + "</invoke></function>"
				}
				tc = ParseToolCall(text)
				if tc.TaskPrompt != string(data) {
					t.Fatal("legacy plan/task_prompt alias lost")
				}
			}
			if tc.Action != "game_maker_project" || tc.Operation != "set_plan" || toolArgString(tc.Params, "job_id") != "job-test" {
				t.Fatalf("routing arguments lost: %+v", tc)
			}
			root := t.TempDir()
			service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "game.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true, AllowEdit: true})
			if err != nil {
				t.Fatal(err)
			}
			defer service.Close()
			previous := gamemaker.DefaultService()
			gamemaker.SetDefaultService(service)
			defer gamemaker.SetDefaultService(previous)
			service.SetSkillStatus(nil, true)
			project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Breaker", Dimension: "2d", Description: want.Objective})
			if err != nil {
				t.Fatal(err)
			}
			accepted := make(chan *gamemaker.GamePlan, 1)
			service.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
				if run.Stage != "planning" {
					accepted <- run.Plan
					return errors.New("test stops after reaching building")
				}
				tc.Params["job_id"] = run.Job.ID
				output, handled := dispatchGameMaker(ctx, tc, nil)
				if !handled || !strings.Contains(output, `"status":"ok"`) {
					accepted <- nil
					return fmt.Errorf("set_plan dispatch: %s", output)
				}
				return nil
			}))
			if _, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{}); err != nil {
				t.Fatal(err)
			}
			select {
			case plan := <-accepted:
				if !reflect.DeepEqual(plan, &want) {
					t.Fatalf("dispatched plan did not reach building intact: %+v", plan)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("set_plan did not advance to building")
			}
		})
	}
}

type gameMakerPlanTestRunner func(context.Context, gamemaker.JobRun) error

func (r gameMakerPlanTestRunner) RunGameMakerJob(ctx context.Context, run gamemaker.JobRun) error {
	return r(ctx, run)
}

func TestGameMakerAssetSchemaKeepsGenerationAndPackOperations(t *testing.T) {
	if got := appendGameMakerToolSchemas(nil, ToolFeatureFlags{}); len(got) != 0 {
		t.Fatal("disabled Game Maker advertised tools")
	}
	tools := appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true})
	if len(tools) != 4 {
		t.Fatalf("tool isolation changed: %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Function.Name != "game_maker_asset" {
			continue
		}
		data, err := json.Marshal(tool.Function.Parameters)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Required   []string
			Properties map[string]struct{ Enum []string }
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(schema.Required, []string{"job_id"}) {
			t.Fatalf("default generation compatibility: required %v", schema.Required)
		}
		if !slices.Equal(schema.Properties["operation"].Enum, []string{"generate", "list_packs", "describe_pack", "import_pack", "search_assets", "describe_asset"}) {
			t.Fatal("missing pack operations")
		}
		for _, key := range []string{"kind", "prompt", "path", "pack_id", "query", "view", "limit", "asset_id", "assembly_id"} {
			if _, ok := schema.Properties[key]; !ok {
				t.Fatalf("missing %s", key)
			}
		}
		return
	}
	t.Fatal("missing game_maker_asset")
}
