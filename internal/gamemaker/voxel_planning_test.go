package gamemaker

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestVoxelDefinitionReportsIndependentCorrectionsTogether(t *testing.T) {
	for _, mode := range []string{"creative", "survival"} {
		t.Run(mode, func(t *testing.T) {
			v := DefaultVoxelDefinition()
			v.Mode = mode
			v.Size[1] = 70
			v.Blocks = slices.DeleteFunc(v.Blocks, func(b VoxelBlock) bool { return b.Material == "ore" || b.Material == "sand" })
			v.Recipes[0].Count = 1000
			v.Goals = []VoxelGoal{{ID: "castle", Kind: "build", Item: "brick", Count: 20}}
			err := v.Validate()
			if err == nil {
				t.Fatal("invalid definition accepted")
			}
			for _, want := range []string{"size[1]", "ore, sand", "recipes[0]", "goals[0]", "collect, craft, place, defeat", "creative", "grass, dirt, stone, wood, leaves, ore, sand"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("correction lacks %q: %v", want, err)
				}
			}
			data, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseVoxelDefinition(data); err == nil || !strings.Contains(err.Error(), "ore, sand") {
				t.Fatalf("file validation lost the complete palette correction: %v", err)
			}
		})
	}
}

func TestVoxelDefinitionKeepsProgressionAndGoalGates(t *testing.T) {
	v := DefaultVoxelDefinition()
	v.Enemies = nil
	v.Blocks[3].Tier = 1 // The first wooden tool cannot require itself.
	v.Goals = []VoxelGoal{{ID: "tools", Kind: "craft", Item: "wood_tool", Count: 1}, {ID: "castle", Kind: "place", Item: "brick", Count: 20}}
	err := v.Validate()
	if err == nil {
		t.Fatal("circular survival progression accepted")
	}
	for _, want := range []string{"reachable tool progression", "goals[0]", "goals[1]"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("progression feedback lacks %q: %v", want, err)
		}
	}
	v.Mode = "creative"
	if err := v.Validate(); err != nil {
		t.Fatalf("creative supply should allow crafting and placing: %v", err)
	}
	v.Goals = append(v.Goals, VoxelGoal{ID: "collect_brick", Kind: "collect", Item: "brick", Count: 1})
	if err := v.Validate(); err == nil || !strings.Contains(err.Error(), "neither a terrain block drop nor an enemy drop") {
		t.Fatalf("creative supply incorrectly counts as collecting a dropped resource: %v", err)
	}
}
