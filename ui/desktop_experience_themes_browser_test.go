package ui

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Exercise the actual close guard and manager pointer path, rather than manufacturing effect events.
func TestDesktopExperienceWindowLifecycleBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.store.set(map[string]string{"appearance.theme": "electronic_dreams", "windows.animations": "false"})
	s.pushSettings(t)
	s.eval(t, `()=>{window.observedThemeEvents=[]; addEventListener('aurago:desktop-window',e=>observedThemeEvents.push({phase:e.detail.phase,id:e.detail.id,rect:e.detail.rect,velocity:e.detail.velocity}));}`)
	s.openApps(t, "calculator")
	if first := s.eval(t, `()=>observedThemeEvents[0]?.phase`).Value.Str(); first != "open" {
		t.Fatalf("window lifecycle must begin with open, got %q", first)
	}
	s.eval(t, `()=>{window.themeWindow=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');themeWindow.beforeClose=async()=>false;}`)
	s.eval(t, `async()=>{await auditTest.closeWindow(themeWindow.id);}`)
	if got := s.eval(t, `()=>({alive:auditTest.state.windows.has(themeWindow.id),closes:observedThemeEvents.filter(e=>e.phase==='close').length})`).Value; !got.Get("alive").Bool() || got.Get("closes").Int() != 0 {
		t.Fatalf("cancelled close emitted destruction: %s", got.String())
	}
	var point struct{ X, Y float64 }
	s.evalInto(t, &point, `()=>{const r=themeWindow.element.querySelector('.vd-window-titlebar').getBoundingClientRect();return {X:r.left+70,Y:r.top+r.height/2};}`)
	if err := s.page.Mouse.MoveTo(proto.Point{X: point.X, Y: point.Y}); err != nil {
		t.Fatal(err)
	}
	if err := s.page.Mouse.Down(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.page.Mouse.MoveTo(proto.Point{X: point.X + 65, Y: point.Y + 40}); err != nil {
		t.Fatal(err)
	}
	s.settle(t)
	if err := s.page.Mouse.Up(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if got := s.eval(t, `()=>{const p=observedThemeEvents.map(e=>e.phase);return ['open','drag-start','drag','drag-end'].every(x=>p.includes(x));}`).Value.Bool(); !got {
		t.Fatalf("manager events missing: %s", s.eval(t, `()=>observedThemeEvents`).Value.String())
	}
	s.eval(t, `async()=>{themeWindow.beforeClose=async()=>true;await auditTest.closeWindow(themeWindow.id);}`)
	if got := s.eval(t, `()=>({alive:auditTest.state.windows.has(themeWindow.id),closes:observedThemeEvents.filter(e=>e.phase==='close').length,remaining:AuraDesktopWindowEffects.snapshot().length})`).Value; got.Get("alive").Bool() || got.Get("closes").Int() != 1 || got.Get("remaining").Int() != 0 {
		t.Fatalf("accepted close lifecycle: %s", got.String())
	}
}

func TestDesktopExperienceThemeAtomicSelectionBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.store.set(map[string]string{"appearance.wallpaper": "forest", "appearance.wallpaper_by_space": `{"1":"midnight","2":"forest"}`, "appearance.theme_cursors": "false", "sound.theme": "wood", "sound.enabled": "false"})
	s.pushSettings(t)
	s.openApps(t, "settings")
	s.eval(t, `()=>{const send=window.fetch;window.themePuts=[];window.rejectTheme=false;window.fetch=async function(url,init){if(String(url).includes('/api/desktop/settings')&&init?.method==='PUT'){const body=JSON.parse(init.body);themePuts.push(body);if(rejectTheme&&body.settings?.['appearance.theme'])return new Response(JSON.stringify({error:'Fixture theme transaction rejected'}),{status:409,headers:{'Content-Type':'application/json'}});}return send.apply(this,arguments);};document.querySelector('[data-desktop-theme="windows_xp"]').click();}`)
	s.waitFor(t, "XP transaction", 10*time.Second, `()=>document.body.dataset.theme==='windows_xp' && !!document.querySelector('.vd-start-xp-links')`)
	if got := s.eval(t, `()=>({puts:themePuts.length,wallpaper:auditTest.settingValue('appearance.wallpaper'),spaces:auditTest.settingValue('appearance.wallpaper_by_space'),cursor:document.body.dataset.cursorTheme,sound:auditTest.settingValue('sound.theme'),enabled:auditTest.settingValue('sound.enabled'),keys:Object.keys(themePuts[0].settings).length})`).Value; got.Get("puts").Int() != 1 || got.Get("keys").Int() != 6 || got.Get("wallpaper").Str() != "windows_xp" || got.Get("spaces").Str() != "{}" || got.Get("cursor").Str() != "windows_xp" || got.Get("sound").Str() != "windows_xp" || got.Get("enabled").Str() != "false" {
		t.Fatalf("theme preset was not applied atomically or changed master mute: %s", got.String())
	}
	if got := s.eval(t, `()=>({theme:themePuts.find(x=>x.settings?.['appearance.theme'])?.settings['appearance.theme'],icon:auditTest.settingValue('appearance.icon_theme'),variant:document.body.dataset.startMenu})`).Value; got.Get("theme").Str() != "windows_xp" || got.Get("icon").Str() != "windows_xp" || got.Get("variant").Str() != "xp" {
		t.Fatalf("non-atomic theme selection: %s", got.String())
	}
	s.eval(t, `()=>{document.getElementById('vd-start-button').click();}`)
	s.waitFor(t, "XP Start menu", 3*time.Second, `()=>!document.getElementById('vd-start-menu').hidden`)
	if !s.eval(t, `()=>{const start=document.getElementById('vd-start-button'),label=start.querySelector('.vd-start-xp-label'),links=document.querySelector('.vd-start-xp-links'),r=start.getBoundingClientRect();return r.height<=48 && r.width>=90 && label.textContent.trim() && getComputedStyle(label).display!=='none' && links.children.length===4;}`).Value.Bool() {
		t.Fatal("XP Start menu dimensions/label/shortcuts missing")
	}
	s.eval(t, `()=>{document.getElementById('vd-start-button').click();}`)
	s.eval(t, `()=>{window.rejectTheme=true;document.querySelector('[data-desktop-theme="electronic_dreams"]').click();}`)
	s.waitFor(t, "rejected transaction notification", 5*time.Second, `()=>document.body.textContent.includes('Fixture theme transaction rejected')`)
	if got := s.eval(t, `()=>[auditTest.settingValue('appearance.theme'),auditTest.settingValue('appearance.icon_theme'),document.body.dataset.theme].join(',')`).Value.Str(); got != "windows_xp,windows_xp,windows_xp" {
		t.Fatalf("failed transaction partially applied: %s", got)
	}
	if !s.eval(t, `()=>['appearance.wallpaper','sound.theme'].every(key=>auditTest.settingValue(key)==='windows_xp') && document.body.dataset.cursorTheme==='windows_xp'`).Value.Bool() {
		t.Fatal("failed theme transaction changed a paired preset setting")
	}
	s.eval(t, `()=>{window.rejectTheme=false;}`)
	for _, id := range []string{"electronic_dreams", "green_slime", "voxel_world", "windows_xp"} {
		s.eval(t, `(id)=>document.querySelector('[data-desktop-theme="'+id+'"]').click()`, id)
		s.waitFor(t, "complete selected preset", 10*time.Second, `(id)=>document.body.dataset.theme===id && document.body.dataset.cursorTheme===id && ['appearance.wallpaper','sound.theme'].every(key=>auditTest.settingValue(key)===id)`, id)
	}
	s.page.MustReload().MustWaitLoad()
	s.waitFor(t, "saved preset after reload", 10*time.Second, `()=>document.body.dataset.theme==='windows_xp' && document.body.dataset.cursorTheme==='windows_xp' && ['appearance.wallpaper','sound.theme'].every(key=>auditTest.settingValue(key)==='windows_xp')`)
	// Later customizations belong to the user until they deliberately select a theme again.
	s.store.set(map[string]string{"appearance.wallpaper": "forest", "appearance.wallpaper_by_space": `{"2":"midnight"}`, "appearance.theme_cursors": "false", "sound.theme": "wood"})
	s.page.MustReload().MustWaitLoad()
	s.waitFor(t, "customized preset after reload", 10*time.Second, `()=>document.body.dataset.theme==='windows_xp' && auditTest.settingValue('appearance.wallpaper')==='forest' && auditTest.settingValue('appearance.wallpaper_by_space')==='{"2":"midnight"}' && auditTest.settingValue('sound.theme')==='wood' && document.body.dataset.cursorTheme!=='windows_xp'`)
}

// Real WebGL rendering and Rapier loading are checked separately from the CPU-only theme sweep.
func TestDesktopExperienceMaterialsBrowser(t *testing.T) {
	s := newThemeSweepSessionWithBrowser(t, "no-preference", func(l *launcher.Launcher) { l.Delete("disable-gpu").Set("enable-unsafe-swiftshader") })
	s.page.MustSetViewport(1920, 1080, 1, false)
	s.openApps(t, "settings", "calculator")
	s.eval(t, `()=>{for(const w of auditTest.state.windows.values()){Object.assign(w.element.style,w.appId==='settings'?{left:'120px',top:'90px',width:'900px',height:'700px'}:{left:'1110px',top:'220px'});}}`)
	dir := filepath.Join("..", "reports", "desktop-experience-themes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"electronic_dreams", "windows_xp", "green_slime", "voxel_world"} {
		t.Run(id, func(t *testing.T) {
			icon := "papirus"
			if id == "windows_xp" {
				icon = id
			}
			s.store.set(map[string]string{"appearance.theme": id, "appearance.icon_theme": icon, "appearance.wallpaper": id, "windows.animations": "true"})
			s.pushSettings(t)
			// Reopen settings so the screenshot's selected card reflects the current theme.
			s.eval(t, `async()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='settings');if(w)await auditTest.closeWindow(w.id);}`)
			s.waitFor(t, "previous settings closed", 5*time.Second, `()=>![...auditTest.state.windows.values()].some(w=>w.appId==='settings')`)
			s.openApps(t, "settings")
			s.waitFor(t, "current settings", 5*time.Second, `()=>!!document.querySelector('.vd-theme-card')`)
			s.eval(t, `()=>{const w=[...auditTest.state.windows.values()].find(w=>w.appId==='settings');Object.assign(w.element.style,{left:'120px',top:'90px',width:'900px',height:'700px'});dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
			s.eval(t, `(id)=>{document.querySelector('[data-desktop-theme="'+id+'"]').scrollIntoView({block:'center'});}`, id)
			if id != "windows_xp" {
				s.waitFor(t, "theme material host", 30*time.Second, `(id)=>window.AuraDesktopThemeFx?.activeId()===id`, id)
			}
			if id == "green_slime" {
				// The WebGL2 gel builds its programs on the driver's threads and loads the wallpaper texture.
				s.waitFor(t, "slime gel renderer", 60*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.ready && f.backdrop!=='loading';}`)
				t.Log("slime renderer:", s.eval(t, `()=>{const f=AuraDesktopThemeFx.inspect();return f.renderer+' backdrop='+f.backdrop+' compileMs='+f.compileMs;}`).Value.String())
			}
			if id == "voxel_world" {
				s.waitFor(t, "real WebGL and Rapier", 30*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.rendererReady && f.physicsReady;}`)
				t.Log("real rendering:", s.eval(t, `()=>AuraDesktopThemeFx.inspect()`).Value.String())
			}
			s.settle(t)
			s.screenshot(t, filepath.Join(dir, id+".png"))
			if got := s.eval(t, `()=>window.fixtureErrors`).Value.String(); got != "[]" {
				t.Fatalf("browser errors: %s", got)
			}
		})
	}
}

// Compare the actual composited pixels with the effect layer hidden: the front
// frame must cross a lower window, while its contents must remain unobscured.
func TestDesktopExperienceEffectOcclusionBrowser(t *testing.T) {
	for _, id := range []string{"electronic_dreams", "green_slime", "voxel_world"} {
		t.Run(id, func(t *testing.T) { checkDesktopThemeOcclusion(t, id, false) })
	}
	t.Run("voxel_without_webgl", func(t *testing.T) { checkDesktopThemeOcclusion(t, "voxel_world", true) })
}

func checkDesktopThemeOcclusion(t *testing.T, id string, fallback bool) {
	t.Helper()
	s := newThemeSweepSessionWithBrowser(t, "reduce", func(l *launcher.Launcher) { l.Delete("disable-gpu").Set("enable-unsafe-swiftshader") })
	s.page.MustSetViewport(1920, 1080, 1, false)
	if fallback {
		s.eval(t, `()=>{const original=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){if(type==='webgl'||type==='webgl2'||type==='experimental-webgl')return null;return original.call(this,type,...args);};}`)
	}
	s.store.set(map[string]string{"appearance.theme": id, "appearance.wallpaper": id, "windows.animations": "false"})
	s.pushSettings(t)
	s.openApps(t, "files", "calculator")
	// Desktop objects are behind windows. Their rectangles must not punch holes
	// through the material frame of a foreground window (including WebGL fallback).
	s.eval(t, `()=>{for(const [name,top] of [['vd-widget',320],['vd-icon',460]]){const el=document.createElement('div');el.className=name;el.style.cssText='position:fixed;left:560px;top:'+top+'px;width:160px;height:120px;background:#604c80;z-index:10;';document.body.appendChild(el);}}`)
	s.eval(t, `()=>{for(const w of auditTest.state.windows.values()){Object.assign(w.element.style,w.appId==='files'?{left:'140px',top:'120px',width:'700px',height:'560px'}:{left:'620px',top:'300px',width:'380px',height:'480px'});}dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));}`)
	s.waitFor(t, "two rendered windows", 30*time.Second, `(id)=>{const f=window.AuraDesktopThemeFx?.inspect();return f?.activeId===id && (f.ready || f.renderer==='fallback') && f.windowCount===2;}`, id)
	if id == "voxel_world" {
		s.waitFor(t, "selected voxel renderer", 30*time.Second, `(fallback)=>{const f=AuraDesktopThemeFx.inspect();return fallback?f.renderer==='fallback':f.rendererReady && f.windowOccluderCount===2;}`, fallback)
	}
	s.settle(t)
	capture := func() image.Image {
		t.Helper()
		data, err := s.page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	visible := capture()
	name := id + "-overlap.png"
	if fallback {
		name = id + "-fallback-overlap.png"
	}
	s.screenshot(t, filepath.Join("..", "reports", "desktop-experience-themes", name))
	s.eval(t, `()=>{document.querySelector('.vd-theme-fx-layer').style.visibility='hidden';}`)
	s.settle(t)
	hidden := capture()
	difference := func(region image.Rectangle) int {
		count := 0
		for y := region.Min.Y; y < region.Max.Y; y++ {
			for x := region.Min.X; x < region.Max.X; x++ {
				r1, g1, b1, a1 := visible.At(x, y).RGBA()
				r2, g2, b2, a2 := hidden.At(x, y).RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					count++
				}
			}
		}
		return count
	}
	if changed := difference(image.Rect(605, 350, 620, 600)); changed < 100 {
		t.Fatalf("front theme frame disappeared over the lower window: only %d effect pixels", changed)
	}
	if changed := difference(image.Rect(650, 370, 960, 650)); changed != 0 {
		t.Fatalf("theme effects painted over front-window contents: %d pixels", changed)
	}
}

func TestDesktopExperienceStaticAndFallbackBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.openApps(t, "calculator")
	// Exercise context creation failure even on machines where software WebGL is available.
	s.eval(t, `()=>{const original=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){if(type==='webgl'||type==='webgl2'||type==='experimental-webgl')return null;return original.call(this,type,...args);};}`)
	for _, id := range []string{"electronic_dreams", "green_slime", "voxel_world"} {
		s.store.set(map[string]string{"appearance.theme": id, "windows.animations": "true"})
		s.pushSettings(t)
		s.waitFor(t, "static materials", 30*time.Second, `(id)=>window.AuraDesktopThemeFx?.activeId()===id`, id)
		if id == "voxel_world" {
			s.waitFor(t, "no-WebGL fallback", 30*time.Second, `()=>AuraDesktopThemeFx.inspect().renderer==='fallback'`)
		}
		s.settle(t)
		if got := s.eval(t, `async()=>{const before=AuraDesktopThemeFx.inspect();await new Promise(r=>setTimeout(r,200));const after=AuraDesktopThemeFx.inspect();return {motion:after.motion,frames:after.frameCount-before.frameCount,windows:after.windowCount,pointer:getComputedStyle(document.querySelector('.vd-theme-fx-layer')).pointerEvents};}`).Value; got.Get("motion").Bool() || got.Get("frames").Int() > 1 || got.Get("windows").Int() != 1 || got.Get("pointer").Str() != "none" {
			t.Fatalf("%s static contract failed: %s", id, got.String())
		}
	}
	// Repeated switching must leave exactly one owner/layer and no stale renderer resurrection.
	for i := 0; i < 3; i++ {
		for _, id := range []string{"voxel_world", "green_slime", "electronic_dreams", "standard"} {
			s.store.set(map[string]string{"appearance.theme": id})
			s.pushSettings(t)
		}
	}
	s.settle(t)
	if got := s.eval(t, `()=>({layers:document.querySelectorAll('.vd-theme-fx-layer').length,active:AuraDesktopThemeFx.activeId(),errors:fixtureErrors})`).Value; got.Get("layers").Int() != 0 || got.Get("active").Str() != "" || got.Get("errors").String() != "[]" {
		t.Fatalf("effects leaked after switching: %s", got.String())
	}
}

func TestDesktopExperienceWindowOperationsBrowser(t *testing.T) {
	s := newThemeSweepSession(t, "reduce")
	s.openApps(t, "calculator")
	s.eval(t, `()=>{window.operationWindow=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');}`)
	for _, id := range []string{"electronic_dreams", "windows_xp", "green_slime", "voxel_world"} {
		t.Run(id, func(t *testing.T) {
			s.store.set(map[string]string{"appearance.theme": id, "windows.animations": "false"})
			s.pushSettings(t)
			s.eval(t, `()=>{auditTest.toggleMaximizeWindow(operationWindow.id);}`)
			if !s.eval(t, `()=>operationWindow.maximized`).Value.Bool() {
				t.Fatal("maximize failed")
			}
			s.eval(t, `()=>{auditTest.toggleMaximizeWindow(operationWindow.id);auditTest.applyWindowSnap(operationWindow.element,'right-half');}`)
			if !s.eval(t, `()=>{const r=operationWindow.element.getBoundingClientRect();return operationWindow.snapped==='right-half' && r.right<=innerWidth+1;}`).Value.Bool() {
				t.Fatal("snap escaped workspace")
			}
			s.eval(t, `()=>{auditTest.minimizeWindow(operationWindow.id);}`)
			s.waitFor(t, "minimize", 3*time.Second, `()=>operationWindow.element.style.display==='none'`)
			s.eval(t, `()=>{auditTest.focusWindow(operationWindow.id);}`)
			s.waitFor(t, "restore", 3*time.Second, `()=>operationWindow.element.getBoundingClientRect().width>0`)
			s.eval(t, `()=>{auditTest.switchSpace('2');}`)
			if s.eval(t, `()=>AuraDesktopWindowEffects.snapshot().length`).Value.Int() != 0 {
				t.Fatal("inactive workspace retained an effect window")
			}
			s.eval(t, `()=>{auditTest.switchSpace('1');window.dialogResult='pending';auditTest.modalDialog({title:'Theme dialog',message:'Readable and interactive',input:true,value:'stable'}).then(v=>dialogResult=v);}`)
			s.waitFor(t, "modal", 3*time.Second, `()=>!!document.querySelector('.vd-modal-input')`)
			if !s.eval(t, `()=>{const el=document.querySelector('.vd-modal-input'),r=el.getBoundingClientRect();return document.elementFromPoint(r.x+r.width/2,r.y+r.height/2)===el && document.activeElement===el;}`).Value.Bool() {
				t.Fatal("effect obscured or intercepted modal input")
			}
			s.eval(t, `()=>{document.querySelector('.vd-modal [data-cancel]').click();}`)
			s.waitFor(t, "modal cancellation", 3*time.Second, `()=>dialogResult!=='pending' && !document.querySelector('.vd-modal')`)
		})
	}
}

func TestDesktopExperienceVoxelPhysicsBrowser(t *testing.T) {
	s := newThemeSweepSessionWithBrowser(t, "no-preference", func(l *launcher.Launcher) { l.Delete("disable-gpu").Set("enable-unsafe-swiftshader") })
	s.page.MustSetViewport(1920, 1080, 1, false)
	s.store.set(map[string]string{"appearance.theme": "voxel_world", "appearance.wallpaper": "voxel_world", "windows.animations": "true"})
	s.pushSettings(t)
	s.waitFor(t, "Rapier and Three", 30*time.Second, `()=>window.AuraDesktopThemeFx?.inspect().physicsReady`)
	s.openApps(t, "settings", "files", "editor", "terminal", "system-info", "calculator")
	s.eval(t, `()=>{let i=0;for(const w of auditTest.state.windows.values()){Object.assign(w.element.style,{left:(80+i*220)+'px',top:(80+i*45)+'px',width:w.appId==='calculator'?'380px':'520px',height:'650px'});i++;}dispatchEvent(new CustomEvent('aurago:desktop-windows-refresh'));window.breakWindow=[...auditTest.state.windows.values()].find(w=>w.appId==='calculator');}`)
	s.waitFor(t, "six geometric frames", 5*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.windowCount===6 && f.frameVoxelCount>400;}`)
	s.eval(t, `()=>{window.closeGeometry=AuraDesktopWindowEffects.snapshot().find(w=>w.id===breakWindow.id).rect;void auditTest.closeWindow(breakWindow.id);}`)
	s.waitFor(t, "actual cube destruction", 3*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.fragmentCount>20 && f.fragmentCount<=256 && f.fragmentSamples.length>0;}`)
	s.eval(t, `()=>{window.firstFragment=AuraDesktopThemeFx.inspect().fragmentSamples[0];}`)
	s.waitFor(t, "integrated motion and rotation", 5*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect(),p=f.fragmentSamples[0];return f.physicsSteps>10 && p && p.age>firstFragment.age && Math.abs(p.y-firstFragment.y)>.05 && JSON.stringify(p.rotation)!==JSON.stringify(firstFragment.rotation);}`)
	s.screenshot(t, filepath.Join("..", "reports", "desktop-experience-themes", "voxel-fracture.png"))
	s.waitFor(t, "Rapier contact-force events", 7*time.Second, `()=>AuraDesktopThemeFx.inspect().physicsCollisionCount>0`)
	t.Log("six-window physics:", s.eval(t, `()=>AuraDesktopThemeFx.inspect()`).Value.String())
	s.waitFor(t, "bounded fragment retirement", 12*time.Second, `()=>AuraDesktopThemeFx.inspect().fragmentCount===0 && !AuraDesktopThemeFx.inspect().rafPending`)
	// Exercise a real visibility event through the host and ensure simulation does not advance.
	s.eval(t, `()=>{Object.defineProperty(document,'hidden',{configurable:true,get:()=>true});document.dispatchEvent(new Event('visibilitychange'));}`)
	if !s.eval(t, `async()=>{const before=AuraDesktopThemeFx.inspect();await new Promise(r=>setTimeout(r,200));const after=AuraDesktopThemeFx.inspect();return after.paused && after.frameCount===before.frameCount && after.physicsSteps===before.physicsSteps;}`).Value.Bool() {
		t.Fatal("hidden document continued rendering/physics")
	}
	s.eval(t, `()=>{delete document.hidden;document.dispatchEvent(new Event('visibilitychange'));}`)
	s.store.set(map[string]string{"appearance.wallpaper": "midnight"})
	s.pushSettings(t)
	s.waitFor(t, "custom wallpaper without terrain", 10*time.Second, `()=>{const f=AuraDesktopThemeFx.inspect();return f.wallpaper==='custom' && f.rendererReady && f.backdropVoxelCount===0;}`)
	if got := s.eval(t, `()=>fixtureErrors`).Value.String(); got != "[]" {
		t.Fatal(got)
	}
}
