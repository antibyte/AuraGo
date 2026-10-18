package ui

import (
	"encoding/json"
	"fmt"
	"io"
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
	mux.HandleFunc("/__minigolf-render/", func(w http.ResponseWriter, r *http.Request) {
		if artifactDir == "" || r.Method != http.MethodPost {
			http.Error(w, "render artifact saving is disabled", http.StatusNotFound)
			return
		}
		name := filepath.Base(r.URL.Path)
		if name != "a-simple-trifle" && name != "piano-emotional-solo-139" && name != "crystal-tune-bright-1" {
			http.Error(w, "unknown track", http.StatusBadRequest)
			return
		}
		bytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
		if err != nil {
			http.Error(w, "render artifact is too large", http.StatusRequestEntityTooLarge)
			return
		}
		if len(bytes) < 44 || string(bytes[:4]) != "RIFF" || string(bytes[8:12]) != "WAVE" {
			http.Error(w, "invalid WAV render", http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(filepath.Join(artifactDir, "minigolf-"+name+".wav"), bytes, 0644); err != nil {
			http.Error(w, "could not save render", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-app-minigolf.css">
<style>html,body,#golf-host,.vd-window{margin:0;width:100%;height:100%;overflow:hidden}.vd-window{position:relative}body{font-family:system-ui,sans-serif}</style>
<script src="/js/vendor/synth-studio/gm-data.js"></script><script src="/js/desktop/apps/synth-studio-voices.js"></script><script src="/js/desktop/apps/synth-studio-audio.js"></script>
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
	page := browser.MustPage("about:blank").Timeout(180 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 760, 1, false)
	fixtureURL := server.URL + "/fixture"
	if artifactDir != "" {
		fixtureURL += "?save-renders=1"
	}
	page.MustNavigate(fixtureURL).MustWaitLoad()
	page.MustWait(`()=>!!window.golfFixture?.inspect && window.golfFixture.inspect().loading===false && window.golfFixture.inspect().panel==='menu'`)
	if !page.MustEval(`()=>window.I18N['minigolf.title']==='AuraGolf' && golfFixture.inspect().audio.unlocked===false`).Bool() {
		t.Fatal("fixture did not load the real locale or audio stayed unlocked before a gesture")
	}
	page.MustEval(`()=>{const e=document.querySelector('[data-players]');e.value='2';e.dispatchEvent(new Event('change',{bubbles:true}));const names=document.querySelectorAll('[data-player-name]');names[0].value='Player 1 Has A Very Long AuraGolf Name';names[1].value='Player 2 Has A Very Long AuraGolf Name';}`)
	page.MustEval(`()=>golfFixture.armIndependentPlayerTest(document.querySelector('[data-action="start"]'))`)
	page.MustElement(`[data-action="start"]`).MustClick()
	page.MustWait(`()=>{const s=golfFixture.inspect();return !s.loading&&s.panel==='tutorial'&&s.course==='garden-01'&&s.graphics?.geometries>0&&s.graphics?.triangles>0}`)
	if !page.MustEval(`()=>golfFixture.inspect().audio.unlocked && golfFixture.inspect().round.players.length===2`).Bool() {
		t.Fatal("first gesture did not unlock audio or two-player mode was not applied")
	}
	if !page.MustEval(`()=>golfFixture.inspect().round.players[0].name==='Player 1 Has A Very Long AuraGolf'&&golfFixture.inspect().round.players[1].name==='Player 2 Has A Very Long AuraGolf'`).Bool() {
		t.Fatalf("long player names did not survive menu validation: %s", page.MustEval(`()=>JSON.stringify(golfFixture.inspect().round.players.map(p=>p.name))`).Str())
	}
	page.Timeout(10 * time.Second).MustWait(`()=>!!golfFixture.independentPlayersPromise`)
	if !page.MustEval(`async()=>{const result=await golfFixture.independentPlayersPromise;return result.independent}`).Bool() {
		t.Fatal("stopping one Synth Studio player interrupted its simultaneous peer")
	}
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
	t.Logf("uninterrupted live music during the visible 3D course intro overlay, before any shots (Chrome playbackStats when available): %s", playbackJSON)
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
	page.MustElement(`[data-action="close"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''`)
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
	pausedMusicTick := page.MustEval(`()=>golfFixture.inspect().audio.tick`).Num()
	samplesJSON := page.Timeout(90 * time.Second).MustEval(`async()=>JSON.stringify(await golfFixture.renderMusicSamples())`).Str()
	var samples []struct {
		ID                string  `json:"id"`
		Seconds           float64 `json:"seconds"`
		RMS               float64 `json:"rms"`
		Peak              float64 `json:"peak"`
		FullScaleFraction float64 `json:"fullScaleFraction"`
	}
	if err := json.Unmarshal([]byte(samplesJSON), &samples); err != nil {
		t.Fatalf("parse short MIDI renders: %v (%s)", err, samplesJSON)
	}
	if len(samples) != 3 {
		t.Fatalf("rendered %d MIDI samples, expected all three", len(samples))
	}
	for _, sample := range samples {
		if sample.Seconds < 4 || sample.Seconds > 10 || sample.RMS < .0005 || sample.Peak <= .01 || sample.FullScaleFraction > .001 {
			t.Fatalf("MIDI render is silent or outside bounds: %+v", sample)
		}
	}
	if got := page.MustEval(`()=>golfFixture.inspect().audio.tick`).Num(); got != pausedMusicTick {
		t.Fatalf("offline WAV render advanced the paused live playlist: before=%f after=%f", pausedMusicTick, got)
	}
	page.MustElement(`[data-action="pause"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel===''&&golfFixture.inspect().audio.playing`)
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,100))`)
	if !page.MustEval(`tick=>golfFixture.inspect().audio.tick>tick`, pausedMusicTick).Bool() {
		t.Fatal("live playlist did not continue from its saved position after offline WAV rendering")
	}
	page.MustEval(`()=>document.querySelector('.mg-canvas canvas').focus()`)
	page.Keyboard.MustType(input.Space)
	page.MustWait(`()=>golfFixture.inspect().busy`)
	page.Timeout(30 * time.Second).MustWait(`()=>!golfFixture.inspect().busy&&golfFixture.inspect().round.turnIndex===1`)
	if !page.MustEval(`()=>golfFixture.inspect().round.players[0].strokes===1`).Bool() {
		t.Fatal("keyboard shot did not resolve for the first player")
	}

	// Hiding the Desktop window pauses both audio and simulation, then resuming
	// the reopened app continues from its persisted round checkpoint.
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
	page.Timeout(30 * time.Second).MustWait(`()=>!golfFixture.inspect().loading&&golfFixture.inspect().panel===''&&golfFixture.inspect().round?.players[1]?.strokes===0&&golfFixture.inspect().round?.turnIndex===1`)
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
	worldCaptured := map[string]bool{}
	for _, course := range courseManifest.Courses {
		if page.MustEval(`()=>golfFixture.inspect().panel!== 'pause' && golfFixture.inspect().panel!== 'menu'`).Bool() {
			page.MustElement(`[data-action="pause"]`).MustClick()
			page.MustWait(`()=>golfFixture.inspect().panel==='pause'`)
		}
		page.MustElement(`[data-action="menu"]`).MustClick()
		page.MustWait(`()=>golfFixture.inspect().panel==='menu'`)
		page.MustEval(`id=>{const mode=document.querySelector('[data-mode]');mode.value='practice';mode.dispatchEvent(new Event('change',{bubbles:true}));const hole=document.querySelector('[data-hole-id]');hole.value=id;const names=document.querySelectorAll('[data-player-name]');if(names.length===2){names[0].value='Player 1 Has A Very Long AuraGolf Name';names[1].value='Player 2 Has A Very Long AuraGolf Name';}}`, course.ID)
		page.MustElement(`[data-action="start"]`).MustClick()
		page.Timeout(30*time.Second).MustWait(`id=>{const s=golfFixture.inspect();return !s.loading&&s.course===id&&s.graphics?.geometries>0&&s.graphics?.triangles>0}`, course.ID)
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
				if !page.MustEval(`()=>{const root=document.querySelector('.mg-app'),bounds=root.getBoundingClientRect();return [...root.querySelectorAll('button,input,select')].filter(e=>e.getClientRects().length).every(e=>{const b=e.getBoundingClientRect();return b.left>=bounds.left-1&&b.right<=bounds.right+1&&b.top>=bounds.top-1&&b.bottom<=bounds.bottom+1})}`).Bool() {
					t.Fatalf("horizontal overflow for %s/%s at %dpx", theme, mode, width)
				}
				if width == 420 && !page.MustEval(`()=>[...document.querySelectorAll('.mg-player-strip span')].filter(e=>e.textContent.includes('Player 1 Has A Very Long AuraGolf')||e.textContent.includes('Player 2 Has A Very Long AuraGolf')).every(e=>{const s=getComputedStyle(e),r=e.getBoundingClientRect(),root=document.querySelector('.mg-app').getBoundingClientRect();return s.overflow==='hidden'&&s.textOverflow==='ellipsis'&&r.left>=root.left-1&&r.right<=root.right+1})`).Bool() {
					t.Fatalf("long player names escaped their ellipsis pills for %s/%s at 420px", theme, mode)
				}
				if artifactDir != "" && width == 420 {
					page.MustScreenshot(filepath.Join(artifactDir, fmt.Sprintf("minigolf-%s-%s-420.png", theme, mode)))
				}
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

	// Measure the live WebAudio output while normal gameplay is actively
	// triggering strike/rolling effects through the enabled Shoot control.
	page.MustSetViewport(1280, 760, 1, false)
	page.MustElement(`[data-action="pause"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='pause'`)
	page.MustElement(`[data-action="menu"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='menu'`)
	page.MustEval(`()=>{const mode=document.querySelector('[data-mode]');mode.value='garden';mode.dispatchEvent(new Event('change',{bubbles:true}));const players=document.querySelector('[data-players]');players.value='1';players.dispatchEvent(new Event('change',{bubbles:true}));}`)
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

	// A failed real course request keeps an actionable, translated retry.
	page.MustEval(`()=>golfFixture.setCourseFailure(true)`)
	page.MustElement(`[data-action="pause"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='pause'`)
	page.MustElement(`[data-action="menu"]`).MustClick()
	page.MustWait(`()=>golfFixture.inspect().panel==='menu'`)
	page.MustEval(`()=>{const mode=document.querySelector('[data-mode]');mode.value='practice';mode.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('[data-hole-id]').value='garden-01';}`)
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
	page.MustEval(`()=>{const e=document.querySelector('[data-music]');e.value='0';e.dispatchEvent(new Event('input',{bubbles:true}));}`)
	if !page.MustEval(`()=>golfFixture.inspect().audio.musicVolume===0`).Bool() {
		t.Fatal("music settings did not persist the zero volume")
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
	page.MustNavigate(server.URL + "/fixture?no-webgl=1").MustWaitLoad()
	page.Timeout(30 * time.Second).MustWait(`()=>!!window.golfFixture?.inspect&&golfFixture.inspect().panel==='error'`)
	if !page.MustEval(`()=>document.querySelector('.mg-panel h2')?.textContent===I18N['minigolf.webgl_error']&&!!document.querySelector('[data-action="retry"]')`).Bool() {
		t.Fatal("WebGL startup failure did not show the localized fallback and retry")
	}
	page.MustEval(`()=>golfFixture.dispose()`)
}
