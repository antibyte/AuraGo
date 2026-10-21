package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// newSlimeSession opens the fixture desktop for one Green Slime renderer. "webgl2" runs on software WebGL
// (SwiftShader) and expects the WebGL2 gel; "canvas2d" keeps the default browser and hides WebGL from the
// page, so the effect has to take the Canvas 2D fallback.
func newSlimeSession(t *testing.T, renderer string) *themeSweepSession {
	t.Helper()
	if renderer == "webgl2" {
		s := newThemeSweepSessionWithBrowser(t, "no-preference", func(l *launcher.Launcher) { l.Delete("disable-gpu").Set("enable-unsafe-swiftshader") })
		// Software WebGL (SwiftShader without a GPU behind it) draws the gel at about one frame per second, so
		// the effect leaves it to the Canvas 2D fallback; the WebGL2 contract needs a real GPU.
		gpu := s.eval(t, `()=>{const g=document.createElement('canvas').getContext('webgl2');if(!g)return 'none';const e=g.getExtension('WEBGL_debug_renderer_info');return String(g.getParameter(e?e.UNMASKED_RENDERER_WEBGL:g.RENDERER));}`).Value.Str()
		if regexp.MustCompile(`(?i)swiftshader|llvmpipe|lavapipe|softpipe|software|basic render|^none$`).MatchString(gpu) {
			t.Skipf("the WebGL2 gel needs a GPU; this browser only has %q", gpu)
		}
		t.Logf("WebGL2 on %s", gpu)
		return s
	}
	s := newThemeSweepSession(t, "no-preference")
	s.eval(t, `()=>{const original=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){if(type==='webgl'||type==='webgl2'||type==='experimental-webgl')return null;return original.call(this,type,...args);};}`)
	return s
}

// waitSlimeRenderer waits until the effect runs on the wanted renderer; the WebGL2 gel additionally waits
// until its backdrop (the wallpaper texture) is no longer loading.
func waitSlimeRenderer(t *testing.T, s *themeSweepSession, renderer string) {
	t.Helper()
	s.waitFor(t, "slime renderer "+renderer, 60*time.Second, `(want)=>{const f=AuraDesktopThemeFx?.inspect();return !!f && f.ready && f.renderer===want && (want!=='webgl2'||f.backdrop!=='loading');}`, renderer)
	// The contract tests measure the gel at full resolution, not at whatever the adaptive quality picked.
	s.eval(t, `()=>AuraDesktopThemeFx.command('quality',{scale:1})`)
}

// slimeSamplerJS installs window.slimeRegion(x, y, w, h) in CSS pixels. It copies a region of the effect
// canvas into a scratch 2D canvas and reads that, which works for any renderer behind the canvas (a WebGL
// canvas has no 2D context to call getImageData on). The result carries the raw RGBA data of the region
// plus alpha(x, y) and rgba(x, y) lookups in CSS coordinates; points outside the canvas read as transparent.
// It must not contain backticks (Go raw string).
const slimeSamplerJS = `() => {
	const scratch = document.createElement('canvas');
	const scratchCtx = scratch.getContext('2d', { willReadFrequently: true });
	window.slimeRegion = (x, y, w, h) => {
		const canvas = document.querySelector('.vd-theme-fx-layer canvas');
		const scale = canvas.width / canvas.clientWidth;
		const sx = Math.max(0, Math.floor(x * scale)), sy = Math.max(0, Math.floor(y * scale));
		const sw = Math.max(1, Math.min(canvas.width - sx, Math.ceil(w * scale)));
		const sh = Math.max(1, Math.min(canvas.height - sy, Math.ceil(h * scale)));
		scratch.width = sw;
		scratch.height = sh;
		scratchCtx.clearRect(0, 0, sw, sh);
		scratchCtx.drawImage(canvas, sx, sy, sw, sh, 0, 0, sw, sh);
		const data = scratchCtx.getImageData(0, 0, sw, sh).data;
		const rgba = (px, py) => {
			const cx = Math.floor(px * scale) - sx, cy = Math.floor(py * scale) - sy;
			if (cx < 0 || cy < 0 || cx >= sw || cy >= sh) return [0, 0, 0, 0];
			const i = (cy * sw + cx) * 4;
			return [data[i], data[i + 1], data[i + 2], data[i + 3]];
		};
		return { data, scale, width: sw, height: sh, rgba, alpha: (px, py) => rgba(px, py)[3] };
	};
}`

func TestDesktopGreenSlimeSpringSurfaceBrowser(t *testing.T) {
	s := newSlimeSession(t, "webgl2")
	s.store.set(map[string]string{
		"appearance.theme":   "green_slime",
		"windows.animations": "true",
	})
	s.pushSettings(t)
	s.waitFor(t, "Green Slime effect and motion", 10*time.Second, `()=>AuraDesktopThemeFx?.activeId()==='green_slime' && AuraDesktopThemeFx.inspect().motion`)
	waitSlimeRenderer(t, s, "webgl2")

	s.eval(t, `()=>{
		const record={
			id:'green-slime-surface-probe', phase:'open',
			rect:{x:170,y:155,width:390,height:270}, z:9000,
			visible:true,minimized:false,silent:true,velocity:{x:0,y:0}
		};
		window.greenSlimeProbe=record;
		window.dispatchEvent(new CustomEvent('aurago:desktop-window',{detail:record}));
	}`)
	s.waitFor(t, "32 contour follower nodes", 5*time.Second, `()=>AuraDesktopThemeFx.inspect().surfaceNodeCount>=32`)
	before := s.eval(t, `()=>AuraDesktopThemeFx.inspect().frameCount`).Value.Int()
	s.eval(t, `()=>{
		const record=window.greenSlimeProbe;
		record.phase='drag';
		record.rect={x:290,y:205,width:390,height:270};
		record.velocity={x:620,y:250};
		window.dispatchEvent(new CustomEvent('aurago:desktop-window',{detail:record}));
	}`)
	s.waitFor(t, "inertial frame deformation", 3*time.Second,
		`before=>{const fx=AuraDesktopThemeFx.inspect();return fx.frameCount>before && fx.maxDisplacement>0 && fx.maxDisplacement<=18}`, before)
	s.eval(t, `()=>{
		const record=window.greenSlimeProbe;
		record.phase='drag-end';
		window.dispatchEvent(new CustomEvent('aurago:desktop-window',{detail:record}));
	}`)
	s.waitFor(t, "jelly release settles", 3*time.Second,
		`()=>AuraDesktopThemeFx.inspect().currentMaxDisplacement<8`)
	dropsBefore := s.eval(t, `()=>AuraDesktopThemeFx.inspect().dropCount`).Value.Int()
	s.waitFor(t, "periodic detached slime drop", 6*time.Second,
		`before=>AuraDesktopThemeFx.inspect().dropCount>before`, dropsBefore)
	if count := s.eval(t, `()=>AuraDesktopThemeFx.inspect().activeDrops`).Value.Int(); count < 1 || count > 72 {
		t.Fatalf("detached drop pool escaped its bounds: active=%v", count)
	}

	s.store.set(map[string]string{"windows.animations": "false"})
	s.pushSettings(t)
	s.waitFor(t, "static Green Slime mode", 5*time.Second,
		`()=>!AuraDesktopThemeFx.inspect().motion && !AuraDesktopThemeFx.inspect().rafPending`)
	maskCount := s.eval(t, `()=>AuraDesktopThemeFx.inspect().maskCount`).Value.Int()
	s.eval(t, `()=>{
		const modal=document.createElement('div');
		modal.className='vd-modal-backdrop';
		modal.style.cssText='position:fixed;left:20px;top:20px;width:31px;height:27px;z-index:999999;background:transparent;';
		modal.id='green-slime-mask-probe';
		document.body.appendChild(modal);
	}`)
	s.waitFor(t, "static modal mask refresh", 3*time.Second,
		`before=>AuraDesktopThemeFx.inspect().maskCount===before+1`, maskCount)
	s.eval(t, `()=>document.getElementById('green-slime-mask-probe')?.remove()`)
	s.waitFor(t, "static modal mask removal refresh", 3*time.Second,
		`before=>AuraDesktopThemeFx.inspect().maskCount===before`, maskCount)
}

// Sample the actual painted canvas in the window manager's motion event, before
// the next frame can conceal a one-frame gap. Real pointer input exercises the
// manager's scheduling and fast reversals, unlike synthetic geometry snapshots.
func TestDesktopGreenSlimeAttachedMembraneBrowser(t *testing.T) {
	// The wallpaper decides the WebGL2 material: the green_slime image is refracted (texture backdrop),
	// any other desktop leaves the gel translucent over the DOM; both must keep the same coverage contract.
	for _, c := range []struct{ name, renderer, wallpaper string }{
		{"webgl2", "webgl2", "green_slime"},
		{"webgl2-translucent", "webgl2", "forest"},
		{"canvas2d", "canvas2d", "green_slime"},
	} {
		t.Run(c.name, func(t *testing.T) { checkSlimeAttachedMembrane(t, c.name, c.renderer, c.wallpaper) })
	}
}

func checkSlimeAttachedMembrane(t *testing.T, label, renderer, wallpaper string) {
	t.Helper()
	s := newSlimeSession(t, renderer)
	s.page.MustSetViewport(1920, 1080, 1, false)
	s.store.set(map[string]string{"appearance.theme": "green_slime", "appearance.wallpaper": wallpaper, "windows.animations": "true"})
	s.pushSettings(t)
	s.openApps(t, "settings")
	s.waitFor(t, "slime material", 10*time.Second, `()=>AuraDesktopThemeFx?.inspect().surfaceCount===1`)
	waitSlimeRenderer(t, s, renderer)
	s.eval(t, slimeSamplerJS)
	s.eval(t, `()=>{
		const win=[...auditTest.state.windows.values()][0];
		Object.assign(win.element.style,{left:'400px',top:'180px',width:'780px',height:'560px'});
		window.slimeDragWindow=win;
		dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));
		window.slimeCoverage={samples:0,missing:[],interior:[],positions:[]};
		window.sampleSlimeCoverage=()=>{
			const r=slimeDragWindow.element.getBoundingClientRect();
			// One region read around the window; every probe point lies within 14 px of its edge.
			const region=slimeRegion(r.left-14,r.top-14,r.width+28,r.height+28);
			const alpha=(x,y)=>region.alpha(x,y);
			const points=[];
			for(const t of [.12,.3,.5,.7,.88])for(const d of [2,7]){
				points.push([r.left+r.width*t,r.top-d],[r.left+r.width*t,r.bottom+d],
					[r.left-d,r.top+r.height*t],[r.right+d,r.top+r.height*t]);
			}
			// These are outside the DOM's round corners but inside the gel.
			points.push([r.left+3,r.top+3],[r.right-3,r.top+3],[r.left+3,r.bottom-3],[r.right-3,r.bottom-3]);
			for(const [x,y] of points)if(alpha(x,y)<220)slimeCoverage.missing.push({x,y,alpha:alpha(x,y),frame:slimeCoverage.samples});
			for(const [x,y] of [[r.left+70,r.top+22],[r.left+100,r.top+150],[r.right-70,r.bottom-60]])
				if(alpha(x,y)>0)slimeCoverage.interior.push({x,y});
			slimeCoverage.samples++;
			slimeCoverage.positions.push({x:r.left,y:r.top,w:r.width,h:r.height});
		};
		addEventListener('aurago:desktop-window',e=>{
			if(e.detail.id===win.id && /^(drag|resize)/.test(e.detail.phase))sampleSlimeCoverage();
		});
	}`)
	s.settle(t)
	s.eval(t, `()=>sampleSlimeCoverage()`)
	var start struct{ X, Y float64 }
	s.evalInto(t, &start, `()=>{const r=slimeDragWindow.element.querySelector('.vd-window-titlebar').getBoundingClientRect();return {X:r.left+170,Y:r.top+20};}`)
	if err := s.page.Mouse.MoveTo(proto.Point{X: start.X, Y: start.Y}); err != nil {
		t.Fatal(err)
	}
	if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []proto.Point{{X: 200, Y: 150}, {X: 400, Y: 240}, {X: 160, Y: 70}, {X: -180, Y: 240}, {X: 90, Y: 20}, {X: 300, Y: 150}, {X: -170, Y: 80}, {X: 0, Y: 0}} {
		if err := s.page.Mouse.MoveTo(proto.Point{X: start.X + offset.X, Y: start.Y + offset.Y}); err != nil {
			t.Fatal(err)
		}
		s.settle(t)
	}
	if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	// Resizing reprojects local contour offsets as well; it must not leave the
	// stationary content corner behind while the outer geometry catches up.
	s.evalInto(t, &start, `()=>{const r=slimeDragWindow.element.querySelector('[data-resize="se"]').getBoundingClientRect();return {X:r.left+r.width/2,Y:r.top+r.height/2};}`)
	if err := s.page.Mouse.MoveTo(proto.Point{X: start.X, Y: start.Y}); err != nil {
		t.Fatal(err)
	}
	if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []proto.Point{{X: 130, Y: 70}, {X: -80, Y: -45}, {X: 65, Y: 40}} {
		if err := s.page.Mouse.MoveTo(proto.Point{X: start.X + offset.X, Y: start.Y + offset.Y}); err != nil {
			t.Fatal(err)
		}
		s.settle(t)
	}
	if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	s.screenshot(t, filepath.Join("..", "reports", "desktop-experience-themes", "green-slime-attached-"+label+".png"))
	// Native browser crop for inspecting the reported corner at full pixel detail.
	s.evalInto(t, &start, `()=>{const r=slimeDragWindow.element.getBoundingClientRect();return {X:r.left-35,Y:r.top-35};}`)
	corner, err := s.page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
		Clip:   &proto.PageViewport{X: start.X, Y: start.Y, Width: 360, Height: 235, Scale: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("..", "reports", "desktop-experience-themes", "green-slime-corner-"+label+".png"), corner, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := s.eval(t, `()=>slimeCoverage`).Value; got.Get("samples").Int() < 10 || len(got.Get("missing").Arr()) != 0 || len(got.Get("interior").Arr()) != 0 {
		t.Fatalf("slime membrane detached or painted over content: %s", got.String())
	}
	if !s.eval(t, `()=>new Set(slimeCoverage.positions.map(p=>p.x)).size>4 && new Set(slimeCoverage.positions.map(p=>p.w)).size>1`).Value.Bool() {
		t.Fatal("pointer input did not exercise both drag and resize")
	}
	// Switch to the still material and verify both geometry and a quiet renderer.
	s.store.set(map[string]string{"windows.animations": "false"})
	s.pushSettings(t)
	s.waitFor(t, "static membrane", 30*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return !f.motion && f.surfaceCount===1 && f.ready && f.backdrop!=='loading';}`)
	s.eval(t, `()=>{slimeCoverage.missing=[];slimeCoverage.interior=[];sampleSlimeCoverage();}`)
	if got := s.eval(t, `()=>({missing:slimeCoverage.missing,interior:slimeCoverage.interior,raf:AuraDesktopThemeFx.inspect().rafPending,errors:fixtureErrors})`).Value; got.Get("missing").String() != "[]" || got.Get("interior").String() != "[]" || got.Get("raf").Bool() || got.Get("errors").String() != "[]" {
		t.Fatalf("static rounded membrane failed: %s", got.String())
	}
}

func TestDesktopGreenSlimeSixWindowsBrowser(t *testing.T) {
	s := newSlimeSession(t, "webgl2")
	s.page.MustSetViewport(1920, 1080, 1, false)
	s.store.set(map[string]string{"appearance.theme": "green_slime", "appearance.wallpaper": "green_slime", "windows.animations": "true"})
	s.pushSettings(t)
	s.openApps(t, "files", "calculator", "settings", "calendar", "notes", "editor")
	waitSlimeRenderer(t, s, "webgl2")
	s.eval(t, `()=>{let i=0;for(const w of auditTest.state.windows.values()){
		Object.assign(w.element.style,{left:(110+(i%3)*530)+'px',top:(90+Math.floor(i/3)*400)+'px',width:'490px',height:'350px'});i++;
	}dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	s.waitFor(t, "six bounded slime surfaces", 10*time.Second, `()=>AuraDesktopThemeFx.inspect().surfaceNodeCount===192 && AuraDesktopThemeFx.inspect().dropCount>=6`)
	// Report frame cadence as a local observation, not a target-hardware guarantee.
	got := s.eval(t, `async()=>{
		const times=[];let last=performance.now();
		for(let i=0;i<90;i++){await new Promise(requestAnimationFrame);const now=performance.now();times.push(now-last);last=now;}
		times.sort((a,b)=>a-b);
		const fx=AuraDesktopThemeFx.inspect();
		return {median:times[45],p95:times[85],activeDrops:fx.activeDrops,nodes:fx.surfaceNodeCount,renderer:fx.renderer,compileMs:fx.compileMs,renderMs:fx.averageRenderMs,quality:fx.qualityScale,draws:fx.drawCalls,quads:fx.quadCount};
	}`).Value
	t.Logf("six windows at 1920x1080: %s", got.String())
	if got.Get("activeDrops").Int() > 72 || got.Get("nodes").Int() != 192 {
		t.Fatalf("slime resource budget escaped: %s", got.String())
	}
	s.screenshot(t, filepath.Join("..", "reports", "desktop-experience-themes", "green-slime-six-windows.png"))
	s.eval(t, `()=>{Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));}`)
	if !s.eval(t, `async()=>{const before=AuraDesktopThemeFx.inspect();await new Promise(r=>setTimeout(r,180));const after=AuraDesktopThemeFx.inspect();return after.paused && !after.rafPending && before.frameCount===after.frameCount;}`).Value.Bool() {
		t.Fatal("hidden slime theme kept animating")
	}
	s.eval(t, `()=>{delete document.hidden;document.dispatchEvent(new Event('visibilitychange'));}`)
	s.waitFor(t, "resumed slime", 5*time.Second, `()=>!AuraDesktopThemeFx.inspect().paused && AuraDesktopThemeFx.inspect().rafPending`)
	s.closeAllWindows(t)
	s.waitFor(t, "closed membrane disposal", 5*time.Second, `()=>AuraDesktopThemeFx.inspect().surfaceCount===0`)
	s.store.set(map[string]string{"appearance.theme": "standard"})
	s.pushSettings(t)
	s.waitFor(t, "slime host disposal", 5*time.Second, `()=>!document.querySelector('.vd-fx-slime-canvas') && !document.querySelector('.vd-theme-fx-layer')`)
	if errors := s.eval(t, `()=>fixtureErrors`).Value.String(); errors != "[]" {
		t.Fatalf("slime lifecycle errors: %s", errors)
	}
}
