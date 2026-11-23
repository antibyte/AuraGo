package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/storymaker"
)

func TestStorymakerCreateProjectAcceptsTheMode(t *testing.T) {
	s, _, token := storymakerAPIFixture(t)
	free := createStorymakerProjectMode(t, s, token, "free")
	if free.Project.Mode != storymaker.ModeFree {
		t.Fatalf("free project mode = %q", free.Project.Mode)
	}
	if plain := createStorymakerProject(t, s, token); plain.Project.Mode != "" {
		t.Fatalf("a project without mode got %q", plain.Project.Mode)
	}
	if w := storymakerRequest(s, token, http.MethodPost, "/projects", strings.NewReader(`{"title":"x","mode":"catalog"}`), nil); w.Code != http.StatusCreated {
		t.Fatalf("catalog mode = %d %s", w.Code, w.Body.String())
	}
	bad := storymakerRequest(s, token, http.MethodPost, "/projects", strings.NewReader(`{"title":"x","mode":"templates"}`), nil)
	if bad.Code != http.StatusBadRequest || !strings.Contains(bad.Body.String(), "invalid_mode") {
		t.Fatalf("junk mode = %d %s", bad.Code, bad.Body.String())
	}
}

func TestStorymakerFacadeCreateTakesTheMode(t *testing.T) {
	client := &smScriptClient{turns: map[string][]smScriptTurn{}}
	env := newSMScriptEnv(t, smScriptNativeModel, client)
	ctx := context.Background()
	f := &storymakerAgentFacade{s: env.s}

	_, err := f.Create(ctx, storymaker.AgentCreateRequest{Topic: "Firewall", Mode: "templates"})
	if code := storymakerFacadeCode(err); code != "invalid_mode" {
		t.Fatalf("junk mode: code %q, error %v", code, err)
	}
	ref, err := f.Create(ctx, storymaker.AgentCreateRequest{Topic: "Firewall", Mode: "free", LengthSeconds: 15, Language: "de"})
	if err != nil {
		t.Fatal(err)
	}
	smScriptWait(t, env.rt.agent, ref.JobID) // the scripted client has no turns, so the job ends without an outline
	project, err := env.rt.projects.Get(ctx, ref.ProjectID)
	if err != nil || project.Mode != storymaker.ModeFree {
		t.Fatalf("created project = %+v, %v", project, err)
	}
	// The refused request left nothing behind: the fixture project and the created one.
	if rows, err := f.List(ctx); err != nil || len(rows) != 2 {
		t.Fatalf("projects = %+v, %v", rows, err)
	}
}

func TestStorymakerAgentJobsTakeTheMode(t *testing.T) {
	client := &smScriptClient{turns: map[string][]smScriptTurn{}}
	env := newSMScriptEnvMode(t, smScriptNativeModel, client, storymaker.ModeFree)

	// A named project keeps its mode; a different one is a mismatch.
	mismatch := storymakerRequest(env.s, env.token, http.MethodPost, "/agent/jobs",
		strings.NewReader(`{"topic":"Firewall","project_id":"`+env.id+`","mode":"catalog"}`), nil)
	if mismatch.Code != http.StatusUnprocessableEntity || !strings.Contains(mismatch.Body.String(), "mode_mismatch") {
		t.Fatalf("mismatch = %d %s", mismatch.Code, mismatch.Body.String())
	}
	junk := storymakerRequest(env.s, env.token, http.MethodPost, "/agent/jobs", strings.NewReader(`{"topic":"Firewall","mode":"templates"}`), nil)
	if junk.Code != http.StatusBadRequest || !strings.Contains(junk.Body.String(), "invalid_mode") {
		t.Fatalf("junk mode = %d %s", junk.Code, junk.Body.String())
	}

	// A request that creates the project sets its mode.
	created := storymakerRequest(env.s, env.token, http.MethodPost, "/agent/jobs", strings.NewReader(`{"topic":"Firewall","mode":"free","length_seconds":15,"language":"de"}`), nil)
	if created.Code != http.StatusAccepted {
		t.Fatalf("create = %d %s", created.Code, created.Body.String())
	}
	var answer struct {
		ProjectID string `json:"project_id"`
		Job       struct {
			ID string `json:"id"`
		} `json:"job"`
	}
	decodeBody(t, created, &answer)
	smScriptWait(t, env.rt.agent, answer.Job.ID) // the scripted client has no turns, so the job ends without an outline
	project, err := env.rt.projects.Get(context.Background(), answer.ProjectID)
	if err != nil || project.Mode != storymaker.ModeFree {
		t.Fatalf("created project = %+v, %v", project, err)
	}
}
