package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSelfTestDashboardBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	dashboard := renderDashboardForBrowser(t)
	id := strings.Repeat("a", 32)
	var starts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/dashboard":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(dashboard)
		case r.URL.Path == "/api/self-tests/catalog":
			_ = json.NewEncoder(w).Encode(map[string]any{"inventory": []any{map[string]any{"name": "synthetic_tool", "enabled": true}}})
		case r.URL.Path == "/api/self-tests" && r.Method == "POST":
			starts.Add(1)
			w.WriteHeader(202)
			_, _ = fmt.Fprintf(w, `{"id":%q,"status":"complete","summary":{"enabled_tools":1,"executed_tools":1,"verdicts":{"PASS":1}},"cases":[{"template_id":"SYS-01","tool":"<img src=x onerror=alert(1)>","verdict":"PASS","cleanup":"verified"}]}`, id)
		case r.URL.Path == "/api/self-tests":
			_, _ = w.Write([]byte(`{"runs":[]}`))
		case r.URL.Path == "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(": connected\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
		case r.URL.Path == "/sw.js":
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/"):
			_, _ = w.Write([]byte(`{}`))
		default:
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	t.Cleanup(func() { server.CloseClientConnections(); server.Close() })
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"light", "dark"} {
		for _, width := range []int{1440, 390} {
			t.Run(fmt.Sprintf("%s-%d", theme, width), func(t *testing.T) {
				p := browser.MustPage("about:blank")
				defer p.MustClose()
				p.MustSetViewport(width, 950, 1, false)
				p.MustNavigate(server.URL + "/dashboard#tools").MustWaitLoad()
				p.Timeout(20 * time.Second).MustWait(`() => window.AuraSelfTests && document.getElementById('selftest-target').options.length === 2 && !document.getElementById('sec-tools').hidden`)
				p.MustEval(`theme => document.documentElement.setAttribute('data-theme', theme)`, theme)
				if p.MustEval(`() => document.querySelector('#card-selftest details').open`).Bool() {
					t.Fatal("case details must start closed")
				}
				p.MustEval(`() => { const s=document.getElementById('selftest-profile'); s.value='targeted'; s.dispatchEvent(new Event('change', {bubbles:true})); }`)
				p.Timeout(5 * time.Second).MustWait(`() => !document.getElementById('selftest-bindings-panel').hidden`)
				p.MustEval(`() => { const s=document.getElementById('selftest-profile'); s.value='read'; s.dispatchEvent(new Event('change', {bubbles:true})); }`)
				p.MustElement("#selftest-start").MustClick()
				p.Timeout(10 * time.Second).MustWait(`() => document.querySelectorAll('#selftest-rows tr').length === 1 && !document.getElementById('selftest-report').hidden`)
				if p.MustEval(`() => !!document.querySelector('#selftest-rows img') || document.getElementById('selftest-stop').disabled !== true`).Bool() {
					t.Fatal("unsafe result rendering or incorrect completed controls")
				}
				if !p.MustEval(`() => document.getElementById('selftest-report').getAttribute('href').endsWith('/report.md')`).Bool() {
					t.Fatal("report link missing")
				}
				if p.MustEval(`() => { const c=document.getElementById('card-selftest'); return c.scrollWidth > c.clientWidth + 1; }`).Bool() {
					t.Fatal("self-test card overflows")
				}
			})
		}
	}
	if starts.Load() != 4 {
		t.Fatalf("unexpected starts: %d", starts.Load())
	}
}
