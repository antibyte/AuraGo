package ui

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// Three defects the review of the app found, each as a browser state against the fixture backend:
//
//   - leaving the project after a failed save must ask first (the typed text exists nowhere else),
//   - two quick project opens must end in the project that was opened last,
//   - a code scene can be added again after one was deleted (the server keeps the file of a deleted code scene).
//
// Gated by AURAGO_RUN_BROWSER_SMOKE=1:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestStorymakerAppReviewFixesBrowser -count=1
func TestStorymakerAppReviewFixesBrowser(t *testing.T) {
	t.Run("leaving after a failed save asks first", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Unsaved")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		// The desktop turned read-only behind the window's back: the save of the typed text is refused.
		fixture.set(func() { fixture.readOnly = true })
		page.MustElement(`[data-stm="narration"]`).MustInput("Typed but never saved")
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='error'`)

		storymakerClick(t, page, `[data-stm="back"]`)
		// Either the question or (the defect) the project list.
		storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal') || !!document.querySelector('.stm-projects')`)
		if !page.MustHas(`.vd-modal`) {
			t.Fatal("the editor was left without asking although the last change was never saved")
		}
		if title := storymakerText(page, `.vd-modal-title`); !strings.Contains(title, "unsaved") {
			t.Fatalf("dialog title = %q", title)
		}
		// Cancel keeps the editor and its text.
		page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
		storymakerWait(t, page, `()=>!document.querySelector('.vd-modal')`)
		if got := page.MustEval(`()=>document.querySelector('[data-stm="narration"]')?.value||''`).String(); got != "Typed but never saved" {
			t.Fatalf("the editor lost the text after Cancel: %q", got)
		}
		// Leaving anyway is possible.
		storymakerClick(t, page, `[data-stm="back"]`)
		storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal')`)
		page.MustEval(`()=>document.querySelector('.vd-modal button[type="submit"]').click()`)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		storymakerNoErrors(t, page)
	})

	t.Run("two quick opens end in the last one", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		alpha, _, _, err := fixture.projects.Create(t.Context(), "Alpha", "16:9", 30)
		if err != nil {
			t.Fatal(err)
		}
		beta, _, _, err := fixture.projects.Create(t.Context(), "Beta", "16:9", 30)
		if err != nil {
			t.Fatal(err)
		}
		// The answer for Alpha is held back until Beta's editor is open.
		release := make(chan struct{})
		var once sync.Once
		free := func() { once.Do(func() { close(release) }) }
		t.Cleanup(free)
		slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/"+alpha.ID) {
				<-release
			}
			fixture.ServeHTTP(w, r)
		})
		page := storymakerBrowserPage(t, slow)
		storymakerWait(t, page, `()=>document.querySelectorAll('[data-stm="open-project"]').length===2`)
		page.MustEval(`(id)=>document.querySelector('[data-stm="open-project"][data-id="'+id+'"]').click()`, alpha.ID)
		page.MustEval(`(id)=>document.querySelector('[data-stm="open-project"][data-id="'+id+'"]').click()`, beta.ID)
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="project-title"]')?.textContent==='Beta'`)
		free()
		time.Sleep(900 * time.Millisecond) // long enough for the late answer to be handled
		if got := storymakerText(page, `[data-stm="project-title"]`); got != "Beta" {
			t.Fatalf("the late answer for Alpha replaced the editor: the window shows %q", got)
		}
		// Typing goes to Beta.
		page.MustElement(`[data-stm="narration"]`).MustInput("For Beta")
		storymakerWait(t, page, `()=>document.querySelector('[data-stm="save-state"]')?.dataset.state==='saved'`)
		stored, _ := fixture.storedStory(beta.ID)
		if narration, _ := stored["scenes"].([]any)[0].(map[string]any)["narration"].(string); narration != "For Beta" {
			t.Fatalf("Beta narration = %q", narration)
		}
		storymakerNoErrors(t, page)
	})

	t.Run("a code scene can be added again after one was deleted", func(t *testing.T) {
		fixture := newStorymakerFixture(t)
		page := storymakerBrowserPage(t, fixture)
		storymakerWait(t, page, `()=>!!document.querySelector('.stm-projects')`)
		page.MustElement(`[data-stm="new-title"]`).MustInput("Code again")
		page.MustElement(`[data-stm="create-project"]`).MustClick()
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="narration"]')`)
		id := fixture.projectIDs()[0]
		scenes := func() []any {
			stored, _ := fixture.storedStory(id)
			list, _ := stored["scenes"].([]any)
			return list
		}
		waitScenes := func(n int) {
			t.Helper()
			deadline := time.Now().Add(10 * time.Second)
			for len(scenes()) != n && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
			if got := len(scenes()); got != n {
				t.Fatalf("the story has %d scenes, want %d (server saw: %s)", got, n, fixture.joined())
			}
		}

		storymakerClick(t, page, `[data-stm="add-code-scene"]`)
		waitScenes(2)
		storymakerWait(t, page, `()=>!!document.querySelector('[data-stm="code-editor"] .cm-editor')`)
		// Delete it again: the server keeps scenes/<id>.js.
		storymakerClick(t, page, `[data-stm="scene-delete"]`)
		storymakerWait(t, page, `()=>!!document.querySelector('.vd-modal')`)
		page.MustEval(`()=>document.querySelector('.vd-modal button[type="submit"]').click()`)
		waitScenes(1)

		// The same count of scenes again, so the first free id is the one whose file is still there.
		storymakerClick(t, page, `[data-stm="add-code-scene"]`)
		waitScenes(2)
		second, _ := scenes()[1].(map[string]any)
		if second["kind"] != "code" {
			t.Fatalf("the new scene is %v", second)
		}
		storymakerNoErrors(t, page)
	})
}
