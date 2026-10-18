package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// TestDashboardDisclosuresBrowser renders the real dashboard against stubbed section data in headless Chrome and
// checks the P5 big cards end to end:
//   - the prompt KPI row renders from the fixture, every prompt disclosure starts closed and nothing is stored;
//   - a disclosure opened through its summary is stored, open again after a reload, and stays open through a section
//     refresh and a CardState error/restore cycle; closing it forgets it;
//   - the memory KPI row renders, and its core facts KPI opens the core facts modal through the delegated action.
func TestDashboardDisclosuresBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("browser required for the dashboard disclosure smoke test")
	}
	page := renderDashboardForBrowser(t)
	fixtures := map[string]any{
		"/api/dashboard/summary": dashboardOverviewSummaryFixture(true, 0),
		"/api/dashboard/prompt-stats": map[string]any{
			"total_builds": 8, "avg_tokens": 3955, "fit_count": 2, "measured_fit_count": 2, "budget_shed_count": 1,
			"avg_optimization_pct": 12.5, "request_count": 300, "cache_hit_count": 250,
			"tier_distribution": map[string]any{"full": 8},
			"recent":            []any{map[string]any{"raw_len": 15821, "optimized_len": 15814}},
			"recent_fits":       []any{map[string]any{"input_tokens": 1200, "output_tokens": 800}},
			"avg_section_sizes": map[string]any{"modules": 12000, "context": 3821},
		},
		"/api/dashboard/memory": map[string]any{"core_memory_facts": 3, "chat_messages": 120, "vectordb_entries": 42, "knowledge_graph": map[string]any{"nodes": 7, "edges": 9}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch body, stubbed := fixtures[r.URL.Path]; {
		case r.URL.Path == "/start":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!doctype html><title>start</title>"))
		case r.URL.Path == "/dashboard":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
		case r.URL.Path == "/events":
			// A quiet event stream: the page stays live and gets no events.
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(": connected\n\n"))
			flusher.Flush()
			<-r.Context().Done()
		case r.URL.Path == "/sw.js":
			http.NotFound(w, r)
		case stubbed:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case strings.HasPrefix(r.URL.Path, "/api/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{}"))
		default:
			http.FileServer(http.FS(Content)).ServeHTTP(w, r)
		}
	}))
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	l := launcher.New().Context(ctx).Bin(bin).Headless(true).NoSandbox(true)
	u, err := l.Launch()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { l.Kill(); l.Cleanup() }()
	b := rod.New().Context(ctx).ControlURL(u).MustConnect()
	defer b.Close()
	// Start on a page of the stub server: rod stalls after an about:blank hop on this host.
	p := b.MustPage(server.URL + "/start").MustWaitLoad()
	defer p.Close()

	var errMu sync.Mutex
	var pageErrors []string
	go p.EachEvent(func(e *proto.RuntimeExceptionThrown) {
		errMu.Lock()
		defer errMu.Unlock()
		pageErrors = append(pageErrors, "exception: "+e.ExceptionDetails.Text+" "+dashboardOverviewRemoteText(e.ExceptionDetails.Exception))
	}, func(e *proto.RuntimeConsoleAPICalled) {
		if e.Type != proto.RuntimeConsoleAPICalledTypeError {
			return
		}
		parts := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			parts = append(parts, dashboardOverviewRemoteText(arg))
		}
		text := strings.Join(parts, " ")
		// The stub serves no service worker (404); shared-core logs that registration failure.
		if strings.Contains(text, "[PWA]") {
			return
		}
		errMu.Lock()
		defer errMu.Unlock()
		pageErrors = append(pageErrors, "console.error: "+text)
	})()

	wait := func(js string) {
		t.Helper()
		p.Timeout(20 * time.Second).MustWait(js)
	}
	navs := 0
	open := func(id string) {
		t.Helper()
		navs++
		p.Timeout(30 * time.Second).MustNavigate(server.URL + "/dashboard?nav=" + strconv.Itoa(navs) + "#" + id).MustWaitLoad()
		wait(fmt.Sprintf(`() => !!window.DashRouter && DashRouter.current() === %q && document.querySelector('[data-section-updated="%s"]').textContent !== ''`, id, id))
	}
	const base = `document.querySelector('[data-disclosure="card-prompt:base"]')`
	p.MustSetViewport(1440, 1000, 1, false)

	// First visit: KPI values from the fixture, every disclosure closed, nothing stored.
	open("prompts")
	first := p.MustEval(`() => ({
		values: [...document.querySelectorAll('#prompt-headline-kpis .dash-kpi-value')].map(el => el.textContent).join('|'),
		closed: [...document.querySelectorAll('#card-prompt details[data-disclosure]')].filter(d => !d.open).length,
		stored: localStorage.getItem('dash-disclosures') === null,
	})`)
	if first.Get("values").Str() != "8|3,955|1|12.5 %" || first.Get("closed").Int() != 5 || !first.Get("stored").Bool() {
		t.Fatalf("prompt card on the first visit: %s", first.JSON("", ""))
	}

	// Open "base" through its summary (a JS click: rod's hit test can stall on the summary's ::before chevron).
	p.MustEval(`() => ` + base + `.querySelector(':scope > summary').click()`)
	wait(`() => ` + base + `.open && localStorage.getItem('dash-disclosures') === '{"card-prompt:base":true}'`)

	// Reload: open again, the other four stay closed.
	open("prompts")
	wait(`() => ` + base + `.open && document.querySelectorAll('#card-prompt details[data-disclosure][open]').length === 1`)

	// A section refresh reloads the data and keeps the block open.
	p.MustEval(`() => { document.querySelector('[data-section-updated="prompts"]').textContent = 'stamp-before'; }`)
	p.MustElement(`[data-section-refresh="prompts"]`).MustClick()
	wait(`() => document.querySelector('[data-section-updated="prompts"]').textContent !== 'stamp-before' && ` + base + `.open`)

	// CardState error, then restore: the card body comes back from its saved markup with the block open.
	if !p.MustEval(`() => {
		CardState.setError('card-prompt', () => {});
		const failed = !!document.querySelector('#card-prompt .dash-error-state');
		CardState.setLoaded('card-prompt');
		return failed && ` + base + `.open && document.querySelectorAll('#prompt-headline-kpis .dash-kpi').length === 4;
	}`).Bool() {
		t.Fatal("the base disclosure must stay open through a CardState error and restore")
	}

	// Closing forgets it.
	p.MustEval(`() => ` + base + `.querySelector(':scope > summary').click()`)
	wait(`() => !` + base + `.open && localStorage.getItem('dash-disclosures') === '{}'`)

	// Memory: KPI values, and the core facts KPI opens the core facts modal (delegated data-action).
	open("memory")
	if values := p.MustEval(`() => [...document.querySelectorAll('#memory-kpis .dash-kpi-value')].map(el => el.textContent).join('|')`).Str(); values != "3|120|42|7" {
		t.Fatalf("memory KPI values = %q, want 3|120|42|7", values)
	}
	p.MustElement(`#memory-kpis [data-action="memory-core-facts"]`).MustClick()
	wait(`() => document.getElementById('cfOverlay').classList.contains('open')`)

	errMu.Lock()
	defer errMu.Unlock()
	if len(pageErrors) > 0 {
		t.Fatalf("dashboard page errors:\n%s", strings.Join(pageErrors, "\n"))
	}
}
