package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

func TestStoryWritesRefusedWhileAgentJobActive(t *testing.T) {
	s, rt, token := storymakerAgentRouteFixture(t, storymakerBlockingRun)
	p := createStorymakerProject(t, s, token)
	job, _, err := rt.agent.Enqueue(render.EnqueueRequest{ProjectID: p.Project.ID, Kind: render.KindAgent})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := storymaker.CanonicalJSON(p.Story)
	if err != nil {
		t.Fatal(err)
	}
	base := "/projects/" + p.Project.ID
	headers := map[string]string{"If-Match": p.ETag}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, base + "/story", string(canonical)},
		{http.MethodPut, base + "/scenes/s1/code", "export default {};"},
		{http.MethodPost, base + "/narrate", ""},
		{http.MethodPost, base + "/render", "{}"},
		{http.MethodPost, base + "/make-silent", ""},
		{http.MethodPost, base + "/restore-speech", ""},
	} {
		w := storymakerRequest(s, token, tc.method, tc.path, strings.NewReader(tc.body), headers)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "agent_job_active") {
			t.Errorf("%s %s = %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// The guard ends with the job.
	if _, found := rt.agent.Cancel(job.ID); !found {
		t.Fatal("the job is gone")
	}
	waitStorymakerManagerJob(t, rt.agent, job.ID)
	w := storymakerRequest(s, token, http.MethodPut, base+"/story", strings.NewReader(string(canonical)), headers)
	if w.Code != http.StatusOK {
		t.Fatalf("save after the job = %d %s", w.Code, w.Body.String())
	}
}

func TestStorymakerMakeSilentRaceKeepsTheBackup(t *testing.T) {
	s, rt, token := storymakerAgentRouteFixture(t, storymakerBlockingRun)
	p := createStorymakerProject(t, s, token)
	ctx := context.Background()
	story := p.Story
	story.Scenes = []storymaker.Scene{{ID: "s1", Narration: "Hello there, this is a narrated scene."}}
	canonical, err := storymaker.CanonicalJSON(story)
	if err != nil {
		t.Fatal(err)
	}
	etag, err := rt.projects.SaveStory(ctx, p.Project.ID, canonical, p.ETag)
	if err != nil {
		t.Fatal(err)
	}
	acc := &storymakerAccess{cfg: s.ConfigSnapshot(), rt: rt}

	const callers = 8
	errs := make([]error, callers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, _, _, errs[i] = storymakerMakeSilent(ctx, acc, p.Project.ID, etag)
		}(i)
	}
	close(start)
	wg.Wait()

	succeeded := 0
	for i, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case !errors.Is(err, storymaker.ErrConflict):
			t.Errorf("caller %d: %v", i, err)
		}
	}
	if succeeded != 1 {
		t.Fatalf("%d callers succeeded, want 1", succeeded)
	}
	backup, err := rt.projects.LoadSilentBackup(ctx, p.Project.ID)
	if err != nil {
		t.Fatalf("the speech backup is gone: %v", err)
	}
	if string(backup) != string(canonical) {
		t.Fatalf("the backup is not the original story: %s", backup)
	}
}
