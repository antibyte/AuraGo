package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// Review images of the Green Slime gel: 2x-8x crops of the window frame, drips, bubble stages, strand, drops
// and splat, written to the git-ignored reports/desktop-experience-themes/slime/. They need a real GPU and
// take about a minute, so they only run on request:
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 go test ./ui -run '^TestDesktopGreenSlimeCloseUpsBrowser$' -count=1 -timeout 10m
//
// Every file is named after what it shows (gl-corner-tl, gl-bubble-grow-900, gl-strand-b, gl-splat, ...);
// the gl-ref-* files are crops of the wallpaper's own gel beads to compare against. Nothing here asserts
// looks - the coverage contracts live in desktop_theme_green_slime_browser_test.go.

func slimeCloseUpDir() string {
	return filepath.Join("..", "reports", "desktop-experience-themes", "slime")
}

func slimeWriteFile(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(slimeCloseUpDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(slimeCloseUpDir(), name+".png"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// slimeShot writes a clip of the page (CSS px) at the given scale.
func slimeShot(t *testing.T, s *themeSweepSession, name string, x, y, w, h, scale float64) {
	t.Helper()
	data, err := s.page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
		Clip:   &proto.PageViewport{X: x, Y: y, Width: w, Height: h, Scale: scale},
	})
	if err != nil {
		t.Fatalf("close-up %s: %v", name, err)
	}
	slimeWriteFile(t, name, data)
}

// slimeSheet lays the named crops out in a grid with cols columns.
func slimeSheet(t *testing.T, sheet string, cols int, names ...string) {
	t.Helper()
	var images []image.Image
	cellW, cellH := 0, 0
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(slimeCloseUpDir(), name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		images = append(images, img)
		cellW, cellH = max(cellW, img.Bounds().Dx()), max(cellH, img.Bounds().Dy())
	}
	rows := (len(images) + cols - 1) / cols
	out := image.NewRGBA(image.Rect(0, 0, cols*(cellW+6), rows*(cellH+6)))
	for i, img := range images {
		ox, oy := (i%cols)*(cellW+6), (i/cols)*(cellH+6)
		b := img.Bounds()
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				out.Set(ox+x, oy+y, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		t.Fatal(err)
	}
	slimeWriteFile(t, sheet, buf.Bytes())
}

// slimeWallpaperCrop writes a magnified crop of the wallpaper image (image pixels) for side-by-side review.
func slimeWallpaperCrop(t *testing.T, name string, x, y, w, h, factor int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("img", "wallpapers", "themes", "green_slime.png"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	out := image.NewRGBA(image.Rect(0, 0, w*factor, h*factor))
	for yy := 0; yy < h*factor; yy++ {
		for xx := 0; xx < w*factor; xx++ {
			out.Set(xx, yy, src.At(x+xx/factor, y+yy/factor))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		t.Fatal(err)
	}
	slimeWriteFile(t, name, buf.Bytes())
}

// slimeCloseUpJS installs window.slimeShots: freeze()/thaw() stop and restart the effect through the page
// visibility seam (a frozen frame can be screenshotted without it moving), frames(n) waits n animation frames,
// stage() freezes on the first frame in which bubble id is in the wanted stage and progress range, and burst()
// grabs the bursting bubble inside the page, in the same frame it is drawn (a burst lasts about 0.14 s, shorter
// than a screenshot round trip), as a PNG data URL over a plain green card.
const slimeCloseUpJS = `() => {
	window.slimeShots = {
		freeze() { Object.defineProperty(document, 'hidden', { configurable: true, value: true }); document.dispatchEvent(new Event('visibilitychange')); },
		thaw() { delete document.hidden; document.dispatchEvent(new Event('visibilitychange')); },
		frames(n) { return new Promise(res => { let i = 0; const f = () => { if (++i >= n) res(i); else requestAnimationFrame(f); }; requestAnimationFrame(f); }); },
		async stage(id, stage, lo, hi, limit) {
			const start = performance.now();
			for (;;) {
				await new Promise(requestAnimationFrame);
				const sample = AuraDesktopThemeFx.inspect().bubbleSamples.find(b => b.id === id && b.stage === stage && b.t >= lo && b.t <= hi);
				if (sample) { slimeShots.freeze(); return sample; }
				if (performance.now() - start > limit) return null;
			}
		},
		async burst(id, size, scale, limit) {
			const start = performance.now();
			const canvas = document.querySelector('.vd-fx-slime-canvas');
			for (;;) {
				await new Promise(requestAnimationFrame);
				const sample = AuraDesktopThemeFx.inspect().bubbleSamples.find(b => b.id === id && b.stage === 'burst' && b.t >= 0.3 && b.t <= 0.75);
				if (sample) {
					const k = canvas.width / canvas.clientWidth;
					const out = document.createElement('canvas');
					out.width = size * scale; out.height = size * scale;
					const ctx = out.getContext('2d');
					ctx.fillStyle = '#66872a'; ctx.fillRect(0, 0, out.width, out.height);
					ctx.imageSmoothingEnabled = true;
					ctx.drawImage(canvas, (sample.x - size / 2) * k, (sample.y - size / 2) * k, size * k, size * k, 0, 0, out.width, out.height);
					return out.toDataURL('image/png');
				}
				if (performance.now() - start > limit) return '';
			}
		}
	};
}`

func TestDesktopGreenSlimeCloseUpsBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" || os.Getenv("AURAGO_THEME_RECORDINGS") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 and AURAGO_THEME_RECORDINGS=1 to write the Green Slime close-ups")
	}
	s := slimeStage(t)
	s.eval(t, slimeCloseUpJS)
	// The files window overlaps the settings window for the occlusion close-ups.
	s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='files');Object.assign(w.element.style,{left:'780px',top:'330px'});dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	time.Sleep(1500 * time.Millisecond)
	settings := slimeWindowBox(t, s, "settings")
	files := slimeWindowBox(t, s, "files")
	var taskbar slimeBox
	s.evalInto(t, &taskbar, `()=>{const r=document.querySelector('.vd-taskbar').getBoundingClientRect();return {X:r.left,Y:r.top,W:r.width,H:r.height,R:r.right,B:r.bottom};}`)
	t.Log("effect:", s.eval(t, `()=>{const f=AuraDesktopThemeFx.inspect();return JSON.stringify({renderer:f.renderer,gpu:f.gpu,compileMs:f.compileMs,blockedMs:f.blockedMs,averageRenderMs:f.averageRenderMs,quality:f.qualityScale});}`).Value.Str())
	frozen := func(name string, x, y, w, h, scale float64) {
		s.eval(t, `()=>slimeShots.freeze()`)
		slimeShot(t, s, name, x, y, w, h, scale)
		s.eval(t, `()=>slimeShots.thaw()`)
	}

	// Frame, corners, edges, drips and the gel of a front window over a lower one.
	s.eval(t, `()=>slimeShots.frames(6)`)
	frozen("gl-full", 0, 0, 1920, 1080, 1)
	s.eval(t, `()=>slimeShots.freeze()`)
	slimeShot(t, s, "gl-corner-tl", settings.X-45, settings.Y-45, 200, 130, 3)
	slimeShot(t, s, "gl-edge-top", settings.X+300, settings.Y-45, 220, 70, 4)
	slimeShot(t, s, "gl-edge-left", settings.X-45, settings.Y+250, 80, 160, 4)
	slimeShot(t, s, "gl-mid-top", settings.X-70, settings.Y-80, 640, 260, 2)
	slimeShot(t, s, "gl-mid-left", settings.X-70, settings.B-300, 640, 360, 2)
	slimeShot(t, s, "gl-mid-files", files.R-420, files.Y-70, 560, 320, 2)
	slimeShot(t, s, "gl-mid-filesbr", files.R-420, files.B-200, 560, 300, 2)
	slimeShot(t, s, "gl-bottom", settings.X+40, settings.B-20, 420, 140, 3)
	slimeShot(t, s, "gl-bottom-b", settings.X+440, settings.B-20, 360, 140, 3)
	slimeShot(t, s, "gl-overlap", files.X-60, files.Y+40, 200, 200, 3)
	slimeShot(t, s, "gl-overlap-top", files.X-60, files.Y-45, 260, 120, 3)
	s.eval(t, `()=>slimeShots.thaw()`)
	slimeWallpaperCrop(t, "gl-ref-beads-br", 1330, 600, 330, 200, 2)
	slimeWallpaperCrop(t, "gl-ref-beads-bl", 0, 620, 330, 200, 2)
	slimeWallpaperCrop(t, "gl-ref-drop", 200, 20, 220, 150, 3)
	slimeWallpaperCrop(t, "gl-ref-bubbles-r", 1400, 120, 280, 160, 3)

	// One bubble through its life; the stage ranges pick the moments worth a look.
	bx, by := settings.X+settings.W*0.5, settings.B+12
	var sheet []string
	for _, step := range []struct {
		stage  string
		lo, hi float64
	}{{"nucleate", .92, 1}, {"grow", .45, .6}, {"grow", .9, 1}, {"hold", .5, 1}, {"burst", .3, .75}, {"crater", .02, .12}, {"crater", .3, .5}} {
		// An ambient bubble can sit on the spot or fill the window's two slots; ask again a moment later.
		var id int
		for attempt := 0; attempt < 25 && id <= 0; attempt++ {
			if got := s.eval(t, `(x,y)=>AuraDesktopThemeFx.command('bubble',{at:{x,y},base:12})`, bx, by).Value; !got.Nil() {
				id = got.Int()
			} else {
				time.Sleep(400 * time.Millisecond)
			}
		}
		if id <= 0 {
			t.Fatalf("no bubble could be spawned for %s", step.stage)
		}
		name := "gl-bubble-" + step.stage + "-" + strings.TrimSuffix(time.Duration(step.lo*1000).String(), "ns")
		if step.stage == "burst" {
			url := s.eval(t, `(id)=>slimeShots.burst(id,68,8,25000)`, id).Value.Str()
			raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:image/png;base64,"))
			if err != nil || url == "" {
				t.Fatalf("the bursting bubble was not captured: %v", err)
			}
			slimeWriteFile(t, name, raw)
		} else {
			sample := s.eval(t, `(id,stage,lo,hi)=>slimeShots.stage(id,stage,lo,hi,25000)`, id, step.stage, step.lo, step.hi).Value
			if sample.Nil() {
				t.Fatalf("bubble %d never reached %s %.2f-%.2f", id, step.stage, step.lo, step.hi)
			}
			slimeShot(t, s, name, sample.Get("x").Num()-34, sample.Get("y").Num()-34, 68, 68, 8)
			s.eval(t, `()=>slimeShots.thaw()`)
		}
		sheet = append(sheet, name)
		s.waitFor(t, "the bubble to heal", 25*time.Second, `(id)=>!AuraDesktopThemeFx.inspect().bubbleSamples.some(b=>b.id===id)`, id)
	}
	slimeSheet(t, "sheet-bubbles", 3, sheet...)

	// A strand pulled with the real mouse: stretching, long and thin with beads, and the moment after the snap.
	sx, sy := settings.X+300, settings.B+14
	mouse := func(x, y float64) {
		if err := s.page.Mouse.MoveTo(proto.Point{X: x, Y: y}); err != nil {
			t.Fatal(err)
		}
	}
	mouse(sx, sy)
	if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 10; i++ {
		mouse(sx+float64(i)*3, sy+float64(i)*9)
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(350 * time.Millisecond)
	slimeShot(t, s, "gl-strand-a", sx-60, sy-30, 160, 160, 4)
	for i := 11; i <= 14; i++ {
		mouse(sx+float64(i)*3, sy+float64(i)*9)
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	slimeShot(t, s, "gl-strand-b", sx-60, sy-30, 160, 200, 4)
	slimeShot(t, s, "gl-strand-root", sx-30, sy-12, 80, 70, 8)
	slimeShot(t, s, "gl-strand-tip", sx+8, sy+95, 80, 70, 8)
	if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	slimeShot(t, s, "gl-strand-snap", sx-60, sy-30, 160, 220, 4)

	// Falling drops with spray, and a splat at three ages.
	s.eval(t, `(x,y)=>{AuraDesktopThemeFx.command('drop',{x,y,vx:0,vy:60,r:9});AuraDesktopThemeFx.command('drop',{x:x+120,y,vx:40,vy:240,r:5});}`, settings.R+70, settings.Y+60)
	s.eval(t, `()=>slimeShots.frames(5)`)
	frozen("gl-drops", settings.R+10, settings.Y+20, 260, 200, 3)
	s.eval(t, `(x,y)=>AuraDesktopThemeFx.command('drop',{x,y,vx:0,vy:260,r:9})`, 1300.0, taskbar.Y-60)
	s.waitFor(t, "a splat on the taskbar", 10*time.Second, `()=>AuraDesktopThemeFx.inspect().splatSamples.some(p=>Math.abs(p.x-1300)<40)`)
	for i, frames := range []int{2, 12, 60} {
		s.eval(t, `(n)=>slimeShots.frames(n)`, frames)
		frozen("gl-splat"+[]string{"", "-b", "-c"}[i], 1250, taskbar.Y-40, 100, 60, 8)
	}
	slimeSheet(t, "sheet-splat", 3, "gl-splat", "gl-splat-b", "gl-splat-c")

	// The still material (no animation, no bubbles in motion).
	s.store.set(map[string]string{"windows.animations": "false"})
	s.pushSettings(t)
	s.waitFor(t, "static gel", 60*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return !f.motion && f.ready && f.backdrop!=='loading' && !f.rafPending && f.frameCount>0;}`)
	time.Sleep(500 * time.Millisecond)
	slimeShot(t, s, "gl-static-full", 0, 0, 1920, 1080, 1)
	slimeShot(t, s, "gl-static-bottom", settings.X+40, settings.B-40, 600, 140, 2)
	slimeShot(t, s, "gl-static-left", settings.X-45, settings.Y+300, 120, 200, 3)
	if errors := s.eval(t, `()=>fixtureErrors`).Value.String(); errors != "[]" {
		t.Fatalf("errors while shooting close-ups: %s", errors)
	}
}
