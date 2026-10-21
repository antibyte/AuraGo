package ui

import (
	"aurago/internal/desktop"
	"encoding/json"
	"fmt"
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
	shell = shell[:cut] + `window.soundTest={state,openApp,closeWindow,inspect:()=>DesktopSounds.inspect(),renderTheme:(id)=>DesktopSounds.renderTheme(id),play:(id)=>desktopSound(id)};` + shell[cut:]

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
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
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
	page := newSmokeBrowser(t).MustPage().Timeout(60 * time.Second)
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

	qualityJSON := page.MustEval(`async()=>{
        const themes=DesktopSounds.inspect().themes;
        const report={themes:{}};
        for (const theme of themes) {
            const cache=await soundTest.renderTheme(theme);
            report.themes[theme]={};
            for (const [eventId, meta] of Object.entries(cache.stats||{})) {
                report.themes[theme][eventId]={peak:meta.peak||0,rms:meta.rms||0,duration:cache.buffers[eventId]?.duration||0};
            }
        }
        return JSON.stringify(report);
    }`).Str()

	var quality struct {
		Themes map[string]map[string]struct {
			Peak     float64 `json:"peak"`
			RMS      float64 `json:"rms"`
			Duration float64 `json:"duration"`
		} `json:"themes"`
	}
	if err := json.Unmarshal([]byte(qualityJSON), &quality); err != nil {
		t.Fatalf("parse render quality: %v\n%s", err, qualityJSON)
	}
	if len(quality.Themes) != 15 {
		t.Fatalf("expected 15 rendered themes, got %d", len(quality.Themes))
	}
	const rmsMin = 0.0005
	const peakMax = 0.891 // -1 dBFS
	const durMax = 1.5
	for theme, events := range quality.Themes {
		if len(events) != len(desktopSoundEvents) {
			t.Fatalf("theme %s rendered %d events, want %d", theme, len(events), len(desktopSoundEvents))
		}
		for event, meta := range events {
			if meta.RMS < rmsMin {
				t.Errorf("theme %s event %s too quiet (rms=%v)", theme, event, meta.RMS)
			}
			if meta.Peak > peakMax+0.02 {
				t.Errorf("theme %s event %s peak too hot (%v)", theme, event, meta.Peak)
			}
			if meta.Duration > durMax {
				t.Errorf("theme %s event %s too long (%v s)", theme, event, meta.Duration)
			}
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
		if cachedTheme := page.MustEval(`()=>soundTest.inspect().cachedTheme`).Str(); cachedTheme != "water" {
			t.Fatalf("material FX should not eagerly render the full voxel sound bank; cached theme = %q", cachedTheme)
		}
		page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,240))`)
	}
	// A dense pile must not turn one landing into a wall of full-volume audio.
	page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1100))`)
	if got := page.MustEval(`async()=>{const before=soundTest.inspect().plays;for(let batch=0;batch<16;batch++){for(let i=0;i<64;i++)DesktopSounds.play('window.impact',{material:'stone',strength:.35,pan:0});await new Promise(r=>setTimeout(r,35));}await new Promise(r=>setTimeout(r,300));const after=soundTest.inspect().plays;await new Promise(r=>setTimeout(r,500));return {plays:after-before,late:soundTest.inspect().plays-after};}`).Map(); got["plays"].Int() < 1 || got["plays"].Int() > 4 || got["late"].Int() != 0 {
		t.Fatalf("voxel burst exceeded playback budget or left a trailing queue: %v", got)
	}
	// A slow first render must never replay a stale impact after the cubes stopped.
	page.MustEval(`async()=>{soundTest.state.bootstrap.settings['sound.windows']='false';applySoundSettingsChange('sound.windows','false');soundTest.state.bootstrap.settings['sound.windows']='true';applySoundSettingsChange('sound.windows','true');const original=DesktopSoundSynth.renderLayers;DesktopSoundSynth.renderLayers=async function(...args){await new Promise(r=>setTimeout(r,400));return original.apply(this,args);};const before=soundTest.inspect().plays;DesktopSounds.play('window.impact',{material:'earth',strength:.7});await new Promise(r=>setTimeout(r,800));DesktopSoundSynth.renderLayers=original;if(soundTest.inspect().plays!==before)throw new Error('stale impact played after delayed synthesis');}`)
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
	if cachedTheme := page.MustEval(`()=>soundTest.inspect().cachedTheme`).Str(); cachedTheme != "water" {
		t.Fatalf("slime FX should not eagerly render the full sound bank; cached theme = %q", cachedTheme)
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

	// The bank renders every variant cleanly and well below a window sound: the peak is about 40 % of it and
	// the 20 ms loudness a quarter to a third, the pop lasts under 180 ms and the snap under 260 ms.
	var bank struct {
		Open    struct{ Peak, Loud float64 } `json:"open"`
		Seconds float64                      `json:"seconds"`
		Gains   map[string]float64           `json:"gains"`
		Items   []struct {
			Event, Variant string
			Finite         bool
			Peak, Loud     float64
			Audible        float64
		} `json:"items"`
	}
	bankJSON := page.MustEval(`async()=>{
        const synth=DesktopSoundSynth, bank=DesktopSoundThemes.green_slime, fx=DesktopSounds.inspect().themeFx.slime;
        const measure=buffer=>{
            const channels=[];for(let c=0;c<buffer.numberOfChannels;c++)channels.push(buffer.getChannelData(c));
            let peak=0,finite=true;
            for(const d of channels)for(let i=0;i<d.length;i++){if(!Number.isFinite(d[i]))finite=false;peak=Math.max(peak,Math.abs(d[i]));}
            const ch=channels[0],rate=buffer.sampleRate;
            let last=0;for(let i=0;i<ch.length;i++)if(Math.abs(ch[i])>peak*0.001)last=i;
            const win=Math.round(rate*0.02);let run=0,loud=0;
            for(let i=0;i<ch.length;i++){run+=ch[i]*ch[i];if(i>=win)run-=ch[i-win]*ch[i-win];if(i>=win-1)loud=Math.max(loud,run/win);}
            return {finite,peak,loud:Math.sqrt(loud),audible:(last+1)/rate};
        };
        const open=measure((await synth.renderLayers(bank.events['window.open'],synth.MAX_SECONDS)).buffer);
        const out={open,seconds:fx.seconds,gains:fx.gains,items:[]};
        for(const event of Object.keys(fx.gains))for(const variant of ['small','medium','large']){
            const m=measure((await synth.renderLayers(bank.specialFx[event][variant],fx.seconds)).buffer);
            out.items.push(Object.assign({event,variant},m));
        }
        return JSON.stringify(out);
    }`).Str()
	if err := json.Unmarshal([]byte(bankJSON), &bank); err != nil {
		t.Fatalf("parse slime FX bank: %v\n%s", err, bankJSON)
	}
	if len(bank.Items) != 6 || bank.Open.Peak <= 0 || bank.Open.Loud <= 0 {
		t.Fatalf("slime FX bank should render 2 events in 3 variants: %s", bankJSON)
	}
	for _, item := range bank.Items {
		name := item.Event + "/" + item.Variant
		gain := bank.Gains[item.Event]
		maxAudible := 0.18
		if item.Event == "slime.snap" {
			maxAudible = 0.26
		}
		if !item.Finite || item.Peak <= 0 {
			t.Errorf("%s renders NaN or silence", name)
		}
		if peak := item.Peak * gain / bank.Open.Peak; peak < 0.3 || peak > 0.5 {
			t.Errorf("%s plays at %.2f of a window sound's peak, want about 0.4", name, peak)
		}
		if loud := item.Loud * gain / bank.Open.Loud; loud < 0.1 || loud > 0.45 {
			t.Errorf("%s plays at %.2f of a window sound's loudness, want a quarter to a third", name, loud)
		}
		if item.Audible < 0.03 || item.Audible > maxAudible || item.Audible > bank.Seconds-0.03 {
			t.Errorf("%s is audible for %.0f ms (render length %.0f ms), want 30 ms to %.0f ms and no truncation", name, item.Audible*1000, bank.Seconds*1000, maxAudible*1000)
		}
	}

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
}
