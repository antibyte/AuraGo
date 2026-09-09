package ui

import (
	"encoding/base64"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopLeafyGraphicsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><body style="margin:0;background:#182d32 url('/img/wallpapers/paper_waves.jpg') center/cover;height:100vh"><script>window.errors=[];addEventListener('error',e=>errors.push(e.message||e.target.src),true);</script><canvas id="plant" style="width:100%;height:100%;position:fixed"></canvas><script src="/js/vendor/three.min.js"></script><script src="/js/desktop/leafy/geometry.js"></script><script src="/js/desktop/leafy/renderer.js"></script><script>

  window.leafyReady=(async()=>{await AuraLeafyRenderer.prepare();
  window.renderer=AuraLeafyRenderer.create(document.getElementById('plant'));
  window.demo=age=>({seed:731,age_hours:age,moisture:80,nutrients:80,vitality:100,branches:Array.from({length:Math.min(52,3+Math.floor(age/12))},(_,i)=>({id:i,parent:i<3?-1:Math.floor((i-3)/2),attach:i<3?0:Math.min(5,Math.max(1,Math.floor(age/12))),nodes:Array.from({length:Math.min(36,2+Math.floor(Math.max(0,age-i*5)/3))},(_,n)=>Math.max(0,i*5+n*3))}))});
  window.show=(age,health=100,light=false)=>{const s=demo(age);s.vitality=health;s.dead=health===0;s.moisture=health<40?0:80;renderer.update(s,innerWidth,innerHeight,{x:.42,y:.91},light)};
  show(1);})();
  </script></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true).Set("enable-unsafe-swiftshader")
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/fixture")
	defer page.Close()
	page.MustSetViewport(1920, 1080, 1, false)
	page.MustWaitLoad()
	t.Log(page.MustEval(`()=>JSON.stringify({renderer:typeof AuraLeafyRenderer,three:typeof THREE,geometry:typeof AuraLeafyGeometry,errors})`).Str())
	page.MustEval(`async()=>await window.leafyReady`)
	dir := filepath.Join("..", "reports", "leafy")
	os.MkdirAll(dir, 0755)
	for _, age := range []int{1, 24, 168, 504} {
		page.MustEval(`age=>show(age)`, age)
		page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("graphics-day-%d.png", age/24)))
	}
	page.MustEval(`()=>show(504,20)`)
	page.MustScreenshot(filepath.Join(dir, "graphics-wilted.png"))
	page.MustEval(`()=>show(504,0)`)
	page.MustScreenshot(filepath.Join(dir, "graphics-dead.png"))
	page.MustEval(`()=>show(168,100,true)`)
	page.MustScreenshot(filepath.Join(dir, "graphics-fruity-light.png"))
	t.Log(page.MustEval(`async()=>{show(504);const canvas=document.getElementById('plant'),gl=canvas.getContext('webgl2')||canvas.getContext('webgl'),ext=gl.getExtension('WEBGL_debug_renderer_info'),times=[];for(let i=0;i<30;i++){await new Promise(requestAnimationFrame);const start=performance.now();renderer.render(Math.sin(i*.15));gl.finish();times.push(performance.now()-start);}times.sort((a,b)=>a-b);return JSON.stringify({gpu:ext?gl.getParameter(ext.UNMASKED_RENDERER_WEBGL):gl.getParameter(gl.RENDERER),p50:times[15],p95:times[28],...renderer.metrics()});}`).Str())
	t.Log(page.MustEval(`()=>JSON.stringify(renderer.metrics())`).Str())
	if errors := page.MustEval(`()=>JSON.stringify(errors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
	if os.Getenv("AURAGO_LEAFY_BAKE") == "1" {
		url := page.MustEval(`()=>renderer.bake()`).Str()
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:image/png;base64,"))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("img", "leafy", "fallback.png")
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	page.MustEval(`()=>renderer.dispose()`)
}
