package gamemaker

import "testing"

func TestBuilderPinsCoverDirectWritesAndDependencies(t *testing.T) {
	before := builderSceneFixture()
	before.Nodes[0].Pinned = true
	before.Placements = []ScenePlacement{{ID: "art", NodeID: "ball", AssetRole: "ball", Behavior: "collect"}}
	after, _ := cloneScene(before)
	after.Nodes[0].Position[0]++
	if protectPinnedScene(before, after) == nil {
		t.Fatal("moved pinned node")
	}
	after, _ = cloneScene(before)
	after.Placements[0].Position[0]++
	if protectPinnedScene(before, after) == nil {
		t.Fatal("moved visual of pinned node")
	}
	after, _ = cloneScene(before)
	after.Nodes[0].Pinned = false
	if err := protectPinnedScene(before, after); err != nil {
		t.Fatal("explicit unpin", err)
	}
	after.Nodes[0].Position[0]++
	if protectPinnedScene(before, after) == nil {
		t.Fatal("unpin also moved content")
	}
	before.Nodes[0].Pinned = false
	before.Nodes[0].RegionID = "landmark"
	before.Regions = []SceneRegion{{ID: "landmark", Pinned: true}}
	after, _ = cloneScene(before)
	after.Nodes = nil
	if protectPinnedScene(before, after) == nil {
		t.Fatal("removed node in pinned region")
	}
}
