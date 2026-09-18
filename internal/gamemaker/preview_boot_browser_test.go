package gamemaker

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// A hidden Studio window is not a broken game. Only a persistently invalid
// canvas inside an active, sized preview should fail the startup check.
func TestPreviewBootLayoutBrowser(t *testing.T) {
	if os.Getenv("GAMEMAKER_PREVIEW_BROWSER") != "1" {
		t.Skip("set GAMEMAKER_PREVIEW_BROWSER=1 for preview layout checks")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		mode := r.URL.Query().Get("mode")
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
			page := fmt.Sprintf(`<!doctype html><style>html,body,#game-root{margin:0;width:100%%;height:100%%}canvas{width:100%%;height:100%%}%s</style><div id="hud">Loading...</div><div id="game-root"><canvas width="640" height="360"></canvas></div><script>document.querySelector('canvas').getContext('2d').fillRect(0,0,640,360);%s</script>`, style, script)
			w.Write(injectPreviewBoot([]byte(page)))
			return
		}
		fmt.Fprintf(w, `<!doctype html><style>body{margin:0}iframe{border:0;width:640px;height:360px}</style><script>
window.reports=[];addEventListener('message',e=>{if(e.source===frame.contentWindow&&e.data?.source==='aurago-game')reports.push(e.data)});
function active(value){frame.contentWindow.postMessage({type:'aurago:game:active',active:value},'*')}
</script><iframe id="frame" sandbox="allow-scripts" src="/game?mode=%s#gm-channel=layout"></iframe><script>
const mode=%q;if(mode==='inactive')frame.style.display='none';if(mode==='collapsed')frame.style.height='0px';
frame.addEventListener('load',()=>active(mode!=='inactive'));
</script>`, mode, mode)
	}))
	defer server.Close()
	bin := os.Getenv("CHROME_BIN")
	if bin == "" {
		bin = "C:/Program Files/Google/Chrome/Application/chrome.exe"
	}
	l := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(l.MustLaunch()).MustConnect()
	defer l.Cleanup()
	defer browser.Close()
	for _, mode := range []string{"inactive", "collapsed", "late", "visible", "hud", "hidden", "outside", "ancestor"} {
		t.Run(mode, func(t *testing.T) {
			page := browser.MustPage("about:blank").Timeout(15 * time.Second)
			defer page.Close()
			page.MustNavigate(server.URL + "/?mode=" + mode).MustWaitLoad()
			if mode == "inactive" || mode == "collapsed" {
				page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,4200))`)
				if got := page.MustEval(`()=>reports`).String(); got != "[]" {
					t.Fatalf("inactive/unsized preview emitted a startup verdict: %s", got)
				}
				page.MustEval(`()=>{frame.style.display='block';frame.style.height='360px';active(true)}`)
			}
			if mode == "hud" {
				frame := page.MustElement("iframe").MustFrame()
				frame.MustWait(`()=>document.getElementById("hud").textContent==='Lives 3'&&!document.getElementById("hud").hidden&&getComputedStyle(document.getElementById("hud")).display!=='none'`)
			}
			invalid := mode == "hidden" || mode == "outside" || mode == "ancestor"
			page.MustWait(`()=>reports.some(r=>r.type==='ready'||r.type==='runtime_error')`)
			if invalid {
				page.MustEval(`()=>new Promise(resolve=>setTimeout(resolve,1200))`)
				if !page.MustEval(`()=>reports.filter(r=>r.type==='runtime_error').length===1&&!reports.some(r=>r.type==='ready')`).Bool() {
					t.Fatal("invalid game must fail once without reporting readiness", page.MustEval(`()=>reports`))
				}
				if msg := page.MustEval(`()=>reports.find(r=>r.type==='runtime_error').message`).Str(); !strings.Contains(msg, "viewport=") {
					t.Fatal("layout error lacks measured viewport evidence", msg)
				}
			} else if !page.MustEval(`()=>reports.some(r=>r.type==='ready'&&r.boot&&r.visible)&&!reports.some(r=>r.type==='runtime_error')`).Bool() {
				t.Fatal("valid game rejected during layout", page.MustEval(`()=>reports`))
			}
		})
	}
}
