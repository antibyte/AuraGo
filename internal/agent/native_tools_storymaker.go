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
var storymakerSceneFields = []string{"environment", "variant", "narration", "speaker", "transition", "duration", "camera", "actors", "props", "beats"}

// StorymakerPhaseToolSchemas returns the closed job tool schemas of one phase.
// The tools never join the normal catalog: a job profile is their only route.
func StorymakerPhaseToolSchemas(phase string, compact bool, names StorymakerNames) []openai.Tool {
	out := []openai.Tool{}
	for _, name := range storymaker.PhaseTools(phase) {
		var definition openai.Tool
		switch name {
		case storymaker.ToolStory:
			definition = storymakerStoryTool(phase, compact, names)
		case storymaker.ToolCatalog:
			definition = storymakerCatalogTool(phase, compact)
		case storymaker.ToolCheck:
			definition = storymakerCheckTool(phase)
		default:
			continue
		}
		params := definition.Function.Parameters.(map[string]interface{})
		normalizeProviderFragileObjectSchemas(params)
		injectAdditionalPropertiesRec(params)
		if name == storymaker.ToolStory && slices.Contains(storymaker.PhaseOperations(phase, name), "patch_scene") {
			storymakerAllowNullRemoval(params)
		}
		out = append(out, definition)
	}
	return out
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

func storymakerStoryTool(phase string, compact bool, names StorymakerNames) openai.Tool {
	operations := storymaker.PhaseOperations(phase, storymaker.ToolStory)
	properties := map[string]interface{}{
		"operation":    storymakerEnum(operations...),
		"scene_id":     storymakerTyped("string"),
		"expected_rev": storymakerTyped("string"),
		"data":         schema(storymakerDataProperties(phase, compact, names)),
	}
	if slices.Contains(operations, "move_scene") {
		properties["after"] = storymakerTyped("string")
	}
	if slices.Contains(operations, "dry_run") {
		properties["op"] = storymakerEnum("put_scene", "patch_scene")
	}
	return tool(storymaker.ToolStory,
		"Read and change this job's storyboard. Put the payload in data. Positions are a spot name or x,y in 0..100.",
		schema(properties, "operation"))
}

func storymakerCatalogTool(phase string, compact bool) openai.Tool {
	kinds := []string{"character", "prop", "environment", "pose", "gesture", "expression", "transition", "text_style", "sfx"}
	if !compact || phase != storymaker.PhaseCast {
		kinds = append(kinds, "voice")
	}
	return tool(storymaker.ToolCatalog,
		"Find exact catalog ids. search takes query and optional kind; describe takes id. Search terms are English.",
		schema(map[string]interface{}{
			"operation": storymakerEnum(storymaker.PhaseOperations(phase, storymaker.ToolCatalog)...),
			"query":     storymakerTyped("string"),
			"id":        storymakerTyped("string"),
			"limit":     map[string]interface{}{"type": "integer", "minimum": 1, "maximum": 12},
			"kind":      storymakerEnum(kinds...),
		}, "operation"))
}

func storymakerCheckTool(phase string) openai.Tool {
	return tool(storymaker.ToolCheck,
		"Check the saved story. Returns the current issues, advice, scene timing and compile state.",
		schema(map[string]interface{}{
			"operation": storymakerEnum(storymaker.PhaseOperations(phase, storymaker.ToolCheck)...),
		}, "operation"))
}

// storymakerDataProperties is the data object of one phase. Revise offers both
// set_meta and the scene operations, so it carries both field groups.
func storymakerDataProperties(phase string, compact bool, names StorymakerNames) map[string]interface{} {
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
	member := map[string]interface{}{
		"id":        storymakerTyped("string"),
		"name":      storymakerTyped("string"),
		"character": storymakerNames(names.Characters),
	}
	properties := map[string]interface{}{
		"title": storymakerTyped("string"),
		"cast":  storymakerArray(schema(member)),
	}
	if !compact {
		member["voice"] = storymakerVoiceSchema()
		properties["voice"] = storymakerVoiceSchema()
		properties["style"] = schema(map[string]interface{}{
			"palette":  prop("string", "Palette name from the catalog"),
			"captions": storymakerTyped("boolean"),
		})
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
	return map[string]interface{}{
		"environment": storymakerNames(names.Environments),
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
			"pose":       storymakerNames(names.Poses),
			"facing":     storymakerEnum("left", "right", "auto"),
			"expression": storymakerNames(names.Expressions),
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
	}
	if !compact {
		properties["count"] = storymakerTyped("integer")
		properties["id"] = storymakerTyped("string")
		properties["sound"] = storymakerTyped("string")
		properties["delay"] = storymakerTyped("number")
		properties["duration"] = storymakerTyped("number")
		properties["ease"] = storymakerEnum("linear", "ease-in", "ease-out", "ease-in-out", "back-out", "bounce-out")
	}
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
