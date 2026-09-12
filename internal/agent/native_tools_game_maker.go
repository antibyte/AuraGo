package agent

import openai "github.com/sashabaranov/go-openai"

func appendGameMakerToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.GameMakerEnabled {
		return tools
	}
	return append(tools,
		tool("game_maker_project",
			"Submit a short set_design (preferred), inspect examples, list files, or use the legacy full set_plan. scene_inspect accepts optional node_ids (up to 32) or region_id filters and returns only bounded node details and bindings. Planning must finish before code or asset mutations. Plans are internal and never player-facing.",
			schema(map[string]interface{}{
				"job_id":          prop("string", "Active Game Maker job ID"),
				"operation":       map[string]interface{}{"type": "string", "enum": []string{"inspect", "list_files", "get_plan", "set_plan", "set_design", "scene_inspect", "scene_set", "scene_patch", "scene_generate"}},
				"design":          gameDesignSchema(),
				"plan":            map[string]interface{}{"type": "object", "description": "Complete GamePlan object. Read inspect.plan_example, retain every required field, replace its example design and submit exact asset IDs from describe_asset."},
				"scene":           gameMakerSceneSchema(),
				"patch":           gameMakerScenePatchSchema(),
				"generate":        gameMakerSceneGenerateSchema(),
				"expected_sha256": prop("string", "Current scene sha256 from scene_inspect; required for existing scene writes"),
				"node_ids":        map[string]interface{}{"type": "array", "items": prop("string", "Exact scene node ID"), "maxItems": 32, "description": "scene_inspect only: return bounded details for these nodes and their bindings"},
				"region_id":       prop("string", "scene_inspect only: return bounded details for this region and its nodes"),
				"dry_run":         prop("boolean", "Validate or generate without writing"),
			}, "job_id", "operation"),
		),
		tool("game_maker_file",
			"Read a bounded source range (includes full-file sha256), replace one unique old_text with new_text using expected_sha256, or write a complete file. Writes return written and build.ok separately; fix compiler diagnostics before runtime validation. Prefer replace for existing files. Managed vendor/dist paths are read-only.",
			schema(map[string]interface{}{
				"job_id":          prop("string", "Active Game Maker job ID"),
				"operation":       map[string]interface{}{"type": "string", "enum": []string{"read", "write", "replace"}},
				"path":            prop("string", "Project-relative source path"),
				"content":         prop("string", "Complete file content for write"),
				"start_line":      prop("integer", "First line, 1-based; default 1"),
				"end_line":        prop("integer", "Last line; default next 120 lines; max 240 lines"),
				"expected_sha256": prop("string", "Full-file sha256 from read; required for replace"),
				"old_text":        prop("string", "Unique exact block to replace"),
				"new_text":        prop("string", "Replacement text; empty deletes the block"),
			}, "job_id", "operation", "path"),
		),
		tool("game_maker_asset",
			"Search matching sprite2d or model3d assets, then describe_asset for exact IDs, actions, orientation and helper usage. import_pack returns project-local copies. For model3d supply 1–64 exact asset_ids; only those models and dependencies are imported. Use three_example and local GLBs for 3D, phaser_example for sprites. Never guess paths, bones or clips. Import and generation require an accepted plan.",
			schema(map[string]interface{}{
				"operation":        map[string]interface{}{"type": "string", "enum": []string{"generate", "list_packs", "describe_pack", "import_pack", "search_assets", "describe_asset"}},
				"query":            prop("string", "English asset search terms; returns six compact matches by default"),
				"asset_kind":       map[string]interface{}{"type": "string", "enum": []string{"sprite2d", "model3d", "effect", "audio"}},
				"view":             map[string]interface{}{"type": "string", "enum": []string{"side", "top", "board", "3d"}},
				"asset_ids":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "minItems": 1, "maxItems": 64, "description": "Required exact IDs for model3d, effect or audio import_pack; omitted for sprite packs"},
				"limit":            map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 12},
				"asset_id":         prop("string", "Exact asset ID for describe_asset; omit when assembly_id is used"),
				"assembly_id":      prop("string", "Exact complete assembly ID for describe_asset"),
				"pack_id":          prop("string", "Pack ID from list_packs; required for describe_pack and import_pack"),
				"job_id":           prop("string", "Active Game Maker job ID"),
				"kind":             map[string]interface{}{"type": "string", "enum": []string{"image", "music"}},
				"prompt":           prop("string", "Concise asset prompt"),
				"path":             prop("string", "Destination under assets/"),
				"title":            prop("string", "Optional music title"),
				"duration_seconds": prop("number", "Local music duration, default 120; within the active profile limit, at most 600 seconds"),
				"bpm":              prop("integer", "Local music BPM, 30–300; omit for automatic"),
				"seed":             prop("integer", "Local music seed, 0–2147483647; omit for random"),
			}, "job_id"),
		),
		tool("game_maker_validate",
			"Validate the current build in the open Studio preview. scope startup (default) checks loading; gameplay/full also execute bounded input and state comparisons. Optional check_ids repeats up to 16 existing checks for repair feedback; targeted runs are never publishable. Missing observations never pass. The server requires full checks for 2D and guided 3D publication; repair at most three times across the job.",
			schema(map[string]interface{}{
				"job_id":    prop("string", "Active Game Maker job ID"),
				"scope":     map[string]interface{}{"type": "string", "enum": []string{"startup", "gameplay", "full"}},
				"check_ids": map[string]interface{}{"type": "array", "items": prop("string", "Existing gameplay check ID"), "maxItems": 16, "description": "Optional targeted repeat of existing checks; partial runs are never publishable"},
			}, "job_id"),
		),
	)
}

// Scene payloads stay intentionally small and bounded. The project operation owns
// their stage gate; source writes remain the escape hatch for custom code.
func gameMakerMechanicsSchema() map[string]interface{} {
	kinds := []string{
		"movement", "camera", "health", "collect", "destroy", "reach", "survive", "checkpoint",
		"patrol", "chase", "keepdistance", "damage", "projectile", "waves", "inventory", "dialogue", "unlock",
	}
	block := schema(map[string]interface{}{
		"id":      prop("string", "Stable optional mechanic block ID"),
		"kind":    map[string]interface{}{"type": "string", "enum": kinds, "description": "Composable style-neutral block kind; use source edits for behavior outside these optional helpers"},
		"role":    prop("string", "Optional semantic role"),
		"target":  prop("string", "Exact target node ID; use role to select an asset role"),
		"value":   prop("number", "Optional bounded numeric parameter"),
		"params":  prop("string", "Optional JSON object of bounded parameters"),
		"enabled": prop("boolean", "Whether this optional block is active"),
	}, "id", "kind")
	return schema(map[string]interface{}{
		"outcomes": map[string]interface{}{"type": "array", "maxItems": 2, "items": map[string]interface{}{"type": "string", "enum": []string{"won", "lost"}}, "description": "Optional terminal outcomes; omit for continuous play"},
		"lives":    map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 99, "description": "Optional lives count; no default is imposed"},
		"blocks":   map[string]interface{}{"type": "array", "maxItems": 32, "items": block},
		"events": map[string]interface{}{"type": "array", "maxItems": 32, "items": schema(map[string]interface{}{
			"event":  prop("string", "Observed event name"),
			"effect": prop("string", "Optional resolved effect ID"),
			"sound":  prop("string", "Optional resolved sound ID"),
		}, "event")},
	})
}
func gameMakerSceneSchema() map[string]interface{} {
	vec := func(description string) map[string]interface{} {
		return map[string]interface{}{"type": "array", "items": prop("number", description), "minItems": 3, "maxItems": 3}
	}
	bounds := func(description string) map[string]interface{} {
		return schema(map[string]interface{}{"min": vec(description + " minimum"), "max": vec(description + " maximum")}, "min", "max")
	}
	levels := map[string]interface{}{"type": "array", "maxItems": 16, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable level ID"), "name": prop("string", "Optional display name"), "active": prop("boolean", "Exactly one active level"),
	}, "id", "active")}
	nodes := map[string]interface{}{"type": "array", "maxItems": 512, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable node ID"), "kind": prop("string", "Node kind such as player, goal, wall or decorative"),
		"position": vec("Node position"), "size": vec("Node size"), "pinned": prop("boolean", "Keep this node during generation"),
		"level_id": prop("string", "Optional level ID"), "region_id": prop("string", "Optional region ID"),
		"properties": map[string]interface{}{"type": "object", "description": "Optional small JSON properties"},
	}, "id", "kind", "position")}
	regions := map[string]interface{}{"type": "array", "maxItems": 128, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable region ID"), "kind": prop("string", "Composable generator kind"),
		"bounds": bounds("Region bounds"), "seed": prop("integer", "Deterministic region seed"),
		"level_id": prop("string", "Optional level ID"), "generator": prop("string", "Optional generator recipe ID"),
		"pinned": prop("boolean", "Keep this region during generation"),
	}, "id", "kind", "bounds", "seed")}
	placements := map[string]interface{}{"type": "array", "maxItems": 512, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable placement ID"), "node_id": prop("string", "Node ID"), "asset_id": prop("string", "Exact imported asset ID"),
		"asset_role": prop("string", "Semantic role independent from behavior"), "behavior": prop("string", "Behavior independent from asset role"),
		"position": vec("Placement position"), "rotation": vec("Optional rotation"), "scale": vec("Optional scale"),
		"level_id": prop("string", "Optional level ID"), "region_id": prop("string", "Optional region ID"),
	}, "id", "node_id", "asset_role", "behavior", "position")}
	colliders := map[string]interface{}{"type": "array", "maxItems": 512, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable collider ID"), "node_id": prop("string", "Node ID"), "shape": prop("string", "Collider shape"),
		"extents": vec("Collider extents"), "offset": vec("Optional collider offset"),
		"level_id": prop("string", "Optional level ID"), "region_id": prop("string", "Optional region ID"),
	}, "id", "node_id", "shape", "extents")}
	attachments := map[string]interface{}{"type": "array", "maxItems": 256, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable attachment ID"), "node_id": prop("string", "Node ID"), "asset_id": prop("string", "Exact imported asset ID"),
		"socket": prop("string", "Attachment socket"), "position": vec("Optional attachment position"), "rotation": vec("Optional attachment rotation"),
		"level_id": prop("string", "Optional level ID"), "region_id": prop("string", "Optional region ID"),
	}, "id", "node_id", "socket")}
	zones := map[string]interface{}{"type": "array", "maxItems": 128, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable zone ID"), "kind": prop("string", "spawn, goal, loss or trigger"), "bounds": bounds("Zone bounds"),
		"level_id": prop("string", "Optional level ID"), "region_id": prop("string", "Optional region ID"),
		"properties": map[string]interface{}{"type": "object", "description": "Optional small JSON properties"},
	}, "id", "kind", "bounds")}
	routes := map[string]interface{}{"type": "array", "maxItems": 256, "items": schema(map[string]interface{}{
		"id": prop("string", "Stable route ID"), "from": prop("string", "Start node or zone ID"), "to": prop("string", "End node or zone ID"),
		"waypoints": map[string]interface{}{"type": "array", "maxItems": 64, "items": vec("Route waypoint")},
		"kind":      prop("string", "Optional route kind"), "level_id": prop("string", "Optional level ID"), "region_id": prop("string", "Optional region ID"),
	}, "id", "from", "to")}
	return schema(map[string]interface{}{
		"schema_version": prop("integer", "Scene schema version; use 1"),
		"dimension":      map[string]interface{}{"type": "string", "enum": []string{"2d", "3d"}},
		"seed":           prop("integer", "Deterministic generation seed"),
		"navigation":     map[string]interface{}{"type": "string", "enum": []string{"topdown", "ground", "platforms", "custom"}},
		"levels":         levels, "world_bounds": bounds("World bounds"), "camera_bounds": bounds("Camera bounds"),
		"nodes": nodes, "regions": regions, "placements": placements, "colliders": colliders,
		"attachments": attachments, "zones": zones, "routes": routes,
	}, "schema_version", "dimension", "seed", "levels", "world_bounds")
}

func gameMakerScenePatchSchema() map[string]interface{} {
	scene := gameMakerSceneSchema()
	sceneProps := scene["properties"].(map[string]interface{})
	properties := map[string]interface{}{
		"replace":               scene,
		"schema_version":        prop("integer", "Optional replacement schema version"),
		"dimension":             map[string]interface{}{"type": "string", "enum": []string{"2d", "3d"}},
		"seed":                  prop("integer", "Optional new generation seed"),
		"navigation":            map[string]interface{}{"type": "string", "enum": []string{"topdown", "ground", "platforms", "custom"}},
		"world_bounds":          sceneProps["world_bounds"],
		"camera_bounds":         sceneProps["camera_bounds"],
		"clear_camera_bounds":   prop("boolean", "Clear camera bounds"),
		"remove_node_ids":       map[string]interface{}{"type": "array", "maxItems": 512, "items": prop("string", "Node ID")},
		"remove_region_ids":     map[string]interface{}{"type": "array", "maxItems": 128, "items": prop("string", "Region ID")},
		"remove_placement_ids":  map[string]interface{}{"type": "array", "maxItems": 512, "items": prop("string", "Placement ID")},
		"remove_collider_ids":   map[string]interface{}{"type": "array", "maxItems": 512, "items": prop("string", "Collider ID")},
		"remove_attachment_ids": map[string]interface{}{"type": "array", "maxItems": 256, "items": prop("string", "Attachment ID")},
		"remove_zone_ids":       map[string]interface{}{"type": "array", "maxItems": 128, "items": prop("string", "Zone ID")},
		"remove_route_ids":      map[string]interface{}{"type": "array", "maxItems": 256, "items": prop("string", "Route ID")},
	}
	for _, key := range []string{"levels", "nodes", "regions", "placements", "colliders", "attachments", "zones", "routes"} {
		properties[key] = sceneProps[key]
	}
	return schema(properties)
}
func gameMakerSceneGenerateSchema() map[string]interface{} {
	vec := func(description string) map[string]interface{} {
		return map[string]interface{}{"type": "array", "items": prop("number", description), "minItems": 3, "maxItems": 3}
	}
	return schema(map[string]interface{}{
		"region_id":    prop("string", "Stable target region ID"),
		"kind":         prop("string", "Generator kind; keep recipes composable and style-neutral"),
		"bounds":       schema(map[string]interface{}{"min": vec("Minimum"), "max": vec("Maximum")}, "min", "max"),
		"seed":         prop("integer", "Deterministic seed"),
		"level_id":     prop("string", "Optional level ID"),
		"density":      prop("integer", "Optional bounded placement density"),
		"cell_size":    prop("number", "Optional grid cell size"),
		"min_distance": prop("number", "Optional minimum spacing"),
		"clear":        prop("boolean", "Clear generated region content first"),
		"node_kind":    prop("string", "Generated node kind"),
		"asset_id":     prop("string", "Exact imported asset ID"),
		"asset_role":   prop("string", "Semantic asset role"),
		"behavior":     prop("string", "Behavior independent from visual asset"),
		"zone_kind":    prop("string", "Optional generated zone kind"),
		"dry_run":      prop("boolean", "Validate and preview without writing"),
	}, "region_id", "kind", "bounds", "seed")
}

// Explicit properties keep the compact design an object on strict providers.
func gameDesignSchema() map[string]interface{} {
	stringsArray := func(description string) map[string]interface{} {
		return map[string]interface{}{"type": "array", "items": prop("string", ""), "description": description}
	}
	return schema(map[string]interface{}{
		"base":      map[string]interface{}{"type": "string", "enum": []string{"shooter", "platformer", "topdown", "blocks", "board", "minimal", "three", "fps", "exploration", "transport", "flight", "space"}},
		"objective": prop("string", "Concrete player objective"),
		"features":  stringsArray("1–12 concrete requested features; additional mechanics use small source edits"),
		"scene":     gameMakerSceneSchema(),
		"mechanics": gameMakerMechanicsSchema(),
		"assets": map[string]interface{}{"type": "array", "maxItems": 64, "items": schema(map[string]interface{}{
			"role":    prop("string", "player, enemy, item, tree, arms, weapon, cargo, goal, building, planet or a distinct custom role"),
			"pack_id": prop("string", "Exact catalog pack; empty for procedural art"), "asset_id": prop("string", "Exact model/sprite ID"), "assembly_id": prop("string", "Complete sprite assembly ID instead of asset_id"), "fallback": prop("string", "Named procedural graphic when no pack is selected"),
		}, "role")},
		"settings": schema(map[string]interface{}{"goal": prop("integer", "1–24 objectives, default 5"), "speed": prop("number", "Guided 3D only: 1–40 meters/second, default 5"), "duration": prop("integer", "15–600 seconds, default 120")}, "goal", "speed", "duration"),
		"preserve": stringsArray("Existing behaviors kept in edit jobs"),
		"presentation": schema(map[string]interface{}{
			"environment": prop("string", "Exact aurago-effects atmosphere ID; search asset_kind effect first"),
			"effects":     stringsArray("Exact additional aurago-effects IDs"),
			"sounds": map[string]interface{}{"type": "array", "maxItems": 40, "items": schema(map[string]interface{}{
				"event": map[string]interface{}{"type": "string", "enum": []string{"step", "jump", "land", "shot", "reload", "hit", "pickup", "win", "lose", "splash", "interact", "engine", "ui", "ambient"}},
				"sound": prop("string", "Exact aurago-sounds ID"),
			}, "event", "sound")},
			"quality": map[string]interface{}{"type": "string", "enum": []string{"auto", "low", "medium", "high"}},
		}),
	}, "base", "objective", "features")
}

// Freeze only phase-relevant schemas; runtime mutation gates remain authoritative.
func GameMakerPhaseToolSchemas(stage, dimension string) []openai.Tool {
	schemas := appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true})
	out := []openai.Tool{}
	for _, t := range schemas {
		name := t.Function.Name
		params := t.Function.Parameters.(map[string]interface{})
		props := params["properties"].(map[string]interface{})
		if stage == "planning" {
			switch name {
			case "game_maker_validate":
				continue
			case "game_maker_file":
				props["operation"] = map[string]interface{}{"type": "string", "enum": []string{"read"}}
				for _, k := range []string{"content", "old_text", "new_text", "expected_sha256"} {
					delete(props, k)
				}
			case "game_maker_asset":
				props["operation"] = map[string]interface{}{"type": "string", "enum": []string{"search_assets", "describe_asset", "list_packs"}}
				for _, k := range []string{"asset_ids", "kind", "prompt", "path", "title", "duration_seconds", "bpm", "seed"} {
					delete(props, k)
				}
			case "game_maker_project":
				props["operation"] = map[string]interface{}{"type": "string", "enum": []string{"set_design", "get_plan", "inspect", "list_files", "scene_inspect"}}
				delete(props, "plan")
				design := props["design"].(map[string]interface{})["properties"].(map[string]interface{})
				bases := []string{"shooter", "platformer", "topdown", "blocks", "board", "minimal"}
				if dimension == "3d" {
					bases = []string{"fps", "exploration", "transport", "flight", "space", "three"}
				}
				design["base"] = map[string]interface{}{"type": "string", "enum": bases}
				if dimension != "3d" {
					delete(design, "settings")
				}
				for _, key := range []string{"scene", "patch", "generate", "expected_sha256", "dry_run"} {
					delete(props, key)
				}
			}
		} else if name == "game_maker_project" {
			props["operation"] = map[string]interface{}{"type": "string", "enum": []string{"get_plan", "inspect", "list_files", "scene_inspect", "scene_set", "scene_patch", "scene_generate"}}
			delete(props, "plan")
			delete(props, "design")
		}
		required := []string{}
		for _, k := range []string{"job_id", "operation", "path"} {
			if _, ok := props[k]; ok && (k != "path" || name == "game_maker_file") {
				required = append(required, k)
			}
		}
		if name == "game_maker_validate" {
			required = []string{"job_id"}
		}
		params["required"] = required
		normalizeProviderFragileObjectSchemas(params)
		injectAdditionalPropertiesRec(params)
		out = append(out, t)
	}
	return out
}
