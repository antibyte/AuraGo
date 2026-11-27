package server

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"testing"

	"aurago/internal/storymaker"

	"github.com/sashabaranov/go-openai"
)

var smFillSceneRe = regexp.MustCompile(`"scene":\{"id":"([a-z0-9-]+)"`)

// A scene that does not get filled in pass 1 is retried only after every other scene had its turn.
func TestStorymakerAgentFillPassesRetryAfterTheOthers(t *testing.T) {
	turns := smScriptHappyTurns()
	turns[storymaker.PhaseFill] = []smScriptTurn{
		{prose: "Ich plane."},
		{prose: "Noch nicht."},
		smScriptStoryCall("put_scene", "s2", smScriptSceneTwo()),
		smScriptStoryCall("put_scene", "s1", smScriptSceneOne()),
	}
	client := &smScriptClient{turns: turns}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	smScriptRequireReady(t, job)
	smScriptRequireFilled(t, smScriptStory(t, env), "s1", "s2")

	var order []string
	for _, req := range client.seen() {
		if smScriptPhase(req) != storymaker.PhaseFill {
			continue
		}
		for _, m := range req.Messages {
			if m.Role != openai.ChatMessageRoleUser {
				continue
			}
			if match := smFillSceneRe.FindStringSubmatch(m.Content); match != nil && (len(order) == 0 || order[len(order)-1] != match[1]) {
				order = append(order, match[1])
			}
		}
	}
	if want := []string{"s1", "s2", "s1"}; !slices.Equal(order, want) {
		t.Fatalf("fill order = %v, want %v", order, want)
	}
}

// A scene that another writer filled while pass 2 ran gets no request of its own any more.
func TestStorymakerAgentFillPassesSkipScenesAnotherLoopFilled(t *testing.T) {
	turns := smScriptHappyTurns()
	turns[storymaker.PhaseFill] = []smScriptTurn{
		{prose: "Ich plane."},
		{prose: "Noch nicht."},
		{prose: "Ich plane auch hier."},
		{prose: "Hier noch nicht."},
		smScriptStoryCall("put_scene", "s1", smScriptSceneOne()),
	}
	client := &smScriptClient{turns: turns}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	fillRequests := 0
	client.beforeReply = func(req openai.ChatCompletionRequest) {
		if smScriptPhase(req) != storymaker.PhaseFill {
			return
		}
		fillRequests++
		if fillRequests != 5 { // the first request of pass 2, scene s1
			return
		}
		raw, etag, err := env.rt.projects.LoadStory(context.Background(), env.id)
		if err != nil {
			t.Error(err)
			return
		}
		story, _, err := storymaker.DecodeStory(raw, storymaker.DefaultLimits())
		if err != nil {
			t.Error(err)
			return
		}
		data, _ := json.Marshal(smScriptSceneTwo())
		var filled storymaker.Scene
		if err := json.Unmarshal(data, &filled); err != nil {
			t.Error(err)
			return
		}
		for i := range story.Scenes {
			if story.Scenes[i].ID == "s2" {
				story.Scenes[i].Environment, story.Scenes[i].Actors = filled.Environment, filled.Actors
				story.Scenes[i].Beats, story.Scenes[i].Transition = filled.Beats, filled.Transition
			}
		}
		if raw, err = json.Marshal(story); err == nil {
			_, err = env.rt.projects.SaveStory(context.Background(), env.id, raw, etag)
		}
		if err != nil {
			t.Error(err)
		}
	}
	job := smScriptWait(t, env.rt.agent, smScriptEnqueue(t, env.rt.agent, env).ID)
	smScriptRequireReady(t, job)
	smScriptRequireFilled(t, smScriptStory(t, env), "s1", "s2")

	var order []string
	for _, req := range client.seen() {
		if smScriptPhase(req) != storymaker.PhaseFill {
			continue
		}
		for _, m := range req.Messages {
			if m.Role != openai.ChatMessageRoleUser {
				continue
			}
			if match := smFillSceneRe.FindStringSubmatch(m.Content); match != nil && (len(order) == 0 || order[len(order)-1] != match[1]) {
				order = append(order, match[1])
			}
		}
	}
	if want := []string{"s1", "s2", "s1"}; !slices.Equal(order, want) {
		t.Fatalf("fill order = %v, want %v", order, want)
	}
	// The scene is skipped before its loop starts, so pass 2 announces no phase for it.
	for _, line := range env.rt.agentLog.after(job.ID, 0) {
		var ev struct {
			Type    string `json:"type"`
			Phase   string `json:"phase"`
			SceneID string `json:"scene_id"`
			Attempt int    `json:"attempt"`
		}
		if json.Unmarshal(line.Data, &ev) == nil && ev.Type == agentEventPhase && ev.Phase == storymaker.PhaseFill && ev.SceneID == "s2" && ev.Attempt == 2 {
			t.Fatalf("pass 2 started a loop for s2, which was already filled: %s", line.Data)
		}
	}
}
