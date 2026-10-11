package agent

import (
	"aurago/internal/cydcreator"

	openai "github.com/sashabaranov/go-openai"
)

// CYD Creator tools exist only in isolated CYD Creator job runs: they are not
// part of builtinToolSchemas, the normal chat catalog, discovery or the
// training catalog. The server's prepared prompt profile carries the schemas of
// one phase (CYDCreatorPhaseToolSchemas); dispatch requires a job bound with
// cydcreator.WithJobContext.

func cydCreatorWidgetTypes() []string {
	var out []string
	for _, spec := range cydcreator.WidgetCatalog() {
		out = append(out, spec.Type)
	}
	return out
}

func cydCreatorThemeIDs() []string {
	themes, _ := cydcreator.Themes()
	out := make([]string, 0, len(themes))
	for _, t := range themes {
		out = append(out, t.ID)
	}
	return out
}

func cydCreatorLibraryIDs() []string {
	var out []string
	for _, l := range cydcreator.Libraries() {
		out = append(out, l.ID)
	}
	return out
}

func cydCreatorStyleSchema() map[string]interface{} {
	return schema(map[string]interface{}{
		"bg_color":     prop("string", "#RRGGBB"),
		"bg_opacity":   map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 255},
		"text_color":   prop("string", "#RRGGBB"),
		"accent_color": prop("string", "#RRGGBB"),
		"border_color": prop("string", "#RRGGBB"),
		"border_width": map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 8},
		"radius":       map[string]interface{}{"type": "integer", "minimum": 0, "maximum": 64},
	})
}

func cydCreatorLayoutOpSchema() map[string]interface{} {
	transitions := []string{"none", "fade", "slide_left", "slide_right", "slide_up", "slide_down"}
	action := schema(map[string]interface{}{
		"type":       map[string]interface{}{"type": "string", "enum": []string{"goto_screen", "back", "open_wifi_setup", "logic"}},
		"screen":     prop("string", "goto_screen: target screen id or name"),
		"transition": map[string]interface{}{"type": "string", "enum": transitions},
		"hint":       prop("string", "logic: what the hook must do (max 500 characters); the coding phase implements it"),
	}, "type")
	event := schema(map[string]interface{}{
		"on":     map[string]interface{}{"type": "string", "enum": []string{"clicked", "long_pressed", "value_changed", "ready"}},
		"action": action,
	}, "on", "action")
	widget := schema(map[string]interface{}{
		"type":   map[string]interface{}{"type": "string", "enum": cydCreatorWidgetTypes()},
		"name":   prop("string", "C identifier ^[a-z][a-z0-9_]{0,31}$, unique; required for widgets with events or that code must reach"),
		"x":      prop("integer", "Left edge in pixels relative to the parent"),
		"y":      prop("integer", "Top edge in pixels relative to the parent"),
		"w":      prop("integer", "Width; omit for the catalog default"),
		"h":      prop("integer", "Height; omit for the catalog default"),
		"hidden": prop("boolean", "Start hidden"),
		"props": map[string]interface{}{"type": "object",
			"description": "Widget properties of the catalog, e.g. {\"text\":\"22 °C\",\"size\":28} for a label"},
		"style":  cydCreatorStyleSchema(),
		"events": map[string]interface{}{"type": "array", "maxItems": 4, "items": event},
	}, "type")
	screen := schema(map[string]interface{}{
		"name":       prop("string", "C identifier, unique"),
		"title":      prop("string", "Shown title"),
		"transition": map[string]interface{}{"type": "string", "enum": transitions},
		"style":      cydCreatorStyleSchema(),
	}, "name")
	return schema(map[string]interface{}{
		"op": map[string]interface{}{"type": "string", "enum": []string{"add_widget", "update_widget", "move_widget", "delete_widget",
			"add_screen", "update_screen", "delete_screen", "set_theme", "set_settings"}},
		"screen":      prop("string", "Screen id or name (add_widget, move_widget target, update_screen/delete_screen target)"),
		"parent":      prop("string", "Container widget id or name; empty = screen root"),
		"tab":         prop("integer", "Tab index when the parent is a tabview"),
		"id":          prop("string", "Target widget or screen id or name (update/move/delete)"),
		"widget":      widget,
		"screen_data": screen,
		"set": map[string]interface{}{"type": "object",
			"description": "update_widget/update_screen/set_settings: fields to merge; props and style merge one level deep, null deletes. set_settings keys: wifi, aurago_link, libraries, backlight, statusbar_clock"},
		"x":     prop("integer", "move_widget/add_widget: new x"),
		"y":     prop("integer", "move_widget/add_widget: new y"),
		"theme": map[string]interface{}{"type": "string", "enum": cydCreatorThemeIDs()},
	}, "op")
}

// cydCreatorToolSchemas returns every CYD Creator tool in cydcreator.ToolNames order.
func cydCreatorToolSchemas() []openai.Tool {
	return []openai.Tool{
		tool(cydcreator.ToolReadProject,
			"Read the job's working copy: layout JSON with validation issues, the src/app file list, board capabilities, the accepted plan and what the user changed since the last job. A layout too large for one result is left out (truncated, layout_summary): then page cyd.project.json with cyd_creator_read_file.",
			schema(map[string]interface{}{})),
		tool(cydcreator.ToolAPIReference,
			"Read reference text: overview (rules and budgets), cyd (template API cyd.h and runtime notes), lvgl (LVGL 9 usage), widgets (generated handles, hooks, screens, series and fonts of this project), libraries (allowed libraries), board (features and budgets). A long topic comes in pages of whole lines (start_line, end_line, total_lines, truncated); read the rest with start_line.",
			schema(map[string]interface{}{
				"topic":      map[string]interface{}{"type": "string", "enum": cydcreator.ReferenceTopics},
				"start_line": prop("integer", "First line, 1-based; default 1"),
				"end_line":   prop("integer", "Last line, inclusive; default: as many lines as fit one page"),
			}, "topic")),
		tool(cydcreator.ToolSubmitPlan,
			"Submit the app plan for the user's confirmation. An accepted plan ends planning; build nothing now.",
			schema(map[string]interface{}{
				"plan": schema(map[string]interface{}{
					"summary":      prop("string", "One short paragraph for the user, in the user's language"),
					"screens":      map[string]interface{}{"type": "array", "items": prop("string", "Screen name and purpose"), "minItems": 1, "maxItems": 16},
					"widgets":      map[string]interface{}{"type": "array", "items": prop("string", "Main widget and what it shows"), "maxItems": 64},
					"data_sources": map[string]interface{}{"type": "array", "items": prop("string", "Where data comes from (API, sensor, user input)"), "maxItems": 16},
					"wifi":         map[string]interface{}{"type": "string", "enum": []string{"off", "optional", "required"}},
					"aurago_link":  prop("boolean", "Over-the-air updates from AuraGo (needs Wi-Fi)"),
					"libraries":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string", "enum": cydCreatorLibraryIDs()}},
				}, "summary", "screens", "wifi"),
			}, "plan")),
		tool(cydcreator.ToolUpdateLayout,
			"Apply validated layout ops atomically to the working copy. An op that introduces a new validation error rejects the whole call with its index and reason; nothing changes then. New widgets get fresh ids and catalog default sizes. Returns per-op summaries and the remaining issues.",
			schema(map[string]interface{}{
				"ops": map[string]interface{}{"type": "array", "minItems": 1, "maxItems": 64, "items": cydCreatorLayoutOpSchema()},
			}, "ops")),
		tool(cydcreator.ToolReadFile,
			"Read a source file of the working copy (src/app/**) with its sha256, or a generated file (src/generated/**, include/**, platformio.ini) or the layout cyd.project.json as read-only text. One call returns one page of whole lines (at most about 32 KiB) with start_line, end_line, total_lines and truncated; read longer files range by range with start_line/end_line.",
			schema(map[string]interface{}{
				"path":       prop("string", "Project-relative path, e.g. src/app/app_main.cpp"),
				"start_line": prop("integer", "First line, 1-based; default 1"),
				"end_line":   prop("integer", "Last line, inclusive; default: as many lines as fit one page"),
			}, "path")),
		tool(cydcreator.ToolWriteFile,
			"Write a complete source file below src/app/ (.cpp .h .hpp .c; at most 256 KiB per file, 64 files, 2 MiB in total). Generated files are read-only.",
			schema(map[string]interface{}{
				"path":    prop("string", "Path below src/app/, e.g. src/app/weather.cpp"),
				"content": prop("string", "Complete file content"),
			}, "path", "content")),
		tool(cydcreator.ToolDeleteFile,
			"Delete a source file below src/app/.",
			schema(map[string]interface{}{"path": prop("string", "Path below src/app/")}, "path")),
		tool(cydcreator.ToolBuild,
			"Build the working copy with PlatformIO and wait for the result: status, error_code, diagnostics of the app code and flash/RAM usage.",
			schema(map[string]interface{}{})),
		tool(cydcreator.ToolFinish,
			"End this round with a short summary for the user (what the app does now, in the user's language). The server builds and publishes afterwards.",
			schema(map[string]interface{}{"summary": prop("string", "One short paragraph")}, "summary")),
	}
}

// CYDCreatorPhaseToolSchemas returns the schemas of one job phase in the
// order of cydcreator.PhaseTools; an unknown phase has no tools. Job identity
// comes from the execution context, never from arguments.
func CYDCreatorPhaseToolSchemas(phase string) []openai.Tool {
	byName := map[string]openai.Tool{}
	for _, t := range cydCreatorToolSchemas() {
		byName[t.Function.Name] = t
	}
	out := []openai.Tool{}
	for _, name := range cydcreator.PhaseTools(phase) {
		t := byName[name]
		params := t.Function.Parameters.(map[string]interface{})
		normalizeProviderFragileObjectSchemas(params)
		injectAdditionalPropertiesRec(params)
		out = append(out, t)
	}
	return out
}
