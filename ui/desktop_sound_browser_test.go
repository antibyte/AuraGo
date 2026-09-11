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
        const themes=['crystal','wood','analog','workshop','water'];
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
	if len(quality.Themes) != 5 {
		t.Fatalf("expected 5 rendered themes, got %d", len(quality.Themes))
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
	page.MustEval(`async()=>{ await previewDesktopSound('crystal'); }`)
	if previews := page.MustEval(`()=>soundTest.inspect().previewPlays`).Int(); previews < 1 {
		t.Fatalf("expected previewPlays >= 1, got %d", previews)
	}
}
