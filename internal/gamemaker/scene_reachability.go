package gamemaker

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func conservativeReachability(scene Scene, nodes map[string]SceneNode) []SceneDiagnostic {
	navigation := strings.ToLower(strings.TrimSpace(scene.Navigation))
	if len(nodes) == 0 || len(scene.Zones) == 0 {
		switch navigation {
		case "topdown", "ground", "platforms", "waypoints":
			return []SceneDiagnostic{sceneWarning("reachability", "reachability cannot be evaluated without scene nodes and spawn/goal zones")}
		default:
			return nil
		}
	}
	switch navigation {
	case "custom":
		return []SceneDiagnostic{sceneWarning("reachability", "custom navigation cannot be verified statically")}
	case "waypoints":
		return routeReachability(scene, nodes)
	case "platforms":
		return platformReachability(scene, nodes)
	case "topdown", "ground", "":
		return rasterReachability(scene, nodes)
	default:
		return []SceneDiagnostic{sceneWarning("reachability", "navigation mode is not statically verified")}
	}
}

func zoneCenters(scene Scene) (starts, goals []Vec3) {
	for _, zone := range scene.Zones {
		center := sceneCenter(zone.Bounds)
		switch strings.ToLower(zone.Kind) {
		case "spawn":
			starts = append(starts, center)
		case "goal":
			goals = append(goals, center)
		}
	}
	return starts, goals
}

type sceneGrid struct {
	minX, minZ   float64
	stepX, stepZ float64
	w, h         int
	blocked      []bool
}

func newSceneGrid(scene Scene) sceneGrid {
	b := scene.WorldBounds
	sizeX := math.Max(b.Max[0]-b.Min[0], 1e-6)
	sizeZ := math.Max(b.Max[1]-b.Min[1], 1e-6)
	if scene.Dimension == "3d" {
		sizeZ = math.Max(b.Max[2]-b.Min[2], 1e-6)
	}
	w := int(math.Ceil(sizeX))
	h := int(math.Ceil(sizeZ))
	if w < 4 {
		w = 4
	}
	if h < 4 {
		h = 4
	}
	if w > 64 {
		w = 64
	}
	if h > 64 {
		h = 64
	}
	minZ := b.Min[1]
	if scene.Dimension == "3d" {
		minZ = b.Min[2]
	}
	return sceneGrid{
		minX:    b.Min[0],
		minZ:    minZ,
		stepX:   sizeX / float64(w),
		stepZ:   sizeZ / float64(h),
		w:       w,
		h:       h,
		blocked: make([]bool, w*h),
	}
}

func (g sceneGrid) index(x, z int) int { return z*g.w + x }

func (g sceneGrid) point(x, z int, dimension string) Vec3 {
	p := Vec3{g.minX + (float64(x)+.5)*g.stepX, 0, g.minZ + (float64(z)+.5)*g.stepZ}
	if dimension == "2d" {
		p[1] = g.minZ + (float64(z)+.5)*g.stepZ
		p[2] = 0
	}
	return p
}

func (g sceneGrid) cell(x, z int) (minX, maxX, minZ, maxZ float64) {
	return g.minX + float64(x)*g.stepX,
		g.minX + float64(x+1)*g.stepX,
		g.minZ + float64(z)*g.stepZ,
		g.minZ + float64(z+1)*g.stepZ
}

func sceneNodeProperty(node SceneNode, key string) any {
	if node.Properties == nil {
		return nil
	}
	return node.Properties[key]
}

func scenePropertyBool(node SceneNode, key string) (bool, bool) {
	value := sceneNodeProperty(node, key)
	if value == nil {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		}
	}
	return false, false
}

func scenePropertyString(node SceneNode, key string) string {
	value := sceneNodeProperty(node, key)
	if value == nil {
		return ""
	}
	text, _ := value.(string)
	return strings.ToLower(strings.TrimSpace(text))
}

func sceneActorMetrics(scene Scene, nodes map[string]SceneNode) (radius, height float64, known bool) {
	radius = -1
	height = -1
	for _, key := range []string{"actor_radius", "player_radius", "agent_radius"} {
		if value, ok := numberValue(scene.Metadata[key]); ok && value >= 0 {
			radius = value
			break
		}
	}
	for _, key := range []string{"actor_height", "player_height", "agent_height"} {
		if value, ok := numberValue(scene.Metadata[key]); ok && value > 0 {
			height = value
			break
		}
	}
	for _, node := range nodes {
		kind := strings.ToLower(strings.TrimSpace(node.Kind))
		if kind != "player" && kind != "actor" && kind != "agent" && kind != "avatar" && kind != "character" {
			continue
		}
		horizontal := math.Abs(node.Size[0])
		if scene.Dimension == "2d" {
			horizontal = math.Max(horizontal, math.Abs(node.Size[1]))
		} else {
			horizontal = math.Max(horizontal, math.Abs(node.Size[2]))
		}
		if radius < 0 {
			radius = horizontal / 2
		}
		if height < 0 && scene.Dimension == "3d" {
			height = math.Abs(node.Size[1])
		}
		known = true
	}
	if radius < 0 {
		// A bounded fallback keeps narrow passages conservative while making
		// the missing actor geometry explicit in diagnostics.
		radius = 0.25
		known = false
	}
	if height <= 0 {
		height = 1
		if scene.Dimension == "3d" {
			known = false
		}
	}
	return radius, height, known
}

func sceneNavigationHeight(scene Scene, starts []Vec3) (float64, bool) {
	if scene.Dimension == "2d" {
		return 0, true
	}
	for _, key := range []string{"navigation_height", "ground_height", "support_height"} {
		if value, ok := numberValue(scene.Metadata[key]); ok {
			return value, true
		}
	}
	if len(starts) > 0 {
		return starts[0][1], false
	}
	return scene.WorldBounds.Min[1], false
}

type rasterColliderInfo struct {
	center    Vec3
	extents   Vec3
	blocked   bool
	uncertain bool
	dynamic   bool
}

func sceneColliderInfo(scene Scene, nodes map[string]SceneNode, collider SceneCollider, supportHeight, actorHeight float64) rasterColliderInfo {
	node, ok := nodes[collider.NodeID]
	if !ok {
		return rasterColliderInfo{}
	}
	if enabled, hasValue := scenePropertyBool(node, "navigation_blocker"); hasValue && !enabled {
		return rasterColliderInfo{}
	}
	if solid, hasValue := scenePropertyBool(node, "solid"); hasValue && !solid {
		return rasterColliderInfo{}
	}
	navigation := scenePropertyString(node, "navigation")
	if navigation == "ignore" || navigation == "none" || navigation == "decorative" || navigation == "floor" || navigation == "ground" || navigation == "support" {
		return rasterColliderInfo{}
	}
	center := node.Position.Add(collider.Offset)
	extents := Vec3{math.Abs(collider.Extents[0]), math.Abs(collider.Extents[1]), math.Abs(collider.Extents[2])}
	if strings.EqualFold(collider.Shape, "circle") {
		radius := math.Max(extents[0], extents[1])
		extents[0], extents[1] = radius, radius
	}
	info := rasterColliderInfo{center: center, extents: extents, blocked: true}
	info.dynamic = scenePropertyString(node, "body_type") == "dynamic" || scenePropertyString(node, "motion") == "dynamic"
	if value, ok := scenePropertyBool(node, "dynamic"); ok {
		info.dynamic = info.dynamic || value
	}
	if strings.EqualFold(collider.Shape, "mesh") {
		info.uncertain = true
	}
	if scene.Dimension == "3d" {
		bottom := center[1] - extents[1]
		top := center[1] + extents[1]
		// A support surface below the actor is not a wall. Colliders above the
		// actor's vertical span likewise do not block ground traversal.
		if top <= supportHeight+1e-6 || bottom >= supportHeight+actorHeight+1e-6 {
			info.blocked = false
		}
	}
	return info
}

func rasterCellOverlaps(g sceneGrid, x, z int, info rasterColliderInfo, dimension string, actorRadius float64) bool {
	if !info.blocked {
		return false
	}
	minX, maxX, minZ, maxZ := g.cell(x, z)
	extX := info.extents[0] + actorRadius
	extZ := info.extents[1] + actorRadius
	if dimension == "3d" {
		extZ = info.extents[2] + actorRadius
	}
	axis := dimensionAxis(dimension, info.center)
	return info.center[0]+extX >= minX &&
		info.center[0]-extX <= maxX &&
		info.center[axis]+extZ >= minZ &&
		info.center[axis]-extZ <= maxZ
}

func dimensionAxis(dimension string, _ Vec3) int {
	if dimension == "3d" {
		return 2
	}
	return 1
}

func colliderContains(scene Scene, nodes map[string]SceneNode, collider SceneCollider, p Vec3) bool {
	radius, height, _ := sceneActorMetrics(scene, nodes)
	support, _ := sceneNavigationHeight(scene, []Vec3{p})
	info := sceneColliderInfo(scene, nodes, collider, support, height)
	if !info.blocked {
		return false
	}
	axis := dimensionAxis(scene.Dimension, p)
	return math.Abs(p[0]-info.center[0]) <= info.extents[0]+radius &&
		math.Abs(p[axis]-info.center[axis]) <= info.extents[axis]+radius
}

func sceneZoneIntersectsCollider(scene Scene, zone SceneZone, info rasterColliderInfo, actorRadius float64) (inside, intersects bool) {
	if !info.blocked {
		return false, false
	}
	axis := dimensionAxis(scene.Dimension, info.center)
	zoneMinAxis, zoneMaxAxis := zone.Bounds.Min[axis], zone.Bounds.Max[axis]
	extAxis := info.extents[axis] + actorRadius
	minX, maxX := info.center[0]-info.extents[0]-actorRadius, info.center[0]+info.extents[0]+actorRadius
	minAxis, maxAxis := info.center[axis]-extAxis, info.center[axis]+extAxis
	overlap := maxX >= zone.Bounds.Min[0] && minX <= zone.Bounds.Max[0] &&
		maxAxis >= zoneMinAxis && minAxis <= zoneMaxAxis
	if !overlap {
		return false, false
	}
	zoneCenter := sceneCenter(zone.Bounds)
	centerInside := math.Abs(zoneCenter[0]-info.center[0]) <= info.extents[0]+actorRadius &&
		math.Abs(zoneCenter[axis]-info.center[axis]) <= extAxis
	zoneInside := zone.Bounds.Min[0] >= minX && zone.Bounds.Max[0] <= maxX &&
		zoneMinAxis >= minAxis && zoneMaxAxis <= maxAxis
	return centerInside || zoneInside, true
}

func sceneRasterBarrierProven(scene Scene, info rasterColliderInfo, from, to Vec3, actorRadius float64) bool {
	if !info.blocked || info.uncertain || info.dynamic {
		return false
	}
	planeAxis := dimensionAxis(scene.Dimension, info.center)
	moveAxis := 0
	if math.Abs(to[planeAxis]-from[planeAxis]) > math.Abs(to[0]-from[0]) {
		moveAxis = planeAxis
	}
	orthAxis := planeAxis
	if moveAxis == planeAxis {
		orthAxis = 0
	}
	worldMin := scene.WorldBounds.Min[orthAxis]
	worldMax := scene.WorldBounds.Max[orthAxis]
	spanMin := info.center[orthAxis] - info.extents[orthAxis] - actorRadius
	spanMax := info.center[orthAxis] + info.extents[orthAxis] + actorRadius
	if spanMin > worldMin+1e-6 || spanMax < worldMax-1e-6 {
		return false
	}
	obstacleMin := info.center[moveAxis] - info.extents[moveAxis] - actorRadius
	obstacleMax := info.center[moveAxis] + info.extents[moveAxis] + actorRadius
	return (from[moveAxis] < obstacleMin-1e-6 && to[moveAxis] > obstacleMax+1e-6) ||
		(to[moveAxis] < obstacleMin-1e-6 && from[moveAxis] > obstacleMax+1e-6)
}

func rasterReachability(scene Scene, nodes map[string]SceneNode) []SceneDiagnostic {
	starts, goals := zoneCenters(scene)
	if len(starts) == 0 || len(goals) == 0 {
		return []SceneDiagnostic{sceneWarning("reachability", "bounded raster needs at least one spawn and one goal zone")}
	}
	actorRadius, actorHeight, actorKnown := sceneActorMetrics(scene, nodes)
	supportHeight, supportKnown := sceneNavigationHeight(scene, starts)
	diagnostics := []SceneDiagnostic{sceneWarning("reachability", "topdown/ground reachability uses a bounded raster and remains unverified at cell resolution")}
	if !actorKnown {
		diagnostics = append(diagnostics, sceneWarning("reachability", "actor clearance is inferred; provide metadata.actor_radius or a player-sized node for exact raster clearance"))
	}
	if scene.Dimension == "3d" && !supportKnown {
		diagnostics = append(diagnostics, sceneWarning("reachability", "3d ground height is inferred from the spawn zone; provide metadata.navigation_height for explicit support geometry"))
	}
	g := newSceneGrid(scene)
	infos := make([]rasterColliderInfo, 0, len(scene.Colliders))
	for _, collider := range scene.Colliders {
		info := sceneColliderInfo(scene, nodes, collider, supportHeight, actorHeight)
		infos = append(infos, info)
		if info.uncertain {
			diagnostics = append(diagnostics, sceneWarning("reachability", "mesh collider bounds are only an approximate raster blocker"))
		}
		if info.dynamic {
			diagnostics = append(diagnostics, sceneWarning("reachability", "dynamic collider geometry may change after the static raster check"))
		}
		for z := 0; z < g.h; z++ {
			for x := 0; x < g.w; x++ {
				if rasterCellOverlaps(g, x, z, info, scene.Dimension, actorRadius) {
					g.blocked[g.index(x, z)] = true
				}
			}
		}
	}
	nearest := func(p Vec3) (int, int) {
		x := int(math.Floor((p[0] - g.minX) / g.stepX))
		zAxis := p[1]
		if scene.Dimension == "3d" {
			zAxis = p[2]
		}
		z := int(math.Floor((zAxis - g.minZ) / g.stepZ))
		return maxInt(0, minInt(g.w-1, x)), maxInt(0, minInt(g.h-1, z))
	}
	for _, zone := range scene.Zones {
		if strings.EqualFold(zone.Kind, "spawn") || strings.EqualFold(zone.Kind, "goal") {
			for _, info := range infos {
				if inside, intersects := sceneZoneIntersectsCollider(scene, zone, info, actorRadius); inside {
					return append(diagnostics, sceneError("reachability", strings.ToLower(zone.Kind)+" zone is inside a collider"))
				} else if intersects {
					diagnostics = append(diagnostics, sceneWarning("reachability", strings.ToLower(zone.Kind)+" zone overlaps a collider; raster result is uncertain"))
				}
			}
		}
	}
	for _, start := range starts {
		sx, sz := nearest(start)
		if g.blocked[g.index(sx, sz)] {
			return append(diagnostics, sceneError("reachability", "spawn zone is inside a collider"))
		}
		seen := make([]bool, len(g.blocked))
		queue := [][2]int{{sx, sz}}
		seen[g.index(sx, sz)] = true
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, delta := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				x, z := current[0]+delta[0], current[1]+delta[1]
				if x < 0 || x >= g.w || z < 0 || z >= g.h || seen[g.index(x, z)] || g.blocked[g.index(x, z)] {
					continue
				}
				seen[g.index(x, z)] = true
				queue = append(queue, [2]int{x, z})
			}
		}
		for _, goal := range goals {
			gx, gz := nearest(goal)
			if g.blocked[g.index(gx, gz)] {
				return append(diagnostics, sceneError("reachability", "goal zone is inside a collider"))
			}
			if !seen[g.index(gx, gz)] {
				proven := false
				for _, info := range infos {
					if sceneRasterBarrierProven(scene, info, start, goal, actorRadius) {
						proven = true
						break
					}
				}
				if proven {
					return append(diagnostics, sceneError("reachability", "goal is unreachable through a proven full-span topdown/ground barrier"))
				}
				diagnostics = append(diagnostics, sceneWarning("reachability", "goal was not connected in the bounded raster; exact reachability remains unverified"))
			}
		}
	}
	return diagnostics
}

func scenePlatformHalfWidth(scene Scene, node SceneNode) float64 {
	half := sceneItemExtent(node)[0]
	for _, collider := range scene.Colliders {
		if collider.NodeID == node.ID {
			half = math.Max(half, math.Abs(collider.Extents[0]))
		}
	}
	return half
}

func platformReachability(scene Scene, nodes map[string]SceneNode) []SceneDiagnostic {
	var platforms []SceneNode
	for _, node := range nodes {
		if strings.EqualFold(node.Kind, "platform") || strings.EqualFold(node.Kind, "platforms") {
			platforms = append(platforms, node)
		}
	}
	if len(platforms) < 2 {
		return []SceneDiagnostic{sceneWarning("reachability", "platform navigation has fewer than two platforms to compare")}
	}
	sort.Slice(platforms, func(i, j int) bool { return platforms[i].Position[0] < platforms[j].Position[0] })
	// Keep static validation on the same nested movement contract used by the
	// platform generator (metadata.movement/mechanics/params are supported).
	maxJump, maxHeight := scenePlatformLimits(scene)
	diagnostics := []SceneDiagnostic{sceneWarning("reachability", "platform checks use a simple bounded ballistic gap estimate")}
	for i := 1; i < len(platforms); i++ {
		left, right := platforms[i-1], platforms[i]
		gap := right.Position[0] - left.Position[0] - scenePlatformHalfWidth(scene, left) - scenePlatformHalfWidth(scene, right)
		height := math.Abs(right.Position[1] - left.Position[1])
		if gap > maxJump || height > maxHeight {
			return append(diagnostics, sceneError("reachability", fmt.Sprintf("platform %q cannot reach %q (gap=%g,height=%g)", left.ID, right.ID, gap, height)))
		}
	}
	return diagnostics
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, finite(typed)
	case float32:
		return float64(typed), finite(float64(typed))
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
