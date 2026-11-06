package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"

	"github.com/sashabaranov/go-openai"
)

// storymakerAgentRouteFixture makes a server whose agent jobs run the given function. Render and audio
// jobs block until they are cancelled.
func storymakerAgentRouteFixture(t *testing.T, run render.RunFunc) (*Server, *storymakerRuntime, string) {
	t.Helper()
	s, _, writeToken := storymakerAPIFixture(t)
	s.Cfg.Storymaker.Agent.Enabled = true
	s.Cfg.Server.UILanguage = "en"
	s.Cfg.LLM.Model = "test-model"
	s.LLMClient = llm.WrapOpenAIClient(openai.NewClientWithConfig(openai.DefaultConfig("test-only")))
	s.storymakerRunOverride = storymakerBlockingRun
	s.storymakerAudioRunOverride = storymakerBlockingRun
	s.storymakerAgentRunOverride = run
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	return s, rt, writeToken
}

func TestStorymakerAgentRoutesRefuseDisabledReadOnlyAndBusy(t *testing.T) {
	s, rt, token := storymakerAgentRouteFixture(t, storymakerBlockingRun)
	start := func(body string) *httptest.ResponseRecorder {
		return storymakerRequest(s, token, http.MethodPost, "/agent/jobs", strings.NewReader(body), nil)
	}
	want := func(name string, w *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if w.Code != status || !strings.Contains(w.Body.String(), code) {
			t.Fatalf("%s = %d %s, want %d %s", name, w.Code, w.Body.String(), status, code)
		}
	}
	withConfig := func(change func(*config.Config), check func()) {
		t.Helper()
		before := *s.ConfigSnapshot()
		next := before
		change(&next)
		s.replaceConfigSnapshot(&next)
		check()
		s.replaceConfigSnapshot(&before)
	}

	want("blank topic", start(`{"topic":"   "}`), http.StatusBadRequest, "invalid_topic")
	withConfig(func(c *config.Config) { c.VirtualDesktop.ReadOnly = true }, func() {
		want("read-only", start(`{"topic":"Backups"}`), http.StatusForbidden, "desktop_readonly")
	})
	withConfig(func(c *config.Config) { c.Storymaker.Enabled = false }, func() {
		want("storymaker off", start(`{"topic":"Backups"}`), http.StatusForbidden, "storymaker_disabled")
	})
	withConfig(func(c *config.Config) { c.Storymaker.Agent.Enabled = false }, func() {
		want("agent off", start(`{"topic":"Backups"}`), http.StatusForbidden, "agent_disabled")
	})

	w := start(`{"topic":"Was ist eine Firewall?"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start = %d %s", w.Code, w.Body.String())
	}
	var started struct {
		ProjectID string     `json:"project_id"`
		Job       render.Job `json:"job"`
	}
	decodeBody(t, w, &started)
	if !storymaker.ValidProjectID(started.ProjectID) || started.Job.Kind != render.KindAgent {
		t.Fatalf("start answer = %+v", started)
	}

	// A second job of the same project is refused and names the job in the way.
	var busy struct {
		Code  string `json:"code"`
		JobID string `json:"job_id"`
	}
	w = start(`{"topic":"Noch einmal","project_id":"` + started.ProjectID + `"}`)
	want("second start", w, http.StatusConflict, render.CodeBusy)
	decodeBody(t, w, &busy)
	if busy.JobID != started.Job.ID {
		t.Fatalf("busy job_id = %q, want %q", busy.JobID, started.Job.ID)
	}

	// A narrate job of another project keeps the agent out of that project.
	other := createStorymakerProject(t, s, token)
	enqueueStorymakerJob(t, rt.narrate, other.Project.ID, render.KindNarrate)
	w = storymakerRequest(s, token, http.MethodPost, "/projects/"+other.Project.ID+"/agent/revise", strings.NewReader(`{"instruction":"Kürzer"}`), nil)
	want("revise during narration", w, http.StatusConflict, render.CodeBusy)

	var status struct {
		Agent struct {
			Enabled   bool   `json:"enabled"`
			Available bool   `json:"available"`
			Model     string `json:"model"`
		} `json:"agent"`
	}
	decodeBody(t, storymakerRequest(s, token, http.MethodGet, "/status", nil, nil), &status)
	if !status.Agent.Enabled || !status.Agent.Available || status.Agent.Model != "test-model" {
		t.Fatalf("status agent = %+v", status.Agent)
	}
}
