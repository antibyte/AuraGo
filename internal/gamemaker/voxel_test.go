package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/dbutil"
)

func voxelFixture(t *testing.T, s *Service) (Project, string, *VoxelDefinition) {
	t.Helper()
	p, err := s.CreateProject(context.Background(), CreateProjectRequest{Name: "Voxel test", Dimension: "3d", Variant: "voxel", Description: "Mine and build"})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.opts.WorkspacePath, p.ProjectKey)
	if err := WriteScaffold(dir, p); err != nil {
		t.Fatal(err)
	}
	plan := ExampleGamePlan(p)
	if err := s.checkPlan(p, plan); err != nil {
		t.Fatal(err)
	}
	if err := installGameTemplate(dir, plan); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(dir, ".aurago"), 0750)
	data, _ := json.Marshal(plan)
	os.WriteFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)), data, 0600)
	if build := buildDirectory(context.Background(), dir, 100, 32<<20); !build.OK {
		t.Fatal(build.Diagnostics)
	}
	return p, dir, plan.Voxel
}

func TestVoxelDefinitionAndBuild(t *testing.T) {
	s := newTestService(t)
	p, dir, v := voxelFixture(t, s)
	if err := v.Validate(); err != nil {
		t.Fatal(err)
	}
	original := v.Compatibility()
	v.Blocks[0].Name = "Localized name"
	if v.Compatibility() != original {
		t.Fatal("label changed save identity")
	}
	v.Seed += "changed"
	if v.Compatibility() == original {
		t.Fatal("seed reused incompatible world")
	}
	for _, raw := range []string{`{"version":1,"unknown":1}`, `null`, strings.Repeat(" ", 65537)} {
		if _, err := ParseVoxelDefinition([]byte(raw)); err == nil {
			t.Fatal("accepted invalid definition")
		}
	}
	if _, err := s.CreateProject(context.Background(), CreateProjectRequest{Name: "Invalid", Description: "x", Dimension: "2d", Variant: "voxel"}); err == nil {
		t.Fatal("accepted 2D voxel project")
	}
	data, err := os.ReadFile(filepath.Join(dir, "src", "voxel.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBuilderSource(dir, "src/voxel.json", strings.Replace(string(data), `"survival"`, `"auto"`, 1)); err == nil {
		t.Fatal("source bypassed definition validation")
	}
	stored, err := s.GetProject(context.Background(), p.ID)
	if err != nil || stored.Variant != "voxel" {
		t.Fatal(stored, err)
	}
	var malformed VoxelDefinition
	if json.Unmarshal([]byte(strings.Replace(string(data), `"size": [`, `"size": [1,`, 1)), &malformed) == nil {
		t.Fatal("plan decoding accepts too many dimensions")
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.ts"), []byte(`console.log("voxel name only");`), 0600); err != nil {
		t.Fatal(err)
	}
	if buildDirectory(context.Background(), dir, 100, 32<<20).OK {
		t.Fatal("disconnected voxel definition accepted as implementation")
	}
	if node, err := exec.LookPath("node"); err == nil {
		definition := filepath.Join(t.TempDir(), "voxel.json")
		data, _ = json.Marshal(DefaultVoxelDefinition())
		os.WriteFile(definition, data, 0600)
		cmd := exec.Command(node, filepath.Join("..", "..", "scripts", "test-game-maker-voxel.mjs"), definition)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("voxel runtime: %v\n%s", err, out)
		}
	} else {
		t.Fatal("node is required for voxel runtime checks")
	}
}

func TestVoxelEvidenceRejectsCounterOnlySuccess(t *testing.T) {
	v := DefaultVoxelDefinition()
	plan := GamePlan{Template: "voxel", Voxel: v}
	scenarios := voxelScenarios(&plan)
	var observations []GameObservation
	for _, s := range scenarios {
		a := &VoxelEvidence{Player: VoxelPlayerState{Position: [3]float64{48, 20, 48}, Health: 100}, Inventory: make([]*VoxelSlot, 36)}
		b := *a
		observations = append(observations, GameObservation{ID: s.ID, Before: map[string]float64{s.Metric: 0}, After: map[string]float64{s.Metric: 999}, VoxelBefore: a, VoxelAfter: &b})
	}
	for _, c := range compareVoxelObservations(&plan, scenarios, observations) {
		if c.Status == "passed" {
			t.Fatal("counter-only check passed", c.ID)
		}
	}
	o := &observations[2]
	o.VoxelBefore.Blocks = []VoxelBlockEvidence{{Cell: [3]int{48, 20, 44}, ID: 4}}
	o.VoxelAfter.Blocks = []VoxelBlockEvidence{{Cell: [3]int{48, 20, 44}, ID: 0}}
	if compareVoxelObservations(&plan, scenarios, observations)[2].Status == "passed" {
		t.Fatal("mining without inventory consequence passed")
	}
	o.VoxelAfter.Inventory = make([]*VoxelSlot, 36)
	o.VoxelAfter.Inventory[0] = &VoxelSlot{Item: "wood", Count: 1}
	if compareVoxelObservations(&plan, scenarios, observations)[2].Status != "passed" {
		t.Fatal("real block and inventory delta rejected")
	}
	v.Recipes = append(v.Recipes, VoxelRecipe{ID: "noop", Item: "wood", Count: 1, Ingredients: map[string]int{"wood": 1}})
	if compareVoxelObservations(&plan, scenarios, observations)[3].Status == "passed" {
		t.Fatal("no-op recipe passed without any inventory consequence")
	}
}

func TestVoxelSaveIsolationConflictAndRestart(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	p, dir, v := voxelFixture(t, s)
	publishExportFixture(t, s, p, dir)
	grant, err := s.CreatePreviewGrant(p.ID)
	if err != nil || grant.PlayToken == "" {
		t.Fatal(grant, err)
	}
	state := VoxelState{Format: 1, Chunks: []VoxelChunkState{}, Player: VoxelPlayerState{Position: [3]float64{48, 30, 48}, Health: 100}, Inventory: make([]*VoxelSlot, 36), Progress: map[string]int{}, Enemies: []VoxelEnemyState{}}
	raw, _ := json.Marshal(state)
	for _, field := range []string{"chunks", "enemies", "progress"} {
		var bad map[string]any
		json.Unmarshal(raw, &bad)
		bad[field] = nil
		encoded, _ := json.Marshal(bad)
		if validateVoxelState(v, encoded) == nil {
			t.Fatal("null save field accepted", field)
		}
	}
	if _, err := s.WritePlayState(ctx, p.ID, grant.Token, 0, raw, false); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("preview token may write", err)
	}
	if _, err := s.WritePlayState(ctx, "foreign", grant.PlayToken, 0, raw, false); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("foreign project may write", err)
	}
	written, err := s.WritePlayState(ctx, p.ID, grant.PlayToken, 0, raw, false)
	if err != nil || written.Version != 1 {
		t.Fatal(written, err)
	}
	if _, err := s.WritePlayState(ctx, p.ID, grant.PlayToken, 0, raw, false); !errors.Is(err, ErrPlayStateConflict) {
		t.Fatal("stale write allowed", err)
	}
	if _, err := s.WritePlayState(ctx, p.ID, grant.PlayToken, 1, make([]byte, MaxPlayStateBytes+1), false); err == nil {
		t.Fatal("oversize write allowed")
	}
	bad := state
	bad.Chunks = []VoxelChunkState{{Position: [3]int{0, 0, 0}, Runs: []int{65, 4096}}}
	invalid, _ := json.Marshal(bad)
	if err := validateVoxelState(v, invalid); err == nil {
		t.Fatal("unknown block accepted")
	}
	bad.Chunks[0].Runs = []int{0, 4095}
	invalid, _ = json.Marshal(bad)
	if err := validateVoxelState(v, invalid); err == nil {
		t.Fatal("short chunk accepted")
	}
	policy := s.policy
	locked := policy
	locked.ReadOnly = true
	s.UpdatePolicy(locked)
	if _, err := s.WritePlayState(ctx, p.ID, grant.PlayToken, 1, raw, false); !errors.Is(err, ErrReadOnly) {
		t.Fatal("readonly save accepted", err)
	}
	s.UpdatePolicy(policy)
	seed := v.Seed
	for _, value := range []string{seed + "-incompatible", seed} {
		v.Seed = value
		definition, _ := json.Marshal(v)
		os.WriteFile(filepath.Join(dir, "src", "voxel.json"), definition, 0600)
		current, err := s.GetProject(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		publishExportFixture(t, s, current, dir)
		newGrant, err := s.CreatePreviewGrant(p.ID)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := s.GetPlayState(ctx, p.ID, newGrant.PlayToken)
		if err != nil {
			t.Fatal(err)
		}
		if value != seed {
			if loaded.Version != 0 || len(loaded.State) != 0 {
				t.Fatal("incompatible world reused saved state")
			}
			if _, err := s.WritePlayState(ctx, p.ID, newGrant.PlayToken, 0, raw, false); err != nil {
				t.Fatal(err)
			}
		} else if loaded.Version != 1 || string(loaded.State) != string(raw) {
			t.Fatal("old compatible save was not preserved")
		}
	}
	if _, err := s.GetPlayState(ctx, p.ID, grant.PlayToken); !errors.Is(err, ErrPlayStateConflict) {
		t.Fatal("old published binding accepted", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewService(s.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	newGrant, err := reopened.CreatePreviewGrant(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.GetPlayState(ctx, p.ID, newGrant.PlayToken)
	if err != nil || loaded.Version != 1 || string(loaded.State) != string(raw) {
		t.Fatal(loaded, err)
	}
	if _, err := reopened.WritePlayState(ctx, p.ID, newGrant.PlayToken, 1, nil, true); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.WritePlayState(ctx, p.ID, newGrant.PlayToken, 1, raw, false); !errors.Is(err, ErrPlayStateConflict) {
		t.Fatal("reset lost its version tombstone", err)
	}
	loaded, err = reopened.GetPlayState(ctx, p.ID, newGrant.PlayToken)
	if err != nil || loaded.Version != 2 || len(loaded.State) != 0 {
		t.Fatal(loaded, err)
	}
	reopened.mu.Lock()
	reopened.tokens["validation-save"] = previewToken{Purpose: "play-state", Revision: 1, ProjectID: p.ID, JobID: "validation", ExpiresAt: time.Now().Add(time.Hour)}
	reopened.mu.Unlock()
	if _, err := reopened.GetPlayState(ctx, p.ID, "validation-save"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("validation save admitted", err)
	}
}

func TestVoxelMigrationBacksUpLegacyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := dbutil.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE gm_projects(id TEXT PRIMARY KEY, name TEXT); INSERT INTO gm_projects VALUES('old','Existing game')`)
	if err != nil {
		t.Fatal(err)
	}
	// Work against a staged database, not the only copy of historical data.
	stage := filepath.Join(t.TempDir(), "staged.db")
	if _, err = db.Exec(`VACUUM INTO '` + strings.ReplaceAll(stage, "'", "''") + `'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = dbutil.Open(stage)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrateVoxelVariant(db); err != nil {
		t.Fatal(err)
	}
	var variant string
	if err := db.QueryRow(`SELECT variant FROM gm_projects WHERE id='old'`).Scan(&variant); err != nil || variant != "" {
		t.Fatal(variant, err)
	}
	backups, _ := filepath.Glob(stage + ".before-voxel-*.bak")
	if len(backups) != 1 {
		t.Fatal("missing backup", backups)
	}
	if err := migrateVoxelVariant(db); err != nil {
		t.Fatal(err)
	}
	original, err := dbutil.Open(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	var name string
	if err := original.QueryRow(`SELECT name FROM gm_projects WHERE id='old'`).Scan(&name); err != nil || name != "Existing game" {
		t.Fatal(name, err)
	}
}

// The voxel driver executes key, wait and observe only; a target or pointer
// step would silently do nothing, so planning must reject it with the reason.
func TestVoxelPlanRejectsStepsItsDriverCannotRun(t *testing.T) {
	s := newTestService(t)
	project := Project{Dimension: "3d", Variant: "voxel"}
	plan := ExampleGamePlan(project)
	plan.Scenarios = []GameScenario{{ID: "walk", Metric: "player_distance", Compare: "increased", Steps: []GameTestStep{{Action: "key", Key: "W", MS: 800}, {Action: "observe"}}}}
	if err := s.checkPlan(project, plan); err != nil {
		t.Fatalf("key/observe voxel scenario rejected: %v", err)
	}
	for _, step := range []GameTestStep{{Action: "target", Target: "player", Mode: "reach", MS: 2000}, {Action: "pointer", X: 10, Y: 10}} {
		plan.Scenarios[0].Steps = []GameTestStep{step, {Action: "observe"}}
		err := s.checkPlan(project, plan)
		if err == nil || !strings.Contains(err.Error(), "key, wait and observe") {
			t.Fatalf("%s step: err = %v", step.Action, err)
		}
	}
}

// Each rejected voxel definition names the element, field and accepted values;
// "invalid goal" alone left a real planning job three corrections to guess.
func TestVoxelDefinitionErrorsNameElementFieldAndAllowedValues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(v *VoxelDefinition)
		want   []string
	}{
		{"size axis", func(v *VoxelDefinition) { v.Size[1] = 70 }, []string{"size[1]", "70", "16", "64"}},
		{"block material", func(v *VoxelDefinition) { v.Blocks[8].Material = "castle" }, []string{"blocks[8]", `material "castle"`, "planks", "brick"}},
		{"block color", func(v *VoxelDefinition) { v.Blocks[2].Color = "grey" }, []string{"blocks[2]", `color "grey"`, "#RRGGBB"}},
		{"block hardness", func(v *VoxelDefinition) { v.Blocks[0].Hardness = 9 }, []string{"blocks[0]", "hardness", "0.1–4"}},
		{"block key", func(v *VoxelDefinition) { v.Blocks[1].Key = "Dirt Block" }, []string{"blocks[1]", `key "Dirt Block"`, "lowercase"}},
		{"duplicate block id", func(v *VoxelDefinition) { v.Blocks[1].ID = 1 }, []string{"blocks[1]", "duplicate id 1"}},
		{"item stack", func(v *VoxelDefinition) { v.Items[0].Stack = 0 }, []string{"items[0]", `"dirt"`, "stack 0", "1–999"}},
		{"item block", func(v *VoxelDefinition) { v.Items[0].Block = 40 }, []string{"items[0]", "block 40"}},
		{"recipe ingredient", func(v *VoxelDefinition) { v.Recipes[0].Ingredients = map[string]int{"unobtainium": 1} }, []string{"recipes[0]", `ingredient "unobtainium"`}},
		{"recipe output", func(v *VoxelDefinition) { v.Recipes[1].Item = "sword" }, []string{"recipes[1]", `item "sword"`}},
		{"enemy behavior", func(v *VoxelDefinition) { v.Enemies[0].Behavior = "flying" }, []string{"enemies[0]", `behavior "flying"`, "melee", "ranged"}},
		{"goal kind", func(v *VoxelDefinition) { v.Goals = []VoxelGoal{{ID: "castle", Kind: "build", Item: "brick", Count: 10}} }, []string{"goals[0]", `kind "build"`, "collect, craft, place, defeat"}},
		{"goal item", func(v *VoxelDefinition) { v.Goals = []VoxelGoal{{ID: "castle", Kind: "place", Item: "tower", Count: 10}} }, []string{"goals[0]", `item "tower"`}},
		{"defeat goal item", func(v *VoxelDefinition) { v.Goals = []VoxelGoal{{ID: "hunt", Kind: "defeat", Item: "wood", Count: 2}} }, []string{"goals[0]", "defeat", "omit item"}},
		{"unreachable defeat", func(v *VoxelDefinition) { v.Goals = []VoxelGoal{{ID: "hunt", Kind: "defeat", Count: 99}} }, []string{"goals[0]", "99", "6 enemies"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := DefaultVoxelDefinition()
			tc.mutate(v)
			err := v.Validate()
			if err == nil {
				t.Fatal("invalid definition accepted")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q lacks %q", err, want)
				}
			}
		})
	}
	if err := DefaultVoxelDefinition().Validate(); err != nil {
		t.Fatalf("default definition rejected: %v", err)
	}
}
