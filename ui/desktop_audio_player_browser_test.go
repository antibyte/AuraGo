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
		page.MustEval(`async()=>{fixtureTheme('standard');aurora.openApp('audio-player',{path:'Music/galaga.mp3'});await new Promise(r=>setTimeout(r,600));}`)

		if issues := page.MustEval(`()=>{
			const issues=[];
			const player=document.querySelector('.vd-player');
			if(!player)return 'player window missing';
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
			for(const sel of ['[data-play]','[data-next]','[data-prev]','[data-seek]','[data-volume]','[data-visualizer-mode]','[data-shuffle]','[data-repeat]']){
				if(!document.querySelector('.vd-player '+sel))issues.push('control missing: '+sel);
			}
			const volume=document.querySelector('.vd-player [data-volume]')?.getBoundingClientRect();
			const speed=document.querySelector('.vd-player [data-speed]')?.getBoundingClientRect();
			if(volume&&speed&&volume.left<speed.right&&volume.right>speed.left&&volume.top<speed.bottom&&volume.bottom>speed.top)issues.push('volume slider overlaps playback speed control');
			const root=player.getBoundingClientRect();
			if(root.width<200||root.height<200)issues.push('player is too small: '+root.width+'x'+root.height);
			return issues.join('\n');
		}`).Str(); issues != "" {
			t.Fatalf("audio player at %dx%d: %s", viewport[0], viewport[1], issues)
		}

		// The spectrum must actually paint, not only exist.
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

		// The play button pauses and resumes again.
		page.MustEval(`()=>document.querySelector('.vd-player [data-play]').click()`)
		if !page.MustEval(`()=>{const a=document.querySelector('.vd-player audio');return !!a&&a.paused;}`).Bool() {
			t.Fatalf("play button must pause at %dx%d", viewport[0], viewport[1])
		}
		page.MustEval(`async()=>{document.querySelector('.vd-player [data-play]').click();await new Promise(r=>setTimeout(r,300));}`)
		if !page.MustEval(`()=>{const a=document.querySelector('.vd-player audio');return !!a&&!a.paused&&a.currentTime>0;}`).Bool() {
			t.Fatalf("play button must resume at %dx%d", viewport[0], viewport[1])
		}

		// The visualizer mode cycles through all three renderers.
		modes := []string{}
		for i := 0; i < 3; i++ {
			page.MustEval(`async()=>{document.querySelector('.vd-player [data-visualizer-mode]').click();await new Promise(r=>setTimeout(r,80));}`)
			modes = append(modes, page.MustEval(`()=>document.querySelector('.vd-player [data-visualizer-mode]').textContent.trim()`).Str())
		}
		if modes[0] == "" || modes[0] == modes[1] || modes[1] == modes[2] {
			t.Fatalf("visualizer mode must cycle through three renderers, got %v", modes)
		}

		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("audio-player-%dx%d.png", viewport[0], viewport[1])))
		}
		page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)
	}

	// Started without a file the player falls back to the music library.
	page.MustEval(`async()=>{aurora.openApp('audio-player',{});await new Promise(r=>setTimeout(r,500));}`)
	if !page.MustEval(`()=>document.querySelectorAll('.vd-player-queue-item').length>0`).Bool() {
		t.Fatal("player opened without a file must show the music library")
	}
	page.MustEval(`()=>[...aurora.state.windows.keys()].forEach(id=>aurora.closeWindow(id))`)

	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}
