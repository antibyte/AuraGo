package ui

import (
	"aurago/internal/desktop"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// desktopSoundBrowserRoles mirrors ROLES in sound-engine.js: loudness target (LUFS, momentary maximum) and maximum length
// in seconds of every shell event.
var desktopSoundBrowserRoles = map[string]struct{ lufs, maxSeconds float64 }{
	"session.start": {-18, 2.8}, "notify.message": {-18, 1.4}, "notify.info": {-19, 1.2}, "notify.success": {-19, 1.2},
	"notify.warning": {-18, 1.2}, "notify.error": {-18, 1.2}, "dialog.open": {-22, 0.6}, "dialog.confirm": {-21, 0.7},
	"dialog.cancel": {-23, 0.5}, "window.open": {-23, 0.6}, "window.close": {-24, 0.5}, "window.maximize": {-24, 0.6},
	"window.minimize": {-25, 0.4}, "window.restore": {-25, 0.4}, "window.snap": {-26, 0.25}, "window.deny": {-23, 0.35},
	"file.drop": {-23, 0.5}, "file.trash": {-23, 0.5}, "file.delete": {-22, 0.6}, "space.switch": {-25, 0.6},
	"menu.open": {-28, 0.2}, "menu.close": {-30, 0.15}, "ui.select": {-31, 0.12}, "ui.toggle.on": {-31, 0.12},
	"ui.toggle.off": {-32, 0.12}, "ui.tick": {-34, 0.06},
}

// desktopSoundBrowserCSP is the production main-UI policy without the external connect origins; it must allow the sound Worker.
const desktopSoundBrowserCSP = "default-src 'self'; script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' blob: ws: wss:; media-src 'self' data: blob: http: https:; worker-src 'self' blob:; object-src 'none'; form-action 'self'; frame-src 'self' http: https:; frame-ancestors 'none'; base-uri 'self';"

func TestDesktopSoundBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)

	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script>
window.fixtureErrors=[];
addEventListener('error', e=>fixtureErrors.push(e.message));
addEventListener('unhandledrejection', e=>fixtureErrors.push(String(e.reason)));
window.t=key=>key;
window.WebSocket=class extends EventTarget { close(){} };
</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/sound-shell.js"></script></body>`, 1)

	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.soundTest={state,openApp,closeWindow,inspect:()=>DesktopSounds.inspect(),renderTheme:(id)=>DesktopSounds.renderTheme(id),renderEvent:(t,e,v)=>DesktopSounds.renderEvent(t,e,v),play:(id)=>desktopSound(id)};` + shell[cut:]

	settings := desktop.DesktopSettingDefaults()
	for key, value := range map[string]string{
		"windows.restore_session": "false",
		"windows.animations":      "false",
		"pet.enabled":             "false",
		"phone_gadget.enabled":    "false",
		"desktop.show_widgets":    "false",
		"appearance.theme":        "standard",
		"sound.enabled":           "false",
	} {
		settings[key] = value
	}

	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", desktopSoundBrowserCSP)
		fmt.Fprint(w, html)
	})
	mux.HandleFunc("/sound-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/desktop/bootstrap":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": true, "builtin_apps": desktop.BuiltinApps(), "installed_apps": []interface{}{},
				"widgets": []interface{}{}, "shortcuts": []interface{}{}, "desktop_files": []interface{}{},
				"workspace": map[string]interface{}{"readonly": false}, "settings": settings,
			})
		case "/api/desktop/settings":
			if r.Method == http.MethodPut {
				var update struct{ Key, Value string }
				if err := json.NewDecoder(r.Body).Decode(&update); err != nil || update.Key == "" {
					http.Error(w, `{"error":"invalid setting"}`, http.StatusBadRequest)
					return
				}
				settings[update.Key] = update.Value
				if windowFn := update.Key; windowFn == "sound.enabled" || strings.HasPrefix(windowFn, "sound.") {
					// client applies via applySoundSettingsChange on save from settings app
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"settings": settings})
		default:
			fmt.Fprint(w, `{"status":"ok","files":[],"pets":[],"settings":{},"enabled":false}`)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(240 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 900, 1, false)

	waitBoot := func() {
		t.Helper()
		page.MustNavigate(server.URL + "/fixture")
		page.MustWaitLoad()
		page.MustWait(`()=>!!(window.soundTest?.state?.bootstrap || window.fixtureErrors?.length)`)
		if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
			t.Fatal(errors)
		}
	}
	waitBoot()

	page.MustEval(`()=>{
        Object.defineProperty(document,'hidden',{configurable:true,get:()=>false});
        Object.defineProperty(document,'visibilityState',{configurable:true,get:()=>'visible'});
    }`)

	if page.MustEval(`()=>soundTest.inspect().enabled`).Bool() {
		t.Fatal("sound should start disabled")
	}
	if page.MustEval(`()=>soundTest.inspect().unlocked`).Bool() {
		t.Fatal("sound context must stay locked before user gesture")
	}

	// Enable sounds and simulate unlock gesture.
	page.MustEval(`async()=>{
        await fetch('/api/desktop/settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({key:'sound.enabled',value:'true'})});
        soundTest.state.bootstrap.settings['sound.enabled']='true';
        applySoundSettingsChange('sound.enabled','true');
        document.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
        await new Promise(r=>setTimeout(r,50));
    }`)
	if !page.MustEval(`()=>soundTest.inspect().unlocked`).Bool() {
		t.Fatal("expected sound unlock after pointer gesture")
	}

	// Every theme renders every event (variant 0) through the real path: the Worker, under the production CSP.
	qualityJSON := page.MustEval(`async events=>{
        const out=[];
        for (const theme of DesktopSounds.inspect().themes) {
            for (const event of events) {
                const r=await soundTest.renderEvent(theme,event,0);
                out.push({theme,event,seconds:r.seconds,lufs:r.lufs,peakDb:r.peakDb,renderer:r.renderer});
            }
        }
        return JSON.stringify(out);
    }`, desktopSoundEvents).Str()

	var quality []struct {
		Theme, Event string
		Seconds      float64
		LUFS         *float64 `json:"lufs"`
		PeakDb       *float64 `json:"peakDb"`
		Renderer     string
	}
	if err := json.Unmarshal([]byte(qualityJSON), &quality); err != nil {
		t.Fatalf("parse render quality: %v\n%s", err, qualityJSON)
	}
	if len(quality) != 16*len(desktopSoundEvents) || len(desktopSoundEvents) != 26 {
		t.Fatalf("expected 16 themes x 26 events, got %d results", len(quality))
	}
	if quality[0].Renderer != "worker" {
		t.Fatalf("first render came from %q, want the Worker (CSP must allow worker-src 'self')", quality[0].Renderer)
	}
	for _, r := range quality {
		role, ok := desktopSoundBrowserRoles[r.Event]
		if !ok {
			t.Fatalf("no role for event %s", r.Event)
		}
		if r.LUFS == nil || r.PeakDb == nil {
			t.Errorf("theme %s event %s has no finite loudness or peak", r.Theme, r.Event)
			continue
		}
		lufs, peak := *r.LUFS, *r.PeakDb
		if peak > -2.9 {
			t.Errorf("theme %s event %s peak too hot (%.2f dBFS)", r.Theme, r.Event, peak)
		}
		if r.Seconds <= 0 || r.Seconds > role.maxSeconds {
			t.Errorf("theme %s event %s lasts %.3f s, want 0..%.2f s", r.Theme, r.Event, r.Seconds, role.maxSeconds)
		}
		// Within 0.6 LU of the target; a sound the peak ceiling limited (peak at or above -3.2 dBFS) may be up to 3.2 LU under it.
		onTarget := lufs >= role.lufs-0.6 && lufs <= role.lufs+0.6
		limited := peak >= -3.2 && lufs >= role.lufs-3.2 && lufs <= role.lufs+0.6
		if !onTarget && !limited {
			t.Errorf("theme %s event %s loudness %.2f LUFS, target %.0f (peak %.2f dBFS)", r.Theme, r.Event, lufs, role.lufs, peak)
		}
	}

	before := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	page.MustEval(`async()=>{
        await desktopSound('window.open');
        await new Promise(r=>setTimeout(r,80));
        await desktopSound('window.close');
        await new Promise(r=>setTimeout(r,80));
    }`)
	after := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	if after <= before {
		t.Fatalf("expected play counter to increase: before=%d after=%d", before, after)
	}

	page.MustEval(`()=>{
        soundTest.state.bootstrap.settings['sound.windows']='false';
        applySoundSettingsChange('sound.windows','false');
    }`)
	beforeCat := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	page.MustEval(`async()=>{ await desktopSound('window.open'); }`)
	afterCat := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	if afterCat != beforeCat {
		t.Fatalf("disabled windows category should not play sounds: before=%d after=%d", beforeCat, afterCat)
	}

	page.MustEval(`async()=>{
        soundTest.state.bootstrap.settings['sound.windows']='true';
        soundTest.state.bootstrap.settings['sound.theme']='water';
        applySoundSettingsChange('sound.windows','true');
        applySoundSettingsChange('sound.theme','water');
        await soundTest.renderTheme('water');
    }`)
	if theme := page.MustEval(`()=>soundTest.inspect().cachedTheme`).Str(); theme != "water" {
		t.Fatalf("expected cached theme water, got %q", theme)
	}

	// Material events are exclusive to the voxel bank and keep the same windows
	// category gate as ordinary shell sounds.
	beforeVoxelFx := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	allowedWhileWater := page.MustEval(`async()=>await DesktopSounds.play('window.impact',{material:'stone',strength:.8,pan:.4})`).Bool()
	if allowedWhileWater || page.MustEval(`()=>soundTest.inspect().plays`).Int() != beforeVoxelFx {
		t.Fatal("voxel impact must be ignored while another effective sound theme is selected")
	}
	page.MustEval(`async()=>{
        soundTest.state.bootstrap.settings['sound.theme']='voxel_world';
        applySoundSettingsChange('sound.theme','voxel_world');
    }`)
	for _, fx := range []struct{ event, material string }{
		{event: "window.fracture", material: "wood"},
		{event: "window.impact", material: "stone"},
		{event: "window.impact", material: "earth"},
	} {
		before := page.MustEval(`()=>soundTest.inspect().plays`).Int()
		ok := page.MustEval(`async payload=>await DesktopSounds.play(payload.event,{material:payload.material,strength:.72,pan:-.35})`, map[string]string{"event": fx.event, "material": fx.material}).Bool()
		if !ok {
			t.Fatalf("%s/%s event was not accepted", fx.event, fx.material)
		}
		page.MustWait(`before=>soundTest.inspect().plays>before`, before)
		if event := page.MustEval(`()=>soundTest.inspect().lastEvent`).Str(); event != fx.event {
			t.Fatalf("voxel FX lastEvent = %q, want %q", event, fx.event)
		}
		page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,240))`)
	}
	// A dense pile must not turn one landing into a wall of full-volume audio.
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1100))`)
	if got := page.MustEval(`async()=>{const before=soundTest.inspect().plays;for(let batch=0;batch<16;batch++){for(let i=0;i<64;i++)DesktopSounds.play('window.impact',{material:'stone',strength:.35,pan:0});await new Promise(r=>setTimeout(r,35));}await new Promise(r=>setTimeout(r,300));const after=soundTest.inspect().plays;await new Promise(r=>setTimeout(r,500));return {plays:after-before,late:soundTest.inspect().plays-after};}`).Map(); got["plays"].Int() < 1 || got["plays"].Int() > 4 || got["late"].Int() != 0 {
		t.Fatalf("voxel burst exceeded playback budget or left a trailing queue: %v", got)
	}
	// A slow first render must never replay a stale impact after the cubes stopped.
	page.MustEval(`async()=>{soundTest.state.bootstrap.settings['sound.windows']='false';applySoundSettingsChange('sound.windows','false');soundTest.state.bootstrap.settings['sound.windows']='true';applySoundSettingsChange('sound.windows','true');window.DesktopSoundsRenderGate=new Promise(r=>setTimeout(r,400));const before=soundTest.inspect().plays;DesktopSounds.play('window.impact',{material:'earth',strength:.7});await new Promise(r=>setTimeout(r,800));window.DesktopSoundsRenderGate=null;if(soundTest.inspect().plays!==before)throw new Error('stale impact played after delayed synthesis');}`)
	page.MustEval(`()=>{
        soundTest.state.bootstrap.settings['sound.windows']='false';
        applySoundSettingsChange('sound.windows','false');
    }`)
	beforeVoxelFx = page.MustEval(`()=>soundTest.inspect().plays`).Int()
	page.MustEval(`async()=>await DesktopSounds.play('window.impact',{material:'earth',strength:1,pan:0})`)
	if page.MustEval(`()=>soundTest.inspect().plays`).Int() != beforeVoxelFx {
		t.Fatal("voxel impact must respect the windows category toggle")
	}
	page.MustEval(`()=>{
        soundTest.state.bootstrap.settings['sound.windows']='true';
        soundTest.state.bootstrap.settings['sound.theme']='water';
        applySoundSettingsChange('sound.windows','true');
        applySoundSettingsChange('sound.theme','water');
    }`)

	// Green Slime FX (a bursting bubble, a parting strand) share the theme FX gate with the voxel sounds: they
	// play only for the Green Slime sound theme, never as a preview, and are rate limited on their own.
	slimePlays := func() int { return page.MustEval(`()=>soundTest.inspect().plays`).Int() }
	beforeSlimeFx := slimePlays()
	if page.MustEval(`async()=>await DesktopSounds.play('slime.pop',{variant:'medium',strength:.8,pan:.2})`).Bool() ||
		page.MustEval(`async()=>await DesktopSounds.play('slime.snap',{variant:'large',strength:1})`).Bool() || slimePlays() != beforeSlimeFx {
		t.Fatal("slime FX must be ignored while another effective sound theme is selected")
	}
	page.MustEval(`()=>{
        soundTest.state.bootstrap.settings['sound.theme']='green_slime';
        applySoundSettingsChange('sound.theme','green_slime');
    }`)
	var rejected map[string]interface{}
	rejectedJSON := page.MustEval(`async()=>{
        const out={}, wait=ms=>new Promise(r=>setTimeout(r,ms));
        const start={requested:DesktopSounds.inspect().themeFx.slime.requested,plays:DesktopSounds.inspect().plays};
        out.preview=await DesktopSounds.play('slime.pop',{preview:true,theme:'green_slime',strength:1});
        out.silent=await DesktopSounds.play('slime.pop',{silent:true,strength:1});
        out.mute=await DesktopSounds.play('slime.snap',{strength:0});
        Object.defineProperty(document,'hidden',{configurable:true,get:()=>true});
        out.hidden=await DesktopSounds.play('slime.pop',{strength:1});
        Object.defineProperty(document,'hidden',{configurable:true,get:()=>false});
        soundTest.state.sessionRestoring=true;
        out.restoring=await DesktopSounds.play('slime.snap',{strength:1});
        soundTest.state.sessionRestoring=false;
        soundTest.state.bootstrap.settings['sound.windows']='false';
        applySoundSettingsChange('sound.windows','false');
        out.category=await DesktopSounds.play('slime.pop',{strength:1});
        soundTest.state.bootstrap.settings['sound.windows']='true';
        applySoundSettingsChange('sound.windows','true');
        await wait(300);
        const now=DesktopSounds.inspect();
        out.requested=now.themeFx.slime.requested-start.requested;
        out.plays=now.plays-start.plays;
        return JSON.stringify(out);
    }`).Str()
	if err := json.Unmarshal([]byte(rejectedJSON), &rejected); err != nil {
		t.Fatalf("parse slime FX rejections: %v\n%s", err, rejectedJSON)
	}
	for _, key := range []string{"preview", "silent", "mute", "hidden", "restoring", "category"} {
		if rejected[key] != false {
			t.Errorf("slime FX must be ignored (%s), got %v", key, rejected[key])
		}
	}
	if rejected["requested"] != float64(0) || rejected["plays"] != float64(0) {
		t.Fatalf("ignored slime FX still queued or played: %s", rejectedJSON)
	}

	// An accepted call renders only its own variant, not the whole bank.
	beforeSlimeFx = slimePlays()
	if !page.MustEval(`async()=>await DesktopSounds.play('slime.pop',{variant:'large',strength:.8,pan:-.3})`).Bool() {
		t.Fatal("slime.pop was not accepted for the Green Slime sound theme")
	}
	page.MustWait(`before=>soundTest.inspect().plays>before`, beforeSlimeFx)
	if event := page.MustEval(`()=>soundTest.inspect().lastEvent`).Str(); event != "slime.pop" {
		t.Fatalf("slime FX lastEvent = %q, want slime.pop", event)
	}
	if rendered := page.MustEval(`()=>soundTest.inspect().themeFx.slime.rendered`).Int(); rendered != 1 {
		t.Fatalf("slime FX rendered %d variants for one pop, want 1", rendered)
	}

	// Rate limits: a pop at most every 350 ms and a snap at most every 900 ms, three slime sounds per 2 s
	// at the most, bursts within 80 ms merged into one. The waits let both gaps and the 2 s window clear.
	var limits struct{ Burst, Stream, Snaps, Limited int }
	limitsJSON := page.MustEval(`async()=>{
        const wait=ms=>new Promise(r=>setTimeout(r,ms)), played=()=>DesktopSounds.inspect().plays;
        await wait(2200);
        let before=played();
        for(let i=0;i<8;i++){DesktopSounds.play('slime.pop',{variant:'small',strength:.7});await wait(25);}
        await wait(400);
        const burst=played()-before;
        await wait(2200);
        before=played();
        for(let i=0;i<14;i++){DesktopSounds.play('slime.pop',{variant:'medium',strength:.7});await wait(120);}
        await wait(400);
        const stream=played()-before;
        await wait(2200);
        before=played();
        for(let i=0;i<6;i++){DesktopSounds.play('slime.snap',{variant:'medium',strength:.7});await wait(100);}
        await wait(400);
        const snaps=played()-before;
        return JSON.stringify({burst,stream,snaps,limited:DesktopSounds.inspect().themeFx.slime.limited});
    }`).Str()
	if err := json.Unmarshal([]byte(limitsJSON), &limits); err != nil {
		t.Fatalf("parse slime FX rate limits: %v\n%s", err, limitsJSON)
	}
	t.Logf("slime FX plays: %s", limitsJSON)
	if limits.Burst != 1 || limits.Stream < 2 || limits.Stream > 3 || limits.Snaps != 1 || limits.Limited < 1 {
		t.Fatalf("slime FX rate limits off (burst of 8 quick pops %d, want 1; 14 pops over 1.7 s %d, want 2-3; 6 quick snaps %d, want 1; limited %d): %s",
			limits.Burst, limits.Stream, limits.Snaps, limits.Limited, limitsJSON)
	}

	// The bank renders every variant cleanly and well below a window sound: the level relative to window.open
	// (with the event gain applied) is 4 to 10 dB down, the pop lasts under 180 ms and the snap under 260 ms.
	var bank struct {
		Open  struct{ LUFS float64 } `json:"open"`
		Gains map[string]float64     `json:"gains"`
		Items []struct {
			Event, Variant string
			LUFS, PeakDb   *float64
			Seconds        float64
		} `json:"items"`
	}
	bankJSON := page.MustEval(`async()=>{
        const fx=DesktopSounds.inspect().themeFx.slime;
        const open=await soundTest.renderEvent('green_slime','window.open',0);
        const out={open:{lufs:open.lufs},gains:fx.gains,items:[]};
        for(const event of Object.keys(fx.gains))for(const variant of ['small','medium','large']){
            const r=await soundTest.renderEvent('green_slime',event,variant);
            out.items.push({event,variant,lufs:r.lufs,peakDb:r.peakDb,seconds:r.seconds});
        }
        return JSON.stringify(out);
    }`).Str()
	if err := json.Unmarshal([]byte(bankJSON), &bank); err != nil {
		t.Fatalf("parse slime FX bank: %v\n%s", err, bankJSON)
	}
	if len(bank.Items) != 6 || bank.Open.LUFS == 0 {
		t.Fatalf("slime FX bank should render 2 events in 3 variants: %s", bankJSON)
	}
	for _, item := range bank.Items {
		name := item.Event + "/" + item.Variant
		gain := bank.Gains[item.Event]
		maxSeconds := 0.18
		if item.Event == "slime.snap" {
			maxSeconds = 0.26
		}
		if item.LUFS == nil || item.PeakDb == nil || gain <= 0 {
			t.Errorf("%s renders NaN or silence (gain %v): %s", name, gain, bankJSON)
			continue
		}
		if *item.PeakDb > -2.9 {
			t.Errorf("%s peak %.2f dBFS, want at most -2.9", name, *item.PeakDb)
		}
		if rel := *item.LUFS + 20*math.Log10(gain) - bank.Open.LUFS; rel < -10 || rel > -4 {
			t.Errorf("%s plays %.1f dB from a window sound, want -10 to -4 dB", name, rel)
		}
		if item.Seconds < 0.03 || item.Seconds > maxSeconds {
			t.Errorf("%s lasts %.0f ms, want 30 ms to %.0f ms", name, item.Seconds*1000, maxSeconds*1000)
		}
	}

	// The Orbit sonar ping shares the theme FX gate: it plays only for the Orbit sound theme (here the slime
	// theme is still active), never as a preview and under the Navigation category, one per 350 ms at the most.
	beforeOrbit := slimePlays()
	if page.MustEval(`async()=>await DesktopSounds.play('orbit.ping',{variant:'medium',strength:.8,pan:.2})`).Bool() || slimePlays() != beforeOrbit {
		t.Fatal("orbit ping must be ignored while another effective sound theme is selected")
	}
	page.MustEval(`()=>{
        soundTest.state.bootstrap.settings['sound.theme']='orbit';
        applySoundSettingsChange('sound.theme','orbit');
    }`)
	var orbitRejected map[string]interface{}
	orbitRejectedJSON := page.MustEval(`async()=>{
        const out={}, wait=ms=>new Promise(r=>setTimeout(r,ms));
        const start={requested:DesktopSounds.inspect().themeFx.orbit.requested,plays:DesktopSounds.inspect().plays};
        out.preview=await DesktopSounds.play('orbit.ping',{preview:true,theme:'orbit',strength:1});
        out.silent=await DesktopSounds.play('orbit.ping',{silent:true,strength:1});
        out.mute=await DesktopSounds.play('orbit.ping',{strength:0});
        soundTest.state.bootstrap.settings['sound.navigation']='false';
        applySoundSettingsChange('sound.navigation','false');
        out.category=await DesktopSounds.play('orbit.ping',{strength:1});
        soundTest.state.bootstrap.settings['sound.navigation']='true';
        applySoundSettingsChange('sound.navigation','true');
        await wait(300);
        const now=DesktopSounds.inspect();
        out.requested=now.themeFx.orbit.requested-start.requested;
        out.plays=now.plays-start.plays;
        return JSON.stringify(out);
    }`).Str()
	if err := json.Unmarshal([]byte(orbitRejectedJSON), &orbitRejected); err != nil {
		t.Fatalf("parse orbit ping rejections: %v\n%s", err, orbitRejectedJSON)
	}
	for _, key := range []string{"preview", "silent", "mute", "category"} {
		if orbitRejected[key] != false {
			t.Errorf("orbit ping must be ignored (%s), got %v", key, orbitRejected[key])
		}
	}
	if orbitRejected["requested"] != float64(0) || orbitRejected["plays"] != float64(0) {
		t.Fatalf("ignored orbit pings still queued or played: %s", orbitRejectedJSON)
	}
	beforeOrbit = slimePlays()
	if !page.MustEval(`async()=>await DesktopSounds.play('orbit.ping',{variant:'large',strength:.8,pan:-.3})`).Bool() {
		t.Fatal("orbit.ping was not accepted for the Orbit sound theme")
	}
	page.MustWait(`before=>soundTest.inspect().plays>before`, beforeOrbit)
	if event := page.MustEval(`()=>soundTest.inspect().lastEvent`).Str(); event != "orbit.ping" {
		t.Fatalf("orbit ping lastEvent = %q, want orbit.ping", event)
	}
	if rendered := page.MustEval(`()=>soundTest.inspect().themeFx.orbit.rendered`).Int(); rendered != 1 {
		t.Fatalf("orbit ping rendered %d variants for one ping, want 1", rendered)
	}
	// Eight quick pings merge into one and a stream of 14 over 1.7 s plays two or three (350 ms gap, three per 2 s).
	var orbitLimits struct{ Burst, Stream, Limited int }
	orbitLimitsJSON := page.MustEval(`async()=>{
        const wait=ms=>new Promise(r=>setTimeout(r,ms)), played=()=>DesktopSounds.inspect().plays;
        await wait(2200);
        let before=played();
        for(let i=0;i<8;i++){DesktopSounds.play('orbit.ping',{variant:'small',strength:.7});await wait(25);}
        await wait(400);
        const burst=played()-before;
        await wait(2200);
        before=played();
        for(let i=0;i<14;i++){DesktopSounds.play('orbit.ping',{variant:'medium',strength:.7});await wait(120);}
        await wait(400);
        const stream=played()-before;
        return JSON.stringify({burst,stream,limited:DesktopSounds.inspect().themeFx.orbit.limited});
    }`).Str()
	if err := json.Unmarshal([]byte(orbitLimitsJSON), &orbitLimits); err != nil {
		t.Fatalf("parse orbit ping rate limits: %v\n%s", err, orbitLimitsJSON)
	}
	t.Logf("orbit ping plays: %s", orbitLimitsJSON)
	if orbitLimits.Burst != 1 || orbitLimits.Stream < 2 || orbitLimits.Stream > 3 || orbitLimits.Limited < 1 {
		t.Fatalf("orbit ping rate limits off (burst of 8 quick pings %d, want 1; 14 pings over 1.7 s %d, want 2-3; limited %d): %s",
			orbitLimits.Burst, orbitLimits.Stream, orbitLimits.Limited, orbitLimitsJSON)
	}
	// Every variant renders cleanly, 4 to 10 dB below a window sound, and rings for 0.35 to 1.0 s.
	var orbitBank struct {
		Open  struct{ LUFS float64 } `json:"open"`
		Gain  float64                `json:"gain"`
		Items []struct {
			Variant      string
			LUFS, PeakDb *float64
			Seconds      float64
		} `json:"items"`
	}
	orbitBankJSON := page.MustEval(`async()=>{
        const fx=DesktopSounds.inspect().themeFx.orbit;
        const open=await soundTest.renderEvent('orbit','window.open',0);
        const out={open:{lufs:open.lufs},gain:fx.gains['orbit.ping'],items:[]};
        for(const variant of ['small','medium','large']){
            const r=await soundTest.renderEvent('orbit','orbit.ping',variant);
            out.items.push({variant,lufs:r.lufs,peakDb:r.peakDb,seconds:r.seconds});
        }
        return JSON.stringify(out);
    }`).Str()
	if err := json.Unmarshal([]byte(orbitBankJSON), &orbitBank); err != nil {
		t.Fatalf("parse orbit ping bank: %v\n%s", err, orbitBankJSON)
	}
	if len(orbitBank.Items) != 3 || orbitBank.Open.LUFS == 0 || orbitBank.Gain <= 0 {
		t.Fatalf("orbit ping bank should render three variants: %s", orbitBankJSON)
	}
	for _, item := range orbitBank.Items {
		if item.LUFS == nil || item.PeakDb == nil {
			t.Errorf("orbit.ping/%s renders NaN or silence: %s", item.Variant, orbitBankJSON)
			continue
		}
		if *item.PeakDb > -2.9 {
			t.Errorf("orbit.ping/%s peak %.2f dBFS, want at most -2.9", item.Variant, *item.PeakDb)
		}
		if rel := *item.LUFS + 20*math.Log10(orbitBank.Gain) - orbitBank.Open.LUFS; rel < -10 || rel > -4 {
			t.Errorf("orbit.ping/%s plays %.1f dB from a window sound, want -10 to -4 dB", item.Variant, rel)
		}
		if item.Seconds < 0.35 || item.Seconds > 1.0 {
			t.Errorf("orbit.ping/%s rings for %.0f ms, want 350 to 1000 ms", item.Variant, item.Seconds*1000)
		}
	}
	page.MustEval(`()=>{
        soundTest.state.bootstrap.settings['sound.theme']='green_slime';
        applySoundSettingsChange('sound.theme','green_slime');
    }`)

	// Sounds off: the context closes and slime FX are ignored until the next gesture unlocks it again.
	if page.MustEval(`async()=>{
        soundTest.state.bootstrap.settings['sound.enabled']='false';
        applySoundSettingsChange('sound.enabled','false');
        const heard=await DesktopSounds.play('slime.pop',{variant:'small',strength:1});
        soundTest.state.bootstrap.settings['sound.enabled']='true';
        applySoundSettingsChange('sound.enabled','true');
        document.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true}));
        await new Promise(r=>setTimeout(r,80));
        return heard;
    }`).Bool() {
		t.Fatal("slime FX must be ignored while desktop sounds are disabled")
	}
	page.MustEval(`()=>{
        soundTest.state.bootstrap.settings['sound.theme']='water';
        applySoundSettingsChange('sound.theme','water');
    }`)

	// Settings UI sound section screenshots in standard + fruity themes.
	page.MustEval(`async()=>{
        await AuraDesktopModules.loadAppAssets('settings');
        soundTest.openApp('settings',{category:'sound'});
    }`)
	if !page.MustEval(`()=>!!document.querySelector('.vd-sound-theme-grid')`).Bool() {
		t.Fatal("settings sound section did not render theme grid")
	}
	artifactDir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")
	if artifactDir == "" {
		artifactDir = filepath.Join("..", "reports", "desktop-sound")
	}
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		page.MustEval(`theme=>{
            document.body.dataset.theme=theme.startsWith('fruity')?'fruity':'standard';
            document.body.dataset.fruityMode=theme.endsWith('dark')?'dark':'light';
        }`, theme)
		page.MustScreenshot(filepath.Join(artifactDir, "settings-sound-"+theme+".png"))
	}

	if !page.MustEval(`()=>!!document.querySelector('[data-sound-preview="wood"]')`).Bool() {
		t.Fatal("settings sound preview control missing")
	}
	activePreviewTheme := page.MustEval(`()=>soundTest.inspect().theme`).Str()
	if activePreviewTheme == "crystal" {
		t.Fatal("inactive-bank preview regression requires crystal to differ from the active sound theme")
	}
	beforePreview := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	page.MustEval(`async()=>{ await previewDesktopSound('crystal'); }`)
	afterPreview := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	if afterPreview-beforePreview != 3 {
		t.Fatalf("previewing inactive crystal bank should play all three preview sounds: before=%d after=%d active=%q", beforePreview, afterPreview, activePreviewTheme)
	}
	if previews := page.MustEval(`()=>soundTest.inspect().previewPlays`).Int(); previews < 1 {
		t.Fatalf("expected previewPlays >= 1, got %d", previews)
	}
	if buttons := page.MustEval(`()=>document.querySelectorAll('[data-sound-category-preview]').length`).Int(); buttons != 6 {
		t.Fatalf("settings sound section has %d category preview buttons, want 6", buttons)
	}
	beforeCategory := page.MustEval(`()=>soundTest.inspect().plays`).Int()
	page.MustEval(`async()=>{ await DesktopSounds.previewCategory('interface','crystal'); }`)
	if got := page.MustEval(`()=>soundTest.inspect().plays`).Int() - beforeCategory; got != 4 {
		t.Fatalf("previewing the interface category should play its 4 sounds, played %d", got)
	}

	// Without a Worker the sounds render on the main thread. Sounds start disabled here, so nothing has created a Worker yet.
	setSoundEnabled := func(value string) {
		mu.Lock()
		settings["sound.enabled"] = value
		mu.Unlock()
	}
	setSoundEnabled("false")
	waitBoot()
	var fallback struct {
		Renderer string
		Finite   bool
	}
	fallbackJSON := page.MustEval(`async()=>{
        window.Worker=class{constructor(){throw new Error('blocked')}};
        const r=await DesktopSounds.renderEvent('wood','window.open',0);
        return JSON.stringify({renderer:r.renderer,finite:Number.isFinite(r.lufs)});
    }`).Str()
	if err := json.Unmarshal([]byte(fallbackJSON), &fallback); err != nil {
		t.Fatalf("parse worker fallback: %v\n%s", err, fallbackJSON)
	}
	if fallback.Renderer != "main" || !fallback.Finite {
		t.Fatalf("a blocked Worker must fall back to main-thread rendering with a finite loudness: %s", fallbackJSON)
	}

	// A Worker that never answers is given up after 4 s, then the sound renders on the main thread.
	waitBoot()
	var silent struct {
		Ms       float64
		Renderer string
	}
	silentJSON := page.MustEval(`async()=>{
        window.Worker=class{postMessage(){} terminate(){}};
        const t0=performance.now();
        const r=await DesktopSounds.renderEvent('wood','menu.open',0);
        return JSON.stringify({ms:performance.now()-t0,renderer:r.renderer});
    }`).Str()
	if err := json.Unmarshal([]byte(silentJSON), &silent); err != nil {
		t.Fatalf("parse silent worker: %v\n%s", err, silentJSON)
	}
	if silent.Renderer != "main" || silent.Ms < 3900 || silent.Ms > 10000 {
		t.Fatalf("a silent Worker must hand over to the main thread after about 4 s: %s", silentJSON)
	}

	// The start sound: once per page load when the browser lets the page play, otherwise skipped for the autoplay policy.
	page.MustEvalOnNewDocument(`Object.defineProperty(document,'hidden',{configurable:true,get:()=>false});Object.defineProperty(document,'visibilityState',{configurable:true,get:()=>'visible'});`)
	setSoundEnabled("true")
	waitBoot()
	startSound := ""
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if startSound = page.MustEval(`()=>DesktopSounds.inspect().startSound`).Str(); startSound != "" {
			break
		}
	}
	if startSound != "played" && startSound != "skipped-autoplay" {
		t.Fatalf("startSound = %q, want played or skipped-autoplay", startSound)
	}
	if startSound == "played" {
		if event := page.MustEval(`()=>DesktopSounds.inspect().lastEvent`).Str(); event != "session.start" {
			t.Fatalf("start sound played but lastEvent = %q, want session.start", event)
		}
	}
	startPlays := page.MustEval(`()=>DesktopSounds.inspect().plays`).Int()
	page.MustEval(`async()=>{ await desktopSound('session.start'); }`)
	if got := page.MustEval(`()=>DesktopSounds.inspect().plays`).Int(); got != startPlays {
		t.Fatalf("the start sound must play at most once per page load: plays %d -> %d", startPlays, got)
	}
	page.MustEval(`async()=>{ document.dispatchEvent(new PointerEvent('pointerdown',{bubbles:true})); await new Promise(r=>setTimeout(r,300)); }`)
	if got := page.MustEval(`()=>DesktopSounds.inspect().plays`).Int(); got != startPlays {
		t.Fatalf("the first click must not replay the start sound: plays %d -> %d", startPlays, got)
	}
	if startSound != "played" {
		if event := page.MustEval(`()=>DesktopSounds.inspect().lastEvent`).Str(); event == "session.start" {
			t.Fatalf("start sound was skipped but lastEvent = session.start")
		}
	}
}
