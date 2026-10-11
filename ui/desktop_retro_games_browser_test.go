package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

type retroGamesFixture struct {
	mu        sync.Mutex
	favorites map[string]bool
	plays     map[string]int
}

var retroGamesFixtureGames = map[string]map[string]any{
	"msdos_Prince_of_Persia_1990": {"identifier": "msdos_Prince_of_Persia_1990", "title": "Prince of Persia", "platform": "msdos", "system": "MS-DOS", "year": 1990},
	"arcade_tron":                 {"identifier": "arcade_tron", "title": "Tron", "platform": "arcade", "system": "Arcade", "year": 1982},
	"msdos_Wolfenstein_3D_1992":   {"identifier": "msdos_Wolfenstein_3D_1992", "title": "Wolfenstein 3D", "platform": "msdos", "system": "MS-DOS", "year": 1992},
}

func newRetroGamesFixture() *retroGamesFixture {
	return &retroGamesFixture{favorites: map[string]bool{}, plays: map[string]int{}}
}

func (f *retroGamesFixture) isFavorite(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.favorites[id]
}

func (f *retroGamesFixture) playCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.plays[id]
}

func (f *retroGamesFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/api/desktop/retro-games/")
	write := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	entry := func(id string) map[string]any {
		out := map[string]any{"available": true, "at": 1791700000}
		for k, v := range retroGamesFixtureGames[id] {
			out[k] = v
		}
		return out
	}
	switch {
	case path == "status":
		platforms := []map[string]any{}
		for _, key := range []string{"msdos", "arcade", "console", "c64", "amiga", "apple2", "zx", "win3"} {
			count := 0
			if key == "msdos" {
				count = 2
			} else if key == "arcade" {
				count = 1
			}
			platforms = append(platforms, map[string]any{"key": key, "label": key, "count": count, "state": "done", "last_success_at": 1791700000})
		}
		write(http.StatusOK, map[string]any{"enabled": true, "syncing": false, "total_games": 3, "platforms": platforms, "systems": []any{}})
	case path == "hall-of-fame":
		write(http.StatusOK, map[string]any{"games": []any{retroGamesFixtureGames["msdos_Prince_of_Persia_1990"], retroGamesFixtureGames["arcade_tron"]}})
	case path == "search":
		games := []any{}
		for _, id := range []string{"msdos_Prince_of_Persia_1990", "msdos_Wolfenstein_3D_1992", "arcade_tron"} {
			g := retroGamesFixtureGames[id]
			if q := strings.ToLower(r.URL.Query().Get("q")); q == "" || strings.Contains(strings.ToLower(g["title"].(string)), q) {
				games = append(games, g)
			}
		}
		write(http.StatusOK, map[string]any{"games": games, "total": len(games), "offset": 0, "limit": 60})
	case path == "library":
		favorites, recent := []any{}, []any{}
		for id, on := range f.favorites {
			if on {
				favorites = append(favorites, entry(id))
			}
		}
		for id, n := range f.plays {
			e := entry(id)
			e["play_count"] = n
			recent = append(recent, e)
		}
		write(http.StatusOK, map[string]any{"favorites": favorites, "recent": recent, "broken": []any{}})
	case strings.HasPrefix(path, "favorites/"):
		id := strings.TrimPrefix(path, "favorites/")
		f.favorites[id] = r.Method == http.MethodPut
		write(http.StatusOK, map[string]any{"favorite": f.favorites[id]})
	case strings.HasPrefix(path, "plays/"):
		f.plays[strings.TrimPrefix(path, "plays/")]++
		write(http.StatusOK, map[string]any{"recorded": true})
	case strings.HasPrefix(path, "games/"):
		if g, ok := retroGamesFixtureGames[strings.TrimPrefix(path, "games/")]; ok {
			write(http.StatusOK, map[string]any{"game": g})
			return
		}
		write(http.StatusNotFound, map[string]any{"error": "Game not found", "code": "game_not_found"})
	case path == "sync":
		write(http.StatusAccepted, map[string]any{"started": true})
	default:
		write(http.StatusNotFound, map[string]any{"error": "Not found", "code": "not_found"})
	}
}

func retroGamesWaitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestDesktopRetroGamesBrowser drives the Retro Games app through the real
// Desktop shell with a fake API. archive.org is stubbed at the browser level,
// so no request leaves the machine.
func TestDesktopRetroGamesBrowser(t *testing.T) {
	fixture := newRetroGamesFixture()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/retro-games/": fixture})
	// The page-level router answers the cross-origin embed navigation and the
	// thumbnail images; a browser-level router only sees the frame navigation
	// and lets the <img> requests through.
	var stubbedMu sync.Mutex
	stubbed := []string{}
	router := page.HijackRequests()
	router.MustAdd("*archive.org*", func(ctx *rod.Hijack) {
		stubbedMu.Lock()
		stubbed = append(stubbed, ctx.Request.URL().String())
		stubbedMu.Unlock()
		ctx.Response.SetHeader("Content-Type", "text/html; charset=utf-8")
		ctx.Response.SetBody("<!doctype html><title>embed stub</title><script>parent.postMessage('retro-games-embed-stub','*')</script>")
	})
	go router.Run()
	defer router.MustStop()
	wasStubbed := func(url string) bool {
		stubbedMu.Lock()
		defer stubbedMu.Unlock()
		for _, seen := range stubbed {
			if seen == url {
				return true
			}
		}
		return false
	}

	page.MustEval(`()=>{window.embedStubs=0;addEventListener('message',e=>{if(e.data==='retro-games-embed-stub')embedStubs++});document.documentElement.lang='de';auditTest.openApp('retro-games')}`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)
	if title := page.MustEval(`()=>document.querySelector('.rg-section-title').textContent`).Str(); title != "Hall of Fame" {
		t.Fatalf("start view title = %q", title)
	}
	retroGamesWaitFor(t, "stubbed thumbnail", func() bool {
		return wasStubbed("https://archive.org/services/img/msdos_Prince_of_Persia_1990")
	})

	// Searching switches to "Alle Spiele".
	page.MustEval(`()=>{const input=document.querySelector('.rg-search');input.value='prince';input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}))}`)
	page.MustWait(`()=>document.querySelector('.rg-nav-item[data-view="all"]').getAttribute('aria-current')==='page'&&document.querySelectorAll('.rg-card').length===1`)

	// Favorite: optimistic UI plus API call.
	page.MustEval(`()=>document.querySelector('.rg-card-fav').click()`)
	page.MustWait(`()=>document.querySelector('.rg-card-fav').getAttribute('aria-pressed')==='true'`)
	retroGamesWaitFor(t, "favorite PUT", func() bool { return fixture.isFavorite("msdos_Prince_of_Persia_1990") })

	// Play: exact frame contract, recorded start, overlay hides after load.
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	attrs := page.MustEval(`()=>{const f=document.querySelector('.rg-frame');return [f.getAttribute('sandbox'),f.getAttribute('allow'),f.getAttribute('referrerpolicy'),f.getAttribute('src')].join('|')}`).Str()
	if attrs != "allow-scripts allow-same-origin allow-pointer-lock|fullscreen; gamepad; autoplay|no-referrer|https://archive.org/embed/msdos_Prince_of_Persia_1990" {
		t.Fatalf("frame attributes = %q", attrs)
	}
	retroGamesWaitFor(t, "play POST", func() bool { return fixture.playCount("msdos_Prince_of_Persia_1990") == 1 })
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	// The frame shows the stub, not the real archive.org page.
	page.MustWait(`()=>window.embedStubs===1`)
	if !wasStubbed("https://archive.org/embed/msdos_Prince_of_Persia_1990") {
		t.Fatal("embed navigation was not answered by the stub")
	}
	if !page.MustEval(`()=>document.querySelector('.rg-player-fav').getAttribute('aria-pressed')==='true'`).Bool() {
		t.Fatal("player does not show the favorite state")
	}

	// Back unloads the emulator.
	page.MustEval(`()=>document.querySelector('.rg-back').click()`)
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-browse').hidden`)

	// A launch with a game id reaches the open window (agent tool path).
	page.MustEval(`()=>auditTest.openApp('retro-games',{game:'arcade_tron'})`)
	page.MustWait(`()=>document.querySelector('.rg-frame')?.getAttribute('src')==='https://archive.org/embed/arcade_tron'`)
	if n := page.MustEval(`()=>document.querySelectorAll('.vd-window[data-app-id="retro-games"]').length`).Int(); n != 1 {
		t.Fatalf("windows = %d, want 1", n)
	}
	page.MustWait(`()=>window.embedStubs===2`)

	// Closing the window disposes everything.
	page.MustEval(`()=>auditTest.closeWindow(auditTest.state.activeWindowId)`)
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-app')`)

	// Layout and contrast in every theme and at phone width.
	page.MustEval(`()=>auditTest.openApp('retro-games')`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)
	dir := filepath.Join("..", "reports", "retro-games")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	themes := []struct{ theme, mode string }{{"standard", "dark"}, {"fruity", "light"}, {"fruity", "dark"}}
	for _, width := range []int{1280, 420} {
		page.MustSetViewport(width, 900, 1, false)
		if width == 420 {
			page.MustEval(`()=>{const a=auditTest;a.applyWindowSnap(a.state.windows.get(a.state.activeWindowId).element,'maximize')}`)
		}
		for _, combo := range themes {
			name := fmt.Sprintf("%s-%s-%d", combo.theme, combo.mode, width)
			page.MustEval(`(theme,mode)=>{const a=auditTest;a.state.bootstrap.settings['appearance.theme']=theme;a.state.bootstrap.settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;a.applyDesktopSettings();a.refreshSpacesForViewport();}`, combo.theme, combo.mode)
			page.MustWait(`()=>!!document.querySelector('.rg-app')`)
			if page.MustEval(`()=>{const app=document.querySelector('.rg-app');return app.scrollWidth>app.clientWidth+1}`).Bool() {
				t.Fatalf("horizontal overflow in %s", name)
			}
			if low := page.MustEval(localWikipediaLowContrastJS, []string{".rg-section-title", ".rg-card-title", ".rg-card-meta", ".rg-nav-label", ".rg-search"}).Str(); low != "" {
				t.Fatalf("text contrast below 4.5:1 in %s: %s", name, low)
			}
			page.MustScreenshot(filepath.Join(dir, name+".png"))
		}
	}
}
