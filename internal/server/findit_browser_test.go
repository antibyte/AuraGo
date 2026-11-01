package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/webassets"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

func TestDesktopFindItBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 for the real-handler browser fixture")
	}
	s, _ := findItHTTPFixture(t)
	installFindItBrowserFonts(t)
	mux := http.NewServeMux()
	registerFindItRoutes(mux, s)
	mux.HandleFunc("/api/qr/", handleQR(s))
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join("..", "..", "ui"))))
	mux.HandleFunc("/findit-fixture", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="/css/desktop-base.css"><link rel="stylesheet" href="/css/desktop-shell-overrides.css"><link rel="stylesheet" href="/css/desktop-materials.css"><link rel="stylesheet" href="/css/desktop-app-common.css"><link rel="stylesheet" href="/css/desktop-icons.css"><link rel="stylesheet" href="/css/desktop-qr.css"><link rel="stylesheet" href="/css/desktop-app-findit.css">
<style>html,body{margin:0;width:100%;height:100%;overflow:hidden}#findit-fixture-root{width:100%;height:100%;min-height:0}body{font-family:var(--vd-font-ui,system-ui)}</style>
</head><body class="desktop-body" data-theme="standard" data-scheme="dark"><div id="findit-fixture-root"></div>
<script>window.errors=[];window.failedRequests=[];const realFetch=window.fetch;window.fetch=async(...args)=>{const response=await realFetch(...args);if(!response.ok)failedRequests.push([String(args[0]),response.status,(await response.clone().text()).slice(0,500)]);return response};addEventListener('error',e=>errors.push(e.error?.stack||e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason)));</script>
<script src="/js/desktop/shared/qr-dialog.js"></script>
<script src="/js/desktop/apps/findit-model.js"></script><script src="/js/desktop/apps/findit-dialogs.js"></script><script src="/js/desktop/apps/findit-csv.js"></script><script src="/js/desktop/apps/findit-items.js"></script><script src="/js/desktop/apps/findit-projects.js"></script><script src="/js/desktop/apps/findit-templates.js"></script><script src="/js/desktop/apps/findit-workflows.js"></script><script src="/js/desktop/apps/findit.js"></script>
<script>window.ready=(async()=>{const labels={...(await(await fetch('/lang/desktop/de.json')).json()),...(await(await fetch('/lang/findit/de.json')).json())};window.findItContext={t:(key,params)=>String(labels[key]||key.split('.').reduce((value,part)=>value?.[part],labels)||key).replace(/\{\{(\w+)\}\}/g,(_,x)=>params?.[x]??''),confirmDialog:async()=>true,setWindowBeforeClose:(id,fn)=>window.closeGuard=fn,setWindowMenus:(id,menus)=>window.menus=menus};window.app=FindItApp.render(document.getElementById('findit-fixture-root'),'test',findItContext);})();</script></body></html>`)
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()
	browserPath, ok := launcher.LookPath()
	if !ok {
		t.Fatal("Chrome or Edge is required for the browser fixture")
	}
	launch := launcher.New().Bin(browserPath).Headless(true).NoSandbox(true).Leakless(false)
	control, err := launch.Launch()
	if err != nil {
		t.Fatal(err)
	}
	defer launch.Cleanup()
	browser := rod.New().ControlURL(control).MustConnect()
	defer browser.Close()
	page := browser.MustPage(origin.URL + "/findit-fixture").Timeout(180 * time.Second)
	defer page.Close()
	page.MustSetViewport(1440, 900, 1, false)
	page.MustWaitLoad()
	page.MustEval(`async()=>await window.ready`)
	page.MustWait(`()=>window.app?.status && !window.app.busy && !!document.querySelector('.fi-app')`)
	out := filepath.Join("..", "..", "ui", "reports", "findit")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	functional, err := os.ReadFile(filepath.Join("..", "..", "ui", "testdata", "findit-functional.js"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := page.Eval(string(functional))
	if err != nil {
		page.MustScreenshot(filepath.Join(out, "failure.png"))
		t.Fatalf("FindIt browser workflow: %v; diagnostics=%s", err, page.MustEval(`()=>({errors:window.errors,requests:window.failedRequests,appError:window.app.error})`).JSON("", ""))
	}
	t.Log("FindIt real-handler workflows:", result.Value.JSON("", ""))
	lifecycle, err := os.ReadFile(filepath.Join("..", "..", "ui", "testdata", "findit-lifecycle.js"))
	if err != nil {
		t.Fatal(err)
	}
	lifecycleResult, err := page.Eval(string(lifecycle))
	if err != nil {
		page.MustScreenshot(filepath.Join(out, "failure-lifecycle.png"))
		t.Fatalf("FindIt lifecycle: %v", err)
	}
	t.Log("FindIt lifecycle:", lifecycleResult.Value.JSON("", ""))
	page.MustElement(`[data-action="new-item"]`).MustClick()
	page.MustElement(`dialog[open] input[name="name"]`).MustFocus()
	page.Keyboard.MustType(input.Tab)
	if !page.MustEval(`()=>!!document.activeElement?.closest('dialog[open]')`).Bool() {
		t.Fatal("keyboard focus left the dialog")
	}
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)
	qr := page.MustEval(`()=>window.findItQR`)
	qrPNG, err := base64.StdEncoding.DecodeString(qr.Get("data").Str())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "label.png"), qrPNG, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "label-target.txt"), []byte(qr.Get("target").Str()), 0600); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity"} {
		for _, scheme := range []string{"dark", "light"} {
			// Standard has a fixed dark scheme; Fruity exposes both modes.
			if theme == "standard" && scheme == "light" {
				continue
			}
			for _, width := range []int{360, 768, 1440} {
				page.MustSetViewport(width, 900, 1, false)
				page.MustEval(`(theme,scheme)=>{document.body.dataset.theme=theme;document.body.dataset.scheme=scheme;document.body.dataset.fruityMode=scheme}`, theme, scheme)
				page.MustEval(`async()=>{await document.fonts.ready;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));}`)
				if page.MustEval(`()=>document.documentElement.scrollWidth>innerWidth+1`).Bool() {
					t.Fatalf("horizontal overflow: %s/%s/%d", theme, scheme, width)
				}
				page.MustScreenshot(filepath.Join(out, fmt.Sprintf("%s-%s-%d.png", theme, scheme, width)))
			}
		}
	}
	if errors := page.MustEval(`()=>window.errors`); len(errors.Arr()) > 0 {
		t.Fatal(errors.JSON("", ""))
	}
}

func installFindItBrowserFonts(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	manifest := webassets.Manifest{Version: 1}
	source := filepath.Join("..", "..", "ui", "fonts", "qr")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := "ui/fonts/qr/" + filepath.ToSlash(rel)
		target := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, webassets.Entry{Path: name, Size: int64(len(data)), SHA256: webassets.Digest(data)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	id := webassets.Digest(data)
	if err := os.WriteFile(filepath.Join(stage, webassets.ManifestName), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(stage, filepath.Join(root, id)); err != nil {
		t.Fatal(err)
	}
	previous := webassets.Default
	store := webassets.Open(root, webassets.Pin{ID: id})
	if !store.Ready() {
		t.Fatal(store.Error)
	}
	webassets.Default = store
	t.Cleanup(func() { webassets.Default = previous; _ = store.Close() })
}
