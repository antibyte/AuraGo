package agent

import openai "github.com/sashabaranov/go-openai"

// storymakerChatSchema describes the chat tool. It appears only while the server publishes an
// available facade (resolveToolFeatureState) and is non-displacing (prompts.NonDisplacingTools).
func storymakerChatSchema() openai.Tool {
	return tool("storymaker",
		"Explainer videos in the Storymaker desktop app: create a story from a topic as a background job, read its status, revise it, export an MP4, open the project or list projects.",
		schema(map[string]interface{}{
			"operation":      operationProperty("create starts a job from a topic, status reads the latest job, revise changes a story, render exports, open shows the project, list names recent projects", []string{"create", "status", "revise", "render", "open", "list"}),
			"topic":          prop("string", "create: what the film explains, 1-2000 characters"),
			"audience":       prop("string", "create: who it is for, at most 120 characters"),
			"length_seconds": prop("integer", "create: target length in seconds, default 60; the server clamps it"),
			"format": map[string]interface{}{
				"type":        "string",
				"description": "create: picture format, default 16:9",
				"enum":        []string{"16:9", "9:16", "1:1"},
			},
			"language": prop("string", "create: language of the film, such as de; default is the UI language"),
			"voice": map[string]interface{}{
				"type":        "object",
				"description": "create: optional voice; leave it out to use the configured one",
				"properties": map[string]interface{}{
					"provider": prop("string", "voice provider id"),
					"voice":    prop("string", "voice id"),
				},
			},
			"project_id":  prop("string", "status, revise, render, open: project id from create or list"),
			"instruction": prop("string", "revise: the change to make, 1-2000 characters"),
			"silent":      prop("boolean", "render: export without speech when no voice is ready; the app can restore the speech"),
		}, "operation"),
	)
}
