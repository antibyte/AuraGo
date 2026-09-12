package gamemaker

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSceneBoundsMatchThreeXYZAndRealCatalog(t *testing.T) {
	rotated := rotateSceneVector(Vec3{1, 0, 0}, Vec3{math.Pi / 2, math.Pi / 2, 0})
	if math.Abs(rotated[0])+math.Abs(rotated[1]-1)+math.Abs(rotated[2]) > 1e-8 {
		t.Fatal("XYZ order", rotated)
	}
	catalog, err := sceneCatalogForPlan(GamePlan{Assets: []PlanAsset{{Role: "car", PackID: ModelPackID, AssetID: "road-sedan", Scale: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	asset := catalog.Assets["road-sedan"]
	p := ScenePlacement{AssetID: asset.ID, AssetRole: "car", Scale: Vec3{1, 1, 1}, Rotation: Vec3{0, math.Pi / 2, 0}}
	box, ok := scenePlacementBounds(p, asset, "3d")
	if !ok || math.Abs((box.Max[0]-box.Min[0])-(asset.Bounds.Max[2]-asset.Bounds.Min[2])) > 1e-7 {
		t.Fatal("catalog scale or rotation", box, asset.Bounds)
	}
	asset.Bounds = SceneBounds{Min: Vec3{-1, -2, -3}, Max: Vec3{4, 2, 3}}
	p.Rotation, p.Scale = Vec3{}, Vec3{-2, 1, 1}
	box, _ = scenePlacementBounds(p, asset, "2d")
	if box.Min != (Vec3{-8, -2, 0}) || box.Max != (Vec3{2, 2, 0}) {
		t.Fatal("reflected 2D projection", box)
	}
}

func TestSceneMovementEstimatesUseCurrentMechanics(t *testing.T) {
	scene := generationFixture("2d")
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "mechanics.json"), []byte(`{"blocks":[{"id":"walk","kind":"movement","target":"player","params":"{\"mode\":\"platformer\",\"speed\":100,\"jump_speed\":200,\"gravity\":400}"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loadSceneMechanics(dir, &scene); err != nil {
		t.Fatal(err)
	}
	gap, height := scenePlatformLimits(scene)
	if math.Abs(gap-85) > 1e-8 || math.Abs(height-42.5) > 1e-8 {
		t.Fatal("wrong current ballistic limits", gap, height)
	}
}

func TestSceneRegenerationKeepsPinsAndIsIdempotent(t *testing.T) {
	scene := generationFixture("2d")
	req := GenerateSceneRegionRequest{RegionID: "steps", Kind: "platforms", Bounds: scene.WorldBounds, Density: 4, Seed: 17}
	one, err := GenerateSceneRegion(scene, req)
	if err != nil {
		t.Fatal(err)
	}
	two, err := GenerateSceneRegion(one, req)
	if err != nil || !reflect.DeepEqual(one, two) {
		t.Fatal("repeat generation changed canonical scene", err)
	}
	one.Nodes[0].Pinned = true
	pinnedID := one.Nodes[0].ID
	one.Placements = []ScenePlacement{{ID: "pin-art", NodeID: pinnedID}, {ID: "removed-art", NodeID: one.Nodes[1].ID}, {ID: "other-art", NodeID: "outside", RegionID: "steps"}}
	one.Nodes = append(one.Nodes, SceneNode{ID: "outside", Kind: "decorative"})
	one.Routes = append(one.Routes, SceneRoute{ID: "external-link", From: pinnedID, To: "outside", RegionID: "elsewhere"})
	removeSceneRegion(&one, "steps")
	if len(one.Nodes) != 2 || len(one.Placements) != 2 || len(one.Colliders) != 1 || len(one.Routes) != 1 || one.Routes[0].ID != "external-link" {
		t.Fatalf("lost pinned or unrelated dependencies: %+v", one)
	}
}

func TestSceneNestedJumpLimitRejectsImpossibleGap(t *testing.T) {
	scene := generationFixture("2d")
	scene.Navigation = "platforms"
	scene.Metadata = map[string]any{"movement": map[string]any{"max_jump": .5, "max_jump_height": 1}}
	scene.Zones = []SceneZone{{ID: "spawn", Kind: "spawn", Bounds: SceneBounds{Min: Vec3{1, 4, 0}, Max: Vec3{3, 5, 0}}}, {ID: "goal", Kind: "goal", Bounds: SceneBounds{Min: Vec3{4, 4, 0}, Max: Vec3{6, 5, 0}}}}
	scene.Nodes = []SceneNode{{ID: "a", Kind: "platform", Position: Vec3{2, 6, 0}, Size: Vec3{1, 1, 0}}, {ID: "b", Kind: "platform", Position: Vec3{5, 6, 0}, Size: Vec3{1, 1, 0}}}
	if err := validateScene(scene, AssetCatalog{}); err == nil || !strings.Contains(err.Error(), "cannot reach") {
		t.Fatal("nested jump limit ignored", err)
	}
}

func TestSceneRoomsConnectExistingEndpointsAndRespectWalls(t *testing.T) {
	scene := sceneFixture()
	scene.Navigation = "waypoints"
	req := GenerateSceneRegionRequest{RegionID: "rooms", Kind: "rooms", Bounds: SceneBounds{Min: Vec3{4, 4, 0}, Max: Vec3{16, 16, 0}}, Density: 4, Seed: 19}
	out, err := GenerateSceneRegion(scene, req)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateScene(out, AssetCatalog{}); err != nil {
		t.Fatal("room endpoints disconnected", err)
	}
	out.Nodes = append(out.Nodes, SceneNode{ID: "wall", Kind: "blocker", Position: Vec3{3.5, 10, 0}, Size: Vec3{.5, 20, 0}})
	out.Colliders = append(out.Colliders, SceneCollider{ID: "wall-body", NodeID: "wall", Shape: "box", Extents: Vec3{.25, 10, 0}})
	if sceneRouteClear(out, sceneNodeMap(out), []Vec3{{2, 10, 0}, {6, 10, 0}}, "player") {
		t.Fatal("connector crossed solid wall")
	}
}
