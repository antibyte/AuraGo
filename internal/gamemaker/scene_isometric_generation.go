package gamemaker

import (
	"fmt"
	"math"
)

// Adds local visual edges and optional single-step links after deterministic
// placement. Fixed nodes and nodes outside the requested region never change.
func generateIsometricJoins(scene *Scene, req GenerateSceneRegionRequest) error {
	if !isIsometricScene(scene) || req.Kind != "grid" {
		return fmt.Errorf("tile_roles/connect_heights require an isometric floor grid")
	}
	palette := IsometricTileRoles{}
	if req.TileRoles != nil {
		palette = *req.TileRoles
	}
	for _, role := range []string{palette.Edge, palette.OuterCorner, palette.InnerCorner, palette.Stairs} {
		if role != "" {
			if _, ok := req.Assets.Roles[role]; !ok {
				return fmt.Errorf("unknown accepted tile role %q", role)
			}
		}
	}
	cells := map[isoCell]*SceneNode{}
	placements := map[string]*ScenePlacement{}
	for i := range scene.Nodes {
		n := &scene.Nodes[i]
		if n.Properties["walkable"] == true && (n.LevelID == "" || n.LevelID == req.LevelID) {
			w, h, ok := isoFootprint(*n)
			if !ok || w != 1 || h != 1 {
				continue
			}
			cells[isoCellAt(n.Position)] = n
		}
	}
	for i := range scene.Placements {
		placements[scene.Placements[i].NodeID] = &scene.Placements[i]
	}
	bind := func(n *SceneNode, role string, angle float64) {
		if role == "" {
			return
		}
		p := placements[n.ID]
		if p == nil {
			return
		}
		p.AssetRole = role
		p.AssetID = req.Assets.Roles[role].ID
		p.Rotation = Vec3{0, 0, angle}
	}
	offsets := []isoCell{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	for i := range scene.Nodes {
		n := &scene.Nodes[i]
		if n.RegionID != req.RegionID || n.LevelID != "" && n.LevelID != req.LevelID || n.Pinned || n.Properties["walkable"] != true {
			continue
		}
		at := isoCellAt(n.Position)
		missing := []int{}
		for dir, o := range offsets {
			other := cells[isoCell{at.X + o.X, at.Y + o.Y}]
			if other == nil || other.Position[2] != n.Position[2] {
				missing = append(missing, dir)
			}
		}
		if len(missing) == 1 {
			bind(n, palette.Edge, float64(missing[0])*math.Pi/2)
		}
		if len(missing) == 2 && (missing[1]-missing[0] == 1 || missing[1]-missing[0] == 3) {
			corner := missing[0]
			if missing[1]-missing[0] == 3 {
				corner = 3
			}
			bind(n, palette.OuterCorner, float64(corner)*math.Pi/2)
		}
		if len(missing) == 0 {
			for dir, o := range offsets {
				next := offsets[(dir+1)%4]
				if cells[isoCell{at.X + o.X + next.X, at.Y + o.Y + next.Y}] == nil {
					bind(n, palette.InnerCorner, float64(dir)*math.Pi/2)
					break
				}
			}
		}
		if !req.ConnectHeights {
			continue
		}
		for dir, o := range offsets {
			other := cells[isoCell{at.X + o.X, at.Y + o.Y}]
			if other == nil || other.Position[2] != n.Position[2]+1 {
				continue
			}
			// One authored stair tile connects one edge; never a hidden teleport or
			// an unrequested alteration of a neighbouring fixed object.
			id := stableSceneChildID("stairs", n.ID+":"+other.ID)
			appendSceneRoute(scene, SceneRoute{ID: id, From: n.ID, To: other.ID, Kind: "stairs", RegionID: req.RegionID, LevelID: req.LevelID})
			bind(n, palette.Stairs, float64(dir)*math.Pi/2)
			break
		}
	}
	return nil
}
