package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopMinigolfBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	artifactDir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if artifactDir != "" {
		if err := os.MkdirAll(artifactDir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/css/desktop-shell.bundle.css"><link rel="stylesheet" href="/css/desktop-app-minigolf.css">
<style>html,body{margin:0;width:100%;height:100%;overflow:hidden}body{font-family:system-ui,sans-serif}
/* The shell bundle turns .vd-window into a titled grid; the fixture host stands in for a maximized window that fills the viewport. */
#golf-host.vd-window{position:relative!important;display:block;inset:auto!important;transform:none!important;width:100%!important;height:100%!important;min-width:0!important;min-height:0!important;border:0!important;border-radius:0!important;box-shadow:none!important;backdrop-filter:none!important;overflow:hidden!important}</style>
</head><body class="desktop-body" data-theme="standard" data-fruity-mode="light"><main class="vd-window" id="golf-host"></main><script type="module" src="/testdata/minigolf-fixture.js"></script></body></html>`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	benchmark := os.Getenv("AURAGO_MINIGOLF_BENCHMARK") == "1"
	browser := newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Set("mute-audio")
		l.Set("enable-unsafe-swiftshader")
		if benchmark {
			l.Delete("disable-gpu")
		}
	})
	// The page deadline is absolute; individual waits below narrow it further.
	page := browser.MustPage("about:blank").Timeout(15 * time.Minute)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 760, 1, false)
	audioManifestData, err := os.ReadFile("3d/minigolf/audio/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var audioManifest struct {
		SFX   map[string][]string `json:"sfx"`
		Music []struct {
			ID string `json:"id"`
		} `json:"music"`
	}
	if err := json.Unmarshal(audioManifestData, &audioManifest); err != nil {
		t.Fatal(err)
	}
	sfxFiles := 0
	for _, paths := range audioManifest.SFX {
		sfxFiles += len(paths)
	}
	if sfxFiles == 0 || len(audioManifest.Music) != 4 {
		t.Fatalf("unexpected audio manifest: %d effect files, %d music tracks", sfxFiles, len(audioManifest.Music))
	}
	page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
	page.MustWait(`()=>!!window.golfFixture?.inspect && window.golfFixture.inspect().loading===false && window.golfFixture.inspect().panel==='menu'`)
	if !page.MustEval(`()=>window.I18N['minigolf.title']==='AuraGolf' && golfFixture.inspect().audio.unlocked===false`).Bool() {
		t.Fatal("fixture did not load the real locale or audio stayed unlocked before a gesture")
	}
	if page.MustEval(`()=>golfFixture.assetRequests.some(path=>path.includes('/3d/minigolf/audio/'))`).Bool() {
		t.Fatal("audio assets were requested before the first user gesture")
	}
	// The shell bundle collapses .vd-window; the fixture must still hand the game the whole viewport.
	if !page.MustEval(`()=>[document.querySelector('#golf-host'),document.querySelector('.mg-app')].every(e=>{const b=e.getBoundingClientRect();return Math.abs(b.width-innerWidth)<1&&Math.abs(b.height-innerHeight)<1})`).Bool() {
		t.Fatalf("the fixture host or .mg-app does not fill the viewport: %s", page.MustEval(`()=>JSON.stringify([innerWidth,innerHeight,...['#golf-host','.mg-app'].map(s=>{const b=document.querySelector(s).getBoundingClientRect();return [b.width,b.height]})])`).Str())
	}
	layoutOK := func(label string) {
		t.Helper()
		if issues := page.MustEval(`()=>JSON.stringify(golfFixture.layoutIssues())`).Str(); issues != "[]" {
			t.Fatalf("layout issues at %s: %s", label, issues)
		}
	}
	shot := func(name string) {
		if artifactDir != "" {
			page.MustScreenshot(filepath.Join(artifactDir, "minigolf-p4-"+name+".png"))
		}
	}
	settle := func() {
		page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	}
	// A freshly loaded course flies the camera from tee to cup unless the browser asks for
	// reduced motion. A wheel event on the canvas ends that flight, so camera checks never
	// depend on how long it takes.
	skipIntro := func() {
		page.MustEval(`()=>{const c=document.querySelector('.mg-canvas canvas');if(c)c.dispatchEvent(new WheelEvent('wheel',{bubbles:true}));return true}`)
		page.Timeout(2 * time.Second).MustWait(`()=>!golfFixture.inspect().graphics?.intro`)
	}

	// Home: one card per round kind, in a fixed order.
	page.MustWait(`()=>golfFixture.inspect().menuStep==='home'`)
	if rounds := page.MustEval(`()=>[...document.querySelectorAll('[data-round]')].map(e=>e.dataset.round).join(',')`).Str(); rounds != "full,garden,coast,night,practice" {
		t.Fatalf("home cards are %q", rounds)
	}
	layoutOK("home 1280")
	shot("home-1280")
	// The start flow has nothing to putt: the footer controls are not rendered, and the cards fit 1280x760 without scrolling.
	if page.MustEval(`()=>document.querySelector('.mg-controls').getClientRects().length`).Int() != 0 {
		t.Fatal("the putting controls are still rendered in the main menu")
	}
	if !page.MustEval(`()=>{const panel=document.querySelector('.mg-panel');return panel.scrollHeight<=panel.clientHeight+1}`).Bool() {
		t.Fatalf("the home panel still scrolls at 1280x760: %s", page.MustEval(`()=>{const p=document.querySelector('.mg-panel');return p.scrollHeight+' > '+p.clientHeight}`).Str())
	}

	// Player setup: two players, unique colours, long names.
	page.MustElement(`[data-round="full"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().menuStep==='players'`)
	page.MustEval(`()=>{const radio=document.querySelector('[data-player-count][value="2"]');radio.checked=true;radio.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustWait(`()=>document.querySelectorAll('[data-player-index]').length===2`)
	if !page.MustEval(`()=>document.querySelector('[data-player-index="1"] [data-player-color][value="cream"]').disabled===true`).Bool() {
		t.Fatal("player 1's colour is still selectable for player 2")
	}
	page.MustEval(`()=>{const names=document.querySelectorAll('[data-player-name]');names[0].value='Player 1 Has A Very Long AuraGolf Name';names[0].dispatchEvent(new Event('input',{bubbles:true}));names[1].value='Player 2 Has A Very Long AuraGolf Name';names[1].dispatchEvent(new Event('input',{bubbles:true}))}`)
	layoutOK("players 1280")
	shot("players-1280")

	// Help from the setup returns to the setup, by button and by the ? key.
	page.MustElement(`.mg-topbar [data-action="help"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='help'`)
	page.MustElement(`.mg-panel [data-action="close"]`).MustClick()
	page.MustWait(`()=>{const s=golfFixture.inspect();return s.panel==='menu'&&s.menuStep==='players'}`)
	if !page.MustEval(`()=>document.querySelector('[data-player-name]').value.startsWith('Player 1 Has')`).Bool() {
		t.Fatal("closing the help dropped the typed player name")
	}
	page.MustEval(`()=>document.querySelector('[data-action="start"]').dispatchEvent(new KeyboardEvent('keydown',{key:'?',bubbles:true}))`)
	page.MustWait(`()=>golfFixture.inspect().panel==='help'`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>{const s=golfFixture.inspect();return s.panel==='menu'&&s.menuStep==='players'}`)

	page.MustElement(`[data-action="start"]`).MustClick()
	page.MustWait(`()=>{const s=golfFixture.inspect();return !s.loading&&s.panel===''&&s.tutorial==='aim'&&s.course==='garden-01'&&s.graphics?.geometries>0&&s.graphics?.triangles>0}`)
	skipIntro()
	if !page.MustEval(`()=>golfFixture.inspect().audio.unlocked && golfFixture.inspect().round.players.length===2`).Bool() {
		t.Fatal("first gesture did not unlock audio or two-player mode was not applied")
	}
	if page.MustEval(`()=>document.querySelector('.mg-controls').getClientRects().length`).Int() == 0 {
		t.Fatal("the putting controls are not rendered while playing")
	}
	if !page.MustEval(`()=>golfFixture.inspect().round.players[0].name==='Player 1 Has A Very Long'&&golfFixture.inspect().round.players[1].name==='Player 2 Has A Very Long'`).Bool() {
		t.Fatalf("long player names did not survive menu validation: %s", page.MustEval(`()=>JSON.stringify(golfFixture.inspect().round.players.map(p=>p.name))`).Str())
	}
	// All recorded effects decode in the background after the first gesture; wait
	// for them so the underrun measurement below does not overlap that work.
	page.Timeout(30*time.Second).MustWait(`count=>{const s=golfFixture.inspect().audio.samples;return s.loaded===count&&s.failed===0}`, sfxFiles)
	page.Timeout(15 * time.Second).MustWait(`()=>golfFixture.inspect().audio.playing`)
	playbackJSON := page.Timeout(45 * time.Second).MustEval(`async()=>JSON.stringify(await golfFixture.measureAudioPlaybackStats(20000))`).Str()
	var playback struct {
		Supported bool `json:"supported"`
		Before    []struct {
			Index       int     `json:"index"`
			State       string  `json:"state"`
			CurrentTime float64 `json:"currentTime"`
		} `json:"before"`
		After []struct {
			Index       int     `json:"index"`
			State       string  `json:"state"`
			CurrentTime float64 `json:"currentTime"`
		} `json:"after"`
		Deltas []struct {
			Index int `json:"index"`
			Delta struct {
				UnderrunEvents   *float64 `json:"underrunEvents"`
				UnderrunDuration *float64 `json:"underrunDuration"`
				TotalDuration    *float64 `json:"totalDuration"`
			} `json:"delta"`
		} `json:"deltas"`
	}
	if err := json.Unmarshal([]byte(playbackJSON), &playback); err != nil {
		t.Fatalf("parse real-time AudioContext playback stats: %v (%s)", err, playbackJSON)
	}
	t.Logf("uninterrupted live music during the first course with coach marks, before any shots (Chrome playbackStats when available): %s", playbackJSON)
	advancedContext := false
	for _, after := range playback.After {
		if after.State != "running" || after.Index >= len(playback.Before) {
			continue
		}
		if after.CurrentTime-playback.Before[after.Index].CurrentTime >= 1 {
			advancedContext = true
		}
	}
	if !advancedContext {
		t.Fatal("no running AudioContext advanced during the uninterrupted course-intro music measurement")
	}
	if playback.Supported {
		for _, delta := range playback.Deltas {
			if delta.Delta.UnderrunEvents != nil && *delta.Delta.UnderrunEvents > 0 || delta.Delta.UnderrunDuration != nil && *delta.Delta.UnderrunDuration > 0 {
				t.Fatalf("live-game AudioContext reported output underruns: %s", playbackJSON)
			}
		}
	}

	// HUD: hole plaque with the course name, one chip per player, and a map that can be hidden.
	if !page.MustEval(`()=>{const name=document.querySelector('.mg-plaque-name');return name.getClientRects().length>0&&name.textContent===I18N['minigolf.course_1']&&document.querySelectorAll('.mg-chip').length===2&&golfFixture.inspect().hud.map.visible===true&&golfFixture.inspect().hud.map.drawn===true}`).Bool() {
		t.Fatalf("HUD plaque, chips or map are wrong: %s", page.MustEval(`()=>JSON.stringify({plaque:document.querySelector('.mg-plaque-name').textContent,chips:document.querySelectorAll('.mg-chip').length,hud:golfFixture.inspect().hud})`).Str())
	}
	page.MustElement(`[data-action="map"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().hud.map.visible===false&&JSON.parse(localStorage.getItem('aurago.desktop.minigolf.settings.v1')).showMap===false`)
	page.MustElement(`[data-action="map"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().hud.map.visible===true&&JSON.parse(localStorage.getItem('aurago.desktop.minigolf.settings.v1')).showMap===true`)

	type cameraSnapshot struct {
		Position      [3]float64 `json:"position"`
		Target        [3]float64 `json:"target"`
		Ball          [3]float64 `json:"ball"`
		ProjectedBall [3]float64 `json:"projectedBall"`
		Distance      float64    `json:"distance"`
	}
	cameraState := func() cameraSnapshot {
		var state cameraSnapshot
		if err := json.Unmarshal([]byte(page.MustEval(`()=>JSON.stringify(golfFixture.inspect().graphics.camera)`).Str()), &state); err != nil {
			t.Fatalf("parse live camera state: %v", err)
		}
		return state
	}
	centered := func(state cameraSnapshot) bool {
		return math.Abs(state.ProjectedBall[0]) < .08 && math.Abs(state.ProjectedBall[1]) < .08
	}
	vectorLength := func(a, b [3]float64) float64 {
		return math.Sqrt(math.Pow(a[0]-b[0], 2) + math.Pow(a[1]-b[1], 2) + math.Pow(a[2]-b[2], 2))
	}
	orientation := func(state cameraSnapshot) [3]float64 {
		return [3]float64{(state.Position[0] - state.Target[0]) / state.Distance, (state.Position[1] - state.Target[1]) / state.Distance, (state.Position[2] - state.Target[2]) / state.Distance}
	}
	cameraCentered := `()=>{const c=golfFixture.inspect().graphics.camera;return !!c&&c.distance<=8.001&&Math.abs(c.projectedBall[0])<.08&&Math.abs(c.projectedBall[1])<.08}`
	overview := cameraState()
	if overview.Distance <= 8 {
		t.Fatalf("camera test did not start from the overview: %+v", overview)
	}
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.Timeout(5 * time.Second).MustWait(cameraCentered)
	focused := cameraState()
	if !centered(focused) || focused.Distance > 8.001 {
		t.Fatalf("explicit focus did not center the current ball within 8m: overview=%+v focused=%+v", overview, focused)
	}
	cameraPoint := page.MustEval(`()=>{const r=document.querySelector('.mg-canvas canvas').getBoundingClientRect();return{x:r.left+r.width/2,y:r.top+r.height/2}}`)
	cameraX, cameraY := cameraPoint.Get("x").Num(), cameraPoint.Get("y").Num()
	shiftPan := func(x, y, endX, endY float64) {
		if err := page.Keyboard.Press(input.ShiftLeft); err != nil {
			t.Fatal(err)
		}
		page.Mouse.MustMoveTo(x, y).MustDown(proto.InputMouseButtonRight).MustMoveTo(endX, endY).MustUp(proto.InputMouseButtonRight)
		if err := page.Keyboard.Release(input.ShiftLeft); err != nil {
			t.Fatal(err)
		}
	}
	page.Mouse.MustMoveTo(cameraX, cameraY).MustScroll(0, -120)
	page.Timeout(3*time.Second).MustWait(`distance=>golfFixture.inspect().graphics.camera.distance<distance-.1`, focused.Distance)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,120))`)
	zoomed := cameraState()
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.Timeout(5 * time.Second).MustWait(cameraCentered)
	afterZoomFocus := cameraState()
	beforeDirection, afterDirection := orientation(zoomed), orientation(afterZoomFocus)
	if math.Abs(afterZoomFocus.Distance-zoomed.Distance) > .2 || !centered(afterZoomFocus) {
		t.Fatalf("focus did not preserve the closer zoom: before=%+v after=%+v", zoomed, afterZoomFocus)
	}
	for axis := range beforeDirection {
		if math.Abs(beforeDirection[axis]-afterDirection[axis]) > .04 {
			t.Fatalf("focus changed the current camera orientation: before=%+v after=%+v", beforeDirection, afterDirection)
		}
	}
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,160))`)
	repeatedFocus := cameraState()
	if repeatedFocus.Distance > afterZoomFocus.Distance+.1 {
		t.Fatalf("repeated focus zoomed back out: before=%+v after=%+v", afterZoomFocus, repeatedFocus)
	}
	page.Mouse.MustMoveTo(cameraX, cameraY).MustDown(proto.InputMouseButtonRight).MustMoveTo(cameraX+120, cameraY+35).MustUp(proto.InputMouseButtonRight)
	rotated := cameraState()
	if vectorLength(orientation(rotated), orientation(repeatedFocus)) < .02 {
		t.Fatal("right-drag did not rotate the camera for the damping regression")
	}
	shiftPan(cameraX+120, cameraY+35, cameraX+210, cameraY+75)
	panned := cameraState()
	if vectorLength(panned.Target, rotated.Target) < .05 {
		t.Fatal("Shift+right-drag did not pan the camera for the damping regression")
	}
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.Timeout(5 * time.Second).MustWait(cameraCentered)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,450))`)
	settledFocus := cameraState()
	if !centered(settledFocus) || math.Abs(settledFocus.Distance-panned.Distance) > .2 {
		t.Fatalf("pending rotate/pan damping moved the focused camera: before=%+v after=%+v", panned, settledFocus)
	}
	page.MustElement(`[data-action="settings"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
	page.MustEval(`()=>{const motion=document.querySelector('[data-motion]');motion.checked=true;motion.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustElement(`[data-action="close"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''`)
	shiftPan(cameraX, cameraY, cameraX+90, cameraY+40)
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.Timeout(5 * time.Second).MustWait(cameraCentered)
	reducedMotionFocus := cameraState()
	if !centered(reducedMotionFocus) || reducedMotionFocus.Distance > 8.001 {
		t.Fatalf("explicit focus failed with reduced motion enabled: %+v", reducedMotionFocus)
	}
	page.MustElement(`[data-action="settings"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
	page.MustEval(`()=>{const motion=document.querySelector('[data-motion]');motion.checked=false;motion.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustElement(`[data-action="close"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''`)
	if artifactDir != "" {
		page.MustScreenshot(filepath.Join(artifactDir, "minigolf-ball-focus.png"))
	}
	page.MustElement(`[data-action="pause"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='pause'&&golfFixture.inspect().audio.paused&&!golfFixture.inspect().audio.playing`)
	pausedMusicPosition := page.MustEval(`()=>golfFixture.inspect().audio.position`).Num()
	samplesJSON := page.Timeout(90 * time.Second).MustEval(`async()=>JSON.stringify(await golfFixture.decodeAudioSamples())`).Str()
	var samples []struct {
		ID                string  `json:"id"`
		Seconds           float64 `json:"seconds"`
		ExpectedSeconds   float64 `json:"expectedSeconds"`
		RMS               float64 `json:"rms"`
		Peak              float64 `json:"peak"`
		FullScaleFraction float64 `json:"fullScaleFraction"`
		Seam              float64 `json:"seam"`
	}
	if err := json.Unmarshal([]byte(samplesJSON), &samples); err != nil {
		t.Fatalf("parse decoded music tracks: %v (%s)", err, samplesJSON)
	}
	if len(samples) != len(audioManifest.Music) {
		t.Fatalf("decoded %d music tracks, expected all %d", len(samples), len(audioManifest.Music))
	}
	for index, sample := range samples {
		if sample.ID != audioManifest.Music[index].ID {
			t.Fatalf("decoded track %d is %q, manifest lists %q", index, sample.ID, audioManifest.Music[index].ID)
		}
		if math.Abs(sample.Seconds-sample.ExpectedSeconds) > .05 || sample.RMS <= .01 || sample.FullScaleFraction > .001 || sample.Seam >= .1 {
			t.Fatalf("music track is silent, clipped, mis-timed or has a loop seam: %+v", sample)
		}
	}
	t.Logf("decoded music tracks: %s", samplesJSON)
	if got := page.MustEval(`()=>golfFixture.inspect().audio.position`).Num(); got != pausedMusicPosition {
		t.Fatalf("offline decoding advanced the paused live music: before=%f after=%f", pausedMusicPosition, got)
	}
	page.MustElement(`[data-action="pause"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''&&golfFixture.inspect().audio.playing`)
	page.Timeout(5*time.Second).MustWait(`position=>golfFixture.inspect().audio.position>position`, pausedMusicPosition)

	// The first putt of a fresh profile walks the coach marks with a real pointer drag:
	// press the ball, pull a little (power), pull further (release), let go (watch, done).
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.Timeout(5 * time.Second).MustWait(cameraCentered)
	if step := page.MustEval(`()=>golfFixture.inspect().tutorial`).Str(); step != "aim" {
		t.Fatalf("the coach marks left the aim step before the first drag: %q", step)
	}
	bannersBefore := page.MustEval(`()=>golfFixture.inspect().hud.bannerCount`).Int()
	ballPoint := page.MustEval(`()=>{const c=golfFixture.inspect().graphics.camera.projectedBall,r=document.querySelector('.mg-canvas canvas').getBoundingClientRect();return{x:r.left+(c[0]+1)/2*r.width,y:r.top+(1-c[1])/2*r.height}}`)
	ballX, ballY := ballPoint.Get("x").Num(), ballPoint.Get("y").Num()
	page.Mouse.MustMoveTo(ballX, ballY).MustDown(proto.InputMouseButtonLeft).MustMoveTo(ballX, ballY+24)
	page.Timeout(5 * time.Second).MustWait(`()=>golfFixture.inspect().tutorial==='power'`)
	page.Mouse.MustMoveTo(ballX, ballY+64)
	page.Timeout(5 * time.Second).MustWait(`()=>golfFixture.inspect().tutorial==='release'`)
	page.Timeout(5 * time.Second).MustWait(`()=>golfFixture.inspect().hud.meter===true`)
	if !page.MustEval(`()=>{const meter=document.querySelector('.mg-meter'),stage=document.querySelector('.mg-stage').getBoundingClientRect(),b=meter.getBoundingClientRect();return meter.getClientRects().length>0&&b.left>=stage.left-.5&&b.right<=stage.right+.5&&b.top>=stage.top-.5&&b.bottom<=stage.bottom+.5}`).Bool() {
		t.Fatalf("the power meter left the stage: %s", page.MustEval(`()=>JSON.stringify({meter:document.querySelector('.mg-meter').getBoundingClientRect(),stage:document.querySelector('.mg-stage').getBoundingClientRect()})`).Str())
	}
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	page.MustWait(`()=>golfFixture.inspect().busy`)
	page.Timeout(30 * time.Second).MustWait(`()=>{const s=golfFixture.inspect();return !s.busy&&s.round.turnIndex===1&&s.tutorial==='done'}`)
	if !page.MustEval(`count=>{const hud=golfFixture.inspect().hud;return hud.bannerCount>count&&hud.banner.includes('Player 2 Has A Very Long')}`, bannersBefore).Bool() {
		t.Fatalf("no turn banner named the second player: %s", page.MustEval(`()=>JSON.stringify(golfFixture.inspect().hud)`).Str())
	}
	page.MustElement(`[data-action="tutorial-done"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().tutorial===null&&JSON.parse(localStorage.getItem('aurago.desktop.minigolf.settings.v1')).tutorial===2`)
	// The coach hands focus back from the game's next animation frame, not from the click, so wait for it.
	page.Timeout(5 * time.Second).MustWait(`()=>document.activeElement===document.querySelector('.mg-canvas canvas')`)

	// A browser that refuses to store anything shows one toast and the game keeps running.
	page.MustEval(`()=>golfFixture.failStorage(true)`)
	page.MustElement(`[data-action="settings"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
	page.MustEval(`()=>{const magnet=document.querySelector('[data-magnet]');magnet.click();magnet.click()}`)
	page.Timeout(5 * time.Second).MustWait(`()=>golfFixture.inspect().hud.toasts.includes('storage_error')`)
	page.MustEval(`()=>golfFixture.failStorage(false)`)
	page.MustElement(`.mg-panel [data-action="close"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''`)

	// Panels nest: Settings, ? and Escape land on Settings again, Credits and Help stack one level deeper,
	// and a panel opened from the pause panel or the top bar hands the focus back to its opener.
	helpKey := `()=>document.activeElement.dispatchEvent(new KeyboardEvent('keydown',{key:'?',bubbles:true}))`
	page.MustElement(`.mg-topbar [data-action="settings"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
	page.MustEval(helpKey)
	page.MustWait(`()=>golfFixture.inspect().panel==='help'`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
	page.MustElement(`.mg-panel [data-action="credits"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='credits'`)
	page.MustEval(helpKey)
	page.MustWait(`()=>golfFixture.inspect().panel==='help'`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>golfFixture.inspect().panel==='credits'`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>golfFixture.inspect().panel==='settings'&&document.activeElement.dataset.action==='credits'`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>golfFixture.inspect().panel===''`)
	page.MustElement(`.mg-topbar [data-action="pause"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='pause'`)
	page.MustElement(`.mg-panel [data-action="help"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='help'`)
	page.MustElement(`.mg-panel [data-action="close"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='pause'&&document.activeElement.dataset.action==='help'`)
	page.MustElement(`.mg-topbar [data-action="pause"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''`)

	// Putting from the footer button must keep arrow keys, Space and Escape working.
	page.MustEval(`()=>document.querySelector('[data-action="shoot"]').focus()`)
	page.Keyboard.MustType(input.Enter)
	page.MustWait(`()=>golfFixture.inspect().busy`)
	if !page.MustEval(`()=>document.activeElement===document.querySelector('.mg-canvas canvas')`).Bool() {
		t.Fatal("putting from the footer button dropped the keyboard focus")
	}
	page.Timeout(30 * time.Second).MustWait(`()=>!golfFixture.inspect().busy&&golfFixture.inspect().round.turnIndex===0`)

	// Hiding the Desktop window pauses both audio and simulation, then resuming
	// the reopened app continues from its persisted round checkpoint.
	checkpoint := page.MustEval(`()=>JSON.stringify({turn:golfFixture.inspect().round.turnIndex,strokes:golfFixture.inspect().round.players.map(p=>p.strokes)})`).Str()
	page.Keyboard.MustType(input.Space)
	page.MustWait(`()=>golfFixture.inspect().busy`)
	page.MustEval(`()=>{document.querySelector('#golf-host').style.display='none'}`)
	page.Timeout(10 * time.Second).MustWait(`()=>!golfFixture.inspect().visible&&golfFixture.inspect().audio.paused`)
	if benchmark {
		page.MustSetViewport(1920, 1080, 1, false)
		baseline := page.Timeout(20 * time.Second).MustEval(`async()=>JSON.stringify(await golfFixture.measureFrameIntervals(240))`).Str()
		t.Logf("minigolf 1920x1080 hidden-host baseline (course RAF paused, 240 continuous rAF intervals): %s", baseline)
		page.MustSetViewport(1280, 760, 1, false)
	}
	paused := page.MustEval(`()=>golfFixture.inspect().physics`).JSON("", " ")
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,300))`)
	if got := page.MustEval(`()=>golfFixture.inspect().physics`).JSON("", " "); got != paused {
		t.Fatalf("physics advanced while the Desktop window was hidden: before=%s after=%s", paused, got)
	}
	page.MustEval(`()=>{document.querySelector('#golf-host').style.display=''}`)
	page.Timeout(10 * time.Second).MustWait(`()=>golfFixture.inspect().visible&&golfFixture.inspect().audio.playing`)
	page.MustEval(`()=>golfFixture.reopen()`)
	page.MustWait(`()=>!golfFixture.inspect().loading&&golfFixture.inspect().panel==='menu'&&!!document.querySelector('[data-action="resume"]')`)
	page.MustElement(`[data-action="resume"]`).MustClick()
	page.Timeout(30*time.Second).MustWait(`before=>{const s=golfFixture.inspect(),saved=JSON.parse(before);return !s.loading&&s.panel===''&&s.round?.turnIndex===saved.turn&&JSON.stringify(s.round.players.map(p=>p.strokes))===JSON.stringify(saved.strokes)}`, checkpoint)
	if !page.MustEval(`()=>golfFixture.inspect().visible`).Bool() {
		t.Fatal("reopening did not resume the persisted interrupted round")
	}

	// Cycle all real practice-course choices through the normal menu and renderer.
	manifest, err := os.ReadFile("3d/minigolf/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var courseManifest struct {
		Courses []struct {
			ID     string `json:"id"`
			World  string `json:"world"`
			Number int    `json:"number"`
		} `json:"courses"`
	}
	if err := json.Unmarshal(manifest, &courseManifest); err != nil {
		t.Fatal(err)
	}
	// Menu paths shared by the passes below: the pause panel leads to the start page,
	// Training to the hole grid, a tile to the player setup, Start into the course.
	openMenu := func() {
		if page.MustEval(`()=>{const p=golfFixture.inspect().panel;return p!=='pause'&&p!=='menu'}`).Bool() {
			page.MustElement(`.mg-topbar [data-action="pause"]`).MustClick()
			page.MustWait(`()=>golfFixture.inspect().panel==='pause'`)
		}
		if page.MustEval(`()=>golfFixture.inspect().panel==='pause'`).Bool() {
			page.MustElement(`.mg-panel [data-action="menu"]`).MustClick()
		}
		page.MustWait(`()=>{const s=golfFixture.inspect();return s.panel==='menu'&&s.menuStep==='home'}`)
	}
	openHoles := func() {
		openMenu()
		page.MustElement(`[data-round="practice"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().menuStep==='holes'&&document.querySelectorAll('[data-hole-id]').length===18`)
	}
	pickHole := func(id string) {
		page.MustElement(`[data-hole-id="` + id + `"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().menuStep==='players'`)
	}
	worldCaptured := map[string]bool{}
	// The three legacy worlds wear one dressed look each.
	lookOf := map[string]string{"garden": "temple", "coast": "pirate", "night": "moon"}
	introChecked := false
	for index, course := range courseManifest.Courses {
		openHoles()
		if index == 0 {
			layoutOK("holes 1280")
			shot("holes-1280")
		}
		pickHole(course.ID)
		// The headless Chrome here reports prefers-reduced-motion: reduce, which suppresses the
		// intro flight. The first course loads under emulated normal motion instead, and the
		// emulation is reset right after the flight was seen and skipped.
		if !introChecked {
			if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}).Call(page); err != nil {
				t.Fatalf("emulate normal motion: %v", err)
			}
			page.Timeout(5 * time.Second).MustWait(`()=>!matchMedia('(prefers-reduced-motion: reduce)').matches`)
		}
		page.MustElement(`[data-action="start"]`).MustClick()
		page.Timeout(30*time.Second).MustWait(`id=>{const s=golfFixture.inspect();return !s.loading&&s.course===id&&s.graphics?.geometries>0&&s.graphics?.triangles>0}`, course.ID)
		page.Timeout(15*time.Second).MustWait(`look=>{const g=golfFixture.inspect().graphics;return g.look===look&&g.props>0&&g.env===true}`, lookOf[course.World])
		if !introChecked {
			if !page.MustEval(`()=>golfFixture.inspect().graphics.intro===true`).Bool() {
				t.Fatalf("%s: loading a course did not start the intro flight", course.ID)
			}
			skipIntro()
			if err := (proto.EmulationSetEmulatedMedia{}).Call(page); err != nil {
				t.Fatalf("reset the media emulation: %v", err)
			}
			introChecked = true
		} else if page.MustEval(`()=>matchMedia('(prefers-reduced-motion: reduce)').matches`).Bool() {
			if page.MustEval(`()=>golfFixture.inspect().graphics.intro`).Bool() {
				t.Fatalf("%s: a course loaded under reduced motion still starts the intro flight", course.ID)
			}
		} else {
			// Without a reduced-motion preference every load flies; end it so later checks see a settled camera.
			skipIntro()
		}
		if artifactDir != "" && !worldCaptured[course.World] {
			if benchmark {
				page.MustElement(`[data-action="settings"]`).MustClick()
				page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
				page.MustEval(`()=>{const e=document.querySelector('[data-quality]');e.value='high';e.dispatchEvent(new Event('change',{bubbles:true}));}`)
				page.MustElement(`[data-action="close"]`).MustClick()
				page.MustWait(`()=>golfFixture.inspect().panel===''&&golfFixture.inspect().graphics?.quality==='high'`)
				page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
				page.MustScreenshot(filepath.Join(artifactDir, "minigolf-"+course.World+"-high.png"))
			} else {
				page.MustScreenshot(filepath.Join(artifactDir, "minigolf-"+course.World+".png"))
			}
			worldCaptured[course.World] = true
		}
		if course.Number == len(courseManifest.Courses) {
			break
		}
	}
	if artifactDir != "" && len(worldCaptured) != 3 {
		t.Fatalf("captured only %d world screenshots", len(worldCaptured))
	}
	if benchmark {
		page.MustSetViewport(1920, 1080, 1, false)
		for _, quality := range []string{"high", "medium", "low"} {
			page.MustElement(`[data-action="settings"]`).MustClick()
			page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
			page.MustEval(`quality=>{const element=document.querySelector('[data-quality]');element.value=quality;element.dispatchEvent(new Event('change',{bubbles:true}));}`, quality)
			page.MustElement(`[data-action="close"]`).MustClick()
			page.MustWait(`()=>golfFixture.inspect().panel===''`)
			result := page.Timeout(20 * time.Second).MustEval(`async()=>JSON.stringify(await golfFixture.benchmarkCurrentScene())`).Str()
			t.Logf("minigolf 1920x1080 quality=%s synchronized=%s", quality, result)
		}
	}

	for _, theme := range []string{"standard", "fruity"} {
		for _, mode := range []string{"light", "dark"} {
			page.MustEval(`(theme,mode)=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode=mode}`, theme, mode)
			for _, width := range []int{1280, 420} {
				page.MustSetViewport(width, 760, 1, width < 600)
				settle()
				label := fmt.Sprintf("%s/%s at %dpx", theme, mode, width)
				layoutOK(label)
				if !page.MustEval(`()=>{const name=document.querySelector('.mg-plaque-name');return name.getClientRects().length>0&&name.textContent.trim()!==''}`).Bool() {
					t.Fatalf("the hole plaque name is not visible for %s", label)
				}
				if width == 420 && !page.MustEval(`()=>{const names=[...document.querySelectorAll('.mg-chip-name')].filter(e=>e.getClientRects().length&&e.textContent.includes('Player '));return names.length>0&&names.every(e=>{const s=getComputedStyle(e),r=e.getBoundingClientRect(),root=document.querySelector('.mg-app').getBoundingClientRect();return s.overflow==='hidden'&&s.textOverflow==='ellipsis'&&r.left>=root.left-1&&r.right<=root.right+1})}`).Bool() {
					t.Fatalf("long player names escaped their ellipsis pills for %s", label)
				}
				if artifactDir != "" && width == 420 {
					page.MustScreenshot(filepath.Join(artifactDir, fmt.Sprintf("minigolf-%s-%s-420.png", theme, mode)))
				}
				page.MustElement(`.mg-topbar [data-action="pause"]`).MustClick()
				page.MustWait(`()=>golfFixture.inspect().panel==='pause'`)
				layoutOK(label + " with the pause panel")
				page.MustElement(`.mg-topbar [data-action="pause"]`).MustClick()
				page.MustWait(`()=>golfFixture.inspect().panel===''`)
			}
		}
	}

	// At the compact touch viewport, centering the camera makes the ball's
	// canvas coordinate deterministic. A tap must not shoot; a drag must.
	page.MustElement(`[data-action="focus"]`).MustClick()
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,180))`)
	page.Timeout(10 * time.Second).MustWait(`()=>(golfFixture.inspect().graphics?.preview||0)>=2`)
	center := page.MustEval(`()=>{const r=document.querySelector('.mg-canvas canvas').getBoundingClientRect();return{x:r.left+r.width/2,y:r.top+r.height/2}}`)
	touchX, touchY := center.Get("x").Num(), center.Get("y").Num()
	beforeTouch := page.MustEval(`()=>JSON.stringify({turn:golfFixture.inspect().round.turnIndex,strokes:golfFixture.inspect().round.players.map(p=>p.strokes)})`).Str()
	page.Touch.MustTap(touchX, touchY)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,120))`)
	if afterTap := page.MustEval(`()=>JSON.stringify({turn:golfFixture.inspect().round.turnIndex,strokes:golfFixture.inspect().round.players.map(p=>p.strokes)})`).Str(); afterTap != beforeTouch {
		t.Fatalf("a centered touch tap fired a shot: before=%s after=%s", beforeTouch, afterTap)
	}
	startTouch := &proto.InputTouchPoint{X: touchX, Y: touchY}
	dragTouch := &proto.InputTouchPoint{X: touchX, Y: touchY + 60}
	page.Touch.MustStart(startTouch).MustMove(dragTouch).MustEnd()
	page.MustWait(`()=>golfFixture.inspect().busy`)
	page.Timeout(30*time.Second).MustWait(`()=>!golfFixture.inspect().busy&&golfFixture.inspect().round.turnIndex!==JSON.parse(arguments[0]).turn`, beforeTouch)

	// Layout of the five screens at the phone and the desktop width, each shot for review.
	layoutPass := func(width int) {
		label := fmt.Sprint(width)
		page.MustSetViewport(width, 760, 1, width < 600)
		settle()
		page.MustWait(`()=>golfFixture.inspect().panel===''`)
		layoutOK("game " + label)
		shot("game-" + label)

		page.MustElement(`.mg-topbar [data-action="scorecard"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().panel==='scorecard'`)
		layoutOK("scorecard " + label)
		shot("scorecard-" + label)
		page.MustElement(`.mg-panel [data-action="close"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().panel===''`)

		openMenu()
		layoutOK("home " + label)
		if width != 1280 {
			shot("home-" + label)
		}
		page.MustElement(`[data-round="practice"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().menuStep==='holes'&&document.querySelectorAll('[data-hole-id]').length===18`)
		layoutOK("holes " + label)
		shot("holes-" + label)
		pickHole("garden-01")
		layoutOK("players " + label)
		shot("players-" + label)
		if width < 600 {
			if !page.MustEval(`()=>document.querySelector('.mg-topbar [data-action="help"]').getClientRects().length>0`).Bool() {
				t.Fatal("the help button is hidden at 420px")
			}
			page.MustElement(`.mg-topbar [data-action="help"]`).MustClick()
			page.MustWait(`()=>golfFixture.inspect().panel==='help'`)
			layoutOK("help " + label)
			page.MustElement(`.mg-panel [data-action="close"]`).MustClick()
			page.MustWait(`()=>{const s=golfFixture.inspect();return s.panel==='menu'&&s.menuStep==='players'}`)
		}
		page.MustElement(`.mg-panel [data-action="back"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().menuStep==='holes'`)
		page.MustElement(`.mg-step-head [data-action="back"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().menuStep==='home'`)
		page.MustElement(`[data-action="resume"]`).MustClick()
		page.Timeout(30 * time.Second).MustWait(`()=>!golfFixture.inspect().loading&&golfFixture.inspect().panel===''`)
	}
	layoutPass(420)
	layoutPass(1280)

	// Measure the live WebAudio output while normal gameplay is actively
	// triggering strike/rolling effects through the enabled Shoot control.
	openMenu()
	page.MustElement(`[data-round="garden"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().menuStep==='players'`)
	page.MustEval(`()=>{const radio=document.querySelector('[data-player-count][value="1"]');radio.checked=true;radio.dispatchEvent(new Event('change',{bubbles:true}))}`)
	page.MustElement(`[data-action="start"]`).MustClick()
	page.Timeout(30 * time.Second).MustWait(`()=>!golfFixture.inspect().loading&&golfFixture.inspect().panel===''&&golfFixture.inspect().course==='garden-01'&&golfFixture.inspect().audio.playing`)
	activePlayJSON := page.Timeout(60 * time.Second).MustEval(`async()=>JSON.stringify(await golfFixture.measureActivePlayAudioStats(15000))`).Str()
	var activePlay struct {
		DurationMs    float64 `json:"durationMs"`
		AcceptedShots int     `json:"acceptedShots"`
		NextHoles     int     `json:"nextHoles"`
		Playing       bool    `json:"playing"`
		Supported     bool    `json:"supported"`
		Deltas        []struct {
			Index int    `json:"index"`
			State string `json:"state"`
			Delta struct {
				UnderrunEvents   *float64 `json:"underrunEvents"`
				UnderrunDuration *float64 `json:"underrunDuration"`
				TotalDuration    *float64 `json:"totalDuration"`
			} `json:"delta"`
		} `json:"deltas"`
	}
	if err := json.Unmarshal([]byte(activePlayJSON), &activePlay); err != nil {
		t.Fatalf("parse active-play AudioContext stats: %v (%s)", err, activePlayJSON)
	}
	t.Logf("active-play audio telemetry during %0.1fs of normal DOM shots: %s", activePlay.DurationMs/1000, activePlayJSON)
	if activePlay.AcceptedShots < 1 || !activePlay.Playing {
		t.Fatalf("active-play measurement did not retain live music and accepted shots: %s", activePlayJSON)
	}
	if !activePlay.Supported {
		t.Fatalf("browser does not expose AudioContext playback underrun stats: %s", activePlayJSON)
	}
	advancedOutput := false
	for _, delta := range activePlay.Deltas {
		if delta.State == "running" && delta.Delta.TotalDuration != nil && *delta.Delta.TotalDuration >= 1 {
			advancedOutput = true
		}
		if delta.Delta.UnderrunEvents != nil && *delta.Delta.UnderrunEvents > 0 || delta.Delta.UnderrunDuration != nil && *delta.Delta.UnderrunDuration > 0 {
			t.Fatalf("active gameplay reported audio output underruns: %s", activePlayJSON)
		}
	}
	if !advancedOutput {
		t.Fatalf("no running AudioContext output advanced during active gameplay: %s", activePlayJSON)
	}
	if artifactDir != "" {
		var pretty []byte
		if pretty, err = json.MarshalIndent(json.RawMessage(activePlayJSON), "", "  "); err != nil {
			t.Fatalf("format active-play telemetry: %v", err)
		}
		if err = os.WriteFile(filepath.Join(artifactDir, "active-play-audio-stats.json"), pretty, 0644); err != nil {
			t.Fatalf("save active-play telemetry: %v", err)
		}
	}

	// A decided round on a fresh profile: the last Garden hole is seeded with both players
	// finished, so Resume opens the scorecard and Next hole the podium. The save and the
	// reopen share one task, so no running shot can overwrite the seed in between.
	page.MustEval(`()=>{document.body.dataset.animations='false';localStorage.removeItem('aurago.desktop.minigolf.settings.v1')}`)
	page.MustEval(`async()=>{await golfFixture.seedFinalHole('garden');golfFixture.reopen()}`)
	page.MustWait(`()=>!golfFixture.inspect().loading&&golfFixture.inspect().panel==='menu'&&!!document.querySelector('[data-action="resume"]')`)
	page.MustElement(`[data-action="resume"]`).MustClick()
	page.Timeout(30 * time.Second).MustWait(`()=>golfFixture.inspect().panel==='scorecard'&&!!document.querySelector('[data-action="next-hole"]')`)
	if running := page.MustEval(`()=>document.querySelector('.mg-app').getAnimations({subtree:true}).length`).Int(); running != 0 {
		t.Fatalf("%d animations still run on the scorecard with animations disabled", running)
	}
	layoutOK("final-hole scorecard 1280")
	shot("scorecard-final-1280")
	page.MustElement(`[data-action="next-hole"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().round.completed&&document.querySelectorAll('.mg-podium .mg-step').length===2`)
	if !page.MustEval(`()=>document.querySelector('.mg-winner').textContent.includes('Ada')`).Bool() {
		t.Fatalf("the podium names the wrong winner: %s", page.MustEval(`()=>document.querySelector('.mg-winner').textContent`).Str())
	}
	for _, width := range []int{1280, 420} {
		page.MustSetViewport(width, 760, 1, width < 600)
		settle()
		// Two players stand as a centred pair: equal free space left and right of the two steps.
		if !page.MustEval(`()=>{const list=document.querySelector('.mg-podium'),box=list.getBoundingClientRect(),steps=[...list.children].map(e=>e.getBoundingClientRect());return list.dataset.count==='2'&&steps.length===2&&Math.abs((steps[0].left-box.left)-(box.right-steps[1].right))<=2}`).Bool() {
			t.Fatalf("the two-player podium is not a centred pair at %dpx", width)
		}
		layoutOK(fmt.Sprintf("podium %dpx", width))
		shot(fmt.Sprintf("podium-%d", width))
	}
	page.MustSetViewport(1280, 760, 1, false)
	settle()
	page.MustElement(`[data-action="again"]`).MustClick()
	page.Timeout(30 * time.Second).MustWait(`()=>{const s=golfFixture.inspect();return !s.loading&&s.panel===''&&s.course==='garden-01'&&s.round.holeIndex===0}`)
	if got := page.MustEval(`()=>golfFixture.inspect().round.players.map(p=>p.name+':'+p.color).join(',')`).Str(); got != "Ada:teal,Lin:gold" {
		t.Fatalf("Play again changed the players: %s", got)
	}
	page.MustEval(`()=>{delete document.body.dataset.animations}`)

	// The fresh profile starts the coach marks again; skipping them is remembered, and Help restarts them.
	page.MustWait(`()=>golfFixture.inspect().tutorial==='aim'`)
	page.MustElement(`.mg-coach [data-action="tutorial-skip"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().tutorial===null&&JSON.parse(localStorage.getItem('aurago.desktop.minigolf.settings.v1')).tutorial===2`)
	page.MustElement(`.mg-topbar [data-action="help"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='help'`)
	page.MustElement(`.mg-panel [data-action="tutorial-again"]`).MustClick()
	page.MustWait(`()=>{const s=golfFixture.inspect();return s.panel===''&&s.tutorial==='aim'}`)
	page.MustElement(`.mg-coach [data-action="tutorial-skip"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().tutorial===null`)

	// A failed real course request keeps an actionable, translated retry.
	page.MustEval(`()=>golfFixture.setCourseFailure(true)`)
	openHoles()
	pickHole("garden-01")
	page.MustElement(`[data-action="start"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='error'`)
	if !page.MustEval(`()=>{const b=document.querySelector('[data-action="retry"]');return !!b&&b.textContent===I18N['minigolf.retry']&&document.querySelector('.mg-panel h2').textContent===I18N['minigolf.load_error']}`).Bool() {
		t.Fatal("course failure did not expose the translated retry action")
	}
	page.MustEval(`()=>golfFixture.setCourseFailure(false)`)
	page.MustElement(`[data-action="retry"]`).MustClick()
	page.Timeout(30 * time.Second).MustWait(`()=>golfFixture.inspect().panel===''&&golfFixture.inspect().graphics?.triangles>0`)

	page.MustElement(`[data-action="settings"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='settings'`)
	page.MustEval(`()=>{const e=document.querySelector('[data-music]');e.focus();e.value='0';e.dispatchEvent(new Event('input',{bubbles:true}));}`)
	if !page.MustEval(`()=>golfFixture.inspect().audio.musicVolume===0`).Bool() {
		t.Fatal("music settings did not persist the zero volume")
	}
	if !page.MustEval(`()=>document.querySelector('[data-action="mute"]').textContent===I18N['minigolf.unmute']&&document.activeElement===document.querySelector('[data-music]')`).Bool() {
		t.Fatal("the Mute button did not follow the music slider to zero, or the slider lost its focus")
	}
	page.MustElement(`[data-action="close"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''`)
	page.MustEval(`()=>golfFixture.dispose()`)
	if !page.MustEval(`()=>document.querySelector('#golf-host').childElementCount===0&&golfFixture.inspect().audio.disposed`).Bool() {
		t.Fatal("disposing the game left DOM or live audio behind")
	}

	if artifactDir != "" {
		page.MustScreenshot(filepath.Join(artifactDir, "minigolf-final.png"))
	}
	if errors := page.MustEval(`()=>golfFixture.errors`).JSON("", " "); errors != "[]" {
		t.Fatalf("browser reported uncaught errors: %s", errors)
	}
	// Nothing may load from outside the test origin: not through the fixture's
	// recorded fetches (path and query only) and not through any page resource.
	if offsite := page.MustEval(`()=>JSON.stringify({requests:golfFixture.assetRequests.filter(path=>path.startsWith('//')||path.startsWith('http')),resources:golfFixture.foreignResources()})`).Str(); offsite != `{"requests":[],"resources":[]}` {
		t.Fatalf("minigolf requested resources outside the test origin: %s", offsite)
	}
	page.MustNavigate(server.URL + "/fixture?no-webgl=1").MustWaitLoad()
	page.Timeout(30 * time.Second).MustWait(`()=>!!window.golfFixture?.inspect&&golfFixture.inspect().panel==='error'`)
	if !page.MustEval(`()=>document.querySelector('.mg-panel h2')?.textContent===I18N['minigolf.webgl_error']&&!!document.querySelector('[data-action="retry"]')`).Bool() {
		t.Fatal("WebGL startup failure did not show the localized fallback and retry")
	}
	page.MustEval(`()=>golfFixture.dispose()`)
}
