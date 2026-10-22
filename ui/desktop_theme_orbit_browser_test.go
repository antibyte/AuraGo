package ui

import (
	"encoding/base64"
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
	before = emitted()
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');auditTest.minimizeWindow(w.id);}`)
	s.waitFor(t, "the minimize stream", 5*time.Second, `(n)=>AuraDesktopThemeFx.inspect().particles.emitted>=n+60`, before)
	s.waitFor(t, "the minimized window to hide", 5*time.Second, `()=>document.querySelector('.vd-window[data-app-id="calculator"]').style.display==='none'`)
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

	// 4. The sonar ping.
	if !s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.ping',{x:300,y:300})`).Value.Bool() {
		t.Error("command orbit.ping did not return true")
	}
	if rings := s.eval(t, `()=>AuraDesktopThemeFx.inspect().particles.rings`).Value.Int(); rings != 1 {
		t.Errorf("rings after one ping: %d, want 1", rings)
	}

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
	if got := s.eval(t, `()=>({layers:document.querySelectorAll('.vd-theme-fx-layer').length,scenes:document.querySelectorAll('.vd-fx-orbit-scene').length,mx:document.body.style.getPropertyValue('--vd-orbit-mx'),tilt:document.querySelectorAll('[data-orbit-tilt]').length})`).Value; got.Get("layers").Int() != 0 || got.Get("scenes").Int() != 0 || got.Get("mx").Str() != "" || got.Get("tilt").Int() != 0 {
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
	s.eval(t, `()=>AuraDesktopThemeFx.command('orbit.time',{t:42})`)
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
