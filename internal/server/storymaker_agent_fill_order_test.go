package server

import (
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
