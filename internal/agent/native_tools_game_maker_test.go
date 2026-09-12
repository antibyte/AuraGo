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

func TestGameMakerProjectSchemaExposesOptionalSceneAndMechanics(t *testing.T) {
	props := nativeToolProperties(t, appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true}), "game_maker_project")
	operation := props["operation"].(map[string]interface{})["enum"]
	for _, want := range []string{"scene_inspect", "scene_set", "scene_patch", "scene_generate"} {
		if !containsInterfaceString(operation, want) {
			t.Fatalf("project operation enum missing %s: %#v", want, operation)
		}
	}
	for _, key := range []string{"scene", "patch", "generate", "expected_sha256", "node_ids", "region_id", "dry_run"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("project schema missing %s", key)
		}
	}
	validate := nativeToolProperties(t, appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true}), "game_maker_validate")
	checkIDs, ok := validate["check_ids"].(map[string]interface{})
	if !ok || checkIDs["maxItems"] != 16 {
		t.Fatalf("validate schema check_ids = %#v", validate["check_ids"])
	}
	design := props["design"].(map[string]interface{})["properties"].(map[string]interface{})
	for _, key := range []string{"scene", "mechanics"} {
		if _, ok := design[key]; !ok {
			t.Fatalf("design schema missing optional %s", key)
		}
	}
	planning := nativeToolProperties(t, GameMakerPhaseToolSchemas("planning", "2d"), "game_maker_project")
	for _, key := range []string{"scene", "patch", "generate", "expected_sha256", "dry_run"} {
		if _, ok := planning[key]; ok {
			t.Fatalf("planning schema exposed mutation payload %s", key)
		}
	}
	for _, key := range []string{"node_ids", "region_id"} {
		if _, ok := planning[key]; !ok {
			t.Fatalf("planning schema missing inspect filter %s", key)
		}
	}
	building := nativeToolProperties(t, GameMakerPhaseToolSchemas("building", "2d"), "game_maker_project")
	for _, key := range []string{"scene", "patch", "generate", "expected_sha256", "dry_run"} {
		if _, ok := building[key]; !ok {
			t.Fatalf("building schema missing %s", key)
		}
	}
}

func TestGameMakerPlanResponseCompactsScene(t *testing.T) {
	plan := gamemaker.GamePlan{
		SchemaVersion: 4,
		Scene: &gamemaker.Scene{
			SchemaVersion: 1, Dimension: "2d", Seed: 4,
			Levels: []gamemaker.SceneLevel{{ID: "main", Active: true}},
			Nodes:  []gamemaker.SceneNode{{ID: "player", Kind: "player"}},
		},
	}
	result, ok := gameMakerPlanResult(&plan).(map[string]any)
	if !ok {
		t.Fatal("plan response is not an object")
	}
	scene, ok := result["scene"].(map[string]any)
	if !ok || scene["path"] != gamemaker.SceneFilePath || scene["schema_version"] != 1 {
		t.Fatalf("compacted scene header missing: %#v", result["scene"])
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "\"nodes\":[") {
		t.Fatalf("get_plan returned full scene arrays: %s", encoded)
	}
}

func TestGameMakerMechanicsSchemaUsesBoundedKinds(t *testing.T) {
	schemaValue := gameMakerMechanicsSchema()
	properties := schemaValue["properties"].(map[string]interface{})
	blocks := properties["blocks"].(map[string]interface{})
	item := blocks["items"].(map[string]interface{})
	blockProperties := item["properties"].(map[string]interface{})
	kind := blockProperties["kind"].(map[string]interface{})
	want := []string{
		"movement", "camera", "health", "collect", "destroy", "reach", "survive", "checkpoint",
		"patrol", "chase", "keepdistance", "damage", "projectile", "waves", "inventory", "dialogue", "unlock",
	}
	got, ok := kind["enum"].([]string)
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("mechanics kind enum = %#v, want %#v", kind["enum"], want)
	}
	params := blockProperties["params"].(map[string]interface{})
	if params["type"] != "string" {
		t.Fatalf("mechanics params type = %#v, want string", params["type"])
	}
}

func TestGameMakerSceneResultForwardsHashesAndPrioritizesConflict(t *testing.T) {
	result := gamemaker.SceneResult{
		SHA256:         "next",
		CurrentSHA256:  "current",
		ProposedSHA256: "next",
		Diagnostics: []gamemaker.SceneDiagnostic{{
			Severity: "warning",
			Path:     "reachability",
			Message:  "not proven",
		}},
	}
	output := gameMakerSceneResult("scene_patch", result, errors.New("scene_sha256_conflict"))
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["current_sha256"] != "current" || decoded["proposed_sha256"] != "next" {
		t.Fatalf("conditional hashes were not forwarded: %#v", decoded)
	}
	if decoded["first_failure"] != "scene_sha256_conflict" {
		t.Fatalf("first failure = %#v, want fallback conflict", decoded["first_failure"])
	}

	result.Changes = &gamemaker.SceneChanges{Modified: []string{"node-player"}, Counts: map[string]int{"nodes": 1}}
	output = gameMakerSceneResult("scene_patch", result, errors.New("scene_sha256_conflict"))
	if !strings.Contains(output, "\"changes\":{\"modified\":[\"node-player\"]") {
		t.Fatalf("scene changes were not compactly forwarded: %s", output)
	}

	result.Diagnostics = []gamemaker.SceneDiagnostic{{
		Severity: "error",
		Path:     "scene",
		Message:  "invalid node",
	}}
	output = gameMakerSceneResult("scene_patch", result, errors.New("scene_sha256_conflict"))
	if !strings.Contains(output, "\"first_failure\":\"scene: invalid node\"") {
		t.Fatalf("diagnostic error should take precedence: %s", output)
	}
}

func TestGameMakerSceneInspectFilterCompactsSelectedNodes(t *testing.T) {
	scene := gamemaker.Scene{
		SchemaVersion: 1,
		Dimension:     "2d",
		Levels:        []gamemaker.SceneLevel{{ID: "main", Active: true}},
	}
	for i := 0; i < 40; i++ {
		region := "west"
		if i%2 == 0 {
			region = "east"
		}
		scene.Nodes = append(scene.Nodes, gamemaker.SceneNode{
			ID: iotaSceneNodeID(i), Kind: "prop", Position: gamemaker.Vec3{float64(i), 2, 0},
			Size: gamemaker.Vec3{1, 1, 0}, RegionID: region,
			Properties: map[string]any{"color": i},
		})
	}
	scene.Placements = []gamemaker.ScenePlacement{{
		ID: "prop-placement", NodeID: iotaSceneNodeID(3), AssetRole: "prop", Behavior: "decorative",
		Position: gamemaker.Vec3{3, 2, 0},
	}}
	filter, err := gameMakerSceneFilterFromParams(map[string]any{
		"node_ids":  []any{iotaSceneNodeID(3), iotaSceneNodeID(4)},
		"region_id": "west",
	})
	if err != nil {
		t.Fatal(err)
	}
	detail := gameMakerSceneDetail(scene, filter)
	nodes := detail["nodes"].([]map[string]any)
	if len(nodes) != 1 || nodes[0]["id"] != iotaSceneNodeID(3) {
		t.Fatalf("filtered nodes = %#v", nodes)
	}
	if _, ok := nodes[0]["placements"]; !ok {
		t.Fatalf("selected node lost its exact placement binding: %#v", nodes[0])
	}

	regionFilter, err := gameMakerSceneFilterFromParams(map[string]any{"region_id": "east"})
	if err != nil {
		t.Fatal(err)
	}
	detail = gameMakerSceneDetail(scene, regionFilter)
	if len(detail["nodes"].([]map[string]any)) != 20 || len(detail["regions"].([]gamemaker.SceneRegion)) != 0 {
		t.Fatalf("region details = %#v", detail)
	}
	for i := 0; i < 40; i++ {
		scene.Nodes = append(scene.Nodes, gamemaker.SceneNode{ID: iotaSceneNodeID(100 + i), RegionID: "east"})
	}
	detail = gameMakerSceneDetail(scene, regionFilter)
	if len(detail["nodes"].([]map[string]any)) != 32 || detail["truncated"] != true {
		t.Fatalf("region detail cap = %#v", detail)
	}
}

func iotaSceneNodeID(index int) string {
	return fmt.Sprintf("node-%d", index)
}
