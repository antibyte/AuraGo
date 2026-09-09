package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
	_ "golang.org/x/image/webp"
)

func TestGalaxyEmbeddedAssetsAndLocales(t *testing.T) {
	for name, size := range map[string][2]int{
		"space-nebula-4k.webp": {3840, 2160}, "space-nebula-2k.webp": {2048, 1152},
		"orb-companion.png": {1024, 1024}, "orbit-mark.png": {512, 512},
		"poster-4k.webp": {3840, 2160}, "poster-2k.webp": {2048, 1152}, "poster-mobile.webp": {1080, 1920},
		"earth-day-4k.jpg": {4096, 2048}, "earth-day-2k.jpg": {2048, 1024},
		"earth-night.png": {2048, 1024}, "earth-clouds.jpg": {1024, 512}, "galaxy-detail.jpg": {4096, 1310},
	} {
		data, err := Content.ReadFile("img/galaxy/" + name)
		if err != nil {
			t.Fatal(err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != size[0] || cfg.Height != size[1] {
			t.Fatalf("invalid embedded image %s: %v %v", name, cfg, err)
		}
	}
	locales, err := Content.ReadDir("lang/chat")
	if err != nil {
		t.Fatal(err)
	}
	if len(locales) != 16 {
		t.Fatalf("expected all 16 chat locales, got %d", len(locales))
	}
	for _, locale := range locales {
		var dict map[string]string
		data, err := Content.ReadFile("lang/chat/" + locale.Name())
		if err != nil || json.Unmarshal(data, &dict) != nil || dict["chat.theme_galaxy"] != "Galaxy" || dict["chat.galaxy_title"] == "" || dict["chat.galaxy_tools"] == "" {
			t.Errorf("Galaxy label missing in %s", locale.Name())
		}
	}
	for _, name := range []string{"orb-companion.png", "orbit-mark.png"} {
		data, _ := Content.ReadFile("img/galaxy/" + name)
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		_, _, _, alpha := img.At(0, 0).RGBA()
		if alpha != 0 {
			t.Errorf("%s must have a transparent canvas", name)
		}
	}
	for _, path := range []string{"fonts/BarlowCondensed-SemiBold.ttf", "fonts/BarlowCondensed-LICENSE.txt", "img/galaxy/CREDITS.md", "img/chat-ui-icons/theme-galaxy.png", "img/galaxy/control-symbols.svg", "img/galaxy/LUCIDE-LICENSE.txt"} {
		if data, err := Content.ReadFile(path); err != nil || len(data) == 0 {
			t.Errorf("missing embedded Galaxy asset %s", path)
		}
	}
}

// Use the actual Chat DOM, picker, drawer and modal code. Only backend data and
// submission are fixtures; the WebGL renderer and requestAnimationFrame are real.
func TestGalaxyBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	extract := func(path, pattern string) string {
		s := regexp.MustCompile(pattern).FindString(readDesktopAssetText(t, path))
		if s == "" {
			t.Fatalf("missing fixture source %s in %s", pattern, path)
		}
		return s
	}
	controls := extract("js/chat/main/state-dom.js", `(?ms)^function bindHeaderActivation\(.*?^}`) + "\n" +
		extract("js/chat/main/state-dom.js", `(?ms)^function applyChatIcon\(.*?^}`) + "\n" +
		extract("js/chat/main/bootstrap.js", `(?ms)^const THEME_ICON_KEYS = \{.*?^};`) + "\n" +
		extract("js/chat/main/bootstrap.js", `(?ms)^function initChatThemePicker\(.*?^}`)
	controls += "\nconst _desktopMQ=window.matchMedia('(min-width:768px)'); const composerPanel=document.getElementById('composer-panel'),composerMoreBtn=document.getElementById('composer-more-btn'); function closeMoodFeedbackRow(){} function closeCheatsheetPicker(){}\n" +
		extract("js/chat/main/i18n-ui-chrome.js", `(?ms)^function isDesktopView\(.*?^}`) + "\n" +
		extract("js/chat/main/i18n-ui-chrome.js", `(?ms)^function closeComposerPanel\(.*?^}`) + "\n" +
		extract("js/chat/main/i18n-ui-chrome.js", `(?ms)^function toggleComposerPanel\(.*?^}`) + "\n" +
		extract("js/chat/main/composer-uploads.js", `(?ms)^if \(composerMoreBtn && composerPanel\) \{.*?^}`) + "\n" +
		extract("js/chat/main/composer-uploads.js", `(?ms)^if \(composerPanel\) \{.*?^}`)
	scene := strings.Replace(readDesktopAssetText(t, "js/chat/galaxy-scene.js"), "    window.AuraGoGalaxy =", `
    window.__galaxy = {get runtime(){return runtime}, draw, resize, updateQuality,
        stats(){const r=runtime;return r ? {ready:r.ready,quality:r.quality,mobile:r.mobile,time:r.time,frameBudget:r.frameBudget,
            width:r.canvas.width,height:r.canvas.height,stars:r.stars?.geometry.attributes.position.count,
            calls:r.renderer.info.render.calls,...r.renderer.info.memory} : null}}
    window.AuraGoGalaxy =`, 1)
	// The shared privacy policy strips Referer. Fault injection is explicit in
	// test-only asset URLs rather than depending on browser referrer behavior.
	scene = strings.Replace(scene, "'?v=' +", "'?mode=' + encodeURIComponent(location.search.slice(1)) + '&v=' +", 1)
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "index.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	fixture := `<script>
document.documentElement.lang='de';window.BUILD_VERSION='galaxy-browser-test';window.I18N=Object.assign(` + readDesktopAssetText(t, "lang/common/de.json") + `,` + readDesktopAssetText(t, "lang/chat/de.json") + `);
window.__errors=[];addEventListener('error',e=>__errors.push(e.message));
const error=console.error;console.error=(...a)=>{__errors.push(a.join(' '));error(...a)};
window.__pending=new Set();const nativeRAF=requestAnimationFrame.bind(window),nativeCancel=cancelAnimationFrame.bind(window);
window.requestAnimationFrame=fn=>{const id=nativeRAF(t=>{__pending.delete(id);fn(t)});__pending.add(id);return id};
window.cancelAnimationFrame=id=>{__pending.delete(id);nativeCancel(id)};
if(location.search.includes('no-webgl')){const get=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(kind,...args){return /webgl/.test(kind)?null:get.call(this,kind,...args)}}
</script>
<script src="/js/shared/prepaint-theme.js?v=galaxy-browser-test"></script>
<script src="/js/shared/shared-core.js"></script><script src="/js/shared/lazy-assets.js"></script>
<script src="/js/shared/shared-chat.js"></script><script src="/js/chat/ui-icons.js"></script>
<script src="/js/chat/modules/session-drawer.js"></script><script src="/js/chat/modules/integrations-drawer.js"></script>
<script>` + controls + `
document.querySelectorAll('[data-i18n]').forEach(el=>el.textContent=t(el.dataset.i18n));
document.querySelectorAll('[data-i18n-aria-label]').forEach(el=>el.setAttribute('aria-label',t(el.dataset.i18nAriaLabel)));
document.querySelector('.greeting-text').textContent=t('chat.greeting');
document.getElementById('user-input').placeholder=t('chat.input_placeholder');
document.getElementById('tokenCounter').textContent='0 Token';
document.getElementById('moodText').textContent='Fokussiert';
AuraChatIcons.applyIcon(document.getElementById('moodEmoji'),'target');
document.getElementById('personality-label').textContent='Thinker';
document.getElementById('personality-current-icon').src='/img/personas/thinker.png';
document.getElementById('moodToggle').style.display='flex';
document.getElementById('debug-pill').classList.add('is-hidden');
document.getElementById('connectionPill').textContent='Verbunden';
document.getElementById('logout-btn').classList.remove('is-hidden');
document.getElementById('logout-btn').textContent=t('chat.logout_label');
document.getElementById('warnings-badge').classList.remove('is-hidden');
document.getElementById('warnings-badge').textContent='3';
document.getElementById('chat-form').addEventListener('submit',e=>{e.preventDefault();window.__submitted=document.getElementById('user-input').value});
SessionDrawer.init();initTheme();initChatThemePicker();
</script><script src="/js/chat/theme-effects.js"></script>`
	html = strings.Replace(html, "</body>", fixture+"</body>", 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(html))
		case "/js/chat/galaxy-scene.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(scene))
		case "/api/chat/sessions", "/api/chat/sessions/default", "/api/integrations/webhosts":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sessions":[],"webhosts":[],"session":{"id":"default"}}`))
		default:
			if strings.Contains(r.URL.Path, "/img/galaxy/earth-day") {
				if r.URL.Query().Get("mode") == "missing" {
					http.NotFound(w, r)
					return
				}
				if r.URL.Query().Get("mode") == "late" {
					time.Sleep(500 * time.Millisecond)
				}
			}
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	url := l.MustLaunch()
	defer func() { l.Kill(); l.Cleanup() }()
	b := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer b.Close()
	p := b.MustPage("about:blank")
	defer p.Close()
	version, err := (proto.BrowserGetVersion{}).Call(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("browser=%s", version.Product)
	reduce := func(value string) {
		t.Helper()
		if err := (proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: value}}}).Call(p); err != nil {
			t.Fatal(err)
		}
	}
	check := func(js, message string) {
		t.Helper()
		if !p.MustEval(js).Bool() {
			t.Fatal(message)
		}
	}
	ready := func() {
		p.Timeout(15 * time.Second).MustWait(`() => window.__galaxy?.runtime?.ready && __galaxy.runtime.canvas.classList.contains('is-ready')`)
	}
	artifact := func(name string) {
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			p.MustScreenshot(filepath.Join(dir, name+".png"))
		}
	}
	p.MustSetViewport(1920, 1080, 1, false)
	reduce("no-preference")
	baseline := p.MustEval(`async () => {let first=0,last=0,n=0;await new Promise(resolve=>{function tick(t){if(!first)first=t;last=t;n++;if(t-first<1000)requestAnimationFrame(tick);else resolve()}requestAnimationFrame(tick)});return (n-1)*1000/(last-first)}`).Num()
	t.Logf("empty page native RAF: %.2f FPS", baseline)
	p.MustNavigate(server.URL).MustWaitLoad()
	p.MustElement("#chat-theme-btn").MustClick()
	p.MustElement(`.chat-theme-option[data-theme="galaxy"]`).MustClick()
	ready()
	check(`() => localStorage.getItem('aurago-theme')==='galaxy' && document.querySelector('meta[name="theme-color"]').content==='#05060d' && document.querySelector('#chat-theme-icon').dataset.chatIcon==='theme-galaxy'`, "picker, persistence, icon or theme color failed")
	p.MustReload().MustWaitLoad()
	ready()
	check(`() => document.documentElement.dataset.theme==='galaxy' && __pending.size===1`, "reload failed or duplicated scheduler")
	color := p.MustEval(`() => getComputedStyle(document.getElementById('chat-theme-btn')).backgroundColor`).Str()
	p.MustElement("#chat-theme-btn").MustHover()
	p.MustEval(`() => new Promise(resolve=>setTimeout(resolve,200))`)
	if p.MustEval(`() => getComputedStyle(document.getElementById('chat-theme-btn')).backgroundColor`).Str() == color {
		t.Fatal("Galaxy button hover state is overridden")
	}
	p.MustElement("#chat-box").MustHover()
	// Read real GPU pixels with only the star layer visible, so background drift
	// cannot masquerade as twinkling. Exercise several independently timed pulses.
	check(`() => {
        const r=__galaxy.runtime,stars=r.stars,geometry=stars.geometry;
        const indices=Array.from(geometry.attributes.aTwinkle.array).flatMap((v,i)=>v>0?[i]:[]);
        if(indices.length!==20 || new Set(indices.map(i=>geometry.attributes.aTwinkle.array[i])).size!==20)return false;
        const target=new THREE.WebGLRenderTarget(256,256),scene=new THREE.Scene(),pixels=new Uint8Array(256*256*4);
        const uniforms=stars.material.uniforms,drift=uniforms.uDrift.value.clone(),time=uniforms.uTime.value;
        const levels=indices.map(()=>[]);scene.add(stars);uniforms.uDrift.value.set(0,0);
        try {
            r.renderer.setRenderTarget(target);
            for(let step=0;step<48;step++){
                uniforms.uTime.value=step*0.4;r.renderer.render(scene,r.camera);
                r.renderer.readRenderTargetPixels(target,0,0,256,256,pixels);
                indices.forEach((i,j)=>{
                    const x=Math.floor((geometry.attributes.position.getX(i)+1)*128),y=Math.floor((geometry.attributes.position.getY(i)+1)*128);
                    let energy=0;for(let dy=-2;dy<=2;dy++)for(let dx=-2;dx<=2;dx++){const p=((y+dy)*256+x+dx)*4;energy+=pixels[p]+pixels[p+1]+pixels[p+2]}
                    levels[j].push(energy);
                });
            }
            return levels.every(v=>Math.max(...v)-Math.min(...v)>2);
        } finally {
            r.renderer.setRenderTarget(null);target.dispose();r.scene.add(stars);uniforms.uDrift.value.copy(drift);uniforms.uTime.value=time;__galaxy.draw(r);
        }
    }`, "twenty independently timed stars did not flicker in real GPU output")
	check(`() => {
        const r=__galaxy.runtime,time=r.time,before=r.ships.map(s=>s.position.clone());
        try {
            r.time+=10;__galaxy.draw(r);
            return r.ships.length===4 && new Set(r.ships.map(s=>s.geometry.id)).size===4 && r.ships.every((s,i)=>{
                const dx=s.position.x-before[i].x,dy=s.position.y-before[i].y;
                return s.geometry.attributes.position.count>500 && Math.abs(dx)>0.1 && Math.abs(dy/dx)>0.45;
            });
        }
        finally {r.time=time;__galaxy.draw(r)}
    }`, "four detailed ship designs did not follow visibly diagonal routes")
	// Inspect the actual meshes at a larger scale as well as at flight distance.
	p.MustEval(`() => {Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));const r=__galaxy.runtime;r.ships.forEach((s,i)=>{s.position.set(-1.2+i*0.8,0.3,-3);s.scale.setScalar(0.22);s.rotation.set(0.35,-0.2,0.4)});r.renderer.render(r.scene,r.camera)}`)
	artifact("galaxy-fleet-detail")
	p.MustEval(`() => {__galaxy.draw(__galaxy.runtime);delete document.hidden;document.dispatchEvent(new Event('visibilitychange'))}`)

	for _, size := range [][2]int{{1672, 941}, {1920, 1080}, {1366, 768}, {1180, 800}, {768, 1024}, {1024, 768}, {390, 844}, {430, 932}} {
		p.MustSetViewport(size[0], size[1], 1, size[0] < 768)
		p.MustReload().MustWaitLoad()
		ready()
		p.MustEval(`async () => {await document.fonts.ready; await new Promise(r=>setTimeout(r,300))}`)
		check(`() => {
            const g=__galaxy.stats();const buttons=[...document.querySelectorAll('.app-header button:not(.chat-theme-option),#chat-form > button,.galaxy-nav > a,.galaxy-nav > button,.galaxy-agents')].filter(e=>e.getClientRects().length && getComputedStyle(e).visibility!=='hidden');
            return document.body.scrollWidth<=innerWidth && g.width*g.height<=3840*2160 && g.stars===(innerWidth<768?850:3500) && buttons.every(e=>{const r=e.getBoundingClientRect();return r.width>=44 && r.height>=44});
        }`, fmt.Sprintf("layout, touch target or resolution budget at %v", size))
		check(`() => {
            const nav=document.querySelector('.galaxy-nav').getBoundingClientRect(),g=document.querySelector('.greeting-row').getBoundingClientRect();
            const input=document.getElementById('user-input').getBoundingClientRect(),send=document.getElementById('send-btn').getBoundingClientRect();
            return g.left>=nav.right && g.right<=innerWidth && input.width>=100 && send.right<=innerWidth && input.top>=0 && send.bottom<=innerHeight;
        }`, fmt.Sprintf("greeting, navigation or composer bounds at %v", size))
		if size[0] == 1672 {
			check(`() => {
                const g=document.querySelector('.greeting-row').getBoundingClientRect(),i=document.getElementById('user-input').getBoundingClientRect();
                return Math.abs(g.x-326)<12 && Math.abs(g.y-375)<12 && Math.abs(g.width-968)<12 && Math.abs(g.height-225)<12 && Math.abs(i.x-510)<12 && Math.abs(i.y-754)<12;
            }`, "Galaxy reference geometry drifted")
		}
		artifact(fmt.Sprintf("galaxy-greeting-%dx%d", size[0], size[1]))
		p.MustElement("#chat-theme-btn").MustClick()
		check(`() => {const d=document.getElementById('chat-theme-dropdown'),r=d.getBoundingClientRect();return !d.hidden && r.left>=0 && r.right<=innerWidth && r.bottom<=innerHeight && r.height>100}`, "Galaxy theme menu must remain reachable inside a scrolling header")
		p.MustElement(`.chat-theme-option[data-theme="galaxy"]`).MustClick()
		p.Timeout(10 * time.Second).MustElement(".galaxy-suggestion").MustClick()
		check(`() => document.getElementById('user-input').value===t('chat.galaxy_ideas') && document.activeElement.id==='user-input' && !window.__submitted`, "suggestion must prepare a draft without sending")
		p.Timeout(10 * time.Second).MustElement("#composer-more-btn").MustClick()
		check(`() => !document.getElementById('composer-panel').classList.contains('is-hidden') && document.getElementById('composer-more-btn').getAttribute('aria-expanded')==='true' && document.getElementById('upload-btn').parentElement.id==='chat-form' && document.getElementById('realtime-speech-btn').parentElement.id==='composer-panel'`, "Galaxy tools or live speech placement failed")
		artifact(fmt.Sprintf("galaxy-tools-%dx%d", size[0], size[1]))
		p.Keyboard.MustType(input.Escape)
		check(`() => document.getElementById('composer-panel').classList.contains('is-hidden')`, "Escape did not close Galaxy tools")
		p.MustEval(`() => {
            const status=document.getElementById('connectionPill');status.className='pill pill-disconnected';status.textContent='Getrennt';
        }`)
		p.MustWait(`() => document.querySelector('.galaxy-welcome-status').dataset.state==='disconnected' && document.querySelector('.galaxy-welcome-status').textContent==='Getrennt'`)
		p.MustEval(`() => {const status=document.getElementById('connectionPill');status.className='pill pill-active';status.textContent='Verbunden'}`)
		check(`() => {
            const badge=document.getElementById('warnings-badge'),button=document.getElementById('warnings-btn');badge.textContent='128';
            const r=badge.getBoundingClientRect(),b=button.getBoundingClientRect();const fits=r.left>=b.left && r.right<=b.right && badge.scrollWidth<=badge.clientWidth;
            badge.classList.add('seen');const readable=parseFloat(getComputedStyle(badge).fontSize)>=12;
            badge.classList.remove('seen');badge.textContent='3';return fits && readable;
        }`, "warning counts are clipped or unreadable")
		p.MustEval(`() => {const c=document.getElementById('chat-content');c.insertAdjacentHTML('beforeend','<div class="msg-row user"><div class="message-stack"><div class="bubble user">Zeig mir, was heute wichtig ist.</div></div></div><div class="msg-row bot"><div class="message-stack"><div class="bubble bot"><p>Alles im Blick.</p><p>Deine Projekte, Termine und Ideen. Bereit für den nächsten Schritt.</p></div></div></div>')}`)
		p.MustWait(`() => !document.getElementById('chat-content').classList.contains('galaxy-welcome-only')`)
		artifact(fmt.Sprintf("galaxy-chat-%dx%d", size[0], size[1]))
	}
	p.MustSetViewport(2560, 1440, 2, false)
	p.MustReload().MustWaitLoad()
	ready()
	check(`() => {const s=__galaxy.stats();return s.width===3840 && s.height===2160}`, "high DPI exceeded or missed the 4K render cap")
	p.MustSetViewport(430, 932, 1, true)
	p.MustReload().MustWaitLoad()
	ready()
	p.MustElement("#user-input").MustInput("Galaxy bleibt bedienbar")
	p.MustElement("#send-btn").MustClick()
	check(`() => __submitted==='Galaxy bleibt bedienbar'`, "composer blocked by graphics")
	p.MustEval(`() => {const c=document.getElementById('chat-content');window.__messages=c.innerHTML;c.innerHTML+=c.innerHTML.repeat(10);document.getElementById('chat-box').scrollTop=100}`)
	check(`() => document.getElementById('chat-box').scrollTop>0`, "chat could not scroll")
	p.MustEval(`() => document.getElementById('chat-content').innerHTML=__messages`)
	for _, drawer := range []string{"session", "integrations"} {
		p.MustElement("#" + drawer + "-toggle-btn").MustClick()
		p.MustElement("#" + drawer + "-drawer.open")
		artifact("galaxy-" + drawer)
		p.MustElement("#" + drawer + "-drawer-close").MustClick()
	}
	p.MustEval(`() => {showAlert('Galaxy','Die Chatfunktionen bleiben erreichbar.');return true}`)
	p.MustElement("#modal-overlay.active")
	artifact("galaxy-dialog")
	p.MustElement("#modal-confirm").MustClick()

	p.MustSetViewport(1920, 1080, 1, false)
	p.MustReload().MustWaitLoad()
	ready()
	for i := 0; i < 10; i++ {
		check(`() => {window.__old=__galaxy.runtime;setChatTheme('dark');return __galaxy.runtime===null && __pending.size===0 && !document.querySelector('#galaxy-scene') && __old.renderer.info.memory.textures===0 && __old.renderer.info.memory.geometries===0 && __old.renderer.info.programs.length===0}`, "theme exit leaked GPU resources")
		check(`() => !document.querySelector('.galaxy-nav,.galaxy-clock,.galaxy-welcome,.galaxy-glyph') && document.getElementById('upload-btn').parentElement.id==='composer-panel' && document.getElementById('composer-more-btn').previousElementSibling.id==='send-btn' && !document.getElementById('composer-panel').classList.contains('is-hidden')`, "Galaxy controls were not restored to the default arrangement")
		p.MustEval(`() => setChatTheme('galaxy')`)
		ready()
		check(`() => {const s=__galaxy.stats();return __pending.size===1 && document.querySelectorAll('#galaxy-scene').length===1 && s.textures===5 && s.geometries===7 && s.calls===10}`, "theme restart grew resources")
	}
	p.MustEval(`() => {Object.defineProperty(document,'hidden',{configurable:true,value:true});document.dispatchEvent(new Event('visibilitychange'));window.__paused=__galaxy.runtime.time}`)
	check(`() => __pending.size===0 && __galaxy.runtime.time===__paused`, "hidden tab did not pause")
	p.MustEval(`() => {delete document.hidden;document.dispatchEvent(new Event('visibilitychange'))}`)
	check(`() => __pending.size===1`, "visible tab did not resume")
	p.MustWait(`() => __galaxy.runtime.cadence===null`)
	check(`() => __galaxy.runtime.quality===1`, "display refresh rate unnecessarily reduced quality")
	// Add bounded CPU work to real rendered frames, independently of the FPS
	// measurement. This catches scheduler/quality integration, not just the formula.
	quality := p.MustEval(`async () => {
        const r=__galaxy.runtime,render=r.renderer.render.bind(r.renderer),work=Math.max(45,r.frameBudget*2000);r.sampleTime=r.sampleFrames=0;
        r.renderer.render=(...args)=>{render(...args);const end=performance.now()+work;while(performance.now()<end){}};
        try {await new Promise(resolve=>setTimeout(resolve,5000))} finally {r.renderer.render=render}
        return r.quality;
    }`).Num()
	t.Logf("CPU load at twice the measured frame budget (minimum 45 ms), 5 seconds: quality %.2f", quality)
	check(`() => __galaxy.runtime.quality<1 && __galaxy.runtime.quality>=0.65 && __galaxy.runtime.canvas.width<1920`, "slow frames did not reduce resolution")

	p.MustEval(`() => {window.__gl=__galaxy.runtime.renderer.getContext();window.__loss=__gl.getExtension('WEBGL_lose_context');__loss.loseContext()}`)
	p.MustWait(`() => __galaxy.runtime.lost`)
	check(`() => __pending.size===0 && !__galaxy.runtime.canvas.classList.contains('is-ready')`, "context loss did not reveal poster")
	p.MustEval(`() => __loss.restoreContext()`)
	ready()
	check(`() => __galaxy.runtime.renderer.getContext()!==__gl`, "context restore failed to recreate resources")
	reduce("reduce")
	p.MustWait(`() => !__galaxy.runtime`)
	artifact("galaxy-reduced-motion")
	reduce("no-preference")
	ready()
	check(`() => __errors.length===0`, "unexpected browser errors: "+p.MustEval(`() => __errors`).Str())

	if os.Getenv("AURAGO_GALAXY_BENCHMARK") == "1" {
		p.MustReload().MustWaitLoad()
		ready()
		p.MustWait(`() => __galaxy.runtime.cadence===null`)
		// Observe completed render submissions across two minutes of native RAF.
		// No manual scene ticks or synthetic FPS values enter this measurement.
		result := p.MustEval(`async () => {
			const r=__galaxy.runtime,render=r.renderer.render.bind(r.renderer),times=[],gl=r.renderer.getContext();
			r.renderer.render=(...a)=>{render(...a);times.push(performance.now())};
            await new Promise(resolve=>setTimeout(resolve,120000));r.renderer.render=render;
            const intervals=times.slice(1).map((v,i)=>v-times[i]).sort((a,b)=>a-b);
			const d=gl.getExtension('WEBGL_debug_renderer_info');
            return {gpu:d?gl.getParameter(d.UNMASKED_RENDERER_WEBGL):gl.getParameter(gl.RENDERER),userAgent:navigator.userAgent,
                frames:times.length,seconds:(times.at(-1)-times[0])/1000,fps:(times.length-1)*1000/(times.at(-1)-times[0]),
				p50:intervals[Math.floor(intervals.length*.5)],p95:intervals[Math.floor(intervals.length*.95)],
                stalls100ms:intervals.filter(x=>x>100).length,quality:r.quality,...__galaxy.stats()};
		}`)
		data := result.Map()
		data["browserProduct"] = gson.New(version.Product)
		data["emptyPageFPS"] = gson.New(baseline)
		result = gson.New(data)
		t.Logf("120 second real rendering: %s", result.JSON("", "  "))
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			if err := os.WriteFile(filepath.Join(dir, "galaxy-benchmark.json"), []byte(result.JSON("", "  ")), 0644); err != nil {
				t.Fatal(err)
			}
		}
		check(`() => __pending.size===1 && __errors.length===0`, "benchmark leaked schedulers or produced errors")
		p.MustElement("#user-input").MustInput("Nach zwei Minuten weiterhin bedienbar")
	}

	for _, mode := range []string{"missing", "no-webgl", "late"} {
		p.MustNavigate(server.URL + "/?" + mode)
		if mode == "late" {
			p.Timeout(15 * time.Second).MustWait(`() => window.__galaxy?.runtime && !__galaxy.runtime.ready`)
			p.MustEval(`() => setChatTheme('dark')`)
			// Let the delayed texture callbacks complete after the theme has exited.
			p.MustEval(`() => new Promise(resolve=>setTimeout(resolve,750))`)
		} else {
			p.MustWaitLoad()
			p.Timeout(15 * time.Second).MustWait(`() => window.__galaxy && !__galaxy.runtime`)
			check(`async () => {const image=new Image();image.src=getComputedStyle(document.body,'::before').backgroundImage.slice(5,-2);await image.decode();return image.naturalWidth>=2048}`, "complete fallback poster missing")
			artifact("galaxy-" + mode)
		}
		check(`() => !document.querySelector('#galaxy-scene') && __pending.size===0`, "fallback or late load activated a discarded renderer")
	}
}
