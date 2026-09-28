package gamemaker

import (
	"fmt"
	"math"
	"reflect"
)

// VoxelEvidence describes sampled game state, not counters or a client pass verdict.
type VoxelEvidence struct {
	Player    VoxelPlayerState     `json:"player"`
	Inventory []*VoxelSlot         `json:"inventory"`
	Blocks    []VoxelBlockEvidence `json:"blocks"`
	Enemies   []VoxelEnemyState    `json:"enemies"`
	Paused    bool                 `json:"paused"`
}
type VoxelBlockEvidence struct {
	Cell [3]int `json:"cell"`
	ID   int    `json:"id"`
}

func validateVoxelEvidence(e *VoxelEvidence) error {
	if e == nil {
		return nil
	}
	bad := func() error { return fmt.Errorf("invalid voxel observation") }
	if len(e.Inventory) != 36 || len(e.Blocks) > 64 || len(e.Enemies) > 24 {
		return bad()
	}
	for _, n := range append(e.Player.Position[:], e.Player.Yaw, e.Player.Pitch, e.Player.Health) {
		if !finite(n) || math.Abs(n) > 1024 {
			return bad()
		}
	}
	seen := map[[3]int]bool{}
	for _, b := range e.Blocks {
		if seen[b.Cell] || b.ID < 0 || b.ID > 64 {
			return bad()
		}
		seen[b.Cell] = true
		for _, n := range b.Cell {
			if n < 0 || n > 128 {
				return bad()
			}
		}
	}
	for _, s := range e.Inventory {
		if s != nil && (!voxelID.MatchString(s.Item) || s.Count < 1 || s.Count > 999) {
			return bad()
		}
	}
	ids := map[string]bool{}
	for _, enemy := range e.Enemies {
		if len(enemy.ID) > 48 || ids[enemy.ID] || !finite(enemy.Health) || enemy.Health < 0 || enemy.Health > 1000 {
			return bad()
		}
		ids[enemy.ID] = true
		for _, n := range enemy.Position {
			if !finite(n) || n < 0 || n > 128 {
				return bad()
			}
		}
	}
	return nil
}

func voxelScenarios(plan *GamePlan) []GameScenario {
	var result []GameScenario
	for _, operation := range []string{"move", "jump", "mine", "craft", "place", "pause"} {
		result = append(result, GameScenario{ID: "required_voxel_" + operation, Metric: "voxel_" + operation, Compare: "increased", Steps: []GameTestStep{{Action: "observe"}}})
	}
	if plan.Voxel != nil && plan.Voxel.Mode == "survival" && len(plan.Voxel.Enemies) > 0 {
		result = append(result, GameScenario{ID: "required_voxel_combat", Metric: "voxel_combat", Compare: "increased", Steps: []GameTestStep{{Action: "observe"}}})
	}
	// Custom checks keep the ordinary bounded keyboard/pointer input language.
	return append(result, plan.Scenarios...)
}

func compareVoxelObservations(plan *GamePlan, scenarios []GameScenario, observations []GameObservation) []CheckResult {
	result := compareGameObservations(scenarios, observations)
	if plan.Voxel == nil {
		return result
	}
	v := plan.Voxel
	count := func(e *VoxelEvidence, id string) int {
		n := 0
		for _, s := range e.Inventory {
			if s != nil && s.Item == id {
				n += s.Count
			}
		}
		return n
	}
	for i, s := range scenarios {
		if len(s.Metric) < 6 || s.Metric[:6] != "voxel_" {
			continue
		}
		check := &result[i]
		check.Status = "unavailable"
		check.Expected = "Real voxel state change: " + s.Metric
		check.Observed = "No matching block, inventory or player evidence"
		var o *GameObservation
		for j := range observations {
			if observations[j].ID == s.ID {
				if o != nil {
					o = nil
					break
				}
				o = &observations[j]
			}
		}
		if o == nil || o.VoxelBefore == nil || o.VoxelAfter == nil {
			continue
		}
		a, b := o.VoxelBefore, o.VoxelAfter
		passed := false
		switch s.Metric {
		case "voxel_move":
			passed = math.Hypot(b.Player.Position[0]-a.Player.Position[0], b.Player.Position[2]-a.Player.Position[2]) > .15
		case "voxel_jump":
			passed = b.Player.Position[1]-a.Player.Position[1] > .15
		case "voxel_pause":
			passed = a.Paused && b.Paused && reflect.DeepEqual(a, b)
		case "voxel_mine", "voxel_place":
			for _, first := range a.Blocks {
				for _, last := range b.Blocks {
					if first.Cell != last.Cell {
						continue
					}
					if s.Metric == "voxel_mine" && first.ID > 0 && last.ID == 0 {
						for _, block := range v.Blocks {
							if block.ID == first.ID && count(b, block.Drop) > count(a, block.Drop) {
								passed = true
							}
						}
					}
					if s.Metric == "voxel_place" && first.ID == 0 && last.ID > 0 {
						for _, item := range v.Items {
							if item.Block == last.ID && (v.Mode == "creative" || count(b, item.ID) == count(a, item.ID)-1) {
								passed = true
							}
						}
					}
				}
			}
		case "voxel_craft":
			for _, r := range v.Recipes {
				valid, changed := true, false
				for _, item := range v.Items {
					delta := 0
					if v.Mode != "creative" {
						delta -= r.Ingredients[item.ID]
					}
					if r.Item == item.ID {
						delta += r.Count
					}
					changed = changed || delta != 0
					if count(b, item.ID)-count(a, item.ID) != delta {
						valid = false
						break
					}
				}
				if valid && changed {
					passed = true
					break
				}
			}
		case "voxel_combat":
			for _, first := range a.Enemies {
				for _, last := range b.Enemies {
					if first.ID == last.ID && first.Health > last.Health {
						passed = true
					}
				}
			}
		case "voxel_respawn":
			passed = a.Player.Health == 0 && b.Player.Health > 0
		}
		if passed {
			check.Status = "passed"
			check.Observed = "Verified sampled world/player state and inventory consequences"
		}
	}
	return result
}
