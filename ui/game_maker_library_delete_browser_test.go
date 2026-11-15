package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestGameMakerLibraryDeleteBrowser proves that a game can be deleted from the
// studio library without opening it first, and that a missing delete grant
// states the real reason instead of silently disabling the control.
func TestGameMakerLibraryDeleteBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	t.Run("delete from the library", func(t *testing.T) {
		server, deleted := newGameMakerDeleteFixture(t, true)
		defer server.Close()

		page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(45 * time.Second).MustWaitLoad()
		defer page.Close()
		page.MustSetViewport(1920, 1080, 1, false)

		page.MustWait(`() => document.querySelectorAll('.gm-project-item').length === 2`)
		buttons := page.MustElements(".gm-project-delete")
		if len(buttons) != 2 {
			t.Fatalf("library delete buttons = %d, want 2", len(buttons))
		}
		if !page.MustEval(`() => Array.from(document.querySelectorAll('.gm-project-delete')).every(button => !button.disabled)`).Bool() {
			t.Fatal("library delete button is disabled although the grant is present")
		}

		// The second game is deleted while the first one stays open.
		buttons[1].MustClick()
		page.MustWait(`() => document.querySelectorAll('.gm-project-item').length === 1`)

		recorded := deleted()
		if len(recorded) != 1 || recorded[0] != "space" {
			t.Fatalf("deleted projects = %v, want [space]", recorded)
		}
		title := page.MustEval(`() => (document.querySelector('[data-gm-title]')?.textContent || '') === 'Waldabenteuer'`).Bool()
		if !title {
			t.Fatal("deleting a game from the library switched the open project")
		}
		notified := page.MustEval(`() => (window.testState.notices || []).some(item => String(item.message || '').includes('Weltraum'))`).Bool()
		if !notified {
			t.Fatal("a deleted game is not reported to the user")
		}
	})

	t.Run("missing grant explains itself", func(t *testing.T) {
		server, _ := newGameMakerDeleteFixture(t, false)
		defer server.Close()

		page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(45 * time.Second).MustWaitLoad()
		defer page.Close()
		page.MustSetViewport(1920, 1080, 1, false)

		page.MustWait(`() => document.querySelectorAll('.gm-project-delete').length === 2`)
		explained := page.MustEval(`() => {
			const hint = 'Projektlöschung erlauben';
			const buttons = Array.from(document.querySelectorAll('.gm-project-delete'));
			const menu = document.querySelector('[data-gm-action="delete"]');
			return buttons.every(button => button.disabled && String(button.title || '').includes(hint))
				&& Boolean(menu && menu.disabled && String(menu.title || '').includes(hint));
		}`).Bool()
		if !explained {
			t.Fatal("delete controls do not state the missing delete grant")
		}
	})
}

// newGameMakerDeleteFixture serves the studio against a stubbed Game Maker API
// and reports every deleted project id.
func newGameMakerDeleteFixture(t *testing.T, allowDelete bool) (*httptest.Server, func() []string) {
	t.Helper()

	var mu sync.Mutex
	deleted := []string{}
	projects := []map[string]any{
		{"id": "forest", "name": "Waldabenteuer", "description": "Ein Plattformspiel im Wald.", "dimension": "2d", "status": "draft", "current_revision": 0},
		{"id": "space", "name": "Weltraum", "description": "Ein Weltraumspiel.", "dimension": "3d", "status": "draft", "current_revision": 0},
	}
	events := make(chan struct{})

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/game-maker/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"enabled":        true,
			"allow_create":   true,
			"allow_edit":     true,
			"allow_delete":   allowDelete,
			"skills_ready":   true,
			"phaser_version": "4.2.1",
			"three_version":  "0.186.1",
		})
	})
	mux.HandleFunc("/api/game-maker/projects", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"projects": projects})
	})
	mux.HandleFunc("/api/game-maker/projects/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/game-maker/projects/")
		parts := strings.Split(rest, "/")
		id := parts[0]
		switch {
		case r.Method == http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, id)
			kept := projects[:0]
			for _, project := range projects {
				if project["id"] != id {
					kept = append(kept, project)
				}
			}
			projects = kept
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case len(parts) > 1 && parts[1] == "workspace-drift":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"changed": []string{}, "added": []string{}, "removed": []string{},
				"changed_count": 0, "added_count": 0, "removed_count": 0,
			})
		case len(parts) > 1 && parts[1] == "events":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, ": connected\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-events:
			case <-r.Context().Done():
			}
		default:
			mu.Lock()
			defer mu.Unlock()
			for _, project := range projects {
				if project["id"] == id {
					_ = json.NewEncoder(w).Encode(map[string]any{"project": project, "messages": []any{}})
					return
				}
			}
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, gameMakerDeleteFixtureHTML)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(func() { close(events) })
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), deleted...)
	}
}

const gameMakerDeleteFixtureHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-game-maker-studio.css">
<style>html,body,#app{width:100%;height:100%;margin:0}*{box-sizing:border-box}</style></head>
<body class="desktop-body" data-theme="standard"><div id="app"></div>
<script src="/js/desktop/apps/game-maker-studio-modals.js"></script>
<script src="/js/desktop/apps/game-maker-studio-api.js"></script>
<script src="/js/desktop/apps/game-maker-studio-activity.js"></script>
<script src="/js/desktop/apps/game-maker-studio-preview.js"></script>
<script src="/js/desktop/apps/game-maker-studio.js"></script>
<script>
window.testState={notices:[]};
fetch('/lang/desktop/de.json').then(r=>r.json()).then(lang=>GameMakerStudioApp.render(document.getElementById('app'),'fixture',{
 esc:value=>String(value??'').replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';'),
 t:(key,args)=>{let value=lang[key]||key;for(const [k,v] of Object.entries(args||{}))value=value.replaceAll('{{'+k+'}}',String(v));return value},
 confirmDialog:async()=>true,
 notify:payload=>{window.testState.notices.push(payload)},
 api:async(path,options)=>{const response=await fetch(path,options);if(response.status===204)return{};const body=await response.json().catch(()=>({}));if(!response.ok){const error=new Error(body.error||('HTTP '+response.status));error.status=response.status;throw error}return body}
}));
</script></body></html>`
