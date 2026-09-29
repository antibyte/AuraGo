package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func TestBrowserVendorESModules(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("Chrome or Edge required")
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/vendor-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><script>window.BUILD_VERSION='vendor-test';</script><script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><canvas id="pdf"></canvas>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	launch := launcher.New().Bin(bin).Headless(true).NoSandbox(true)
	browser := rod.New().ControlURL(launch.MustLaunch()).MustConnect()
	defer launch.Cleanup()
	defer browser.Close()
	page := browser.MustPage(srv.URL + "/vendor-fixture").Timeout(45 * time.Second)
	defer page.Close()
	page.MustWaitLoad()
	page.MustEval(`async()=>{
        await AuraDesktopModules.loadAppAssets('quick-connect');
        if(typeof RFB!=='function')throw Error('noVNC was not initialized before app readiness');
        await AuraDesktopModules.loadAppAssets('viewer');
        if(!pdfjsLib.GlobalWorkerOptions.workerSrc.endsWith('?v=vendor-test'))throw Error('PDF worker lost its asset version');
        const objects=['<< /Type /Catalog /Pages 2 0 R >>','<< /Type /Pages /Kids [3 0 R] /Count 1 >>','<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] /Resources << >> /Contents 4 0 R >>','<< /Length 0 >>\nstream\n\nendstream'];
        let source='%PDF-1.7\n',offsets=[0];
        objects.forEach((object,index)=>{offsets.push(source.length);source+=(index+1)+' 0 obj\n'+object+'\nendobj\n';});
        const xref=source.length;source+='xref\n0 5\n0000000000 65535 f \n'+offsets.slice(1).map(offset=>String(offset).padStart(10,'0')+' 00000 n \n').join('')+'trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n'+xref+'\n%%EOF';
        const doc=await pdfjsLib.getDocument({data:new TextEncoder().encode(source)}).promise;
        const first=await doc.getPage(1),viewport=first.getViewport({scale:1}),canvas=document.getElementById('pdf');
        canvas.width=viewport.width;canvas.height=viewport.height;
        await first.render({canvasContext:canvas.getContext('2d'),viewport}).promise;
        if(doc.numPages!==1)throw Error('PDF page did not load');
        await doc.loadingTask.destroy();
    }`)
}
