package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDesktopPrinterWidgetBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1")
	}
	runtime := readDesktopAssetText(t, "js/desktop/core/widget-printer-runtime.js")
	browser := newSmokeBrowser(t)
	for _, lang := range []string{"de", "en"} {
		t.Run(lang, func(t *testing.T) {
			words := readDesktopAssetText(t, "lang/desktop/"+lang+".json")
			mux := http.NewServeMux()
			mux.Handle("/", http.FileServer(http.FS(Content)))
			mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `<html lang="`+lang+`"><link rel="stylesheet" href="/css/desktop-widgets.css"><style>body{--vd-text:#eee;--vd-text-muted:#aaa;--vd-bg:#20242a;--vd-theme-control-bg:#30343a;--vd-accent:#7abaff;--vd-border:#555}#card{width:320px}</style><div id="card"></div><script>
const words=`+words+`;const t=k=>words[k]||k;const esc=s=>String(s).replace(/[&<>"']/g,c=>'&#'+c.charCodeAt(0)+';');
const cleanups=[];const registerWidgetCleanup=f=>cleanups.push(f);window.requests=[];window.failed=false;
window.raw={Status:{PrintInfo:{Status:13,Progress:42,CurrentTicks:60,TotalTicks:3660,CurrentLayer:12,TotalLayer:100,Filename:'<img src=x onerror=alert(1)>'},TempOfNozzle:210,TempOfHotbed:60}};
const api=async(url,opts)=>{requests.push(url);if(failed)throw Error('offline');return url.includes('?')?raw:{printers:[{id:'lab',name:'Elegoo'},{id:'second',name:'Second'}],default_printer:'lab'}};
`+runtime+`;renderPrinterWidget(document.querySelector('#card'));</script></html>`)
			})
			server := httptest.NewServer(mux)
			defer server.Close()
			page := browser.MustPage(server.URL + "/fixture").Timeout(30 * time.Second)
			defer page.Close()
			page.MustWaitLoad()
			waitForJSBool(t, page, `()=>document.querySelector('progress').value===42`)
			if !page.MustEval(`()=>{const p=document.querySelector('.vd-printer');return p.scrollWidth<=320 && p.getBoundingClientRect().height<350}`).Bool() {
				t.Fatal("printer widget must remain compact")
			}
			if !page.MustEval(`()=>{const missing=printerWidgetData({Status:{PrintInfo:{Progress:null,CurrentTicks:null,TotalTicks:500}}});const klipper=printerWidgetData({result:{status:{print_stats:{state:'printing'},virtual_sdcard:{progress:0.5}}}});return missing.progress===null&&missing.remaining===null&&klipper.progress===50&&klipper.remaining===null}`).Bool() {
				t.Fatal("missing values or protocol normalization failed")
			}
			if !page.MustEval(`()=>document.querySelector('[data-printer=file]').textContent===raw.Status.PrintInfo.Filename && !document.querySelector('[data-printer=file] img') && document.querySelector('[data-printer=remaining]').textContent.includes('60')`).Bool() {
				t.Fatal("printer values or text safety failed")
			}
			var dict map[string]string
			_ = json.Unmarshal([]byte(words), &dict)
			if got := page.MustElement("[data-printer=expand]").MustText(); got != dict["desktop.widget_printer_enlarge"] {
				t.Fatalf("button: %q", got)
			}
			page.MustEval(`()=>{const img=document.querySelector('[data-printer=image]');img.src='data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="blue"/></svg>';}`)
			waitForJSBool(t, page, `()=>!document.querySelector('[data-printer=expand]').disabled`)
			if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "printer-widget-"+lang+".png"), page.MustScreenshot(), 0644); err != nil {
					t.Fatal(err)
				}
			}
			page.MustElement("[data-printer=expand]").MustClick()
			if !page.MustEval(`()=>document.querySelector('dialog').open && document.querySelector('dialog img') && document.querySelectorAll('[data-printer=image]').length===1`).Bool() {
				t.Fatal("camera expansion must reuse image")
			}
			page.MustElement("[data-printer=close]").MustClick()
			page.MustEval(`()=>{window.raw={Status:{PrintInfo:{Status:6}}};document.querySelector('[data-printer=refresh]').click();}`)
			waitForJSBool(t, page, `()=>document.querySelector('progress').hidden && document.querySelector('[data-printer=remaining]').textContent==='—'`)
			page.MustEval(`()=>{const s=document.querySelector('select');s.value='second';s.dispatchEvent(new Event('change'));}`)
			waitForJSBool(t, page, `()=>requests.some(url=>url.endsWith('printer_id=second'))`)
			page.MustEval(`()=>{failed=true;document.querySelector('[data-printer=refresh]').click();}`)
			waitForJSBool(t, page, `()=>document.querySelector('[data-printer=state]').textContent===t('desktop.load_failed')`)
			page.MustEval(`()=>cleanups.forEach(f=>f())`)
			if !page.MustEval(`()=>!document.querySelector('[data-printer=image]').hasAttribute('src')`).Bool() {
				t.Fatal("camera not stopped")
			}
			if strings.Contains(page.MustElement("#card").MustText(), "desktop.widget_printer_") {
				t.Fatal("untranslated widget text")
			}
		})
	}
}
