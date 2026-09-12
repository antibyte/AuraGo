package gamemaker

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

func sceneFixture() Scene {
	return Scene{
		SchemaVersion: SceneSchemaVersion,
		Dimension:     "2d",
		Seed:          77,
		Navigation:    "topdown",
		Levels:        []SceneLevel{{ID: "main", Name: "Main", Active: true}},
		WorldBounds:   SceneBounds{Min: Vec3{0, 0, 0}, Max: Vec3{20, 20, 0}},
		Nodes: []SceneNode{
			{ID: "player", Kind: "player", Position: Vec3{2, 10, 0}, Size: Vec3{1, 1, 0}},
			{ID: "goal-node", Kind: "goal", Position: Vec3{18, 10, 0}, Size: Vec3{1, 1, 0}},
		},
		Zones: []SceneZone{
			{ID: "spawn", Kind: "spawn", Bounds: SceneBounds{Min: Vec3{1, 9, 0}, Max: Vec3{3, 11, 0}}},
			{ID: "goal", Kind: "goal", Bounds: SceneBounds{Min: Vec3{17, 9, 0}, Max: Vec3{19, 11, 0}}},
		},
	}
}

func TestGenerateSceneRegionIsDeterministicAndPreservesLocalContent(t *testing.T) {
	scene := sceneFixture()
	scene.Nodes = append(scene.Nodes,
		SceneNode{ID: "keep", Kind: "decorative", Position: Vec3{19, 19, 0}},
		SceneNode{ID: "pinned", Kind: "blocker", Position: Vec3{10, 10, 0}, Pinned: true, RegionID: "arena"},
	)
	scene.Regions = []SceneRegion{{ID: "arena", Kind: "old", Bounds: SceneBounds{Min: Vec3{4, 4, 0}, Max: Vec3{16, 16, 0}}}}
	req := GenerateSceneRegionRequest{RegionID: "arena", Kind: "scatter", Bounds: scene.Regions[0].Bounds, Seed: 123, Density: 12, Clear: true}
	one, err := GenerateSceneRegion(scene, req)
	if err != nil {
		t.Fatal(err)
	}
	two, err := GenerateSceneRegion(scene, req)
	if err != nil {
		t.Fatal(err)
	}
	oneData, _ := MarshalSceneJSON(one)
	twoData, _ := MarshalSceneJSON(two)
	if !bytes.Equal(oneData, twoData) {
		t.Fatal("same scene seed and region did not produce identical canonical JSON")
	}
	for _, id := range []string{"keep", "pinned", "player", "goal-node"} {
		found := false
		for _, node := range one.Nodes {
			found = found || node.ID == id
		}
		if !found {
			t.Fatalf("regeneration removed unrelated or pinned node %q", id)
		}
	}
}

func TestSceneValidationRejectsWorldExtentAndKnownImpossibleLoss(t *testing.T) {
	scene := sceneFixture()
	scene.Nodes[0].Position[0] = 30
	if err := validateScene(scene, AssetCatalog{}); err == nil {
		t.Fatal("out-of-world gameplay node accepted")
	}
	scene = sceneFixture()
	scene.Zones = append(scene.Zones, SceneZone{ID: "loss", Kind: "loss", Bounds: scene.Zones[0].Bounds})
	if err := validateScene(scene, AssetCatalog{}); err == nil || !errors.Is(err, ErrSceneInvalid) {
		t.Fatalf("spawn/loss contradiction was not rejected: %v", err)
	}
}

func TestSceneValidationRejectsRasterUnreachableAndBallisticGap(t *testing.T) {
	scene := sceneFixture()
	scene.Nodes = append(scene.Nodes, SceneNode{ID: "wall", Kind: "blocker", Position: Vec3{10, 10, 0}, Size: Vec3{2, 20, 0}})
	scene.Colliders = []SceneCollider{{ID: "wall-collider", NodeID: "wall", Shape: "box", Extents: Vec3{1, 10, 0}}}
	if err := validateScene(scene, AssetCatalog{}); err == nil {
		t.Fatal("raster-unreachable scene accepted")
	}
	platforms := sceneFixture()
	platforms.Navigation = "platforms"
	platforms.Metadata = map[string]any{"movement": map[string]any{"max_jump": 4, "max_jump_height": 3}}
	platforms.Nodes = []SceneNode{
		{ID: "p1", Kind: "platform", Position: Vec3{2, 5, 0}, Size: Vec3{1, 1, 0}},
		{ID: "p2", Kind: "platform", Position: Vec3{15, 12, 0}, Size: Vec3{1, 1, 0}},
	}
	if err := validateScene(platforms, AssetCatalog{}); err == nil {
		t.Fatal("ballistically impossible platform gap accepted")
	}
}

func TestSceneHashConditionalWriteHelper(t *testing.T) {
	data := []byte(`{"schema_version":1}`)
	if err := checkExpectedSceneHash("", nil, false); err != nil {
		t.Fatal(err)
	}
	if err := checkExpectedSceneHash("", data, true); !errors.Is(err, ErrSceneConflict) {
		t.Fatalf("missing expected hash was accepted: %v", err)
	}
	hash := sceneHash(data)
	if err := checkExpectedSceneHash(hash, data, true); err != nil {
		t.Fatal(err)
	}
	if err := checkExpectedSceneHash("deadbeef", data, true); !errors.Is(err, ErrSceneConflict) {
		t.Fatalf("stale hash was accepted: %v", err)
	}
}

func TestSceneCatalogRoleAndSocketValidation(t *testing.T) {
	scene := sceneFixture()
	scene.Placements = []ScenePlacement{{ID: "player-placement", NodeID: "player", AssetID: "ship", AssetRole: "enemy", Behavior: "player", Position: Vec3{2, 10, 0}}}
	scene.Attachments = []SceneAttachment{{ID: "pilot", NodeID: "player", AssetID: "ship", Socket: "missing"}}
	catalog := AssetCatalog{Assets: map[string]SceneAsset{"ship": {ID: "ship", Roles: []string{"player"}, Sockets: []string{"pilot"}}}}
	if err := validateScene(scene, catalog); err == nil {
		t.Fatal("unaccepted role or socket was accepted")
	}
}

func TestGenerateSceneRegionResolvesRoleAndCatalogBounds(t *testing.T) {
	scene := sceneFixture()
	catalog := AssetCatalog{Assets: map[string]SceneAsset{
		"hero": {
			ID:      "hero",
			Roles:   []string{"enemy"},
			Bounds:  SceneBounds{Min: Vec3{-2, -1, -3}, Max: Vec3{4, 2, 1}},
			Sockets: []string{"weapon"},
		},
	}}
	generated, err := GenerateSceneRegion(scene, GenerateSceneRegionRequest{
		RegionID:    "arena",
		Kind:        "scatter",
		MinDistance: .25,
		Bounds:      SceneBounds{Min: Vec3{4, 4, 0}, Max: Vec3{16, 16, 0}},
		Seed:        11,
		Density:     2,
		AssetRole:   "enemy",
		Assets:      catalog,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Placements) != 2 {
		t.Fatalf("placements = %d, want 2", len(generated.Placements))
	}
	for _, placement := range generated.Placements {
		if placement.AssetID != "hero" || placement.AssetRole != "enemy" {
			t.Fatalf("generator binding = %#v", placement)
		}
	}
}

func TestSceneValidationEnforcesSharedCollectionLimit(t *testing.T) {
	scene := sceneFixture()
	scene.Nodes = make([]SceneNode, SceneMaxItems+1)
	for i := range scene.Nodes {
		scene.Nodes[i] = SceneNode{ID: fmt.Sprintf("node_%d", i), Kind: "decorative", Position: Vec3{1, 1, 0}}
	}
	diagnostics := ValidateScene(scene, AssetCatalog{})
	found := false
	for _, diagnostic := range diagnostics {
		found = found || diagnostic.Severity == "error" && diagnostic.Path == "nodes"
	}
	if !found {
		t.Fatalf("node collection limit diagnostic missing: %#v", diagnostics)
	}
}
