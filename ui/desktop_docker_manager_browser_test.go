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
	readonly bool // the docker_readonly member of GET /hosts, as the server reads docker.readonly per request
}

func (f *dockerManagerFixture) setReadonly(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readonly = on
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
	readonly := f.readonly
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(status int, body map[string]any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/docker-manager")
	switch {
	case r.Method == http.MethodGet && path == "/hosts":
		write(http.StatusOK, map[string]any{"status": "ok", "docker_readonly": readonly, "hosts": []map[string]any{
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
	case r.Method == http.MethodPost && path == "/hosts/test":
		// A draft the server cannot test at all: the same 400 a rejected save answers.
		write(http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_host", "field": "stacks_dir", "message": "must not contain blanks, control characters or any of : , = quotes"})
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/hosts/local/containers/"):
		if r.URL.Query().Get("confirm") != "protected" {
			write(http.StatusConflict, map[string]any{"status": "error", "code": "container_protected_confirmation_required", "owner": "ruview", "message": "AuraGo manages this container (ruview). Repeat the request with confirm=protected to continue."})
			return
		}
		write(http.StatusOK, map[string]any{"status": "ok"})
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

	// Host form: what was typed into the SSH password or key field must be gone from the page once the dialog is closed,
	// however it is closed, and the prefs in localStorage never hold it.
	const secretsGone = `()=>{const d=document.querySelector('[data-dialog]');return !d.open&&d.querySelector('[data-dialog-content]').childElementCount===0` +
		`&&![...document.querySelectorAll('input,textarea')].some(e=>/hunter2-secret|pem-secret/.test(e.value))` +
		`&&!Object.values(localStorage).some(v=>/hunter2-secret|pem-secret/.test(v))}`
	pressEscape := func() {
		if err := page.Keyboard.Type(input.Escape); err != nil {
			t.Fatal(err)
		}
	}
	closers := []struct {
		name string
		run  func()
	}{
		{"Escape twice", func() {
			// Chrome closes a dialog on a second Escape without a user gesture in between, whatever cancel does; a real click first.
			page.MustElement("dialog[open] [data-host-form] input[name=name]").MustClick()
			pressEscape()
			page.MustWait(`()=>!document.querySelector('dialog[open] [data-host-form]')&&!!document.querySelector('dialog[open] .dm-hostlist')`)
			pressEscape()
		}},
		{"Back and Close", func() {
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="back"]').click()`)
			page.MustWait(`()=>!!document.querySelector('dialog[open] .dm-hostlist')`)
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="close"]').click()`)
		}},
		{"close button", func() { page.MustEval(`()=>document.querySelector('dialog[open] [data-dialog-dismiss]').click()`) }},
		{"backdrop", func() { page.MustEval(`()=>document.querySelector('dialog[open]').click()`) }},
	}
	for _, closer := range closers {
		page.MustEval(`()=>document.querySelector('[data-action="manage-hosts"]').click()`)
		page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="add"]')`)
		page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="add"]').click()`)
		page.MustWait(`()=>!!document.querySelector('dialog[open] [data-host-form]')`)
		page.MustEval(`()=>{const f=document.querySelector('[data-host-form]');f.elements.ssh_password.value='hunter2-secret';f.elements.ssh_key.value='pem-secret';f.elements.ssh_key.dispatchEvent(new Event('input',{bubbles:true}))}`)
		closer.run()
		page.MustWait(`()=>!document.querySelector('dialog[open]')`)
		if !page.MustEval(secretsGone).Bool() {
			t.Fatalf("closing the host form with %s left its content or the typed secrets in the page", closer.name)
		}
	}

	// Editing the stored SSH host: its credentials stay tucked away until the address changes, then the secret is asked for
	// again and the form says why (the text is localised, so only its presence is checked); putting the address back keeps
	// the stored secret.
	page.MustEval(`()=>document.querySelector('[data-action="manage-hosts"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="edit"][data-host-id="h_nas"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="edit"][data-host-id="h_nas"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-host-form]')`)
	const secretState = `()=>{const f=document.querySelector('[data-host-form]');return [!f.querySelector('[data-stored-row]').hidden,!f.querySelector('[data-for="ssh"][data-secret]').hidden,f.querySelector('[data-secret-again]').textContent.trim()!==''].join('|')}`
	const setAddress = `(value)=>{const f=document.querySelector('[data-host-form]');f.elements.address.value=value;f.elements.address.dispatchEvent(new Event('input',{bubbles:true}))}`
	if got := page.MustEval(secretState).String(); got != "true|false|false" {
		t.Fatalf("stored SSH host form = %q, want the credentials tucked away behind Replace", got)
	}
	page.MustEval(setAddress, "192.0.2.99")
	if got := page.MustEval(secretState).String(); got != "false|true|true" {
		t.Fatalf("form after the address changed = %q, want the secret input and the reason", got)
	}
	page.MustEval(setAddress, "192.0.2.20")
	if got := page.MustEval(secretState).String(); got != "true|false|false" {
		t.Fatalf("form after the address was restored = %q, want the stored credentials again", got)
	}
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="back"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] .dm-hostlist')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="close"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)

	// A draft test the server rejects for a field (400 invalid_host) is shown on that field like a rejected save: the
	// Advanced section opens, the input is marked, and no "Connection failed" result is left behind.
	page.MustEval(`()=>document.querySelector('[data-action="manage-hosts"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="add"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="add"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-host-form]')`)
	page.MustEval(`()=>{const f=document.querySelector('[data-host-form]');f.elements.name.value='Draft';f.elements.kind.value='tcp';f.elements.kind.dispatchEvent(new Event('change',{bubbles:true}));f.elements.address.value='192.0.2.77';f.elements.address.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('[data-dact="test-form"]').click()}`)
	page.MustWait(`()=>{const e=document.querySelector('[data-form-error]');return !!e&&!e.hidden}`)
	if got := page.MustEval(`()=>{const f=document.querySelector('[data-host-form]');return [f.querySelector('.dm-advanced').open,f.elements.stacks_dir.getAttribute('aria-invalid'),f.querySelector('[data-test-result]').textContent.trim()===''].join('|')}`).String(); got != "true|true|true" {
		t.Fatalf("rejected draft test = %q, want the Advanced section open, stacks_dir marked and no connection result", got)
	}
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="back"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] .dm-hostlist')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="close"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)

	// A remove the server wants confirmed (409 with an owner) shows the translated sentence for that owner in the second
	// dialog, not the server's English message with its confirm=protected instruction.
	page.MustEval(`()=>[...document.querySelectorAll('.dm-row')].find(r=>r.querySelector('.dm-name').textContent==='tools').focus()`)
	if err := page.Keyboard.Type(input.Enter); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('[data-drawer]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-drawer] [data-action="remove"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="yes"]').click()`)
	page.MustWait(`()=>{const n=document.querySelector('dialog[open] .dm-note');return !!n&&/ruview/.test(n.textContent)}`)
	if note := page.MustEval(`()=>document.querySelector('dialog[open] .dm-note').textContent`).String(); strings.Contains(note, "confirm=protected") || strings.Contains(note, "Repeat the request") {
		t.Fatalf("protected remove note = %q, want the translated owner sentence, not the server's message", note)
	}
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="no"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)
	page.MustEval(`()=>document.querySelector('[data-action="close-detail"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-drawer]').hidden`)

	// docker.readonly arrives with the host list: the actions and the chip follow it up and down, without any refusal.
	const readonlyState = `()=>document.querySelector('[data-dm-app]').dataset.readonly+'|'+document.querySelector('[data-readonly-chip]').hidden`
	if got := page.MustEval(readonlyState).String(); got != "false|true" {
		t.Fatalf("read-only state before the switch = %q, want actions shown and no chip", got)
	}
	fixture.setReadonly(true)
	page.MustEval(`()=>document.querySelector('[data-refresh]').click()`)
	page.MustWait(`()=>document.querySelector('[data-dm-app]').dataset.readonly==='true'&&!document.querySelector('[data-readonly-chip]').hidden`)
	fixture.setReadonly(false)
	page.MustEval(`()=>document.querySelector('[data-refresh]').click()`)
	page.MustWait(`()=>document.querySelector('[data-dm-app]').dataset.readonly==='false'&&document.querySelector('[data-readonly-chip]').hidden`)

	if errors := page.MustEval(`()=>window.fixtureErrors`).Arr(); len(errors) != 0 {
		t.Fatalf("page errors: %v", errors)
	}
}
