package agent

import (
	"aurago/internal/videostudio/storyplan"

	openai "github.com/sashabaranov/go-openai"
)

// The Video Studio plan job tools. They never join the normal catalog: the profile of a plan job is their only
// route, and the action policy lets them through only with that job's hooks and tool scope.
const (
	VideoStudioPlanToolRead       = "vs_plan_read"
	VideoStudioPlanToolWrite      = "vs_plan_write"
	VideoStudioPlanToolPatchScene = "vs_plan_patch_scene"
	VideoStudioPlanToolCheck      = "vs_plan_check"
	VideoStudioPlanToolFinish     = "vs_plan_finish"
	// VideoStudioPlanToolPrefix starts the name of every plan job tool.
	VideoStudioPlanToolPrefix = "vs_plan_"
)

// The phases and the modes of a plan job.
const (
	VideoStudioPlanPhaseOutline = "outline"
	VideoStudioPlanPhaseDetails = "details"
	VideoStudioPlanPhaseCheck   = "check"
	VideoStudioPlanPhaseRevise  = "revise"

	VideoStudioPlanModeCreate = "create"
	VideoStudioPlanModeRevise = "revise"
)

// VideoStudioPlanPhaseTools lists the tools of one phase in the order of its schemas. An unknown phase has none.
func VideoStudioPlanPhaseTools(phase string) []string {
	switch phase {
	case VideoStudioPlanPhaseOutline:
		return []string{VideoStudioPlanToolRead, VideoStudioPlanToolWrite}
	case VideoStudioPlanPhaseDetails:
		return []string{VideoStudioPlanToolRead, VideoStudioPlanToolWrite, VideoStudioPlanToolPatchScene}
	case VideoStudioPlanPhaseCheck:
		return []string{VideoStudioPlanToolRead, VideoStudioPlanToolCheck, VideoStudioPlanToolPatchScene, VideoStudioPlanToolWrite, VideoStudioPlanToolFinish}
	case VideoStudioPlanPhaseRevise:
		return []string{VideoStudioPlanToolRead, VideoStudioPlanToolPatchScene, VideoStudioPlanToolCheck, VideoStudioPlanToolFinish}
	}
	return nil
}

// VideoStudioPlanToolSchemas returns the closed tool schemas of one phase. compact drops the property descriptions
// and negative_prompt, which a small model rarely needs; the prompt explains the fields instead.
func VideoStudioPlanToolSchemas(phase string, compact bool) []openai.Tool {
	out := []openai.Tool{}
	for _, name := range VideoStudioPlanPhaseTools(phase) {
		var definition openai.Tool
		switch name {
		case VideoStudioPlanToolRead:
			definition = tool(name, "Read the saved plan, the video model profile, what is available and the current issues.", schema(map[string]interface{}{}))
		case VideoStudioPlanToolWrite:
			definition = tool(name, "Save parts of the plan: style, characters, scenes, audio. A part you leave out stays as saved; scenes replaces the whole scene list. Returns the issues.",
				schema(map[string]interface{}{"plan": videoStudioPlanWriteSchema(phase, compact)}, "plan"))
		case VideoStudioPlanToolPatchScene:
			definition = tool(name, "Change the given fields of one scene; every other field stays as saved. Returns the issues.", schema(map[string]interface{}{
				"scene_id": videoStudioPlanProp("string", "Id of the scene to change, such as s2.", compact),
				"scene":    schema(videoStudioPlanSceneProperties(phase, compact, false)),
			}, "scene_id", "scene"))
		case VideoStudioPlanToolCheck:
			definition = tool(name, "Validate the saved plan and return its issues.", schema(map[string]interface{}{}))
		case VideoStudioPlanToolFinish:
			definition = tool(name, "Submit the plan for the user's review. Refused while blocking issues remain.", schema(map[string]interface{}{}))
		default:
			continue
		}
		params := definition.Function.Parameters.(map[string]interface{})
		normalizeProviderFragileObjectSchemas(params)
		injectAdditionalPropertiesRec(params)
		out = append(out, definition)
	}
	return out
}

// videoStudioPlanWriteSchema is the plan argument of vs_plan_write. The outline phase writes the scene list only.
func videoStudioPlanWriteSchema(phase string, compact bool) map[string]interface{} {
	scenes := map[string]interface{}{
		"type":     "array",
		"minItems": 1,
		"maxItems": storyplan.MaxScenes,
		"items":    schema(videoStudioPlanSceneProperties(phase, compact, true), "id", "title", "generate_seconds", "use_seconds", "transition_in"),
	}
	if phase == VideoStudioPlanPhaseOutline {
		return schema(map[string]interface{}{"scenes": scenes}, "scenes")
	}
	character := schema(map[string]interface{}{
		"name":        videoStudioPlanProp("string", "Name the prompts use for this character.", compact),
		"description": videoStudioPlanProp("string", "English description of the look: age, build, hair, clothes.", compact),
	}, "name", "description")
	return schema(map[string]interface{}{
		"style":      videoStudioPlanProp("string", "English style block shared by all clips: look, palette, lens, camera language.", compact),
		"characters": map[string]interface{}{"type": "array", "maxItems": storyplan.MaxCharacters, "items": character},
		"scenes":     scenes,
		"audio": schema(map[string]interface{}{
			"music": schema(map[string]interface{}{
				"prompt":       videoStudioPlanProp("string", "One English music prompt for the whole video; only when music is available.", compact),
				"instrumental": videoStudioPlanProp("boolean", "True for music without vocals.", compact),
			}),
			"clip_audio": storymakerEnum(storyplan.ClipAudioKeep, storyplan.ClipAudioDuck, storyplan.ClipAudioMute),
		}),
	})
}

// videoStudioPlanSceneProperties is one scene as the agent writes it. The outline phase writes the frame of a scene
// only. States, asset ids and costs belong to the server and are never offered; library images are the user's
// choice, so the agent gets no library source. withID adds the id, which a write needs and a patch names in scene_id.
func videoStudioPlanSceneProperties(phase string, compact, withID bool) map[string]interface{} {
	properties := map[string]interface{}{
		"title":            videoStudioPlanProp("string", "Short scene title in the plan language.", compact),
		"generate_seconds": videoStudioPlanProp("integer", "Clip length to generate, a value of profile.durations_seconds.", compact),
		"use_seconds":      videoStudioPlanProp("number", "Seconds of the clip the timeline shows, at most generate_seconds.", compact),
		"transition_in": schema(map[string]interface{}{
			"kind":    storymakerEnum(storyplan.TransitionCut, storyplan.TransitionContinue, storyplan.TransitionBridge, storyplan.TransitionBlend),
			"type":    videoStudioPlanProp("string", "Blend only: a type from availability.blend_types.", compact),
			"seconds": videoStudioPlanProp("number", "Blend only: overlap in seconds, 0.5 to 1.", compact),
		}, "kind"),
	}
	if withID {
		properties["id"] = videoStudioPlanProp("string", "Scene id such as s1; keep it stable.", compact)
	}
	if phase == VideoStudioPlanPhaseOutline {
		return properties
	}
	properties["video_prompt"] = videoStudioPlanProp("string", "English prompt of one shot: camera movement, subject, action, setting, light.", compact)
	properties["keyframe"] = schema(map[string]interface{}{
		"source":       storymakerEnum(storyplan.KeyframeGenerated, storyplan.KeyframeChain, storyplan.KeyframeBridge, storyplan.KeyframeNone),
		"image_prompt": videoStudioPlanProp("string", "English description of exactly the first frame; needed when source is generated and for a bridge scene.", compact),
	})
	properties["narration"] = schema(map[string]interface{}{
		"text": videoStudioPlanProp("string", "Narration in the plan language within the scene's word budget; only when TTS is available.", compact),
	})
	if !compact {
		properties["negative_prompt"] = prop("string", "English list of what the clip must not show.")
	}
	return properties
}

// videoStudioPlanProp is a typed property; compact leaves the description out.
func videoStudioPlanProp(typ, description string, compact bool) map[string]interface{} {
	if compact {
		return map[string]interface{}{"type": typ}
	}
	return prop(typ, description)
}
