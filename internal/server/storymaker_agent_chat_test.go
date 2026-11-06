package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/storymaker"
	"aurago/internal/storymaker/render"
)

func storymakerFacadeCode(err error) string {
	var refused *storymaker.AgentError
	if errors.As(err, &refused) {
		return refused.Code
	}
	return ""
}

func TestStorymakerFacadeDailyLimitFailsClosedAndReadOnly(t *testing.T) {
	s, _, writeToken := storymakerAPIFixture(t)
	on := *s.ConfigSnapshot()
	on.Storymaker.Agent.Enabled = true
	s.replaceConfigSnapshot(&on)
	project := createStorymakerProject(t, s, writeToken)
	projectID := project.Project.ID
	// Revise refuses a blank story before it counts, so this one has a filled scene.
	story := `{"schema":"storymaker/1","title":"Limit","format":{"aspect":"16:9","fps":30},` +
		`"cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],"scenes":[{"id":"s1","environment":"env.office","narration":"Eine Szene.",` +
		`"actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"gesture","who":"anna","gesture":"point"}]}]}`
	if saved := storymakerRequest(s, writeToken, http.MethodPut, "/projects/"+projectID+"/story", strings.NewReader(story), map[string]string{"If-Match": project.ETag}); saved.Code != http.StatusOK {
		t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
	}

	ctx := context.Background()
	f := &storymakerAgentFacade{s: s}
	if !f.Available() {
		t.Fatal("the facade must be available with both switches and the desktop on")
	}
	writes := map[string]func() error{
		"create": func() error {
			_, err := f.Create(ctx, storymaker.AgentCreateRequest{Topic: "Was ist eine Firewall?"})
			return err
		},
		"revise": func() error {
			_, err := f.Revise(ctx, projectID, "Mach Szene 1 kürzer.")
			return err
		},
		"render": func() error {
			_, err := f.Render(ctx, projectID)
			return err
		},
		"render silent": func() error {
			_, _, err := f.RenderSilent(ctx, projectID)
			return err
		},
	}

	ro := *s.ConfigSnapshot()
	ro.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&ro)
	for name, call := range writes {
		if code := storymakerFacadeCode(call()); code != "desktop_readonly" {
			t.Errorf("read-only %s: code %q", name, code)
		}
	}
	if status, err := f.Status(ctx, projectID); err != nil || status.State != storymakerFacadeNoJobState {
		t.Errorf("read-only status: %+v, %v", status, err)
	}
	if rows, err := f.List(ctx); err != nil || len(rows) != 1 {
		t.Errorf("read-only list: %+v, %v", rows, err)
	}
	ro.VirtualDesktop.ReadOnly = false
	s.replaceConfigSnapshot(&ro)

	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	usage := filepath.Join(storymakerDir(rt.dataDir), "agent", "usage.json")
	if err := os.MkdirAll(filepath.Dir(usage), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usage, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, call := range writes {
		if code := storymakerFacadeCode(call()); code != "daily_limit_reached" {
			t.Errorf("unreadable count, %s: code %q", name, code)
		}
	}
	if rows, err := f.List(ctx); err != nil || len(rows) != 1 {
		t.Errorf("a refused create must leave no project: %+v, %v", rows, err)
	}
}

func TestStorymakerRenderSilentRestoresOnRefusal(t *testing.T) {
	s, _, writeToken := storymakerAPIFixture(t)
	on := *s.ConfigSnapshot()
	on.Storymaker.Agent.Enabled = true
	// This FFmpeg path does not exist, so the export gate refuses before the speech is removed.
	on.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg")
	s.replaceConfigSnapshot(&on)
	project := createStorymakerProject(t, s, writeToken)
	projectID := project.Project.ID
	// A scene with narration, so a silent save would change the story.
	story := `{"schema":"storymaker/1","title":"Silent","format":{"aspect":"16:9","fps":30},` +
		`"cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],"scenes":[{"id":"s1","environment":"env.office","narration":"Eine Szene.",` +
		`"actors":[{"cast":"anna","at":"left","pose":"present"}],"beats":[{"do":"gesture","who":"anna","gesture":"point"}]}]}`
	if saved := storymakerRequest(s, writeToken, http.MethodPut, "/projects/"+projectID+"/story", strings.NewReader(story), map[string]string{"If-Match": project.ETag}); saved.Code != http.StatusOK {
		t.Fatalf("save = %d %s", saved.Code, saved.Body.String())
	}

	ctx := context.Background()
	rt, err := s.storymakerRuntime()
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	f := &storymakerAgentFacade{s: s}
	_, _, err = f.RenderSilent(ctx, projectID)
	if code := storymakerFacadeCode(err); code != render.CodeFFmpegUnavailable {
		t.Fatalf("silent render code %q (%v), want %s", code, err, render.CodeFFmpegUnavailable)
	}
	after, _, err := rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("a refused silent export changed the story")
	}
	if has, err := rt.projects.HasSilentBackup(ctx, projectID); has || err != nil {
		t.Fatalf("a refused silent export left a backup: %v %v", has, err)
	}

	// The restore that runs after a refused save: the kept copy goes back with the normal save and is removed.
	if err := rt.projects.SaveSilentBackup(ctx, projectID, before); err != nil {
		t.Fatal(err)
	}
	_, etag, err := rt.projects.LoadStory(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if err := storymakerRestoreSilent(ctx, &storymakerAccess{rt: rt}, projectID, etag); err != nil {
		t.Fatal(err)
	}
	if has, err := rt.projects.HasSilentBackup(ctx, projectID); has || err != nil {
		t.Fatalf("restore left the backup: %v %v", has, err)
	}
}
