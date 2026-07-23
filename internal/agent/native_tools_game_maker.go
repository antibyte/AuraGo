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
			"Generate or create a project-local image or music asset. Provider and budget failures return a procedural fallback instead of failing the game.",
			schema(map[string]interface{}{
				"job_id": prop("string", "Active Game Maker job ID"),
				"kind":   map[string]interface{}{"type": "string", "enum": []string{"image", "music"}},
				"prompt": prop("string", "Concise asset prompt"),
				"path":   prop("string", "Destination under assets/"),
				"title":  prop("string", "Optional music title"),
			}, "job_id", "kind", "prompt", "path"),
		),
		tool("game_maker_validate",
			"Compile the current TypeScript game with Pure-Go esbuild and return bounded diagnostics. A successful build triggers a live preview reload.",
			schema(map[string]interface{}{
				"job_id": prop("string", "Active Game Maker job ID"),
			}, "job_id"),
		),
	)
}
