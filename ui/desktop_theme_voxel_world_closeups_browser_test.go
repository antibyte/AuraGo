package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Review images of the Voxel World diorama: the day cycle, a shell-free poster frame, close-ups of the
// waterfall, village and windmill, the pixel-art window frames and a fracture. They are written to the
// git-ignored reports/desktop-experience-themes/voxel/ and need a real GPU (the diorama stays on the poster
// wallpaper on software WebGL, and the test is skipped then), so they only run on request:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 go test ./ui -run '^TestDesktopVoxelWorldCloseUpsBrowser$' -count=1 -timeout 10m
//
// AURAGO_VOXEL_SOFTWARE_GL=1 additionally lets the diorama draw on software WebGL (slow, for a look only).
// Nothing here asserts looks; the scene contracts live in desktop_experience_themes_browser_test.go.

func voxelCloseUpDir() string {
	return filepath.Join("..", "reports", "desktop-experience-themes", "voxel")
}

// voxelShot writes a PNG of the page, or of a clip of it (CSS px, scaled by scale).
func voxelShot(t *testing.T, s *themeSweepSession, name string, clip *proto.PageViewport) {
	t.Helper()
	time.Sleep(150 * time.Millisecond) // let the compositor present the newest frame
	data, err := s.page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng, Clip: clip})
	if err != nil {
		t.Fatalf("shot %s: %v", name, err)
	}
	if err := os.MkdirAll(voxelCloseUpDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(voxelCloseUpDir(), name+".png"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// voxelClipAround returns a w x h clip centred on (cx, cy), moved to stay inside the 1920x1080 viewport.
func voxelClipAround(cx, cy, w, h, scale float64) *proto.PageViewport {
	x := min(max(cx-w/2, 0), 1920-w)
	y := min(max(cy-h/2, 0), 1080-h)
	return &proto.PageViewport{X: x, Y: y, Width: w, Height: h, Scale: scale}
}

func TestDesktopVoxelWorldCloseUpsBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" || os.Getenv("AURAGO_THEME_RECORDINGS") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 and AURAGO_THEME_RECORDINGS=1 to write the Voxel World close-ups")
	}
	s := newThemeSweepSessionWithBrowser(t, "no-preference", func(l *launcher.Launcher) { l.Delete("disable-gpu").Set("enable-unsafe-swiftshader") })
	s.page.MustSetViewport(1920, 1080, 1, false)
	if os.Getenv("AURAGO_VOXEL_SOFTWARE_GL") == "1" {
		s.eval(t, `()=>{window.__auraVoxelSoftwareGL=true;}`)
	}
	s.store.set(map[string]string{"appearance.theme": "voxel_world", "appearance.wallpaper": "voxel_world", "windows.animations": "true"})
	s.pushSettings(t)
	s.waitFor(t, "diorama scene", 60*time.Second, `()=>{const s=AuraDesktopThemeFx.inspect().scene;return !!s && ((s.ready && s.frames>3) || s.backend==='software' || s.backend==='error');}`)
	if backend := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.backend`).Value.Str(); backend != "three-webgl" {
		t.Skipf("the diorama needs a GPU; its backend here is %q: %s", backend, s.eval(t, `()=>JSON.stringify(AuraDesktopThemeFx.inspect().scene)`).Value.Str())
	}
	// Headless timing must not degrade the shots: lock the best quality level.
	s.eval(t, `()=>AuraDesktopThemeFx.command('voxel.quality',{level:0})`)

	// setTime pins the hour and the clock, then waits for three fresh frames and the fade-in.
	setTime := func(hour, freeze float64) {
		t.Helper()
		before := s.eval(t, `()=>AuraDesktopThemeFx.inspect().scene.frames`).Value.Int()
		s.eval(t, `(h,f)=>{AuraDesktopThemeFx.command('voxel.time',{hour:h});AuraDesktopThemeFx.command('voxel.freeze',{t:f});}`, hour, freeze)
		s.waitFor(t, "three new frames", 30*time.Second, `(n)=>{const s=AuraDesktopThemeFx.inspect().scene;return s.frames>=n+3 && s.quality===0 && getComputedStyle(document.querySelector('.vd-fx-voxel-scene')).opacity==='1';}`, before)
	}
	anchor := func(name string) (float64, float64) {
		t.Helper()
		v := s.eval(t, `(n)=>{const a=AuraDesktopThemeFx.inspect().scene.anchors[n];return {x:a[0],y:a[1]};}`, name).Value
		return v.Get("x").Num(), v.Get("y").Num()
	}

	for _, shot := range []struct {
		name string
		hour float64
	}{{"dawn", 6.3}, {"day", 11}, {"golden", 18.8}, {"dusk", 19.9}, {"night", 22.5}} {
		setTime(shot.hour, 12)
		voxelShot(t, s, shot.name, nil)
	}

	// The poster: the same frame without the shell.
	s.eval(t, `()=>{const style=document.createElement('style');style.id='voxel-poster-style';style.textContent='.vd-taskbar,.vd-icon,.vd-widget,.vd-dock,.vd-global-menu,.vd-start-menu{visibility:hidden!important}';document.head.appendChild(style);}`)
	setTime(11, 12)
	voxelShot(t, s, "poster", nil)

	for _, shot := range []struct {
		name, anchor string
		hour         float64
	}{{"closeup-waterfall", "waterfall", 11}, {"closeup-waterfall-rainbow", "waterfall", 17}, {"closeup-village", "village", 11}, {"closeup-windmill", "windmill", 11}, {"closeup-village-night", "village", 22.5}} {
		setTime(shot.hour, 12)
		x, y := anchor(shot.anchor)
		voxelShot(t, s, shot.name, voxelClipAround(x, y, 640, 360, 2))
	}
	s.eval(t, `()=>{document.getElementById('voxel-poster-style').remove();}`)

	// Pixel-art block frames over the isle.
	s.openApps(t, "files", "calculator")
	s.eval(t, `()=>{for(const w of auditTest.state.windows.values()){Object.assign(w.element.style,w.appId==='files'?{left:'140px',top:'120px',width:'700px',height:'560px'}:{left:'620px',top:'300px',width:'380px',height:'480px'});}dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	s.waitFor(t, "two framed windows", 15*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.windowCount===2 && f.frameVoxelCount>100;}`)
	setTime(11, 12)
	time.Sleep(900 * time.Millisecond) // the frames finish assembling
	voxelShot(t, s, "windows-day", nil)
	voxelShot(t, s, "frame-closeup", voxelClipAround(620, 300, 480, 320, 2))

	// Closing the calculator breaks its frame into fragments.
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');void auditTest.closeWindow(w.id);}`)
	time.Sleep(650 * time.Millisecond)
	voxelShot(t, s, "fracture", nil)

	t.Log("scene:", s.eval(t, `()=>JSON.stringify(AuraDesktopThemeFx.inspect().scene)`).Value.Str())
	t.Log("inspect:", s.eval(t, `()=>JSON.stringify(AuraDesktopThemeFx.inspect())`).Value.Str())
	if errors := s.eval(t, `()=>fixtureErrors`).Value.String(); errors != "[]" {
		t.Fatalf("errors while shooting close-ups: %s", errors)
	}
}
