package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHorizonOptionalNoticeBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	var mu sync.Mutex
	read := map[int64]bool{}
	ackAttempts, lifecycleCalls := 0, 0
	openedConfig := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, chatCodeBlockFixture)
		case "/config":
			select {
			case openedConfig <- struct{}{}:
			default:
			}
			fmt.Fprint(w, "<!doctype html><title>Guardian settings</title>")
		case "/api/system/notifications":
			mu.Lock()
			defer mu.Unlock()
			notes := []map[string]interface{}{}
			for _, note := range []map[string]interface{}{
				{"id": int64(7), "type": "security_advisory", "message": "English fallback", "data": map[string]interface{}{"code": "horizon_optional", "download_bytes": 302796789, "runtime_bytes": 8200000}},
				{"id": int64(8), "type": "legacy", "message": "Other system notice"},
				{"id": int64(9), "type": "legacy", "message": "intentional-render-failure"},
			} {
				if !read[note["id"].(int64)] {
					notes = append(notes, note)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"notifications": notes})
		case "/api/system/notifications/read":
			var body struct {
				IDs []int64 `json:"ids"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			ackAttempts++
			if !reflect.DeepEqual(body.IDs, []int64{7, 8}) {
				t.Errorf("acknowledged unrendered notifications: %v", body.IDs)
			}
			if ackAttempts == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			for _, id := range body.IDs {
				read[id] = true
			}
			fmt.Fprint(w, `{"status":"ok"}`)
		default:
			if strings.HasPrefix(r.URL.Path, "/api/guardian/") {
				mu.Lock()
				lifecycleCalls++
				mu.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	browser := newSmokeBrowser(t)
	const setup = `async()=>{
 const source=await(await fetch('/js/chat/chat-history.js')).text();
 const locale=await(await fetch('/lang/chat/de.json')).json();
 window.t=key=>locale[key]||key;window.conversation=[];
 (0,eval)(source.replace(/if \(document\.readyState === 'loading'\)[\s\S]*$/, ''));
 const append=window.appendMessage;
 window.appendMessage=(role,text)=>{if(text.includes('intentional-render-failure'))throw Error('fixture rendering failure');append(role,text)};
 return true;
}`
	for _, theme := range []string{"dark", "light"} {
		for _, width := range []int{390, 1440} {
			t.Run(fmt.Sprintf("%s/%d", theme, width), func(t *testing.T) {
				mu.Lock()
				read = map[int64]bool{}
				ackAttempts = 0
				mu.Unlock()
				page := browser.MustPage("about:blank")
				defer page.MustClose()
				page.MustSetViewport(width, 900, 1, false)
				page.MustNavigate(server.URL + "/?theme=" + theme).MustWaitLoad()
				page.MustEval(setup)
				if !page.MustEval(`async()=>{await loadChatSystemNotifications(()=>false);return chatContent.children.length===0}`).Bool() {
					t.Fatal("stale history received a notice")
				}
				result := page.MustEval(`async()=>{
 await loadChatSystemNotifications();
 const text=chatContent.textContent;
 return {count:chatContent.querySelectorAll('.bubble').length,localized:text.includes('Du entscheidest')&&text.includes('303 MB')&&text.includes('9 MB'),
  leaked:text.includes('{{')||text.includes('English fallback'),history:conversation.length,
  overflow:document.documentElement.scrollWidth>innerWidth+1,link:!!chatContent.querySelector('a[href="/config#guardian"]')};
}`).Map()
				if result["count"].Int() != 2 || !result["localized"].Bool() || result["leaked"].Bool() || result["history"].Int() != 0 || result["overflow"].Bool() || !result["link"].Bool() {
					t.Fatalf("incorrect advisory display: %v", result)
				}
				if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("horizon-notice-%s-%d.png", theme, width)), page.MustScreenshot(), 0644); err != nil {
						t.Fatal(err)
					}
				}
				page.MustElement(`a[href="/config#guardian"]`).MustClick()
				select {
				case <-openedConfig:
				case <-time.After(5 * time.Second):
					t.Fatal("settings link did not open settings")
				}
				if !page.MustEval(`async()=>{chatContent.innerHTML='';await loadChatSystemNotifications();return chatContent.querySelectorAll('.bubble').length===2&&conversation.length===0}`).Bool() {
					t.Fatal("failed acknowledgment lost the notice")
				}
				// A fresh page has no browser-local receipt; the shared server receipt suppresses replay.
				page.MustNavigate(server.URL + "/?theme=" + theme).MustWaitLoad()
				page.MustEval(setup)
				if !page.MustEval(`async()=>{await loadChatSystemNotifications();return chatContent.children.length===0&&conversation.length===0}`).Bool() {
					t.Fatal("a read advisory replayed in a fresh page")
				}
				mu.Lock()
				defer mu.Unlock()
				if ackAttempts != 2 || lifecycleCalls != 0 || read[9] {
					t.Fatalf("unexpected side effects: acknowledgments=%d lifecycle=%d readFailed=%t", ackAttempts, lifecycleCalls, read[9])
				}
			})
		}
	}
}
