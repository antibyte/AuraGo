package agent

import (
	"strings"

	"aurago/internal/gamemaker"

	openai "github.com/sashabaranov/go-openai"
)

func appendGameMakerToolSchemas(tools []openai.Tool, ff ToolFeatureFlags) []openai.Tool {
	if !ff.GameMakerEnabled {
		return tools
	}
	return append(tools,
		tool("game_maker_project", "Inspect project files and capabilities, build, finish and publish, add local runtime libraries without overwriting source, or request optional framework documentation, craft recipes and starter templates. finish ends the job only after a successful build.", schema(map[string]interface{}{
			"job_id":    prop("string", "Active job; omitted in Studio"),
			"operation": map[string]interface{}{"type": "string", "enum": []string{"inspect", "list_files", "build", "finish", "reference", "use_template", "add_library"}},
			"template":  prop("string", "Optional template: "+strings.Join(gamemaker.StudioTemplateNames(), ", ")+". use_template replaces entry files; read existing work first."),
			"framework": map[string]interface{}{"type": "string", "enum": []string{"phaser", "three", "voxel"}, "description": "reference: optional framework documentation; omit to inspect installed helpers and available topics/libraries"},
			"topic":     map[string]interface{}{"type": "string", "enum": gamemaker.CraftRecipeTopics(), "description": "reference: optional craft recipe topic with copy-pasteable JS/TS snippets"},
			"library":   map[string]interface{}{"type": "string", "enum": []string{"phaser", "three", "voxel", "effects-2d", "effects-3d", "zzfx", "all"}, "description": "add_library: local runtime/helper bundle to copy into vendor/ without overwriting index.html or src/main.ts"},
		}, "operation")),
		tool("game_maker_file",
			"Search a literal query in one file or, without path, across editable project text files, including HTML, CSS, JS and local libraries. Read bounded lines with full-file sha256, replace a unique block, or write a complete file. replace_many atomically saves 1–8 edits. Writes only save; request project build or finish separately. Supply path for read, write and replace. Writes return written and sha256. Managed vendor/dist paths are read-only.",
			schema(map[string]interface{}{
				"job_id":    prop("string", "Active Game Maker job ID"),
				"operation": map[string]interface{}{"type": "string", "enum": []string{"read", "search", "write", "replace", "replace_many"}},
				"path":      prop("string", "Project-relative source path; omit only for a project-wide search"),
				"query":     prop("string", "search: literal single-line text, 1–120 characters; returns up to 12 matching lines with each file's sha256"),
				"edits": map[string]interface{}{"type": "array", "minItems": 1, "maxItems": 8, "items": schema(map[string]interface{}{
					"path":            prop("string", "Distinct existing project-relative file"),
					"expected_sha256": prop("string", "Full-file sha256 from read/search"),
					"old_text":        prop("string", "Unique exact block"),
					"new_text":        prop("string", "Replacement; empty string deletes"),
				}, "path", "expected_sha256", "old_text", "new_text")},
				"content":         prop("string", "Complete file content for write"),
				"start_line":      prop("integer", "First line, 1-based; default 1"),
				"end_line":        prop("integer", "Last line; default next 120 lines; max 240 lines"),
				"expected_sha256": prop("string", "Full-file sha256 from read; required for replace"),
				"old_text":        prop("string", "Unique exact block to replace"),
				"new_text":        prop("string", "Replacement text; empty deletes the block"),
			}, "job_id", "operation"),
		),
		tool("game_maker_asset",
			"Search matching sprite2d or model3d assets, then describe_asset for exact IDs, actions, orientation and helper usage. import_pack returns project-local copies. For model3d supply 1–64 exact asset_ids; only those models and dependencies are imported. generate supports image, music or local procedural sfx (.wav + ZzFX). Never guess paths, bones or clips. Import and generation are optional.",
			schema(map[string]interface{}{
				"operation":        map[string]interface{}{"type": "string", "enum": []string{"generate", "list_packs", "describe_pack", "import_pack", "search_assets", "describe_asset"}},
				"query":            prop("string", "English asset search terms; returns six compact matches by default"),
				"asset_kind":       map[string]interface{}{"type": "string", "enum": []string{"sprite2d", "model3d", "effect", "audio"}},
				"view":             map[string]interface{}{"type": "string", "enum": []string{"side", "top", "board", "3d", "isometric"}},
				"asset_ids":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "minItems": 1, "maxItems": 64, "description": "Required exact IDs for model3d, atlas sprite schema 2, effect or audio import_pack; omit only for legacy whole-sheet sprite packs"},
				"limit":            map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 12},
				"asset_id":         prop("string", "Exact asset ID for describe_asset; omit when assembly_id is used"),
				"assembly_id":      prop("string", "Exact complete assembly ID for describe_asset"),
				"pack_id":          prop("string", "Exact pack_id from search_assets or list_packs; required for describe_pack and import_pack, and recommended for describe_asset (auto-resolved when asset_id/assembly_id is unique across packs)."),
				"job_id":           prop("string", "Active Game Maker job ID"),
				"kind":             map[string]interface{}{"type": "string", "enum": []string{"image", "music", "sfx"}},
				"prompt":           prop("string", "Concise asset prompt"),
				"path":             prop("string", "Destination under assets/"),
				"title":            prop("string", "Optional music title"),
				"duration_seconds": prop("number", "Local music or sfx duration; music default 120 (max 600), sfx 0.06–3.0 seconds"),
				"bpm":              prop("integer", "Local music BPM, 30–300; omit for automatic"),
				"seed":             prop("integer", "Local music or sfx seed, 0–2147483647; omit for automatic/random"),
			}, "job_id"),
		),
		tool("game_maker_preview", "Control the independent browser for this Studio job. open/reload build current files. logs returns build-tagged browser errors; screenshot captures the current viewport without resetting state. input accepts keyboard, mouse, touch, waits and inline capture actions (2–6 captures return a stitched contact sheet). probe reads window.__gameDebug and HUD/canvas telemetry. close releases the browser.", schema(map[string]interface{}{
			"operation": map[string]interface{}{"type": "string", "enum": []string{"open", "reload", "logs", "screenshot", "input", "probe", "close"}},
			"viewport":  prop("string", "Optional viewport preset or size: landscape (1024x768), hd (1280x720), portrait/mobile (430x932), tablet (768x1024) or WIDTHxHEIGHT (320..1920)"),
			"actions": map[string]interface{}{"type": "array", "maxItems": 64, "items": schema(map[string]interface{}{
				"type": map[string]interface{}{"type": "string", "enum": []string{"key_down", "key_up", "key_press", "mouse_move", "mouse_down", "mouse_up", "click", "touch_start", "touch_move", "touch_end", "tap", "wait", "capture"}},
				"key":  prop("string", "Browser key, e.g. ArrowLeft, Space, Enter, a"),
				"x":    prop("number", "Viewport x"), "y": prop("number", "Viewport y"),
				"button":       map[string]interface{}{"type": "string", "enum": []string{"left", "middle", "right"}},
				"milliseconds": prop("integer", "Wait duration, 0–10000 ms"),
			}, "type")},
		}, "operation")),
	)
}

func GameMakerPhaseToolSchemas(stage, dimension string, variant ...string) []openai.Tool {
	out := appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true})
	for _, t := range out {
		params := t.Function.Parameters.(map[string]interface{})
		props := params["properties"].(map[string]interface{})
		delete(props, "job_id")
		params["required"] = []string{"operation"}
		normalizeProviderFragileObjectSchemas(params)
		injectAdditionalPropertiesRec(params)
	}
	return out
}
