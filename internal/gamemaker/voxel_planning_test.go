package gamemaker

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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

func TestVoxelDesignCorrectionPreservesCreativeDraft(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "object"
		if native {
			name = "native_string"
		}
		t.Run(name, func(t *testing.T) {
			s := newTestService(t)
			project, err := s.CreateProject(context.Background(), CreateProjectRequest{Name: "Castle", Description: "Creative castle sandbox", Dimension: "3d", Variant: "voxel"})
			if err != nil {
				t.Fatal(err)
			}
			s.SetRunner(planningRunner(func(ctx context.Context, run JobRun) error {
				definition := DefaultVoxelDefinition()
				definition.Mode, definition.Terrain, definition.Enemies = "creative", "flat", []VoxelEnemy{}
				definition.Goals = []VoxelGoal{{ID: "castle", Kind: "place", Item: "brick", Count: 20}}
				broken := *definition
				broken.Blocks = slices.DeleteFunc(slices.Clone(definition.Blocks), func(b VoxelBlock) bool { return b.Material == "ore" || b.Material == "sand" })
				broken.Goals = []VoxelGoal{{ID: "castle", Kind: "build", Item: "brick", Count: 20}}
				encode := func(v *VoxelDefinition) any {
					if native {
						data, _ := json.Marshal(v)
						return string(data)
					}
					return v
				}
				objective := "Build a castle freely without enemies or a timer"
				features := []string{"Creative building with stone walls and wooden roofs"}
				data, _ := json.Marshal(map[string]any{"base": "voxel", "objective": objective, "features": features, "voxel": encode(&broken)})
				err := s.SetDesignJSON(ctx, run.Job.ID, data)
				var detail *DesignValidationError
				if !errors.As(err, &detail) || detail.RemainingAttempts != 2 {
					return errors.New("invalid voxel draft did not retain the two correction attempts")
				}
				for _, want := range []string{"ore, sand", "goals[0]"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("planning feedback lacks %q: %v", want, err)
					}
				}
				if plan, _ := s.GetPlan(ctx, run.Job.ID); plan != nil {
					t.Error("invalid palette passed plan acceptance")
				}
				data, _ = json.Marshal(map[string]any{"voxel": encode(definition)})
				if err := s.SetDesignJSON(ctx, run.Job.ID, data); err != nil {
					return err
				}
				plan, err := s.GetPlan(ctx, run.Job.ID)
				if err != nil {
					return err
				}
				if plan.Template != "voxel" || plan.Objective != objective || !reflect.DeepEqual(plan.Scope, features) || !reflect.DeepEqual(plan.Voxel, definition) {
					t.Errorf("correction lost the authored creative design: %#v", plan)
				}
				return errors.New("verified creative correction")
			}))
			job, err := s.StartJob(context.Background(), project.ID, StartJobRequest{})
			if err != nil {
				t.Fatal(err)
			}
			if done := waitJob(t, s, job.ID); done.Error != "verified creative correction" {
				t.Fatal(done.Error)
			}
		})
	}
}

func TestVoxelNativeDesignKeepsStrictDecoding(t *testing.T) {
	s := newTestService(t)
	project := Project{Dimension: "3d", Variant: "voxel"}
	data, err := json.Marshal(DefaultVoxelDefinition())
	if err != nil {
		t.Fatal(err)
	}
	for name, definition := range map[string]string{
		"syntax":        string(data[:len(data)-1]),
		"unknown field": strings.Replace(string(data), `"version":1`, `"version":1,"unsupported":true`, 1),
		"size shape":    strings.Replace(string(data), `"size":[96,48,96]`, `"size":[96,48,96,16]`, 1),
		"nested string": `"` + strings.ReplaceAll(string(data), `"`, `\"`) + `"`,
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := json.Marshal(map[string]any{"base": "voxel", "objective": "Build a castle", "features": []string{"Creative building"}, "voxel": definition})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.expandDesign(context.Background(), name, project, encoded); err == nil {
				t.Fatal("malformed native definition accepted")
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
