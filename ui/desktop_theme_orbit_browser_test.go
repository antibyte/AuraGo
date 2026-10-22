package ui

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Browser tests of the Orbit theme: the live space scene, the particle layer and the window bridge.
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run '^TestDesktopOrbit(Browser|SoftwareFallbackBrowser|ReducedMotionBrowser)$' -count=1 -timeout 15m
//
// Headless Chrome draws WebGL with SwiftShader, which the scene refuses unless window.__auraOrbitSoftwareGL is true, so
// the tests set that flag before the theme starts (the flag does nothing on a real GPU). Frames are slow on SwiftShader:
// the tests lock a fixed quality level and wait for frame counters instead of timing. TestDesktopOrbitCloseUpsBrowser
// writes the review images of the scene and the 3D window effects to the git-ignored
// reports/desktop-experience-themes/orbit/ and only runs on request:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 go test ./ui -run '^TestDesktopOrbitCloseUpsBrowser$' -count=1 -timeout 15m

func orbitCloseUpDir() string {
	return filepath.Join("..", "reports", "desktop-experience-themes", "orbit")
}

// newOrbitSession opens the fixture desktop at 1920x1080 in a headless browser that is allowed to use SwiftShader.
func newOrbitSession(t *testing.T, reducedMotion string, allowSoftwareGL bool) *themeSweepSession {
	t.Helper()
	s := newThemeSweepSessionWithBrowser(t, reducedMotion, func(l *launcher.Launcher) { l.Delete("disable-gpu").Set("enable-unsafe-swiftshader") })
	s.page.MustSetViewport(1920, 1080, 1, false)
	if allowSoftwareGL {
		s.eval(t, `()=>{window.__auraOrbitSoftwareGL=true;}`)
	}
	return s
}

// startOrbit selects the theme with its wallpaper and waits until the scene has drawn, then locks the quality level.
func startOrbit(t *testing.T, s *themeSweepSession, animations bool, level int) {
	t.Helper()
	anim := "false"
	if animations {
		anim = "true"
	}
	s.store.set(map[string]string{"appearance.theme": "orbit", "appearance.wallpaper": "orbit", "windows.animations": anim})
	s.pushSettings(t)
	s.waitFor(t, "the orbit effect", 20*time.Second, `()=>AuraDesktopThemeFx.activeId()==='orbit'`)
	s.waitFor(t, "the orbit scene object", 30*time.Second, `()=>!!AuraDesktopThemeFx.inspect().scene`)
	s.eval(t, `(l)=>AuraDesktopThemeFx.command('orbit.quality',{level:l})`, level)
	s.waitFor(t, "the first frames of the orbit scene", 120*time.Second, `()=>{const s=AuraDesktopThemeFx.inspect().scene;return !!s && ((s.ready && s.frames>1) || s.backend==='software' || s.backend==='error' || s.backend==='none');}`)
}

func orbitSceneJSON(t *testing.T, s *themeSweepSession) string {
	t.Helper()
	return s.eval(t, `()=>JSON.stringify(AuraDesktopThemeFx.inspect().scene)`).Value.Str()
}

// orbitConsoleErrors returns the console errors and exceptions the page logged so far.
func orbitConsoleErrors(s *themeSweepSession) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, line := range s.console {
		if (strings.HasPrefix(line, "error:") || strings.HasPrefix(line, "exception:")) && !strings.Contains(line, "ResizeObserver loop") {
			out = append(out, line)
		}
	}
	return out
}

func TestDesktopOrbitBrowser(t *testing.T) {
	s := newOrbitSession(t, "no-preference", true)
	startOrbit(t, s, true, 1)

	// 1. The scene runs on WebGL with all four models and is visible.
	if backend := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.backend`).Value.Str(); backend != "three-webgl" {
		t.Fatalf("the orbit scene backend is %q, want three-webgl: %s", backend, orbitSceneJSON(t, s))
	}
	s.waitFor(t, "the orbit scene to be ready with all models", 60*time.Second, `()=>{const s=AuraDesktopThemeFx.inspect().scene;return s.ready && Object.values(s.models).every(m=>m!=='pending');}`)
	for _, model := range []string{"station", "shuttle", "satellite", "asteroids"} {
		if got := s.eval(t, `(m)=>AuraDesktopThemeFx.inspect().scene.models[m]`, model).Value.Str(); got != "ok" {
			t.Errorf("model %s is %q, want ok: %s", model, got, orbitSceneJSON(t, s))
		}
	}
	s.waitFor(t, "the scene canvas to be visible", 15*time.Second, `()=>{const h=document.querySelector('.vd-fx-orbit-scene');return !!h && !!h.querySelector('canvas') && getComputedStyle(h).opacity==='1';}`)
	t.Log("scene:", orbitSceneJSON(t, s))

	// 2. Windows opening and closing emit particles.
	emitted := func() int { return s.eval(t, `()=>AuraDesktopThemeFx.inspect().particles.emitted`).Value.Int() }
	before := emitted()
	s.openApps(t, "calculator")
	s.waitFor(t, "the open burst", 5*time.Second, `(n)=>AuraDesktopThemeFx.inspect().particles.emitted>=n+100`, before)
	before = emitted()
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');void auditTest.closeWindow(w.id);}`)
	s.waitFor(t, "the close burst", 5*time.Second, `(n)=>AuraDesktopThemeFx.inspect().particles.emitted>=n+100`, before)
	s.waitFor(t, "the calculator to be gone", 5*time.Second, `()=>!document.querySelector('.vd-window[data-app-id="calculator"]')`)

	// 3. Dragging a window by its title bar tilts it, and the tilt goes away after the release.
	s.openApps(t, "calculator")
	// far from its taskbar button, so that a stream aimed at the window's own column cannot pass for one aimed at the button
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');w.element.style.left=Math.max(0,innerWidth-250-w.element.offsetWidth/2)+'px';w.element.style.top='80px';dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	time.Sleep(900 * time.Millisecond) // the opening animation is over
	from := s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');const r=w.element.getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2,dx:w.element.style.getPropertyValue('--vd-minimize-dx')};}`).Value
	t.Logf("minimizing the calculator from (%.0f, %.0f), --vd-minimize-dx %q", from.Get("x").Num(), from.Get("y").Num(), from.Get("dx").Str())
	before = emitted()
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');auditTest.minimizeWindow(w.id);}`)
	s.waitFor(t, "the minimize stream", 5*time.Second, `(n)=>AuraDesktopThemeFx.inspect().particles.emitted>=n+60`, before)
	s.waitFor(t, "the minimized window to hide", 5*time.Second, `()=>document.querySelector('.vd-window[data-app-id="calculator"]').style.display==='none'`)
	// The stream is aimed at the window's taskbar button, also on the very first minimize (the shell writes its delta after the event).
	aim := s.eval(t, `()=>{
		const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');
		const button=document.querySelector('#vd-taskbar-apps [data-window-id="'+CSS.escape(w.id)+'"]');
		const target=AuraDesktopThemeFx.inspect().windows.lastStreamTarget;
		if(!button||!target)return {button:!!button,target:!!target};
		const r=button.getBoundingClientRect();
		return {button:true,target:true,dx:target.x-(r.left+r.width/2),dy:target.y-(r.top+r.height/2),bx:r.left+r.width/2,by:r.top+r.height/2,tx:target.x,ty:target.y};
	}`).Value
	if !aim.Get("button").Bool() || !aim.Get("target").Bool() {
		t.Errorf("minimize stream aim: taskbar button found %v, stream target recorded %v", aim.Get("button").Bool(), aim.Get("target").Bool())
	} else {
		t.Logf("minimize stream aims at (%.0f, %.0f), the taskbar button is centred at (%.0f, %.0f)", aim.Get("tx").Num(), aim.Get("ty").Num(), aim.Get("bx").Num(), aim.Get("by").Num())
		if math.Abs(aim.Get("dx").Num()) > 24 || math.Abs(aim.Get("dy").Num()) > 24 {
			t.Errorf("the minimize stream misses the taskbar button by (%.0f, %.0f) px; want within 24 px", aim.Get("dx").Num(), aim.Get("dy").Num())
		}
	}
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');auditTest.focusWindow(w.id);}`)
	s.waitFor(t, "the restored window", 5*time.Second, `()=>AuraDesktopWindowEffects.snapshot().some(w=>w.appId==='calculator' && w.visible!==false && !w.minimized)`)
	if got := s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');return [AuraDesktopThemeFx.command('orbit.burst',{kind:'dissolve',windowId:w.id}),AuraDesktopThemeFx.command('orbit.burst',{kind:'materialize',windowId:'no-such-window'})];}`).Value; !got.Get("0").Bool() || !got.Get("1").Nil() {
		t.Errorf("orbit.burst answers %s, want [true, null]", got.String())
	}
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');Object.assign(w.element.style,{left:'700px',top:'300px'});dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	time.Sleep(900 * time.Millisecond) // the opening animation is over
	grip := s.eval(t, `()=>{const r=document.querySelector('.vd-window[data-app-id="calculator"] .vd-window-titlebar').getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2};}`).Value
	x, y := grip.Get("x").Num(), grip.Get("y").Num()
	if err := s.page.Mouse.MoveTo(proto.Point{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	tilted := false
	for i := 1; i <= 10; i++ {
		if err := s.page.Mouse.MoveTo(proto.Point{X: x + float64(i)*45, Y: y + float64(i)*10}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(16 * time.Millisecond)
		if s.eval(t, `()=>{const el=document.querySelector('.vd-window[data-app-id="calculator"]');return el.hasAttribute('data-orbit-tilt') && Math.abs(parseFloat(el.style.getPropertyValue('--vd-orbit-tilt-y')))>0.2;}`).Value.Bool() {
			tilted = true
		}
	}
	if !tilted {
		t.Errorf("the dragged window never got data-orbit-tilt with a --vd-orbit-tilt-y; windows: %s", s.eval(t, `()=>JSON.stringify(AuraDesktopThemeFx.inspect().windows)`).Value.Str())
	}
	if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	s.waitFor(t, "the tilt to settle after the drag", 2*time.Second, `()=>!document.querySelector('.vd-window[data-app-id="calculator"]').hasAttribute('data-orbit-tilt')`)

	// A window that covers the screen slows the scene down; the plumbing is coverage -> setState -> throttled.
	s.openApps(t, "files")
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='files');auditTest.toggleMaximizeWindow(w.id);}`)
	s.waitFor(t, "the maximized window", 5*time.Second, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='files');return w.maximized===true;}`)
	s.waitFor(t, "the throttled scene behind a screen-filling window", 5*time.Second, `()=>AuraDesktopThemeFx.inspect().scene.throttled===true`)
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='files');void auditTest.closeWindow(w.id);}`)
	s.waitFor(t, "the scene to run at full cadence again", 5*time.Second, `()=>AuraDesktopThemeFx.inspect().scene.throttled===false`)

	// Integrated GPUs: the scene publishes its quality on the body, and at 2 and below the window frame loses its blur.
	s.openApps(t, "calculator")
	frameFilter := `()=>getComputedStyle(document.querySelector('.vd-window')).backdropFilter`
	if got := s.eval(t, `()=>document.body.dataset.orbitQuality`).Value.Str(); got != "1" {
		t.Errorf("data-orbit-quality is %q at quality 1", got)
	}
	if got := s.eval(t, frameFilter).Value.Str(); !strings.Contains(got, "blur") {
		t.Errorf("the window frame has no backdrop blur at quality 1: %q", got)
	}
	s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.quality',{level:2})`)
	if got := s.eval(t, `()=>document.body.dataset.orbitQuality`).Value.Str(); got != "2" {
		t.Errorf("data-orbit-quality is %q at quality 2", got)
	}
	if got := s.eval(t, frameFilter).Value.Str(); got != "none" {
		t.Errorf("the window frame keeps its backdrop blur at quality 2: %q", got)
	}
	s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.quality',{level:1})`)
	s.waitFor(t, "the blur to return at quality 1", 5*time.Second, `()=>document.body.dataset.orbitQuality==='1' && getComputedStyle(document.querySelector('.vd-window')).backdropFilter.includes('blur')`)

	// 4. The sonar ping. The scene draws the scanner pulse, so the overlay keeps only the sparks; the call reaches the sound
	// system (a spy around DesktopSounds.play records it, the sound system itself decides what is audible).
	s.eval(t, `()=>{window.orbitSoundCalls=[];const sounds=window.DesktopSounds;if(!sounds)return;const play=sounds.play;sounds.play=function(...args){window.orbitSoundCalls.push([args[0],args[1]&&args[1].variant]);return play.apply(this,args);};}`)
	before = emitted()
	if !s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.ping',{x:300,y:300})`).Value.Bool() {
		t.Error("command orbit.ping did not return true")
	}
	if got := s.eval(t, `()=>({rings:AuraDesktopThemeFx.inspect().particles.rings,pulse:AuraDesktopThemeFx.inspect().scene.pulse,emitted:AuraDesktopThemeFx.inspect().particles.emitted,sounds:window.orbitSoundCalls,sink:typeof window.DesktopSounds==='object'?DesktopSounds.inspect().themeFx.orbit.theme:null})`).Value; got.Get("rings").Int() != 0 || got.Get("pulse").Nil() || got.Get("emitted").Int() < before+24 {
		t.Errorf("a ping with the scene running should start the scene pulse and the sparks but no overlay ring: %s", got.String())
	} else if got.Get("sink").Nil() {
		t.Log("the fixture has no sound system; the orbit.ping call was not observed")
	} else if calls := got.Get("sounds").String(); !strings.Contains(calls, "orbit.ping") || !strings.Contains(calls, "medium") || got.Get("sink").Str() != "orbit" {
		t.Errorf("the ping did not reach DesktopSounds.play('orbit.ping', {variant: 'medium'}): %s", got.String())
	}
	// Without the scene (another wallpaper) the overlay ring is the ping.
	s.store.set(map[string]string{"appearance.wallpaper": "groupshoot"})
	s.pushSettings(t)
	s.waitFor(t, "the effect without a scene", 20*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.activeId==='orbit' && f.wallpaper==='other' && !f.scene && !!f.particles;}`)
	if !s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.ping',{x:300,y:300})`).Value.Bool() {
		t.Error("command orbit.ping did not return true without a scene")
	}
	if rings := s.eval(t, `()=>AuraDesktopThemeFx.inspect().particles.rings`).Value.Int(); rings != 1 {
		t.Errorf("rings after one ping without a scene: %d, want 1", rings)
	}
	startOrbit(t, s, true, 1)

	// 5. Without window animations the scene stays, as a still frame, and the particle layer is gone.
	s.store.set(map[string]string{"windows.animations": "false"})
	s.pushSettings(t)
	s.waitFor(t, "the static orbit effect", 30*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.activeId==='orbit' && f.motion===false && !!f.scene;}`)
	s.eval(t, `(l)=>AuraDesktopThemeFx.command('orbit.quality',{level:l})`, 1)
	s.waitFor(t, "the static scene", 120*time.Second, `()=>{const s=AuraDesktopThemeFx.inspect().scene;return !!s && s.ready && s.frames>0;}`)
	if got := s.eval(t, `()=>({canvas:document.querySelectorAll('.vd-fx-orbit-particles').length,scene:document.querySelectorAll('.vd-fx-orbit-scene').length,particles:AuraDesktopThemeFx.inspect().particles})`).Value; got.Get("canvas").Int() != 0 || got.Get("scene").Int() != 1 || !got.Get("particles").Nil() {
		t.Errorf("static mode leaves the wrong layers: %s", got.String())
	}
	if got := s.eval(t, `async()=>{const before=AuraDesktopThemeFx.inspect().scene.frames;await new Promise(r=>setTimeout(r,800));return AuraDesktopThemeFx.inspect().scene.frames-before;}`).Value.Int(); got != 0 {
		t.Errorf("the static scene rendered %d frames in 800 ms", got)
	}

	// 6. Switching the theme releases everything.
	s.store.set(map[string]string{"appearance.theme": "standard", "windows.animations": "true"})
	s.pushSettings(t)
	s.waitFor(t, "the orbit effect to stop", 10*time.Second, `()=>AuraDesktopThemeFx.activeId()===''`)
	if got := s.eval(t, `()=>({layers:document.querySelectorAll('.vd-theme-fx-layer').length,scenes:document.querySelectorAll('.vd-fx-orbit-scene').length,mx:document.body.style.getPropertyValue('--vd-orbit-mx'),quality:document.body.dataset.orbitQuality||'',tilt:document.querySelectorAll('[data-orbit-tilt]').length})`).Value; got.Get("layers").Int() != 0 || got.Get("scenes").Int() != 0 || got.Get("mx").Str() != "" || got.Get("quality").Str() != "" || got.Get("tilt").Int() != 0 {
		t.Errorf("the orbit effect leaked after switching themes: %s", got.String())
	}

	// 7. No errors.
	if got := s.eval(t, `()=>fixtureErrors`).Value.String(); got != "[]" {
		t.Errorf("page errors: %s", got)
	}
	if errs := orbitConsoleErrors(s); len(errs) > 0 {
		t.Errorf("console errors: %q", errs)
	}
}

// Reduced motion keeps the scene as a still frame, with no particle layer and no loop of its own.
func TestDesktopOrbitReducedMotionBrowser(t *testing.T) {
	s := newOrbitSession(t, "reduce", true)
	startOrbit(t, s, true, 1)
	if backend := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.backend`).Value.Str(); backend != "three-webgl" {
		t.Skipf("the orbit scene needs WebGL; its backend here is %q", backend)
	}
	s.waitFor(t, "the still frame", 60*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.activeId==='orbit' && f.motion===false && f.scene.ready && f.scene.frames>0;}`)
	if got := s.eval(t, `async()=>{const before=AuraDesktopThemeFx.inspect().scene.frames;await new Promise(r=>setTimeout(r,800));return AuraDesktopThemeFx.inspect().scene.frames-before;}`).Value.Int(); got != 0 {
		t.Errorf("the still scene rendered %d frames in 800 ms", got)
	}
	// An explicit change of the clock draws exactly one new frame.
	before := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.frames`).Value.Int()
	s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.time',{t:70})`)
	if after := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.frames`).Value.Int(); after != before+1 {
		t.Errorf("orbit.time should draw one frame: %d -> %d", before, after)
	}
	if got := s.eval(t, `()=>({canvas:document.querySelectorAll('.vd-fx-orbit-particles').length,particles:AuraDesktopThemeFx.inspect().particles,windows:AuraDesktopThemeFx.inspect().windows})`).Value; got.Get("canvas").Int() != 0 || !got.Get("particles").Nil() || !got.Get("windows").Nil() {
		t.Errorf("reduced motion leaves particle machinery: %s", got.String())
	}
	if errs := orbitConsoleErrors(s); len(errs) > 0 {
		t.Errorf("console errors: %q", errs)
	}
}

// Without WebGL access the scene stays away and the poster wallpaper shows.
func TestDesktopOrbitSoftwareFallbackBrowser(t *testing.T) {
	// The default browser flags (no GPU) leave WebGL to the software path or to nothing at all.
	s := newThemeSweepSession(t, "no-preference")
	s.page.MustSetViewport(1920, 1080, 1, false)
	startOrbit(t, s, true, 0)
	backend := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.backend`).Value.Str()
	if backend == "three-webgl" {
		t.Skipf("this machine gives headless Chrome a hardware GPU (%s)", orbitSceneJSON(t, s))
	}
	if backend != "software" && backend != "none" {
		t.Fatalf("backend %q, want software or none: %s", backend, orbitSceneJSON(t, s))
	}
	if got := s.eval(t, `()=>{const h=document.querySelector('.vd-fx-orbit-scene');return h?getComputedStyle(h).display:'gone';}`).Value.Str(); got != "none" && got != "gone" {
		t.Errorf("the scene host is %q, want hidden", got)
	}
	// The poster shows instead: the wallpaper rule points at it and the file is the 1920x1080 render.
	if got := s.eval(t, `async()=>({bg:getComputedStyle(document.body).backgroundImage,width:await new Promise(r=>{const i=new Image();i.onload=()=>r(i.naturalWidth+'x'+i.naturalHeight);i.onerror=()=>r('missing');i.src='/img/wallpapers/themes/orbit.png';})})`).Value; !strings.Contains(got.Get("bg").Str(), "/img/wallpapers/themes/orbit.png") || got.Get("width").Str() != "1920x1080" {
		t.Errorf("the poster wallpaper is not in place: %s", got.String())
	}
	if os.Getenv("AURAGO_THEME_RECORDINGS") == "1" {
		s.screenshot(t, filepath.Join(orbitCloseUpDir(), "poster-fallback.png"))
	}
	if got := s.eval(t, `()=>fixtureErrors`).Value.String(); got != "[]" {
		t.Errorf("page errors: %s", got)
	}
	if errs := orbitConsoleErrors(s); len(errs) > 0 {
		t.Errorf("console errors: %q", errs)
	}
}

// orbitShot writes a PNG of the page, or of a clip of it (CSS px, scaled by scale).
func orbitShot(t *testing.T, s *themeSweepSession, name string, clip *proto.PageViewport) {
	t.Helper()
	time.Sleep(150 * time.Millisecond) // let the compositor present the newest frame
	data, err := s.page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng, Clip: clip})
	if err != nil {
		t.Fatalf("shot %s: %v", name, err)
	}
	if err := os.MkdirAll(orbitCloseUpDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orbitCloseUpDir(), name+".png"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func orbitClip(cx, cy, w, h, scale float64) *proto.PageViewport {
	x := min(max(cx-w/2, 0), 1920-w)
	y := min(max(cy-h/2, 0), 1080-h)
	return &proto.PageViewport{X: x, Y: y, Width: w, Height: h, Scale: scale}
}

func TestDesktopOrbitCloseUpsBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" || os.Getenv("AURAGO_THEME_RECORDINGS") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 and AURAGO_THEME_RECORDINGS=1 to write the Orbit close-ups")
	}
	s := newOrbitSession(t, "no-preference", true)
	startOrbit(t, s, true, 0)
	if backend := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.backend`).Value.Str(); backend != "three-webgl" {
		t.Skipf("the orbit scene needs WebGL; its backend here is %q: %s", backend, orbitSceneJSON(t, s))
	}
	s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.time',{t:44})`)
	waitFrames := func(n int) {
		t.Helper()
		before := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.frames`).Value.Int()
		s.waitFor(t, "fresh frames", 120*time.Second, `(n)=>{const s=AuraDesktopThemeFx.inspect().scene;return s.frames>=n && s.quality===0 && getComputedStyle(document.querySelector('.vd-fx-orbit-scene')).opacity==='1';}`, before+n)
	}
	waitFrames(3)
	t.Log("scene:", orbitSceneJSON(t, s))

	// The raw scene, exactly as the renderer draws it (no shell, no compositor).
	url := s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.capture',{width:1920,height:1080})`).Value.Str()
	if raw, ok := strings.CutPrefix(url, "data:image/png;base64,"); ok {
		if png, err := base64.StdEncoding.DecodeString(raw); err == nil {
			_ = os.MkdirAll(orbitCloseUpDir(), 0o755)
			_ = os.WriteFile(filepath.Join(orbitCloseUpDir(), "scene-capture.png"), png, 0o644)
		}
	} else {
		t.Errorf("orbit.capture returned %.40q", url)
	}
	waitFrames(2)

	// The desktop without windows.
	orbitShot(t, s, "scene", nil)
	planet := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.planet`).Value
	orbitShot(t, s, "closeup-planet", orbitClip(planet.Get("x").Num(), planet.Get("y").Num(), 960, 540, 1.6))
	orbitShot(t, s, "closeup-station", orbitClip(1920*0.24, 1080*0.30, 640, 360, 2))

	// Two apps open: the glass chrome over the scene.
	s.openApps(t, "files", "calculator")
	s.eval(t, `()=>{for(const w of auditTest.state.windows.values()){Object.assign(w.element.style,w.appId==='files'?{left:'140px',top:'120px',width:'700px',height:'560px'}:{left:'900px',top:'260px',width:'380px',height:'480px'});}dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	time.Sleep(1200 * time.Millisecond)
	waitFrames(2)
	orbitShot(t, s, "windows", nil)

	// 300 ms into the opening of a window.
	s.openApps(t, "terminal")
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='terminal');Object.assign(w.element.style,{left:'1100px',top:'120px',width:'560px',height:'360px'});dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	time.Sleep(1200 * time.Millisecond)
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='terminal');void auditTest.closeWindow(w.id);}`)
	time.Sleep(900 * time.Millisecond)
	s.eval(t, `()=>auditTest.openApp('terminal')`)
	time.Sleep(300 * time.Millisecond)
	orbitShot(t, s, "open-mid", nil)
	time.Sleep(1200 * time.Millisecond)

	// A window in the middle of a drag.
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='terminal');Object.assign(w.element.style,{left:'760px',top:'420px',width:'560px',height:'360px'});dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	time.Sleep(600 * time.Millisecond)
	grip := s.eval(t, `()=>{const r=document.querySelector('.vd-window[data-app-id="terminal"] .vd-window-titlebar').getBoundingClientRect();return {x:r.left+r.width/2,y:r.top+r.height/2};}`).Value
	x, y := grip.Get("x").Num(), grip.Get("y").Num()
	if err := s.page.Mouse.MoveTo(proto.Point{X: x, Y: y}); err != nil {
		t.Fatal(err)
	}
	if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 14; i++ {
		if err := s.page.Mouse.MoveTo(proto.Point{X: x + float64(i)*38, Y: y + float64(i)*14}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(14 * time.Millisecond)
	}
	t.Log("drag tilt:", s.eval(t, `()=>{const el=document.querySelector('.vd-window[data-app-id="terminal"]');const s=getComputedStyle(el);return el.getAttribute('data-orbit-tilt')+' '+s.getPropertyValue('--vd-orbit-tilt-x')+' '+s.getPropertyValue('--vd-orbit-tilt-y')+' '+s.getPropertyValue('--vd-orbit-lift')+' '+s.transform;}`).Value.Str())
	orbitShot(t, s, "drag-tilt", nil)
	if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(700 * time.Millisecond)

	// Menus.
	s.eval(t, `()=>document.getElementById('vd-start-button').click()`)
	time.Sleep(500 * time.Millisecond)
	orbitShot(t, s, "start-menu", nil)
	s.eval(t, `()=>document.getElementById('vd-start-button').click()`)
	time.Sleep(400 * time.Millisecond)
	s.eval(t, `()=>{const w=document.querySelector('.vd-workspace');w.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:420,clientY:760}));}`)
	time.Sleep(500 * time.Millisecond)
	orbitShot(t, s, "context-menu", nil)
	s.eval(t, `()=>document.body.dispatchEvent(new MouseEvent('pointerdown',{bubbles:true}))`)
	s.eval(t, `()=>{const m=document.querySelector('.vd-context-menu');if(m)m.remove();}`)

	// The scanner pulse, 150 ms after the ping.
	s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.ping',{x:1500,y:800})`)
	time.Sleep(150 * time.Millisecond)
	orbitShot(t, s, "ping", nil)

	t.Log("inspect:", s.eval(t, `()=>JSON.stringify(AuraDesktopThemeFx.inspect())`).Value.Str())
	if errs := orbitConsoleErrors(s); len(errs) > 0 {
		t.Errorf("console errors while shooting close-ups: %q", errs)
	}
	if errors := s.eval(t, `()=>fixtureErrors`).Value.String(); errors != "[]" {
		t.Fatalf("errors while shooting close-ups: %s", errors)
	}
}

// TestDesktopOrbitPosterBrowser renders the poster wallpaper: the still scene exactly as the reduced-motion desktop draws
// it (neutral camera, a moment of the cycle with the shuttle out and no comet, shooting star or flash, no film grain so
// that the PNG stays small), without the shell and the particle layer. It writes ui/img/wallpapers/themes/orbit.png and
// only runs on request:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_ORBIT_POSTER=1 go test ./ui -run '^TestDesktopOrbitPosterBrowser$' -count=1 -timeout 15m
func TestDesktopOrbitPosterBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" || os.Getenv("AURAGO_ORBIT_POSTER") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 and AURAGO_ORBIT_POSTER=1 to render the Orbit poster")
	}
	s := newOrbitSession(t, "reduce", true)
	startOrbit(t, s, true, 0)
	if backend := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.backend`).Value.Str(); backend != "three-webgl" {
		t.Fatalf("the poster needs the live scene; its backend here is %q: %s", backend, orbitSceneJSON(t, s))
	}
	s.waitFor(t, "the scene with all models", 60*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect(),s=f.scene;return f.motion===false && s.ready && Object.values(s.models).every(m=>m==='ok');}`)
	s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.time',{t:44})`)
	url := s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.capture',{width:1920,height:1080,grain:0})`).Value.Str()
	raw, ok := strings.CutPrefix(url, "data:image/png;base64,")
	if !ok {
		t.Fatalf("orbit.capture returned %.40q", url)
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("captured %d bytes", len(data))
	if len(data) > 2_500_000 {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&out, img); err != nil {
			t.Fatal(err)
		}
		t.Logf("re-encoded with the best compression: %d bytes", out.Len())
		if out.Len() < len(data) {
			data = out.Bytes()
		}
	}
	if len(data) > 2_800_000 {
		t.Errorf("the poster is %d bytes; the other theme wallpapers stay below 2.8 MB", len(data))
	}
	path := filepath.Join("img", "wallpapers", "themes", "orbit.png")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%d bytes)", path, len(data))
	if errs := orbitConsoleErrors(s); len(errs) > 0 {
		t.Errorf("console errors: %q", errs)
	}
}
