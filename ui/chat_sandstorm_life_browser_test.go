package ui

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// The page and the stubs match TestSandstormWeatherBrowserSmoke: a frame queue the
// test steps by hand (__frame), a seeded Math.random and a stubbed matchMedia. The
// additions are the unhandled-rejection capture, the ?fallback switch that removes
// WebGL (so hundreds of frames cost milliseconds instead of software GL) and the ?lazy
// switch that stands in for the shared AuraLazyAssets loader (its first load fails, see __lazyFail).
const sandstormLifeFixturePage = `<!doctype html><html data-theme="sandstorm"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<link rel="stylesheet" href="/css/chat.bundle.css"><style>
body{height:100vh;overflow:hidden;display:flex;flex-direction:column;margin:0}
.app-header{position:relative;flex:0 0 64px;padding:0 32px}
#chat-box{flex:1;min-height:0;width:100%;padding:56px max(24px,calc((100vw - 820px)/2));box-sizing:border-box;overflow-y:auto}
.app-footer{position:relative;flex:0 0 110px;padding:22px max(24px,calc((100vw - 820px)/2))}
.msg-row{margin-bottom:36px}.bubble{padding:18px 24px;max-width:600px}.msg-row.user{justify-content:flex-end}
input{width:100%;padding:16px;color:var(--text-primary);background:var(--input-bg);border:1px solid var(--input-border);border-radius:16px}
</style></head><body><header class="app-header"><strong>AuraGo</strong></header><main id="chat-box">
<div class="msg-row"><div class="bubble bot">Der Wind zieht über die Dünen. Feiner Staub wirbelt durch die warme Abendluft.</div></div>
<div class="msg-row user"><div class="bubble user">Wie entwickelt sich der Sandsturm?</div></div>
<div class="msg-row"><div class="bubble bot">Eine Böe kommt näher. Die Sandkörner tanzen über den Boden, während dünne Staubschleier vorbeiziehen.</div></div>
</main><footer class="app-footer"><input id="composer" aria-label="Nachricht" placeholder="Nachricht schreiben …"></footer>
<script>
window.__errors=[];window.addEventListener('error',e=>__errors.push(e.message));console.error=(...args)=>__errors.push(args.join(' '));
window.addEventListener('unhandledrejection',e=>__errors.push('unhandled rejection: '+e.reason));
window.__reduce=false;window.matchMedia=()=>({get matches(){return window.__reduce},addEventListener(){}});
window.__nativeRAF=window.requestAnimationFrame.bind(window);window.__pending=new Map();let raf=0;window.requestAnimationFrame=fn=>{__pending.set(++raf,fn);return raf};window.cancelAnimationFrame=id=>__pending.delete(id);
window.__frame=time=>{const jobs=[...__pending.values()];__pending.clear();jobs.forEach(fn=>fn(time))};
if(location.search.includes('fallback')){const get=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(type,...args){return type==='webgl'?null:get.call(this,type,...args)}}
if(location.search.includes('lazy')){window.__lazyCalls=[];window.__lazyFail=true;window.AuraLazyAssets={loadScript(src){__lazyCalls.push(src);
/* like the shared loader: a script element that is already there is waited for, even when it failed */
if([...document.scripts].some(s=>(s.getAttribute('src')||'').includes(src.split('/').pop())))return window.AuraGoSandstormLife?Promise.resolve():new Promise(()=>{});
return new Promise((resolve,reject)=>{const s=document.createElement('script');s.src=(__lazyFail?'/missing':'')+src+'?v=lazy';s.onload=resolve;s.onerror=()=>reject(new Error('lazy load failed'));document.head.appendChild(s)})}}}
let seed=17;Math.random=()=>((seed=(Math.imul(seed,1664525)+1013904223)>>>0)/4294967296);
</script><script src="/sandstorm.js"></script></body></html>`

// sandstormLifeShim exposes the few engine internals the harness needs, ahead of the
// engine's public surface (the replaced line stays valid JavaScript).
const sandstormLifeShim = `
    window.__sand = {
        stats() { return { active, fogActive, stormIntensity, stormActive, weatherTime, windSpeed, dunes: duneLayers.length }; },
        calm() { nextStormTime = 1e12; if (stormActive) { stormActive = false; afterglow = 0; setWeatherState('calm'); } },
        forceStorm(time) { nextStormTime = time; stormActive = false; },
        setNextStorm(time) { nextStormTime = time; },
        ridge(i, x) { return lifeGeom.ridgeAt(i, x); },
        lifeGeom() { return lifeGeom; }
    };
    window.AuraGoSandstorm =`

// sandstormLifeManualClock installs __step(n, probe): n frames of 1000/60 ms on a synthetic clock,
// every stats() of the companion handed to probe. Two native frames come first, so the
// engine's ResizeObserver has delivered its initial callback (a resize rebuilds the
// dunes and clears the sand in the air; the creatures outlive it).
const sandstormLifeManualClock = `async () => {
        await new Promise(__nativeRAF);
        await new Promise(__nativeRAF);
        window.__t = performance.now();
        window.__step = (n, probe) => {
            for (let i = 0; i < n; i++) {
                window.__t += 1000 / 60;
                __frame(window.__t);
                if (probe) probe(AuraGoSandstormLife.stats(), i);
            }
        };
        __sand.calm();
        __step(3);
    }`

// TestSandstormDesertLifeBrowserSmoke drives the real engine and the real companion:
// a sandworm event in calm weather with the vultures circling, a storm that breaks over
// a second one (and a resize in the middle of its breach) and sends the vultures away,
// the forced tumbleweed, beetle and caravan (and what a storm does to them, and a resize
// to a narrower canvas), the scheduled tumbleweed waiting for a storm to pass, the worm's
// choice of a spot the chat bubbles leave free and the vultures' choice of a thermal they
// leave free (also while the thermal sways), the calm gate of the schedule, a pointer
// gust, the heat shimmer and a scroll.
// It checks that the creatures stand on the painted ridges, stay inside their pools,
// never add an animation loop and stop with the theme, that both loaders (the plain
// script fallback and the shared lazy one) attach, and that the lazy one retries after
// a failed load.
func TestSandstormDesertLifeBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	source := readDesktopAssetText(t, "js/chat/sandstorm-particles.js")
	if !strings.Contains(source, "    window.AuraGoSandstorm =") {
		t.Fatal("engine public surface line not found; the shim cannot be injected")
	}
	source = strings.Replace(source, "    window.AuraGoSandstorm =", sandstormLifeShim, 1)
	var (
		requestLock  sync.Mutex
		lifeRequests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(sandstormLifeFixturePage))
		case "/sandstorm.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(source))
		case "/js/chat/sandstorm-life.js":
			// Straight from disk; the engine's ?v= cache buster is irrelevant here.
			requestLock.Lock()
			lifeRequests = append(lifeRequests, r.URL.RawQuery)
			requestLock.Unlock()
			life, err := os.ReadFile(filepath.Join("js", "chat", "sandstorm-life.js"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write(life)
		default:
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	url := launch.MustLaunch()
	defer func() { launch.Kill(); launch.Cleanup() }()
	browser := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer browser.Close()

	openPage := func(query string) *rod.Page {
		page := browser.MustPage("about:blank")
		page.MustSetViewport(1280, 800, 1, false)
		page.MustNavigate(server.URL + "/?" + query).MustWaitLoad()
		return page
	}
	// waitFor polls until the page expression is true: the companion is fetched by the
	// engine on its own, asynchronously.
	waitFor := func(page *rod.Page, expr, what string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !page.MustEval(expr).Bool() {
			if time.Now().After(deadline) {
				t.Fatalf("%s: %v", what, page.MustEval(`() => ({errors: __errors, scripts: [...document.scripts].map(s => s.src)})`))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	page := openPage("fallback")
	defer page.Close()
	waitFor(page, `() => !!window.AuraGoSandstormLife && __sand.stats().active`, "the companion never attached")
	requestLock.Lock()
	requests := append([]string(nil), lifeRequests...)
	requestLock.Unlock()
	if len(requests) != 1 || !strings.HasPrefix(requests[0], "v=") {
		t.Errorf("the engine must request the companion exactly once, versioned with ?v=: %q", requests)
	}
	if page.MustEval(`() => document.querySelectorAll('script[data-aurago-sandstorm-life]').length`).Int() != 1 {
		t.Error("the fallback loader must leave exactly one marked script element")
	}
	page.MustEval(sandstormLifeManualClock)
	warm := page.MustEval(`() => AuraGoSandstormLife.stats()`).Map()
	if !warm["attached"].Bool() || !warm["finite"].Bool() || !warm["worm"].Nil() || warm["shimmer"].Num() < 0.3 {
		t.Fatalf("calm start must be idle with heat shimmer on: %v", warm)
	}
	// (the first tumbleweed is due 15 s after the start, the first beetle 20 s, the first caravan 80 s;
	// the vultures are still gliding in from the edge, so none of them counts as in view yet)
	if warm["vultures"].Int() != 0 || warm["tumbleweeds"].Int() != 0 || warm["beetles"].Int() != 0 || warm["caravan"].Bool() || !warm["tumbleweedX"].Nil() {
		t.Errorf("the first moments of a run must be empty of critters: %v", warm)
	}

	// One full sandworm event in calm weather: tremor, breach, settle.
	if !page.MustEval(`() => AuraGoSandstormLife.forceWorm() === true && AuraGoSandstormLife.stats().worm === 'tremor'`).Bool() {
		t.Fatal("forceWorm() must start a worm right away")
	}
	if page.MustEval(`() => AuraGoSandstormLife.forceWorm()`).Bool() {
		t.Error("forceWorm() must refuse while a worm is running")
	}
	run := page.MustEval(`() => {
        const seen = [], at = {};
        let maxSpray = 0, maxPuffs = 0, maxWisps = 0, finite = true, shimmer = 0, vulturesAt4s = -1, maxCritters = 0;
        __step(780, (s, i) => {
            if (s.worm && seen[seen.length - 1] !== s.worm) { seen.push(s.worm); at[s.worm] = i; }
            maxSpray = Math.max(maxSpray, s.spray); maxPuffs = Math.max(maxPuffs, s.puffs); maxWisps = Math.max(maxWisps, s.wisps);
            shimmer = Math.max(shimmer, s.shimmer);
            if (i === 270) vulturesAt4s = s.vultures;
            maxCritters = Math.max(maxCritters, s.tumbleweeds + s.beetles + (s.caravan ? 1 : 0));
            if (!s.finite) finite = false;
        });
        return { seen, at, maxSpray, maxPuffs, maxWisps, shimmer, finite, vulturesAt4s, maxCritters, last: AuraGoSandstormLife.stats(), pending: __pending.size,
                 errors: __errors, storm: __sand.stats().stormIntensity };
    }`).Map()
	t.Logf("worm run: %v", run)
	if got := fmt.Sprint(run["seen"].Arr()); got != "[tremor breach settle]" {
		t.Errorf("worm phases = %s, want tremor, breach, settle in that order", got)
	}
	at := run["at"].Map()
	if breach, settle := at["breach"].Int(), at["settle"].Int(); breach < 170 || breach > 240 || settle-breach < 200 || settle-breach > 280 {
		t.Errorf("phase timing off (frames at 60 Hz): breach starts at %d (want about 3.4 s = 204), settle starts %d frames later (want about 4.2 s = 252)", breach, settle-breach)
	}
	if run["maxSpray"].Int() <= 20 || run["maxSpray"].Int() > 320 || run["maxPuffs"].Int() == 0 || run["maxPuffs"].Int() > 24 || run["maxWisps"].Int() > 140 {
		t.Errorf("sand spray and dust puffs must show and stay inside their pools: spray=%v puffs=%v wisps=%v", run["maxSpray"], run["maxPuffs"], run["maxWisps"])
	}
	last := run["last"].Map()
	if !run["finite"].Bool() || !last["finite"].Bool() {
		t.Error("every live coordinate must stay finite")
	}
	if run["vulturesAt4s"].Int() < 2 || last["vultures"].Int() < 2 {
		t.Errorf("after 4 s of calm the vultures must circle in view: %v at 4.5 s, %v at the end", run["vulturesAt4s"], last["vultures"])
	}
	if run["maxCritters"].Int() != 0 {
		t.Errorf("no tumbleweed, beetle or caravan is due within the first 13 s: %v", run["maxCritters"])
	}
	// The engine's calm wind gusts to windExcess ~0.65 every few seconds. That must read as a
	// light drift of sand, not as a full pool.
	if run["maxWisps"].Int() >= 100 {
		t.Errorf("calm gusts must not fill the sand streak pool: %v wisps", run["maxWisps"])
	}
	if !last["worm"].Nil() || last["wormCount"].Int() != 1 || last["spray"].Int() != 0 || last["puffs"].Int() != 0 {
		t.Errorf("the event must end clean: %v", last)
	}
	if run["storm"].Num() != 0 {
		t.Error("the calm run must not meet a storm")
	}
	if run["shimmer"].Num() < 0.3 || run["shimmer"].Num() > 2.2 {
		t.Errorf("heat shimmer amplitude out of range: %v", run["shimmer"])
	}
	if run["pending"].Int() != 1 {
		t.Errorf("the companion must not add an animation loop: %d pending", run["pending"].Int())
	}
	if len(run["errors"].Arr()) > 0 {
		t.Errorf("page errors (a failing companion switches itself off and reports through console.error): %v", run["errors"])
	}

	// Crest wisps: a pointer gust lifts sand off the crests and the pool never overflows.
	gust := page.MustEval(`() => {
        const box = document.getElementById('chat-box'), r = box.getBoundingClientRect();
        box.dispatchEvent(new PointerEvent('pointermove', {clientX: r.x + 200, clientY: r.y + 200, bubbles: true}));
        box.dispatchEvent(new PointerEvent('pointermove', {clientX: r.x + 262, clientY: r.y + 188, bubbles: true}));
        let maxWisps = 0, finite = true;
        __step(40, s => { maxWisps = Math.max(maxWisps, s.wisps); finite = finite && s.finite; });
        return { maxWisps, finite, pending: __pending.size };
    }`).Map()
	if gust["maxWisps"].Int() < 5 || gust["maxWisps"].Int() > 140 || !gust["finite"].Bool() || gust["pending"].Int() != 1 {
		t.Errorf("a pointer gust must lift sand off the crests inside the pool: %v", gust)
	}

	// Heat shimmer: the horizon band of a striped test card moves sideways row by row and
	// nothing outside the band is touched.
	shimmer := page.MustEval(`() => {
        const g = __sand.lifeGeom(), w = Math.round(g.width), h = Math.round(g.height);
        const cv = document.createElement('canvas');
        cv.width = w; cv.height = h;
        const c = cv.getContext('2d');
        for (let x = 0; x < w; x += 8) { c.fillStyle = (x / 8) % 2 ? '#fff' : '#000'; c.fillRect(x, 0, 8, h); }
        const before = c.getImageData(0, 0, w, h).data;
        AuraGoSandstormLife.draw(c, g, 2);
        const after = c.getImageData(0, 0, w, h).data;
        const y0 = Math.max(0, g.layerTop(0) - 8), y1 = Math.min(g.ridgeAt(1, 0) + 12, g.groundLine);
        let inside = 0, changedInside = 0, changedOutside = 0;
        for (let y = 0; y < h; y++) {
            let changed = false;
            for (let x = 0; x < w * 4 && !changed; x += 4) changed = before[y * w * 4 + x] !== after[y * w * 4 + x];
            if (y >= Math.floor(y0) && y < Math.ceil(y1)) { inside++; if (changed) changedInside++; } else if (changed) changedOutside++;
        }
        return { y0, y1, inside, changedInside, changedOutside };
    }`).Map()
	if shimmer["inside"].Int() < 40 || float64(shimmer["changedInside"].Int()) < 0.7*float64(shimmer["inside"].Int()) || shimmer["changedOutside"].Int() != 0 {
		t.Errorf("heat shimmer must shift the rows of the horizon band and nothing else: %v", shimmer)
	}

	// The geometry contract: where the engine says a ridge is, the scene canvas is opaque
	// from there down. Checked flat, then after a scroll moved every dune by its parallax.
	ridgeCheck := `() => {
        const cv = document.getElementById('sandstorm-scene'), c = cv.getContext('2d'), k = cv.width / cv.clientWidth;
        const out = [];
        // (the scan starts at the far dune's image: above it the vultures fly, and they are opaque too)
        const skyEnd = Math.max(0, Math.floor(__sand.lifeGeom().layerTop(0) * k));
        for (const x of [90, 240, 420, 600, 780, 960, 1130, 1230]) {
            const col = c.getImageData(Math.round(x * k), 0, 1, cv.height).data;
            let first = -1;
            for (let y = skyEnd; y < cv.height; y++) if (col[y * 4 + 3] >= 160) { first = y / k; break; }
            out.push({ x, first, want: Math.min(__sand.ridge(0, x), __sand.ridge(1, x), __sand.ridge(2, x)) });
        }
        return out;
    }`
	assertRidges := func(label string) {
		t.Helper()
		for _, p := range page.MustEval(ridgeCheck).Arr() {
			first, want := p.Get("first").Num(), p.Get("want").Num()
			if first < 0 || first-want > 3.5 || want-first > 3.5 {
				t.Errorf("%s: at x=%v the painted ridge is at y=%v but ridgeAt says %v", label, p.Get("x"), first, want)
			}
		}
	}
	assertRidges("flat")
	page.MustEval(`async () => {
        const box = document.getElementById('chat-box'), spacer = document.createElement('div');
        spacer.style.height = '4000px';
        box.appendChild(spacer);
        box.style.scrollBehavior = 'auto';   // a smooth scroll would still be on its way when the dunes are checked
        box.scrollTop = 900;
        // the scrollbar narrows the chat: let the ResizeObserver rebuild the dunes before drawing again
        await new Promise(__nativeRAF);
        await new Promise(__nativeRAF);
        __step(2);
    }`)
	if top := page.MustEval(`() => document.getElementById('chat-box').scrollTop`).Int(); top < 800 {
		t.Fatalf("the chat did not scroll (scrollTop=%d)", top)
	}
	assertRidges("scrolled")
	page.MustEval(`() => { document.getElementById('chat-box').scrollTop = 0; __step(2); }`)

	// A storm that breaks over a worm does not abort it: the event runs to its end,
	// faded by the dust, the heat shimmer is off meanwhile, and nothing leaks. Neither does
	// a resize in mid-breach (the footer or the chat box changed): the worm runs on.
	storm := page.MustEval(`() => {
        const life = AuraGoSandstormLife, seen = [];
        let peak = 0, finite = true, maxSpray = 0, shimmerInStorm = 0;
        life.forceWorm({ layer: 1, dir: 1, x: 500 });
        __step(160);
        const vulturesBefore = life.stats().vultures;
        __sand.forceStorm(window.__t);
        __step(720, (s, i) => {
            if (i === 100) life.resize(__sand.lifeGeom());
            const storm = __sand.stats().stormIntensity;
            if (s.worm && seen[seen.length - 1] !== s.worm) seen.push(s.worm);
            peak = Math.max(peak, storm);
            if (storm > 0.7) shimmerInStorm = Math.max(shimmerInStorm, s.shimmer);
            maxSpray = Math.max(maxSpray, s.spray);
            finite = finite && s.finite;
        });
        return { seen, peak, finite, maxSpray, shimmerInStorm, vulturesBefore, last: life.stats(), pending: __pending.size, errors: __errors };
    }`).Map()
	t.Logf("storm over a worm: %v", storm)
	if storm["vulturesBefore"].Int() < 2 || storm["last"].Map()["vultures"].Int() != 0 {
		t.Errorf("the vultures must circle until the storm breaks and be gone after it: %v before, %v after", storm["vulturesBefore"], storm["last"].Map()["vultures"])
	}
	if storm["peak"].Num() < 0.9 || fmt.Sprint(storm["seen"].Arr()) != "[tremor breach settle]" || storm["last"].Map()["wormCount"].Int() != 2 || !storm["last"].Map()["worm"].Nil() {
		t.Errorf("a storm must neither abort nor restart a running worm: %v", storm)
	}
	if storm["shimmerInStorm"].Num() != 0 {
		t.Errorf("the heat shimmer must be off in a storm: %v", storm["shimmerInStorm"])
	}
	if !storm["finite"].Bool() || storm["maxSpray"].Int() > 320 || storm["pending"].Int() != 1 || len(storm["errors"].Arr()) > 0 {
		t.Errorf("storm run broke a contract: %v", storm)
	}

	// The critters on the ridges. A reset starts the schedule over (the first random spawn is 15 s
	// away), so only the forced ones exist and the counts are exact. Each force function starts its
	// creature with the next update and refuses a second one once the family is at its maximum.
	critters := page.MustEval(`() => {
        const life = AuraGoSandstormLife;
        __sand.calm();
        life.reset(window.__t);
        __step(2);
        const forced = {
            weed: [life.forceTumbleweed(), life.forceTumbleweed(), life.forceTumbleweed()],
            beetle: [life.forceBeetle(), life.forceBeetle()],
            caravan: [life.forceCaravan(), life.forceCaravan()]
        };
        __step(2);
        const spawned = life.stats();
        const x1 = spawned.tumbleweedX;
        __step(30);                                     // 500 ms later
        const x2 = life.stats().tumbleweedX;
        const refused = { weed: life.forceTumbleweed(), beetle: life.forceBeetle(), caravan: life.forceCaravan() };
        let finite = true, burrowed = -1, dissolved = -1;
        __step(60, s => { finite = finite && s.finite; });
        const calm = life.stats();                      // a calm second later they are all still there
        __sand.forceStorm(window.__t);
        __step(180, (s, i) => {                         // 3 s of storm
            finite = finite && s.finite;
            if (burrowed < 0 && s.beetles === 0) burrowed = i / 60;
            if (dissolved < 0 && !s.caravan) dissolved = i / 60;
        });
        const stormy = life.stats();
        __sand.calm();                                  // leave the storm behind for the blocks that follow
        __step(2);
        return { forced, spawned, x1, x2, refused, calm, burrowed, dissolved, stormy, finite, pending: __pending.size, errors: __errors,
                 valid: life.stats().finite };
    }`).Map()
	t.Logf("critters: %v", critters)
	forced := critters["forced"].Map()
	if fmt.Sprint(forced["weed"].Arr()) != "[true true false]" || fmt.Sprint(forced["beetle"].Arr()) != "[true false]" || fmt.Sprint(forced["caravan"].Arr()) != "[true false]" {
		t.Errorf("the force functions return true until their family is at its maximum (2 balls, 1 beetle, 1 caravan): %v", forced)
	}
	spawned := critters["spawned"].Map()
	if spawned["tumbleweeds"].Int() != 2 || spawned["beetles"].Int() != 1 || !spawned["caravan"].Bool() {
		t.Errorf("forced critters appear with the next update: %v", spawned)
	}
	if dx := critters["x2"].Num() - critters["x1"].Num(); dx < 8 && dx > -8 {
		t.Errorf("a tumbleweed rolls: x went from %v to %v in 500 ms", critters["x1"], critters["x2"])
	}
	if refused := critters["refused"].Map(); refused["weed"].Bool() || refused["beetle"].Bool() || refused["caravan"].Bool() {
		t.Errorf("a family at its maximum refuses another one: %v", refused)
	}
	calm := critters["calm"].Map()
	if calm["tumbleweeds"].Int() < 1 || calm["beetles"].Int() != 1 || !calm["caravan"].Bool() {
		t.Errorf("in calm weather the beetle and the caravan stay: %v", calm)
	}
	if b, d := critters["burrowed"].Num(), critters["dissolved"].Num(); b <= 0 || b > 3 || d <= 0 || d > 3 {
		t.Errorf("a storm makes the beetle burrow and dissolves the caravan within 3 s: burrowed after %v s, caravan gone after %v s", critters["burrowed"], critters["dissolved"])
	}
	if stormy := critters["stormy"].Map(); stormy["beetles"].Int() != 0 || stormy["caravan"].Bool() {
		t.Errorf("beetle and caravan are gone at the end of the storm window: %v", stormy)
	}
	if !critters["finite"].Bool() || !critters["valid"].Bool() || critters["pending"].Int() != 1 || len(critters["errors"].Arr()) > 0 {
		t.Errorf("the critters broke a contract (finite state, one animation loop, no page errors): %v", critters)
	}

	// A resize clears the sand in the air, not the creatures. On a canvas that got narrower none of them
	// is left outside it: the worm is aimed again inside (the other way round when there is no room), the
	// beetle is pulled in, a ball or a line of camels heading left that now starts beyond the right edge is
	// put back at it. (One heading right is dropped by its next update once it is 40 px past the edge.)
	narrow := page.MustEval(`() => {
        const life = AuraGoSandstormLife, g = __sand.lifeGeom(), width = g.width;
        __sand.calm();
        life.reset(window.__t);
        __step(2);
        life.forceWorm({ layer: 1, dir: 1, x: 900 });
        __step(250);                                    // the breach has begun (frame 204)
        life.forceBeetle({ x: 1100 });
        life.forceTumbleweed({ x: 1150, dir: -1 });
        life.forceCaravan({ x: 1100, dir: -1 });
        __step(3);
        const before = life.stats();
        g.width = 500;                                  // the chat got narrower
        life.resize(g);
        const at = life.debug.where(), after = life.stats();
        g.width = width;
        life.reset(window.__t);
        __step(2);
        return { before, after, worm: at.worm, beetle: at.beetle, tumbleweed: at.tumbleweed, caravan: at.caravan, errors: __errors };
    }`).Map()
	t.Logf("resize to a narrower canvas: %v", narrow)
	narrowAt := func(key string, i int) float64 {
		if a := narrow[key].Arr(); len(a) > i {
			return a[i].Num()
		}
		return math.NaN()
	}
	narrowBefore, narrowAfter := narrow["before"].Map(), narrow["after"].Map()
	if narrowBefore["worm"].Str() != "breach" || narrowAfter["worm"].Str() != "breach" || narrowAfter["tumbleweeds"].Int() != 1 || narrowAfter["beetles"].Int() != 1 || !narrowAfter["caravan"].Bool() || len(narrow["errors"].Arr()) > 0 {
		t.Errorf("a resize must leave the creatures alive (a worm in mid-breach, a ball, the beetle, the caravan): %v", narrow)
	}
	if x0, x1 := narrowAt("worm", 0), narrowAt("worm", 1); !(x0 >= 24 && x0 <= 476 && x1 >= 24 && x1 <= 476 && math.Abs(x1-x0) >= 60) {
		t.Errorf("the worm must be aimed again inside a canvas of 500 px, with room for its arch: entry %v, dive %v", x0, x1)
	}
	if x, ball, lead := narrowAt("beetle", 0), narrowAt("tumbleweed", 0), narrowAt("caravan", 0); !(x <= 0.94*500+0.01 && ball <= 530.01 && lead <= 500.01) {
		t.Errorf("nothing may be left beyond the edge of a canvas of 500 px: beetle at %v (want 470 or less), ball at %v (530), caravan leader at %v (500)", x, ball, lead)
	}

	// The scheduled tumbleweed keeps out of storms like the worm, the beetle and the caravan: a ball that
	// comes due while it storms waits (the one a storm throws as it breaks is another matter), and starts
	// within 2 s of the calm.
	stormBall := page.MustEval(`() => {
        const life = AuraGoSandstormLife;
        __sand.calm();
        __sand.forceStorm(window.__t);
        __step(30);                                     // the storm has broken
        life.reset(window.__t - 40000);                 // the first ball was due 5-25 s ago ...
        __step(90);                                     // ... and the storm is still on (it lasts 9 s)
        const duringStorm = life.stats().tumbleweeds, stormy = __sand.stats().stormIntensity;
        __sand.calm();
        __step(180);
        const afterStorm = life.stats().tumbleweeds;
        life.reset(window.__t);
        __step(2);
        return { duringStorm, stormy, afterStorm, errors: __errors };
    }`).Map()
	t.Logf("tumbleweed schedule in a storm: %v", stormBall)
	if stormBall["stormy"].Num() <= 0 || stormBall["duringStorm"].Int() != 0 || stormBall["afterStorm"].Int() < 1 || len(stormBall["errors"].Arr()) > 0 {
		t.Errorf("a scheduled tumbleweed must wait while it storms and start soon after: %v", stormBall)
	}

	// Worm placement: the worm is drawn behind the chat, so it picks its spot with the chat
	// bubbles in mind. The fixture has no #chat-content, so the test builds one (never shown, with
	// a bubble whose rectangle covers the left half of the scene) and draws 24 worms twice. Every
	// trial is seeded alike in both runs, so the candidates are identical and only the scoring can
	// make a difference: with the bubble the entry and dive points must end up behind it far less
	// often. 80 % of the worms rise from the mid ridge.
	placement := page.MustEval(`() => {
        const life = AuraGoSandstormLife, g = __sand.lifeGeom();
        const wrap = document.createElement('div'), row = document.createElement('div'), bubble = document.createElement('div');
        wrap.id = 'chat-content';
        wrap.style.display = 'none';
        row.className = 'msg-row';
        bubble.className = 'bubble';
        const o = g.canvas.getBoundingClientRect();
        bubble.getBoundingClientRect = () => ({ left: o.left, top: o.top, right: o.left + g.width / 2, bottom: o.top + g.height,
                                                 width: g.width / 2, height: g.height });
        row.appendChild(bubble);
        wrap.appendChild(row);
        document.body.appendChild(wrap);
        const half = g.width / 2, original = Math.random;
        const mulberry = a => () => {
            a = a + 0x6D2B79F5 | 0;
            let t = Math.imul(a ^ a >>> 15, 1 | a);
            t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
            return ((t ^ t >>> 14) >>> 0) / 4294967296;
        };
        const run = () => {
            let behind = 0, mid = 0, seen = 0;
            for (let i = 0; i < 24; i++) {
                life.reset(window.__t);
                __step(1);
                Math.random = mulberry(1000 + i);
                life.forceWorm();
                Math.random = original;
                const at = life.debug.where(), w = at.worm;
                behind += (w[0] < half ? 1 : 0) + (w[1] < half ? 1 : 0);     // entry and dive point behind the bubble
                if (w[2] === 1) mid++;
                seen = at.occluders;
            }
            return { behind, mid, seen };
        };
        const withBubble = run();
        wrap.remove();
        const without = run();
        life.reset(window.__t);
        __step(1);
        return { withBubble, without, errors: __errors, finite: life.stats().finite };
    }`).Map()
	t.Logf("worm placement: %v", placement)
	withBubble, without := placement["withBubble"].Map(), placement["without"].Map()
	if withBubble["seen"].Int() != 1 || without["seen"].Int() != 0 {
		t.Errorf("the placement must find the bubble (and nothing without it): %v and %v rectangles", withBubble["seen"], without["seen"])
	}
	if without["behind"].Int()-withBubble["behind"].Int() < 6 {
		t.Errorf("worms must keep out from behind chat bubbles: %v of 48 entry and dive points behind a bubble over the left half, %v without it", withBubble["behind"], without["behind"])
	}
	if withBubble["mid"].Int() != without["mid"].Int() || withBubble["mid"].Int() < 14 {
		t.Errorf("about 80 %% of the worms rise from the mid ridge: %v and %v of 24", withBubble["mid"], without["mid"])
	}
	if !placement["finite"].Bool() || len(placement["errors"].Arr()) > 0 {
		t.Errorf("placement broke a contract: %v", placement)
	}

	// Vulture placement: the thermal the birds circle is chosen with the chat in mind as well. The
	// same kind of never-shown fake chat as above, with a bubble that spans the whole height of the
	// scene (so it covers the sky band), in five situations: (1) over the left half, where the default
	// spot beside the sun is free anyway, so the flock must stay in the right half; (2) none at all, so
	// the next start is back at the default spot; (3) over the middle, covering the default spot: the
	// flock must move out from behind it; (4) the same bubble appears over a flock that circles already:
	// 15 s after it glided in the thermal drifts out from behind it, at most 24 px/s; (5) a bubble whose
	// edge stands just beyond the reach of a flock at the default spot: the sway (0.02 width to and fro,
	// a period of 90 s) would carry the outer birds under it, so the flock must not stay there, and for
	// 100 s of flight no bird may go behind the bubble. A pick tries eight random spots, so (3) runs
	// eight seeded trials and allows a rare one to keep a bird half behind the bubble. The chat is read
	// once per glide-in and once per look, never per frame (the fake bubble counts the reads). Only (1)
	// runs engine frames; the rest drives the companion's update alone, which is cheap.
	vulturePlacement := page.MustEval(`() => {
        const life = AuraGoSandstormLife, g = __sand.lifeGeom(), original = Math.random;
        const mulberry = a => () => {
            a = a + 0x6D2B79F5 | 0;
            let t = Math.imul(a ^ a >>> 15, 1 | a);
            t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t;
            return ((t ^ t >>> 14) >>> 0) / 4294967296;
        };
        const fakeChat = (x0, x1) => {
            const wrap = document.createElement('div'), row = document.createElement('div'), bubble = document.createElement('div');
            wrap.id = 'chat-content';
            wrap.style.display = 'none';
            row.className = 'msg-row';
            bubble.className = 'bubble';
            wrap.reads = 0;                             // how often the companion looked at this bubble
            bubble.getBoundingClientRect = () => {
                wrap.reads++;
                const o = g.canvas.getBoundingClientRect();
                return { left: o.left + x0, top: o.top, right: o.left + x1, bottom: o.top + g.height, width: x1 - x0, height: g.height };
            };
            row.appendChild(bubble);
            wrap.appendChild(row);
            document.body.appendChild(wrap);
            return wrap;
        };
        const look = () => {
            const at = life.debug.where();
            return { thermalX: at.thermal ? at.thermal[0] : -1, birdsX: (at.vultures || []).map(b => b[0]), occluders: at.occluders };
        };
        // frames for the companion alone (an engine frame paints the whole scene, about 3 ms each, and the
        // thermal is the companion's business). The engine advances weatherTime, in seconds, by dt / 60 before
        // every update and the sway of the thermal is a function of it, so this helper does the same.
        const fly = n => { const dt = 1; for (let i = 0; i < n; i++) { window.__t += 1000 / 60; g.weatherTime += dt / 60; life.update(dt, window.__t, g); } };
        __sand.calm();
        const out = { width: g.width, height: g.height, defaultX: g.sunX - 0.16 * g.width };

        let wrap = fakeChat(0, g.width / 2);
        life.reset(window.__t);
        __step(270);                                    // 4.5 s of calm, engine and all: every bird has glided in
        out.left = look();
        wrap.remove();

        life.reset(window.__t);
        fly(270);
        out.plain = look();

        const x0 = 0.44 * g.width, x1 = 0.9 * g.width;
        wrap = fakeChat(x0, x1);
        out.trials = [];
        for (let seed = 1; seed <= 8; seed++) {
            Math.random = mulberry(seed);               // seeded for the first update only: that is where the spot is picked
            life.reset(window.__t);
            fly(1);
            Math.random = original;
            fly(200);                                   // gliding in takes 3 s = 180 frames
            const at = look();
            at.behind = at.birdsX.filter(x => x > x0 + 8 && x < x1 - 8).length;   // well inside the bubble
            out.trials.push(at);
        }
        out.trialReads = wrap.reads;
        wrap.remove();

        life.reset(window.__t);
        fly(270);                                       // circling at the default spot, nothing in the way
        wrap = fakeChat(x0, x1);                        // now the bubble appears over them
        let [px, py] = life.debug.where().thermal;
        const drift = { startX: px, maxStep: 0, firstMove: -1 };
        Math.random = mulberry(77);
        for (let i = 1; i <= 2700; i++) {               // 45 s, one frame at a time: the longest drift ends 39 s in
            fly(1);
            const [tx, ty] = life.debug.where().thermal, step = Math.hypot(tx - px, ty - py);
            drift.maxStep = Math.max(drift.maxStep, step);
            if (drift.firstMove < 0 && step > 1e-6) drift.firstMove = i / 60;
            px = tx;
            py = ty;
        }
        Math.random = original;
        Object.assign(drift, look(), { endX: px, reads: wrap.reads });
        drift.behind = drift.birdsX.filter(x => x > x0 + 8 && x < x1 - 8).length;
        out.drift = drift;
        wrap.remove();

        // (5) The sway. The bubble stands from the start, its left edge 4 px beyond the reach of the biggest
        // ellipse (0.1 width) of a flock at the default spot. A pick that scores the moment of the look only
        // (12 points per ellipse, 30 degrees apart from 15 degrees on, and a sway that is 0 just then) sees the
        // flock clear of it and keeps it there; a quarter of a sway period later the sway has carried the outer
        // birds 0.02 width to the right, under the edge. So the flock must leave the default spot, and for
        // 100 s (more than one period) no bird may be behind the bubble. The birds' orbits and the pick are
        // seeded: with seed 16 the moment-only pick puts a bird under the edge within 7 s.
        const edge = out.defaultX + 0.1 * g.width + 4;
        Math.random = mulberry(16);
        g.weatherTime = 0;
        wrap = fakeChat(edge, x1);
        life.reset(window.__t);
        fly(270);                                       // the pick is made in the first frame, gliding in takes 3 s
        const sway = Object.assign({ edge, samples: 0, hidden: 0, minBirds: 99, minClear: 1e9 }, look());
        const swayFrom = g.weatherTime;
        for (let i = 1; i <= 6000; i++) {               // 100 s, looked at once a second
            fly(1);
            if (i % 60) continue;
            const birds = life.debug.where().vultures;
            sway.samples++;
            sway.minBirds = Math.min(sway.minBirds, birds.length);
            for (const [bx] of birds) {
                if (bx >= edge && bx <= x1) sway.hidden++;
                sway.minClear = Math.min(sway.minClear, bx < edge ? edge - bx : (bx > x1 ? bx - x1 : -Math.min(bx - edge, x1 - bx)));
            }
        }
        sway.seconds = g.weatherTime - swayFrom;
        Math.random = original;
        out.sway = sway;
        wrap.remove();

        life.reset(window.__t);
        __step(1);
        out.errors = __errors;
        out.finite = life.stats().finite;
        return out;
    }`).Map()
	t.Logf("vulture placement: %v", vulturePlacement)
	vWidth, vDefault := vulturePlacement["width"].Num(), vulturePlacement["defaultX"].Num()
	vLeft, vPlain := vulturePlacement["left"].Map(), vulturePlacement["plain"].Map()
	if vLeft["occluders"].Int() != 1 || vPlain["occluders"].Int() != 0 {
		t.Errorf("the thermal placement must find the bubble (and nothing without it): %v and %v rectangles", vLeft["occluders"], vPlain["occluders"])
	}
	if len(vLeft["birdsX"].Arr()) < 2 || len(vPlain["birdsX"].Arr()) < 2 {
		t.Errorf("after 4.5 s of calm at least two vultures must circle in view: %v and %v", vLeft["birdsX"], vPlain["birdsX"])
	}
	if x := vLeft["thermalX"].Num(); x <= vWidth/2 {
		t.Errorf("with a bubble over the left half the thermal must lie in the right half: x=%v of %v", x, vWidth)
	}
	for _, bird := range vLeft["birdsX"].Arr() {
		if x := bird.Num(); x <= vWidth/2-20 {
			t.Errorf("a vulture circles behind the bubble over the left half: x=%v of %v", x, vWidth)
		}
	}
	if x := vPlain["thermalX"].Num(); math.Abs(x-vDefault) >= 0.05*vWidth {
		t.Errorf("without a bubble the thermal sits at its default spot beside the sun: x=%v, default %v", x, vDefault)
	}
	moved, clear := 0, 0
	for _, trial := range vulturePlacement["trials"].Arr() {
		if math.Abs(trial.Get("thermalX").Num()-vDefault) > 0.1*vWidth {
			moved++
		}
		if len(trial.Get("birdsX").Arr()) >= 2 && trial.Get("behind").Int() == 0 {
			clear++
		}
	}
	if moved < 6 || clear < 6 {
		t.Errorf("a bubble over the default spot must move the thermal out from behind it (%d of 8 trials moved, want 6+) and leave the birds in the clear (%d of 8, want 6+): %v", moved, clear, vulturePlacement["trials"])
	}
	// (4): the first look is 15 s after the glide-in, so 10.5 s after the bubble appeared; the drift is
	// capped at 24 px/s = 0.4 px per frame and ends in the clear.
	drift := vulturePlacement["drift"].Map()
	if first := drift["firstMove"].Num(); first < 8 || first > 15 {
		t.Errorf("the flock looks at the chat again 15 s after it glided in (10.5 s after the bubble appeared): first move after %v s", drift["firstMove"])
	}
	if step := drift["maxStep"].Num(); step > 24.0/60*1.05 {
		t.Errorf("the thermal drifts at most 24 px/s (0.4 px per frame): moved %v px in one frame", step)
	}
	if math.Abs(drift["endX"].Num()-drift["startX"].Num()) < 0.2*vWidth || len(drift["birdsX"].Arr()) < 2 || drift["behind"].Int() != 0 {
		t.Errorf("a bubble that appears over circling vultures must send their thermal out from behind it: %v", drift)
	}
	if drift["reads"].Int() > 12 || vulturePlacement["trialReads"].Int() > 16 {
		t.Errorf("the chat is read once per glide-in and every 15 s, not per frame: %v reads in 45 s of flight, %v for eight glide-ins", drift["reads"], vulturePlacement["trialReads"])
	}
	// (5): the flock leaves the spot whose bubble edge only the sway reaches, and stays in the clear.
	sway := vulturePlacement["sway"].Map()
	if sway["occluders"].Int() != 1 || math.Abs(sway["thermalX"].Num()-vDefault) < 0.05*vWidth {
		t.Errorf("a spot the sway carries under a bubble edge is no spot: the thermal must leave the default x=%v (edge at %v) and find the bubble: %v", vDefault, sway["edge"], sway)
	}
	if sway["samples"].Int() != 100 || sway["seconds"].Num() < 99.9 || sway["minBirds"].Int() < 2 || sway["hidden"].Int() != 0 {
		t.Errorf("for 100 s of sway every vulture must stay clear of the bubble (%v of %v looks found one behind it, nearest %v px): %v", sway["hidden"], sway["samples"], sway["minClear"], sway)
	}
	if !vulturePlacement["finite"].Bool() || len(vulturePlacement["errors"].Arr()) > 0 {
		t.Errorf("vulture placement broke a contract: %v", vulturePlacement)
	}

	// The schedule: a fresh run's first worm is due 28-50 s after the start but waits while a
	// storm is due within 9 s (the engine's calmLeft), then starts on its own.
	gate := page.MustEval(`() => {
        const life = AuraGoSandstormLife;
        life.reset(window.__t);
        __step(1);
        const jump = window.__t + 60000;
        __sand.setNextStorm(jump + 5000);
        window.__t = jump;
        __frame(jump);
        const waiting = life.stats().worm;
        __sand.setNextStorm(1e12);
        __step(130);
        return { waiting, started: life.stats().worm, refused: life.forceWorm(), calmLeft: __sand.lifeGeom().calmLeft };
    }`).Map()
	if !gate["waiting"].Nil() || gate["started"].Str() != "tremor" || gate["refused"].Bool() || gate["calmLeft"].Num() < 9000 {
		t.Errorf("the worm must wait for a calm stretch and then start on its own: %v", gate)
	}

	// Leaving the theme stops the engine and empties the companion (the natural worm above
	// is still running); coming back starts clean.
	page.MustEval(`() => {
        __step(30);
        document.documentElement.dataset.theme = 'dark';
        AuraGoSandstorm.sync();
    }`)
	off := page.MustEval(`() => ({ engine: __sand.stats().active, pending: __pending.size, life: AuraGoSandstormLife.stats() })`).Map()
	offLife := off["life"].Map()
	if off["engine"].Bool() || off["pending"].Int() != 0 || !offLife["worm"].Nil() || offLife["spray"].Int() != 0 || offLife["puffs"].Int() != 0 || offLife["wisps"].Int() != 0 || offLife["wormCount"].Int() != 0 {
		t.Errorf("theme exit must stop the loop and empty the companion: %v", off)
	}
	// (the restart schedules its first storm on the real clock, which the manual clock has left far behind)
	page.MustEval(`() => { document.documentElement.dataset.theme = 'sandstorm'; AuraGoSandstorm.sync(); AuraGoSandstorm.sync(); __sand.calm(); __step(5); }`)
	if !page.MustEval(`() => __sand.stats().active && __pending.size === 1 && AuraGoSandstormLife.stats().finite && document.querySelectorAll('script[data-aurago-sandstorm-life]').length === 1`).Bool() {
		t.Error("theme restart must resume with one loop and without loading the companion again")
	}

	// Screenshots for a visual review go first: opening more tabs below hides this one, which stops the engine.
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		captureSandstormLifeArtifacts(t, page, dir)
	}

	// The shared lazy loader: its first attempt fails, the engine keeps running without the
	// companion, and the next theme start asks again. This time the load succeeds, with the plain
	// path (the loader versions the URL itself) and no marker element left behind.
	lazy := openPage("fallback&lazy")
	defer lazy.Close()
	waitFor(lazy, `() => __lazyCalls.length > 0 && __sand.stats().active`, "the engine never asked for the companion")
	lazy.MustEval(sandstormLifeManualClock)
	// (a failed script element left in the page would make every later attempt wait for ever)
	waitFor(lazy, `() => document.querySelectorAll('script[src*="sandstorm-life.js"]').length === 0`, "the failed script element was not removed")
	failed := lazy.MustEval(`() => {
        const calls = __lazyCalls.length;
        __step(20);
        const running = __sand.stats().active && __pending.size === 1;
        const attached = !!window.AuraGoSandstormLife;
        window.__lazyFail = false;
        document.documentElement.dataset.theme = 'dark';
        AuraGoSandstorm.sync();
        document.documentElement.dataset.theme = 'sandstorm';
        AuraGoSandstorm.sync();
        return { calls, retried: __lazyCalls.length, running, attached, errors: __errors };
    }`).Map()
	if failed["attached"].Bool() || !failed["running"].Bool() || failed["retried"].Int() <= failed["calls"].Int() || len(failed["errors"].Arr()) > 0 {
		t.Errorf("a failed companion load must leave the engine running and be retried by the next start: %v", failed)
	}
	waitFor(lazy, `() => !!window.AuraGoSandstormLife`, "the retried load never attached")
	retried := lazy.MustEval(`() => {
        __sand.calm();
        __step(20);
        return { path: __lazyCalls[__lazyCalls.length - 1], marked: document.querySelectorAll('script[data-aurago-sandstorm-life]').length,
                 pending: __pending.size, errors: __errors, finite: AuraGoSandstormLife.stats().finite };
    }`).Map()
	if retried["path"].Str() != "/js/chat/sandstorm-life.js" || retried["marked"].Int() != 0 || retried["pending"].Int() != 1 || len(retried["errors"].Arr()) > 0 || !retried["finite"].Bool() {
		t.Errorf("lazy loader contract: %v", retried)
	}
}

// captureSandstormLifeArtifacts saves screenshots along one worm event (and the same
// frames with the ridge overlay), then one scene each for the vultures, a hopping
// tumbleweed, the beetle, the caravan and a calm gust, for a visual review.
func captureSandstormLifeArtifacts(t *testing.T, page *rod.Page, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// right of the chat bubbles, so the whole worm is in the clear
	page.MustEval(`() => { __sand.calm(); AuraGoSandstormLife.forceWorm({ layer: 1, dir: 1, x: 880 }); }`)
	frame := 0
	shot := func(name string, at int, ridges bool) {
		t.Helper()
		page.MustEval(`(n, ridges) => { AuraGoSandstormLife.debug.ridges = ridges; __step(n); }`, at-frame, ridges)
		frame = at
		t.Logf("artifact %s: %v", name, page.MustEval(`() => AuraGoSandstormLife.stats()`))
		if err := os.WriteFile(filepath.Join(dir, "sandstorm-life-"+name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []struct {
		name string
		at   int
	}{
		{"01-tremor", 110}, {"02-tremor-late", 170}, {"03-breach-start", 212}, {"04-maw-opening", 226}, {"05-rise", 244},
		{"06-maw-open", 262}, {"07-apex", 285}, {"08-arc", 325}, {"09-dive", 380}, {"10-tail", 435}, {"11-settle", 520},
	} {
		shot(s.name, s.at, false)
	}
	shot("12-ridges", 521, true)

	// a strong gust: sand streaks and banners off the crests
	page.MustEval(`() => {
        AuraGoSandstormLife.debug.ridges = false;
        const box = document.getElementById('chat-box'), r = box.getBoundingClientRect();
        box.dispatchEvent(new PointerEvent('pointermove', {clientX: r.x + 200, clientY: r.y + 200, bubbles: true}));
        box.dispatchEvent(new PointerEvent('pointermove', {clientX: r.x + 262, clientY: r.y + 188, bubbles: true}));
        __step(6);
    }`)
	t.Logf("artifact 13-gust: %v", page.MustEval(`() => AuraGoSandstormLife.stats()`))
	if err := os.WriteFile(filepath.Join(dir, "sandstorm-life-13-gust.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}

	// the far worm: smaller, rising behind the mid dune
	page.MustEval(`() => { __step(200); AuraGoSandstormLife.forceWorm({ layer: 0, dir: -1, x: 1000 }); __step(285); }`)
	t.Logf("artifact 14-far-worm: %v", page.MustEval(`() => AuraGoSandstormLife.stats()`))
	if err := os.WriteFile(filepath.Join(dir, "sandstorm-life-14-far-worm.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}

	// The critters, one clean scene each: a reset clears the others and restarts the schedule.
	// where() says where the first of each family stands, for a zoomed crop of the screenshot.
	snap := func(name string) {
		t.Helper()
		t.Logf("artifact %s: canvas at %v, where=%v stats=%v", name,
			page.MustEval(`() => { const r = document.getElementById('sandstorm-scene').getBoundingClientRect(); return [r.left, r.top]; }`),
			page.MustEval(`() => AuraGoSandstormLife.debug.where()`), page.MustEval(`() => AuraGoSandstormLife.stats()`))
		if err := os.WriteFile(filepath.Join(dir, "sandstorm-life-"+name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	scene := func(script string) {
		t.Helper()
		page.MustEval(`(script) => { __sand.calm(); AuraGoSandstormLife.debug.ridges = false; AuraGoSandstormLife.reset(window.__t); __step(2); new Function(script)(); }`, script)
	}
	scene(`__step(330);`)
	snap("15-vultures")
	scene(`AuraGoSandstormLife.forceTumbleweed({ dir: -1, x: 1250 }); __step(70);`)
	snap("16-tumbleweed-a")
	page.MustEval(`() => __step(11)`)
	snap("17-tumbleweed-b")
	page.MustEval(`() => __step(11)`)
	snap("18-tumbleweed-c")
	scene(`AuraGoSandstormLife.forceBeetle({ x: 1130, dir: -1 }); __step(80);`)
	snap("19-beetle-a")
	page.MustEval(`() => __step(26)`)
	snap("20-beetle-b")
	scene(`AuraGoSandstormLife.forceCaravan({ x: 1020, dir: 1 }); __step(220);`)
	snap("21-caravan")
	// a calm gust: wait for the engine's own wind to peak, no pointer involved
	scene(`for (let i = 0; i < 1500; i++) { __step(1); if (__sand.stats().windSpeed > 2.2 && AuraGoSandstormLife.stats().wisps >= 20) break; }`)
	t.Logf("artifact 22-calm-gust: wind=%v", page.MustEval(`() => __sand.stats().windSpeed`))
	snap("22-calm-gust")
}
