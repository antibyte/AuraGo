package ui

import (
	"bytes"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Record short, real pointer-driven theme studies. These are review artifacts, not CI fixtures.
//
//	AURAGO_RUN_BROWSER_SMOKE=1 AURAGO_THEME_RECORDINGS=1 AURAGO_BROWSER_MUTE_AUDIO=1 go test ./ui -run '^TestDesktopThemeMotionRecordingsBrowser$' -count=1 -timeout 10m
func TestDesktopThemeMotionRecordingsBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" || os.Getenv("AURAGO_THEME_RECORDINGS") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 and AURAGO_THEME_RECORDINGS=1 to record desktop theme motion GIFs")
	}

	s := newThemeSweepSessionWithBrowser(t, "no-preference", func(l *launcher.Launcher) {
		l.Delete("disable-gpu").Set("enable-unsafe-swiftshader")
	})
	s.page.MustSetViewport(1920, 1080, 1, false)
	s.openApps(t, "settings")
	s.eval(t, `()=>{
		const settings=[...auditTest.state.windows.values()].find(w=>w.appId==='settings');
		if(!settings)throw new Error('settings window did not open');
		Object.assign(settings.element.style,{left:'180px',top:'105px',width:'1080px',height:'760px'});
		dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));
	}`)

	outputDir := filepath.Join("..", "reports", "desktop-experience-themes", "motion")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create recording output directory: %v", err)
	}
	for _, id := range []string{"electronic_dreams", "windows_xp", "green_slime", "voxel_world"} {
		t.Run(id, func(t *testing.T) {
			icon := "papirus"
			if id == "windows_xp" {
				icon = id
			}
			s.store.set(map[string]string{
				"appearance.icon_theme": icon,
				"appearance.wallpaper":  id, "windows.animations": "true",
			})
			s.pushSettings(t)
			// Select the theme through Settings so its live card re-renders with the active state.
			// pushSettings updates the shell state but intentionally does not re-render app content.
			s.eval(t, `(id)=>{
				const card=document.querySelector('[data-desktop-theme="'+id+'"]');
				if(!card)throw new Error('Settings theme card is missing for '+id);
				card.click();
			}`, id)
			s.waitFor(t, "selected theme and Settings card", 15*time.Second, `(id)=>{
				const card=document.querySelector('[data-desktop-theme="'+id+'"][aria-pressed="true"]');
				return document.body.dataset.theme===id && !!card && card.classList.contains('active');
			}`, id)
			s.waitForStore(t, "theme card save", 15*time.Second, func(store *themeSweepStore) bool {
				return store.value("appearance.theme") == id
			})
			if id != "windows_xp" {
				s.waitFor(t, "theme effect host", 30*time.Second, `(id)=>window.AuraDesktopThemeFx?.activeId()===id`, id)
			}
			if id == "voxel_world" {
				s.waitFor(t, "voxel WebGL and physics", 30*time.Second, `()=>{
					const fx=window.AuraDesktopThemeFx?.inspect();
					return !!fx?.rendererReady && !!fx?.physicsReady;
				}`)
			}
			s.eval(t, `async()=>{await AuraDesktopModules.loadAppAssets('calculator');}`)

			frames := make([]themeMotionRecordingFrame, 0, 32)
			frames = append(frames, captureThemeMotionFrame(t, s))
			s.eval(t, `()=>{auditTest.openApp('calculator');}`)
			s.waitFor(t, "calculator window and content", 10*time.Second, `()=>{
				const win=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');
				return !!win && !!win.element.querySelector('.vd-window-content')?.children.length;
			}`)
			s.eval(t, `()=>{
			const win=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');
			window.motionRecordingCalculator=win;
			Object.assign(win.element.style,{left:'690px',top:'265px'});
			dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));
		}`)
			for i := 0; i < 7; i++ {
				frames = append(frames, captureThemeMotionFrame(t, s))
				if i < 6 {
					time.Sleep(85 * time.Millisecond)
				}
			}
			s.settle(t)
			frames = append(frames, captureThemeMotionFrame(t, s))

			var start struct{ X, Y float64 }
			s.evalInto(t, &start, `()=>{
				const bar=motionRecordingCalculator.element.querySelector('.vd-window-titlebar');
				const r=bar.getBoundingClientRect();
				return {X:r.left+85,Y:r.top+r.height/2};
			}`)
			if start.X <= 0 || start.Y <= 0 || start.X >= 1920 || start.Y >= 1080 {
				t.Fatalf("calculator title bar is outside the recording viewport: %+v", start)
			}
			if err := s.page.Mouse.MoveTo(proto.Point{X: start.X, Y: start.Y}); err != nil {
				t.Fatalf("move pointer onto calculator title bar: %v", err)
			}
			if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
				t.Fatalf("start calculator drag: %v", err)
			}
			frames = append(frames, captureThemeMotionFrame(t, s))
			end := proto.Point{X: math.Min(start.X+300, 1860), Y: math.Min(start.Y+145, 950)}
			for i := 1; i <= 8; i++ {
				point := proto.Point{
					X: start.X + (end.X-start.X)*float64(i)/8,
					Y: start.Y + (end.Y-start.Y)*float64(i)/8,
				}
				if err := s.page.Mouse.MoveTo(point); err != nil {
					t.Fatalf("drag calculator through recording frame %d: %v", i, err)
				}
				frames = append(frames, captureThemeMotionFrame(t, s))
				time.Sleep(85 * time.Millisecond)
			}
			if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
				t.Fatalf("finish calculator drag: %v", err)
			}
			for i := 0; i < 2; i++ {
				frames = append(frames, captureThemeMotionFrame(t, s))
				time.Sleep(85 * time.Millisecond)
			}

			var closePoint struct{ X, Y float64 }
			s.evalInto(t, &closePoint, `()=>{
				const button=motionRecordingCalculator.element.querySelector('.vd-window-button[data-action="close"]');
				const r=button.getBoundingClientRect();
				return {X:r.left+r.width/2,Y:r.top+r.height/2};
			}`)
			if err := s.page.Mouse.MoveTo(proto.Point{X: closePoint.X, Y: closePoint.Y}); err != nil {
				t.Fatalf("move pointer to calculator close button: %v", err)
			}
			if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
				t.Fatalf("press calculator close button: %v", err)
			}
			if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
				t.Fatalf("release calculator close button: %v", err)
			}
			for i := 0; i < 6; i++ {
				frames = append(frames, captureThemeMotionFrame(t, s))
				if i < 5 {
					time.Sleep(85 * time.Millisecond)
				}
			}
			s.waitFor(t, "calculator to close", 5*time.Second, `()=>!auditTest.state.windows.has(motionRecordingCalculator.id)`)
			frames = append(frames, captureThemeMotionFrame(t, s))
			if len(frames) < 24 || len(frames) > 36 {
				t.Fatalf("recording has %d frames; expected 24-36", len(frames))
			}
			path := filepath.Join(outputDir, id+".gif")
			if err := writeThemeMotionGIF(path, frames, 85*time.Millisecond); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}
			t.Logf("wrote %s (%d frames, 960x540)", path, len(frames))
		})
	}
}

type themeMotionRecordingFrame struct {
	image    image.Image
	captured time.Time
}

func captureThemeMotionFrame(t *testing.T, s *themeSweepSession) themeMotionRecordingFrame {
	t.Helper()
	data, err := s.page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
		Clip:   &proto.PageViewport{X: 0, Y: 0, Width: 1920, Height: 1080, Scale: .5},
	})
	if err != nil {
		t.Fatalf("capture 960x540 desktop frame: %v", err)
	}
	frame, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode desktop frame PNG: %v", err)
	}
	if got := frame.Bounds().Size(); got.X != 960 || got.Y != 540 {
		t.Fatalf("capture returned %dx%d; want 960x540", got.X, got.Y)
	}
	return themeMotionRecordingFrame{image: frame, captured: time.Now()}
}

func writeThemeMotionGIF(path string, frames []themeMotionRecordingFrame, finalDelay time.Duration) error {
	animation := &gif.GIF{
		Image:     make([]*image.Paletted, len(frames)),
		Delay:     make([]int, len(frames)),
		Disposal:  make([]byte, len(frames)),
		LoopCount: 0,
	}
	for i, frame := range frames {
		paletted := image.NewPaletted(frame.image.Bounds(), palette.Plan9)
		draw.FloydSteinberg.Draw(paletted, paletted.Bounds(), frame.image, image.Point{})
		animation.Image[i] = paletted
		animation.Disposal[i] = gif.DisposalNone
		delay := finalDelay
		if i+1 < len(frames) {
			delay = frames[i+1].captured.Sub(frame.captured)
		}
		animation.Delay[i] = max(1, int(math.Round(float64(delay)/float64(10*time.Millisecond))))
	}
	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, animation); err != nil {
		return err
	}
	return os.WriteFile(path, encoded.Bytes(), 0o644)
}
