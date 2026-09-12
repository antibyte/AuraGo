package gamemaker

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

var (
	ErrSceneConflict = errors.New("scene_conflict")
	ErrSceneInvalid  = errors.New("scene_invalid")
)

// SceneValidationError retains every actionable error while keeping warnings
// available through ValidateScene for callers that want to show them.
type SceneValidationError struct {
	Diagnostics []SceneDiagnostic
}

func (e *SceneValidationError) Error() string {
	parts := make([]string, 0, len(e.Diagnostics))
	for _, d := range e.Diagnostics {
		if d.Severity == "error" {
			if d.Path != "" {
				parts = append(parts, d.Path+": "+d.Message)
			} else {
				parts = append(parts, d.Message)
			}
		}
	}
	if len(parts) == 0 {
		return ErrSceneInvalid.Error()
	}
	return ErrSceneInvalid.Error() + ": " + strings.Join(parts, "; ")
}

func (e *SceneValidationError) Unwrap() error { return ErrSceneInvalid }

func sceneError(path, message string) SceneDiagnostic {
	return SceneDiagnostic{Severity: "error", Path: path, Message: message}
}

func sceneWarning(path, message string) SceneDiagnostic {
	return SceneDiagnostic{Severity: "warning", Path: path, Message: message}
}

func finiteVec(v Vec3) bool {
	return finite(v[0]) && finite(v[1]) && finite(v[2])
}

func validBounds(b SceneBounds) bool {
	if !finiteVec(b.Min) || !finiteVec(b.Max) {
		return false
	}
	return b.Min[0] <= b.Max[0] && b.Min[1] <= b.Max[1] && b.Min[2] <= b.Max[2]
}

func pointInBounds(p Vec3, b SceneBounds) bool {
	return p[0] >= b.Min[0] && p[0] <= b.Max[0] && p[1] >= b.Min[1] && p[1] <= b.Max[1] && p[2] >= b.Min[2] && p[2] <= b.Max[2]
}

func boundsInBounds(inner, outer SceneBounds) bool {
	return pointInBounds(inner.Min, outer) && pointInBounds(inner.Max, outer)
}

func samePoint(a, b Vec3) bool {
	return math.Abs(a[0]-b[0]) < 1e-9 && math.Abs(a[1]-b[1]) < 1e-9 && math.Abs(a[2]-b[2]) < 1e-9
}

func sceneID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for i, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sceneItemExtent(node SceneNode) Vec3 {
	return Vec3{math.Abs(node.Size[0]) / 2, math.Abs(node.Size[1]) / 2, math.Abs(node.Size[2]) / 2}
}

func placementAssetID(placement ScenePlacement) string { return strings.TrimSpace(placement.AssetID) }

func sceneScale(value float64) float64 {
	if math.Abs(value) <= 1e-9 {
		return 1
	}
	return value
}

func sceneAssetBoundsAvailable(bounds SceneBounds) bool {
	if !validBounds(bounds) {
		return false
	}
	return bounds.Min != bounds.Max
}

func rotateSceneVector(value, rotation Vec3) Vec3 {
	// Three.js applies the default XYZ Euler order. Keeping the same order here
	// makes validation and generation agree with the runtime model transform.
	x, y, z := rotation[0], rotation[1], rotation[2]
	cx, sx := math.Cos(x), math.Sin(x)
	cy, sy := math.Cos(y), math.Sin(y)
	cz, sz := math.Cos(z), math.Sin(z)
	// XYZ Euler uses Rx * Ry * Rz with column vectors.
	return Vec3{
		cy*cz*value[0] - cy*sz*value[1] + sy*value[2],
		(sx*sy*cz+cx*sz)*value[0] + (cx*cz-sx*sy*sz)*value[1] - sx*cy*value[2],
		(sx*sz-cx*sy*cz)*value[0] + (sx*cz+cx*sy*sz)*value[1] + cx*cy*value[2],
	}
}

func scenePlacementBounds(placement ScenePlacement, asset SceneAsset, dimension string) (SceneBounds, bool) {
	if !sceneAssetBoundsAvailable(asset.Bounds) {
		return SceneBounds{}, false
	}
	min := Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}
	max := Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for mask := 0; mask < 8; mask++ {
		corner := Vec3{}
		for axis := 0; axis < 3; axis++ {
			if mask&(1<<axis) == 0 {
				corner[axis] = asset.Bounds.Min[axis]
			} else {
				corner[axis] = asset.Bounds.Max[axis]
			}
			corner[axis] *= sceneScale(placement.Scale[axis])
		}
		corner = rotateSceneVector(corner, placement.Rotation).Add(placement.Position)
		for axis := 0; axis < 3; axis++ {
			min[axis] = math.Min(min[axis], corner[axis])
			max[axis] = math.Max(max[axis], corner[axis])
		}
	}
	if dimension == "2d" {
		min[2], max[2] = 0, 0
	}
	return SceneBounds{Min: min, Max: max}, true
}

func sceneBoundsFromCenterExtent(center, extent Vec3) SceneBounds {
	return SceneBounds{Min: center.Sub(extent), Max: center.Add(extent)}
}

func unionSceneBounds(base SceneBounds, next SceneBounds) SceneBounds {
	for axis := 0; axis < 3; axis++ {
		base.Min[axis] = math.Min(base.Min[axis], next.Min[axis])
		base.Max[axis] = math.Max(base.Max[axis], next.Max[axis])
	}
	return base
}

func sceneNodeBounds(node SceneNode, placements []ScenePlacement, colliders []SceneCollider, assets AssetCatalog, dimension string) SceneBounds {
	bounds := sceneBoundsFromCenterExtent(node.Position, sceneItemExtent(node))
	for _, placement := range placements {
		if placement.NodeID != node.ID {
			continue
		}
		if asset, ok := assets.Assets[placementAssetID(placement)]; ok {
			if placementBounds, ok := scenePlacementBounds(placement, asset, dimension); ok {
				bounds = unionSceneBounds(bounds, placementBounds)
			}
		}
	}
	for _, collider := range colliders {
		if collider.NodeID != node.ID {
			continue
		}
		colliderBounds := sceneBoundsFromCenterExtent(node.Position.Add(collider.Offset), Vec3{
			math.Abs(collider.Extents[0]), math.Abs(collider.Extents[1]), math.Abs(collider.Extents[2]),
		})
		bounds = unionSceneBounds(bounds, colliderBounds)
	}
	return bounds
}

// ValidateScene is pure and does not inspect disk. A non-empty asset catalog
// enables exact asset-role and socket checks; an empty catalog is suitable for
// procedural or not-yet-imported scene drafts.
func ValidateScene(scene Scene, assets AssetCatalog) []SceneDiagnostic {
	var out []SceneDiagnostic
	for _, collection := range []struct {
		path  string
		count int
	}{
		{"levels", len(scene.Levels)},
		{"nodes", len(scene.Nodes)},
		{"regions", len(scene.Regions)},
		{"placements", len(scene.Placements)},
		{"colliders", len(scene.Colliders)},
		{"attachments", len(scene.Attachments)},
		{"zones", len(scene.Zones)},
		{"routes", len(scene.Routes)},
	} {
		if collection.count > SceneMaxItems {
			out = append(out, sceneError(collection.path, fmt.Sprintf("at most %d entries are supported", SceneMaxItems)))
		}
	}
	if scene.SchemaVersion != SceneSchemaVersion {
		out = append(out, sceneError("schema_version", fmt.Sprintf("must be %d", SceneSchemaVersion)))
	}
	scene.Dimension = strings.ToLower(strings.TrimSpace(scene.Dimension))
	if scene.Dimension != "2d" && scene.Dimension != "3d" {
		out = append(out, sceneError("dimension", "must be 2d or 3d"))
	}
	if !validBounds(scene.WorldBounds) {
		out = append(out, sceneError("world_bounds", "must contain finite min/max coordinates with min <= max"))
	}
	if scene.CameraBounds != nil && !validBounds(*scene.CameraBounds) {
		out = append(out, sceneError("camera_bounds", "must be finite with min <= max"))
	}
	if len(scene.Levels) == 0 {
		out = append(out, sceneError("levels", "requires exactly one active level"))
	}
	levelIDs := map[string]bool{}
	activeLevels := 0
	regionIDs := map[string]bool{}
	for i, level := range scene.Levels {
		path := fmt.Sprintf("levels[%d]", i)
		if !sceneID(level.ID) {
			out = append(out, sceneError(path+".id", "must be a short stable identifier"))
		} else if levelIDs[level.ID] {
			out = append(out, sceneError(path+".id", "must be unique"))
		}
		levelIDs[level.ID] = true
		if level.Active {
			activeLevels++
		}
	}
	if activeLevels != 1 {
		out = append(out, sceneError("levels", "requires exactly one active level"))
	}
	if scene.Navigation != "" && !containsString([]string{"topdown", "ground", "platforms", "waypoints", "custom"}, strings.ToLower(scene.Navigation)) {
		out = append(out, sceneError("navigation", "must be topdown, ground, platforms, waypoints, or custom"))
	}
	if strings.EqualFold(scene.Navigation, "custom") {
		out = append(out, sceneWarning("navigation", "custom navigation is unverified; provide runtime validation"))
	}

	ids := map[string]string{}
	claimID := func(path, id string) {
		if !sceneID(id) {
			out = append(out, sceneError(path, "must be a short stable identifier"))
			return
		}
		if previous, exists := ids[id]; exists {
			out = append(out, sceneError(path, "duplicates "+previous))
			return
		}
		ids[id] = path
	}
	for i, region := range scene.Regions {
		path := fmt.Sprintf("regions[%d]", i)
		claimID(path+".id", region.ID)
		if !validBounds(region.Bounds) || !boundsInBounds(region.Bounds, scene.WorldBounds) {
			out = append(out, sceneError(path+".bounds", "must be finite with min <= max"))
		}
		if region.LevelID != "" && !levelIDs[region.LevelID] {
			out = append(out, sceneError(path+".level_id", "references an unknown level"))
		}
		if sceneID(region.ID) {
			regionIDs[region.ID] = true
		}
	}
	nodes := map[string]SceneNode{}
	for i, node := range scene.Nodes {
		path := fmt.Sprintf("nodes[%d]", i)
		claimID(path+".id", node.ID)
		nodes[node.ID] = node
		if !finiteVec(node.Position) || !finiteVec(node.Size) {
			out = append(out, sceneError(path, "position and size must be finite"))
		}
		if node.Size[0] < 0 || node.Size[1] < 0 || node.Size[2] < 0 {
			out = append(out, sceneError(path+".size", "must not be negative"))
		}
		if node.LevelID != "" && !levelIDs[node.LevelID] {
			out = append(out, sceneError(path+".level_id", "references an unknown level"))
		}
		if node.RegionID != "" && !regionIDs[node.RegionID] {
			out = append(out, sceneError(path+".region_id", "references an unknown region"))
		}
		if !isDecorative(node.Kind) {
			if !pointInBounds(node.Position, scene.WorldBounds) {
				out = append(out, sceneError(path+".position", "must be inside world_bounds"))
			}
			if !boundsInBounds(sceneNodeBounds(node, scene.Placements, scene.Colliders, assets, scene.Dimension), scene.WorldBounds) {
				out = append(out, sceneError(path+".bounds", "node, placement, and collider extents must be inside world_bounds"))
			}
		}
	}

	placementsByNode := map[string]ScenePlacement{}
	for i, placement := range scene.Placements {
		path := fmt.Sprintf("placements[%d]", i)
		claimID(path+".id", placement.ID)
		if nodes[placement.NodeID].ID == "" {
			out = append(out, sceneError(path+".node_id", "references an unknown node"))
		}
		if strings.TrimSpace(placement.AssetRole) == "" {
			out = append(out, sceneError(path+".asset_role", "is required and independent from behavior"))
		}
		if strings.TrimSpace(placement.Behavior) == "" {
			out = append(out, sceneError(path+".behavior", "is required and independent from asset_role"))
		}
		if !finiteVec(placement.Position) || !finiteVec(placement.Rotation) || !finiteVec(placement.Scale) {
			out = append(out, sceneError(path, "transform coordinates must be finite"))
		}
		if !isDecorative(placement.Behavior) && !pointInBounds(placement.Position, scene.WorldBounds) {
			out = append(out, sceneError(path+".position", "must be inside world_bounds unless decorative"))
		}
		if placement.LevelID != "" && !levelIDs[placement.LevelID] {
			out = append(out, sceneError(path+".level_id", "references an unknown level"))
		}
		if placement.RegionID != "" && !regionIDs[placement.RegionID] {
			out = append(out, sceneError(path+".region_id", "references an unknown region"))
		}
		placementsByNode[placement.NodeID] = placement
		if len(assets.Assets) > 0 {
			assetID := placementAssetID(placement)
			asset, ok := assets.Assets[assetID]
			if !ok {
				out = append(out, sceneError(path+".asset_id", "is not an accepted catalog asset"))
			} else {
				if len(asset.Roles) == 0 || !containsString(asset.Roles, placement.AssetRole) {
					out = append(out, sceneError(path+".asset_role", "is not accepted for asset_id "+assetID))
				}
				if !isDecorative(placement.Behavior) {
					if placementBounds, hasBounds := scenePlacementBounds(placement, asset, scene.Dimension); hasBounds && !boundsInBounds(placementBounds, scene.WorldBounds) {
						out = append(out, sceneError(path+".bounds", "asset bounds must be inside world_bounds"))
					}
				}
			}
		}
	}

	colliderNodes := map[string]bool{}
	for i, collider := range scene.Colliders {
		path := fmt.Sprintf("colliders[%d]", i)
		if colliderNodes[collider.NodeID] {
			out = append(out, sceneError(path, "one simple collider per node; use separate nodes or custom physics for compound bodies"))
		}
		colliderNodes[collider.NodeID] = true
		if scene.Dimension == "3d" && collider.Extents[2] <= 0 {
			out = append(out, sceneError(path+".extents", "3D requires a positive z half-extent"))
		}
		if collider.Shape == "mesh" || collider.Shape == "capsule" || scene.Dimension == "3d" && collider.Shape == "circle" {
			out = append(out, sceneWarning(path+".shape", "the shared renderer uses bounding boxes for this shape; exact contacts require a custom physics hook and runtime test"))
		}
		claimID(path+".id", collider.ID)
		if nodes[collider.NodeID].ID == "" {
			out = append(out, sceneError(path+".node_id", "references an unknown node"))
		}
		if !containsString([]string{"box", "circle", "capsule", "mesh"}, strings.ToLower(collider.Shape)) {
			out = append(out, sceneError(path+".shape", "must be box, circle, capsule, or mesh"))
		}
		if !finiteVec(collider.Extents) || collider.Extents[0] <= 0 || collider.Extents[1] <= 0 || collider.Extents[2] < 0 {
			out = append(out, sceneError(path+".extents", "requires positive x/y and non-negative z extents"))
		}
		if !finiteVec(collider.Offset) {
			out = append(out, sceneError(path+".offset", "must be finite"))
		}
		if collider.RegionID != "" && !regionIDs[collider.RegionID] {
			out = append(out, sceneError(path+".region_id", "references an unknown region"))
		}
		if node, ok := nodes[collider.NodeID]; ok && !isDecorative(node.Kind) &&
			!boundsInBounds(sceneBoundsFromCenterExtent(node.Position.Add(collider.Offset), Vec3{
				math.Abs(collider.Extents[0]), math.Abs(collider.Extents[1]), math.Abs(collider.Extents[2]),
			}), scene.WorldBounds) {
			out = append(out, sceneError(path+".bounds", "collider extents must be inside world_bounds"))
		}
	}

	for i, attachment := range scene.Attachments {
		path := fmt.Sprintf("attachments[%d]", i)
		claimID(path+".id", attachment.ID)
		if nodes[attachment.NodeID].ID == "" {
			out = append(out, sceneError(path+".node_id", "references an unknown node"))
		}
		if strings.TrimSpace(attachment.Socket) == "" {
			out = append(out, sceneError(path+".socket", "is required"))
		}
		if attachment.RegionID != "" && !regionIDs[attachment.RegionID] {
			out = append(out, sceneError(path+".region_id", "references an unknown region"))
		}
		if len(assets.Assets) > 0 {
			assetID := attachment.AssetID
			if assetID == "" {
				assetID = placementsByNode[attachment.NodeID].AssetID
			}
			asset, ok := assets.Assets[assetID]
			if !ok {
				out = append(out, sceneError(path+".asset_id", "is not an accepted catalog asset"))
			} else if len(asset.Sockets) == 0 || !containsString(asset.Sockets, attachment.Socket) {
				out = append(out, sceneError(path+".socket", "is not present on asset_id "+assetID))
			}
		}
	}

	var spawn, goal []SceneZone
	for i, zone := range scene.Zones {
		path := fmt.Sprintf("zones[%d]", i)
		claimID(path+".id", zone.ID)
		if !containsString([]string{"spawn", "goal", "loss", "trigger"}, strings.ToLower(zone.Kind)) {
			out = append(out, sceneError(path+".kind", "must be spawn, goal, loss, or trigger"))
		}
		if !validBounds(zone.Bounds) || !boundsInBounds(zone.Bounds, scene.WorldBounds) {
			out = append(out, sceneError(path+".bounds", "must be finite with min <= max"))
		}
		if zone.LevelID != "" && !levelIDs[zone.LevelID] {
			out = append(out, sceneError(path+".level_id", "references an unknown level"))
		}
		if zone.RegionID != "" && !regionIDs[zone.RegionID] {
			out = append(out, sceneError(path+".region_id", "references an unknown region"))
		}
		switch strings.ToLower(zone.Kind) {
		case "spawn":
			spawn = append(spawn, zone)
		case "goal":
			goal = append(goal, zone)
		}
	}
	if len(spawn) == 0 {
		out = append(out, sceneWarning("zones", "no spawn zone is defined"))
	}
	if len(goal) == 0 {
		out = append(out, sceneWarning("zones", "no goal zone is defined"))
	}
	for _, s := range spawn {
		for i, loss := range scene.Zones {
			if strings.EqualFold(loss.Kind, "loss") && boundsOverlap(s.Bounds, loss.Bounds) {
				out = append(out, sceneError(fmt.Sprintf("zones[%d]", i), "loss zone overlaps spawn zone and makes the start impossible"))
			}
		}
	}
	for i, route := range scene.Routes {
		path := fmt.Sprintf("routes[%d]", i)
		claimID(path+".id", route.ID)
		if nodes[route.From].ID == "" || nodes[route.To].ID == "" {
			out = append(out, sceneError(path, "from and to must reference existing node IDs"))
		}
		if route.RegionID != "" && !regionIDs[route.RegionID] {
			out = append(out, sceneError(path+".region_id", "references an unknown region"))
		}
		if len(route.Waypoints) > 256 {
			out = append(out, sceneError(path+".waypoints", "is limited to 256 points"))
		}
		for j, point := range route.Waypoints {
			if !finiteVec(point) || !pointInBounds(point, scene.WorldBounds) {
				out = append(out, sceneError(fmt.Sprintf("%s.waypoints[%d]", path, j), "must be finite with min <= max"))
			}
		}
	}

	if scene.Dimension == "2d" {
		zeroZ := func(path string, value Vec3) {
			if value[2] != 0 {
				out = append(out, sceneError(path, "2d coordinates must use z=0"))
			}
		}
		if scene.CameraBounds != nil {
			zeroZ("camera_bounds.min", scene.CameraBounds.Min)
			zeroZ("camera_bounds.max", scene.CameraBounds.Max)
		}
		zeroZ("world_bounds.min", scene.WorldBounds.Min)
		zeroZ("world_bounds.max", scene.WorldBounds.Max)
		for i, region := range scene.Regions {
			zeroZ(fmt.Sprintf("regions[%d].bounds.min", i), region.Bounds.Min)
			zeroZ(fmt.Sprintf("regions[%d].bounds.max", i), region.Bounds.Max)
		}
		for i, node := range scene.Nodes {
			zeroZ(fmt.Sprintf("nodes[%d].position", i), node.Position)
			zeroZ(fmt.Sprintf("nodes[%d].size", i), node.Size)
		}
		for i, placement := range scene.Placements {
			zeroZ(fmt.Sprintf("placements[%d].position", i), placement.Position)
		}
		for i, collider := range scene.Colliders {
			zeroZ(fmt.Sprintf("colliders[%d].extents", i), collider.Extents)
			zeroZ(fmt.Sprintf("colliders[%d].offset", i), collider.Offset)
		}
		for i, attachment := range scene.Attachments {
			zeroZ(fmt.Sprintf("attachments[%d].position", i), attachment.Position)
		}
		for i, route := range scene.Routes {
			for j, point := range route.Waypoints {
				zeroZ(fmt.Sprintf("routes[%d].waypoints[%d]", i, j), point)
			}
		}
		for i, z := range scene.Zones {
			zeroZ(fmt.Sprintf("zones[%d].bounds.min", i), z.Bounds.Min)
			zeroZ(fmt.Sprintf("zones[%d].bounds.max", i), z.Bounds.Max)
		}
	}
	out = append(out, conservativeReachability(scene, nodes)...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity == "error"
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func isDecorative(kind string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	return kind == "decorative" || kind == "decoration" || kind == "background" || kind == "scenery"
}

func boundsOverlap(a, b SceneBounds) bool {
	return a.Min[0] <= b.Max[0] && a.Max[0] >= b.Min[0] && a.Min[1] <= b.Max[1] && a.Max[1] >= b.Min[1] && a.Min[2] <= b.Max[2] && a.Max[2] >= b.Min[2]
}

func nearestNode(nodes map[string]SceneNode, zone SceneZone) string {
	center := Vec3{(zone.Bounds.Min[0] + zone.Bounds.Max[0]) / 2, (zone.Bounds.Min[1] + zone.Bounds.Max[1]) / 2, (zone.Bounds.Min[2] + zone.Bounds.Max[2]) / 2}
	best, bestDistance := "", math.Inf(1)
	for id, node := range nodes {
		d := (node.Position[0]-center[0])*(node.Position[0]-center[0]) + (node.Position[1]-center[1])*(node.Position[1]-center[1]) + (node.Position[2]-center[2])*(node.Position[2]-center[2])
		if d < bestDistance || (d == bestDistance && id < best) {
			best, bestDistance = id, d
		}
	}
	return best
}

func routeReachability(scene Scene, nodes map[string]SceneNode) []SceneDiagnostic {
	if len(nodes) == 0 || len(scene.Zones) == 0 {
		return nil
	}
	var starts, goals []SceneZone
	for _, zone := range scene.Zones {
		switch strings.ToLower(zone.Kind) {
		case "spawn":
			starts = append(starts, zone)
		case "goal":
			goals = append(goals, zone)
		}
	}
	if len(starts) == 0 || len(goals) == 0 {
		return []SceneDiagnostic{sceneWarning("reachability", "waypoint reachability needs at least one spawn and one goal zone")}
	}
	if len(scene.Routes) == 0 {
		return []SceneDiagnostic{sceneWarning("reachability", "waypoint reachability cannot be verified without declared routes")}
	}
	graph := map[string][]string{}
	for _, route := range scene.Routes {
		graph[route.From] = append(graph[route.From], route.To)
		if strings.EqualFold(route.Kind, "undirected") || route.Kind == "" {
			graph[route.To] = append(graph[route.To], route.From)
		}
	}
	for _, start := range starts {
		from := nearestNode(nodes, start)
		for _, goal := range goals {
			to := nearestNode(nodes, goal)
			if from == "" || to == "" {
				continue
			}
			seen := map[string]bool{from: true}
			queue := []string{from}
			for len(queue) > 0 {
				current := queue[0]
				queue = queue[1:]
				for _, next := range graph[current] {
					if !seen[next] {
						seen[next] = true
						queue = append(queue, next)
					}
				}
			}
			if !seen[to] {
				return []SceneDiagnostic{sceneError("reachability", "goal is unreachable from spawn through the declared routes")}
			}
		}
	}
	return nil
}

func validateScene(scene Scene, assets AssetCatalog) error {
	diagnostics := ValidateScene(scene, assets)
	var errorsOnly []SceneDiagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			errorsOnly = append(errorsOnly, diagnostic)
		}
	}
	if len(errorsOnly) > 0 {
		return &SceneValidationError{Diagnostics: errorsOnly}
	}
	return nil
}
