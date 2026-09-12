package gamemaker

import (
	"sort"
	"strings"
	"testing"
)

func generationFixture(dimension string) Scene {
	max := Vec3{20, 12, 0}
	if dimension == "3d" {
		max = Vec3{20, 10, 20}
	}
	return Scene{
		SchemaVersion: SceneSchemaVersion,
		Dimension:     dimension,
		Seed:          41,
		Navigation:    "custom",
		Levels:        []SceneLevel{{ID: "main", Active: true}},
		WorldBounds:   SceneBounds{Min: Vec3{0, 0, 0}, Max: max},
	}
}

func TestGenerateSceneGridUsesCellSizeAndThreeDGroundAxes(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		scene := generationFixture(dimension)
		generated, err := GenerateSceneRegion(scene, GenerateSceneRegionRequest{
			RegionID: "grid-region", Kind: "grid", Bounds: scene.WorldBounds,
			Density: 4, CellSize: 5, Seed: 99,
		})
		if err != nil {
			t.Fatalf("%s grid: %v", dimension, err)
		}
		if len(generated.Nodes) != 4 {
			t.Fatalf("%s grid nodes = %d, want 4", dimension, len(generated.Nodes))
		}
		var nonCenterDepth bool
		for _, node := range generated.Nodes {
			if node.Size[0] > 5.000001 || (dimension == "3d" && node.Size[2] > 5.000001) {
				t.Fatalf("%s grid ignored cell size: %#v", dimension, node)
			}
			if dimension == "3d" {
				if node.Position[1] != node.Size[1]/2 {
					t.Fatalf("3d grid left ground plane: %#v", node.Position)
				}
				if node.Position[2] != 10 {
					nonCenterDepth = true
				}
			}
		}
		if dimension == "3d" && !nonCenterDepth {
			t.Fatal("3d grid still varies in x/y instead of x/z")
		}
	}
}

func TestGenerateSceneRoomsDeclareFreeCorridorRoutes(t *testing.T) {
	scene := generationFixture("2d")
	generated, err := GenerateSceneRegion(scene, GenerateSceneRegionRequest{
		RegionID: "rooms", Kind: "rooms", Bounds: scene.WorldBounds,
		Density: 6, CellSize: 6, Seed: 12,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Routes) != len(generated.Nodes)-1 {
		t.Fatalf("room routes = %d, want %d", len(generated.Routes), len(generated.Nodes)-1)
	}
	for _, route := range generated.Routes {
		if len(route.Waypoints) < 3 || route.Kind != "undirected" {
			t.Fatalf("room route does not expose a corridor: %#v", route)
		}
		if !strings.HasPrefix(route.ID, "route_") {
			t.Fatalf("room route ID is not stable: %q", route.ID)
		}
	}
}

func TestGenerateScenePlatformsRespectMovementLimits(t *testing.T) {
	scene := generationFixture("2d")
	scene.Navigation = "platforms"
	scene.Metadata = map[string]any{
		"movement": map[string]any{"max_jump": 2.5, "max_jump_height": 1.0},
	}
	generated, err := GenerateSceneRegion(scene, GenerateSceneRegionRequest{
		RegionID: "platforms", Kind: "platforms", Bounds: scene.WorldBounds,
		Density: 6, Seed: 23,
	})
	if err != nil {
		t.Fatal(err)
	}
	nodes := append([]SceneNode(nil), generated.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Position[0] < nodes[j].Position[0] })
	for i := 1; i < len(nodes); i++ {
		left, right := nodes[i-1], nodes[i]
		gap := right.Position[0] - left.Position[0] - left.Size[0]/2 - right.Size[0]/2
		if gap > 2.500001 {
			t.Fatalf("platform gap = %g, want <= 2.5", gap)
		}
		if diff := right.Position[1] - left.Position[1]; diff > 1.000001 || diff < -1.000001 {
			t.Fatalf("platform height step = %g, want <= 1", diff)
		}
	}
}

func TestGenerateScenePlatformsReadMechanicsMovementParams(t *testing.T) {
	scene := generationFixture("2d")
	scene.Metadata = map[string]any{
		"mechanics": map[string]any{
			"blocks": []any{map[string]any{
				"kind": "movement", "params": `{"max_jump":1.5,"max_jump_height":0.75}`,
			}},
		},
	}
	generated, err := GenerateSceneRegion(scene, GenerateSceneRegionRequest{
		RegionID: "mechanic-platforms", Kind: "platforms", Bounds: scene.WorldBounds,
		Density: 5, Seed: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	nodes := append([]SceneNode(nil), generated.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Position[0] < nodes[j].Position[0] })
	for i := 1; i < len(nodes); i++ {
		left, right := nodes[i-1], nodes[i]
		gap := right.Position[0] - left.Position[0] - left.Size[0]/2 - right.Size[0]/2
		if gap > 1.500001 {
			t.Fatalf("mechanics platform gap = %g, want <= 1.5", gap)
		}
	}
}
