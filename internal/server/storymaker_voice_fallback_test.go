package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
	"aurago/internal/storymaker/storymakertest"
)

// A story that was spoken with another voice keeps its speech: the answer finds the recording by
// its words and names the blocks as revoiced, so the panel can offer the narration of the current
// voice without the film falling silent.
func TestStorymakerNarrationKeepsTheAudioOfAnotherVoice(t *testing.T) {
	s, readToken, writeToken := storymakerAPIFixture(t)
	ctx := context.Background()
	p := createStorymakerProject(t, s, writeToken)
	id := p.Project.ID
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}

	const body = `{"schema":"storymaker/1","title":"Alt","voice":{"provider":"phonon","voice":"female-de","language":"de"},` +
		`"scenes":[{"id":"s1","narration":"Das ist ein ganzer Satz zur Firewall."}]}`
	musicKeyPutStory(t, s, writeToken, id, p.ETag, body)

	// The speech of the story, made with the voice of that moment.
	data, _, err := rt.projects.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var story storymaker.Story
	if err := json.Unmarshal(data, &story); err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	old := storymaker.Voice{Provider: "sanotts", Voice: "thorsten", Language: "de"}
	story.Voice = &old
	if err := storymakertest.SeedNarration(ctx, rt.projects, id, &story, 1); err != nil {
		t.Fatal(err)
	}

	// The voice of the story is the one of the policy now: the speech is found by its words.
	answer := musicKeyAnswer(t, s, readToken, id)
	if len(answer.Missing) != 0 {
		t.Fatalf("the speech of another voice counts as missing: %+v", answer.Missing)
	}
	if len(answer.Narration["s1"].Blocks) != 1 {
		t.Fatalf("the narration of the scene: %+v", answer.Narration["s1"])
	}
	if len(answer.Revoiced) != 1 || answer.Revoiced[0].BlockID != "s1:n" {
		t.Fatalf("revoiced = %+v", answer.Revoiced)
	}
}

// The narration route asks for the exact answer: a block that was spoken with another voice has to
// be made again with the voice of the story, so the route starts a job.
func TestStorymakerNarrateMakesTheVoiceOfTheStoryAgain(t *testing.T) {
	s, _, writeToken := storymakerAPIFixture(t)
	s.storymakerVoicesOverride = readyStorymakerPhononVoices
	release := make(chan struct{})
	// Registered after the fixture's cleanup, so it runs first and the manager can close.
	t.Cleanup(func() { close(release) })
	s.storymakerAudioRunOverride = func(ctx context.Context, job render.Job, payload any, h *render.Handle) (render.Output, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return render.Output{}, nil
	}
	ctx := context.Background()
	p := createStorymakerProject(t, s, writeToken)
	id := p.Project.ID
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}

	const body = `{"schema":"storymaker/1","title":"Alt","voice":{"provider":"phonon","voice":"female-de","language":"de"},` +
		`"scenes":[{"id":"s1","narration":"Das ist ein ganzer Satz zur Firewall."}]}`
	musicKeyPutStory(t, s, writeToken, id, p.ETag, body)

	// The speech of the story, made with the voice of that moment.
	data, _, err := rt.projects.LoadStory(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	var story storymaker.Story
	if err := json.Unmarshal(data, &story); err != nil {
		t.Fatal(err)
	}
	storymaker.ApplyDefaults(&story)
	old := storymaker.Voice{Provider: "sanotts", Voice: "thorsten", Language: "de"}
	story.Voice = &old
	if err := storymakertest.SeedNarration(ctx, rt.projects, id, &story, 1); err != nil {
		t.Fatal(err)
	}

	w := storymakerRequest(s, writeToken, http.MethodPost, "/projects/"+id+"/narrate", nil, nil)
	if w.Code != http.StatusAccepted {
		t.Fatalf("narrate = %d %s", w.Code, w.Body.String())
	}
	var body2 struct {
		Job *render.Job `json:"job"`
	}
	decodeBody(t, w, &body2)
	if body2.Job == nil || body2.Job.Kind != render.KindNarrate {
		t.Fatalf("the job of the narration: %s", w.Body.String())
	}
}
