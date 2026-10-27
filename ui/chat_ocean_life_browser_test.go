package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestChatOceanSeaLifeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html data-theme="ocean"><head><meta charset="utf-8">
<link rel="stylesheet" href="/css/chat.bundle.css"><style>
body{height:100vh;overflow:hidden;display:flex;flex-direction:column;margin:0}
.app-header{position:relative;flex:0 0 64px;padding:0 32px}
#chat-box{flex:1;min-height:0;width:100%;padding:56px max(24px,calc((100vw - 820px)/2));box-sizing:border-box;overflow-y:auto}
.app-footer{position:relative;flex:0 0 110px}
.msg-row{display:flex;margin-bottom:36px}.bubble{padding:18px 24px;max-width:600px}.msg-row.user{justify-content:flex-end}
</style></head><body>
<script>
window.__errors=[];window.addEventListener('error',e=>__errors.push(e.message));
window.__auraOceanLifeDebug={firstWhaleDelay:2};
window.__reduce=location.search.includes('reduce');
const nativeMatchMedia=window.matchMedia.bind(window);
window.matchMedia=q=>q==='(prefers-reduced-motion: reduce)'?{get matches(){return window.__reduce},addEventListener(){}}:nativeMatchMedia(q);
</script>
<header class="app-header"><strong>AuraGo</strong></header>
<main id="chat-box"><div id="chat-content">
<div class="msg-row"><div class="bubble bot" id="first-bubble">Die Strömung trägt feinen Sand über das Riff.</div></div>
<div class="msg-row user"><div class="bubble user">Was schwimmt da hinten?</div></div>
<div class="msg-row"><div class="bubble bot">Ein Schwarm Sardinen zieht vorbei, dahinter ein Riffhai.</div></div>
</div></main>
<div id="agentStatusContainer" class="is-hidden"></div>
<footer class="app-footer"></footer>
<script src="/js/chat/ocean-shader.js"></script></body></html>`))
			return
		}
		http.FileServer(http.FS(Content)).ServeHTTP(w, r)
	}))
	defer server.Close()
	bin, ok := browserExecutable()
	if !ok {
		t.Skip("Chrome or Edge required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	launch := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	url := launch.MustLaunch()
	defer launch.Cleanup()
	browser := rod.New().Context(ctx).ControlURL(url).MustConnect()
	defer browser.Close()

	waitWithin := func(page *rod.Page, expr string, what string, limit time.Duration) {
		t.Helper()
		deadline := time.Now().Add(limit)
		for time.Now().Before(deadline) {
			if page.MustEval(expr).Bool() {
				return
			}
			time.Sleep(150 * time.Millisecond)
		}
		t.Fatalf("%s: %s", what, page.MustEval(`() => JSON.stringify({errors: window.__errors, stats: window.__auraOceanLife && __auraOceanLife.stats()})`).String())
	}
	waitFor := func(page *rod.Page, expr string, what string) {
		t.Helper()
		waitWithin(page, expr, what, 40*time.Second)
	}

	page := browser.MustPage("about:blank")
	page.MustSetViewport(1280, 800, 1, false)
	page.MustNavigate(server.URL).MustWaitLoad()
	waitFor(page, `() => !!window.__auraOceanLife && __auraOceanLife.stats().creatures > 0 && __auraOceanLife.stats().frames >= 5`, "sea life never rendered")
	result := page.MustEval(`() => {
        const canvas = document.getElementById('ocean-life-overlay');
        const box = document.getElementById('chat-box').getBoundingClientRect();
        const c = canvas.getBoundingClientRect();
        const bubble = document.getElementById('first-bubble');
        const b = bubble.getBoundingClientRect();
        const hit = document.elementFromPoint(b.left + b.width / 2, b.top + b.height / 2);
        const s = __auraOceanLife.stats();
        return {
            aligned: Math.abs(c.left - box.left) <= 1 && Math.abs(c.top - box.top) <= 1 && Math.abs(c.width - box.width) <= 1 && Math.abs(c.height - box.height) <= 1,
            behind: !!(hit && hit.closest('.bubble') === bubble),
            animated: s.animated, creatures: s.creatures, quality: s.quality,
            species: Object.keys(s.species || {}).length, errors: window.__errors
        };
    }`)
	if !result.Get("aligned").Bool() || !result.Get("behind").Bool() || !result.Get("animated").Bool() || result.Get("species").Int() < 5 {
		t.Fatalf("sea life layer misbehaves: %s", result.String())
	}
	if errs := result.Get("errors").Arr(); len(errs) > 0 {
		t.Fatalf("page errors: %v", errs)
	}
	// the debug seam above brings the first whale pass forward to t = 2 s
	waitWithin(page, `() => __auraOceanLife.stats().whaleVisible === true`, "whale never appeared", 20*time.Second)

	still := browser.MustPage("about:blank")
	still.MustSetViewport(1280, 800, 1, false)
	still.MustNavigate(server.URL + "/?reduce").MustWaitLoad()
	waitFor(still, `() => !!window.__auraOceanLife && __auraOceanLife.stats().creatures > 0 && __auraOceanLife.stats().animated === false`, "still frame never rendered")
	frames := still.MustEval(`() => __auraOceanLife.stats().frames`).Int()
	time.Sleep(600 * time.Millisecond)
	if again := still.MustEval(`() => __auraOceanLife.stats().frames`).Int(); again != frames {
		t.Fatalf("reduced motion must freeze the scene: %d -> %d frames", frames, again)
	}
	if frames < 1 {
		t.Fatal(fmt.Sprintf("still frame rendered %d frames", frames))
	}
}
