package server

import (
	"context"
	"slices"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/storymaker"

	"github.com/sashabaranov/go-openai"
)

func TestStorymakerProfileOffersTheViewToolOnlyToAModelThatSees(t *testing.T) {
	names := agent.StorymakerNames{Poses: []string{"stand"}, Gestures: []string{"point"}, Expressions: []string{"neutral"}, Environments: []string{"env.office"}, Characters: []string{"char.office-woman-1"}}
	hasView := func(profile *agent.PreparedPromptProfile) bool {
		return slices.ContainsFunc(profile.Tools(), func(tool openai.Tool) bool { return tool.Function.Name == storymaker.ToolView })
	}
	for _, mode := range []string{storymaker.ModeCatalog, storymaker.ModeFree} {
		for _, phase := range []string{storymaker.PhaseOutline, storymaker.PhaseFill, storymaker.PhaseCheck, storymaker.PhaseRevise} {
			for _, text := range []bool{false, true} {
				seeing := storymaker.ViewPhase(phase)
				profile, err := storymakerAgentProfileFor(mode, phase, false, text, true, names)
				if err != nil {
					t.Fatalf("%s %s: %v", mode, phase, err)
				}
				system := profile.SystemPrompt()
				if hasView(profile) != seeing || strings.Contains(profile.Name(), "/vision") != seeing ||
					strings.Contains(system, "## Looking at the film") != seeing {
					t.Errorf("%s %s text=%v with vision: name %s, tool %v", mode, phase, text, profile.Name(), hasView(profile))
				}
				if want := text && seeing; strings.Contains(system, "storymaker_view operations: screenshot") != want ||
					strings.Contains(system, `{"action":"storymaker_view","operation":"screenshot","scene_ids":["s1"]}`) != want {
					t.Errorf("%s %s text=%v: the text guide names the view tool = %v, want %v", mode, phase, text, !want, want)
				}
				plain, err := storymakerAgentProfileFor(mode, phase, false, text, false, names)
				if err != nil {
					t.Fatal(err)
				}
				if hasView(plain) || strings.Contains(plain.Name(), "/vision") || strings.Contains(plain.SystemPrompt(), "storymaker_view") {
					t.Errorf("%s %s without vision must not mention the view tool: %s", mode, phase, plain.Name())
				}
			}
		}
	}
}

func TestStorymakerAttachScreenshots(t *testing.T) {
	j := &storymakerAgentJob{}
	j.pendingImages = []storymaker.StudioImage{{Label: "Scene s1 at 2.0 s", MIME: "image/jpeg", Data: []byte("abc")}}
	request := &openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "start"}}}
	if err := j.attachScreenshots(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if len(request.Messages) != 2 || len(j.pendingImages) != 0 {
		t.Fatalf("messages %d, pending %d", len(request.Messages), len(j.pendingImages))
	}
	last := request.Messages[1]
	if last.Role != openai.ChatMessageRoleUser || last.Name != "storymaker_view" || len(last.MultiContent) != 3 {
		t.Fatalf("image message %+v", last)
	}
	if got := last.MultiContent[2]; got.Type != openai.ChatMessagePartTypeImageURL || got.ImageURL.URL != "data:image/jpeg;base64,YWJj" || last.MultiContent[1].Text != "Scene s1 at 2.0 s" {
		t.Fatalf("image part %+v", got)
	}
	// The same request again keeps the picture until an assistant message followed it.
	if err := j.attachScreenshots(context.Background(), request); err != nil || len(request.Messages[1].MultiContent) != 3 || len(request.Messages) != 2 {
		t.Fatalf("a retry must keep the picture: %+v %v", request.Messages, err)
	}
	request.Messages = append(request.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "ok"})
	if err := j.attachScreenshots(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if old := request.Messages[1]; old.MultiContent != nil || old.Content != "Earlier Storymaker screenshot; take another one to see the current story." {
		t.Fatalf("the old picture must be replaced: %+v", old)
	}
}
