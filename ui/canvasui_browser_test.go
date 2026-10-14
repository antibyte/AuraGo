package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCanvasUIUpdatedRuntimeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><div id="host" style="position:relative;width:400px;height:240px"><canvas id="source" style="width:400px;height:240px"></canvas><div id="content">Fixture</div><canvas id="output" style="position:absolute;inset:0;width:400px;height:240px"></canvas></div></body></html>`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage().Timeout(30 * time.Second)
	defer page.Close()
	page.MustNavigate(server.URL + "/fixture").MustWaitLoad()
	page.MustEval(`async()=>{
        const support=await import('/js/vendor/canvasui/canvas-support.js');
        const frame=()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
        const host=document.getElementById('host');
        let resized=0;
        const observer=support.createCanvasResizeObserver(()=>resized++);
        observer.observe(host); await frame();
        const before=resized;
        window.dispatchEvent(new Event('resize')); window.visualViewport?.dispatchEvent(new Event('resize'));
        await frame(); if(resized<=before)throw Error('viewport resize did not reach renderer');
        observer.disconnect(); const stopped=resized;
        window.dispatchEvent(new Event('resize')); window.visualViewport?.dispatchEvent(new Event('resize'));
        await frame(); if(resized!==stopped)throw Error('resize listener survived dispose');
        const ratio=support.getCanvasPixelRatio({clientWidth:100000,clientHeight:100000});
        if(ratio*ratio*1e10>16777217)throw Error('canvas allocation unbounded');
        for(const [file,name] of [['droplets','createDroplets'],['flame-wrap','createFlameWrap']]){
            const module=await import('/js/vendor/canvasui/'+file+'.js');
            const output=document.getElementById('output').cloneNode(); document.getElementById('output').replaceWith(output);
            const bitmap=document.createElement('canvas');bitmap.width=400;bitmap.height=240;bitmap.getContext('2d').fillRect(0,0,400,240);
            const instance=module[name]({output,source:document.getElementById('source'),content:document.getElementById('content'),bitmap});
            if(!instance)throw Error(file+' WebGL renderer unavailable');
            await frame(); instance.resize(); instance.destroy();
        }
    }`)
}
