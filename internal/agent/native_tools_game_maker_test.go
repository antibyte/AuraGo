package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/memory"
)

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

func TestGameMakerEnhancementsDispatchAndSchemas(t *testing.T) {
	tools := appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true})
	raw, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	for _, token := range []string{`"add_library"`, `"topic"`, `"sfx"`, `"probe"`, `"viewport"`, `"capture"`, `"touch_start"`, `"tap"`, `Optional template: canvas,`} {
		if !strings.Contains(encoded, token) {
			t.Fatalf("game maker schema missing %s", token)
		}
	}

	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{
		DBPath:        filepath.Join(root, "game.db"),
		WorkspacePath: filepath.Join(root, "workspace"),
		Enabled:       true,
		AllowCreate:   true,
		AllowEdit:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(service)
	defer gamemaker.SetDefaultService(previous)
	service.SetSkillStatus(nil, true)

	project, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: "SFX Test", Dimension: "2d", Description: "Canvas test"})
	if err != nil {
		t.Fatal(err)
	}
	var dispatchErr error
	done := make(chan struct{})
	service.SetRunner(gameMakerPlanTestRunner(func(ctx context.Context, run gamemaker.JobRun) error {
		defer close(done)
		bound := gamemaker.WithJobContext(ctx, run.Job.ID)
		dc := &DispatchContext{Cfg: &config.Config{}} // AllowMediaGeneration=false

		// 1. generate kind=sfx succeeds without external media permission.
		sfxOut, handled := dispatchGameMaker(bound, ToolCall{
			Action: "game_maker_asset", Operation: "generate",
			Params: map[string]any{"kind": "sfx", "prompt": "coin pickup chime", "path": "assets/sfx/coin.wav"},
		}, dc)
		if !handled || !strings.Contains(sfxOut, `"status":"ok"`) || !strings.Contains(sfxOut, `"generator":"procedural-sfx"`) {
			dispatchErr = context.Canceled
			t.Errorf("sfx generate failed: %s", sfxOut)
		}
		if _, err := service.ReadJobFile(ctx, run.Job.ID, "assets/sfx/coin.wav"); err != nil {
			t.Errorf("generated sfx wav missing: %v", err)
		}

		// 2. add_library installs vendor/zzfx.js without touching index.html.
		libOut, handled := dispatchGameMaker(bound, ToolCall{
			Action: "game_maker_project", Operation: "add_library",
			Params: map[string]any{"library": "zzfx"},
		}, dc)
		if !handled || !strings.Contains(libOut, `"status":"ok"`) || !strings.Contains(libOut, `vendor/zzfx.js`) {
			t.Errorf("add_library zzfx failed: %s", libOut)
		}

		// 3. inspect includes capabilities.
		inspOut, handled := dispatchGameMaker(bound, ToolCall{Action: "game_maker_project", Operation: "inspect"}, dc)
		if !handled || !strings.Contains(inspOut, `"sfx_generation_ready":true`) || !strings.Contains(inspOut, `"canvas"`) {
			t.Errorf("inspect missing capabilities: %s", inspOut)
		}

		// 4. reference topic returns craft snippet.
		refOut, handled := dispatchGameMaker(bound, ToolCall{
			Action: "game_maker_project", Operation: "reference",
			Params: map[string]any{"topic": "juice"},
		}, dc)
		if !handled || !strings.Contains(refOut, `addShake`) {
			t.Errorf("reference topic=juice missing snippet: %s", refOut)
		}

		if res := service.FinishJob(ctx, run.Job.ID); !res.OK {
			t.Errorf("finish failed: %+v", res.Diagnostics)
		}
		return dispatchErr
	}))
	job, err := service.StartJob(context.Background(), project.ID, gamemaker.StartJobRequest{Prompt: "test"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for runner")
	}
	for i := 0; i < 40; i++ {
		got, err := service.GetJob(context.Background(), job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == "ready" || got.Status == "failed" {
			if got.Status != "ready" {
				t.Fatalf("job ended with status=%s error=%s", got.Status, got.Error)
			}
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatal("job did not reach terminal status")
}

func TestLearningGameMakerKeepsProjectsIsolated(t *testing.T) {
	s, dc, _ := correctionAgentFixture(t)
	dc.Cfg.Learnings.Enabled = true
	const quote = "Phaser StaticBody has no setVelocity; keep moving paddles dynamic."
	draft := memory.LearningDraft{Statement: quote, Rationale: "keep moving paddles dynamic", Kind: "technical", Confidence: .99}
	scopeA := learningAnalysisScope{Scope: memory.LearningScope{Project: "game-maker:proj-a"}, HumanText: strPtr(quote)}
	applyExtractedLearnings(t.Context(), dc.Cfg, dc.ShortTermMem, "s1", quote, []extractedLearning{{
		LearningDraft: draft,
		Quote:         quote,
	}}, scopeA)

	retired := memory.LearningScope{Project: "game-maker:studio"}
	if entries, err := dc.ShortTermMem.ListLearnings(t.Context(), retired, "", "", 10); err != nil || len(entries) != 0 {
		t.Fatalf("automatic extraction copied project learning: %v, %v", entries, err)
	}
	source, err := learningSourceFromEvidence("s1", memory.CorrectionEvidence{Reference: "user:current", Kind: "user", Quote: quote}, quote)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dc.ShortTermMem.ObserveLearning(t.Context(), draft, retired, source, ""); err != nil {
		t.Fatal(err)
	}

	const query = "Phaser StaticBody setVelocity"
	s.runCfg.AllowProjectLearnings = true
	s.runCfg.LearningScope = scopeA.Scope
	if got := learningPromptContext(t.Context(), s.runCfg, query, "gpt-4o"); !strings.Contains(got, quote) {
		t.Fatalf("project A did not recall its own learning: %q", got)
	}
	s.runCfg.LearningScope = memory.LearningScope{Project: "game-maker:proj-b"}
	if got := learningPromptContext(t.Context(), s.runCfg, query, "gpt-4o"); got != "" {
		t.Fatalf("project B recalled another project's learning: %q", got)
	}
}

func strPtr(v string) *string { return &v }
