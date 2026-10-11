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
	// reject, when set, answers every favorite PUT with that refusal code.
	reject string
	// disabled answers every request like a switched-off Retro Games (503).
	disabled bool
	// statusQueries records the raw query of every status request.
	statusQueries []string
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

func (f *retroGamesFixture) rejectFavorites(code string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reject = code
}

func (f *retroGamesFixture) setDisabled(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disabled = on
}

func (f *retroGamesFixture) statusLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.statusQueries...)
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
	if path == "status" {
		f.statusQueries = append(f.statusQueries, r.URL.RawQuery)
	}
	if f.disabled {
		write(http.StatusServiceUnavailable, map[string]any{"error": "Retro Games is switched off", "code": "disabled"})
		return
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
		if f.reject != "" && r.Method == http.MethodPut {
			status := http.StatusConflict
			if f.reject == "desktop_readonly" {
				status = http.StatusForbidden
			}
			write(status, map[string]any{"error": f.reject, "code": f.reject})
			return
		}
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

	// A refused favorite is explained with its own notice and the heart goes
	// back to the state the server holds.
	german := readMergedLangMap(t, "de")
	for _, refusal := range []struct{ code, key string }{
		{"library_full", "desktop.retro_games_library_full"},
		{"desktop_readonly", "desktop.retro_games_readonly"},
	} {
		if german[refusal.key] == "" {
			t.Fatalf("de lacks %s", refusal.key)
		}
		fixture.rejectFavorites(refusal.code)
		page.MustEval(`()=>document.querySelector('.rg-card-fav').click()`)
		page.MustWait(`(text)=>{const n=document.querySelector('.rg-notice');return !!n&&!n.hidden&&n.textContent===text}`, german[refusal.key])
		page.MustWait(`()=>document.querySelector('.rg-card-fav').getAttribute('aria-pressed')==='false'`)
		if fixture.isFavorite("msdos_Prince_of_Persia_1990") {
			t.Fatalf("%s: the refused favorite was stored", refusal.code)
		}
	}
	fixture.rejectFavorites("")

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
	// Space/Enter/arrows must reach the game: the frame, not Back, holds the focus.
	if !page.MustEval(`()=>document.activeElement===document.querySelector('.rg-frame')`).Bool() {
		t.Fatal("the emulator frame does not hold the keyboard focus after the game opens")
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

// TestDesktopRetroGamesRecoversWhenReenabled switches the fake API off and on
// again: an open window shows the disabled message, keeps polling the status
// without ever starting a sync (passive=1) and brings the games back by itself.
func TestDesktopRetroGamesRecoversWhenReenabled(t *testing.T) {
	fixture := newRetroGamesFixture()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/retro-games/": fixture})
	router := page.HijackRequests()
	router.MustAdd("*archive.org*", func(ctx *rod.Hijack) {
		ctx.Response.SetHeader("Content-Type", "image/gif")
		ctx.Response.SetBody("")
	})
	go router.Run()
	defer router.MustStop()
	german := readMergedLangMap(t, "de")

	// The slow disabled poll (30 s) is shortened so the test does not wait for it.
	page.MustEval(`()=>{const real=window.setTimeout.bind(window);window.setTimeout=(fn,ms,...args)=>real(fn,ms===30000?200:ms,...args);auditTest.openApp('retro-games')}`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)

	// A request answered with 503 "disabled" switches the window off.
	fixture.setDisabled(true)
	marker := len(fixture.statusLog())
	page.MustEval(`()=>{const input=document.querySelector('.rg-search');input.value='tron';input.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}))}`)
	page.MustWait(`(text)=>{const e=document.querySelector('.rg-empty');return !!e&&!e.hidden&&e.textContent===text&&document.querySelectorAll('.rg-card').length===0}`, german["desktop.retro_games_disabled"])

	// While off it keeps asking, passively.
	retroGamesWaitFor(t, "two passive status polls", func() bool { return len(fixture.statusLog()) >= marker+2 })
	whileOff := fixture.statusLog()[marker:]
	fixture.setDisabled(false)
	reenabledAt := marker + len(whileOff)
	for i, query := range whileOff {
		if query != "passive=1" {
			t.Fatalf("status request %d while disabled = %q, want passive=1", i, query)
		}
	}

	// Back on: the same path as the first open fills the list again.
	page.MustWait(`()=>{const e=document.querySelector('.rg-empty');return e.hidden&&document.querySelectorAll('.rg-card').length===1}`)
	retroGamesWaitFor(t, "status refresh after re-enabling", func() bool {
		log := fixture.statusLog()
		return len(log) > reenabledAt && log[len(log)-1] == ""
	})
}

// TestDesktopRetroGamesBrowserStalledHint keeps the embed from ever firing its
// load event (archive.org only fires it after every sub-resource, including a
// multi-megabyte splash GIF) and checks that the player stays playable: the
// hint is a bottom strip that lets clicks through to the game, the frame holds
// the keyboard focus, and interacting with the frame retires the hint.
func TestDesktopRetroGamesBrowserStalledHint(t *testing.T) {
	fixture := newRetroGamesFixture()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/desktop/retro-games/": fixture})
	release := make(chan struct{})
	router := page.HijackRequests()
	router.MustAdd("*archive.org*", func(ctx *rod.Hijack) {
		if strings.HasSuffix(ctx.Request.URL().Path, "/never.gif") {
			<-release
			ctx.Response.SetHeader("Content-Type", "image/gif")
			ctx.Response.SetBody("")
			return
		}
		ctx.Response.SetHeader("Content-Type", "text/html; charset=utf-8")
		ctx.Response.SetBody(`<!doctype html><title>slow embed stub</title><body style="margin:0;background:#123"><img src="/serve/never.gif" width="1" height="1">`)
	})
	go router.Run()
	defer router.MustStop()
	// Registered after MustStop so it runs first: the held sub-resource is
	// answered before the router goes away.
	defer close(release)
	german := readMergedLangMap(t, "de")
	stalledText := german["desktop.retro_games_player_stalled"]
	if stalledText != "Das Spiel lädt ungewöhnlich lange." {
		t.Fatalf("de stalled text = %q", stalledText)
	}

	page.MustEval(`()=>{document.documentElement.lang='de';auditTest.openApp('retro-games')}`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)
	// Only the tests shorten the stall delay; production keeps one minute.
	page.MustEval(`()=>{const real=RetroGamesPlayer.mount;RetroGamesPlayer.mount=(stage,game,options)=>real(stage,game,Object.assign({},options,{stallMs:900}))}`)
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	page.MustEval(`()=>{window.frameLoads=0;document.querySelector('.rg-frame').addEventListener('load',()=>frameLoads++)}`)

	// The keyboard belongs to the game from the first moment, not to Back.
	if !page.MustEval(`()=>document.activeElement===document.querySelector('.rg-frame')`).Bool() {
		t.Fatal("the emulator frame does not hold the keyboard focus after mount")
	}

	// While loading: a bottom bar that lets every click through.
	state := page.MustEval(`()=>{const o=document.querySelector('.rg-overlay');return [o.hidden,o.classList.contains('is-stalled'),getComputedStyle(o).pointerEvents].join('|')}`).Str()
	if state != "false|false|none" {
		t.Fatalf("loading bar state = %q, want a visible, click-through bar", state)
	}

	// After the stall delay: the compact hint, still no cover over the game.
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)
	hint := page.MustEval(`()=>{
		const o=document.querySelector('.rg-overlay'),s=document.querySelector('.rg-screen').getBoundingClientRect(),f=document.querySelector('.rg-frame').getBoundingClientRect(),h=o.getBoundingClientRect();
		const centre=document.elementFromPoint(f.left+f.width/2,f.top+f.height/2);
		const buttons=[...o.querySelectorAll('.rg-overlay-actions .rg-btn')];
		const reachable=buttons.map(b=>{const r=b.getBoundingClientRect();return document.elementFromPoint(r.left+r.width/2,r.top+r.height/2)===b}).every(Boolean);
		return {
			hidden:o.hidden,text:o.querySelector('.rg-overlay-text').textContent,pointer:getComputedStyle(o).pointerEvents,
			bottomGap:Math.abs(s.bottom-h.bottom),heightShare:h.height/s.height,
			centreIsFrame:centre===document.querySelector('.rg-frame'),buttons:buttons.length,buttonsReachable:reachable,
			actionsHidden:o.querySelector('.rg-overlay-actions').hidden,
			focusIsFrame:document.activeElement===document.querySelector('.rg-frame'),loads:window.frameLoads
		}
	}`).Map()
	if hint["hidden"].Bool() || hint["actionsHidden"].Bool() || hint["text"].Str() != stalledText {
		t.Fatalf("hint hidden=%v actionsHidden=%v text=%q, want %q", hint["hidden"].Bool(), hint["actionsHidden"].Bool(), hint["text"].Str(), stalledText)
	}
	if hint["pointer"].Str() != "none" || !hint["centreIsFrame"].Bool() {
		t.Fatalf("the hint covers the game: pointer-events=%s, frame centre hit-tests to the frame = %v", hint["pointer"].Str(), hint["centreIsFrame"].Bool())
	}
	if hint["bottomGap"].Num() > 2 || hint["heightShare"].Num() > 0.4 {
		t.Fatalf("hint is not a compact bottom strip: gap to the bottom = %.1fpx, share of the screen height = %.2f", hint["bottomGap"].Num(), hint["heightShare"].Num())
	}
	if hint["buttons"].Int() != 3 || !hint["buttonsReachable"].Bool() {
		t.Fatalf("hint buttons = %d, reachable = %v", hint["buttons"].Int(), hint["buttonsReachable"].Bool())
	}
	if !hint["focusIsFrame"].Bool() {
		t.Fatal("the hint took the keyboard focus from the game")
	}
	if hint["loads"].Int() != 0 {
		t.Fatalf("the embed fired load %d times: the stall premise is gone", hint["loads"].Int())
	}
	dir := filepath.Join("..", "reports", "retro-games")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	page.MustScreenshot(filepath.Join(dir, "stalled-hint-1280.png"))
	// Phone width: the strip grows with its wrapped buttons but still leaves the game clickable.
	page.MustSetViewport(420, 900, 1, false)
	page.MustEval(`()=>{const a=auditTest;a.applyWindowSnap(a.state.windows.get(a.state.activeWindowId).element,'maximize')}`)
	narrow := page.MustEval(`()=>{
		const o=document.querySelector('.rg-overlay'),s=document.querySelector('.rg-screen').getBoundingClientRect(),f=document.querySelector('.rg-frame').getBoundingClientRect(),h=o.getBoundingClientRect();
		return {share:h.height/s.height,centreIsFrame:document.elementFromPoint(f.left+f.width/2,f.top+f.height/2)===document.querySelector('.rg-frame'),stalled:o.classList.contains('is-stalled')}
	}`).Map()
	page.MustScreenshot(filepath.Join(dir, "stalled-hint-420.png"))
	if !narrow["stalled"].Bool() || !narrow["centreIsFrame"].Bool() || narrow["share"].Num() > 0.6 {
		t.Fatalf("hint at phone width: stalled=%v, game centre clickable=%v, share of the screen height=%.2f", narrow["stalled"].Bool(), narrow["centreIsFrame"].Bool(), narrow["share"].Num())
	}
	page.MustSetViewport(1280, 900, 1, false)

	// Restart (real click): the focus goes back to the game and the bar re-arms.
	page.MustElement(".rg-restart").MustClick()
	page.MustWait(`()=>document.activeElement===document.querySelector('.rg-frame')`)
	page.MustWait(`()=>{const o=document.querySelector('.rg-overlay');return !o.hidden&&!o.classList.contains('is-stalled')}`)
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)

	// The heart gives the focus back, too (the optimistic toggle runs first).
	page.MustElement(".rg-player-fav").MustClick()
	page.MustWait(`()=>document.activeElement===document.querySelector('.rg-frame')`)
	retroGamesWaitFor(t, "favorite PUT from the player", func() bool { return fixture.isFavorite("msdos_Prince_of_Persia_1990") })

	// Restart from the hint itself: the strip falls back to the loading bar and
	// the keyboard stays with the game instead of dying with the clicked button.
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)
	page.MustElement(".rg-overlay-restart").MustClick()
	page.MustWait(`()=>document.activeElement===document.querySelector('.rg-frame')`)
	page.MustWait(`()=>{const o=document.querySelector('.rg-overlay');return !o.hidden&&!o.classList.contains('is-stalled')&&o.querySelector('.rg-overlay-actions').hidden}`)

	// Interaction retires the bar and the hint: a real focus change into the
	// frame (not the player's own focus call) is what the window blur reports.
	// The blur is delivered synchronously inside focus(), so reading the bar in
	// the same task proves the window blur listener did it, not the stall timer.
	steps := page.MustEval(`()=>{
		const o=document.querySelector('.rg-overlay');
		document.querySelector('.rg-restart').focus();
		const afterButton=o.hidden;
		document.querySelector('.rg-frame').focus();
		return [afterButton,o.hidden]
	}`).Arr()
	if steps[0].Bool() {
		t.Fatal("moving the focus to our own button counted as interaction with the game")
	}
	if !steps[1].Bool() {
		t.Fatal("focusing the frame (window blur) did not retire the loading bar")
	}
	time.Sleep(1300 * time.Millisecond)
	if !page.MustEval(`()=>{const o=document.querySelector('.rg-overlay');return o.hidden&&!o.classList.contains('is-stalled')}`).Bool() {
		t.Fatal("the hint came back after the user interacted with the game")
	}

	// Back still unloads the emulator.
	page.MustEval(`()=>document.querySelector('.rg-back').click()`)
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-browse').hidden`)
}
