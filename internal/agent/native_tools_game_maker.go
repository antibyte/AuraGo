package agent

import openai "github.com/sashabaranov/go-openai"

func appendGameMakerToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.GameMakerEnabled {
		return tools
	}
	return append(tools,
		tool("game_maker_project",
			"Inspect the job and plan example, list files, get_plan, or submit set_plan. Planning must finish before code or asset mutations. Plans are internal and never player-facing.",
			schema(map[string]interface{}{
				"job_id":    prop("string", "Active Game Maker job ID"),
				"operation": map[string]interface{}{"type": "string", "enum": []string{"inspect", "list_files", "get_plan", "set_plan"}},
				"plan":      map[string]interface{}{"type": "object", "description": "Complete GamePlan object. Read inspect.plan_example, retain every required field, replace its example design and submit exact asset IDs from describe_asset."},
			}, "job_id", "operation"),
		),
		tool("game_maker_file",
			"Read or atomically write a source file in the current Game Maker staging workspace. Use operation=write, path and complete content; only validate after status=ok. Isolated Studio runs supply an omitted job_id and infer write when content is supplied without operation. Other callers must provide job_id and operation. Managed vendor and dist paths cannot be written.",
			schema(map[string]interface{}{
				"job_id":    prop("string", "Active Game Maker job ID"),
				"operation": map[string]interface{}{"type": "string", "enum": []string{"read", "write"}},
				"path":      prop("string", "Project-relative source path"),
				"content":   prop("string", "Complete file content for write"),
			}, "job_id", "operation", "path"),
		),
		tool("game_maker_asset",
			"Find a few matching sprites with search_assets, then describe_asset for exact IDs, actions, directions and helper usage. Prefer built-in packs; import_pack returns project-local PNG/JSON paths and a phaser_example using aurago-game-1.js. A sheet is never a single sprite. Import and media generation require an accepted plan. Missing operation means generate for compatibility.",
			schema(map[string]interface{}{
				"operation":   map[string]interface{}{"type": "string", "enum": []string{"generate", "list_packs", "describe_pack", "import_pack", "search_assets", "describe_asset"}},
				"query":       prop("string", "English asset search terms; returns six compact matches by default"),
				"view":        map[string]interface{}{"type": "string", "enum": []string{"side", "top", "board"}},
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
			"Validate the current build in the open Studio preview. scope startup (default) checks loading; gameplay/full also execute bounded input and state comparisons. Missing observations never pass. The server requires full checks for 2D publication; repair at most three times across the job.",
			schema(map[string]interface{}{
				"job_id": prop("string", "Active Game Maker job ID"),
				"scope":  map[string]interface{}{"type": "string", "enum": []string{"startup", "gameplay", "full"}},
			}, "job_id"),
		),
	)
}
