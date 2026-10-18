package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"html/template"
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

// dashboardOverviewSummaryFixture is the stubbed GET /api/dashboard/summary answer: one critical, two warning and
// one info attention row. health stays null (as before the collector's first 10 s tick) until withHealth is
// set; more is attention_more.
func dashboardOverviewSummaryFixture(withHealth bool, more int) map[string]any {
	var health any
	unavailable := []string{"health"}
	if withHealth {
		health = map[string]any{"cpu_percent": 23.4, "memory_percent": 61.2, "disk_percent": 72.6, "uptime_seconds": 7260, "sampled_at": "2026-10-18T09:40:50+02:00"}
		unavailable = []string{}
	}
	return map[string]any{
		"generated_at":        "2026-10-18T09:41:00+02:00",
		"overall":             "critical",
		"agent":               map[string]any{"model": "fixture-model", "provider": "openrouter", "personality": "friend", "context_window": 128000, "busy": false, "debug": false, "maintenance": true, "mood": "focused"},
		"context":             map[string]any{"total_chars": 64000},
		"last_activity_hours": 2.4,
		"maintenance":         map[string]any{"enabled": true, "last_run": "2026-10-18T03:00:00+02:00", "last_status": "completed", "next_run": "2026-10-19T03:00:00+02:00"},
		"health":              health,
		"attention": []any{
			map[string]any{"id": "operational_issue:abc123", "severity": "critical", "code": "operational_issue", "params": map[string]any{"title": "Backup job keeps failing", "kind": "runtime", "source": "cron", "occurrences": 3}, "section": "problems", "count": 1, "at": "2026-10-18T09:30:00+02:00"},
			map[string]any{"id": "cron_errors", "severity": "warning", "code": "cron_errors", "params": map[string]any{"first_id": "nightly-backup"}, "section": "automations", "count": 1, "at": ""},
			map[string]any{"id": "budget_warning", "severity": "warning", "code": "budget_warning", "params": map[string]any{"percent": 85}, "section": "llm", "count": 1, "at": ""},
			map[string]any{"id": "memory_conflicts", "severity": "info", "code": "memory_conflicts", "params": map[string]any{}, "section": "memory", "count": 2, "at": ""},
		},
		"attention_more": more,
		"badges":         map[string]any{"problems": 1, "automations": 1},
		"kpis": map[string]any{
			"budget":      map[string]any{"enabled": true, "spent_usd": 4.25, "daily_limit_usd": 5, "percentage": 0.85, "state": "warning"},
			"automations": map[string]any{"missions_running": 1, "missions_queued": 2, "missions_failed_24h": 0, "cron_errors": 1, "daemons_failed": 0},
			"knowledge":   map[string]any{"kg_nodes": 321, "kg_edges": 654, "core_facts": 12},
		},
		"recent_activity": []any{
			map[string]any{"id": 7, "timestamp": "2026-10-18T09:35:00+02:00", "source": "mission", "event_type": "mission_run", "target_name": "Nightly report", "status": "success", "duration_ms": 1200, "section": "automations"},
			map[string]any{"id": 6, "timestamp": "2026-10-18T09:20:00+02:00", "source": "agent_tool", "event_type": "tool_call", "target_name": "web_search", "status": "error", "duration_ms": 300, "section": "audit"},
		},
		"unavailable": unavailable,
	}
}

// renderDashboardForBrowser executes the real template set (shell + 14 partials) with the English dashboard strings.
func renderDashboardForBrowser(t *testing.T) []byte {
	t.Helper()
	tmpl, err := template.ParseFS(Content, append([]string{"dashboard.html"}, dashboardSectionPartialFiles(t)...)...)
	if err != nil {
		t.Fatalf("parse dashboard template: %v", err)
	}
	var i18n map[string]string
	if err := json.Unmarshal(mustReadUIFile(t, "lang/dashboard/en.json"), &i18n); err != nil {
		t.Fatalf("lang/dashboard/en.json: %v", err)
	}
	payload, err := json.Marshal(map[string]any{"systemLang": "en", "buildVersion": "p3-test", "i18n": i18n})
	if err != nil {
		t.Fatal(err)
	}
	data := struct {
		Lang             string
		BuildVersion     string
		TemplateDataJSON template.JS
	}{Lang: "en", BuildVersion: "p3-test", TemplateDataJSON: template.JS(payload)}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatalf("execute dashboard template: %v", err)
	}
	return out.Bytes()
}

func dashboardOverviewRemoteText(o *proto.RuntimeRemoteObject) string {
	if o == nil {
		return ""
	}
	if o.Description != "" {
		return o.Description
	}
	return o.Value.String()
}

// TestDashboardOverviewBrowserSmoke renders the real dashboard against a stubbed summary and SSE stream and
// checks the overview in headless Chrome:
//   - attention rows, the overall pill and the sidebar badges;
//   - the waiting system tile, then the in-place system_metrics update;
//   - an SSE-triggered summary refresh;
//   - a failing first load and a failing refresh (500): error state and retry on all eight tiles, no new Updated stamp;
//   - hiding a tile through Customize (kept after a reload, reset by "Show all");
//   - the drawer at 700 px: section navigation, and the status block leading back to the overview.
func TestDashboardOverviewBrowserSmoke(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	bin, ok := browserExecutable()
	if !ok {
		t.Fatal("browser required for the dashboard overview smoke test")
	}
	page := renderDashboardForBrowser(t)

	var stateMu sync.Mutex
	withHealth, attentionMore, summaryFails := false, 0, false
	events := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/start":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!doctype html><title>start</title>"))
		case r.URL.Path == "/dashboard":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(page)
		case r.URL.Path == "/api/dashboard/summary":
			stateMu.Lock()
			body := dashboardOverviewSummaryFixture(withHealth, attentionMore)
			failing := summaryFails
			stateMu.Unlock()
			if failing {
				http.Error(w, "summary unavailable", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		case r.URL.Path == "/events":
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write([]byte(": connected\n\n"))
			flusher.Flush()
			for {
				select {
				case <-r.Context().Done():
					return
				case message := <-events:
					_, _ = w.Write([]byte("data: " + message + "\n\n"))
					flusher.Flush()
				}
			}
		case r.URL.Path == "/sw.js":
			http.NotFound(w, r)
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
	open := func(width int) {
		t.Helper()
		navs++
		p.MustSetViewport(width, 1000, 1, false)
		p.Timeout(30 * time.Second).MustNavigate(server.URL + "/dashboard?nav=" + strconv.Itoa(navs) + "#overview").MustWaitLoad()
		wait(`() => !!(window.DashRouter && DashRouter.current() === 'overview' && window.DashSummary && DashSummary.latest())`)
		wait(`() => document.querySelectorAll('#ov-attention-list a.ov-attention-row').length > 0 && document.getElementById('ov-agent-state-body').textContent.trim() !== ''`)
	}

	// The eight tiles all show the CardState error state with a retry control (failed) or none of them does (ok).
	const tiles = `['ov-status', 'ov-attention', 'ov-system', 'ov-budget', 'ov-automations', 'ov-knowledge', 'ov-activity', 'ov-agent-state']`
	const allFailed = `() => ` + tiles + `.every(id => { const tile = document.getElementById(id); return tile.dataset.state === 'error' && !!tile.querySelector('.dash-error-state .dash-error-retry'); })`
	const allRecovered = `() => ` + tiles + `.every(id => { const tile = document.getElementById(id); return tile.dataset.state === 'loaded' && !tile.querySelector('.dash-error-state'); })`

	// The first summary load fails (500): no tile may stay empty, and one retry restores them all.
	stateMu.Lock()
	summaryFails = true
	stateMu.Unlock()
	navs++
	p.MustSetViewport(1440, 1000, 1, false)
	p.Timeout(30 * time.Second).MustNavigate(server.URL + "/dashboard?nav=" + strconv.Itoa(navs) + "#overview").MustWaitLoad()
	wait(`() => !!window.DashRouter && DashRouter.current() === 'overview'`)
	wait(allFailed)
	if got := p.MustEval(`() => ({ latest: DashSummary.latest() === null, stamp: document.querySelector('[data-section-updated="overview"]').textContent })`); !got.Get("latest").Bool() || got.Get("stamp").Str() != "" {
		t.Fatalf("a failed first load must leave no summary and no Updated stamp: %s", got.JSON("", ""))
	}
	stateMu.Lock()
	summaryFails = false
	stateMu.Unlock()
	p.MustElement(`#ov-knowledge .dash-error-retry`).MustClick()
	wait(allRecovered)
	wait(`() => document.querySelectorAll('#ov-attention-list a.ov-attention-row').length > 0 && document.querySelector('[data-section-updated="overview"]').textContent !== ''`)

	// 1440 px: rows, pill, badges, tiles; the SSE stream is open (live dot green).
	open(1440)
	wait(`() => document.querySelector('[data-sidebar-live]').classList.contains('green')`)
	got := p.MustEval(`() => {
		const rows = [...document.querySelectorAll('#ov-attention-list a.ov-attention-row')];
		const badge = id => { const el = document.querySelector('[data-nav-badge="' + id + '"]'); return el.hidden ? '' : el.textContent + '|' + el.getAttribute('aria-label'); };
		const pillWant = t('dashboard.overview.pill_critical', { n: '1' }) + ', ' + t('dashboard.overview.pill_warning', { n: '2' });
		return {
			severities: rows.map(r => r.dataset.severity).join(','),
			hrefs: rows.map(r => r.getAttribute('href')).join(','),
			first: rows[0].textContent,
			pillOK: document.getElementById('ov-overall').textContent === pillWant && document.querySelector('[data-sidebar-overall]').textContent === pillWant,
			overall: document.querySelector('[data-sidebar-status]').dataset.overall,
			badges: ['overview', 'problems', 'automations'].map(badge).join(';'),
			badgesWant: ['3|' + t('dashboard.overview.badge_overview', { n: '3' }), '1|' + t('dashboard.overview.badge_problems', { n: '1' }), '1|' + t('dashboard.overview.badge_automations', { n: '1' })].join(';'),
			waiting: document.getElementById('ov-system-body').textContent.includes(t('dashboard.overview.waiting_for_sample')),
			budget: document.getElementById('ov-budget-body').textContent,
			knowledge: document.getElementById('ov-knowledge-body').textContent,
			activity: document.querySelectorAll('#ov-activity-list a.ov-activity-row').length,
			agentOff: document.getElementById('ov-agent-state-body').textContent.includes(t('dashboard.personality_disabled')),
			overflow: document.documentElement.scrollWidth > innerWidth + 1,
		};
	}`)
	if got.Get("severities").Str() != "critical,warning,warning,info" || got.Get("hrefs").Str() != "#problems,#automations,#llm,#memory" ||
		!strings.Contains(got.Get("first").Str(), "Backup job keeps failing (3×)") || !got.Get("pillOK").Bool() ||
		got.Get("overall").Str() != "critical" || got.Get("badges").Str() != got.Get("badgesWant").Str() ||
		!got.Get("waiting").Bool() || !strings.Contains(got.Get("budget").Str(), "$4.25") ||
		!strings.Contains(got.Get("knowledge").Str(), "321") || got.Get("activity").Int() != 2 ||
		!got.Get("agentOff").Bool() || got.Get("overflow").Bool() {
		t.Fatalf("overview at 1440 px: %s", got.JSON("", ""))
	}

	// First sample via the section refresh button; system_metrics then updates the tile in place.
	stateMu.Lock()
	withHealth = true
	stateMu.Unlock()
	p.MustElement(`[data-section-refresh="overview"]`).MustClick()
	wait(`() => document.querySelectorAll('#ov-system-body .ov-meter').length === 3 && document.getElementById('ov-system-body').textContent.includes('23%')`)
	events <- `{"type":"system_metrics","payload":{"cpu":{"usage_percent":97.2},"memory":{"used_percent":41},"disk":{"used_percent":55},"uptime_seconds":7300}}`
	wait(`() => document.getElementById('ov-system-body').textContent.includes('97%')`)

	// An SSE trigger refetches the summary (2 s debounce): the "+N more" link appears.
	stateMu.Lock()
	attentionMore = 3
	stateMu.Unlock()
	events <- `{"type":"audit_update","payload":{}}`
	wait(`() => !!document.querySelector('#ov-attention-list a.ov-attention-more[href="#problems"]')`)

	// A refresh fails (500) while a good summary is on screen: the stale data gives way to the error state on all
	// eight tiles, the Updated stamp is not renewed, and a retry brings the tiles back with the Updated stamp.
	p.MustEval(`() => { document.querySelector('[data-section-updated="overview"]').textContent = 'stamp-before'; }`)
	stateMu.Lock()
	summaryFails = true
	stateMu.Unlock()
	p.MustElement(`[data-section-refresh="overview"]`).MustClick()
	wait(allFailed)
	if stamp := p.MustEval(`() => document.querySelector('[data-section-updated="overview"]').textContent`).Str(); stamp != "stamp-before" {
		t.Fatalf("a failed refresh must not renew the Updated stamp, got %q", stamp)
	}
	stateMu.Lock()
	summaryFails = false
	stateMu.Unlock()
	p.MustElement(`#ov-system .dash-error-retry`).MustClick()
	wait(allRecovered)
	wait(`() => !!document.querySelector('#ov-attention-list a.ov-attention-more[href="#problems"]') && document.querySelectorAll('#ov-system-body .ov-meter').length === 3 && document.querySelector('[data-section-updated="overview"]').textContent !== 'stamp-before'`)

	// Customize: hide ov-budget, leave the mode, reload, then "Show all".
	p.MustElement(`[data-action="overview-customize"]`).MustClick()
	wait(`() => document.querySelector('[data-action="overview-customize"]').getAttribute('aria-pressed') === 'true' && !document.querySelector('[data-action="overview-show-all"]').hidden && document.querySelectorAll('[data-tile-toggle]:not([hidden])').length === 6`)
	// rod's hit test reports the knob (::after of the track) as a CSSPseudoElement that "covers" the button at its
	// centre point, so MustClick would wait forever. Click the button's left edge, which no pseudo-element reaches.
	toggle := p.MustElement(`[data-tile-toggle="ov-budget"]`)
	toggle.MustScrollIntoView()
	toggleBox := toggle.MustShape().Box()
	p.Mouse.MustMoveTo(toggleBox.X+3, toggleBox.Y+toggleBox.Height/2)
	p.Mouse.MustClick(proto.InputMouseButtonLeft)
	wait(`() => document.querySelector('[data-tile-toggle="ov-budget"]').getAttribute('aria-pressed') === 'false' && document.getElementById('ov-budget').classList.contains('ov-tile-off') && document.getElementById('ov-system').classList.contains('span-3')`)
	p.MustElement(`[data-action="overview-customize"]`).MustClick()
	wait(`() => getComputedStyle(document.getElementById('ov-budget')).display === 'none' && localStorage.getItem('aurago-dash-hidden-tiles') === '["ov-budget"]' && document.querySelectorAll('[data-tile-toggle]:not([hidden])').length === 0 && ['ov-system', 'ov-automations', 'ov-knowledge'].every(id => document.getElementById(id).classList.contains('span-4'))`)
	open(1440)
	wait(`() => getComputedStyle(document.getElementById('ov-budget')).display === 'none' && getComputedStyle(document.getElementById('ov-system')).display !== 'none'`)
	p.MustElement(`[data-action="overview-customize"]`).MustClick()
	p.MustElement(`[data-action="overview-show-all"]`).MustClick()
	p.MustElement(`[data-action="overview-customize"]`).MustClick()
	wait(`() => getComputedStyle(document.getElementById('ov-budget')).display !== 'none' && localStorage.getItem('aurago-dash-hidden-tiles') === '[]' && document.getElementById('ov-system').classList.contains('span-3')`)

	// 700 px: drawer with the status block; a section link closes it; the status block returns to the overview.
	open(700)
	wait(`() => getComputedStyle(document.getElementById('dash-menu-btn')).display !== 'none' && document.getElementById('dash-sidebar').inert`)
	p.MustElement("#dash-menu-btn").MustClick()
	wait(`() => document.getElementById('dash-sidebar').classList.contains('open') && !document.getElementById('dash-sidebar-backdrop').hidden`)
	time.Sleep(400 * time.Millisecond)
	if !p.MustEval(`() => document.querySelector('[data-sidebar-status]').getBoundingClientRect().width > 100`).Bool() {
		t.Error("the status block must be visible in the open drawer")
	}
	p.MustElement(`#dash-sidebar [data-section-link="logs"]`).MustClick()
	wait(`() => DashRouter.current() === 'logs' && !document.getElementById('dash-sidebar').classList.contains('open')`)
	p.MustElement("#dash-menu-btn").MustClick()
	wait(`() => document.getElementById('dash-sidebar').classList.contains('open')`)
	time.Sleep(400 * time.Millisecond)
	p.MustElement(`[data-sidebar-status]`).MustClick()
	wait(`() => DashRouter.current() === 'overview' && location.hash === '#overview' && !document.getElementById('dash-sidebar').classList.contains('open')`)
	if p.MustEval(`() => document.documentElement.scrollWidth > innerWidth + 1`).Bool() {
		t.Error("horizontal overflow at 700 px")
	}

	errMu.Lock()
	defer errMu.Unlock()
	if len(pageErrors) > 0 {
		t.Fatalf("dashboard overview page errors:\n%s", strings.Join(pageErrors, "\n"))
	}
}
