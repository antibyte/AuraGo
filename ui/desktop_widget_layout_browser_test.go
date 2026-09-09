package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopWidgetLayoutBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell startup seam missing")
	}
	shell = shell[:cut] + `
window.layout = {state, updateWidgetCard, resizeWidgetToContent, alignDefaultBuiltinWidgetStack};
window.t = key => key;
api = async (url, options) => {
    if (url !== '/api/desktop/widgets' || options.method !== 'POST') throw Error('unexpected request');
    const record = JSON.parse(options.body);
    state.bootstrap.widgets = state.bootstrap.widgets.map(w => w.id === record.id ? record : w);
    localStorage.setItem('layout', JSON.stringify(state.bootstrap.widgets));
    window.saved = record;
};
loadBootstrap = async () => state.bootstrap.widgets.forEach((w, i) => updateWidgetCard(document.querySelector('[data-widget-id="'+w.id+'"]'), w, i));
state.bootstrap = {widgets: JSON.parse(localStorage.getItem('layout') || 'null') || [
    {id:'builtin-weather', type:'builtin', w:320, x:0, y:0},
    {id:'builtin-sysmon', type:'builtin', w:460, x:0, y:0},
    {id:'builtin-printer', type:'builtin', w:300, x:400, y:160},
    {id:'custom', type:'html', entry:'custom.html', w:600, x:48, y:160, config:{keep:'yes'}},
    {id:'sticky', type:'sticky-note', w:220, x:400, y:400, config:{text:'Remember',auto_size:false}}
]};
for (const [i, widget] of state.bootstrap.widgets.entries()) {
    const card = document.createElement('article');
    document.querySelector('#vd-widgets').appendChild(card);
    updateWidgetCard(card, widget, i);
    if (widget.type !== 'sticky-note') card.style.setProperty('--vd-widget-auto-height', '120px');
    wireDraggableWidget(card, widget);
}
})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><link rel="stylesheet" href="/css/desktop-widgets.css"><style>*{box-sizing:border-box}body{margin:0}#vd-workspace{position:relative;width:1103px;height:803px}</style><div id="vd-workspace"><div id="vd-widgets" class="vd-widgets"></div></div><script>window.errors=[];window.addEventListener('error',e=>errors.push(e.message));</script><script src="/shell.js"></script></html>`)
	})
	mux.HandleFunc("/shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(30 * time.Second)
	defer page.Close()
	page.MustSetViewport(1200, 950, 1, false)
	page.MustWaitLoad()
	if !page.MustEval(`()=>document.querySelectorAll('.vd-widget').length===5 && [...document.querySelectorAll('.vd-widget')].every(c=>c.offsetWidth===320 && c.offsetLeft%8===0 && c.offsetTop%8===0)`).Bool() {
		t.Fatalf("all widget types must restore at weather width on the shared grid: %s", page.MustEval(`()=>JSON.stringify({errors,cards:[...document.querySelectorAll('.vd-widget')].map(c=>({id:c.dataset.widgetId,w:c.offsetWidth,x:c.offsetLeft,y:c.offsetTop}))})`).Str())
	}
	if !page.MustEval(`()=>{layout.alignDefaultBuiltinWidgetStack();const w=document.querySelector('[data-widget-id="builtin-weather"]'),s=document.querySelector('[data-widget-id="builtin-sysmon"]');return s.offsetLeft===w.offsetLeft && s.offsetTop%8===0 && s.offsetTop>=w.offsetTop+w.offsetHeight+8}`).Bool() {
		t.Fatal("default widget stack must keep its gap and grid alignment")
	}
	// Verify live mouse snapping before release, then the persisted/reloaded geometry.
	page.Mouse.MustMoveTo(60, 165).MustDown(proto.InputMouseButtonLeft).MustMoveTo(67, 178)
	if !page.MustEval(`()=>{const c=document.querySelector('[data-widget-id="custom"]');return c.offsetLeft===56 && c.offsetTop===176 && c.classList.contains('vd-dragging') && !window.saved}`).Bool() {
		t.Fatal("widget must snap during movement")
	}
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	waitForJSBool(t, page, `()=>saved?.x===56 && saved?.y===176 && saved?.w===320 && saved?.config.keep==='yes'`)
	page.MustReload().MustWaitLoad()
	if !page.MustEval(`()=>{const c=document.querySelector('[data-widget-id="custom"]');return c.offsetLeft===56 && c.offsetTop===176 && c.offsetWidth===320}`).Bool() {
		t.Fatal("reload must retain snapped geometry")
	}
	page.Mouse.MustMoveTo(68, 181).MustDown(proto.InputMouseButtonLeft).MustMoveTo(1190, 940).MustUp(proto.InputMouseButtonLeft)
	waitForJSBool(t, page, `()=>saved?.x===768 && saved?.y===672`)
	if !page.MustEval(`()=>{const c=document.querySelector('[data-widget-id="custom"]');layout.resizeWidgetToContent('custom',{width:900,viewportWidth:280,height:80});return c.offsetWidth===320 && parseFloat(c.style.getPropertyValue('--vd-widget-frame-height'))>=80}`).Bool() {
		t.Fatal("iframe resize must adapt height without widening the card")
	}
	// Interactive controls and readonly cards must never initiate a drag.
	page.MustEval(`()=>{const c=document.querySelector('[data-widget-id="custom"]');c.innerHTML='<button>Action</button>';window.saved=null;}`)
	page.MustElement("[data-widget-id=custom] button").MustClick()
	page.MustEval(`()=>layout.state.bootstrap.readonly=true`)
	page.Mouse.MustMoveTo(780, 677).MustDown(proto.InputMouseButtonLeft).MustMoveTo(800, 690).MustUp(proto.InputMouseButtonLeft)
	if !page.MustEval(`()=>!window.saved && document.querySelector('[data-widget-id="custom"]').offsetLeft===768`).Bool() {
		t.Fatal("controls and readonly state must prevent widget dragging")
	}
	if !page.MustEval(`()=>{document.querySelector('#vd-workspace').style.width='300px';layout.state.bootstrap.widgets.forEach((w,i)=>layout.updateWidgetCard(document.querySelector('[data-widget-id="'+w.id+'"]'),w,i));return [...document.querySelectorAll('.vd-widget')].every(c=>c.offsetWidth===284 && c.offsetLeft===8 && c.offsetLeft+c.offsetWidth<=300)}`).Bool() {
		t.Fatal("all widgets must fit a narrow workspace with the same width")
	}
}
