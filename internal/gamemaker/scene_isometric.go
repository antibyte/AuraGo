package gamemaker

import (
	"fmt"
	"math"
)

type isoCell struct{ X, Y int }

func isoCellAt(p Vec3) isoCell { return isoCell{int(math.Floor(p[0])), int(math.Floor(p[1]))} }
func isoFootprint(n SceneNode) (int, int, bool) {
	w, h := 1.0, 1.0
	switch v := n.Properties["footprint"].(type) {
	case []any:
		if len(v) != 2 {
			return 0, 0, false
		}
		w, _ = v[0].(float64)
		h, _ = v[1].(float64)
	case []float64:
		if len(v) != 2 {
			return 0, 0, false
		}
		w, h = v[0], v[1]
	}
	return int(math.Ceil(w)), int(math.Ceil(h)), finite(w) && finite(h) && w > 0 && h > 0 && w <= 256 && h <= 256
}

// Mirrors the runtime's cardinal footprint navigation. Custom controllers,
// moving geometry and inter-level story conditions remain runtime-only evidence.
func isometricReachability(scene Scene) []SceneDiagnostic {
	var out []SceneDiagnostic
	for _, level := range scene.Levels {
		floors := map[isoCell]float64{}
		blocked := map[isoCell]bool{}
		nodes := map[string]SceneNode{}
		links := map[[2]isoCell]bool{}
		var starts, goals []Vec3
		count := 0
		active := func(id string) bool { return id == "" || id == level.ID }
		for _, n := range scene.Nodes {
			if !active(n.LevelID) {
				continue
			}
			nodes[n.ID] = n
			w, h, ok := isoFootprint(n)
			if !ok {
				return []SceneDiagnostic{sceneError("nodes", "isometric footprint must contain two finite positive sizes, at most 256")}
			}
			if !finiteVec(n.Position) {
				continue
			}
			count += w * h
			if count > 65536 {
				return []SceneDiagnostic{sceneError("nodes", "isometric grid exceeds 65536 cells per level")}
			}
			at := isoCellAt(n.Position)
			if n.Kind == "player" || n.Properties["player"] == true {
				starts = append(starts, n.Position)
			}
			if n.Properties["goal"] == true {
				goals = append(goals, n.Position)
			}
			for x := 0; x < w; x++ {
				for y := 0; y < h; y++ {
					cell := isoCell{at.X + x, at.Y + y}
					if n.Properties["walkable"] == true {
						if n.Position[0] != math.Floor(n.Position[0]) || n.Position[1] != math.Floor(n.Position[1]) || n.Position[2] != math.Floor(n.Position[2]) {
							out = append(out, sceneError("nodes", "walkable isometric cells need integer grid positions and elevation levels"))
							continue
						}
						if old, exists := floors[cell]; exists && old != n.Position[2] {
							out = append(out, sceneError("nodes", "overlapping walkable elevations require separate levels"))
						}
						floors[cell] = n.Position[2]
					}
					if n.Properties["solid"] == true {
						blocked[cell] = true
					}
				}
			}
		}
		for _, zone := range scene.Zones {
			if !active(zone.LevelID) {
				continue
			}
			center := sceneCenter(zone.Bounds)
			if zone.Kind == "spawn" {
				starts = append(starts, center)
			}
			if zone.Kind == "goal" {
				goals = append(goals, center)
			}
		}
		for _, c := range scene.Colliders {
			n, ok := nodes[c.NodeID]
			if !ok || n.Kind == "player" || n.Properties["dynamic"] == true || n.Properties["walkable"] == true || !finiteVec(c.Extents) || !finiteVec(c.Offset) {
				continue
			}
			p := n.Position.Add(c.Offset)
			for x := int(math.Floor(p[0] - c.Extents[0])); x < int(math.Ceil(p[0]+c.Extents[0])); x++ {
				for y := int(math.Floor(p[1] - c.Extents[1])); y < int(math.Ceil(p[1]+c.Extents[1])); y++ {
					count++
					if count > 65536 {
						return []SceneDiagnostic{sceneError("colliders", "isometric grid exceeds 65536 cells")}
					}
					blocked[isoCell{x, y}] = true
				}
			}
		}
		for _, r := range scene.Routes {
			if !active(r.LevelID) || (r.Kind != "stairs" && r.Kind != "ramp") {
				continue
			}
			a, aok := nodes[r.From]
			b, bok := nodes[r.To]
			ac, bc := isoCellAt(a.Position), isoCellAt(b.Position)
			if !aok || !bok || a.Properties["walkable"] != true || b.Properties["walkable"] != true || math.Abs(float64(ac.X-bc.X))+math.Abs(float64(ac.Y-bc.Y)) != 1 {
				out = append(out, sceneError("routes", "isometric stairs/ramps must join adjacent floor nodes in one level"))
				continue
			}
			links[[2]isoCell{ac, bc}] = true
			links[[2]isoCell{bc, ac}] = true
		}
		if !level.Active {
			continue
		}
		if scene.Navigation == "custom" || len(floors) == 0 {
			out = append(out, sceneWarning("reachability", "isometric custom movement or missing walkable floors requires a runtime test"))
			continue
		}
		if len(starts) == 0 {
			out = append(out, sceneWarning("reachability", "isometric navigation requires a player node"))
			continue
		}
		for _, position := range starts {
			start := isoCellAt(position)
			if height, ok := floors[start]; !ok || blocked[start] || math.Abs(height-position[2]) > .001 {
				out = append(out, sceneError("reachability", "isometric player starts outside a walkable floor, at the wrong elevation or inside a collider"))
				continue
			}
			seen := map[isoCell]bool{start: true}
			queue := []isoCell{start}
			for head := 0; head < len(queue); head++ {
				c := queue[head]
				for _, offset := range []isoCell{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					next := isoCell{c.X + offset.X, c.Y + offset.Y}
					height, ok := floors[next]
					if !ok || blocked[next] || seen[next] || (height != floors[c] && !links[[2]isoCell{c, next}]) {
						continue
					}
					seen[next] = true
					queue = append(queue, next)
				}
			}
			for _, position := range goals {
				goal := isoCellAt(position)
				if !seen[goal] || math.Abs(floors[goal]-position[2]) > .001 {
					out = append(out, sceneError("reachability", fmt.Sprintf("isometric goal (%d,%d) has no walkable path from the player", goal.X, goal.Y)))
				}
			}
		}
	}
	return out
}
