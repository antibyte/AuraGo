package gamemaker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func builderSceneFixture() Scene {
	return Scene{SchemaVersion: 1, Dimension: "2d", Seed: 1729, Levels: []SceneLevel{{ID: "main", Active: true}}, WorldBounds: SceneBounds{Min: Vec3{0, 0, 0}, Max: Vec3{960, 540, 0}}, Nodes: []SceneNode{{ID: "ball", Kind: "actor", Position: Vec3{480, 300, 0}, Size: Vec3{16, 16, 0}}}}
}

func TestBuilderSourceChecksActualPlanBindings(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".aurago"), 0700); err != nil {
		t.Fatal(err)
	}
	plan := GamePlan{Assets: []PlanAsset{{Role: "ball", AssetID: "ball_01", PackID: "arcade"}}}
	raw, _ := json.Marshal(plan)
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)), raw, 0600); err != nil {
		t.Fatal(err)
	}
	scene := builderSceneFixture()
	scene.Placements = []ScenePlacement{{ID: "ball-art", NodeID: "ball", AssetRole: "ball", AssetID: "wrong", Behavior: "actor", Position: Vec3{480, 300, 0}}}
	raw, _ = json.Marshal(scene)
	if validateBuilderSource(dir, SceneFilePath, string(raw)) == nil {
		t.Fatal("wrong accepted asset was written")
	}
	scene.Placements[0].AssetID = "" // The accepted role resolves the exact ID.
	raw, _ = json.Marshal(scene)
	if err := validateBuilderSource(dir, SceneFilePath, string(raw)); err != nil {
		t.Fatal(err)
	}
	if err := validateBuilderSource(dir, "src/main.ts", "custom code"); err != nil {
		t.Fatal("custom source was constrained", err)
	}
}

func TestBuilderSceneMustBeConnectedAndCannotDisableAcceptedScene(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"src", ".aurago"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	scene := builderSceneFixture()
	raw, _ := json.Marshal(scene)
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(SceneFilePath)), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if checkBuilderBuildGraph(dir, `{"inputs":{},"outputs":{}}`) == nil {
		t.Fatal("disconnected scene passed")
	}
	if err := checkBuilderBuildGraph(dir, `{"inputs":{"src/scene.json":{}},"outputs":{"dist/game.js":{"imports":[{"path":"../vendor/scene-builder.js"}]}}}`); err != nil {
		t.Fatal(err)
	}
	if err := validateBuilderSource(dir, SceneFilePath, "null"); err != nil {
		t.Fatal("legacy disabled scene", err)
	}
	raw, _ = json.Marshal(GamePlan{Scene: &scene})
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(gamePlanPath)), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if validateBuilderSource(dir, SceneFilePath, "null") == nil {
		t.Fatal("accepted scene silently disabled")
	}
}
