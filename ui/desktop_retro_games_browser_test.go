package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/server"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/ysmood/gson"
)

type retroGamesFixture struct {
	mu        sync.Mutex
	favorites map[string]bool
	broken    map[string]bool
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
	// Only reachable by identifier (the lists above name their games); its fake loader misbehaves.
	retroGamesJunkGame: {"identifier": retroGamesJunkGame, "title": "Junk Mail", "platform": "arcade", "system": "Arcade", "year": 1985},
}

func newRetroGamesFixture() *retroGamesFixture {
	return &retroGamesFixture{favorites: map[string]bool{}, broken: map[string]bool{}, plays: map[string]int{}}
}

func (f *retroGamesFixture) isBroken(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.broken[id]
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
		broken := []any{}
		for id, on := range f.broken {
			if on {
				broken = append(broken, entry(id))
			}
		}
		write(http.StatusOK, map[string]any{"favorites": favorites, "recent": recent, "broken": broken})
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
	case strings.HasPrefix(path, "broken/"):
		id := strings.TrimPrefix(path, "broken/")
		f.broken[id] = r.Method == http.MethodPut
		write(http.StatusOK, map[string]any{"broken": f.broken[id]})
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

const (
	retroGamesBrowserFSURL = "https://archive.org/services/emularity/engine/browserfs.min.js"
	retroGamesLoaderURL    = "https://archive.org/services/emularity/config/loader.js"
	// retroGamesBrokenGame is the identifier whose fake loader fails the way
	// archive.org's does: it writes "Failed ..." into the Emularity splash title.
	retroGamesBrokenGame = "msdos_Wolfenstein_3D_1992"
	// retroGamesJunkGame is the identifier whose fake loader abuses the message
	// channel to the app: archive.org's code shares the player page's realm and
	// can post anything to its parent.
	retroGamesJunkGame = "arcade_junk_mail"
)

// retroGamesFakeLoader stands in for archive.org's loader.js. Like the real
// IALoader it draws into the canvas it was given, plays through Web Audio (an
// oscillator wired to the destination, so the page's master gain exists) and
// calls before_run once the "emulator" runs. Keys that reach the page are
// collected in window.__rgKeys.
const retroGamesFakeLoader = `
globalThis.IALoader = function (canvas, id, cb) {
  return {
    start() {
      const audio = new AudioContext();
      audio.createOscillator().connect(audio.destination);
      canvas.width = 320;
      canvas.height = 200;
      const ctx = canvas.getContext('2d');
      ['#e22', '#2c2', '#26e', '#ee2', '#2ee', '#e2e', '#eee', '#888'].forEach((color, i) => {
        ctx.fillStyle = color;
        ctx.fillRect(i * 40, 0, 40, 200);
      });
      window.__rgKeys = [];
      addEventListener('keydown', (event) => window.__rgKeys.push(event.key));
      if (id === '` + retroGamesJunkGame + `') {
        // Every mute message the page receives from the app is kept.
        window.__rgMuteMessages = [];
        addEventListener('message', (event) => {
          if (event.data && event.data.type === 'mute') window.__rgMuteMessages.push(event.data.muted);
        });
        const send = (message) => parent.postMessage(message, '*');
        // Ill-formed or unknown messages first, then a flood of "loading".
        [
          { rg: 1, type: 'state', state: '<img src=x onerror=alert(1)>' }, { rg: 1, type: 'state' }, { rg: 1, type: 'state', state: ['running'] },
          { rg: 1, type: 'state', state: {} }, { rg: 1, type: 'x' }, { rg: 1, type: '<b>', state: 'failed' }, { rg: 1 },
          { rg: 1, type: 'audio', blocked: 'yes' }, { rg: 1, type: 'audio' }, { rg: '1', type: 'state', state: 'failed' }
        ].forEach(send);
        for (let i = 0; i < 500; i++) send({ rg: 1, type: 'state', state: 'loading' });
        // The one real verdict, then everything that must not resurrect the game.
        send({ rg: 1, type: 'state', state: 'failed' });
        for (let i = 0; i < 500; i++) {
          send({ rg: 1, type: 'state', state: 'running' });
          send({ rg: 1, type: 'state', state: 'loading' });
        }
        send({ rg: 1, type: 'audio', blocked: true });
        return;
      }
      if (id === '` + retroGamesBrokenGame + `') {
        const title = document.createElement('div');
        title.className = 'emularity-splash-title';
        title.textContent = 'Failed to download game data!';
        document.body.appendChild(title);
        return;
      }
      setTimeout(() => cb.before_run && cb.before_run(), 300);
    }
  };
};
`

// retroGamesArchive answers everything the browser asks archive.org for: the
// two player scripts and the thumbnails. The loader can be held back so a test
// controls how long the game "loads".
type retroGamesArchive struct {
	mu   sync.Mutex
	gate chan struct{}
	down bool
	seen []string
}

// setDown makes loader.js answer 404, like archive.org being unreachable or blocked.
func (a *retroGamesArchive) setDown(down bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.down = down
}

func (a *retroGamesArchive) hold() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gate == nil {
		a.gate = make(chan struct{})
	}
}

func (a *retroGamesArchive) release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gate != nil {
		close(a.gate)
		a.gate = nil
	}
}

func (a *retroGamesArchive) requested(url string) bool {
	return a.requestCount(url) > 0
}

func (a *retroGamesArchive) requestCount(url string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, seen := range a.seen {
		if seen == url {
			n++
		}
	}
	return n
}

func (a *retroGamesArchive) serve(ctx *rod.Hijack) {
	url := ctx.Request.URL().String()
	a.mu.Lock()
	a.seen = append(a.seen, url)
	gate, down := a.gate, a.down
	a.mu.Unlock()
	switch url {
	case retroGamesBrowserFSURL:
		ctx.Response.SetHeader("Content-Type", "text/javascript")
		ctx.Response.SetBody("/* BrowserFS stub */")
	case retroGamesLoaderURL:
		if gate != nil {
			select {
			case <-gate:
			case <-time.After(30 * time.Second):
			}
		}
		if down {
			ctx.Response.Payload().ResponseCode = http.StatusNotFound
			ctx.Response.SetBody("not found")
			return
		}
		ctx.Response.SetHeader("Content-Type", "text/javascript")
		ctx.Response.SetBody(retroGamesFakeLoader)
	default:
		ctx.Response.SetHeader("Content-Type", "image/gif")
		ctx.Response.SetBody("")
	}
}

// newRetroGamesPlayerPage opens the real desktop shell with the fake Retro
// Games API and the real player page (server.ServeRetroGamesPlayerUngated, headers and
// all). archive.org is answered by the returned fake, so no request leaves the
// machine. The player runs in a frame with an opaque origin; Chrome puts such a
// frame into its own process by default, and requests of another process never
// reach the page-level router, so site isolation is switched off for these tests.
func newRetroGamesPlayerPage(t *testing.T, fixture *retroGamesFixture) (*rod.Page, *retroGamesArchive) {
	t.Helper()
	page := desktopAuditBrowserWith(t, map[string]http.Handler{
		"/api/desktop/retro-games/":       fixture,
		"/api/desktop/retro-games/player": http.HandlerFunc(server.ServeRetroGamesPlayerUngated),
	}, func(l *launcher.Launcher) {
		l.Set("disable-site-isolation-trials").Append("disable-features", "IsolateOrigins", "IsolateSandboxedIframes")
	})
	archive := &retroGamesArchive{}
	router := page.HijackRequests()
	router.MustAdd("*archive.org*", archive.serve)
	go router.Run()
	// Cleanups run last in, first out: the held loader is answered before the router goes away.
	t.Cleanup(func() { router.MustStop() })
	t.Cleanup(archive.release)
	return page, archive
}

// retroGamesFrame returns the player frame's page (its own JS context).
func retroGamesFrame(page *rod.Page) *rod.Page {
	return page.MustElement(".rg-frame").MustFrame()
}

// retroGamesShortStall makes the next mounted players give up waiting after
// ms milliseconds; only the tests shorten the stall delay (production: 60 s).
func retroGamesShortStall(page *rod.Page, ms int) {
	page.MustEval(`(ms)=>{const real=RetroGamesPlayer.mount;RetroGamesPlayer.mount=(stage,game,options)=>real(stage,game,Object.assign({},options,{stallMs:ms}))}`, ms)
}

// TestDesktopRetroGamesBrowser drives the Retro Games app through the real
// Desktop shell with a fake API. The player page is the real one; archive.org
// is answered by a fake, so no request leaves the machine.
func TestDesktopRetroGamesBrowser(t *testing.T) {
	fixture := newRetroGamesFixture()
	page, archive := newRetroGamesPlayerPage(t, fixture)
	wasStubbed := archive.requested

	page.MustEval(`()=>{document.documentElement.lang='de';auditTest.openApp('retro-games')}`)
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

	// Play: exact frame contract, recorded start, the loading bar retires when the game runs.
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	attrs := page.MustEval(`()=>{const f=document.querySelector('.rg-frame');return [f.getAttribute('sandbox'),f.getAttribute('allow'),f.getAttribute('referrerpolicy')].join('|')}`).Str()
	if attrs != "allow-scripts allow-pointer-lock|fullscreen; gamepad; autoplay|no-referrer" {
		t.Fatalf("frame attributes = %q", attrs)
	}
	assertRetroGamesPlayerSource(t, page.MustEval(`()=>document.querySelector('.rg-frame').getAttribute('src')`).Str(), "msdos_Prince_of_Persia_1990", false, german["desktop.retro_games_click_for_sound"])
	// Space/Enter/arrows must reach the game: the frame, not Back, holds the focus.
	if !page.MustEval(`()=>document.activeElement===document.querySelector('.rg-frame')`).Bool() {
		t.Fatal("the emulator frame does not hold the keyboard focus after the game opens")
	}
	retroGamesWaitFor(t, "play POST", func() bool { return fixture.playCount("msdos_Prince_of_Persia_1990") == 1 })
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	// The frame runs the real player page with archive.org's loader replaced by the fake.
	if !archive.requested(retroGamesBrowserFSURL) || !archive.requested(retroGamesLoaderURL) {
		t.Fatal("the player page did not load archive.org's scripts through the fake")
	}
	if got := retroGamesFrame(page).MustEval(`()=>document.querySelector('meta[name=rg-id]').content`).Str(); got != "msdos_Prince_of_Persia_1990" {
		t.Fatalf("the player page runs %q", got)
	}
	if !page.MustEval(`()=>document.querySelector('.rg-player-fav').getAttribute('aria-pressed')==='true'`).Bool() {
		t.Fatal("player does not show the favorite state")
	}

	// Back unloads the emulator.
	page.MustEval(`()=>document.querySelector('.rg-back').click()`)
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-browse').hidden`)

	// A launch with a game id reaches the open window (agent tool path).
	page.MustEval(`()=>auditTest.openApp('retro-games',{game:'arcade_tron'})`)
	page.MustWait(`()=>(document.querySelector('.rg-frame')?.getAttribute('src')||'').startsWith('/api/desktop/retro-games/player?id=arcade_tron&')`)
	if n := page.MustEval(`()=>document.querySelectorAll('.vd-window[data-app-id="retro-games"]').length`).Int(); n != 1 {
		t.Fatalf("windows = %d, want 1", n)
	}
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)

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

// assertRetroGamesPlayerSource checks a frame src: AuraGo's own player route
// with the identifier, the muted flag and the sound label, and nothing else.
func assertRetroGamesPlayerSource(t *testing.T, src, id string, muted bool, sound string) {
	t.Helper()
	if !strings.HasPrefix(src, "/api/desktop/retro-games/player?id=") || !strings.Contains(src, "muted=") || !strings.Contains(src, "sound=") {
		t.Fatalf("frame src = %q, want /api/desktop/retro-games/player?id=...&muted=...&sound=...", src)
	}
	u, err := url.Parse(src)
	if err != nil {
		t.Fatalf("frame src %q: %v", src, err)
	}
	wantMuted := "0"
	if muted {
		wantMuted = "1"
	}
	query := u.Query()
	if u.Host != "" || u.Path != "/api/desktop/retro-games/player" || len(query) != 3 || query.Get("id") != id || query.Get("muted") != wantMuted || query.Get("sound") != sound {
		t.Fatalf("frame src = %q, want id=%s muted=%s sound=%q", src, id, wantMuted, sound)
	}
}

// retroGamesWaitForFrame waits until js, evaluated inside the player frame,
// returns true. The frame may be in the middle of a navigation meanwhile.
func retroGamesWaitForFrame(t *testing.T, page *rod.Page, what, js string) {
	t.Helper()
	retroGamesWaitFor(t, what, func() bool {
		ok := false
		if err := rod.Try(func() { ok = retroGamesFrame(page).MustEval(js).Bool() }); err != nil {
			return false
		}
		return ok
	})
}

// retroGamesWaitForMuted waits until the player page's sound is muted (or not).
// The page mutes at the Web Audio level: whatever the game connects to the
// destination goes through one master GainNode per AudioContext (rgPlayer is a
// global of the page), which is 0 while muted and 1 otherwise. The fake loader
// wires an oscillator to its context, so a master gain exists once it started.
func retroGamesWaitForMuted(t *testing.T, page *rod.Page, what string, muted bool) {
	t.Helper()
	gain := 1
	if muted {
		gain = 0
	}
	retroGamesWaitForFrame(t, page, what, fmt.Sprintf(`()=>{const gains=[...rgPlayer.masters.values()].map(master=>master.gain.value);return gains.length>0&&gains.every(value=>value===%d)}`, gain))
}

// retroGamesHint describes the bottom strip (loading bar or hint) and whether
// the game underneath it stays reachable.
func retroGamesHint(page *rod.Page) map[string]gson.JSON {
	return page.MustEval(`()=>{
		const o=document.querySelector('.rg-overlay'),s=document.querySelector('.rg-screen').getBoundingClientRect(),f=document.querySelector('.rg-frame').getBoundingClientRect(),h=o.getBoundingClientRect();
		const centre=document.elementFromPoint(f.left+f.width/2,f.top+f.height/2);
		const buttons=[...o.querySelectorAll('.rg-overlay-actions .rg-btn')];
		const reachable=buttons.map(b=>{const r=b.getBoundingClientRect();return document.elementFromPoint(r.left+r.width/2,r.top+r.height/2)===b}).every(Boolean);
		return {
			hidden:o.hidden,stalled:o.classList.contains('is-stalled'),text:o.querySelector('.rg-overlay-text').textContent,pointer:getComputedStyle(o).pointerEvents,
			bottomGap:Math.abs(s.bottom-h.bottom),heightShare:h.height/s.height,
			centreIsFrame:centre===document.querySelector('.rg-frame'),buttons:buttons.length,buttonsReachable:reachable,
			actionsHidden:o.querySelector('.rg-overlay-actions').hidden,
			focusIsFrame:document.activeElement===document.querySelector('.rg-frame')
		}
	}`).Map()
}

// assertRetroGamesHintIsCompact fails unless the strip is shown with text, is a
// compact bottom strip that lets every click through to the game, offers the
// three buttons and leaves the keyboard with the game.
func assertRetroGamesHintIsCompact(t *testing.T, page *rod.Page, text string) {
	t.Helper()
	hint := retroGamesHint(page)
	if hint["hidden"].Bool() || !hint["stalled"].Bool() || hint["actionsHidden"].Bool() || hint["text"].Str() != text {
		t.Fatalf("hint hidden=%v stalled=%v actionsHidden=%v text=%q, want the hint with %q", hint["hidden"].Bool(), hint["stalled"].Bool(), hint["actionsHidden"].Bool(), hint["text"].Str(), text)
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
}

func retroGamesReportsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "reports", "retro-games")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDesktopRetroGamesPlayerBrowser plays a game through the real shell, the
// real player page and a fake archive.org loader: the loading bar, the frame
// contract, the sound toggle (also while the page still loads), focus, the
// message protocol and unloading.
func TestDesktopRetroGamesPlayerBrowser(t *testing.T) {
	fixture := newRetroGamesFixture()
	page, archive := newRetroGamesPlayerPage(t, fixture)
	german := readMergedLangMap(t, "de")
	sound := german["desktop.retro_games_click_for_sound"]
	soundOff, soundOn := german["desktop.retro_games_sound_off"], german["desktop.retro_games_sound_on"]
	if sound != "Klick für Ton" || soundOff != "Ton aus" || soundOn != "Ton an" {
		t.Fatalf("de texts = %q %q %q", sound, soundOff, soundOn)
	}
	loadingText, stalledText := german["desktop.retro_games_player_loading"], german["desktop.retro_games_player_stalled"]

	// Every message listener the app registers is counted, so "one per player,
	// gone after Back" can be asserted.
	page.MustEval(`()=>{
		window.__rgMessageListeners=new Set();
		const add=window.addEventListener,remove=window.removeEventListener;
		window.addEventListener=function(type,fn,opt){if(type==='message')__rgMessageListeners.add(fn);return add.call(this,type,fn,opt)};
		window.removeEventListener=function(type,fn,opt){if(type==='message')__rgMessageListeners.delete(fn);return remove.call(this,type,fn,opt)};
		document.documentElement.lang='de';auditTest.openApp('retro-games')
	}`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)
	retroGamesShortStall(page, 900)
	baseline := page.MustEval(`()=>__rgMessageListeners.size`).Int()
	listeners := func() int { return page.MustEval(`()=>__rgMessageListeners.size`).Int() - baseline }
	muteState := func() string {
		return page.MustEval(`()=>{const b=document.querySelector('.rg-mute');return [b.getAttribute('aria-pressed'),b.getAttribute('aria-label'),b.title,b.textContent,localStorage.getItem('aurago.retroGames.muted')].join('|')}`).Str()
	}
	overlayHidden := func() bool { return page.MustEval(`()=>document.querySelector('.rg-overlay').hidden`).Bool() }
	frameIsFocused := func() bool {
		return page.MustEval(`()=>document.activeElement===document.querySelector('.rg-frame')`).Bool()
	}
	waitFocus := func(what string) {
		t.Helper()
		retroGamesWaitFor(t, "the frame to hold the focus "+what, frameIsFocused)
	}

	// Loading: the loader is held back, so the page cannot start yet.
	archive.hold()
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	if attrs := page.MustEval(`()=>{const f=document.querySelector('.rg-frame');return [f.getAttribute('sandbox'),f.getAttribute('allow'),f.getAttribute('referrerpolicy')].join('|')}`).Str(); attrs != "allow-scripts allow-pointer-lock|fullscreen; gamepad; autoplay|no-referrer" {
		t.Fatalf("frame attributes = %q", attrs)
	}
	assertRetroGamesPlayerSource(t, page.MustEval(`()=>document.querySelector('.rg-frame').getAttribute('src')`).Str(), "msdos_Prince_of_Persia_1990", false, sound)
	waitFocus("after mount")
	if n := listeners(); n != 1 {
		t.Fatalf("message listeners of one player = %d, want 1", n)
	}
	if state := page.MustEval(`()=>{const o=document.querySelector('.rg-overlay');return [o.hidden,o.classList.contains('is-stalled'),getComputedStyle(o).pointerEvents,o.querySelector('.rg-overlay-text').textContent,o.querySelector('.rg-overlay-actions').hidden].join('|')}`).Str(); state != "false|false|none|"+loadingText+"|true" {
		t.Fatalf("loading bar = %q, want a visible, click-through bar saying %q", state, loadingText)
	}
	if got := muteState(); got != "false|"+soundOff+"|"+soundOff+"|🔊|" {
		t.Fatalf("mute button before any click = %q", got)
	}
	// The app's own window posting {rg:1} is not the player frame: nothing it
	// says counts, however well-formed.
	page.MustEval(`()=>{window.postMessage({rg:1,type:'state',state:'running'},'*')}`)
	time.Sleep(200 * time.Millisecond)
	if overlayHidden() {
		t.Fatal("a state message from the app's own window retired the loading bar")
	}

	// Sound toggled while the page still loads (it cannot hear us yet): real click.
	page.MustElement(".rg-mute").MustClick()
	if got := muteState(); got != "true|"+soundOn+"|"+soundOn+"|🔇|1" {
		t.Fatalf("mute button after muting = %q", got)
	}
	waitFocus("after the mute button")
	// 900 ms without a running game: the hint.
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)
	if text := page.MustEval(`()=>document.querySelector('.rg-overlay-text').textContent`).Str(); text != stalledText {
		t.Fatalf("stalled text = %q, want %q", text, stalledText)
	}
	if !frameIsFocused() {
		t.Fatal("the hint took the keyboard focus from the game")
	}

	// The loader arrives: the page starts, says "running", the hint goes, and the
	// page learns the mute click it missed while loading.
	archive.release()
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	retroGamesWaitForMuted(t, page, "the page to get the mute click from before it started", true)
	if !frameIsFocused() {
		t.Fatal("the frame lost the keyboard focus while the game started")
	}
	time.Sleep(1200 * time.Millisecond)
	if !overlayHidden() {
		t.Fatal("the stall hint came back after the game was running")
	}

	// The page itself: identifier, label and the picture fitting its window.
	info := retroGamesFrame(page).MustEval(`()=>{
		const canvas=document.getElementById('canvas'),r=canvas.getBoundingClientRect();
		let storage=true;
		try{localStorage.length}catch(e){storage=false}
		return {
			id:document.querySelector('meta[name=rg-id]').content,muted:document.querySelector('meta[name=rg-muted]').content,
			pill:document.getElementById('rg-sound').textContent,pillHidden:document.getElementById('rg-sound').hidden,
			inside:r.left>=-1&&r.top>=-1&&r.right<=innerWidth+1&&r.bottom<=innerHeight+1,
			touches:Math.abs(r.width-innerWidth)<=1||Math.abs(r.height-innerHeight)<=1,
			storage:storage
		}
	}`).Map()
	if info["id"].Str() != "msdos_Prince_of_Persia_1990" || info["muted"].Str() != "0" || info["pill"].Str() != sound || !info["pillHidden"].Bool() {
		t.Fatalf("player page = %v", info)
	}
	// Never cut off, and as large as the window allows (letterboxed on one axis).
	if !info["inside"].Bool() || !info["touches"].Bool() {
		t.Fatalf("the picture does not fit the frame: inside=%v, uses the full width or height=%v", info["inside"].Bool(), info["touches"].Bool())
	}
	if info["storage"].Bool() {
		t.Fatal("the player page can reach storage: it does not run in an opaque origin")
	}
	page.MustScreenshot(filepath.Join(retroGamesReportsDir(t), "player-running-1280.png"))

	// Keys reach the game.
	page.Keyboard.MustType(input.ArrowRight)
	retroGamesWaitForFrame(t, page, "the arrow key to reach the game", `()=>window.__rgKeys.includes('ArrowRight')`)

	// Messages: only the frame's own, marked {rg:1}, count; the rest is ignored.
	cabinet := func() int { return page.MustEval(`()=>document.querySelectorAll('.rg-cabinet *').length`).Int() }
	before := cabinet()
	page.MustEval(`()=>{window.postMessage({rg:1,type:'state',state:'failed'},'*');window.postMessage(null,'*');window.postMessage('failed','*')}`)
	retroGamesFrame(page).MustEval(`()=>{
		parent.postMessage({type:'state',state:'failed'},'*');parent.postMessage({rg:2,type:'state',state:'failed'},'*');
		parent.postMessage(null,'*');parent.postMessage('failed','*');parent.postMessage({rg:1},'*');parent.postMessage({rg:1,type:'state',state:'nonsense'},'*');
		parent.postMessage({rg:1,type:'audio',blocked:true},'*');parent.postMessage({rg:1,type:'audio',blocked:false},'*');
		parent.postMessage({rg:1,type:'state',state:'loading'},'*');parent.postMessage({rg:1,type:'state',state:'running'},'*')
	}`)
	time.Sleep(300 * time.Millisecond)
	if !overlayHidden() || cabinet() != before {
		t.Fatalf("a message outside the protocol changed the player (overlay hidden=%v, cabinet nodes %d -> %d)", overlayHidden(), before, cabinet())
	}
	// ...while the frame's own {rg:1} state messages do.
	retroGamesFrame(page).MustEval(`()=>parent.postMessage({rg:1,type:'state',state:'failed'},'*')`)
	page.MustWait(`()=>{const o=document.querySelector('.rg-overlay');return !o.hidden&&o.classList.contains('is-stalled')}`)
	if text := page.MustEval(`()=>document.querySelector('.rg-overlay-text').textContent`).Str(); text != german["desktop.retro_games_player_failed"] {
		t.Fatalf("failed text = %q", text)
	}
	// A failure is final until a restart: later states change nothing.
	retroGamesFrame(page).MustEval(`()=>{for(const state of ['running','loading','running'])parent.postMessage({rg:1,type:'state',state},'*');parent.postMessage({rg:1,type:'state',state:'ping'},'*')}`)
	time.Sleep(300 * time.Millisecond)
	if text := page.MustEval(`()=>{const o=document.querySelector('.rg-overlay');return o.hidden?'':o.querySelector('.rg-overlay-text').textContent}`).Str(); text != german["desktop.retro_games_player_failed"] {
		t.Fatalf("a state after failed changed the hint: %q", text)
	}

	// Restart (real click): focus stays with the game, the remembered sound state
	// is in the new URL and reaches the new page's loader at once.
	page.MustElement(".rg-restart").MustClick()
	waitFocus("after restart")
	page.MustWait(`()=>(document.querySelector('.rg-frame').getAttribute('src')||'').includes('muted=1')`)
	assertRetroGamesPlayerSource(t, page.MustEval(`()=>document.querySelector('.rg-frame').getAttribute('src')`).Str(), "msdos_Prince_of_Persia_1990", true, sound)
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	retroGamesWaitForMuted(t, page, "the restarted page to start muted", true)
	retroGamesWaitForFrame(t, page, "the restarted page to be started muted", `()=>document.querySelector('meta[name=rg-muted]').content==='1'`)
	if n := listeners(); n != 1 {
		t.Fatalf("message listeners after a restart = %d, want still 1", n)
	}

	// Sound back on: pressed state, label, storage, the game's master gain and the focus all follow.
	page.MustElement(".rg-mute").MustClick()
	if got := muteState(); got != "false|"+soundOff+"|"+soundOff+"|🔊|0" {
		t.Fatalf("mute button after unmuting = %q", got)
	}
	retroGamesWaitForMuted(t, page, "the game to be unmuted", false)
	waitFocus("after unmuting")
	// Every one of our other buttons hands the keyboard back, too.
	for _, selector := range []string{".rg-player-fav", ".rg-full"} {
		page.MustElement(selector).MustClick()
		waitFocus("after " + selector)
	}
	// Fullscreen puts the bezel on top of everything: leave it before the next real click.
	if page.MustEval(`()=>!!document.fullscreenElement`).Bool() {
		page.MustEval(`()=>document.exitFullscreen()`)
		page.MustWait(`()=>!document.fullscreenElement`)
		waitFocus("after leaving fullscreen")
	}
	retroGamesWaitFor(t, "favorite PUT from the player", func() bool { return fixture.isFavorite("msdos_Prince_of_Persia_1990") })

	// The choice is remembered for the next game.
	page.MustElement(".rg-mute").MustClick()
	retroGamesWaitForMuted(t, page, "the game to be muted again", true)
	page.MustElement(".rg-back").MustClick()
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-browse').hidden`)
	if n := listeners(); n != 0 {
		t.Fatalf("message listeners after Back = %d, want 0", n)
	}
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	assertRetroGamesPlayerSource(t, page.MustEval(`()=>document.querySelector('.rg-frame').getAttribute('src')`).Str(), "msdos_Prince_of_Persia_1990", true, sound)
	if got := muteState(); got != "true|"+soundOn+"|"+soundOn+"|🔇|1" {
		t.Fatalf("mute button of the next player = %q", got)
	}
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	page.MustElement(".rg-back").MustClick()
	page.MustWait(`()=>!document.querySelector('.rg-frame')`)

	// Blocked storage never stops a game: the sound defaults to on and can still be toggled.
	page.MustEval(`()=>{const s=Storage.prototype;window.__realGet=s.getItem;window.__realSet=s.setItem;s.getItem=()=>{throw new Error('blocked')};s.setItem=()=>{throw new Error('blocked')}}`)
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	assertRetroGamesPlayerSource(t, page.MustEval(`()=>document.querySelector('.rg-frame').getAttribute('src')`).Str(), "msdos_Prince_of_Persia_1990", false, sound)
	page.MustElement(".rg-mute").MustClick()
	if pressed := page.MustEval(`()=>document.querySelector('.rg-mute').getAttribute('aria-pressed')`).Str(); pressed != "true" {
		t.Fatalf("the mute button does not toggle without storage: aria-pressed=%s", pressed)
	}
	page.MustEval(`()=>{const s=Storage.prototype;s.getItem=window.__realGet;s.setItem=window.__realSet}`)
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	retroGamesWaitForMuted(t, page, "the game to be muted without storage", true)

	// Closing the window unloads everything.
	page.MustEval(`()=>auditTest.closeWindow(auditTest.state.activeWindowId)`)
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-app')`)
	if n := listeners(); n != 0 {
		t.Fatalf("message listeners after closing the window = %d, want 0", n)
	}
	if errs := page.MustEval(`()=>window.fixtureErrors`).Arr(); len(errs) > 0 {
		t.Fatalf("script errors in the shell: %v", errs)
	}
}

// TestDesktopRetroGamesPlayerBrowserFailure lets the fake loader fail the way
// archive.org's does (a "Failed..." splash title): the page reports it, the app
// shows the hint strip with the failure text and its three buttons, and the
// game underneath stays reachable.
func TestDesktopRetroGamesPlayerBrowserFailure(t *testing.T) {
	fixture := newRetroGamesFixture()
	page, archive := newRetroGamesPlayerPage(t, fixture)
	german := readMergedLangMap(t, "de")
	failedText := german["desktop.retro_games_player_failed"]
	if failedText != "Das Spiel konnte nicht gestartet werden." {
		t.Fatalf("de failed text = %q", failedText)
	}

	page.MustEval(`()=>{document.documentElement.lang='de';window.__opened=[];window.open=(...args)=>{__opened.push(args);return null};auditTest.openApp('retro-games')}`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)
	page.MustEval(`(id)=>auditTest.openApp('retro-games',{game:id})`, retroGamesBrokenGame)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)
	assertRetroGamesHintIsCompact(t, page, failedText)
	page.MustScreenshot(filepath.Join(retroGamesReportsDir(t), "failed-hint-1280.png"))

	// The hint's buttons: open on archive.org (a new tab, the game page), mark as not starting.
	page.MustElement(".rg-overlay-external").MustClick()
	page.MustElement(".rg-external").MustClick()
	opened := page.MustEval(`()=>__opened.map(a=>a.join('|'))`).Arr()
	want := "https://archive.org/details/" + retroGamesBrokenGame + "|_blank|noopener,noreferrer"
	if len(opened) != 2 || opened[0].Str() != want || opened[1].Str() != want {
		t.Fatalf("open on archive.org opened %v, want twice %q", opened, want)
	}
	page.MustElement(".rg-overlay-broken").MustClick()
	retroGamesWaitFor(t, "broken marker PUT", func() bool { return fixture.isBroken(retroGamesBrokenGame) })
	page.MustWait(`()=>document.querySelector('.rg-player-broken').getAttribute('aria-pressed')==='true'`)
	retroGamesWaitFor(t, "the frame to hold the focus after the hint button", func() bool {
		return page.MustEval(`()=>document.activeElement===document.querySelector('.rg-frame')`).Bool()
	})

	// Restart from the hint: back to the loading bar (held there, so the test can
	// look at it), then the same failure again once the loader answers.
	archive.hold()
	page.MustElement(".rg-overlay-restart").MustClick()
	page.MustWait(`()=>{const o=document.querySelector('.rg-overlay');return !o.hidden&&!o.classList.contains('is-stalled')&&o.querySelector('.rg-overlay-actions').hidden}`)
	archive.release()
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)
	assertRetroGamesHintIsCompact(t, page, failedText)

	// Phone width: the strip grows with its wrapped buttons but still leaves the game clickable.
	page.MustSetViewport(420, 900, 1, false)
	page.MustEval(`()=>{const a=auditTest;a.applyWindowSnap(a.state.windows.get(a.state.activeWindowId).element,'maximize')}`)
	narrow := retroGamesHint(page)
	page.MustScreenshot(filepath.Join(retroGamesReportsDir(t), "failed-hint-420.png"))
	if !narrow["stalled"].Bool() || !narrow["centreIsFrame"].Bool() || narrow["heightShare"].Num() > 0.6 {
		t.Fatalf("failed hint at phone width: shown=%v, game centre clickable=%v, share of the screen height=%.2f", narrow["stalled"].Bool(), narrow["centreIsFrame"].Bool(), narrow["heightShare"].Num())
	}
	if overflow := page.MustEval(`()=>{const m=document.querySelector('.rg-marquee');return m.scrollWidth>m.clientWidth+1}`).Bool(); overflow {
		t.Fatal("the cabinet marquee overflows at phone width")
	}
	page.MustSetViewport(1280, 900, 1, false)

	// Back still unloads the player.
	page.MustEval(`()=>document.querySelector('.rg-back').click()`)
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-browse').hidden`)

	// archive.org unreachable: the loader script itself fails to load. The page
	// reports it at once (no wait for the stall delay), and once archive.org is
	// back, restarting from the hint plays the game.
	archive.setDown(true)
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)
	assertRetroGamesHintIsCompact(t, page, failedText)
	archive.setDown(false)
	page.MustElement(".rg-overlay-restart").MustClick()
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	retroGamesWaitForMuted(t, page, "the recovered game to have sound", false)
}

// TestDesktopRetroGamesPlayerBrowserHostileMessages lets the game's own code
// (the fake loader runs in the player page's realm, like archive.org's) post
// junk and out-of-order states to the app: the UI shows exactly the one honest
// verdict, renders nothing from the messages and does not repeat the mute
// handshake for every "loading".
func TestDesktopRetroGamesPlayerBrowserHostileMessages(t *testing.T) {
	fixture := newRetroGamesFixture()
	page, archive := newRetroGamesPlayerPage(t, fixture)
	german := readMergedLangMap(t, "de")
	failedText := german["desktop.retro_games_player_failed"]

	page.MustEval(`()=>{document.documentElement.lang='de';auditTest.openApp('retro-games')}`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)
	// The stall timer must not outlive the verdict: it would turn the failure text into the stall text.
	retroGamesShortStall(page, 1500)
	// The mute click comes before the page starts, so the app owes it one handshake.
	archive.hold()
	page.MustEval(`(id)=>auditTest.openApp('retro-games',{game:id})`, retroGamesJunkGame)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)
	page.MustElement(".rg-mute").MustClick()
	archive.release()

	page.MustWait(`()=>{const o=document.querySelector('.rg-overlay');return !o.hidden&&o.classList.contains('is-stalled')}`)
	assertRetroGamesHintIsCompact(t, page, failedText)
	retroGamesWaitForFrame(t, page, "the handshake after loading", `()=>JSON.stringify(window.__rgMuteMessages)==='[true]'`)

	// Longer than the stall delay and long enough for late messages: nothing changes.
	time.Sleep(1900 * time.Millisecond)
	assertRetroGamesHintIsCompact(t, page, failedText)
	if got := retroGamesFrame(page).MustEval(`()=>JSON.stringify(window.__rgMuteMessages)`).Str(); got != "[true]" {
		t.Fatalf("mute messages the page received = %s, want exactly one handshake [true]", got)
	}
	rendered := page.MustEval(`()=>({
		nodes:document.querySelectorAll('.rg-cabinet img,.rg-cabinet b').length,
		markup:document.querySelector('.rg-cabinet').innerHTML.includes('onerror')||document.querySelector('.rg-cabinet').innerHTML.includes('&lt;img'),
		errors:window.fixtureErrors.length
	})`).Map()
	if rendered["nodes"].Int() != 0 || rendered["markup"].Bool() || rendered["errors"].Int() != 0 {
		t.Fatalf("message content reached the UI or broke it: %v", rendered)
	}

	// A restart is a fresh start: the new page's own verdict counts again, without a handshake (already muted).
	page.MustElement(".rg-overlay-restart").MustClick()
	page.MustWait(`()=>(document.querySelector('.rg-frame').getAttribute('src')||'').includes('muted=1')`)
	page.MustWait(`(text)=>{const o=document.querySelector('.rg-overlay');return !o.hidden&&o.classList.contains('is-stalled')&&o.querySelector('.rg-overlay-text').textContent===text}`, failedText)
	retroGamesWaitForFrame(t, page, "the restarted page to need no handshake", `()=>JSON.stringify(window.__rgMuteMessages)==='[]'`)
}

// TestDesktopRetroGamesBrowserStalledHint keeps the loader from ever arriving
// (archive.org can be slow or blocked) and checks that the player stays
// playable meanwhile: the hint is a bottom strip that lets clicks through to
// the game, the frame holds the keyboard focus, and the game starting retires
// the hint for good.
func TestDesktopRetroGamesBrowserStalledHint(t *testing.T) {
	fixture := newRetroGamesFixture()
	page, archive := newRetroGamesPlayerPage(t, fixture)
	archive.hold()
	german := readMergedLangMap(t, "de")
	stalledText := german["desktop.retro_games_player_stalled"]
	if stalledText != "Das Spiel lädt ungewöhnlich lange." {
		t.Fatalf("de stalled text = %q", stalledText)
	}

	page.MustEval(`()=>{document.documentElement.lang='de';auditTest.openApp('retro-games')}`)
	page.MustWait(`()=>document.querySelectorAll('.rg-card').length===2`)
	// Only the tests shorten the stall delay; production keeps one minute.
	retroGamesShortStall(page, 900)
	page.MustEval(`()=>document.querySelector('.rg-card-play').click()`)
	page.MustWait(`()=>!!document.querySelector('.rg-frame')`)

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
	assertRetroGamesHintIsCompact(t, page, stalledText)
	dir := retroGamesReportsDir(t)
	page.MustScreenshot(filepath.Join(dir, "stalled-hint-1280.png"))
	// Phone width: the strip grows with its wrapped buttons but still leaves the game clickable.
	page.MustSetViewport(420, 900, 1, false)
	page.MustEval(`()=>{const a=auditTest;a.applyWindowSnap(a.state.windows.get(a.state.activeWindowId).element,'maximize')}`)
	narrow := retroGamesHint(page)
	page.MustScreenshot(filepath.Join(dir, "stalled-hint-420.png"))
	if !narrow["stalled"].Bool() || !narrow["centreIsFrame"].Bool() || narrow["heightShare"].Num() > 0.6 {
		t.Fatalf("hint at phone width: stalled=%v, game centre clickable=%v, share of the screen height=%.2f", narrow["stalled"].Bool(), narrow["centreIsFrame"].Bool(), narrow["heightShare"].Num())
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

	// Focusing the frame (a click into the game) says nothing about its state:
	// the loading bar stays until the page reports the game running.
	page.MustEval(`()=>{document.querySelector('.rg-restart').focus();document.querySelector('.rg-frame').focus()}`)
	time.Sleep(200 * time.Millisecond)
	if page.MustEval(`()=>document.querySelector('.rg-overlay').hidden`).Bool() {
		t.Fatal("moving the focus into the frame retired the loading bar")
	}

	// The loader arrives at last: the game runs, the strip goes and stays gone.
	page.MustWait(`()=>document.querySelector('.rg-overlay').classList.contains('is-stalled')`)
	archive.release()
	page.MustWait(`()=>document.querySelector('.rg-overlay').hidden===true`)
	time.Sleep(1300 * time.Millisecond)
	if !page.MustEval(`()=>{const o=document.querySelector('.rg-overlay');return o.hidden&&!o.classList.contains('is-stalled')}`).Bool() {
		t.Fatal("the hint came back after the game started")
	}

	// Back still unloads the emulator.
	page.MustEval(`()=>document.querySelector('.rg-back').click()`)
	page.MustWait(`()=>!document.querySelector('.rg-frame')&&!document.querySelector('.rg-browse').hidden`)
}
