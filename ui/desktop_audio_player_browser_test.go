package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/launcher"
)

// The music player must open as a normal desktop window with a live
// visualizer, not as a fullscreen overlay.
func TestDesktopAudioPlayerWindowBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/audio-player-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("desktop startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,closeContextMenu,openApp,closeWindow,focusWindow};})();`

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/audio-player-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(desktop.BuiltinApps()) })
	server := httptest.NewServer(mux)
	defer server.Close()

	// A synthetic click is not a user gesture, so Chrome would block playback.
	browser := newSmokeBrowserWith(t, func(l *launcher.Launcher) {
		l.Set("autoplay-policy", "no-user-gesture-required")
	})
	page := browser.MustPage(server.URL + "/fixture").Timeout(40 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>{await fixtureReady;await AuraDesktopModules.loadAppAssets('audio-player');}`)

	for _, viewport := range [][2]int{{1366, 900}, {390, 667}} {
		page.MustSetViewport(viewport[0], viewport[1], 1, false)
		page.MustEval(`async()=>{
            if(innerWidth>800&&!window.mediaSessionProbe){
                window.mediaSessionProbe=[];
                window.mediaSessionUnsubscribe=AuraDesktopMediaSession.subscribe(snapshot=>mediaSessionProbe.push(snapshot));
                window.mediaFallbackOwner={};
                AuraDesktopMediaSession.claim(mediaFallbackOwner,{priority:10,playbackState:'paused',metadata:{title:'Fallback player'},handlers:{play:()=>{}}});
            }
            fixtureTheme('standard');aurora.openApp('audio-player',{path:'Music/galaga.mp3'});await new Promise(r=>setTimeout(r,600));
        }`)

		if issues := page.MustEval(`()=>{
			const issues=[];
			const player=document.querySelector('.vd-player');
			if(!player)return 'player window missing; errors='+JSON.stringify(window.fixtureErrors||[]);
			if(document.querySelector('.vd-lightbox'))issues.push('audio must not open in the fullscreen lightbox');
			const win=player.closest('.vd-window');
			if(!win)issues.push('player is not inside a desktop window');
			if(win&&win.classList.contains('maximized'))issues.push('player window must not be maximized');
			const items=[...document.querySelectorAll('.vd-player-queue-item')];
			if(items.length!==1)issues.push('playlist must hold the folder tracks, got '+items.length);
			const title=(document.querySelector('.vd-player-title')||{}).textContent||'';
			if(!title.trim())issues.push('title must show the track, got '+title);
			const queueTitle=(document.querySelector('.vd-player-queue-title')||{}).textContent||'';
			if(!queueTitle.trim())issues.push('playlist entry must show the track');
			const canvas=document.querySelector('.vd-player-canvas');
			if(!canvas)issues.push('visualizer canvas missing');
			else if(canvas.width<10||canvas.height<10)issues.push('visualizer canvas is not sized: '+canvas.width+'x'+canvas.height);
			for(const sel of ['[data-play]','[data-next]','[data-prev]','[data-seek]','[data-volume]','[data-visualizer-mode]','[data-shuffle]','[data-repeat]','[data-eq-toggle]']){
				if(!document.querySelector('.vd-player '+sel))issues.push('control missing: '+sel);
			}
			const pills=document.querySelectorAll('.vd-player [data-viz-pill]');
			if(pills.length!==5)issues.push('expected 5 visualizer pills, got '+pills.length);
			const overlaps=(a,b)=>a&&b&&a.left<b.right&&a.right>b.left&&a.top<b.bottom&&a.bottom>b.top;
			const volume=document.querySelector('.vd-player [data-volume]')?.getBoundingClientRect();
			const speed=document.querySelector('.vd-player [data-speed]')?.getBoundingClientRect();
			const eqBtn=document.querySelector('.vd-player [data-eq-toggle]')?.getBoundingClientRect();
			const modeBtn=document.querySelector('.vd-player [data-visualizer-mode]')?.getBoundingClientRect();
			if(overlaps(volume,speed))issues.push('volume slider overlaps playback speed control');
			if(overlaps(speed,modeBtn))issues.push('speed overlaps visualizer mode button');
			if(overlaps(eqBtn,document.querySelector('.vd-player [data-prev]')?.getBoundingClientRect()))issues.push('eq toggle overlaps transport controls');
			const root=player.getBoundingClientRect();
			if(root.width<200||root.height<200)issues.push('player is too small: '+root.width+'x'+root.height);
			if(player.scrollWidth>player.clientWidth+2)issues.push('horizontal overflow: '+player.scrollWidth+' > '+player.clientWidth);
			return issues.join('\n');
		}`).Str(); issues != "" {
			t.Fatalf("audio player at %dx%d: %s", viewport[0], viewport[1], issues)
		}

		// The spectrum must actually paint in every single one of the 5 renderers.
		painted := page.MustEval(`async()=>{
			const canvas=document.querySelector('.vd-player-canvas');
			await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
			const ctx=canvas.getContext('2d');
			const data=ctx.getImageData(0,0,canvas.width,canvas.height).data;
			let painted=0;
			for(let i=3;i<data.length;i+=4){if(data[i]>0)painted++;}
			return painted;
		}`).Int()
		if painted <= 0 {
			t.Fatalf("visualizer painted nothing at %dx%d", viewport[0], viewport[1])
		}

		// Playback must start with the opened track.
		if !page.MustEval(`()=>{const a=document.querySelector('.vd-player audio');return !!a&&!a.paused&&a.currentTime>0;}`).Bool() {
			diag := page.MustEval(`()=>{const a=document.querySelector('.vd-player audio');if(!a)return 'no audio element';return JSON.stringify({src:a.src,paused:a.paused,currentTime:a.currentTime,readyState:a.readyState,error:a.error?a.error.code:0});}`).Str()
			t.Fatalf("audio did not start playing at %dx%d: %s", viewport[0], viewport[1], diag)
		}
		if !page.MustEval(`()=>!!document.querySelector('.vd-player.is-playing')`).Bool() {
			t.Fatalf("player did not switch into the playing state at %dx%d", viewport[0], viewport[1])
		}
		if viewport[0] > 800 && !page.MustEval(`()=>{
            const snapshot=AuraDesktopMediaSession.snapshot();
            return snapshot.available&&snapshot.status==='playing'&&snapshot.metadata.title
                &&['play','pause','previoustrack','nexttrack'].every(action=>snapshot.actions.includes(action))
                &&mediaSessionProbe.some(item=>item.status==='playing');
        }`).Bool() {
			t.Fatal("the active player must publish title, transport actions and live playback status")
		}

		// The play button pauses and resumes again.
		page.MustEval(`()=>document.querySelector('.vd-player [data-play]').click()`)
		if !page.MustEval(`()=>{const a=document.querySelector('.vd-player audio');return !!a&&a.paused;}`).Bool() {
			t.Fatalf("play button must pause at %dx%d", viewport[0], viewport[1])
		}
		page.MustEval(`async()=>{document.querySelector('.vd-player [data-play]').click();await new Promise(r=>setTimeout(r,300));}`)
		if !page.MustEval(`()=>{const a=document.querySelector('.vd-player audio');return !!a&&!a.paused&&a.currentTime>0;}`).Bool() {
			t.Fatalf("play button must resume at %dx%d", viewport[0], viewport[1])
		}
		if viewport[0] > 800 {
			page.MustEval(`async()=>{
                const audio=document.querySelector('.vd-player audio');
                audio.dispatchEvent(new Event('waiting'));
                if(AuraDesktopMediaSession.snapshot().status!=='buffering')throw Error('waiting must publish buffering');
                audio.dispatchEvent(new Event('playing'));
                if(AuraDesktopMediaSession.snapshot().status!=='playing')throw Error('playing must clear buffering');
                await AuraDesktopMediaSession.dispatch('pause');
                if(!audio.paused||AuraDesktopMediaSession.snapshot().status!=='paused')throw Error('dispatch pause state');
                if(!await AuraDesktopMediaSession.dispatch('play'))throw Error('dispatch play unsupported');
                await new Promise(resolve=>setTimeout(resolve,250));
                await AuraDesktopMediaSession.dispatch('pause');
                audio.dispatchEvent(new Event('ended'));
                if(AuraDesktopMediaSession.snapshot().status!=='ended')throw Error('ended state missing');
                await AuraDesktopMediaSession.dispatch('play');
                await new Promise(resolve=>setTimeout(resolve,250));
                audio.dispatchEvent(new Event('error'));
                if(AuraDesktopMediaSession.snapshot().status!=='error')throw Error('error state missing');
            }`)
		}

		// The visualizer mode cycles through all 5 distinct renderers and every mode paints non-empty pixels.
		seenModes := map[string]bool{}
		for i := 0; i < 5; i++ {
			modeResult := page.MustEval(`async()=>{
				const label=document.querySelector('.vd-player [data-visualizer-mode]').textContent.trim();
				await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
				const canvas=document.querySelector('.vd-player-canvas');
				const ctx=canvas.getContext('2d');
				const data=ctx.getImageData(0,0,canvas.width,canvas.height).data;
				let painted=0;
				for(let p=3;p<data.length;p+=4){if(data[p]>0)painted++;}
				document.querySelector('.vd-player [data-visualizer-mode]').click();
				await new Promise(r=>setTimeout(r,90));
				return {label,painted};
			}`)
			lbl := modeResult.Get("label").Str()
			if lbl == "" {
				t.Fatalf("empty visualizer label at step %d", i)
			}
			if modeResult.Get("painted").Int() <= 0 {
				t.Fatalf("visualizer mode %q painted zero pixels at %dx%d", lbl, viewport[0], viewport[1])
			}
			seenModes[lbl] = true
		}
		if len(seenModes) != 5 {
			t.Fatalf("visualizer mode must cycle through 5 distinct renderers, got %v", seenModes)
		}

		// Desktop E2E verification for visualizer pills & 8-band EQ presets/profiles.
		if viewport[0] > 800 {
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for _, m := range []string{"bars", "radial", "wave", "nebula", "horizon"} {
					page.MustEval(fmt.Sprintf(`async()=>{document.querySelector('.vd-player [data-viz-pill=%q]').click();await new Promise(r=>setTimeout(r,260));}`, m))
					page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("audio-player-viz-%s.png", m)))
				}
			}
			page.MustEval(`async()=>{
				localStorage.removeItem('aurago.audio-player.eq.v1');
				document.querySelector('.vd-player [data-viz-pill="nebula"]').click();
				await new Promise(r=>setTimeout(r,80));
				if(!document.querySelector('.vd-player [data-viz-pill="nebula"]').classList.contains('is-active'))throw Error('nebula pill not activated');
				document.querySelector('.vd-player [data-eq-toggle]').click();
				const panel=document.querySelector('.vd-player [data-eq-panel]');
				if(!panel||panel.hidden)throw Error('EQ panel did not open');
				const sliders=[...document.querySelectorAll('.vd-player [data-eq-band]')];
				if(sliders.length!==8)throw Error('expected 8 EQ sliders, got '+sliders.length);
				if(panel.scrollWidth>panel.clientWidth+2)throw Error('EQ panel overflows horizontally');
				const flatPath=document.querySelector('.vd-player [data-eq-curve-line]')?.getAttribute('d')||'';
				if(!flatPath)throw Error('EQ SVG curve missing');
				const sel=document.querySelector('.vd-player [data-eq-select]');
				sel.value='preset:bass_boost';
				sel.dispatchEvent(new Event('change',{bubbles:true}));
				if(sliders[0].value!=='6.5')throw Error('bass_boost did not set 60Hz to 6.5, got '+sliders[0].value);
				const boostPath=document.querySelector('.vd-player [data-eq-curve-line]')?.getAttribute('d')||'';
				if(boostPath===flatPath)throw Error('EQ SVG curve did not update for bass_boost');
				sliders[3].value='4.5';
				sliders[3].dispatchEvent(new Event('input',{bubbles:true}));
				if(document.querySelector('.vd-player [data-eq-db="3"]').textContent.trim()!=='+4.5')throw Error('band 3 db readout did not update');
				const input=document.querySelector('.vd-player [data-eq-profile-name]');
				input.value='Studio Night';
				document.querySelector('.vd-player [data-eq-save]').click();
				if(![...sel.options].some(o=>o.text==='Studio Night'))throw Error('saved profile missing from dropdown');
			}`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				page.MustScreenshot(filepath.Join(dir, "audio-player-eq-open-dark.png"))
				page.MustEval(`()=>{fixtureTheme('fruity-light');}`)
				page.MustScreenshot(filepath.Join(dir, "audio-player-eq-open-light.png"))
				page.MustEval(`()=>{fixtureTheme('standard');}`)
			}
			// Reopen window and assert custom profile persisted.
			page.MustEval(`async()=>{
				[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id));
				await new Promise(r=>setTimeout(r,80));
				aurora.openApp('audio-player',{path:'Music/galaga.mp3'});
				await new Promise(r=>setTimeout(r,400));
				document.querySelector('.vd-player [data-eq-toggle]').click();
				const sliders=[...document.querySelectorAll('.vd-player [data-eq-band]')];
				if(sliders[0].value!=='6.5'||sliders[3].value!=='4.5')throw Error('custom EQ profile did not persist across reopen: '+sliders[0].value+'/'+sliders[3].value);
				document.querySelector('.vd-player [data-eq-delete]').click();
				const sel=document.querySelector('.vd-player [data-eq-select]');
				if([...sel.options].some(o=>o.text==='Studio Night'))throw Error('deleted profile still present');
				document.querySelector('.vd-player [data-eq-reset]').click();
				if(sliders.some(s=>s.value!=='0'))throw Error('EQ reset did not zero all bands');
				document.querySelector('.vd-player [data-eq-enabled]').click();
				if(!document.querySelector('.vd-player [data-eq-panel]').classList.contains('is-bypassed'))throw Error('EQ bypass toggle failed');
				document.querySelector('.vd-player [data-eq-close]').click();
				if(!document.querySelector('.vd-player [data-eq-panel]').hidden)throw Error('EQ close button did not hide panel');
			}`)
		}

		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if viewport[0] < 800 {
				page.MustEval(`()=>{document.querySelector('.vd-player [data-eq-toggle]').click();}`)
				page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("audio-player-eq-%dx%d.png", viewport[0], viewport[1])))
				page.MustEval(`()=>{document.querySelector('.vd-player [data-eq-close]').click();}`)
			}
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("audio-player-%dx%d.png", viewport[0], viewport[1])))
		}
		page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
		if viewport[0] > 800 {
			page.MustWait(`()=>AudioPlayerApp.instances.size===0`)
			if !page.MustEval(`()=>AuraDesktopMediaSession.snapshot().metadata.title==='Fallback player'`).Bool() {
				t.Fatal("disposing the native player must release only its owner and restore the fallback owner")
			}
		}
	}

	// Started without a file the player falls back to the music library.
	page.MustEval(`async()=>{aurora.openApp('audio-player',{});await new Promise(r=>setTimeout(r,500));}`)
	if !page.MustEval(`()=>document.querySelectorAll('.vd-player-queue-item').length>0`).Bool() {
		t.Fatal("player opened without a file must show the music library")
	}
	page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
	page.MustWait(`()=>AudioPlayerApp.instances.size===0`)
	page.MustEval(`()=>{mediaSessionUnsubscribe?.();AuraDesktopMediaSession.release(mediaFallbackOwner);}`)

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
