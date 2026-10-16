package ui

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

var bookshelfLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

func TestDesktopBookshelfTranslations(t *testing.T) {
	english, err := os.ReadFile(filepath.Join("lang", "desktop", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	englishLabels := map[string]string{}
	if err := json.Unmarshal(english, &englishLabels); err != nil {
		t.Fatalf("decode en locale: %v", err)
	}
	required := make([]string, 0)
	for key, value := range englishLabels {
		if strings.HasPrefix(key, "bookshelf.") && strings.TrimSpace(value) != "" {
			required = append(required, key)
		}
	}
	if len(required) < 250 {
		t.Fatalf("English Bookshelf catalog has only %d keys; refusing an incomplete locale check", len(required))
	}
	for _, locale := range bookshelfLocales {
		body, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		labels := map[string]string{}
		if err := json.Unmarshal(body, &labels); err != nil {
			t.Fatalf("decode %s locale: %v", locale, err)
		}
		keys := make([]string, 0)
		for key, value := range labels {
			if strings.HasPrefix(key, "bookshelf.") {
				if strings.TrimSpace(value) == "" {
					t.Errorf("%s has an empty translation for %s", locale, key)
				}
				keys = append(keys, key)
			}
		}
		for _, key := range required {
			if strings.TrimSpace(labels[key]) == "" {
				t.Errorf("%s is missing %s", locale, key)
			}
		}
		if len(keys) != len(required) {
			t.Errorf("%s has %d Bookshelf keys, want %d", locale, len(keys), len(required))
		}
		if labels["desktop.app_bookshelf"] == "" {
			t.Errorf("%s is missing the Bookshelf app name", locale)
		}
	}
}

func TestDesktopBookshelfBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Bookshelf browser fixture")
	}
	browser := newSmokeBrowser(t)
	books := sampleBookshelfFixtureBooks()
	byID := make(map[string]map[string]any, len(books))
	createdSources := make(chan map[string]any, 1)
	bookLoadRequests := make(chan string, 8)
	uploadRequests := make(chan string, 16)
	lateUploadStarted := make(chan struct{}, 1)
	releaseLateUpload := make(chan struct{})
	var releaseLateUploadOnce sync.Once
	releaseLateUploadNow := func() { releaseLateUploadOnce.Do(func() { close(releaseLateUpload) }) }
	t.Cleanup(releaseLateUploadNow)
	var uploadRetryMu sync.Mutex
	uploadFailuresRemaining := map[string]int{"retry-standard.epub": 1, "retry-fruity.epub": 1}
	slowSearchStarted := make(chan struct{})
	releaseSlowSearch := make(chan struct{})
	var releaseSlowSearchOnce sync.Once
	t.Cleanup(func() { releaseSlowSearchOnce.Do(func() { close(releaseSlowSearch) }) })
	var collectionMu sync.Mutex
	collectionIDs := []string{"siddhartha", "1984", "dispossessed", "stranger"}
	collectionRevision := 1
	collection := func() map[string]any {
		return map[string]any{"id": "classics", "name": "Modern classics", "position": 0, "revision": collectionRevision, "book_count": len(collectionIDs), "book_ids": append([]string(nil), collectionIDs...)}
	}
	readerShell, err := os.ReadFile("bookshelf-reader.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, book := range books {
		byID[book["id"].(string)] = book
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(".")))
	mux.HandleFunc("/api/desktop/bookshelf/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		path := strings.TrimPrefix(r.URL.Path, "/api/desktop/bookshelf/")
		if path == "reader" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src http://"+r.Host+"/js/vendor/foliate/reader.bundle.js http://"+r.Host+"/js/desktop/apps/bookshelf/reader-host.js; style-src 'unsafe-inline' blob:; img-src blob: data:; font-src blob: data:; connect-src blob:; frame-src data: blob:; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
			_, _ = w.Write(readerShell)
			return
		}
		if path == "books/1984/content" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprint(w, "A real reader integration fixture.\n\nThe lighthouse stands beyond the harbor.\n\nThe book can be opened from the shelf and closed again.")
			return
		}
		if strings.HasPrefix(path, "books/no-cover/") && strings.HasSuffix(path, "/cover") {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(path, "/cover") {
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 340"><defs><linearGradient id="a" x2="1"><stop stop-color="#172a35"/><stop offset="1" stop-color="#b36d42"/></linearGradient></defs><rect width="240" height="340" fill="url(#a)"/><path d="M22 24h196v292H22z" fill="none" stroke="#e8ca9a" stroke-width="2"/><circle cx="120" cy="132" r="59" fill="#dbb777" opacity=".84"/><path d="M64 218h112" stroke="#e8ca9a" stroke-width="2"/><text x="120" y="267" fill="#fff0d3" text-anchor="middle" font-family="Georgia" font-size="15">AURA EDITIONS</text></svg>`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if path == "capabilities" {
			_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "can_read": true, "can_write": true, "readonly": false, "local_roots": []any{}, "limits": map[string]any{"max_book_bytes": 16}, "sources": []any{map[string]any{"provider": "local", "available": true}, map[string]any{"provider": "webdav", "available": true}, map[string]any{"provider": "smb", "available": true}, map[string]any{"provider": "koofr", "available": false, "reason": "not_configured"}}, "converter": map[string]any{"available": false, "enabled": false, "reason": "not_configured", "formats": []any{}}})
			return
		}
		switch path {
		case "imports":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, "invalid upload", http.StatusBadRequest)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "missing upload", http.StatusBadRequest)
				return
			}
			_ = file.Close()
			filename := header.Filename
			uploadRequests <- filename
			if header.Size > 16 {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "limit_exceeded", "max_bytes": 16})
				return
			}
			if strings.HasPrefix(filename, "late-") {
				lateUploadStarted <- struct{}{}
				<-releaseLateUpload
			}
			if strings.HasPrefix(filename, "server-limit-") {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "limit_exceeded", "max_bytes": 1})
				return
			}
			uploadRetryMu.Lock()
			if uploadFailuresRemaining[filename] > 0 {
				uploadFailuresRemaining[filename]--
				uploadRetryMu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "fixture_failure"})
				return
			}
			uploadRetryMu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"imported": true})
		case "books/1984/prepare":
			_ = json.NewEncoder(w).Encode(map[string]any{"content_url": "/api/desktop/bookshelf/books/1984/content", "format": "txt", "revision": "fixture-revision", "progress": map[string]any{"revision": 1}})
		case "books/no-cover/prepare":
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "source_changed"})
		case "books/1984/annotations":
			_ = json.NewEncoder(w).Encode(map[string]any{"annotations": []any{}})
		case "books/1984/progress":
			if r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "progress request must use JSON", http.StatusBadRequest)
				return
			}
			var progress map[string]any
			if err := json.NewDecoder(r.Body).Decode(&progress); err != nil {
				http.Error(w, "invalid progress", 400)
				return
			}
			progress["revision"] = 2
			_ = json.NewEncoder(w).Encode(map[string]any{"progress": progress})
		case "sources":
			if r.Method == http.MethodPost {
				if r.Header.Get("Content-Type") != "application/json" {
					http.Error(w, "source request must use JSON", http.StatusBadRequest)
					return
				}
				var payload struct {
					Source map[string]any `json:"source"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, "invalid source request", http.StatusBadRequest)
					return
				}
				createdSources <- payload.Source
				_ = json.NewEncoder(w).Encode(map[string]any{"source": map[string]any{"id": "webdav-fixture", "generation": 1, "name": "Remote books", "provider": "webdav"}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sources": []any{map[string]any{"id": "shelf-local", "provider": "local", "name": "Home Library", "path": "Books", "available": true}}})
		case "sources/browse":
			if r.URL.Query().Get("provider") != "webdav" || r.URL.Query().Get("mount_id") != "" {
				http.Error(w, "WebDAV does not have a mount selector", http.StatusBadRequest)
				return
			}
			items := []any{map[string]any{"name": "Books", "path": "Books", "is_dir": true}}
			if r.URL.Query().Get("path") == "Books" {
				items = []any{map[string]any{"name": "sample.epub", "path": "Books/sample.epub", "format": "epub", "is_dir": false}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case "sources/webdav-fixture/scan":
			_ = json.NewEncoder(w).Encode(map[string]any{"job_id": "scan-fixture"})
		case "jobs/scan-fixture":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "scan-fixture", "status": "partial", "scanned": 1, "imported": 1, "skipped": 0})
		case "collections":
			collectionMu.Lock()
			defer collectionMu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"collections": []any{collection()}})
		case "collections/classics/books":
			collectionMu.Lock()
			defer collectionMu.Unlock()
			if r.Method != http.MethodPatch || r.Header.Get("If-Match") != fmt.Sprint(collectionRevision) {
				http.Error(w, "missing collection revision", 412)
				return
			}
			var input struct {
				BookIDs []string `json:"book_ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				http.Error(w, "invalid order", 400)
				return
			}
			collectionIDs = append([]string(nil), input.BookIDs...)
			collectionRevision++
			_ = json.NewEncoder(w).Encode(map[string]any{"collection": collection()})
		case "books":
			query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
			if query == "__bookshelf_slow_search__" {
				bookLoadRequests <- query + ":" + r.URL.Query().Get("offset")
				close(slowSearchStarted)
				<-releaseSlowSearch
				_ = json.NewEncoder(w).Encode(map[string]any{"books": []any{}, "total": 0})
				return
			}
			if query == "__bookshelf_latest_search__" {
				offset := r.URL.Query().Get("offset")
				bookLoadRequests <- query + ":" + offset
				book := books[2]
				if offset == "36" {
					book = books[0]
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"books": []map[string]any{book}, "total": 37})
				return
			}
			filtered := make([]map[string]any, 0, len(books))
			candidates := books
			if r.URL.Query().Get("collection_id") == "classics" {
				collectionMu.Lock()
				candidates = make([]map[string]any, 0, len(collectionIDs))
				for _, id := range collectionIDs {
					candidates = append(candidates, byID[id])
				}
				collectionMu.Unlock()
			}
			for _, book := range candidates {
				if query == "" || strings.Contains(strings.ToLower(book["title"].(string)+" "+book["author"].(string)), query) {
					filtered = append(filtered, book)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"books": filtered, "total": len(filtered)})
		case "cache":
			_ = json.NewEncoder(w).Encode(map[string]any{"used_bytes": 0, "pinned_bytes": 0, "limit_bytes": 1 << 30})
		default:
			if strings.HasPrefix(path, "books/") && strings.HasSuffix(path, "/cover") {
				return
			}
			if strings.HasPrefix(path, "books/") {
				id := strings.TrimSuffix(strings.TrimPrefix(path, "books/"), "/cover")
				if book, ok := byID[id]; ok {
					_ = json.NewEncoder(w).Encode(map[string]any{"book": book, "collections": []any{}, "renditions": []any{}})
					return
				}
			}
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/bookshelf-fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><link rel="stylesheet" href="/css/desktop-app-bookshelf.css"><style>html,body{width:100%;height:100%;margin:0;overflow:hidden}body{background:#17191c}#host{height:100%;width:100%}</style></head><body class="desktop-body" data-theme="standard"><div id="host"></div><script src="/js/desktop/apps/bookshelf/api.js"></script><script src="/js/desktop/apps/bookshelf/progress.js"></script><script src="/js/desktop/apps/bookshelf/reader.js"></script><script src="/js/desktop/apps/bookshelf.js"></script><script>window.errors=[];window.__slowResponseParsed=false;window.__disposeRequestStarted=false;window.__disposeResponseParsed=false;window.__uploadResponsesParsed=0;const nativeFetch=window.fetch.bind(window);window.fetch=(input,...args)=>{const url=String(input);if(url.includes('__bookshelf_dispose_search__')){window.__disposeRequestStarted=true;return new Promise(resolve=>{window.__releaseDisposeRequest=()=>resolve({ok:true,statusText:'OK',json:async()=>{window.__disposeResponseParsed=true;return{books:[],total:0};}});});}return nativeFetch(input,...args).then(response=>{if(url.includes('/imports')){const json=response.json.bind(response);response.json=(...jsonArgs)=>json(...jsonArgs).then(body=>{window.__uploadResponsesParsed++;return body;});}if(url.includes('__bookshelf_slow_search__')){const json=response.json.bind(response);response.json=(...jsonArgs)=>json(...jsonArgs).then(body=>{window.__slowResponseParsed=true;return body;});}return response;});};addEventListener('error',e=>errors.push(e.error?.stack||e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason?.stack||e.reason)));window.ready=(async()=>{const labels=await(await fetch('/lang/desktop/en.json')).json();const api=async(path,opts={})=>{const response=await fetch(path,opts);const body=await response.json().catch(()=>({}));if(!response.ok){const error=new Error(body.error||response.statusText);error.status=response.status;error.body=body;throw error;}return body;};window.ctx={t:key=>labels[key]||key,api,confirmDialog:async()=>true};BookshelfApp.render(document.querySelector('#host'),'bookshelf-fixture',ctx);await new Promise(resolve=>{const deadline=Date.now()+5000;const poll=()=>document.querySelector('.bs-book')||Date.now()>deadline?resolve():setTimeout(poll,25);poll();});})();</script></body></html>`)
	})
	fixtureURL := ""
	if address := os.Getenv("AURAGO_BOOKSHELF_FIXTURE_ADDR"); address != "" {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("listen on Bookshelf fixture address %s: %v", address, err)
		}
		server := &http.Server{Handler: mux}
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(func() { _ = server.Close() })
		fixtureURL = "http://" + listener.Addr().String()
	} else {
		server := httptest.NewServer(mux)
		t.Cleanup(server.Close)
		fixtureURL = server.URL
	}
	page := browser.MustPage(fixtureURL + "/bookshelf-fixture").Timeout(60 * time.Second)
	t.Cleanup(func() { _ = page.Close() })
	page.MustSetViewport(1280, 860, 1, false)
	page.MustWaitLoad()
	if err := page.Wait(rod.Eval(`() => !!document.querySelector('.bs-book') || (window.errors || []).length > 0`)); err != nil {
		t.Fatalf("shelf initialization failed: %v; browser state: %s", err, page.CancelTimeout().MustEval(`() => JSON.stringify({url:location.href,ready:typeof window.ready,errors:window.errors,body:document.body.innerText})`).Str())
	}
	if errs := page.MustEval(`() => JSON.stringify(window.errors || [])`).Str(); errs != "[]" {
		t.Fatalf("shelf initialization errors: %s", errs)
	}
	page.MustElement(".bs-book")
	if page.MustElement(`[data-action="add"]`).MustProperty("disabled").Bool() {
		t.Fatal("authorized Add books action remains disabled after capabilities load")
	}
	if count := page.MustEval(`() => document.querySelectorAll('.bs-book').length`).Int(); count != len(books) {
		t.Fatalf("shelf rendered %d books, want %d", count, len(books))
	}
	if !page.MustEval(`() => { const r=document.querySelector('.bs-library-view').getBoundingClientRect(); return r.width>0 && document.querySelector('.bs-paper-page').getBoundingClientRect().width>380 }`).Bool() {
		t.Fatal("wide Reading Salon composition did not render both panes")
	}
	selectBook := func(id string) {
		if !page.MustEval(`id=>{
			const state=BookshelfApp.instances.get('bookshelf-fixture');
			const buttons=Array.from(document.querySelectorAll('[data-shelf] [data-book]'));
			const target=buttons.find(button=>button.dataset.book===id);
			let current=buttons.find(button=>button.dataset.book===state?.selected?.id);
			if(!target||!current)return false;
			current.focus();
			for(let step=0;step<buttons.length;step++){
				const from=buttons.indexOf(current),to=buttons.indexOf(target);
				if(from===to)return state.selected?.id===id;
				current.dispatchEvent(new KeyboardEvent('keydown',{key:from<to?'ArrowRight':'ArrowLeft',bubbles:true,cancelable:true}));
				current=document.activeElement?.closest('[data-book]');
				if(!current)return false;
			}
			return state.selected?.id===id;
		}`, id).Bool() {
			t.Fatalf("could not select Bookshelf book %q with shelf keyboard navigation", id)
		}
	}
	for _, theme := range []string{"standard", "fruity"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme;document.body.dataset.fruityMode='light';}`, theme)
		page.MustSetViewport(1024, 768, 1, false)
		selectBook("no-cover")
		page.MustEval(`() => { const image=document.querySelector('.bs-preview-art img'); if(image) image.loading='eager'; }`)
		missingCover := false
		missingCoverDeadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(missingCoverDeadline) {
			missingCover = page.MustEval(`() => { const state=BookshelfApp.instances.get('bookshelf-fixture'); return state?.selected?.id==='no-cover' && !!document.querySelector('.bs-preview-art.is-missing'); }`).Bool()
			if missingCover {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !missingCover {
			previewState := page.MustEval(`() => { const state=BookshelfApp.instances.get('bookshelf-fixture'); const preview=document.querySelector('.bs-preview-art'); const image=preview?.querySelector('img'); return JSON.stringify({selected:state?.selected?.id,preview:preview?.className,title:preview?.querySelector('.bs-cover-fallback b')?.textContent,image:image&&{src:image.getAttribute('src'),complete:image.complete,naturalWidth:image.naturalWidth,loading:image.loading}}); }`).Str()
			t.Fatalf("Bookshelf preview did not enter missing-cover state: %s", previewState)
		}
		if !page.MustEval(`() => { const cover=document.querySelector('.bs-preview-art.is-missing .bs-cover-fallback b'); const hero=document.querySelector('.bs-paper-page>h2'); return cover?.textContent==='fpgas_for_dummies_ebook' && hero?.textContent==='fpgas_for_dummies_ebook' && getComputedStyle(cover).overflowWrap==='anywhere' && getComputedStyle(hero).overflowWrap==='anywhere' && cover.scrollWidth<=cover.clientWidth+1 && hero.scrollWidth<=hero.clientWidth+1; }`).Bool() {
			t.Fatalf("long Bookshelf title overflowed its cover or featured preview in %s theme", theme)
		}
		page.MustElement(`[data-read="no-cover"]`).MustClick()
		page.MustWait(`() => { const notice=document.querySelector('.bs-notice'); return notice && notice.textContent===window.ctx.t('bookshelf.error_source_changed'); }`)
		if notice := page.MustElement(".bs-notice").MustText(); strings.Contains(notice, "source_changed") {
			t.Fatalf("source_changed was shown raw in %s theme: %q", theme, notice)
		}
	}
	page.MustSetViewport(1280, 860, 1, false)
	selectBook("siddhartha")
	for _, theme := range []string{"standard", "fruity"} {
		assertBookshelfUploadFailureUX(t, page, uploadRequests, theme)
	}
	assertBookshelfLateUploadReply(t, page, uploadRequests, lateUploadStarted, releaseLateUploadNow)
	dir := filepath.Join("..", "reports", "bookshelf")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"standard", "fruity", "fruity-dark"} {
		page.MustEval(`theme=>{document.body.dataset.theme=theme.startsWith('fruity')?'fruity':theme;document.body.dataset.fruityMode=theme==='fruity-dark'?'dark':'light';}`, theme)
		for _, size := range []struct {
			width, height int
			name          string
		}{{1440, 1024, "reference"}, {1280, 800, "wide"}, {1024, 768, "medium"}, {390, 844, "narrow"}} {
			page.MustSetViewport(size.width, size.height, 1, false)
			if size.width < 500 {
				if !page.MustEval(`() => getComputedStyle(document.querySelector('.bs-reading-preview')).display==='none' && document.querySelector('.bs-library-view').getBoundingClientRect().width>0`).Bool() {
					t.Fatal("narrow layout did not switch to the single library view")
				}
				if page.MustEval(`() => { const toolbar=document.querySelector('.bs-toolbar'); return toolbar.scrollWidth > toolbar.clientWidth+1; }`).Bool() {
					t.Fatal("narrow toolbar overflows horizontally")
				}
			} else if page.MustEval(`() => document.querySelector('.bs-app').scrollWidth>document.querySelector('.bs-app').clientWidth+1`).Bool() {
				t.Fatal("wide layout overflows horizontally")
			}
			page.MustScreenshot(filepath.Join(dir, fmt.Sprintf("reading-salon-%s-%s.png", theme, size.name)))
		}
	}
	page.MustSetViewport(1280, 860, 1, false)
	page.MustElement(`[data-details="siddhartha"]`).MustClick()
	page.MustElement(".bs-details-panel:not([hidden])")
	if title := page.MustElement(".bs-details-panel h2").MustText(); title != "Siddhartha" {
		t.Fatalf("book response envelope was not unpacked: details title %q", title)
	}
	page.MustElement(`[data-action="details-close"]`).MustClick()
	search := page.MustElement("[data-search]")
	search.MustInput("dispossessed")
	page.MustWait(`() => document.querySelectorAll('.bs-book').length===1 && document.querySelector('.bs-book-title')?.textContent.includes('Dispossessed')`)
	page.MustElement(`[data-action="add"]`).MustClick()
	page.MustElement(`[data-add-choice="source"]`).MustClick()
	page.MustElement(`dialog [name="provider"]`).MustSelect("WebDAV")
	page.MustElement(`dialog [name="name"]`).MustInput("Remote books")
	page.MustElement(`[data-pick-folder]`).MustClick()
	page.MustElement(`[data-folder-enter="Books"]`).MustClick()
	page.MustWait(`() => document.querySelector('[data-folder-picker]')?.textContent.includes('sample.epub')`)
	page.MustElement(`[data-folder-use]`).MustClick()
	page.MustElement(`dialog [type="submit"]`).MustClick()
	page.MustWait(`() => !document.querySelector('dialog').open`)
	select {
	case source := <-createdSources:
		if source["provider"] != "webdav" || source["path"] != "Books" || source["mount_id"] != nil && source["mount_id"] != "" {
			t.Fatalf("incorrect WebDAV source payload: %#v", source)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("source setup did not submit a request")
	}
	page.MustWait(`() => document.querySelector('.bs-notice')?.textContent.includes('some folders unavailable')`)
	search.MustSelectAllText().MustInput("")
	page.MustWait(`() => document.querySelectorAll('.bs-book').length===9`)
	page.MustElement(`[data-action="collections"]`).MustClick()
	page.MustElement(`[data-collection="classics"]`).MustClick()
	page.MustWait(`() => document.querySelectorAll('.bs-book').length===4 && document.querySelector('.bs-book-select').dataset.book==='siddhartha'`)
	page.MustElement(`[data-book-order="down"][data-book-id="siddhartha"]`).MustClick()
	page.MustWait(`() => document.querySelector('.bs-book-select').dataset.book==='1984'`)
	page.MustScreenshot(filepath.Join(dir, "collection-ordered.png"))
	page.MustElement(`[data-action="library"]`).MustClick()
	page.MustWait(`() => document.querySelectorAll('.bs-book').length===9`)
	page.MustElement(`.bs-book-select[data-book="1984"]`).MustClick()
	page.MustWait(`() => document.querySelector('.bs-app').dataset.page==='reader' && !!document.querySelector('[data-toc]') && typeof BookshelfApp.instances.get('bookshelf-fixture').reader?.goTo==='function'`)
	page.MustEval(`() => { const state=BookshelfApp.instances.get('bookshelf-fixture'); state.readerData={isFixedLayout:true}; state.selected.format='pdf'; state.toc=[{label:'Part I',href:'part.xhtml',subitems:[{label:'Nested chapter',locator:{cfi:'epubcfi(/6/4[chapter2]!)'}}]}]; state.__originalGoTo=state.reader.goTo; state.reader.goTo=locator=>{window.__bookshelfTOCLocator=locator;}; }`)
	page.MustElement(`[data-reader-action="settings"]`).MustClick()
	if !page.MustEval(`() => !!document.querySelector('[data-pref="fit"]') && !!document.querySelector('[data-pref="zoom"]') && !document.querySelector('[data-pref="width"]') && !document.querySelector('[data-pref="theme"]')`).Bool() {
		t.Fatal("fixed-layout settings did not render only their supported controls")
	}
	if errs := page.MustEval(`() => JSON.stringify(window.errors)`).Str(); errs != "[]" {
		t.Fatalf("fixed-layout settings raised a browser error: %s", errs)
	}
	page.MustElement(`[data-reader-action="toc"]`).MustClick()
	if !page.MustEval(`() => { const child=document.querySelector('[data-toc="0.0"]'); return child instanceof HTMLButtonElement && !!child.closest('ol ol') && child.tabIndex>=0; }`).Bool() {
		t.Fatal("nested table-of-contents chapter is missing or not keyboard accessible")
	}
	page.MustElement(`[data-toc="0.0"]`).MustClick()
	if !page.MustEval(`() => window.__bookshelfTOCLocator?.cfi === 'epubcfi(/6/4[chapter2]!)'`).Bool() {
		t.Fatal("nested table-of-contents navigation lost its chapter locator")
	}
	page.MustEval(`() => { let item={label:'Deepest',locator:'deepest'}; for(let i=0;i<10000;i++) item={label:'Depth '+i,subitems:[item]}; BookshelfApp.instances.get('bookshelf-fixture').toc=[item]; }`)
	page.MustElement(`[data-reader-action="settings"]`).MustClick()
	page.MustElement(`[data-reader-action="toc"]`).MustClick()
	if count := page.MustEval(`() => document.querySelectorAll('[data-toc]').length`).Int(); count != 16 {
		t.Fatalf("deep table of contents rendered %d items before the depth bound, want 16", count)
	}
	if errs := page.MustEval(`() => JSON.stringify(window.errors)`).Str(); errs != "[]" {
		t.Fatalf("deep table of contents raised a browser error: %s", errs)
	}
	if visible := page.MustEval(`() => document.querySelector('.bs-library-view').getBoundingClientRect().height > 0`).Bool(); visible {
		t.Fatal("reader mode must hide the library and use the available window")
	}
	page.MustScreenshot(filepath.Join(dir, "reader-from-shelf.png"))
	page.MustElement(`[data-action="back"]`).MustClick()
	page.MustWait(`() => document.querySelector('.bs-app').dataset.page==='library' && !document.querySelector('[data-reader-host] iframe')`)
	search.MustSelectAllText().MustInput("__bookshelf_slow_search__")
	select {
	case <-slowSearchStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("slow Bookshelf query did not start")
	}
	if request := <-bookLoadRequests; request != "__bookshelf_slow_search__:0" {
		t.Fatalf("slow Bookshelf query used unexpected page: %q", request)
	}
	search.MustSelectAllText().MustInput("__bookshelf_latest_search__")
	select {
	case request := <-bookLoadRequests:
		if request != "__bookshelf_latest_search__:0" {
			t.Fatalf("latest Bookshelf query used unexpected page: %q", request)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("latest Bookshelf query was dropped while an older request was pending")
	}
	page.MustWait(`() => document.querySelector('[data-book="dispossessed"]') && BookshelfApp.instances.get('bookshelf-fixture').query==='__bookshelf_latest_search__'`)
	page.MustElement(`[data-action="page"][data-offset="36"]`).MustClick()
	select {
	case request := <-bookLoadRequests:
		if request != "__bookshelf_latest_search__:36" {
			t.Fatalf("Bookshelf next page used an unexpected offset: %q", request)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Bookshelf next-page request did not start")
	}
	page.MustWait(`() => document.querySelector('[data-book="siddhartha"]') && BookshelfApp.instances.get('bookshelf-fixture').offset===36`)
	releaseSlowSearchOnce.Do(func() { close(releaseSlowSearch) })
	page.MustWait(`() => window.__slowResponseParsed && document.querySelector('[data-book="siddhartha"]') && BookshelfApp.instances.get('bookshelf-fixture').offset===36`)
	if errs := page.MustEval(`() => JSON.stringify(window.errors)`).Str(); errs != "[]" {
		t.Fatalf("overlapping Bookshelf queries raised a browser error: %s", errs)
	}
	search.MustSelectAllText().MustInput("__bookshelf_dispose_search__")
	page.MustWait(`() => window.__disposeRequestStarted`)
	page.MustEval(`() => { window.__shelfBeforeDispose=document.querySelector('[data-shelf]').innerHTML; window.__bookshelfStateBeforeDispose=BookshelfApp.instances.get('bookshelf-fixture'); window.__totalBeforeDispose=window.__bookshelfStateBeforeDispose.total; }`)
	page.MustEval(`() => { BookshelfApp.dispose('bookshelf-fixture'); window.__releaseDisposeRequest(); }`)
	page.MustWait(`() => window.__disposeResponseParsed`)
	if !page.MustEval(`() => document.querySelector('[data-shelf]').innerHTML===window.__shelfBeforeDispose && window.__bookshelfStateBeforeDispose.total===window.__totalBeforeDispose`).Bool() {
		t.Fatal("a Bookshelf request completed into a disposed app instance")
	}
	if errs := page.MustEval(`() => JSON.stringify(window.errors)`).Str(); errs != "[]" {
		t.Fatalf("browser errors: %s", errs)
	}
	if os.Getenv("AURAGO_BOOKSHELF_FIXTURE_HOLD") == "1" {
		fmt.Printf("Bookshelf fixture is available at %s/bookshelf-fixture. Press Ctrl+C to stop.\n", fixtureURL)
		select {}
	}
}

func assertBookshelfUploadFailureUX(t *testing.T, page *rod.Page, uploads <-chan string, theme string) {
	t.Helper()
	page.MustEval(`theme => { document.body.dataset.theme=theme; document.body.dataset.fruityMode='light'; }`, theme)
	openBookshelfUploadDialog(page)
	oversizedName := "too-large-" + theme + ".epub"
	setBookshelfUploadFiles(page, map[string]any{"name": oversizedName, "size": 17})
	page.MustEval(`() => document.querySelector('dialog form').requestSubmit()`)
	page.MustWait(fmt.Sprintf(`() => { const error=document.querySelector('[data-form-error]'); return !error.hidden && error.textContent.includes(%q) && error.textContent.includes('16 B'); }`, oversizedName))
	if !page.MustEval(fmt.Sprintf(`() => document.querySelector('[data-form-error]').textContent === window.ctx.t('bookshelf.upload_too_large').replace('{name}', %q).replace('{limit}', '16 B')`, oversizedName)).Bool() {
		t.Fatalf("%s preflight error did not use the localized size-limit message", theme)
	}
	select {
	case got := <-uploads:
		t.Fatalf("%s oversized preflight unexpectedly uploaded %q", theme, got)
	case <-time.After(100 * time.Millisecond):
	}

	serverLimitedName := "server-limit-" + theme + ".epub"
	setBookshelfUploadFiles(page,
		map[string]any{"name": "before-limit-" + theme + ".epub", "size": 1},
		map[string]any{"name": serverLimitedName, "size": 2},
	)
	page.MustWait(`() => document.querySelector('[data-form-error]').hidden`)
	page.MustEval(`() => document.querySelector('dialog form').requestSubmit()`)
	waitForBookshelfUpload(t, uploads, "before-limit-"+theme+".epub")
	waitForBookshelfUpload(t, uploads, serverLimitedName)
	page.MustWait(fmt.Sprintf(`() => { const error=document.querySelector('[data-form-error]'); const expected=window.ctx.t('bookshelf.upload_too_large').replace('{name}', %q).replace('{limit}', '1 B')+' '+window.ctx.t('bookshelf.upload_partial').replace('{done}', '1').replace('{total}', '2'); const button=document.querySelector('dialog [type=submit]'); const input=document.querySelector('dialog [name=files]'); return !error.hidden && error.textContent===expected && !button.disabled && button.textContent===window.ctx.t('bookshelf.upload') && !input.disabled && document.querySelector('[data-upload-limit]').textContent===window.ctx.t('bookshelf.upload_limit').replace('{limit}', '1 B'); }`, serverLimitedName))
	waitForBookshelfShelf(page)
	setBookshelfUploadFiles(page, map[string]any{"name": "replacement-" + theme + ".epub", "size": 1})
	page.MustWait(`() => document.querySelector('[data-form-error]').hidden`)
	page.MustEval(`() => document.querySelector('dialog form').requestSubmit()`)
	waitForBookshelfUpload(t, uploads, "replacement-"+theme+".epub")
	page.MustWait(`() => !document.querySelector('dialog').open`)
	waitForBookshelfShelf(page)

	openBookshelfUploadDialog(page)
	knownName := "known-" + theme + ".epub"
	retryName := "retry-" + theme + ".epub"
	setBookshelfUploadFiles(page, map[string]any{"name": knownName, "size": 1}, map[string]any{"name": retryName, "size": 1})
	page.MustEval(`() => document.querySelector('dialog form').requestSubmit()`)
	waitForBookshelfUpload(t, uploads, knownName)
	waitForBookshelfUpload(t, uploads, retryName)
	page.MustWait(fmt.Sprintf(`() => { const error=document.querySelector('[data-form-error]'); const expected=window.ctx.t('bookshelf.upload_failed').replace('{name}', %q)+' '+window.ctx.t('bookshelf.upload_partial').replace('{done}', '1').replace('{total}', '2'); const button=document.querySelector('dialog [type=submit]'); const input=document.querySelector('dialog [name=files]'); return !error.hidden && error.textContent===expected && !button.disabled && button.textContent===window.ctx.t('bookshelf.upload') && !input.disabled; }`, retryName))
	waitForBookshelfShelf(page)
	page.MustEval(`() => document.querySelector('dialog form').requestSubmit()`)
	waitForBookshelfUpload(t, uploads, retryName)
	page.MustWait(`() => !document.querySelector('dialog').open`)
	waitForBookshelfShelf(page)
}

func assertBookshelfLateUploadReply(t *testing.T, page *rod.Page, uploads <-chan string, started <-chan struct{}, release func()) {
	t.Helper()
	openBookshelfUploadDialog(page)
	page.MustEval(`() => { window.__bookshelfLateUploadBefore=window.__uploadResponsesParsed; }`)
	setBookshelfUploadFiles(page, map[string]any{"name": "late-reply.epub", "size": 1})
	page.MustEval(`() => document.querySelector('dialog form').requestSubmit()`)
	waitForBookshelfUpload(t, uploads, "late-reply.epub")
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("late upload did not reach the fixture")
	}
	page.MustElement(`dialog [data-dialog-close]`).MustClick()
	page.MustWait(`() => !document.querySelector('dialog').open`)
	openBookshelfUploadDialog(page)
	setBookshelfUploadFiles(page, map[string]any{"name": "after-late-reply.epub", "size": 1})
	release()
	page.MustWait(`() => window.__uploadResponsesParsed > window.__bookshelfLateUploadBefore`)
	waitForBookshelfShelf(page)
	if !page.MustEval(`() => { const dialog=document.querySelector('dialog'); const input=dialog.querySelector('[name=files]'); const error=dialog.querySelector('[data-form-error]'); return dialog.open && input.files[0]?.name==='after-late-reply.epub' && error.hidden; }`).Bool() {
		t.Fatal("a late upload response changed or closed the newly opened upload dialog")
	}
	page.MustEval(`() => document.querySelector('dialog form').requestSubmit()`)
	waitForBookshelfUpload(t, uploads, "after-late-reply.epub")
	page.MustWait(`() => !document.querySelector('dialog').open`)
	waitForBookshelfShelf(page)
}

func openBookshelfUploadDialog(page *rod.Page) {
	page.MustElement(`[data-action="add"]`).MustClick()
	page.MustElement(`[data-add-choice="upload"]`).MustClick()
	page.MustElement(`dialog [name="files"]`)
}

func waitForBookshelfShelf(page *rod.Page) {
	page.MustWait(`() => { const state=BookshelfApp.instances.get('bookshelf-fixture'); return state && !state.loading && state.books.length===9 && document.querySelectorAll('.bs-book').length===9; }`)
}

func setBookshelfUploadFiles(page *rod.Page, files ...map[string]any) {
	page.MustEval(`files => { const input=document.querySelector('dialog [name="files"]'); const transfer=new DataTransfer(); for (const file of files) transfer.items.add(new File([new Uint8Array(file.size)], file.name, {type:'application/epub+zip'})); input.files=transfer.files; input.dispatchEvent(new Event('change', {bubbles:true})); }`, files)
}

func waitForBookshelfUpload(t *testing.T, uploads <-chan string, want string) {
	t.Helper()
	select {
	case got := <-uploads:
		if got != want {
			t.Fatalf("upload request used filename %q, want %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("upload request for %q did not reach the fixture", want)
	}
}

func sampleBookshelfFixtureBooks() []map[string]any {
	titles := [][2]string{{"Siddhartha", "Hermann Hesse"}, {"1984", "George Orwell"}, {"The Dispossessed", "Ursula K. Le Guin"}, {"The Stranger", "Albert Camus"}, {"The Hobbit", "J. R. R. Tolkien"}, {"Pride and Prejudice", "Jane Austen"}, {"The Left Hand of Darkness", "Ursula K. Le Guin"}, {"Meditations", "Marcus Aurelius"}, {"fpgas_for_dummies_ebook", "A. Very Long Author Name"}}
	books := make([]map[string]any, 0, len(titles))
	for index, item := range titles {
		id := []string{"siddhartha", "1984", "dispossessed", "stranger", "hobbit", "pride", "left-hand", "meditations", "no-cover"}[index]
		fraction := 0.0
		if index == 0 {
			fraction = .12
		} else if index == 3 {
			fraction = .73
		}
		books = append(books, map[string]any{
			"id": id, "title": item[0], "author": item[1], "format": "epub", "source_id": "shelf-local", "path": "Books/" + id + ".epub", "revision": 1, "available": true,
			"description": item[0] + " explores the quiet, restless questions that shape a life. A richly layered novel to return to slowly.",
			"progress":    map[string]any{"revision": 1, "locator": map[string]any{"cfi": "epubcfi(/6/2[chapter1]!)"}, "fraction": fraction},
		})
	}
	return books
}
