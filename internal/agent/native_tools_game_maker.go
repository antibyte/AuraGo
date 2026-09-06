package agent

import openai "github.com/sashabaranov/go-openai"

func appendGameMakerToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.GameMakerEnabled {
		return tools
	}
	return append(tools,
		tool("game_maker_project",
			"Inspect the current Game Maker job, project manifest, and safe staging file list.",
			schema(map[string]interface{}{
				"job_id":    prop("string", "Active Game Maker job ID"),
				"operation": map[string]interface{}{"type": "string", "enum": []string{"inspect", "list_files"}},
			}, "job_id", "operation"),
		),
		tool("game_maker_file",
			"Read or atomically write a source file in the current Game Maker staging workspace. Managed vendor and dist paths cannot be written.",
			schema(map[string]interface{}{
				"job_id":    prop("string", "Active Game Maker job ID"),
				"operation": map[string]interface{}{"type": "string", "enum": []string{"read", "write"}},
				"path":      prop("string", "Project-relative source path"),
				"content":   prop("string", "Complete file content for write"),
			}, "job_id", "operation", "path"),
		),
		tool("game_maker_asset",
			"Browse, describe, and import offline sprite packs, or generate a project-local image/music asset. Prefer matching built-in packs; import returns exact PNG and JSON paths with frame/animation metadata. Missing operation means generate for compatibility.",
			schema(map[string]interface{}{
				"operation": map[string]interface{}{"type": "string", "enum": []string{"generate", "list_packs", "describe_pack", "import_pack"}},
				"pack_id":   prop("string", "Pack ID from list_packs; required for describe_pack and import_pack"),
				"job_id":    prop("string", "Active Game Maker job ID"),
				"kind":      map[string]interface{}{"type": "string", "enum": []string{"image", "music"}},
				"prompt":    prop("string", "Concise asset prompt"),
				"path":      prop("string", "Destination under assets/"),
				"title":     prop("string", "Optional music title"),
			}, "job_id"),
		),
		tool("game_maker_validate",
			"Compile the game, reload the Studio preview, and wait for a browser startup check. Returns bounded build/runtime diagnostics; fix reported errors. An open Studio preview is required. A successful check covers startup only, not all gameplay.",
			schema(map[string]interface{}{
				"job_id": prop("string", "Active Game Maker job ID"),
			}, "job_id"),
		),
	)
}
