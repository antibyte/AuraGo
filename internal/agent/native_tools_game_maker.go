package agent

import openai "github.com/sashabaranov/go-openai"

func appendGameMakerToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.GameMakerEnabled {
		return tools
	}
	return append(tools,
		tool("game_maker_project",
			"Submit a short set_design (preferred), inspect examples, list files, or use the legacy full set_plan. Planning must finish before code or asset mutations. Plans are internal and never player-facing.",
			schema(map[string]interface{}{
				"job_id":    prop("string", "Active Game Maker job ID"),
				"operation": map[string]interface{}{"type": "string", "enum": []string{"inspect", "list_files", "get_plan", "set_plan", "set_design"}},
				"design":    gameDesignSchema(),
				"plan":      map[string]interface{}{"type": "object", "description": "Complete GamePlan object. Read inspect.plan_example, retain every required field, replace its example design and submit exact asset IDs from describe_asset."},
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
				"operation":   map[string]interface{}{"type": "string", "enum": []string{"generate", "list_packs", "describe_pack", "import_pack", "search_assets", "describe_asset"}},
				"query":       prop("string", "English asset search terms; returns six compact matches by default"),
				"view":        map[string]interface{}{"type": "string", "enum": []string{"side", "top", "board", "3d"}},
				"asset_ids":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "minItems": 1, "maxItems": 64, "description": "Required exact model IDs for model3d import_pack; omitted for sprite packs"},
				"limit":       map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 12},
				"asset_id":    prop("string", "Exact asset ID for describe_asset; omit when assembly_id is used"),
				"assembly_id": prop("string", "Exact complete assembly ID for describe_asset"),
				"pack_id":     prop("string", "Pack ID from list_packs; required for describe_pack and import_pack"),
				"job_id":      prop("string", "Active Game Maker job ID"),
				"kind":        map[string]interface{}{"type": "string", "enum": []string{"image", "music"}},
				"prompt":      prop("string", "Concise asset prompt"),
				"path":        prop("string", "Destination under assets/"),
				"title":       prop("string", "Optional music title"),
			}, "job_id"),
		),
		tool("game_maker_validate",
			"Validate the current build in the open Studio preview. scope startup (default) checks loading; gameplay/full also execute bounded input and state comparisons. Missing observations never pass. The server requires full checks for 2D and guided 3D publication; repair at most three times across the job.",
			schema(map[string]interface{}{
				"job_id": prop("string", "Active Game Maker job ID"),
				"scope":  map[string]interface{}{"type": "string", "enum": []string{"startup", "gameplay", "full"}},
			}, "job_id"),
		),
	)
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
		"assets": map[string]interface{}{"type": "array", "maxItems": 64, "items": schema(map[string]interface{}{
			"role":    prop("string", "player, enemy, item, tree, arms, weapon, cargo, goal, building, planet or a distinct custom role"),
			"pack_id": prop("string", "Exact catalog pack; empty for procedural art"), "asset_id": prop("string", "Exact model/sprite ID"), "assembly_id": prop("string", "Complete sprite assembly ID instead of asset_id"), "fallback": prop("string", "Named procedural graphic when no pack is selected"),
		}, "role")},
		"settings": schema(map[string]interface{}{"goal": prop("integer", "1–24 objectives, default 5"), "speed": prop("number", "Guided 3D only: 1–40 meters/second, default 5"), "duration": prop("integer", "15–600 seconds, default 120")}, "goal", "speed", "duration"),
		"preserve": stringsArray("Existing behaviors kept in edit jobs"),
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
				for _, k := range []string{"asset_ids", "kind", "prompt", "path", "title"} {
					delete(props, k)
				}
			case "game_maker_project":
				props["operation"] = map[string]interface{}{"type": "string", "enum": []string{"set_design", "get_plan", "inspect", "list_files"}}
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
			}
		} else if name == "game_maker_project" {
			props["operation"] = map[string]interface{}{"type": "string", "enum": []string{"get_plan", "inspect", "list_files"}}
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
