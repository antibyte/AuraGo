package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

// infraPendingEndpoint holds one API request until the browser cancels it or
// the test releases it. This exercises the real fetch/AbortController path.
type infraPendingEndpoint struct {
	base       http.Handler
	method     string
	path       string
	started    chan struct{}
	release    chan struct{}
	finished   chan struct{}
	startOnce  sync.Once
	finishOnce sync.Once
	relOnce    sync.Once
}

func newInfraPendingEndpoint(base http.Handler, method, path string) *infraPendingEndpoint {
	return &infraPendingEndpoint{
		base:     base,
		method:   method,
		path:     path,
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		finished: make(chan struct{}),
	}
}

func (h *infraPendingEndpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == h.method && r.URL.Path == h.path {
		h.startOnce.Do(func() { close(h.started) })
		defer h.finishOnce.Do(func() { close(h.finished) })
		select {
		case <-r.Context().Done():
			return
		case <-h.release:
		}
	}
	h.base.ServeHTTP(w, r)
}

func (h *infraPendingEndpoint) releaseRequest() {
	h.relOnce.Do(func() { close(h.release) })
	select {
	case <-h.started:
		select {
		case <-h.finished:
		case <-time.After(8 * time.Second):
		}
	default:
	}
}

func waitInfraBrowserSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(8 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func closeInfraBrowserWindow(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`async()=>{
		const win=document.querySelector('.vd-window[data-app-id="infra"]');
		if(!win) throw new Error('Infra window was not open');
		await auditTest.closeWindow(win.dataset.windowId);
	}`)
	page.MustWait(`()=>!document.querySelector('.vd-window[data-app-id="infra"]')`)
}

func requireInfraBrowserClean(t *testing.T, page *rod.Page) {
	t.Helper()
	if got := page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str(); got != "[]" {
		stacks := page.MustEval(`()=>JSON.stringify(window.infraExceptionStacks||[])`).Str()
		t.Fatalf("browser reported uncaught errors: %s; stacks: %s", got, stacks)
	}
}

func infraBrowserNodeWithSources() map[string]any {
	registry := map[string]any{"kind": "registry", "instance": "local", "key": "registry-device-9", "name": "Atlas gateway", "state": "current"}
	docker := map[string]any{"kind": "docker", "instance": "engine-a", "key": "container-full-id-2", "name": "runtime-worker", "state": "current"}
	return map[string]any{
		"id": "manual-pending", "version": 1, "kind": "device", "name": "Atlas gateway", "icon": "router",
		"profile":    map[string]any{"tags": []any{}, "interfaces": []any{}, "links": []any{}, "app_bindings": []any{}, "custom": map[string]any{}},
		"placements": map[string]any{"atlas": map[string]any{"x": 250, "y": 180, "pinned": true}},
		"sources":    []any{registry, docker}, "available_actions": []any{"start"},
		"source_actions":        map[string]any{infraAppSourceID("docker", "engine-a", "container-full-id-2"): []string{"start"}},
		"source_action_reasons": map[string]any{},
	}
}

func infraBrowserPageWithFixture(t *testing.T, handler http.Handler) *rod.Page {
	t.Helper()
	return desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/infra/":                   handler,
		"/api/desktop/integrations/credentials": handler,
	})
}

func observeInfraBrowserFetchAbort(page *rod.Page) {
	page.MustEval(`()=>{
		window.infraFetchSignals=[];
		window.infraExceptionStacks=[];
		addEventListener('error',event=>window.infraExceptionStacks.push(event.error?.stack||event.message));
		addEventListener('unhandledrejection',event=>window.infraExceptionStacks.push(event.reason?.stack||String(event.reason)));
		const original=window.fetch.bind(window);
		window.fetch=(url,options={})=>{
			const path=String(url);
			if(path.includes('/api/desktop/infra/')){
				const record={path,aborted:!!options.signal?.aborted};
				window.infraFetchSignals.push(record);
				options.signal?.addEventListener('abort',()=>record.aborted=true,{once:true});
			}
			return original(url,options);
		};
	}`)
}

func openInfraBrowserApp(page *rod.Page) {
	page.MustEval(`()=>auditTest.openApp('infra')`)
}

func beginInfraBrowserCloseAttempt(t *testing.T, page *rod.Page) {
	t.Helper()
	page.MustEval(`()=>{
		const win=document.querySelector('.vd-window[data-app-id="infra"]');
		if(!win) throw new Error('Infra window was not open');
		window.infraCloseAttempt=auditTest.closeWindow(win.dataset.windowId);
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="discard"]')`)
	if !page.MustEval(`()=>{
		const modal=document.querySelector('.vd-modal');
		return !!modal?.querySelector('[data-cancel]')?.textContent.trim() &&
			!!modal?.querySelector('[data-choice="discard"]')?.textContent.trim() &&
			!!modal?.querySelector('.vd-modal-copy')?.textContent.trim();
	}`).Bool() {
		t.Fatal("closing Infra with an unsaved draft did not show a cancel/discard warning")
	}
}

func answerInfraBrowserCloseAttempt(page *rod.Page, discard bool) {
	selector := `[data-cancel]`
	if discard {
		selector = `[data-choice="discard"]`
	}
	page.MustEval(`async()=>{
		const button=document.querySelector('.vd-modal ` + selector + `');
		if(!button) throw new Error('Infra close answer button was missing');
		button.click();
		await window.infraCloseAttempt;
	}`)
}

func TestDesktopInfraUnsavedDraftCloseBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	type scenario struct {
		name        string
		handler     http.Handler
		edit        func(*testing.T, *rod.Page)
		assertDraft func(*testing.T, *rod.Page)
		assertSaved func(*testing.T, *rod.Page)
	}

	nodeFixture := newInfraAppBrowserFixture()
	nodeFixture.nodes = []map[string]any{infraBrowserNodeWithSources()}
	sourceFixture := newInfraAppBrowserFixture()
	sourceFixture.nodes = []map[string]any{infraBrowserNodeWithSources()}
	layoutFixture := newInfraAppBrowserFixture()
	layoutNode := infraBrowserNodeWithSources()
	layoutNode["placements"] = map[string]any{"atlas": map[string]any{"x": 250, "y": 180, "pinned": false}}
	layoutFixture.nodes = []map[string]any{layoutNode}
	layoutHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/layout") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"forced_layout_failure"}`))
			return
		}
		layoutFixture.ServeHTTP(w, r)
	})

	cases := []scenario{
		{
			name:    "asset draft",
			handler: nodeFixture,
			edit: func(t *testing.T, page *rod.Page) {
				page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
				page.MustWait(`()=>!!document.querySelector('[data-node-field="name"]')`)
				page.MustEval(`()=>{
					const field=document.querySelector('[data-node-field="name"]');
					window.infraSavedNodeName=field.value;
					field.value='Unsaved asset draft';field.dispatchEvent(new Event('input',{bubbles:true}));
				}`)
				page.MustWait(`()=>!!document.querySelector('.infra-draft-banner')`)
			},
			assertDraft: func(t *testing.T, page *rod.Page) {
				if got := page.MustEval(`()=>document.querySelector('[data-node-field="name"]')?.value`).Str(); got != "Unsaved asset draft" {
					t.Fatalf("cancel did not preserve the asset draft: %q", got)
				}
			},
			assertSaved: func(t *testing.T, page *rod.Page) {
				page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
				page.MustWait(`()=>!!document.querySelector('[data-node-field="name"]')`)
				if got := page.MustEval(`()=>document.querySelector('[data-node-field="name"]')?.value`).Str(); got != page.MustEval(`()=>window.infraSavedNodeName`).Str() {
					t.Fatalf("reopening did not restore the saved asset name: %q", got)
				}
			},
		},
		{
			name:    "source draft",
			handler: sourceFixture,
			edit: func(t *testing.T, page *rod.Page) {
				page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
				page.MustWait(`()=>!!document.querySelector('[data-edit-source="registry"]')`)
				page.MustEval(`()=>document.querySelector('[data-edit-source="registry"]').click()`)
				page.MustWait(`()=>!!document.querySelector('[data-source-field="name"]')`)
				page.MustEval(`()=>{
					const field=document.querySelector('[data-source-field="name"]');
					window.infraSavedSourceName=field.value;
					field.value='Unsaved source draft';field.dispatchEvent(new Event('input',{bubbles:true}));
				}`)
				page.MustWait(`()=>!!document.querySelector('.infra-source-edit .infra-draft-banner')`)
			},
			assertDraft: func(t *testing.T, page *rod.Page) {
				if got := page.MustEval(`()=>document.querySelector('[data-source-field="name"]')?.value`).Str(); got != "Unsaved source draft" {
					t.Fatalf("cancel did not preserve the source draft: %q", got)
				}
			},
			assertSaved: func(t *testing.T, page *rod.Page) {
				page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
				page.MustWait(`()=>!!document.querySelector('[data-edit-source="registry"]')`)
				page.MustEval(`()=>document.querySelector('[data-edit-source="registry"]').click()`)
				page.MustWait(`()=>!!document.querySelector('[data-source-field="name"]')`)
				if got := page.MustEval(`()=>document.querySelector('[data-source-field="name"]')?.value`).Str(); got != page.MustEval(`()=>window.infraSavedSourceName`).Str() {
					t.Fatalf("reopening did not restore the saved source name: %q", got)
				}
			},
		},
		{
			name:    "layout draft",
			handler: layoutHandler,
			edit: func(t *testing.T, page *rod.Page) {
				page.MustEval(`()=>{
					const node=document.querySelector('[data-node="manual-pending"]');
					window.infraSavedLayout={left:node.style.left,top:node.style.top};
					document.querySelector('[data-action="arrange"]').click();
				}`)
				page.MustWait(`()=>!document.querySelector('[data-action="retry-layout"]').disabled`)
			},
			assertDraft: func(t *testing.T, page *rod.Page) {
				if !page.MustEval(`()=>{
					const node=document.querySelector('[data-node="manual-pending"]');
					return !document.querySelector('[data-action="retry-layout"]').disabled &&
						(node.style.left!==window.infraSavedLayout.left || node.style.top!==window.infraSavedLayout.top);
				}`).Bool() {
					t.Fatal("cancel did not preserve the pending layout draft")
				}
			},
			assertSaved: func(t *testing.T, page *rod.Page) {
				if !page.MustEval(`()=>{
					const node=document.querySelector('[data-node="manual-pending"]');
					return document.querySelector('[data-action="retry-layout"]').disabled &&
						node.style.left===window.infraSavedLayout.left && node.style.top===window.infraSavedLayout.top;
				}`).Bool() {
					t.Fatal("reopening did not restore the saved layout and clear the draft")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := infraBrowserPageWithFixture(t, tc.handler)
			defer page.Close()
			openInfraBrowserApp(page)
			page.MustWait(`()=>!!document.querySelector('[data-node="manual-pending"]')`)
			tc.edit(t, page)

			beginInfraBrowserCloseAttempt(t, page)
			answerInfraBrowserCloseAttempt(page, false)
			if !page.MustEval(`()=>!!document.querySelector('.vd-window[data-app-id="infra"]')`).Bool() {
				t.Fatal("cancel closed the Infra window")
			}
			tc.assertDraft(t, page)

			beginInfraBrowserCloseAttempt(t, page)
			answerInfraBrowserCloseAttempt(page, true)
			page.MustWait(`()=>!document.querySelector('.vd-window[data-app-id="infra"]')`)
			openInfraBrowserApp(page)
			page.MustWait(`()=>!!document.querySelector('[data-node="manual-pending"]')`)
			tc.assertSaved(t, page)
			requireInfraBrowserClean(t, page)
		})
	}
}

func TestDesktopInfraPendingRequestsAbortOnCloseBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	t.Run("initial snapshot", func(t *testing.T) {
		fixture := newInfraAppBrowserFixture()
		pending := newInfraPendingEndpoint(fixture, http.MethodGet, "/api/desktop/infra/snapshot")
		defer pending.releaseRequest()
		page := infraBrowserPageWithFixture(t, pending)
		defer page.Close()
		observeInfraBrowserFetchAbort(page)
		openInfraBrowserApp(page)
		waitInfraBrowserSignal(t, pending.started, "initial snapshot request")

		closeInfraBrowserWindow(t, page)
		if !page.MustEval(`()=>window.infraFetchSignals.some(request=>request.path.includes('/api/desktop/infra/snapshot')&&request.aborted)`).Bool() {
			t.Error("closing Infra did not abort the initial snapshot fetch")
		}
		pending.releaseRequest()
		requireInfraBrowserClean(t, page)
	})

	t.Run("source save", func(t *testing.T) {
		fixture := newInfraAppBrowserFixture()
		fixture.nodes = []map[string]any{infraBrowserNodeWithSources()}
		pending := newInfraPendingEndpoint(fixture, http.MethodPut, "/api/desktop/infra/nodes/manual-pending/source-edit")
		defer pending.releaseRequest()
		page := infraBrowserPageWithFixture(t, pending)
		defer page.Close()
		observeInfraBrowserFetchAbort(page)
		openInfraBrowserApp(page)
		page.MustWait(`()=>!!document.querySelector('[data-node="manual-pending"]')`)
		page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
		page.MustWait(`()=>!!document.querySelector('[data-edit-source="registry"]')`)
		page.MustEval(`()=>document.querySelector('[data-edit-source="registry"]').click()`)
		page.MustWait(`()=>!!document.querySelector('[data-source-field="name"]')`)
		page.MustEval(`()=>{
			const field=document.querySelector('[data-source-field="name"]');
			field.value='Gateway updated';field.dispatchEvent(new Event('input',{bubbles:true}));
			const save=document.querySelector('[data-action="save-source"]');if(save.disabled)throw new Error('source save did not become available');save.click();
		}`)
		waitInfraBrowserSignal(t, pending.started, "source save request")

		beginInfraBrowserCloseAttempt(t, page)
		answerInfraBrowserCloseAttempt(page, true)
		page.MustWait(`()=>!document.querySelector('.vd-window[data-app-id="infra"]')`)
		if !page.MustEval(`()=>window.infraFetchSignals.some(request=>request.path.includes('/source-edit')&&request.aborted)`).Bool() {
			t.Error("closing Infra did not abort the source-save fetch")
		}
		pending.releaseRequest()
		requireInfraBrowserClean(t, page)
	})

	t.Run("source action", func(t *testing.T) {
		fixture := newInfraAppBrowserFixture()
		fixture.nodes = []map[string]any{infraBrowserNodeWithSources()}
		pending := newInfraPendingEndpoint(fixture, http.MethodPost, "/api/desktop/infra/nodes/manual-pending/actions")
		defer pending.releaseRequest()
		page := infraBrowserPageWithFixture(t, pending)
		defer page.Close()
		observeInfraBrowserFetchAbort(page)
		openInfraBrowserApp(page)
		page.MustWait(`()=>!!document.querySelector('[data-node="manual-pending"]')`)
		page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
		page.MustWait(`()=>!!document.querySelector('[data-source-action][data-source-identity]')`)
		page.MustEval(`()=>document.querySelector('[data-source-action][data-source-identity]').click()`)
		page.MustWait(`()=>!!document.querySelector('[data-run-action="start"]')`)
		page.MustEval(`()=>document.querySelector('[data-run-action="start"]').click()`)
		waitInfraBrowserSignal(t, pending.started, "source action request")

		closeInfraBrowserWindow(t, page)
		if !page.MustEval(`()=>window.infraFetchSignals.some(request=>request.path.includes('/actions')&&request.aborted)`).Bool() {
			t.Error("closing Infra did not abort the source-action fetch")
		}
		pending.releaseRequest()
		requireInfraBrowserClean(t, page)
	})
}

func TestDesktopInfraSourceConflictReviewBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.nodes = []map[string]any{infraBrowserNodeWithSources()}
	var mu sync.Mutex
	putCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/source-edit") {
			mu.Lock()
			putCount++
			mu.Unlock()
			http.Error(w, `{"error":"conflict"}`, http.StatusConflict)
			return
		}
		fixture.ServeHTTP(w, r)
	})
	page := infraBrowserPageWithFixture(t, handler)
	defer page.Close()
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="manual-pending"]')`)
	page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-edit-source="registry"]')`)
	page.MustEval(`()=>document.querySelector('[data-edit-source="registry"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-source-field="name"]')`)
	page.MustEval(`()=>{
		const field=document.querySelector('[data-source-field="name"]');
		field.value='Gateway draft';field.dispatchEvent(new Event('input',{bubbles:true}));
		document.querySelector('[data-action="save-source"]').click();
	}`)
	page.MustWait(`()=>!!document.querySelector('[data-action="review-source"]')`)
	if !page.MustEval(`()=>{
		const field=document.querySelector('[data-source-field="name"]');
		return field?.value==='Gateway draft'&&!!document.querySelector('[data-action="review-source"]');
	}`).Bool() {
		t.Fatal("source conflict did not preserve the draft and require an explicit source review")
	}
	mu.Lock()
	gotPuts := putCount
	mu.Unlock()
	if gotPuts != 1 {
		t.Fatalf("source conflict made %d writes before explicit reapply; want 1", gotPuts)
	}
	page.MustEval(`()=>document.querySelector('[data-action="review-source"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-action="reapply-source-draft"]')`)
	if !page.MustEval(`()=>{
		const form=document.querySelector('[data-source-form]');
		return form?.textContent.includes('Atlas gateway')&&form?.textContent.includes('Gateway draft')&&!!form.querySelector('[data-action="reapply-source-draft"]');
	}`).Bool() {
		t.Fatal("source review did not show current provider values beside the preserved draft")
	}
	mu.Lock()
	gotPuts = putCount
	mu.Unlock()
	if gotPuts != 1 {
		t.Fatalf("reviewing the current source unexpectedly made %d writes; want 1", gotPuts)
	}
	page.MustEval(`()=>document.querySelector('[data-action="reapply-source-draft"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-action="review-source"]') && !document.querySelector('[data-action="reapply-source-draft"]')`)
	if !page.MustEval(`()=>document.querySelector('[data-source-field="name"]')?.value==='Gateway draft' && !document.querySelector('[data-action="save-source"]').disabled`).Bool() {
		t.Fatal("explicit reapply did not keep the draft editable against the reviewed version")
	}
	mu.Lock()
	gotPuts = putCount
	mu.Unlock()
	if gotPuts != 1 {
		t.Fatalf("reapplying a draft unexpectedly made %d writes; want 1", gotPuts)
	}
	beginInfraBrowserCloseAttempt(t, page)
	answerInfraBrowserCloseAttempt(page, true)
	page.MustWait(`()=>!document.querySelector('.vd-window[data-app-id="infra"]')`)
	requireInfraBrowserClean(t, page)
}

func TestDesktopInfraUnknownActionRequiresExplicitRearmBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.nodes = []map[string]any{infraBrowserNodeWithSources()}
	var mu sync.Mutex
	var requests []map[string]any
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/actions") {
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
				return
			}
			mu.Lock()
			requests = append(requests, body)
			status := "unknown"
			if len(requests) > 1 {
				status = "completed"
			}
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "action": body["action"], "request_id": body["request_id"]})
			return
		}
		fixture.ServeHTTP(w, r)
	})
	page := infraBrowserPageWithFixture(t, handler)
	defer page.Close()
	observeInfraBrowserFetchAbort(page)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="manual-pending"]')`)
	page.MustEval(`()=>document.querySelector('[data-node="manual-pending"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-source-action][data-source-identity]')`)
	page.MustEval(`()=>document.querySelector('[data-source-action][data-source-identity]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-run-action="start"]')`)
	page.MustEval(`()=>document.querySelector('[data-run-action="start"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-action-rearm="start"]') && document.querySelector('[data-run-action="start"]')?.disabled`)
	mu.Lock()
	firstCount := len(requests)
	mu.Unlock()
	if firstCount != 1 {
		t.Fatalf("unknown action outcome triggered %d requests before re-arm; want 1", firstCount)
	}
	page.MustEval(`()=>document.querySelector('[data-action-rearm="start"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-run-action="start"]') && !document.querySelector('[data-run-action="start"]').disabled`)
	mu.Lock()
	countAfterRearm := len(requests)
	mu.Unlock()
	if countAfterRearm != 1 {
		t.Fatalf("explicit re-arm triggered a request before the action was chosen; got %d", countAfterRearm)
	}
	page.MustEval(`()=>document.querySelector('[data-run-action="start"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-dialog]').open`)
	mu.Lock()
	got := append([]map[string]any(nil), requests...)
	mu.Unlock()
	if len(got) != 2 || got[0]["request_id"] == "" || got[0]["request_id"] == got[1]["request_id"] {
		t.Fatalf("explicitly re-armed action should produce one new request ID: %#v", got)
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
