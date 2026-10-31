package server

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/gamemaker"

	"github.com/sashabaranov/go-openai"
)

// gameMakerProjectProperties returns the game_maker_project parameter schema of
// one prepared profile.
func gameMakerProjectProperties(t *testing.T, profile *agent.PreparedPromptProfile) map[string]any {
	t.Helper()
	for _, tool := range profile.Tools() {
		if tool.Function.Name != "game_maker_project" {
			continue
		}
		return tool.Function.Parameters.(map[string]any)["properties"].(map[string]any)
	}
	t.Fatalf("%s exposes no game_maker_project tool", profile.Name())
	return nil
}

func gameMakerDesignProperties(t *testing.T, profile *agent.PreparedPromptProfile) map[string]any {
	t.Helper()
	design, ok := gameMakerProjectProperties(t, profile)["design"].(map[string]any)
	if !ok {
		t.Fatalf("%s has no compact design object", profile.Name())
	}
	properties, ok := design["properties"].(map[string]any)
	if !ok {
		t.Fatalf("%s design has no properties", profile.Name())
	}
	return properties
}

// schemaJSON serializes one ordered tool list for byte comparisons.
func schemaJSON(t *testing.T, tools []openai.Tool) string {
	t.Helper()
	data, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func toolsExcept(t *testing.T, profile *agent.PreparedPromptProfile, skip string) []openai.Tool {
	t.Helper()
	out := []openai.Tool{}
	for _, tool := range profile.Tools() {
		if tool.Function.Name == skip {
			continue
		}
		out = append(out, tool)
	}
	return out
}

// The compact planning schema drops only the server-filled design fields and
// leaves every other tool byte-identical.
func TestCompactProfileOmitsDesignFields(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		normal, err := gameMakerPromptProfile("planning", dimension)
		if err != nil {
			t.Fatal(err)
		}
		compact, err := gameMakerPromptProfile("planning", dimension, "compact")
		if err != nil {
			t.Fatal(err)
		}
		normalDesign := gameMakerDesignProperties(t, normal)
		compactDesign := gameMakerDesignProperties(t, compact)
		removedFields := []string{"assets", "presentation", "scenarios", "scene"}
		if dimension == "3d" {
			// 2D never offered guided settings; 3D does.
			removedFields = append(removedFields, "settings")
		}
		for _, removed := range removedFields {
			if normalDesign[removed] == nil {
				t.Fatalf("%s normal design lost %q", dimension, removed)
			}
			if _, exists := compactDesign[removed]; exists {
				t.Errorf("%s compact design still offers %q", dimension, removed)
			}
		}
		for _, kept := range []string{"base", "objective", "features", "stages"} {
			if compactDesign[kept] == nil {
				t.Errorf("%s compact design lacks %q", dimension, kept)
			}
		}
		// No other tool changes, and only the design object changes inside
		// game_maker_project.
		if got, want := schemaJSON(t, toolsExcept(t, compact, "game_maker_project")), schemaJSON(t, toolsExcept(t, normal, "game_maker_project")); got != want {
			t.Errorf("%s compact profile changed another tool schema", dimension)
		}
		compactProject := gameMakerProjectProperties(t, compact)
		normalProject := gameMakerProjectProperties(t, normal)
		for key := range normalProject {
			if key == "design" {
				continue
			}
			compactValue, _ := json.Marshal(compactProject[key])
			normalValue, _ := json.Marshal(normalProject[key])
			if string(compactValue) != string(normalValue) {
				t.Errorf("%s compact game_maker_project changed %q: %s != %s", dimension, key, compactValue, normalValue)
			}
		}
		// Building schemas are identical: the compact variant only reshapes design.
		normalBuilding, _ := gameMakerPromptProfile("building", dimension)
		compactBuilding, _ := gameMakerPromptProfile("building", dimension, "compact")
		if got, want := schemaJSON(t, compactBuilding.Tools()), schemaJSON(t, normalBuilding.Tools()); got != want {
			t.Errorf("%s compact building schemas differ from the frozen set", dimension)
		}
	}
}

// Required arguments keep their documented order; only the stage commitment is
// capped for small models.
func TestCompactProfileKeepsRequiredOrder(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		normal, err := gameMakerPromptProfile("planning", dimension)
		if err != nil {
			t.Fatal(err)
		}
		compact, err := gameMakerPromptProfile("planning", dimension, "compact")
		if err != nil {
			t.Fatal(err)
		}
		for name, profile := range map[string]*agent.PreparedPromptProfile{"normal": normal, "compact": compact} {
			design := gameMakerProjectProperties(t, profile)["design"].(map[string]any)
			required, ok := design["required"].([]any)
			wantRequired := []any{"base", "objective", "features"}
			if !ok || !reflect.DeepEqual(required, wantRequired) {
				t.Fatalf("%s %s design required = %v, want %v", dimension, name, design["required"], wantRequired)
			}
			projectRequired, ok := gameMakerProjectProperties(t, profile)["operation"].(map[string]any)
			if !ok || projectRequired["type"] != "string" {
				t.Fatalf("%s %s project operation schema changed", dimension, name)
			}
		}
		normalDesign := gameMakerDesignProperties(t, normal)
		compactDesign := gameMakerDesignProperties(t, compact)
		if got := normalDesign["stages"].(map[string]any)["maxItems"]; got != float64(6) {
			t.Fatalf("%s normal stages maxItems = %v, want 6", dimension, got)
		}
		if got := compactDesign["stages"].(map[string]any)["maxItems"]; got != float64(2) {
			t.Fatalf("%s compact stages maxItems = %v, want 2", dimension, got)
		}
		// The properties themselves stay the same objects: compact only removes keys.
		for _, kept := range []string{"base", "objective", "features"} {
			left, _ := json.Marshal(normalDesign[kept])
			right, _ := json.Marshal(compactDesign[kept])
			if string(left) != string(right) {
				t.Fatalf("%s compact %q schema changed: %s != %s", dimension, kept, right, left)
			}
		}
	}
}

// Non-voxel profiles publish as v4 after the base hints and repair examples.
// Voxel text is unchanged: v2, or v3 only in the small-model name.
func TestNormalProfilesStayOnV2(t *testing.T) {
	type profileCase struct {
		stage, dimension string
		variant          []string
		want             string
	}
	cases := []profileCase{
		{"planning", "2d", nil, "game-maker/v4/planning/2d"},
		{"building", "2d", nil, "game-maker/v4/building/2d"},
		{"repair", "2d", nil, "game-maker/v4/building/2d"},
		{"planning", "3d", nil, "game-maker/v4/planning/3d"},
		{"building", "3d", nil, "game-maker/v4/building/3d"},
		{"repair", "3d", nil, "game-maker/v4/building/3d"},
		{"planning", "3d", []string{"voxel"}, "game-maker/v2/planning/voxel"},
		{"building", "3d", []string{"voxel"}, "game-maker/v2/building/voxel"},
		{"planning", "2d", []string{"compact"}, "game-maker/v4/planning/2d/compact"},
		{"building", "2d", []string{"compact"}, "game-maker/v4/building/2d/compact"},
		{"planning", "3d", []string{"compact"}, "game-maker/v4/planning/3d/compact"},
		{"building", "3d", []string{"compact"}, "game-maker/v4/building/3d/compact"},
		{"planning", "3d", []string{"voxel", "compact"}, "game-maker/v3/planning/3d/voxel"},
		{"building", "3d", []string{"voxel", "compact"}, "game-maker/v3/building/3d/voxel"},
	}
	for _, c := range cases {
		profile, err := gameMakerPromptProfile(c.stage, c.dimension, c.variant...)
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		if profile.Name() != c.want {
			t.Errorf("%s/%s/%v profile = %q, want %q", c.stage, c.dimension, c.variant, profile.Name(), c.want)
		}
	}
	// A voxel small-model run must stay byte-identical to its v2 profile: only
	// the versioned name moves.
	for _, stage := range []string{"planning", "building"} {
		v2, err := gameMakerPromptProfile(stage, "3d", "voxel")
		if err != nil {
			t.Fatal(err)
		}
		v3, err := gameMakerPromptProfile(stage, "3d", "voxel", "compact")
		if err != nil {
			t.Fatal(err)
		}
		if v2.SystemPrompt() != v3.SystemPrompt() {
			t.Errorf("%s voxel small-model prompt changed", stage)
		}
		if got, want := schemaJSON(t, v3.Tools()), schemaJSON(t, v2.Tools()); got != want {
			t.Errorf("%s voxel small-model schemas changed", stage)
		}
	}
	// Ordinary profiles never advertise the compact or adapt additions.
	for _, stage := range []string{"planning", "building"} {
		for _, dimension := range []string{"2d", "3d"} {
			profile, err := gameMakerPromptProfile(stage, dimension)
			if err != nil {
				t.Fatal(err)
			}
			for _, unwanted := range []string{"## Compact design", "edit_targets", "mode_requires_replace"} {
				if strings.Contains(profile.SystemPrompt(), unwanted) {
					t.Errorf("%s/%s ordinary profile carries %q", stage, dimension, unwanted)
				}
			}
			hasExamples := strings.Contains(profile.SystemPrompt(), "## Repair examples")
			if (stage == "building") != hasExamples {
				t.Errorf("%s/%s repair examples present=%v", stage, dimension, hasExamples)
			}
		}
	}
	compactBuilding, err := gameMakerPromptProfile("building", "2d", "compact")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"edit_targets", "mode_requires_replace", "## Building and repair workflow"} {
		if !strings.Contains(compactBuilding.SystemPrompt(), want) {
			t.Errorf("compact building profile lacks %q", want)
		}
	}
	compactPlanning, err := gameMakerPromptProfile("planning", "2d", "compact")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(compactPlanning.SystemPrompt(), "## Compact design") || strings.Contains(compactPlanning.SystemPrompt(), "## Presentation choices") {
		t.Error("compact planning profile keeps the presentation choices it cannot submit")
	}
}

// Small-model selection: an explicit opt-in wins, automatic detection still
// applies after an explicit false, and unknown models are never small.
func TestSmallModelAutoDetection(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = []config.ProviderEntry{
		{ID: "small-context", Type: "openai", Model: "tiny-context-model", ContextWindow: 32000, MaxOutputTokens: 64000},
		{ID: "small-output", Type: "openai", Model: "tiny-output-model", ContextWindow: 128000, MaxOutputTokens: 8000},
		{ID: "comfortable", Type: "openai", Model: "comfortable-model", ContextWindow: 131072, MaxOutputTokens: 64000},
	}
	for _, tc := range []struct {
		name     string
		provider string
		model    string
		explicit bool
		want     bool
	}{
		{"explicit opt-in wins", "comfortable", "comfortable-model", true, true},
		{"comfortable route stays ordinary", "comfortable", "comfortable-model", false, false},
		{"explicit false cannot disable context detection", "small-context", "tiny-context-model", false, true},
		{"context detection also beats explicit false on the model only", "", "tiny-context-model", false, false},
		{"output detection promotes a wide context", "small-output", "tiny-output-model", false, true},
		{"explicit opt-in on an unknown model", "", "totally-unknown-model-xyz", true, true},
		{"unknown models are never detected", "", "totally-unknown-model-xyz", false, false},
	} {
		if got := gameMakerSmallModelDecision(cfg, tc.provider, tc.model, tc.explicit); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	if gameMakerSmallModelDecision(nil, "small-context", "tiny-context-model", false) {
		t.Error("nil configuration must not resolve a small model")
	}
	// The boundary matches gameMakerOutputTokens: 65536 context and 16384 output
	// are the comfortable side.
	boundary := &config.Config{}
	boundary.Providers = []config.ProviderEntry{
		{ID: "edge", Type: "openai", Model: "edge-model", ContextWindow: 65536, MaxOutputTokens: 16384},
	}
	if gameMakerSmallModelDecision(boundary, "edge", "edge-model", false) {
		t.Error("the exact reasoning-allowance boundary counts as small")
	}
}

// The compact building context bounds the complete entry source and replaces the
// full runtime reference with a cheat sheet derived from it.
func TestCompactSourceWindowIsBounded(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.SetSkillStatus(nil, true)
	service.SetRunner(implementationTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		if run.Stage == "planning" {
			return service.SetDesignJSON(ctx, run.Job.ID, []byte(`{"base":"platformer","objective":"Collect glowing seeds","features":["Collect seeds through distinct stages"],"stages":["Meadow","Ridge"]}`))
		}
		if run.Stage != "building" {
			return nil
		}
		// A complete reference is installed for the plan-bound helper.
		runtimeContext := service.RuntimeContext(ctx, run.Job.ID)
		reference, _ := runtimeContext["api_reference"].(string)
		if len(reference) <= gameMakerCompactAPIReferenceMaxBytes {
			return errors.New("fixture runtime reference is unexpectedly small")
		}
		compactRuntime := gameMakerCompactRuntime(runtimeContext)
		sheet, _ := compactRuntime["api_reference"].(string)
		if len(sheet) > gameMakerCompactAPIReferenceMaxBytes {
			return errors.New("compact api cheat sheet exceeds its bound")
		}
		body, _, _ := strings.Cut(sheet, "\n- Cheat sheet only:")
		if body == "" || !strings.HasPrefix(reference, body) {
			return errors.New("cheat sheet is not derived from the installed reference")
		}
		if compactRuntime["version"] != runtimeContext["version"] {
			return errors.New("compact runtime lost the helper version")
		}
		// A source at the compact window is supplied, one byte more is omitted.
		// The diagnostic header keeps the build from growing the fixture.
		window := gameMakerCompactSourceMaxBytes
		header := "// __AURAGO_GAME_DIAGNOSTICS__\n"
		sized := func(size int) string { return header + strings.Repeat("x", size-len(header)) }
		for _, tc := range []struct {
			name     string
			size     int
			supplied bool
		}{
			{"at the compact window", window, true},
			{"above the compact window", window + 1, false},
			{"between compact and normal", gameMakerCurrentSourceMaxBytes - 1, false},
		} {
			if err := service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", sized(tc.size)); err != nil {
				return err
			}
			current, err := service.ReadJobSource(ctx, run.Job.ID, "src/main.ts", gamemaker.SourceGenerationMaxBytes)
			if err != nil {
				return err
			}
			if len(current.Content) != tc.size {
				return errors.New(tc.name + ": fixture source changed during the build")
			}
			sources, omitted := gameMakerCurrentSourcesWithin(ctx, service, run.Job.ID, window)
			if tc.supplied {
				if len(sources) != 1 || len(omitted) != 0 || sources[0] != current {
					return errors.New(tc.name + ": compact window dropped a complete source")
				}
				continue
			}
			if len(sources) != 0 || len(omitted) != 1 {
				return errors.New(tc.name + ": compact window supplied an oversized source")
			}
			if omitted[0]["max_content_bytes"] != window {
				return errors.New(tc.name + ": omission does not name the compact window")
			}
		}
		// The ordinary window still accepts the larger source.
		if err := service.WriteJobFile(ctx, run.Job.ID, "src/main.ts", sized(gameMakerCurrentSourceMaxBytes)); err != nil {
			return err
		}
		sources, omitted := gameMakerCurrentSources(ctx, service, run.Job.ID)
		if len(sources) != 1 || len(omitted) != 0 || len(sources[0].Content) != gameMakerCurrentSourceMaxBytes {
			return errors.New("ordinary window no longer supplies its own bound")
		}
		return errors.New("compact context bounds verified")
	}))
	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "Compact window", Description: "Collect glowing seeds", Dimension: "2d"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "Collect glowing seeds"})
	if err != nil {
		t.Fatal(err)
	}
	if finished := waitGameMakerJob(t, service, job.ID); finished.Error != "compact context bounds verified" {
		t.Fatalf("job = %+v", finished)
	}
}

// waitGameMakerJob stops at the terminal state of one service job.
func waitGameMakerJob(t *testing.T, service *gamemaker.Service, id string) gamemaker.Job {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); ; {
		done, err := service.GetJob(context.Background(), id)
		if err == nil && !isActiveGameMakerStatus(done.Status) {
			return done
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s did not terminate: %+v", id, done)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func isActiveGameMakerStatus(status string) bool {
	switch status {
	case "queued", "planning", "building", "validating", "polishing":
		return true
	default:
		return false
	}
}
