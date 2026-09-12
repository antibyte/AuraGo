package gamemaker

import (
	"strings"
	"testing"
)

func TestSceneValidationAcceptsOpenRasterAndConnectedWaypoints(t *testing.T) {
	scene := sceneFixture()
	if err := validateScene(scene, AssetCatalog{}); err != nil {
		t.Fatalf("open topdown scene rejected: %v", err)
	}
	scene.Navigation = "waypoints"
	scene.Routes = []SceneRoute{{ID: "route", From: "player", To: "goal-node", Kind: "undirected", Waypoints: []Vec3{{2, 10, 0}, {18, 10, 0}}}}
	if err := validateScene(scene, AssetCatalog{}); err != nil {
		t.Fatalf("connected waypoint scene rejected: %v", err)
	}
}

func TestSceneRasterReportsCoarseUnreachableAsWarning(t *testing.T) {
	scene := sceneFixture()
	scene.Metadata = map[string]any{"actor_radius": 3.0}
	scene.Nodes = append(scene.Nodes,
		SceneNode{ID: "lower-wall", Kind: "blocker", Position: Vec3{10, 4, 0}, Size: Vec3{2, 8, 0}},
		SceneNode{ID: "upper-wall", Kind: "blocker", Position: Vec3{10, 16, 0}, Size: Vec3{2, 8, 0}},
	)
	scene.Colliders = []SceneCollider{
		{ID: "lower-wall-collider", NodeID: "lower-wall", Shape: "box", Extents: Vec3{1, 4, 0}},
		{ID: "upper-wall-collider", NodeID: "upper-wall", Shape: "box", Extents: Vec3{1, 4, 0}},
	}
	diagnostics := ValidateScene(scene, AssetCatalog{})
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" && strings.Contains(diagnostic.Message, "unreachable") {
			t.Fatalf("coarse raster result was treated as proof: %#v", diagnostic)
		}
	}
	foundWarning := false
	for _, diagnostic := range diagnostics {
		foundWarning = foundWarning || diagnostic.Severity == "warning" && strings.Contains(diagnostic.Message, "unverified")
	}
	if !foundWarning {
		t.Fatalf("coarse raster uncertainty warning missing: %#v", diagnostics)
	}
}

func TestSceneRasterFiltersThreeDHeightAndSupport(t *testing.T) {
	scene := Scene{
		SchemaVersion: SceneSchemaVersion,
		Dimension:     "3d",
		Navigation:    "ground",
		Metadata:      map[string]any{"navigation_height": 0.0},
		Levels:        []SceneLevel{{ID: "main", Active: true}},
		WorldBounds:   SceneBounds{Min: Vec3{0, 0, 0}, Max: Vec3{20, 10, 20}},
		Nodes: []SceneNode{
			{ID: "player", Kind: "player", Position: Vec3{2, 0, 2}, Size: Vec3{1, 2, 1}},
			{ID: "goal-node", Kind: "goal", Position: Vec3{18, 0, 2}, Size: Vec3{1, 1, 1}},
			{ID: "floor", Kind: "floor", Position: Vec3{10, -0.1, 10}, Properties: map[string]any{"navigation": "floor"}},
			{ID: "ceiling", Kind: "ceiling", Position: Vec3{10, 8, 10}},
		},
		Colliders: []SceneCollider{
			{ID: "floor-collider", NodeID: "floor", Shape: "box", Extents: Vec3{10, 0.1, 10}},
			{ID: "ceiling-collider", NodeID: "ceiling", Shape: "box", Extents: Vec3{10, 0.1, 10}},
		},
		Zones: []SceneZone{
			{ID: "spawn", Kind: "spawn", Bounds: SceneBounds{Min: Vec3{1, 0, 1}, Max: Vec3{3, 0, 3}}},
			{ID: "goal", Kind: "goal", Bounds: SceneBounds{Min: Vec3{17, 0, 1}, Max: Vec3{19, 0, 3}}},
		},
	}
	for _, diagnostic := range ValidateScene(scene, AssetCatalog{}) {
		if diagnostic.Severity == "error" && strings.Contains(diagnostic.Message, "unreachable") {
			t.Fatalf("height-filtered 3d support was blocked: %#v", diagnostic)
		}
	}
}
