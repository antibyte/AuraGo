package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/storymaker/render"
)

// smappCreate makes a project through the real API and returns its id and etag.
func smappCreate(t *testing.T, origin, title string) (string, string) {
	t.Helper()
	res, err := http.Post(origin+storymakerAPIBase+"/projects", "application/json", strings.NewReader(`{"title":`+smappJS(title)+`}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
		ETag string `json:"etag"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("create %q = %d %v", title, res.StatusCode, err)
	}
	return created.Project.ID, created.ETag
}

// TestStorymakerAppRealServerStatesBrowser opens the app against the real handlers in the states the server can be in: a
// browser that the first export downloads, no FFmpeg, read-only, switched off, a story that somebody else changed, and a
// story.json that was edited into something unreadable. (The stand-in server of package ui covers the same list for the
// app alone; these are the answers the real handlers give.) Gated by AURAGO_RUN_BROWSER_SMOKE=1.
func TestStorymakerAppRealServerStatesBrowser(t *testing.T) {
	t.Run("browser is downloaded by the first export", func(t *testing.T) {
		s := storymakerRealServer(t)
		s.Cfg.Storymaker.Render.BrowserPath = ""
		s.storymakerBrowsers = render.NewBrowsersWithProbe(t.TempDir(),
			func() (string, bool) { return "", false },
			func(context.Context, string) (string, error) { return "", fmt.Errorf("no browser") })
		page, origin := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		var status struct {
			Browser struct {
				Ready       bool `json:"ready"`
				CanDownload bool `json:"can_download"`
			} `json:"browser"`
		}
		smappGet(t, origin, "/status", &status)
		if status.Browser.Ready || !status.Browser.CanDownload {
			t.Fatalf("status.browser = %+v, want not ready but downloadable", status.Browser)
		}
		if page.MustEval(`()=>!!document.querySelector('[data-banner="export_unavailable"]')`).Bool() {
			t.Fatal("the export-unavailable banner shows although the first export downloads a browser")
		}
		page.MustElement(`[data-stm="new-title"]`).MustInput("Download")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export"]') && document.querySelector('[data-stm="clock-total"]').textContent!=='0:00.0'`)
		smappClick(t, page, `[data-stm="export"]`)
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-start"]') && !document.querySelector('[data-stm="export-start"]').disabled`)
		if page.MustEval(`()=>!!document.querySelector('[data-stm="export-blockers"]')`).Bool() {
			t.Fatalf("a downloadable browser must not block the export: %s", smappText(page, `[data-stm="export-panel"]`))
		}
		if caps := smappText(page, `[data-stm="export-caps"]`); !strings.Contains(caps, "downloaded once") {
			t.Fatalf("the capability list should say that the browser is downloaded: %q", caps)
		}
		smappNoErrors(t, page)
	})

	t.Run("no ffmpeg", func(t *testing.T) {
		s := storymakerRealServer(t)
		s.Cfg.VideoStudio.FFmpegPath = filepath.Join(t.TempDir(), "no-such-ffmpeg")
		page, _ := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('[data-banner="export_unavailable"]')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Offline")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export"]')`)
		smappClick(t, page, `[data-stm="export"]`)
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="export-blockers"]')`)
		if text := smappText(page, `[data-stm="export-blockers"]`); !strings.Contains(text, "FFmpeg is missing") || strings.Contains(text, "No browser") {
			t.Fatalf("blockers = %q", text)
		}
		if !page.MustEval(`()=>document.querySelector('[data-stm="export-start"]').disabled`).Bool() {
			t.Fatal("the start button must be disabled")
		}
		if page.MustEval(`()=>document.querySelector('[data-stm="narration"]').disabled`).Bool() {
			t.Fatal("editing must stay possible without FFmpeg")
		}
		smappNoErrors(t, page)
	})

	t.Run("read-only", func(t *testing.T) {
		s := storymakerRealServer(t)
		page, origin := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Before")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		ro := *s.ConfigSnapshot()
		ro.VirtualDesktop.ReadOnly = true
		s.replaceConfigSnapshot(&ro)
		// A write that races the policy event is refused by the server with desktop_readonly.
		res, err := http.Post(origin+storymakerAPIBase+"/projects", "application/json", strings.NewReader(`{"title":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("a create while read-only = %d", res.StatusCode)
		}
		page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{readonly:true}}))`)
		smappWait(t, page, `()=>!!document.querySelector('[data-banner="readonly"]') && document.querySelector('[data-stm="narration"]').disabled && document.querySelector('[data-stm="add-scene"]').disabled`)
		smappClick(t, page, `[data-stm="export"]`)
		smappWait(t, page, `()=>document.querySelector('[data-stm="export-blockers"] [data-reason="readonly"]')!==null && document.querySelector('[data-stm="export-start"]').disabled`)
		smappNoErrors(t, page)
	})

	t.Run("switched off", func(t *testing.T) {
		s := storymakerRealServer(t)
		off := *s.ConfigSnapshot()
		off.Storymaker.Enabled = false
		s.replaceConfigSnapshot(&off)
		page, _ := smappPage(t, s)
		smappWait(t, page, `()=>document.querySelector('[data-stm="state"]')?.dataset.state==='storymaker.state.disabled_title'`)
		if !strings.Contains(smappText(page, `[data-stm="state"]`), "Storymaker is switched off") {
			t.Fatalf("state card = %q", smappText(page, `[data-stm="state"]`))
		}
		smappNoErrors(t, page)
	})

	t.Run("somebody else saved the story", func(t *testing.T) {
		s := storymakerRealServer(t)
		page, origin := smappPage(t, s)
		smappWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Conflict")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		smappWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		id := smappProjectIDs(t, origin)[0]
		var p smappProject
		smappGet(t, origin, "/projects/"+id, &p)
		p.Story["title"] = "Changed elsewhere"
		data, _ := json.Marshal(p.Story)
		req, _ := http.NewRequest(http.MethodPut, origin+storymakerAPIBase+"/projects/"+id+"/story", strings.NewReader(string(data)))
		req.Header.Set("If-Match", p.ETag)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("the other client's save: %v %v", err, res)
		}
		res.Body.Close()
		page.MustElement(`[data-stm="narration"]`).MustInput("My text")
		smappWait(t, page, `()=>!!document.querySelector('[data-banner="conflict"]')`)
		smappClick(t, page, `[data-stm="conflict-overwrite"]`)
		smappWait(t, page, `()=>!document.querySelector('[data-banner="conflict"]') && document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
		scene := smappStored(t, origin, id)["scenes"].([]any)[0].(map[string]any)
		if narration, _ := scene["narration"].(string); narration != "My text" {
			t.Fatalf("stored narration after overwrite = %q", narration)
		}
		smappNoErrors(t, page)
	})

	t.Run("story.json cannot be read", func(t *testing.T) {
		s := storymakerRealServer(t)
		var id string
		page, origin := smappPageWith(t, s, func(origin string) {
			// A project whose story.json was damaged by a hand edit in Files.
			id, _ = smappCreate(t, origin, "Broken")
			file := filepath.Join(s.Cfg.VirtualDesktop.WorkspaceDir, "Documents", "Storymaker", id, "story.json")
			if err := os.WriteFile(file, []byte(`{"schema": "storymaker/1", "scenes": [`), 0o600); err != nil {
				t.Fatal(err)
			}
		})
		smappWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===1`)
		var got struct {
			Story     any    `json:"story"`
			StoryText string `json:"story_text"`
		}
		smappGet(t, origin, "/projects/"+id, &got)
		if got.Story != nil || !strings.Contains(got.StoryText, `"scenes": [`) {
			t.Fatalf("the server should answer story null with the text: %+v", got)
		}
		smappClick(t, page, `[data-stm="open-project"]`)
		smappWait(t, page, `()=>(document.querySelector('.stm-projects')?.textContent||'').includes('cannot be read')`)
		if page.MustEval(`()=>!!document.querySelector('[data-stm="narration"]')`).Bool() {
			t.Fatal("a project without a readable story must not open the editor")
		}
		smappNoErrors(t, page)
	})
}

// TestStorymakerEnvironmentChangeRealServerBrowser: the editor saves a new scene at once, with the server's fade for a scene in
// the same environment, so the default cannot react to a later environment change. The editor turns that fade into a wipe. A fade
// picked afterwards stays and the motion lint flags it in the issues panel. Gated by AURAGO_RUN_BROWSER_SMOKE=1.
func TestStorymakerEnvironmentChangeRealServerBrowser(t *testing.T) {
	s := storymakerRealServer(t)
	page, origin := smappPage(t, s)
	smappWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
	page.MustElement(`[data-stm="new-title"]`).MustInput("Worlds")
	page.MustElement(`[data-stm="create-project"]`).MustClick()
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===1 && document.querySelector('[data-stm="clock-total"]').textContent!=='0:00.0'`)
	id := smappProjectIDs(t, origin)[0]
	const saved = `document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`
	const setSelect = `(sel,value)=>{const s=document.querySelector(sel);s.value=value;s.dispatchEvent(new Event('change',{bubbles:true}));}`

	smappClick(t, page, `[data-stm="add-scene"]`)
	smappWait(t, page, `()=>document.querySelectorAll('[data-stm="scene"]').length===2 && document.querySelector('[data-stm="transition"]')?.value==='fade' && `+saved)

	page.MustEval(setSelect, `[data-stm="environment"]`, "env.home-office")
	smappWait(t, page, `()=>document.querySelector('[data-stm="transition"]')?.value==='wipe' && `+saved)
	second := smappStored(t, origin, id)["scenes"].([]any)[1].(map[string]any)
	if second["environment"] != "env.home-office" || second["transition"] != "wipe" {
		t.Fatalf("stored scene after the environment change = %v", second)
	}

	page.MustEval(setSelect, `[data-stm="transition"]`, "fade")
	smappWait(t, page, `()=>document.querySelector('[data-stm="transition"]')?.value==='fade' && `+saved)
	if got := smappStored(t, origin, id)["scenes"].([]any)[1].(map[string]any)["transition"]; got != "fade" {
		t.Fatalf("a fade picked by the user must stay, stored = %v", got)
	}
	smappClick(t, page, `[data-stm="issues-toggle"]`)
	smappWait(t, page, `()=>!!document.querySelector('.stm-issue[data-code="fade_between_worlds"]')`)
	smappNoErrors(t, page)
}
