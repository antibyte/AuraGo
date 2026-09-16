package gamemaker

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func isometricFixture() Scene {
	return Scene{SchemaVersion: 2, Dimension: "2d", Projection: &SceneProjection{Kind: "isometric", TileWidth: 128, TileHeight: 64, HeightStep: 32}, Seed: 7, Navigation: "topdown", Levels: []SceneLevel{{ID: "main", Active: true}, {ID: "inside"}}, WorldBounds: SceneBounds{Min: Vec3{-1, -1, 0}, Max: Vec3{12, 12, 4}}, Nodes: []SceneNode{
		{ID: "low", Kind: "floor", Position: Vec3{0, 0, 0}, Properties: map[string]any{"walkable": true}},
		{ID: "high", Kind: "floor", Position: Vec3{1, 0, 1}, Properties: map[string]any{"walkable": true}},
		{ID: "player", Kind: "player", Position: Vec3{.5, .5, 0}},
		{ID: "goal", Kind: "goal", Position: Vec3{1.5, .5, 1}, Properties: map[string]any{"goal": true}},
	}, Routes: []SceneRoute{{ID: "stairs", From: "low", To: "high", Kind: "stairs"}}}
}

func TestIsometricGeneratedJoinsPreserveFixedNeighbours(t *testing.T) {
	scene := isometricFixture()
	scene.Nodes = nil
	scene.Routes = nil
	catalog := AssetCatalog{Assets: map[string]SceneAsset{}, Roles: map[string]SceneAsset{}}
	for _, role := range []string{"floor", "edge", "corner", "stairs"} {
		a := SceneAsset{ID: role, Roles: []string{role}}
		catalog.Assets[role] = a
		catalog.Roles[role] = a
	}
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			id := fmt.Sprintf("n-%d-%d", x, y)
			scene.Nodes = append(scene.Nodes, SceneNode{ID: id, Position: Vec3{float64(x), float64(y), 0}, RegionID: "garden", LevelID: "main", Properties: map[string]any{"walkable": true}})
			scene.Placements = append(scene.Placements, ScenePlacement{ID: "art-" + id, NodeID: id, AssetID: "floor", AssetRole: "floor"})
		}
	}
	scene.Nodes[0].Pinned = true
	scene.Nodes = append(scene.Nodes, SceneNode{ID: "fixed", Pinned: true, Position: Vec3{3, 1, 1}, RegionID: "other", LevelID: "main", Properties: map[string]any{"walkable": true}})
	request := GenerateSceneRegionRequest{Kind: "grid", RegionID: "garden", LevelID: "main", Assets: catalog, TileRoles: &IsometricTileRoles{Edge: "edge", OuterCorner: "corner", Stairs: "stairs"}, ConnectHeights: true}
	if err := generateIsometricJoins(&scene, request); err != nil {
		t.Fatal(err)
	}
	if scene.Placements[0].AssetRole != "floor" || !scene.Nodes[len(scene.Nodes)-1].Pinned {
		t.Fatal("fixed content changed")
	}
	if len(scene.Routes) != 1 || scene.Routes[0].To != "fixed" {
		t.Fatal(scene.Routes)
	}
	if scene.Placements[7].AssetRole != "stairs" || scene.Placements[8].AssetRole != "corner" {
		t.Fatal("tile transitions missing")
	}
	before, _ := json.Marshal(scene)
	if err := generateIsometricJoins(&scene, request); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(scene)
	if string(before) != string(after) {
		t.Fatal("non-idempotent joins")
	}
	request.TileRoles.Edge = "invented"
	if generateIsometricJoins(&scene, request) == nil {
		t.Fatal("unknown role accepted")
	}
}

func TestIsometricSceneElevationAndGeneration(t *testing.T) {
	scene := isometricFixture()
	if err := validateScene(scene, AssetCatalog{}); err != nil {
		t.Fatal(err)
	}
	broken := scene
	broken.Routes = nil
	if err := validateScene(broken, AssetCatalog{}); err == nil || !strings.Contains(err.Error(), "walkable path") {
		t.Fatalf("unreachable height: %v", err)
	}
	broken = scene
	broken.Nodes = append(append([]SceneNode{}, scene.Nodes...), SceneNode{ID: "overlap", Kind: "floor", Position: Vec3{0, 0, 1}, Properties: map[string]any{"walkable": true}})
	if err := validateScene(broken, AssetCatalog{}); err == nil || !strings.Contains(err.Error(), "overlapping") {
		t.Fatalf("overlapping floors: %v", err)
	}
	broken = scene
	broken.Routes = []SceneRoute{{ID: "stairs", From: "low", To: "goal", Kind: "stairs"}}
	broken.Nodes = append([]SceneNode{}, scene.Nodes...)
	broken.Nodes[3].Position = Vec3{8.5, .5, 1}
	if err := validateScene(broken, AssetCatalog{}); err == nil || !strings.Contains(err.Error(), "adjacent") {
		t.Fatalf("teleporting stairs: %v", err)
	}
	request := GenerateSceneRegionRequest{RegionID: "garden", Kind: "grid", Bounds: SceneBounds{Min: Vec3{4, 4, 2}, Max: Vec3{8, 8, 2}}, NodeKind: "floor", Density: 16, CellSize: 1, Seed: 88, Clear: true}
	first, err := GenerateSceneRegion(scene, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateSceneRegion(first, request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed changed scene")
	}
	count := 0
	for _, node := range first.Nodes {
		if node.RegionID == "garden" {
			count++
			if node.Position[2] != 2 || node.Properties["walkable"] != true {
				t.Fatal(node)
			}
		}
	}
	if count != 16 {
		t.Fatal(count)
	}
	first.Nodes[len(first.Nodes)-1].Pinned = true
	pinned := first.Nodes[len(first.Nodes)-1]
	request.Seed++
	next, err := GenerateSceneRegion(first, request)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range next.Nodes {
		if n.ID == pinned.ID {
			a, _ := json.Marshal(n)
			b, _ := json.Marshal(pinned)
			found = string(a) == string(b)
		}
	}
	if !found {
		t.Fatal("regeneration changed pinned floor")
	}
}

func TestIsometricWrongHeightsAndInactiveLevelErrors(t *testing.T) {
	for _, index := range []int{2, 3} {
		scene := isometricFixture()
		scene.Nodes[index].Position[2]++
		if err := validateScene(scene, AssetCatalog{}); err == nil {
			t.Fatalf("floating actor/goal %d accepted", index)
		}
	}
	scene := isometricFixture()
	scene.Zones = []SceneZone{{ID: "unreachable", Kind: "goal", LevelID: "main", Bounds: SceneBounds{Min: Vec3{7, 7, 0}, Max: Vec3{8, 8, 0}}}}
	if err := validateScene(scene, AssetCatalog{}); err == nil {
		t.Fatal("unreachable goal zone accepted")
	}
	scene = isometricFixture()
	scene.Navigation = "custom"
	scene.Nodes = append(scene.Nodes, SceneNode{ID: "bad-inside", LevelID: "inside", Position: Vec3{0, 0, 0}, Properties: map[string]any{"footprint": []float64{0, 1}}})
	if err := validateScene(scene, AssetCatalog{}); err == nil {
		t.Fatal("custom movement skipped inactive level validation")
	}
}

func TestScenePackQualifiedRoleCollision(t *testing.T) {
	catalog := AssetCatalog{Assets: map[string]SceneAsset{}, Roles: map[string]SceneAsset{
		"small": {ID: "ship", Roles: []string{"small"}, Sockets: []string{"oar"}, Bounds: SceneBounds{Min: Vec3{-1, 0, -2}, Max: Vec3{1, 1, 2}}},
		"large": {ID: "ship", Roles: []string{"large"}, Sockets: []string{"cannon"}, Bounds: SceneBounds{Min: Vec3{-4, 0, -12}, Max: Vec3{4, 8, 12}}},
	}}
	small, ok := sceneBoundAsset(catalog, ScenePlacement{AssetRole: "small", AssetID: "ship"})
	if !ok || small.Sockets[0] != "oar" {
		t.Fatal(small)
	}
	large, ok := sceneBoundAsset(catalog, ScenePlacement{AssetRole: "large", AssetID: "ship"})
	if !ok || large.Bounds.Max[2] != 12 {
		t.Fatal(large)
	}
	if _, ok = sceneBoundAsset(catalog, ScenePlacement{AssetRole: "large", AssetID: "wrong"}); ok {
		t.Fatal("wrong motif identity accepted")
	}
}
