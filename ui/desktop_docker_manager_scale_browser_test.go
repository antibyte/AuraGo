package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// dockerManagerScaleFixture is the Docker Manager fixture with a container list of the test's own for the local host (the rest of the
// API, the stacks and the live stream are the fixture's).
type dockerManagerScaleFixture struct {
	*dockerManagerFixture
	listMu sync.Mutex
	local  []map[string]any
	lists  int // GET /hosts/local/containers answered from the test's list
}

func (f *dockerManagerScaleFixture) setLocal(list []map[string]any) {
	f.listMu.Lock()
	defer f.listMu.Unlock()
	f.local = list
}

func (f *dockerManagerScaleFixture) listCount() int {
	f.listMu.Lock()
	defer f.listMu.Unlock()
	return f.lists
}

func (f *dockerManagerScaleFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/api/docker-manager/hosts/local/containers" {
		f.listMu.Lock()
		list := f.local
		if list != nil {
			f.lists++
		}
		f.listMu.Unlock()
		if list != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "containers": list})
			return
		}
	}
	f.dockerManagerFixture.ServeHTTP(w, r)
}

// dockerManagerContainerEvent is a container event of the live stream.
func dockerManagerContainerEvent(name, project string) map[string]any {
	id := (name + strings.Repeat("0", 64))[:64]
	return map[string]any{"host": "local", "type": "container", "action": "start", "id": id, "short_id": id[:12], "name": name, "project": project, "service": "", "t": time.Now().UnixMilli()}
}

// A reload that only changes the counts must not rebuild the rail: the focused rail button is the same element afterwards and no
// repaint calls focus() (a focus() call forces a synchronous style and layout pass, 1.5 s on a page with thousands of rows).
func TestDesktopDockerManagerRailKeepsFocusBrowser(t *testing.T) {
	fixture := &dockerManagerScaleFixture{dockerManagerFixture: newDockerManagerFixture()}
	fixture.setLocal([]map[string]any{dockerManagerContainer("local", "alpha", nil), dockerManagerContainer("local", "beta", nil)})
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/docker-manager/": fixture})
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)
	dockerManagerWaitFor(t, "the stream subscription", func() bool { return strings.Contains(fixture.lastSubscription(), `"hosts":["local","h_nas","h_edge"]`) })
	const containersCount = `()=>document.querySelector('[data-focus-id="nav:containers"] .dm-count')?.textContent`
	page.MustWait(`()=>(` + containersCount + `)()==='3'`)

	page.MustEval(`()=>{
		const button=document.querySelector('[data-focus-id="nav:containers"]');
		window.__railButton=button;
		window.__railLine=document.querySelector('[data-host-id="all"] .dm-host-sub');
		window.__railLineText=window.__railLine.textContent;
		button.focus();
		window.__focusCalls=0;
		const focus=HTMLElement.prototype.focus;
		HTMLElement.prototype.focus=function(...args){window.__focusCalls++;return focus.apply(this,args)};
	}`)
	if !page.MustEval(`()=>document.activeElement===window.__railButton`).Bool() {
		t.Fatal("the rail button could not be focused")
	}

	// A third container: the same hosts, sections and health, so only the numbers of the rail change.
	fixture.setLocal([]map[string]any{dockerManagerContainer("local", "alpha", nil), dockerManagerContainer("local", "beta", nil), dockerManagerContainer("local", "gamma", nil)})
	fixture.pushStream("event", dockerManagerContainerEvent("gamma", ""))
	page.MustWait(`()=>(` + containersCount + `)()==='4'`)
	time.Sleep(400 * time.Millisecond)

	if !page.MustEval(`()=>window.__railButton.isConnected && document.activeElement===window.__railButton`).Bool() {
		t.Fatal("the focused rail button was replaced or lost its focus when only the counts changed")
	}
	if calls := page.MustEval(`()=>window.__focusCalls`).Int(); calls != 0 {
		t.Fatalf("a reload with changed counts called focus() %d times, want none", calls)
	}
	if !page.MustEval(`()=>window.__railLine.isConnected && window.__railLine.textContent!==window.__railLineText && /4/.test(window.__railLine.textContent)`).Bool() {
		t.Fatal("the count line of All hosts must be updated in place (same element, new text)")
	}
}
