package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktop"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopStickyNotesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	svc, err := desktop.NewService(desktop.Config{Enabled: true, WorkspaceDir: filepath.Join(t.TempDir(), "workspace"), DBPath: filepath.Join(t.TempDir(), "desktop.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	for i, text := range []string{"Nicht vergessen:\n\nKaffee kaufen ☕", "Ideen für morgen\n\n• Musik aufdrehen\n• Etwas Neues bauen", "Bin im Garten.\n\nBis gleich!", "Kleine Schritte.\nGroße Ideen.\n\nDu schaffst das :)"} {
		if err := svc.UpsertWidget(context.Background(), desktop.Widget{ID: fmt.Sprintf("sticky-demo-%d", i), Title: "Sticky note", Type: "sticky-note", Icon: "notes", X: 175 + i*245, Y: 180 + i%2*55, W: 220, H: 220, Config: map[string]interface{}{"text": text, "auto_size": false}}, desktop.SourceUser); err != nil {
			t.Fatal(err)
		}
	}
	html := regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`).ReplaceAllString(readDesktopAssetText(t, "desktop.html"), "")
	html = regexp.MustCompile(`\{\{[^}]*\}\}`).ReplaceAllString(html, "")
	html = strings.Replace(html, "</body>", `<script src="/js/shared/lazy-assets.js"></script><script src="/js/desktop/core/module-loader.js"></script><script src="/sticky-shell.js"></script><script src="/testdata/aurora-fixture.js"></script></body>`, 1)
	shell := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	cut := strings.LastIndex(shell, "    ensureDesktopRadialMenuAnchor();")
	if cut < 0 {
		t.Fatal("shell startup seam missing")
	}
	shell = shell[:cut] + `window.aurora={state,renderWidgets,loadBootstrap,editStickyNote,loadIconManifest,applyDesktopSettings,renderIcons,renderStartApps,renderStartButtonIcon,wireShellChromeControls,bindViewportMetrics,handleDesktopKeydown,showDesktopContextMenu,closeContextMenu};})();`
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(Content)))
	mux.HandleFunc("/fixture", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, html) })
	mux.HandleFunc("/sticky-shell.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shell)
	})
	mux.HandleFunc("/fixture-words", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, readDesktopAssetText(t, "lang/desktop/de.json"))
	})
	mux.HandleFunc("/fixture-apps", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "[]") })
	mux.HandleFunc("/api/desktop/widgets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var err error
		switch r.Method {
		case http.MethodGet:
			all, e := svc.ListAllWidgets(r.Context())
			if e != nil {
				http.Error(w, e.Error(), 500)
				return
			}
			widgets := []desktop.Widget{}
			for _, widget := range all {
				if widget.Type == "sticky-note" {
					widgets = append(widgets, widget)
				}
			}
			json.NewEncoder(w).Encode(widgets)
			return
		case http.MethodPost:
			var widget desktop.Widget
			err = json.NewDecoder(r.Body).Decode(&widget)
			if err == nil {
				err = svc.UpsertWidget(r.Context(), widget, desktop.SourceUser)
			}
		case http.MethodDelete:
			err = svc.DeleteWidget(r.Context(), r.URL.Query().Get("id"), desktop.SourceUser)
		}
		if err != nil {
			t.Logf("widget API: %v", err)
			http.Error(w, err.Error(), 400)
			return
		}
		fmt.Fprint(w, `{}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	page := newSmokeBrowser(t).MustPage(server.URL + "/fixture").Timeout(40 * time.Second)
	defer page.Close()
	defer func() {
		if t.Failed() {
			t.Log(page.MustEval(`()=>JSON.stringify({errors:fixtureErrors,dialogs:[...document.querySelectorAll('[role=dialog],dialog')].map(d=>d.textContent),notes:aurora.state.bootstrap.widgets.map(w=>({id:w.id,text:w.config?.text})),menus:[...document.querySelectorAll('.vd-context-menu')].map(m=>({class:m.className,text:m.textContent}))})`).Str())
		}
	}()
	page.MustSetViewport(1366, 768, 1, false)
	page.MustWaitLoad()
	setup := `async()=>{
        await fixtureReady;
        const baseFetch=window.fetch;
        window.fetch=async(url,options={})=>{
            if(String(url).startsWith('/api/desktop/widgets')) {
                if(window.failSave && options.method==='POST') return new Response('{}',{status:503});
                return nativeFetch(url,options);
            }
            if(url==='/api/desktop/bootstrap') {
                const widgets=await (await nativeFetch('/api/desktop/widgets')).json();
                return reply({...aurora.state.bootstrap,widgets,all_widgets:widgets});
            }
            return baseFetch(url,options);
        };
        aurora.state.bootstrap.settings['desktop.show_widgets']=true;
        aurora.state.bootstrap.settings['appearance.wallpaper']='paper_waves';
        await aurora.loadBootstrap();
    }`
	page.MustEval(setup)
	page.MustEval(`()=>{document.querySelector('.vd-sticky-menu').focus();}`)
	waitForJSBool(t, page, `()=>getComputedStyle(document.querySelector('.vd-sticky-menu')).opacity==='1'`)
	if !page.MustEval(`()=>document.querySelectorAll('.vd-sticky-note').length===4 && new Set([...document.querySelectorAll('.vd-sticky-note')].map(n=>n.style.getPropertyValue('--sticky-angle'))).size===4`).Bool() {
		t.Fatal("notes missing or paper variation missing")
	}
	page.MustEval(`()=>aurora.showDesktopContextMenu({target:document.getElementById('vd-workspace'),preventDefault(){},clientX:500,clientY:480})`)
	page.MustElement(`[data-context-action="new-sticky-note"]`).MustClick()
	page.MustElement(".vd-sticky-editor textarea").MustInput("Mehrzeilig\n<img src=x onerror=alert(1)>")
	page.MustEval(`()=>window.failSave=true`)
	page.MustElement(".vd-sticky-editor [type=submit]").MustClick()
	waitForJSBool(t, page, `()=>!!document.querySelector('.vd-sticky-error').textContent`)
	if !page.MustEval(`()=>document.querySelector('textarea').value.includes('<img') && document.querySelector('dialog').open`).Bool() {
		t.Fatal("failed save lost the draft")
	}
	page.MustEval(`()=>window.failSave=false`)
	page.MustElement(".vd-sticky-editor [type=submit]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.vd-sticky-note').length===5`)
	if !page.MustEval(`()=>{const n=[...document.querySelectorAll('.vd-sticky-note')].find(n=>n.textContent.includes('<img'));window.noteId=n.dataset.widgetId;return !n.querySelector('img') && n.textContent.includes('Mehrzeilig\n<img');}`).Bool() {
		t.Fatal("note text was not rendered safely")
	}
	point := page.MustEval(`()=>{const n=document.querySelector('[data-widget-id="'+noteId+'"]');const r=n.getBoundingClientRect();return {x:r.left+45,y:r.top+20};}`)
	x, y := point.Get("x").Num(), point.Get("y").Num()
	page.Mouse.MustMoveTo(x, y).MustDown(proto.InputMouseButtonLeft).MustMoveTo(x+80, y-60).MustUp(proto.InputMouseButtonLeft)
	waitForJSBool(t, page, `()=>aurora.state.bootstrap.widgets.find(w=>w.id===noteId).x>=575`)
	page.MustEval(`()=>{const n=document.querySelector('[data-widget-id="'+noteId+'"]');window.savedNote={id:noteId,left:n.style.left,top:n.style.top,angle:n.style.getPropertyValue('--sticky-angle')};sessionStorage.setItem('note',JSON.stringify(savedNote));}`)
	page.MustReload().MustWaitLoad()
	page.MustEval(setup)
	if !page.MustEval(`()=>{const s=JSON.parse(sessionStorage.getItem('note'));window.noteId=s.id;const n=document.querySelector('[data-widget-id="'+s.id+'"]');return n.style.left===s.left && n.style.top===s.top && n.style.getPropertyValue('--sticky-angle')===s.angle && n.textContent.includes('<img');}`).Bool() {
		t.Fatal("reload lost content, position or paper appearance")
	}
	page.MustEval(`()=>document.querySelector('[data-widget-id="'+noteId+'"] .vd-sticky-menu').click()`)
	page.MustElement(`.vd-context-menu:not(.vd-context-menu-closing) [data-context-action="0"]`).MustClick()
	page.MustEval(`()=>{const input=document.querySelector('textarea');input.value='Erledigt!';document.querySelector('form').requestSubmit();}`)
	waitForJSBool(t, page, `()=>document.querySelector('[data-widget-id="'+noteId+'"] .vd-sticky-text').textContent==='Erledigt!'`)
	page.MustEval(`()=>document.querySelector('[data-widget-id="'+noteId+'"] .vd-sticky-menu').click()`)
	page.MustElement(`.vd-context-menu:not(.vd-context-menu-closing) [data-context-action="1"]`).MustClick()
	page.MustElement(".vd-sticky-editor [type=submit]").MustClick()
	waitForJSBool(t, page, `()=>document.querySelectorAll('.vd-sticky-note').length===6`)
	page.MustEval(`()=>document.querySelector('[data-widget-id="'+noteId+'"] .vd-sticky-menu').click()`)
	page.MustElement(`.vd-context-menu:not(.vd-context-menu-closing) [data-context-action="3"]`).MustClick()
	page.MustElement(".vd-modal [type=submit]").MustClick()
	waitForJSBool(t, page, `()=>!document.querySelector('[data-widget-id="'+noteId+'"]')`)
	for _, theme := range []string{"standard", "fruity-light", "fruity-dark"} {
		page.MustEval(`theme=>fixtureTheme(theme)`, theme)
		if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
			os.MkdirAll(dir, 0755)
			page.MustScreenshot(filepath.Join(dir, "sticky-notes-"+theme+".png"))
		}
	}
	page.MustEval(`()=>{aurora.state.bootstrap.readonly=true;document.querySelector('.vd-sticky-menu').click();}`)
	if !page.MustEval(`()=>[...document.querySelectorAll('.vd-context-item')].every(b=>b.disabled)`).Bool() {
		t.Fatal("read-only note actions are enabled")
	}
	page.MustEval(`()=>{aurora.closeContextMenu(true);aurora.editStickyNote(null,20,20);}`)
	if !page.MustEval(`()=>!document.querySelector('.vd-sticky-editor')`).Bool() {
		t.Fatal("read-only desktop allowed note creation")
	}
	page.MustSetViewport(900, 700, 1, false)
	page.MustEval(`()=>aurora.renderWidgets()`)
	if !page.MustEval(`()=>[...document.querySelectorAll('.vd-sticky-note')].every(n=>{const r=n.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth && r.bottom<=innerHeight;})`).Bool() {
		t.Fatal("notes escaped the smaller desktop viewport")
	}
	if errors := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); errors != "[]" {
		t.Fatal(errors)
	}
}

func TestDesktopStickyNotesTranslations(t *testing.T) {
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+locale+".json")), &words); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"desktop.sticky_note", "desktop.sticky_add", "desktop.sticky_placeholder", "desktop.sticky_actions"} {
			if strings.TrimSpace(words[key]) == "" {
				t.Errorf("%s missing %s", locale, key)
			}
		}
	}
}
