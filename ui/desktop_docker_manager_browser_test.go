package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/go-rod/rod/lib/input"
)

// dockerManagerFixture answers /api/docker-manager like the server does, with two reachable hosts (Local, NAS) and one that
// times out (Edge). It records every request so the test can see what the app asked for.
type dockerManagerFixture struct {
	mu       sync.Mutex
	requests []string
}

func dockerManagerContainer(host, name string, extra map[string]any) map[string]any {
	full := (host + name + strings.Repeat("0", 64))[:64]
	container := map[string]any{
		"id": full[:12], "full_id": full, "names": []string{name}, "image": "nginx:1.27", "image_id": "sha256:aaa", "state": "running",
		"status": "Up 3 hours", "created": 1760000000, "ports": []any{}, "mounts": []any{}, "networks": []string{"bridge"},
	}
	for key, value := range extra {
		container[key] = value
	}
	return container
}

func (f *dockerManagerFixture) joined() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.requests, "\n")
}

func (f *dockerManagerFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(status int, body map[string]any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/docker-manager")
	switch {
	case r.Method == http.MethodGet && path == "/hosts":
		write(http.StatusOK, map[string]any{"status": "ok", "hosts": []map[string]any{
			{"id": "local", "name": "Local", "kind": "local", "address": "unix:///var/run/docker.sock", "port": 0, "stacks_dir": "/opt/stacks", "host_socket_path": "/var/run/docker.sock", "color": "blue", "sort": -1, "builtin": true, "secret_set": false, "plaintext": false,
				"status": map[string]any{"state": "ok", "checked_at": "2026-10-26T10:00:00Z", "engine_version": "27.3.1", "api_version": "1.47", "os": "linux", "arch": "amd64"}},
			// Never tested: the server sends the zero time, which the app must read as "not checked yet".
			{"id": "h_nas", "name": "NAS", "kind": "ssh", "address": "192.0.2.20", "port": 0, "stacks_dir": "/srv/stacks", "host_socket_path": "/var/run/docker.sock", "color": "teal", "ssh_user": "root", "sort": 1, "builtin": false, "secret_set": true, "plaintext": false,
				"status": map[string]any{"state": "unknown", "checked_at": "0001-01-01T00:00:00Z"}},
			{"id": "h_edge", "name": "Edge", "kind": "tcp", "address": "192.0.2.50", "port": 2375, "stacks_dir": "/opt/stacks", "host_socket_path": "/var/run/docker.sock", "color": "amber", "sort": 2, "builtin": false, "secret_set": false, "plaintext": true,
				"status": map[string]any{"state": "error", "code": "timeout", "message": "dial tcp 192.0.2.50:2375: i/o timeout", "checked_at": "2026-10-26T10:00:00Z"}},
		}})
	case r.Method == http.MethodGet && path == "/hosts/local/containers":
		write(http.StatusOK, map[string]any{"status": "ok", "containers": []map[string]any{
			dockerManagerContainer("local", "shop-web", map[string]any{"compose_project": "shop", "compose_service": "web", "ports": []map[string]any{{"private": 80, "public": 8080, "type": "tcp"}}}),
			dockerManagerContainer("local", "shop-db", map[string]any{"compose_project": "shop", "compose_service": "db", "image": "postgres:16", "health": "unhealthy", "status": "Up 3 hours (unhealthy)"}),
			dockerManagerContainer("local", "tools", map[string]any{"state": "exited", "status": "Exited (0) 2 days ago"}),
		}})
	case r.Method == http.MethodGet && path == "/hosts/h_nas/containers":
		write(http.StatusOK, map[string]any{"status": "ok", "containers": []map[string]any{
			dockerManagerContainer("h_nas", "plex", map[string]any{"compose_project": "media", "compose_service": "plex", "image": "lscr.io/linuxserver/plex:latest"}),
		}})
	case r.Method == http.MethodGet && path == "/hosts/h_edge/containers":
		write(http.StatusBadGateway, map[string]any{"status": "error", "code": "host_unreachable", "reason": "timeout", "message": "dial tcp 192.0.2.50:2375: i/o timeout"})
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/logs"):
		write(http.StatusOK, map[string]any{"status": "ok", "logs": "2026-10-26T10:00:01Z starting\n2026-10-26T10:00:02Z listening on :80\n"})
	default:
		write(http.StatusNotFound, map[string]any{"status": "error", "code": "not_found", "message": "no route " + r.Method + " " + path})
	}
}

func TestDesktopDockerManagerBrowser(t *testing.T) {
	fixture := &dockerManagerFixture{}
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/docker-manager/": fixture})
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===4`)

	// Rail: All hosts and three hosts; the unreachable one says so and gets a banner with the reason.
	if got := page.MustEval(`()=>document.querySelectorAll('.dm-host-btn').length`).Int(); got != 4 {
		t.Fatalf("rail entries = %d, want All hosts + 3 hosts", got)
	}
	if !page.MustEval(`()=>document.querySelectorAll('.dm-banner').length===1 && /Edge/.test(document.querySelector('.dm-banner h3').textContent)`).Bool() {
		t.Fatal("the unreachable host needs exactly one banner naming it")
	}
	if page.MustEval(`()=>document.querySelector('.dm-host-btn[data-host-id="h_nas"] .dm-dot').dataset.tone`).String() != "ok" {
		t.Fatal("a host that answered must show as reachable, even when its status was never checked")
	}

	// The list is grouped by compose project, single containers last, and every multi-host row carries a host chip.
	if got := page.MustEval(`()=>[...document.querySelectorAll('.dm-group-row .dm-group-name')].slice(0,2).map(e=>e.textContent).join(',')`).String(); got != "media,shop" {
		t.Fatalf("project groups = %q, want media,shop", got)
	}
	if got := page.MustEval(`()=>document.querySelectorAll('.dm-row .dm-chip').length`).Int(); got != 4 {
		t.Fatalf("host chips = %d, want one per row", got)
	}
	if page.MustEval(`()=>[...document.querySelectorAll('.dm-row')].find(r=>r.querySelector('.dm-name').textContent==='shop-db').dataset.tone`).String() != "warn" {
		t.Fatal("an unhealthy container must be amber")
	}

	// State filter and search.
	page.MustEval(`()=>document.querySelector('[data-filter="problem"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===1`)
	page.MustEval(`()=>document.querySelector('[data-filter="all"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===4`)
	page.MustEval(`()=>{const s=document.querySelector('[data-search]');s.value='plex';s.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===1`)
	page.MustEval(`()=>{const s=document.querySelector('[data-search]');s.value='';s.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===4`)

	// Keyboard: Enter on a row opens the drawer, the Logs tab shows the log lines, Escape closes the drawer again.
	page.MustEval(`()=>[...document.querySelectorAll('.dm-row')].find(r=>r.querySelector('.dm-name').textContent==='shop-web').focus()`)
	if err := page.Keyboard.Type(input.Enter); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('[data-drawer]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-tab="logs"]').click()`)
	page.MustWait(`()=>{const log=document.querySelector('[data-log]');return !!log && log.textContent.includes('listening on :80')}`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>document.querySelector('[data-drawer]').hidden`)
	if !strings.Contains(fixture.joined(), "/logs?tail=200") {
		t.Fatalf("the Logs tab must read the last 200 lines: %s", fixture.joined())
	}

	// Host manager: lists the three hosts, Escape closes it.
	page.MustEval(`()=>document.querySelector('[data-action="manage-hosts"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('dialog[open] .dm-hostrow').length===3`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)

	if errors := page.MustEval(`()=>window.fixtureErrors`).Arr(); len(errors) != 0 {
		t.Fatalf("page errors: %v", errors)
	}
}
