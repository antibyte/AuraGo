package server

import (
	"net/http"
	"strings"
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
