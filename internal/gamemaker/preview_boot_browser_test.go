package gamemaker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Readiness describes a loaded document, never a verdict on its game or canvas.
func TestPreviewBootHTMLBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" && os.Getenv("GAMEMAKER_PREVIEW_BROWSER") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 for preview checks")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		mode := r.URL.Query().Get("mode")
		if r.URL.Path == "/missing.png" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/game" {
			style, script := "", ""
			switch mode {
			case "late", "hidden":
				style = "canvas{display:none}"
				if mode == "late" {
					script = "setTimeout(()=>document.querySelector('canvas').style.display='block',1800)"
				}
			case "outside":
				style = "canvas{position:absolute;left:200vw}"
			case "hud":
				script = "setTimeout(()=>document.getElementById('hud').textContent='Lives 3',900)"
			case "ancestor":
				style = "#game-root{opacity:0}"
			}
			game := `<canvas width="640" height="360"></canvas>`
			if mode == "html" || mode == "errors" {
				game = `<button onclick="this.textContent=String(++window.count)">Play</button>`
				script = "window.count=0;"
			}
			if mode == "errors" {
				game += `<img src="/missing.png">`
				script += `console.error('engine feedback');Promise.reject(new Error('promise feedback'));setTimeout(()=>{throw new Error('runtime feedback')},0);`
			}
			page := fmt.Sprintf(`<!doctype html><html><head><style>html,body,#game-root{margin:0;width:100%%;height:100%%}canvas{width:100%%;height:100%%}%s</style></head><body><div id="hud">Loading...</div><div id="game-root">%s</div><script>%s</script></body></html>`, style, game, script)
			w.Write(injectPreviewBoot([]byte(page)))
			return
		}
		fmt.Fprintf(w, `<!doctype html><style>body{margin:0}iframe{border:0;width:640px;height:360px}</style><script>
window.reports=[];addEventListener('message',e=>{if(e.source===frame.contentWindow&&e.data?.source==='aurago-game')reports.push(e.data)});
</script><iframe id="frame" sandbox="allow-scripts" src="/game?mode=%s#gm-channel=layout"></iframe><script>
const mode=%q;if(mode==='inactive')frame.style.display='none';if(mode==='collapsed')frame.style.height='0px';
</script>`, mode, mode)
	}))
	defer server.Close()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		var found bool
		bin, found = launcher.LookPath()
		if !found {
			t.Fatal("browser smoke requested but Chrome/Chromium is unavailable")
		}
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, mode := range []string{"inactive", "collapsed", "late", "visible", "hud", "hidden", "outside", "ancestor", "html", "errors"} {
		t.Run(mode, func(t *testing.T) {
			page := browser.MustPage("about:blank").Timeout(15 * time.Second)
			defer page.Close()
			page.MustNavigate(server.URL + "/?mode=" + mode).MustWaitLoad()
			page.MustWait(`()=>reports.some(r=>r.type==='ready')`)
			frame := page.MustElement("iframe").MustFrame()
			if mode == "late" {
				frame.MustWait(`()=>getComputedStyle(document.querySelector('canvas')).display==='block'`)
			}
			if mode == "hud" {
				frame.MustWait(`()=>document.getElementById("hud").textContent==='Lives 3'&&!document.getElementById("hud").hidden&&getComputedStyle(document.getElementById("hud")).display!=='none'`)
			}
			if mode == "html" || mode == "errors" {
				frame.MustElement("button").MustClick()
				if !frame.MustEval(`()=>window.count===1&&document.querySelector('button').textContent==='1'`).Bool() {
					t.Fatal("preview boot interfered with the running HTML game")
				}
			}
			if mode == "errors" {
				if err := page.Timeout(5 * time.Second).Wait(rod.Eval(`()=>reports.some(r=>r.type==='resource_error'&&r.message.includes('/missing.png'))&&['engine feedback','promise feedback','runtime feedback'].every(text=>reports.some(r=>r.type==='runtime_error'&&r.message.includes(text)))`)); err != nil {
					t.Fatal("preview missed browser feedback", page.MustEval(`()=>reports`), err)
				}
			} else if !page.MustEval(`()=>reports.every(r=>r.type==='ready')`).Bool() {
				t.Fatal("document readiness must not impose a canvas/layout verdict", page.MustEval(`()=>reports`))
			}
			if !page.MustEval(`()=>reports.filter(r=>r.type==='ready').length===1&&reports.every(r=>r.channel==='layout'&&r.message.length<=2000)`).Bool() {
				t.Fatal("preview feedback lost its channel, bound or single readiness", page.MustEval(`()=>reports`))
			}
			if !frame.MustEval(`()=>{try{void parent.document;return false}catch(e){return e.name==='SecurityError'}}`).Bool() {
				t.Fatal("interactive preview escaped opaque-origin isolation")
			}
		})
	}
}
