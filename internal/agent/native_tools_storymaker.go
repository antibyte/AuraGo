package agent

import (
	"slices"
	"strings"

	"aurago/internal/storymaker"

	openai "github.com/sashabaranov/go-openai"
)

// StorymakerNames lists the catalog names a phase schema offers as enums. An
// empty list leaves that field a plain string; Studio still validates the value.
type StorymakerNames struct {
	Poses, Gestures, Expressions, Environments, Characters []string
}

// storymakerSceneFields are the scene fields patch_scene can remove with null.
var storymakerSceneFields = []string{"kind", "environment", "variant", "narration", "speaker", "transition", "duration", "camera", "actors", "props", "beats"}

// StorymakerPhaseToolSchemas returns the closed job tool schemas of one phase in the catalog mode.
func StorymakerPhaseToolSchemas(phase string, compact bool, names StorymakerNames) []openai.Tool {
	return StorymakerPhaseToolSchemasFor(storymaker.ModeCatalog, phase, compact, names)
}

// StorymakerPhaseToolSchemasFor returns the closed job tool schemas of one phase for the mode of a project.
// The tools never join the normal catalog: a job profile is their only route.
func StorymakerPhaseToolSchemasFor(mode, phase string, compact bool, names StorymakerNames) []openai.Tool {
	out := []openai.Tool{}
	for _, name := range storymaker.PhaseToolsFor(mode, phase) {
		var definition openai.Tool
		switch name {
		case storymaker.ToolStory:
			definition = storymakerStoryTool(mode, phase, compact, names)
		case storymaker.ToolCatalog:
			definition = storymakerCatalogTool(mode, phase, compact)
		case storymaker.ToolCheck:
			definition = storymakerCheckTool(mode, phase)
		default:
			continue
		}
		params := definition.Function.Parameters.(map[string]interface{})
		normalizeProviderFragileObjectSchemas(params)
		injectAdditionalPropertiesRec(params)
		if name == storymaker.ToolStory && slices.Contains(storymaker.PhaseOperationsFor(mode, phase, name), "patch_scene") {
			storymakerAllowNullRemoval(params)
		}
		out = append(out, definition)
	}
	return out
}

// StorymakerViewToolSchema is the screenshot tool of a job whose model can see. It is offered in the check and revise
// phases only, so it is not part of StorymakerPhaseToolSchemasFor. Like the phase tools it gets no strict schema.
func StorymakerViewToolSchema() openai.Tool {
	definition := tool(storymaker.ToolView,
		"Look at the saved story as pictures from the real player: one to four scenes (scene_ids), or no scene_ids for one overview sheet of all scenes. at_s picks seconds into a single scene; default is the moment the scene has settled. Pictures arrive in the next message.",
		schema(map[string]interface{}{
			"operation": storymakerEnum("screenshot"),
			"scene_ids": map[string]interface{}{"type": "array", "maxItems": 4, "items": storymakerTyped("string")},
			"at_s":      map[string]interface{}{"type": "number", "minimum": 0},
		}, "operation"))
	params := definition.Function.Parameters.(map[string]interface{})
	normalizeProviderFragileObjectSchemas(params)
	injectAdditionalPropertiesRec(params)
	return definition
}

// storymakerJobScope reports whether every tool the run may call is a Storymaker
// job tool. The chat tool is named storymaker, without the underscore, and does
// not count.
func storymakerJobScope(dc *DispatchContext) bool {
	if dc == nil || !dc.ToolScopeRestricted || len(dc.AllowedTools) == 0 {
		return false
	}
	for name := range dc.AllowedTools {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(name)), "storymaker_") {
			return false
		}
	}
	return true
}

// storymakerAllowNullRemoval keeps a null in a native patch_scene call:
// stripStrictNulls deletes it unless the schema of that field allows null.
// It runs after injectAdditionalPropertiesRec, which skips a type list, so each
// object field is closed here before it is made nullable.
func storymakerAllowNullRemoval(params map[string]interface{}) {
	data, _ := params["properties"].(map[string]interface{})["data"].(map[string]interface{})
	fields, _ := data["properties"].(map[string]interface{})
	for _, name := range storymakerSceneFields {
		if field, ok := fields[name].(map[string]interface{}); ok {
			injectAdditionalPropertiesRec(field)
			makeSchemaNullable(field)
		}
	}
}

func storymakerStoryTool(mode, phase string, compact bool, names StorymakerNames) openai.Tool {
	operations := storymaker.PhaseOperationsFor(mode, phase, storymaker.ToolStory)
	properties := map[string]interface{}{
		"operation":    storymakerEnum(operations...),
		"scene_id":     storymakerTyped("string"),
		"expected_rev": storymakerTyped("string"),
		"data":         schema(storymakerDataProperties(mode, phase, compact, names)),
	}
	if slices.Contains(operations, "move_scene") {
		properties["after"] = storymakerTyped("string")
	}
	if slices.Contains(operations, "dry_run") {
		properties["op"] = storymakerEnum("put_scene", "patch_scene")
	}
	description := "Job storyboard via data; positions: spot or x,y 0..100."
	if storymaker.EffectiveMode(mode) == storymaker.ModeFree {
		description = "Job scenes via data: every scene is a code scene (kind, narration, transition, duration); put_code takes data.source."
	}
	return tool(storymaker.ToolStory, description, schema(properties, "operation"))
}

func storymakerCatalogTool(mode, phase string, compact bool) openai.Tool {
	if storymaker.EffectiveMode(mode) == storymaker.ModeFree {
		return tool(storymaker.ToolCatalog,
			"Read the code reference with operation reference and topic code. A free project has no catalog assets.",
			schema(map[string]interface{}{
				"operation": storymakerEnum(storymaker.PhaseOperationsFor(mode, phase, storymaker.ToolCatalog)...),
				"topic":     storymakerEnum("code"),
			}, "operation"))
	}
	kinds := []string{"character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx"}
	if !compact || phase != storymaker.PhaseCast {
		kinds = append(kinds, "voice")
	}
	return tool(storymaker.ToolCatalog,
		"Find exact asset ids, or read the animation/code reference with operation reference and topic. Search terms are English.",
		schema(map[string]interface{}{
			"operation": storymakerEnum(storymaker.PhaseOperationsFor(mode, phase, storymaker.ToolCatalog)...),
			"topic":     storymakerEnum("animation", "code"),
			"query":     storymakerTyped("string"),
			"id":        storymakerTyped("string"),
			"limit":     map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 12},
			"kind":      storymakerEnum(kinds...),
		}, "operation"))
}

func storymakerCheckTool(mode, phase string) openai.Tool {
	return tool(storymaker.ToolCheck,
		"Check the saved story. Returns the current issues, advice, scene timing and compile state.",
		schema(map[string]interface{}{
			"operation": storymakerEnum(storymaker.PhaseOperationsFor(mode, phase, storymaker.ToolCheck)...),
		}, "operation"))
}

// storymakerDataProperties is the data object of one phase. Revise offers both
// set_meta and the scene operations, so it carries both field groups.
func storymakerDataProperties(mode, phase string, compact bool, names StorymakerNames) map[string]interface{} {
	if storymaker.EffectiveMode(mode) == storymaker.ModeFree {
		switch phase {
		case storymaker.PhaseOutline:
			return storymakerOutlineProperties()
		case storymaker.PhaseRevise:
			properties := storymakerFreeSceneProperties()
			properties["title"] = storymakerTyped("string")
			return properties
		default:
			return storymakerFreeSceneProperties()
		}
	}
	switch phase {
	case storymaker.PhaseOutline:
		return storymakerOutlineProperties()
	case storymaker.PhaseCast:
		return storymakerMetaProperties(compact, names)
	case storymaker.PhaseRevise:
		properties := storymakerSceneProperties(compact, names)
		for key, value := range storymakerMetaProperties(compact, names) {
			properties[key] = value
		}
		return properties
	default:
		return storymakerSceneProperties(compact, names)
	}
}

// storymakerFreeSceneProperties is the scene data of a free project: its picture is the code, so a scene has a kind, its
// narration and timing, and the source for put_code.
func storymakerFreeSceneProperties() map[string]interface{} {
	return map[string]interface{}{
		"kind":       storymakerEnum("code"),
		"source":     map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes, "description": "put_code only; first read get_code and the code reference"},
		"narration":  storymakerTyped("string"),
		"transition": storymakerEnum("cut", "fade", "slide-left", "wipe"),
		"duration":   prop("string", "auto or seconds"),
	}
}

func storymakerOutlineProperties() map[string]interface{} {
	scene := schema(map[string]interface{}{
		"id":        storymakerTyped("string"),
		"narration": storymakerTyped("string"),
	}, "id", "narration")
	return map[string]interface{}{
		"template":       storymakerEnum("product_explainer", "how_to", "concept_explainer", "dialogue_story", "data_story", "onboarding"),
		"target_seconds": storymakerTyped("integer"),
		"scenes":         map[string]interface{}{"type": "array", "minItems": 2, "maxItems": 24, "items": scene},
	}
}

func storymakerMetaProperties(compact bool, names StorymakerNames) map[string]interface{} {
	character := storymakerNames(names.Characters)
	if compact {
		// Asset IDs already travel in the phase packet/catalog.
		character = storymakerTyped("string")
	}
	member := map[string]interface{}{
		"id":        storymakerTyped("string"),
		"name":      storymakerTyped("string"),
		"character": character,
	}
	properties := map[string]interface{}{
		"title": storymakerTyped("string"),
		"cast":  storymakerArray(schema(member)),
		"style": schema(map[string]interface{}{
			"rhythm":   storymakerEnum("calm", "standard", "lively"),
			"auto_sfx": storymakerTyped("boolean"),
		}),
	}
	if !compact {
		member["voice"] = storymakerVoiceSchema()
		properties["voice"] = storymakerVoiceSchema()
		fields := properties["style"].(map[string]interface{})["properties"].(map[string]interface{})
		fields["palette"] = prop("string", "Palette name from the catalog")
		fields["captions"] = storymakerTyped("boolean")
	}
	return properties
}

func storymakerVoiceSchema() map[string]interface{} {
	return schema(map[string]interface{}{
		"provider": storymakerTyped("string"),
		"voice":    storymakerTyped("string"),
		"language": storymakerTyped("string"),
	})
}

func storymakerSceneProperties(compact bool, names StorymakerNames) map[string]interface{} {
	environment := storymakerNames(names.Environments)
	pose, expression := storymakerNames(names.Poses), storymakerNames(names.Expressions)
	if compact {
		environment = storymakerTyped("string")
		// Names remain in the phase packet and beat enums; avoid duplicating them.
		pose, expression = storymakerTyped("string"), storymakerTyped("string")
	}
	return map[string]interface{}{
		"kind":        storymakerEnum("storyboard", "code"),
		"source":      map[string]interface{}{"type": "string", "maxLength": storymaker.StudioCodeBytes, "description": "put_code only"},
		"environment": environment,
		"variant":     storymakerEnum("day", "night"),
		"narration":   storymakerTyped("string"),
		"speaker":     storymakerTyped("string"),
		"transition":  storymakerEnum("cut", "fade", "slide-left", "wipe"),
		"duration":    prop("string", "auto or seconds"),
		"camera": schema(map[string]interface{}{
			"framing": storymakerEnum("wide", "medium", "close"),
			"target":  storymakerTyped("string"),
		}),
		"actors": storymakerArray(schema(map[string]interface{}{
			"cast":       storymakerTyped("string"),
			"at":         storymakerTyped("string"),
			"pose":       pose,
			"facing":     storymakerEnum("left", "right", "auto"),
			"expression": expression,
			"scale":      storymakerTyped("number"),
		})),
		"props": storymakerArray(schema(map[string]interface{}{
			"id":    storymakerTyped("string"),
			"item":  storymakerTyped("string"),
			"at":    storymakerTyped("string"),
			"layer": storymakerEnum("back", "front"),
			"scale": storymakerTyped("number"),
		})),
		"beats": storymakerArray(storymakerBeatSchema(compact, names)),
	}
}

func storymakerBeatSchema(compact bool, names StorymakerNames) map[string]interface{} {
	properties := map[string]interface{}{
		"do":         storymakerEnum("enter", "exit", "walk", "move", "pose", "gesture", "emote", "look", "say", "show", "hide", "highlight", "camera", "sfx", "wait"),
		"who":        storymakerTyped("string"),
		"target":     storymakerTyped("string"),
		"from":       storymakerTyped("string"),
		"to":         storymakerTyped("string"),
		"at":         storymakerTyped("string"),
		"pose":       storymakerNames(names.Poses),
		"gesture":    storymakerNames(names.Gestures),
		"expression": storymakerNames(names.Expressions),
		"text":       storymakerTyped("string"),
		"style":      storymakerTyped("string"),
		"item":       storymakerTyped("string"),
		"entry":      storymakerEnum("pop", "fade", "slide", "none"),
		"op":         storymakerEnum("zoom", "pan", "focus", "shake"),
		"amount":     storymakerTyped("number"),
		"seconds":    storymakerTyped("number"),
		"with":       storymakerEnum("prev"),
		"cue":        storymakerTyped("integer"),
		"cue_phrase": map[string]interface{}{"type": "string", "maxLength": 60, "description": "narration words to cue on"},
	}
	properties["count"] = storymakerTyped("integer")
	properties["id"] = storymakerTyped("string")
	properties["sound"] = storymakerTyped("string")
	properties["delay"] = storymakerTyped("number")
	properties["duration"] = storymakerTyped("number")
	properties["ease"] = storymakerEnum("linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out")
	properties["scale"] = storymakerTyped("number")
	return schema(properties, "do")
}

func storymakerTyped(typ string) map[string]interface{} {
	return map[string]interface{}{"type": typ}
}

func storymakerEnum(values ...string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "enum": slices.Clone(values)}
}

func storymakerNames(names []string) map[string]interface{} {
	if len(names) == 0 {
		return storymakerTyped("string")
	}
	return storymakerEnum(names...)
}

func storymakerArray(items map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"type": "array", "items": items}
}
