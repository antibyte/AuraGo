package storymaker

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

var (
	// storyArgFields are the arguments of storymaker_story itself. Other keys belong to data.
	storyArgFields = nameSet("scene_id", "after", "op", "expected_rev", "data")
	// storyArgDropped never reach a handler: the operation and tool name of the call, and ids the server binds.
	storyArgDropped = nameSet("operation", "action", "tool", "project_id", "job_id")
	// storySceneAliases are accepted for scene_id when their value is text.
	storySceneAliases = []string{"id", "scene", "sceneId", "sceneID"}
)

// studioArgString returns a text argument without surrounding space, or "" for any other value.
func studioArgString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return strings.TrimSpace(v)
}

// repairStoryArgs brings the arguments of a storymaker_story call into their canonical form. It
// wraps flat fields into data (a value already in data wins), decodes data given as a JSON string,
// accepts the scene id aliases and turns the string forms of at, from, to and duration into values
// the story format reads. Anything it cannot repair stays, so that the handler or DecodeScene names
// the problem. Nested maps of args are changed in place.
func repairStoryArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	flat := map[string]any{}
	for key, value := range args {
		switch {
		case storyArgDropped[key]:
		case storyArgFields[key]:
			out[key] = value
		case containsName(storySceneAliases, key):
			if _, isText := value.(string); !isText {
				flat[key] = value
			}
		default:
			flat[key] = value
		}
	}
	if studioArgString(out, "scene_id") == "" {
		for _, alias := range storySceneAliases {
			if id := studioArgString(args, alias); id != "" {
				out["scene_id"] = id
				break
			}
		}
	}
	raw := out["data"]
	if text, isText := raw.(string); isText {
		var decoded any
		if json.Unmarshal([]byte(text), &decoded) == nil {
			raw = decoded
		}
	}
	data, isObject := raw.(map[string]any)
	if raw != nil && !isObject {
		out["data"] = raw
		return out
	}
	if data == nil && len(flat) == 0 {
		delete(out, "data")
		return out
	}
	if data == nil {
		data = map[string]any{}
	}
	for key, value := range flat {
		if _, taken := data[key]; !taken {
			data[key] = value
		}
	}
	for _, key := range []string{"scene_id", "id"} {
		if id, isText := data[key].(string); isText {
			if studioArgString(out, "scene_id") == "" && strings.TrimSpace(id) != "" {
				out["scene_id"] = strings.TrimSpace(id)
			}
			delete(data, key)
		}
	}
	repairSceneForms(data)
	out["data"] = data
	return out
}

// repairSceneForms canonicalises the string forms of one scene object.
func repairSceneForms(data map[string]any) {
	if text, isText := data["duration"].(string); isText {
		data["duration"] = repairDuration(text)
	}
	for _, list := range []string{"actors", "props"} {
		storyArgObjects(data[list], func(obj map[string]any) { repairPosition(obj, "at") })
	}
	storyArgObjects(data["beats"], func(obj map[string]any) {
		for _, key := range []string{"at", "from", "to"} {
			repairPosition(obj, key)
		}
	})
}

func storyArgObjects(list any, visit func(map[string]any)) {
	items, _ := list.([]any)
	for _, item := range items {
		if obj, isObject := item.(map[string]any); isObject {
			visit(obj)
		}
	}
}

// repairDuration turns "6" or "6s" into the number 6. "auto" and anything unreadable stay as they are.
func repairDuration(text string) any {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "auto" {
		return "auto"
	}
	if seconds, ok := parseStoryNumber(strings.TrimSuffix(t, "s")); ok {
		return seconds
	}
	return text
}

// repairPosition turns "x,y" into a point. A spot name stays.
func repairPosition(obj map[string]any, key string) {
	text, isText := obj[key].(string)
	if !isText {
		return
	}
	left, right, found := strings.Cut(text, ",")
	if !found {
		return
	}
	x, okX := parseStoryNumber(left)
	y, okY := parseStoryNumber(right)
	if okX && okY {
		obj[key] = map[string]any{"x": x, "y": y}
	}
}

func parseStoryNumber(text string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
