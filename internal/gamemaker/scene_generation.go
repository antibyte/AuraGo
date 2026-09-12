package gamemaker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const maxGeneratedSceneItems = SceneMaxItems

func cloneScene(scene Scene) (Scene, error) {
	data, err := json.Marshal(scene)
	if err != nil {
		return Scene{}, fmt.Errorf("clone scene: %w", err)
	}
	var copy Scene
	if err := json.Unmarshal(data, &copy); err != nil {
		return Scene{}, fmt.Errorf("clone scene: %w", err)
	}
	return copy, nil
}

func emptyBounds(bounds SceneBounds) bool {
	return bounds.Min == (Vec3{}) && bounds.Max == (Vec3{})
}

func activeLevel(scene Scene) string {
	for _, level := range scene.Levels {
		if level.Active {
			return level.ID
		}
	}
	return ""
}

func stableSceneID(prefix, region string, seed int64, index int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d\x00%d", prefix, region, seed, index)))
	return prefix + "_" + hex.EncodeToString(h[:8])
}
func stableSceneChildID(prefix, parent string) string {
	h := sha256.Sum256([]byte(prefix + "\x00" + parent))
	return prefix + "_" + hex.EncodeToString(h[:8])
}

func sceneRandom(seed int64, index uint64) float64 {
	x := uint64(seed) + index + 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) / float64(uint64(1)<<53)
}

func sceneCenter(bounds SceneBounds) Vec3 {
	return Vec3{(bounds.Min[0] + bounds.Max[0]) / 2, (bounds.Min[1] + bounds.Max[1]) / 2, (bounds.Min[2] + bounds.Max[2]) / 2}
}

func sceneSize(bounds SceneBounds) Vec3 {
	return Vec3{bounds.Max[0] - bounds.Min[0], bounds.Max[1] - bounds.Min[1], bounds.Max[2] - bounds.Min[2]}
}

func scenePoint(bounds SceneBounds, fx, fy, fz float64, dimension string) Vec3 {
	p := Vec3{bounds.Min[0] + (bounds.Max[0]-bounds.Min[0])*fx, bounds.Min[1] + (bounds.Max[1]-bounds.Min[1])*fy, bounds.Min[2] + (bounds.Max[2]-bounds.Min[2])*fz}
	if dimension == "2d" {
		p[2] = 0
	}
	return p
}

// sceneGroundPoint keeps the 3D generator on the X/Z ground plane. The Y
// coordinate is height in 3D; using the second normalized value as Z avoids
// placing a ground grid in a vertical X/Y slice.
func sceneGroundPoint(bounds SceneBounds, fx, fz float64, dimension string) Vec3 {
	if dimension == "3d" {
		return scenePoint(bounds, fx, .5, fz, dimension)
	}
	return scenePoint(bounds, fx, fz, 0, dimension)
}

func sceneGridAxes(bounds SceneBounds, dimension string) (width, depth float64) {
	size := sceneSize(bounds)
	width, depth = size[0], size[1]
	if dimension == "3d" {
		depth = size[2]
	}
	return math.Max(width, 1e-6), math.Max(depth, 1e-6)
}

// sceneGridLayout returns a bounded grid whose cell width/depth respect the
// requested CellSize when supplied, while still providing enough cells for
// the requested item count.
func sceneGridLayout(bounds SceneBounds, dimension string, count int, cellSize float64) (columns, rows int) {
	width, depth := sceneGridAxes(bounds, dimension)
	columns = int(math.Ceil(math.Sqrt(float64(maxInt(count, 1)))))
	rows = (maxInt(count, 1) + columns - 1) / columns
	if finite(cellSize) && cellSize > 0 {
		columns = maxInt(columns, int(math.Ceil(width/cellSize)))
		rows = maxInt(rows, int(math.Ceil(depth/cellSize)))
	}
	columns = minInt(maxInt(columns, 1), maxGeneratedSceneItems)
	rows = minInt(maxInt(rows, 1), maxGeneratedSceneItems)
	return columns, rows
}

func sceneCorridorCorner(from, to Vec3, dimension string) Vec3 {
	if dimension == "3d" {
		return Vec3{to[0], from[1], from[2]}
	}
	return Vec3{to[0], from[1], 0}
}
func sceneGridNodeSize(bounds SceneBounds, dimension string, columns, rows int) Vec3 {
	width, depth := sceneGridAxes(bounds, dimension)
	size := sceneSize(bounds)
	if dimension == "3d" {
		return Vec3{width / float64(columns), math.Min(1, size[1]), depth / float64(rows)}
	}
	return Vec3{width / float64(columns), depth / float64(rows), 0}
}

func sceneCount(requested, fallback int) int {
	if requested <= 0 {
		requested = fallback
	}
	if requested > maxGeneratedSceneItems {
		return maxGeneratedSceneItems + 1
	}
	return requested
}

func sceneAssetSize(assets AssetCatalog, assetID, dimension string) (Vec3, bool) {
	asset, ok := assets.Assets[strings.TrimSpace(assetID)]
	if !ok || !sceneAssetBoundsAvailable(asset.Bounds) {
		return Vec3{}, false
	}
	size := Vec3{
		asset.Bounds.Max[0] - asset.Bounds.Min[0],
		asset.Bounds.Max[1] - asset.Bounds.Min[1],
		asset.Bounds.Max[2] - asset.Bounds.Min[2],
	}
	if dimension == "2d" {
		size[2] = 0
	}
	return size, true
}

func sceneGeneratedNodeSize(size Vec3, dimension string, assets AssetCatalog, assetID string) Vec3 {
	if assetSize, ok := sceneAssetSize(assets, assetID, dimension); ok {
		size = assetSize
	}
	if dimension == "2d" {
		size[2] = 0
	}
	return size
}

func sceneClampCenter(value, minValue, maxValue, half float64) float64 {
	low, high := minValue+half, maxValue-half
	if low > high {
		return (minValue + maxValue) / 2
	}
	return math.Max(low, math.Min(high, value))
}

func sceneBoundsOverlapWithClearance(a, b SceneBounds, clearance float64, dimension string) bool {
	axes := 3
	if dimension == "2d" {
		axes = 2
	}
	for axis := 0; axis < axes; axis++ {
		if a.Max[axis]+clearance < b.Min[axis] || b.Max[axis]+clearance < a.Min[axis] {
			return false
		}
	}
	return true
}

func sceneNodeClear(scene Scene, p Vec3, clearance float64, assets AssetCatalog, region SceneBounds, candidateAssetID ...string) bool {
	size := Vec3{math.Max(clearance, .1), math.Max(clearance, .1), math.Max(clearance, 0)}
	assetID := ""
	if len(candidateAssetID) > 0 {
		assetID = candidateAssetID[0]
	}
	size = sceneGeneratedNodeSize(size, scene.Dimension, assets, assetID)
	candidate := SceneNode{ID: "__candidate__", Position: p, Size: size}
	placements := []ScenePlacement(nil)
	if assetID != "" {
		placements = []ScenePlacement{{NodeID: candidate.ID, AssetID: assetID, Position: p.Sub(sceneCenter(assets.Assets[assetID].Bounds)), Scale: Vec3{1, 1, 1}}}
	}
	candidateBounds := sceneNodeBounds(candidate, placements, nil, assets, scene.Dimension)
	if !boundsInBounds(candidateBounds, region) {
		return false
	}
	for _, node := range scene.Nodes {
		existingBounds := sceneNodeBounds(node, scene.Placements, scene.Colliders, assets, scene.Dimension)
		if sceneBoundsOverlapWithClearance(candidateBounds, existingBounds, math.Max(clearance, 0), scene.Dimension) {
			return false
		}
	}
	return true
}

func appendSceneNode(scene *Scene, node SceneNode, req GenerateSceneRegionRequest) {
	node.Size = sceneGeneratedNodeSize(node.Size, scene.Dimension, req.Assets, req.AssetID)
	for _, existing := range scene.Nodes {
		if existing.ID == node.ID {
			return
		}
	}
	scene.Nodes = append(scene.Nodes, node)
	if strings.TrimSpace(req.AssetRole) != "" {
		behavior := strings.TrimSpace(req.Behavior)
		if behavior == "" {
			behavior = "decorative"
		}
		placement := ScenePlacement{ID: stableSceneChildID("placement", node.ID), NodeID: node.ID, AssetID: req.AssetID, AssetRole: req.AssetRole, Behavior: behavior, Position: node.Position, Scale: Vec3{1, 1, 1}, LevelID: node.LevelID, RegionID: req.RegionID}
		if asset, ok := req.Assets.Assets[req.AssetID]; ok && sceneAssetBoundsAvailable(asset.Bounds) {
			placement.Position = node.Position.Sub(sceneCenter(asset.Bounds))
			if scene.Dimension == "2d" {
				placement.Position[2] = 0
			}
		}
		for _, existing := range scene.Placements {
			if existing.NodeID == placement.NodeID {
				return
			}
		}
		scene.Placements = append(scene.Placements, placement)
	}
}

func appendSceneZone(scene *Scene, zone SceneZone) {
	for _, existing := range scene.Zones {
		if existing.ID == zone.ID {
			return
		}
	}
	scene.Zones = append(scene.Zones, zone)
}

func appendSceneRoute(scene *Scene, route SceneRoute) {
	for _, existing := range scene.Routes {
		if existing.ID == route.ID {
			return
		}
	}
	scene.Routes = append(scene.Routes, route)
}

func upsertSceneCollider(scene *Scene, collider SceneCollider) {
	for i, existing := range scene.Colliders {
		if existing.ID == collider.ID ||
			existing.NodeID == collider.NodeID && existing.RegionID == collider.RegionID {
			scene.Colliders[i] = collider
			return
		}
	}
	scene.Colliders = append(scene.Colliders, collider)
}

func removeSceneRegion(scene *Scene, regionID string) {
	pinnedNodes := map[string]bool{}
	removedNodes := map[string]bool{}
	keepNodes := scene.Nodes[:0]
	for _, item := range scene.Nodes {
		if item.RegionID == regionID && !item.Pinned {
			removedNodes[item.ID] = true
			continue
		}
		keepNodes = append(keepNodes, item)
		if item.RegionID == regionID && item.Pinned {
			pinnedNodes[item.ID] = true
		}
	}
	scene.Nodes = keepNodes
	keepRegions := scene.Regions[:0]
	for _, item := range scene.Regions {
		if item.ID != regionID || item.Pinned {
			keepRegions = append(keepRegions, item)
		}
	}
	scene.Regions = keepRegions
	scene.Placements = filterSceneRegionPlacements(scene.Placements, removedNodes, pinnedNodes)
	scene.Colliders = filterSceneRegionColliders(scene.Colliders, removedNodes, pinnedNodes)
	scene.Attachments = filterSceneRegionAttachments(scene.Attachments, removedNodes, pinnedNodes)
	scene.Zones = filterSceneRegionZones(scene.Zones, regionID)
	scene.Routes = filterSceneRegionRoutes(scene.Routes, removedNodes)
}

func filterSceneRegionPlacements(items []ScenePlacement, removedNodes, pinnedNodes map[string]bool) []ScenePlacement {
	keep := items[:0]
	for _, item := range items {
		if removedNodes[item.NodeID] && !pinnedNodes[item.NodeID] {
			continue
		}
		keep = append(keep, item)
	}
	return keep
}
func filterSceneRegionColliders(items []SceneCollider, removedNodes, pinnedNodes map[string]bool) []SceneCollider {
	keep := items[:0]
	for _, item := range items {
		if removedNodes[item.NodeID] && !pinnedNodes[item.NodeID] {
			continue
		}
		keep = append(keep, item)
	}
	return keep
}
func filterSceneRegionAttachments(items []SceneAttachment, removedNodes, pinnedNodes map[string]bool) []SceneAttachment {
	keep := items[:0]
	for _, item := range items {
		if removedNodes[item.NodeID] && !pinnedNodes[item.NodeID] {
			continue
		}
		keep = append(keep, item)
	}
	return keep
}
func filterSceneRegionZones(items []SceneZone, regionID string) []SceneZone {
	keep := items[:0]
	for _, item := range items {
		if item.RegionID != regionID {
			keep = append(keep, item)
		}
	}
	return keep
}
func filterSceneRegionRoutes(items []SceneRoute, removedNodes map[string]bool) []SceneRoute {
	keep := items[:0]
	for _, item := range items {
		// Keep cross-region routes whose endpoints remain. Any route touching a
		// removed node is invalid regardless of its own region tag.
		if removedNodes[item.From] || removedNodes[item.To] {
			continue
		}
		keep = append(keep, item)
	}
	return keep
}

func sceneLookupNumber(value any, keys []string, depth int) (float64, bool) {
	if depth > 6 || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if parsed, ok := numberValue(typed[key]); ok {
				return parsed, true
			}
		}
		for _, key := range []string{"movement", "mechanics", "params", "blocks"} {
			if parsed, ok := sceneLookupNumber(typed[key], keys, depth+1); ok {
				return parsed, true
			}
		}
	case []any:
		for _, item := range typed {
			if parsed, ok := sceneLookupNumber(item, keys, depth+1); ok {
				return parsed, true
			}
		}
	case string:
		text := strings.TrimSpace(typed)
		if text == "" || (text[0] != '{' && text[0] != '[') {
			return 0, false
		}
		var decoded any
		if json.Unmarshal([]byte(text), &decoded) == nil {
			return sceneLookupNumber(decoded, keys, depth+1)
		}
	}
	return 0, false
}

func sceneMovementLimit(scene Scene, aliases []string, fallback float64) float64 {
	if parsed, ok := sceneLookupNumber(scene.Metadata, aliases, 0); ok && finite(parsed) && parsed >= 0 {
		return parsed
	}
	return fallback
}

func scenePlatformLimits(scene Scene) (maxJump, maxHeight float64) {
	defaultJump, defaultHeight := 4.0, 3.0
	if scene.Dimension == "2d" {
		speed := sceneMovementLimit(scene, []string{"speed"}, 240)
		jump := sceneMovementLimit(scene, []string{"jump_speed"}, 420)
		gravity := sceneMovementLimit(scene, []string{"gravity"}, 900)
		if gravity <= 0 {
			gravity = 900
		}
		// Same speed/jump/gravity as common.ts; reserve a landing margin.
		defaultJump, defaultHeight = .85*speed*2*jump/gravity, .85*jump*jump/(2*gravity)
	}
	maxJump = sceneMovementLimit(scene, []string{"max_jump", "max_jump_distance", "jump_distance", "horizontal_jump"}, defaultJump)
	maxHeight = sceneMovementLimit(scene, []string{"max_jump_height", "jump_height", "vertical_jump"}, defaultHeight)
	return math.Max(maxJump, .1), math.Max(maxHeight, .1)
}
func sceneSegmentIntersectsBounds(from, to Vec3, bounds SceneBounds, dimension string) bool {
	axes := 3
	if dimension == "2d" {
		axes = 2
	}
	tMin, tMax := 0.0, 1.0
	for axis := 0; axis < axes; axis++ {
		delta := to[axis] - from[axis]
		if math.Abs(delta) <= 1e-9 {
			if from[axis] < bounds.Min[axis] || from[axis] > bounds.Max[axis] {
				return false
			}
			continue
		}
		enter := (bounds.Min[axis] - from[axis]) / delta
		exit := (bounds.Max[axis] - from[axis]) / delta
		if enter > exit {
			enter, exit = exit, enter
		}
		tMin = math.Max(tMin, enter)
		tMax = math.Min(tMax, exit)
		if tMin > tMax {
			return false
		}
	}
	return true
}

func sceneNodeMap(scene Scene) map[string]SceneNode {
	nodes := make(map[string]SceneNode, len(scene.Nodes))
	for _, node := range scene.Nodes {
		nodes[node.ID] = node
	}
	return nodes
}

func sceneRouteClear(scene Scene, nodes map[string]SceneNode, points []Vec3, ignoredNodes ...string) bool {
	if len(points) < 2 {
		return true
	}
	ignored := make(map[string]bool, len(ignoredNodes))
	for _, id := range ignoredNodes {
		ignored[id] = true
	}
	actorRadius, actorHeight, _ := sceneActorMetrics(scene, nodes)
	supportHeight, _ := sceneNavigationHeight(scene, nil)
	for _, collider := range scene.Colliders {
		if ignored[collider.NodeID] {
			continue
		}
		info := sceneColliderInfo(scene, nodes, collider, supportHeight, actorHeight)
		if !info.blocked {
			continue
		}
		pad := Vec3{actorRadius, actorRadius, actorRadius}
		if scene.Dimension == "3d" {
			pad[1] = math.Max(actorRadius, actorHeight/2)
		}
		blockedBounds := sceneBoundsFromCenterExtent(info.center, Vec3{
			info.extents[0] + pad[0], info.extents[1] + pad[1], info.extents[2] + pad[2],
		})
		for i := 1; i < len(points); i++ {
			if sceneSegmentIntersectsBounds(points[i-1], points[i], blockedBounds, scene.Dimension) {
				return false
			}
		}
	}
	return true
}

func appendSceneRouteIfClear(scene *Scene, nodes map[string]SceneNode, route SceneRoute) {
	if sceneRouteClear(*scene, nodes, route.Waypoints, route.From, route.To) {
		appendSceneRoute(scene, route)
	}
}

func nearestSceneNode(nodes map[string]SceneNode, zone SceneZone) SceneNode {
	id := nearestNode(nodes, zone)
	return nodes[id]
}

func nearestRoomNode(rooms []SceneNode, zone SceneZone) SceneNode {
	center := sceneCenter(zone.Bounds)
	best := SceneNode{}
	bestDistance := math.Inf(1)
	for _, node := range rooms {
		distance := (node.Position[0]-center[0])*(node.Position[0]-center[0]) +
			(node.Position[1]-center[1])*(node.Position[1]-center[1]) +
			(node.Position[2]-center[2])*(node.Position[2]-center[2])
		if distance < bestDistance || distance == bestDistance && node.ID < best.ID {
			best, bestDistance = node, distance
		}
	}
	return best
}

func appendRoomEndpointRoutes(scene *Scene, regionID, levelID string, seed int64, rooms []SceneNode) {
	if !strings.EqualFold(scene.Navigation, "waypoints") || len(rooms) == 0 {
		return
	}
	roomIDs := make(map[string]bool, len(rooms))
	for _, room := range rooms {
		roomIDs[room.ID] = true
	}
	anchors := make(map[string]SceneNode)
	for _, node := range scene.Nodes {
		if !roomIDs[node.ID] {
			anchors[node.ID] = node
		}
	}
	for index, zone := range scene.Zones {
		kind := strings.ToLower(strings.TrimSpace(zone.Kind))
		if kind != "spawn" && kind != "goal" {
			continue
		}
		anchor := nearestSceneNode(anchors, zone)
		room := nearestRoomNode(rooms, zone)
		if anchor.ID == "" || room.ID == "" || anchor.ID == room.ID {
			continue
		}
		points := []Vec3{anchor.Position, sceneCorridorCorner(anchor.Position, room.Position, scene.Dimension), room.Position}
		route := SceneRoute{
			ID:   stableSceneID("connector", regionID+"\x00"+zone.ID, seed, index),
			From: anchor.ID, To: room.ID, Waypoints: points, Kind: "undirected",
			LevelID: levelID, RegionID: regionID,
		}
		appendSceneRouteIfClear(scene, sceneNodeMap(*scene), route)
	}
}

func resolveSceneAssetID(req GenerateSceneRegionRequest) string {
	if strings.TrimSpace(req.AssetID) != "" || strings.TrimSpace(req.AssetRole) == "" {
		return req.AssetID
	}
	ids := make([]string, 0, len(req.Assets.Assets))
	for id, asset := range req.Assets.Assets {
		if strings.TrimSpace(id) == "" || !containsString(asset.Roles, req.AssetRole) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) > 0 {
		return ids[0]
	}
	return req.AssetID
}

// GenerateSceneRegion deterministically regenerates one bounded region. IDs
// depend only on region, seed, kind and index, so repeated runs are stable.
func GenerateSceneRegion(scene Scene, req GenerateSceneRegionRequest) (Scene, error) {
	// A generator may repair an unfinished route graph. Structural errors remain
	// fatal, and the complete generated scene is validated before returning.
	for _, diagnostic := range ValidateScene(scene, AssetCatalog{}) {
		if diagnostic.Severity == "error" && diagnostic.Path != "reachability" {
			return Scene{}, fmt.Errorf("scene_invalid: %s: %s", diagnostic.Path, diagnostic.Message)
		}
	}
	req.RegionID = strings.TrimSpace(req.RegionID)
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	if !sceneID(req.RegionID) {
		return Scene{}, fmt.Errorf("scene region id must be a stable identifier")
	}
	if !containsString([]string{"grid", "rooms", "path", "scatter", "platforms", "zones"}, req.Kind) {
		return Scene{}, fmt.Errorf("scene generator kind must be grid, rooms, path, scatter, platforms, or zones")
	}
	req.AssetID = resolveSceneAssetID(req)
	if req.Seed == 0 {
		req.Seed = scene.Seed
	}
	if emptyBounds(req.Bounds) {
		for _, region := range scene.Regions {
			if region.ID == req.RegionID {
				req.Bounds = region.Bounds
				break
			}
		}
	}
	if !validBounds(req.Bounds) || !boundsInBounds(req.Bounds, scene.WorldBounds) || (req.Bounds.Min == req.Bounds.Max) {
		return Scene{}, fmt.Errorf("scene region bounds must be finite, non-empty, and inside world_bounds")
	}
	if req.LevelID == "" {
		req.LevelID = activeLevel(scene)
	}
	levelFound := false
	for _, level := range scene.Levels {
		levelFound = levelFound || level.ID == req.LevelID
	}
	if !levelFound {
		return Scene{}, fmt.Errorf("scene region references unknown level %q", req.LevelID)
	}
	for _, region := range scene.Regions {
		if region.ID == req.RegionID && region.Pinned {
			return Scene{}, fmt.Errorf("scene region %s is pinned; unpin it before regeneration", req.RegionID)
		}
	}
	count := sceneCount(req.Density, 16)
	if count > maxGeneratedSceneItems {
		return Scene{}, fmt.Errorf("scene generator is limited to %d items", maxGeneratedSceneItems)
	}
	out, err := cloneScene(scene)
	if err != nil {
		return Scene{}, err
	}
	if req.Clear {
		removeSceneRegion(&out, req.RegionID)
	}
	regionExists := false
	for _, region := range out.Regions {
		if region.ID == req.RegionID {
			regionExists = true
		}
	}
	if !regionExists {
		out.Regions = append(out.Regions, SceneRegion{ID: req.RegionID, Kind: req.Kind, Bounds: req.Bounds, Seed: req.Seed, LevelID: req.LevelID, Generator: req.Kind})
	}
	if req.NodeKind == "" {
		req.NodeKind = req.Kind
	}
	if req.Behavior == "" {
		req.Behavior = "decorative"
	}
	addNode := func(index int, position, size Vec3) SceneNode {
		if scene.Dimension == "2d" {
			position[2] = 0
			size[2] = 0
		}
		size = sceneGeneratedNodeSize(size, scene.Dimension, req.Assets, req.AssetID)
		if scene.Dimension == "3d" && (req.Kind == "grid" || req.Kind == "rooms" || req.Kind == "scatter") {
			position[1] = req.Bounds.Min[1] + size[1]/2
		}
		for axis := 0; axis < 3; axis++ {
			position[axis] = sceneClampCenter(position[axis], req.Bounds.Min[axis], req.Bounds.Max[axis], size[axis]/2)
		}
		node := SceneNode{ID: stableSceneID("node", req.RegionID, req.Seed, index), Kind: req.NodeKind, Position: position, Size: size, LevelID: req.LevelID, RegionID: req.RegionID}
		appendSceneNode(&out, node, req)
		for _, existing := range out.Nodes {
			if existing.ID == node.ID {
				return existing
			}
		}
		return node
	}
	size := sceneSize(req.Bounds)
	switch req.Kind {
	case "grid":
		columns, rows := sceneGridLayout(req.Bounds, scene.Dimension, count, req.CellSize)
		cell := sceneGridNodeSize(req.Bounds, scene.Dimension, columns, rows)
		if assetSize, ok := sceneAssetSize(req.Assets, req.AssetID, scene.Dimension); ok {
			depthAxis := 1
			if scene.Dimension == "3d" {
				depthAxis = 2
			}
			if assetSize[0] > cell[0] || assetSize[depthAxis] > cell[depthAxis] {
				return Scene{}, fmt.Errorf("asset footprint exceeds grid cell; reduce density or enlarge the region")
			}
		}
		for i := 0; i < count; i++ {
			x, z := i%columns, i/columns
			fx, fz := (float64(x)+.5)/float64(columns), (float64(z)+.5)/float64(rows)
			addNode(i, sceneGroundPoint(req.Bounds, fx, fz, scene.Dimension), cell)
		}
	case "rooms":
		columns, rows := sceneGridLayout(req.Bounds, scene.Dimension, count, req.CellSize)
		cell := sceneGridNodeSize(req.Bounds, scene.Dimension, columns, rows)
		if assetSize, ok := sceneAssetSize(req.Assets, req.AssetID, scene.Dimension); ok {
			depthAxis := 1
			if scene.Dimension == "3d" {
				depthAxis = 2
			}
			if assetSize[0] > cell[0] || assetSize[depthAxis] > cell[depthAxis] {
				return Scene{}, fmt.Errorf("asset footprint exceeds grid cell; reduce density or enlarge the region")
			}
		}
		roomNodes := make([]SceneNode, count)
		nodesByID := sceneNodeMap(out)
		for i := 0; i < count; i++ {
			x, z := i%columns, i/columns
			roomSize := Vec3{cell[0] * .8, cell[1], cell[2] * .8}
			room := addNode(i, sceneGroundPoint(req.Bounds, (float64(x)+.5)/float64(columns), (float64(z)+.5)/float64(rows), scene.Dimension), roomSize)
			roomNodes[i] = room
			nodesByID[room.ID] = room
			if i == 0 {
				continue
			}
			from := i - 1
			if x == 0 && z > 0 {
				from = i - columns
			}
			appendSceneRouteIfClear(&out, nodesByID, SceneRoute{
				ID: stableSceneID("route", req.RegionID, req.Seed, i-1), From: roomNodes[from].ID, To: room.ID,
				Waypoints: []Vec3{roomNodes[from].Position, sceneCorridorCorner(roomNodes[from].Position, room.Position, scene.Dimension), room.Position},
				Kind:      "undirected", LevelID: req.LevelID, RegionID: req.RegionID,
			})
		}
		appendRoomEndpointRoutes(&out, req.RegionID, req.LevelID, req.Seed, roomNodes)
	case "path":
		if count < 2 {
			count = 2
		}
		var points []Vec3
		for i := 0; i < count; i++ {
			point := scenePoint(req.Bounds, float64(i)/float64(count-1), .5+.12*math.Sin(float64(i)), .5, scene.Dimension)
			node := addNode(i, point, Vec3{math.Max(size[0]/float64(count), .1), math.Max(size[1]*.08, .1), math.Max(size[2]*.08, 0)})
			point = node.Position
			points = append(points, point)
			if i > 0 {
				appendSceneRoute(&out, SceneRoute{ID: stableSceneID("route", req.RegionID, req.Seed, i-1), From: stableSceneID("node", req.RegionID, req.Seed, i-1), To: stableSceneID("node", req.RegionID, req.Seed, i), Waypoints: []Vec3{points[i-1], point}, Kind: "undirected", LevelID: req.LevelID, RegionID: req.RegionID})
			}
		}
	case "scatter":
		clearance := req.MinDistance
		if clearance <= 0 {
			clearance = math.Max(size[0], math.Max(size[1], size[2])) / math.Sqrt(float64(count)*8)
		}
		for i, attempt := 0, 0; i < count && attempt < count*8; attempt++ {
			p := scenePoint(req.Bounds, .05+.9*sceneRandom(req.Seed, uint64(attempt*3)), .05+.9*sceneRandom(req.Seed, uint64(attempt*3+1)), .5+.45*sceneRandom(req.Seed, uint64(attempt*3+2)), scene.Dimension)
			if scene.Dimension == "3d" {
				candidateSize := sceneGeneratedNodeSize(Vec3{clearance, clearance, clearance}, scene.Dimension, req.Assets, req.AssetID)
				p[1] = req.Bounds.Min[1] + candidateSize[1]/2
			}
			if !sceneNodeClear(out, p, clearance, req.Assets, req.Bounds, req.AssetID) {
				continue
			}
			addNode(i, p, Vec3{math.Max(clearance, .1), math.Max(clearance, .1), math.Max(clearance, 0)})
			i++
		}
	case "platforms":
		if count < 2 {
			count = 2
		}
		maxJump, maxHeight := scenePlatformLimits(scene)
		marginX := math.Min(size[0]*.08, maxJump*.25)
		if marginX <= 0 {
			marginX = math.Min(size[0]*.08, .1)
		}
		span := math.Max(size[0]-2*marginX, 0)
		if count > 1 {
			span = math.Min(span, maxJump*float64(count-1))
		}
		startX := req.Bounds.Min[0] + marginX
		verticalHalf := math.Max(size[1]*.03, .075)
		minY := req.Bounds.Min[1] + verticalHalf
		maxY := req.Bounds.Max[1] - verticalHalf
		if maxY < minY {
			minY, maxY = req.Bounds.Min[1], req.Bounds.Max[1]
		}
		var points []Vec3
		for i := 0; i < count; i++ {
			step := span / float64(maxInt(count-1, 1))
			x := startX + step*float64(i)
			y := (minY + maxY) / 2
			if i == 0 {
				y = minY + (maxY-minY)*(.35+.3*sceneRandom(req.Seed, uint64(i)))
			} else {
				delta := (sceneRandom(req.Seed, uint64(i*2+1)) - .5) * 2 * math.Min(maxHeight, math.Max(size[1]*.25, .1))
				y = points[i-1][1] + delta
				if y < minY {
					y = minY
				} else if y > maxY {
					y = maxY
				}
			}
			p := Vec3{x, y, 0}
			if scene.Dimension == "3d" {
				p[2] = req.Bounds.Min[2] + size[2]/2
			}
			platformStep := math.Max(step, .1)
			platformSize := Vec3{math.Max(math.Min(size[0]/float64(count)*.72, platformStep*.6), .2), math.Max(size[1]*.06, .15), math.Max(size[2]*.18, .15)}
			platformSize = sceneGeneratedNodeSize(platformSize, scene.Dimension, req.Assets, req.AssetID)
			if scene.Dimension == "2d" {
				platformSize[2] = 0
			}
			p[0] = sceneClampCenter(p[0], req.Bounds.Min[0], req.Bounds.Max[0], platformSize[0]/2)
			p[1] = sceneClampCenter(p[1], req.Bounds.Min[1], req.Bounds.Max[1], platformSize[1]/2)
			if scene.Dimension == "3d" {
				p[2] = sceneClampCenter(p[2], req.Bounds.Min[2], req.Bounds.Max[2], platformSize[2]/2)
			}
			node := addNode(i, p, platformSize)
			platformSize = node.Size
			p = node.Position
			points = append(points, p)
			if !node.Pinned {
				upsertSceneCollider(&out, SceneCollider{ID: stableSceneID("collider", req.RegionID, req.Seed, i), NodeID: node.ID, Shape: "box", Extents: Vec3{platformSize[0] / 2, platformSize[1] / 2, platformSize[2] / 2}, LevelID: req.LevelID, RegionID: req.RegionID})
			}
			if i > 0 {
				appendSceneRoute(&out, SceneRoute{ID: stableSceneID("route", req.RegionID, req.Seed, i-1), From: stableSceneID("node", req.RegionID, req.Seed, i-1), To: node.ID, Waypoints: []Vec3{points[i-1], p}, Kind: "undirected", LevelID: req.LevelID, RegionID: req.RegionID})
			}
		}
	case "zones":
		zoneKind := strings.ToLower(strings.TrimSpace(req.ZoneKind))
		if zoneKind != "" && !containsString([]string{"spawn", "goal", "loss", "trigger"}, zoneKind) {
			return Scene{}, fmt.Errorf("scene zone kind must be spawn, goal, loss, or trigger")
		}
		kinds := []string{"spawn", "goal", "trigger"}
		if zoneKind != "" {
			kinds = []string{zoneKind}
		}
		for i, kind := range kinds {
			fx := .15
			if kind == "goal" {
				fx = .85
			} else if kind == "trigger" {
				fx = .5
			}
			center := scenePoint(req.Bounds, fx, .5, .5, scene.Dimension)
			half := Vec3{math.Max(size[0]*.05, .1), math.Max(size[1]*.05, .1), math.Max(size[2]*.05, 0)}
			if scene.Dimension == "2d" {
				half[2] = 0
			}
			appendSceneZone(&out, SceneZone{ID: stableSceneID("zone", req.RegionID, req.Seed, i), Kind: kind, Bounds: SceneBounds{Min: center.Sub(half), Max: center.Add(half)}, LevelID: req.LevelID, RegionID: req.RegionID})
		}
	}
	sort.SliceStable(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	if err := validateScene(out, req.Assets); err != nil {
		return Scene{}, err
	}
	return out, nil
}

func (v Vec3) Add(other Vec3) Vec3 { return Vec3{v[0] + other[0], v[1] + other[1], v[2] + other[2]} }
func (v Vec3) Sub(other Vec3) Vec3 { return Vec3{v[0] - other[0], v[1] - other[1], v[2] - other[2]} }

func upsertSceneNodes(base []SceneNode, values []SceneNode) []SceneNode {
	for _, value := range values {
		found := false
		for i := range base {
			if base[i].ID == value.ID {
				base[i], found = value, true
				break
			}
		}
		if !found {
			base = append(base, value)
		}
	}
	return base
}

func removeSceneIDs[T any](base []T, ids []string, id func(T) string) []T {
	if len(ids) == 0 {
		return base
	}
	remove := map[string]bool{}
	for _, value := range ids {
		remove[value] = true
	}
	keep := base[:0]
	for _, value := range base {
		if !remove[id(value)] {
			keep = append(keep, value)
		}
	}
	return keep
}

func applyScenePatch(scene Scene, patch ScenePatch) (Scene, error) {
	if patch.Replace != nil {
		var err error
		scene, err = cloneScene(*patch.Replace)
		if err != nil {
			return Scene{}, err
		}
	}
	if patch.SchemaVersion != nil {
		scene.SchemaVersion = *patch.SchemaVersion
	}
	if patch.Dimension != nil {
		scene.Dimension = *patch.Dimension
	}
	if patch.Seed != nil {
		scene.Seed = *patch.Seed
	}
	if patch.Navigation != nil {
		scene.Navigation = *patch.Navigation
	}
	if patch.WorldBounds != nil {
		scene.WorldBounds = *patch.WorldBounds
	}
	if patch.CameraBounds != nil {
		scene.CameraBounds = patch.CameraBounds
	}
	if patch.ClearCameraBounds {
		scene.CameraBounds = nil
	}
	if patch.Levels != nil {
		scene.Levels = append([]SceneLevel(nil), (*patch.Levels)...)
	}
	scene.Nodes = upsertSceneNodes(scene.Nodes, patch.Nodes)
	scene.Nodes = removeSceneIDs(scene.Nodes, patch.RemoveNodeIDs, func(v SceneNode) string { return v.ID })
	scene.Regions = upsertSceneRegions(scene.Regions, patch.Regions)
	scene.Regions = removeSceneIDs(scene.Regions, patch.RemoveRegionIDs, func(v SceneRegion) string { return v.ID })
	scene.Placements = upsertScenePlacements(scene.Placements, patch.Placements)
	scene.Placements = removeSceneIDs(scene.Placements, patch.RemovePlacementIDs, func(v ScenePlacement) string { return v.ID })
	scene.Colliders = upsertSceneColliders(scene.Colliders, patch.Colliders)
	scene.Colliders = removeSceneIDs(scene.Colliders, patch.RemoveColliderIDs, func(v SceneCollider) string { return v.ID })
	scene.Attachments = upsertSceneAttachments(scene.Attachments, patch.Attachments)
	scene.Attachments = removeSceneIDs(scene.Attachments, patch.RemoveAttachmentIDs, func(v SceneAttachment) string { return v.ID })
	scene.Zones = upsertSceneZones(scene.Zones, patch.Zones)
	scene.Zones = removeSceneIDs(scene.Zones, patch.RemoveZoneIDs, func(v SceneZone) string { return v.ID })
	scene.Routes = upsertSceneRoutes(scene.Routes, patch.Routes)
	scene.Routes = removeSceneIDs(scene.Routes, patch.RemoveRouteIDs, func(v SceneRoute) string { return v.ID })
	return scene, nil
}

func upsertSceneRegions(base []SceneRegion, values []SceneRegion) []SceneRegion {
	for _, v := range values {
		found := false
		for i := range base {
			if base[i].ID == v.ID {
				base[i], found = v, true
				break
			}
		}
		if !found {
			base = append(base, v)
		}
	}
	return base
}
func upsertScenePlacements(base []ScenePlacement, values []ScenePlacement) []ScenePlacement {
	for _, v := range values {
		found := false
		for i := range base {
			if base[i].ID == v.ID {
				base[i], found = v, true
				break
			}
		}
		if !found {
			base = append(base, v)
		}
	}
	return base
}
func upsertSceneColliders(base []SceneCollider, values []SceneCollider) []SceneCollider {
	for _, v := range values {
		found := false
		for i := range base {
			if base[i].ID == v.ID {
				base[i], found = v, true
				break
			}
		}
		if !found {
			base = append(base, v)
		}
	}
	return base
}
func upsertSceneAttachments(base []SceneAttachment, values []SceneAttachment) []SceneAttachment {
	for _, v := range values {
		found := false
		for i := range base {
			if base[i].ID == v.ID {
				base[i], found = v, true
				break
			}
		}
		if !found {
			base = append(base, v)
		}
	}
	return base
}
func upsertSceneZones(base []SceneZone, values []SceneZone) []SceneZone {
	for _, v := range values {
		found := false
		for i := range base {
			if base[i].ID == v.ID {
				base[i], found = v, true
				break
			}
		}
		if !found {
			base = append(base, v)
		}
	}
	return base
}
func upsertSceneRoutes(base []SceneRoute, values []SceneRoute) []SceneRoute {
	for _, v := range values {
		found := false
		for i := range base {
			if base[i].ID == v.ID {
				base[i], found = v, true
				break
			}
		}
		if !found {
			base = append(base, v)
		}
	}
	return base
}
