package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

// dockerManagerFixture answers /api/docker-manager like the server does, with two reachable hosts (Local, NAS), one that
// times out (Edge), and the stack routes of phase 2: Local has two stacks and an external project, NAS must be prepared
// first, operations stream over SSE. It records every request so the test can see what the app asked for.
type dockerManagerFixture struct {
	mu           sync.Mutex
	requests     []string
	bodies       map[string]string // last body per "METHOD path"
	readonly     bool              // the docker_readonly member of GET /hosts, as the server reads docker.readonly per request
	composeText  string            // compose file of the stack "shop" on the host
	composeRev   int
	conflictNext bool // the next PUT of shop answers 409 revision_conflict with a changed host version
	nasPrepared  bool
	created      map[string]string // stacks created through POST /stacks
	deleted      map[string]bool
	operations   map[string]dockerManagerOperation
	opSeq        int
	held         string
	release      chan struct{}
}

type dockerManagerOperation struct{ host, stack, kind string }

const dockerManagerShopCompose = "services:\n  web:\n    image: nginx:latest\n    ports:\n      - \"8080:80\"\n  db:\n    image: postgres:16\n"

func newDockerManagerFixture() *dockerManagerFixture {
	return &dockerManagerFixture{
		bodies: map[string]string{}, composeText: dockerManagerShopCompose, composeRev: 1,
		created: map[string]string{}, deleted: map[string]bool{}, operations: map[string]dockerManagerOperation{}, release: make(chan struct{}),
	}
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

func dockerManagerStack(host, name, status string, running, total int, ports []map[string]any) map[string]any {
	if ports == nil {
		ports = []map[string]any{}
	}
	return map[string]any{
		"name": name, "host_id": host, "dir": "/opt/stacks/" + name, "compose_file": "compose.yaml", "has_env": false, "status": status,
		"services": map[string]any{"total": total, "running": running}, "ports": ports, "container_ids": []string{}, "created": 1760000000,
	}
}

func (f *dockerManagerFixture) stackDetail(name string) (map[string]any, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	compose, rev, status := f.composeText, fmt.Sprintf("r%d", f.composeRev), "running"
	var services []map[string]any
	var ports []map[string]any
	switch {
	case name == "shop":
		ports = []map[string]any{{"service": "web", "ip": "0.0.0.0", "public_port": 8080, "private_port": 80, "protocol": "tcp"}}
		services = []map[string]any{
			{"name": "web", "image": "nginx:latest", "state": "running", "health": "healthy", "container_id": "localshop-web", "ports": ports},
			{"name": "db", "image": "postgres:16", "state": "exited", "exit_code": 1, "ports": []any{}},
		}
	case name == "blog":
		compose, rev, status = "services:\n  site:\n    image: nginx:1.27\n", "b1", "draft"
	case f.created[name] != "":
		compose, rev, status = f.created[name], "c1", "draft"
	default:
		return nil, false
	}
	detail := dockerManagerStack("local", name, status, 1, 2, ports)
	detail["service_list"] = services
	if services == nil {
		detail["service_list"] = []any{}
	}
	detail["compose"] = map[string]any{"name": "compose.yaml", "content": compose, "revision": rev, "exists": true}
	detail["env"] = map[string]any{"name": ".env", "content": "", "revision": "", "exists": false}
	return detail, true
}

func (f *dockerManagerFixture) joined() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.requests, "\n")
}

func (f *dockerManagerFixture) body(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[key]
}

func (f *dockerManagerFixture) newOperation(host, stack, kind string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opSeq++
	id := fmt.Sprintf("op_%016x", f.opSeq)
	f.operations[id] = dockerManagerOperation{host: host, stack: stack, kind: kind}
	return dockerManagerOperationBody(id, host, stack, kind, "queued", "")
}

func dockerManagerOperationBody(id, host, stack, kind, state, output string) map[string]any {
	return map[string]any{"id": id, "host_id": host, "stack": stack, "kind": kind, "options": map[string]any{}, "state": state, "output": output, "created_at": time.Now().UTC().Format(time.RFC3339)}
}

var (
	dockerManagerServiceName = regexp.MustCompile(`(?m)^  ([a-z0-9_-]+):\s*$`)
	dockerManagerImageLine   = regexp.MustCompile(`(?m)^\s+image:\s*(\S+)`)
)

// lint mimics POST /lint closely enough for the UI: a hint for every :latest, a syntax error for BROKEN, the services of the file.
func (f *dockerManagerFixture) lint(compose string) map[string]any {
	diagnostics := []map[string]any{}
	for index, line := range strings.Split(compose, "\n") {
		if strings.Contains(line, ":latest") {
			diagnostics = append(diagnostics, map[string]any{"line": index + 1, "severity": "hint", "code": "image_latest", "message": "latest tag", "service": "web"})
		}
		if strings.Contains(line, "BROKEN") {
			diagnostics = append(diagnostics, map[string]any{"line": index + 1, "severity": "error", "code": "yaml_syntax", "message": "did not find expected key", "params": map[string]string{"detail": "did not find expected key"}})
		}
	}
	services := []map[string]any{}
	names, images := dockerManagerServiceName.FindAllStringSubmatch(compose, -1), dockerManagerImageLine.FindAllStringSubmatch(compose, -1)
	for index, match := range names {
		service := map[string]any{"name": match[1], "line": 2, "ports": []any{}, "volumes": []any{}}
		if index < len(images) {
			service["image"] = images[index][1]
		}
		services = append(services, service)
	}
	return map[string]any{"diagnostics": diagnostics, "preview": map[string]any{"services": services, "named_volumes": []string{}, "variables": []any{}}}
}

func (f *dockerManagerFixture) stream(w http.ResponseWriter, r *http.Request, id string) {
	f.mu.Lock()
	op, known := f.operations[id]
	if op.kind == "prepare" {
		f.nasPrepared = true
	}
	if op.kind == "delete" {
		f.deleted[op.stack] = true
	}
	f.mu.Unlock()
	if !known {
		http.Error(w, "unknown operation", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	send := func(event string, data any) {
		payload, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
		flusher.Flush()
		time.Sleep(60 * time.Millisecond)
	}
	holding := op.kind == "update" && op.stack == "shop"
	if holding {
		f.mu.Lock()
		f.held = id
		f.mu.Unlock()
	}
	send("snapshot", dockerManagerOperationBody(id, op.host, op.stack, op.kind, "running", "Container shop-web  Pulling\n"))
	send("output", map[string]any{"type": "output", "seq": 1, "text": "web Pulling fs layer\n"})
	send("progress", map[string]any{"type": "progress", "seq": 2, "progress": map[string]any{"id": "web layer 1", "status": "Downloading", "current": 50, "total": 100}})
	send("state", map[string]any{"type": "state", "seq": 3, "operation": dockerManagerOperationBody(id, op.host, op.stack, op.kind, "health_wait", "")})
	if holding {
		// This operation keeps running until the test closes release, so the list of active operations can be asked meanwhile.
		select {
		case <-f.release:
		case <-r.Context().Done():
			return
		case <-time.After(25 * time.Second):
		}
		f.mu.Lock()
		f.held = ""
		f.mu.Unlock()
	}
	send("done", dockerManagerFinished(id, op))
}

// dockerManagerFinished is the operation as the stream's done event and GET /operations/{id} report it once it ended.
func dockerManagerFinished(id string, op dockerManagerOperation) map[string]any {
	done := dockerManagerOperationBody(id, op.host, op.stack, op.kind, "succeeded", "Container shop-web  Started\n")
	if op.stack == "blog" && op.kind == "up" {
		// The compose check names the line it stopped at.
		done = dockerManagerOperationBody(id, op.host, op.stack, op.kind, "failed", "validating compose.yaml: services.site.image must be a string\n")
		done["error"], done["error_line"], done["exit_code"] = "line 3: services.site.image must be a string", 3, 1
	}
	done["finished_at"] = time.Now().UTC().Format(time.RFC3339)
	return done
}

func (f *dockerManagerFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rawBody, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/api/docker-manager")
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	f.bodies[r.Method+" "+path] = string(rawBody)
	readonly, nasPrepared, composeRev, conflict := f.readonly, f.nasPrepared, f.composeRev, f.conflictNext
	f.mu.Unlock()
	if strings.HasPrefix(path, "/operations/") && strings.HasSuffix(path, "/stream") && r.Method == http.MethodGet {
		f.stream(w, r, strings.TrimSuffix(strings.TrimPrefix(path, "/operations/"), "/stream"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	write := func(status int, body map[string]any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
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
			dockerManagerContainer("local", "aurago-dm-op-1", map[string]any{"aurago_helper": "runner", "protected_owner": "docker_manager"}),
		}})
	case r.Method == http.MethodGet && path == "/hosts/h_nas/containers":
		write(http.StatusOK, map[string]any{"status": "ok", "containers": []map[string]any{
			dockerManagerContainer("h_nas", "plex", map[string]any{"compose_project": "media", "compose_service": "plex", "image": "lscr.io/linuxserver/plex:latest"}),
		}})
	case r.Method == http.MethodGet && path == "/hosts/h_edge/containers":
		write(http.StatusBadGateway, map[string]any{"status": "error", "code": "host_unreachable", "reason": "timeout", "message": "dial tcp 192.0.2.50:2375: i/o timeout"})
	case r.Method == http.MethodGet && path == "/hosts/local/stacks":
		f.mu.Lock()
		stacks := []map[string]any{}
		if !f.deleted["blog"] {
			stacks = append(stacks, dockerManagerStack("local", "blog", "draft", 0, 0, nil))
		}
		stacks = append(stacks, dockerManagerStack("local", "shop", "running", 1, 2, []map[string]any{{"service": "web", "ip": "0.0.0.0", "public_port": 8080, "private_port": 80, "protocol": "tcp"}}))
		for name := range f.created {
			stacks = append(stacks, dockerManagerStack("local", name, "draft", 0, 0, nil))
		}
		f.mu.Unlock()
		write(http.StatusOK, map[string]any{"status": "ok", "host_id": "local", "stacks_dir": "/opt/stacks", "stacks": stacks, "scanned_at": "2026-10-27T10:00:00Z", "cached": false,
			"external": []map[string]any{{"name": "legacy", "working_dir": "/srv/legacy", "status": "partial", "services": map[string]any{"total": 2, "running": 1}, "ports": []any{}, "container_ids": []string{}}},
			"ignored":  []map[string]any{{"name": "Bad Name", "reason": "name_invalid"}}})
	case r.Method == http.MethodGet && path == "/hosts/h_nas/stacks":
		if !nasPrepared {
			write(http.StatusConflict, map[string]any{"status": "error", "code": "host_not_prepared", "message": "prepare first", "helpers": map[string]any{
				"api_version": "1.47", "api_supported": true, "runner_image": "docker:29.9.2-cli", "runner_image_present": false, "io_container": "missing", "prepared": false}})
			return
		}
		write(http.StatusOK, map[string]any{"status": "ok", "host_id": "h_nas", "stacks_dir": "/srv/stacks", "stacks": []map[string]any{dockerManagerStack("h_nas", "media", "stopped", 0, 1, nil)}, "external": []any{}, "ignored": []any{}, "scanned_at": "2026-10-27T10:00:00Z"})
	case r.Method == http.MethodGet && path == "/hosts/h_edge/stacks":
		write(http.StatusBadGateway, map[string]any{"status": "error", "code": "host_unreachable", "reason": "timeout", "message": "dial tcp 192.0.2.50:2375: i/o timeout"})
	case r.Method == http.MethodPost && path == "/hosts/h_nas/prepare":
		write(http.StatusAccepted, map[string]any{"status": "ok", "operation": f.newOperation("h_nas", "", "prepare")})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/hosts/local/stacks/") && strings.Count(path, "/") == 4:
		if detail, ok := f.stackDetail(strings.TrimPrefix(path, "/hosts/local/stacks/")); ok {
			write(http.StatusOK, map[string]any{"status": "ok", "stack": detail})
			return
		}
		write(http.StatusNotFound, map[string]any{"status": "error", "code": "stack_not_found", "message": "no such stack"})
	case r.Method == http.MethodPut && path == "/hosts/local/stacks/shop/files":
		var in struct {
			Compose *struct{ Content, Revision string } `json:"compose"`
		}
		_ = json.Unmarshal(rawBody, &in)
		if conflict || in.Compose == nil || in.Compose.Revision != fmt.Sprintf("r%d", composeRev) {
			f.mu.Lock()
			f.conflictNext = false
			f.composeText, f.composeRev = "services:\n  web:\n    image: nginx:1.27\n", 9
			f.mu.Unlock()
			write(http.StatusConflict, map[string]any{"status": "error", "code": "revision_conflict", "message": "changed on the host", "file": "compose",
				"current": map[string]any{"name": "compose.yaml", "content": "services:\n  web:\n    image: nginx:1.27\n", "revision": "r9", "exists": true}})
			return
		}
		f.mu.Lock()
		f.composeText, f.composeRev = in.Compose.Content, composeRev+1
		revision := fmt.Sprintf("r%d", f.composeRev)
		f.mu.Unlock()
		write(http.StatusOK, map[string]any{"status": "ok", "revisions": map[string]any{"compose": revision}})
	case r.Method == http.MethodPost && path == "/hosts/local/lint":
		var in struct {
			Compose string `json:"compose"`
		}
		_ = json.Unmarshal(rawBody, &in)
		write(http.StatusOK, f.lint(in.Compose))
	case r.Method == http.MethodPost && path == "/hosts/local/stacks":
		var in struct {
			Name    string `json:"name"`
			Compose string `json:"compose"`
			Deploy  bool   `json:"deploy"`
		}
		_ = json.Unmarshal(rawBody, &in)
		f.mu.Lock()
		f.created[in.Name] = in.Compose
		f.mu.Unlock()
		detail, _ := f.stackDetail(in.Name)
		body := map[string]any{"status": "ok", "stack": detail}
		if in.Deploy && in.Name == "oops" {
			// The files are written but the deployment could not start: 201 with deploy_error and no operation.
			body["deploy_error"] = map[string]any{"code": "internal_error", "message": "The deployment could not be started"}
		} else if in.Deploy {
			body["operation"] = f.newOperation("local", in.Name, "up")
		}
		write(http.StatusCreated, body)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "/hosts/local/stacks/") && strings.HasSuffix(path, "/ops"):
		var in struct {
			Kind string `json:"kind"`
		}
		_ = json.Unmarshal(rawBody, &in)
		name := strings.TrimSuffix(strings.TrimPrefix(path, "/hosts/local/stacks/"), "/ops")
		write(http.StatusAccepted, map[string]any{"status": "ok", "operation": f.newOperation("local", name, in.Kind)})
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/hosts/local/stacks/"):
		write(http.StatusAccepted, map[string]any{"status": "ok", "operation": f.newOperation("local", strings.TrimPrefix(path, "/hosts/local/stacks/"), "delete")})
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/volumes"):
		write(http.StatusOK, map[string]any{"status": "ok", "volumes": []map[string]any{{"name": "blog_data", "driver": "local", "size": 2048}}})
	case r.Method == http.MethodPost && path == "/hosts/local/projects/legacy/restart":
		write(http.StatusOK, map[string]any{"status": "ok"})
	case r.Method == http.MethodPost && path == "/convert/docker-run":
		write(http.StatusOK, map[string]any{"status": "ok", "compose": "services:\n  web:\n    image: nginx:1.27\n    ports:\n      - \"8080:80\"\n", "service": "web",
			"warnings": []map[string]any{{"code": "flag_ignored", "flag": "--rm", "message": "ignored"}}})
	case r.Method == http.MethodGet && path == "/operations":
		// Like the server's list: the output of every operation is blanked, the operation itself has it.
		active := []any{}
		f.mu.Lock()
		if f.held != "" {
			held := f.operations[f.held]
			active = append(active, dockerManagerOperationBody(f.held, held.host, held.stack, held.kind, "health_wait", ""))
		}
		f.mu.Unlock()
		write(http.StatusOK, map[string]any{"status": "ok", "operations": active})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/operations/"):
		id := strings.TrimPrefix(path, "/operations/")
		f.mu.Lock()
		op, known := f.operations[id]
		running := f.held == id
		f.mu.Unlock()
		switch {
		case !known:
			write(http.StatusNotFound, map[string]any{"status": "error", "code": "operation_not_found", "message": "no such operation"})
		case running:
			write(http.StatusOK, map[string]any{"status": "ok", "operation": dockerManagerOperationBody(id, op.host, op.stack, op.kind, "health_wait", "Container shop-web  Pulling\nweb Pulling fs layer\n")})
		default:
			write(http.StatusOK, map[string]any{"status": "ok", "operation": dockerManagerFinished(id, op)})
		}
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

// dockerManagerPress sends a key chord with Ctrl held, like the shortcuts of the app.
func dockerManagerPress(page *rod.Page, key input.Key) {
	page.KeyActions().Press(input.ControlLeft).Type(key).MustDo()
}

func TestDesktopDockerManagerBrowser(t *testing.T) {
	fixture := newDockerManagerFixture()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/docker-manager/": fixture})
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)

	// Stacks section (the default): own stacks by name, the external project and the single containers, each group with its
	// rows. A host that must be prepared explains itself in a card, one that cannot be reached in the banner above the list.
	page.MustWait(`()=>document.querySelectorAll('.dm-srow-main[data-stack-key]').length===2`)
	const groups = `()=>[...document.querySelectorAll('.dm-sgroup')].map(g=>g.dataset.sgroup+':'+[...g.querySelectorAll('.dm-name')].map(n=>n.textContent).join(',')).join('|')`
	if got := page.MustEval(groups).String(); got != "stacks:blog,shop|external:legacy|singles:aurago-dm-op-1,tools" {
		t.Fatalf("stack groups = %q, want own stacks, the external project and the single containers (helper container included)", got)
	}
	if !page.MustEval(`()=>!!document.querySelector('[data-host-card="h_nas"] [data-action="prepare-host"]') && !document.querySelector('[data-host-card="h_edge"]') && document.querySelectorAll('.dm-banner').length===1 && document.querySelector('[data-action="pick-section"][data-section="stacks"]').getAttribute('aria-current')==='page'`).Bool() {
		t.Fatal("the unprepared host needs a card with Prepare host, the unreachable one only its banner, and the Stacks section must be the current one")
	}
	if got := page.MustEval(`()=>[...document.querySelectorAll('[data-action="pick-section"] .dm-count')].map(e=>e.textContent).join(',')`).String(); got != "3,5" {
		t.Fatalf("section counts = %q, want 3 stacks and projects, 5 containers", got)
	}
	if got := page.MustEval(`()=>document.querySelector('.dm-srow-main[data-row-key*="aurago-dm-op-1"] .dm-lock')?.getAttribute('title')||''`).String(); got != "Hilfscontainer des Docker Managers" {
		t.Fatalf("lock of the helper container = %q, want the helper sentence although the server also names docker_manager as its owner", got)
	}
	if !strings.Contains(page.MustEval(`()=>document.querySelector('[data-stack-list] .dm-ignored').textContent`).String(), "Bad Name") {
		t.Fatal("the folder with an invalid name must be mentioned")
	}

	// Prepare host: the operation opens the drawer, streams progress and ends; the host then lists its stacks.
	page.MustEval(`()=>document.querySelector('[data-action="prepare-host"][data-host-id="h_nas"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-ops] .dm-ops-body').hidden && !!document.querySelector('li.dm-op[data-state="succeeded"]')`)
	page.MustWait(`()=>!!document.querySelector('.dm-srow-main[data-stack-key="h_nas/media"]') && !document.querySelector('[data-host-card="h_nas"]')`)
	if got := page.MustEval(`()=>document.querySelector('li.dm-op progress.dm-progress')?.value`).Int(); got != 50 {
		t.Fatalf("download progress bar = %d, want the 50 percent the stream reported", got)
	}
	page.MustEval(`()=>document.querySelector('[data-action="ops-toggle"]').focus()`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>document.querySelector('[data-ops] .dm-ops-body').hidden`)

	// Open a stack: services with state and ports as links; the Compose tab loads CodeMirror (the bundle with YAML) and the lint hint
	// shows on the gutter and in the list below.
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/shop"]').click()`)
	page.MustWait(`()=>{const p=document.querySelector('[data-stack-pane]');return !p.hidden && p.querySelectorAll('.dm-svc').length===2}`)
	if got := page.MustEval(`()=>document.querySelector('[data-stack-pane] a.dm-portlink')?.href||''`).String(); !strings.HasPrefix(got, "http://127.0.0.1:") || !strings.Contains(got, ":8080") {
		t.Fatalf("port link = %q, want a link to the page's own host name and the published port", got)
	}
	page.MustEval(`()=>document.querySelector('[data-action="stack-tab"][data-tab="compose"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-ed-surface="compose"] .cm-editor') && document.querySelector('[data-ed-surface="compose"] .cm-content').textContent.includes('nginx:latest')`)
	page.MustWait(`()=>!!document.querySelector('[data-ed-surface="compose"] .cm-lint-marker') && !!document.querySelector('[data-ed-surface="compose"] .dm-problem[data-severity="hint"]')`)

	// Edit and save (Ctrl+S): the file goes to the host with the revision it was loaded with.
	page.MustElement(`[data-ed-surface="compose"] .cm-content`).MustClick()
	page.KeyActions().Press(input.ControlLeft).Type(input.Home).MustDo()
	page.MustInsertText("# edited\n")
	page.MustWait(`()=>!document.querySelector('[data-action="save-file"][data-file="compose"]').disabled && !!document.querySelector('[data-action="stack-tab"][data-tab="compose"] .dm-dirty')`)
	dockerManagerPress(page, 's')
	page.MustWait(`()=>document.querySelector('[data-action="save-file"][data-file="compose"]').disabled && !document.querySelector('[data-action="stack-tab"][data-tab="compose"] .dm-dirty')`)
	if body := fixture.body("PUT /hosts/local/stacks/shop/files"); !strings.Contains(body, "# edited") || !strings.Contains(body, `"revision":"r1"`) {
		t.Fatalf("saved files = %s, want the edited text with revision r1", body)
	}

	// The file changes on the host meanwhile: a diff dialog offers the host version or an overwrite with the host's revision.
	fixture.mu.Lock()
	fixture.conflictNext = true
	fixture.mu.Unlock()
	page.MustInsertText("# again\n")
	dockerManagerPress(page, 's')
	page.MustWait(`()=>!!document.querySelector('dialog[open] .dm-diff [data-diff="add"]') && !!document.querySelector('dialog[open] .dm-diff [data-diff="del"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="mine"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]') && document.querySelector('[data-action="save-file"][data-file="compose"]').disabled`)
	if body := fixture.body("PUT /hosts/local/stacks/shop/files"); !strings.Contains(body, "# again") || !strings.Contains(body, `"revision":"r9"`) {
		t.Fatalf("overwrite = %s, want the edit saved against the host's revision r9", body)
	}

	// Update (the accent button of a running stack): the operation joins the drawer through the stream and keeps running. A refresh
	// asks for the active operations; that list carries no output (the server blanks it) and must not wipe what the stream delivered.
	page.MustEval(`()=>document.querySelector('[data-stack-pane] [data-action="stack-op"][data-op="update"]').click()`)
	page.MustWait(`()=>{const op=document.querySelector('li.dm-op[data-state="health_wait"]');return !!op && !!op.querySelector('progress.dm-progress')}`)
	page.MustEval(`()=>{if(document.querySelector('[data-ops] .dm-ops-body').hidden)document.querySelector('[data-action="ops-toggle"]').click()}`)
	page.MustWait(`()=>!!document.querySelector('li.dm-op[data-state="health_wait"] pre.dm-op-output')?.textContent.includes('web Pulling fs layer')`)
	listed := strings.Count(fixture.joined(), "GET /api/docker-manager/operations?active=1")
	page.MustEval(`()=>document.querySelector('[data-refresh]').click()`)
	for i := 0; i < 100 && strings.Count(fixture.joined(), "GET /api/docker-manager/operations?active=1") == listed; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(400 * time.Millisecond)
	if !page.MustEval(`()=>{const op=document.querySelector('li.dm-op[data-state="health_wait"]');return !!op && !!op.querySelector('pre.dm-op-output')?.textContent.includes('web Pulling fs layer') && op.querySelector('progress.dm-progress')?.value===50}`).Bool() {
		t.Fatal("a refresh listed the running operation without output and wiped the output and the progress of its stream")
	}
	close(fixture.release)
	page.MustWait(`()=>[...document.querySelectorAll('li.dm-op[data-state="succeeded"]')].some(o=>o.textContent.includes('shop'))`)
	if body := fixture.body("POST /hosts/local/stacks/shop/ops"); body != `{"kind":"update"}` {
		t.Fatalf("operation request = %s, want kind update", body)
	}

	// Ctrl+K opens the palette of the app and Spotlight stays closed; outside the app the desktop still gets its shortcut.
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/shop"]').focus()`)
	dockerManagerPress(page, 'k')
	page.MustWait(`()=>!!document.querySelector('[data-palette] input')`)
	if page.MustEval(`()=>!!document.getElementById('vd-spotlight-backdrop')`).Bool() {
		t.Fatal("Spotlight opened although the palette took Ctrl+K")
	}
	page.MustElement(`[data-palette] input`).MustInput("shop")
	page.MustWait(`()=>{const items=[...document.querySelectorAll('[data-palette] .dm-palette-item')];return items.length>=3 && items.every(i=>i.textContent.toLowerCase().includes('shop'))}`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('[data-palette]') && document.activeElement?.dataset?.stackKey==='local/shop'`)
	page.MustEval(`()=>document.activeElement.blur()`)
	dockerManagerPress(page, 'k')
	page.MustWait(`()=>!!document.getElementById('vd-spotlight-backdrop')`)
	page.MustEval(`()=>auditTest.closeSpotlight()`)

	// Leaving a stack with unsaved edits asks; Cancel keeps the edit, Discard moves on.
	page.MustElement(`[data-ed-surface="compose"] .cm-content`).MustClick()
	page.MustInsertText("# unsaved\n")
	page.MustWait(`()=>!document.querySelector('[data-action="save-file"][data-file="compose"]').disabled`)
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/blog"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="no"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]') && document.querySelector('[data-stack-pane] h2').textContent==='shop' && document.querySelector('[data-ed-surface="compose"] .cm-content').textContent.includes('# unsaved')`)
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/blog"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="yes"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]') && document.querySelector('[data-stack-pane] h2')?.textContent==='blog'`)

	// A deploy whose compose check fails on a line reopens the Compose tab with that line marked and the drawer names the error.
	page.MustWait(`()=>!!document.querySelector('[data-stack-pane] [data-action="stack-op"][data-op="deploy"]')`)
	page.MustEval(`()=>document.querySelector('[data-stack-pane] [data-action="stack-op"][data-op="deploy"]').click()`)
	page.MustWait(`()=>!!document.querySelector('li.dm-op[data-state="failed"] .dm-op-error')`)
	page.MustWait(`()=>!document.querySelector('[data-panel="compose"]').hidden && !!document.querySelector('[data-ed-surface="compose"] .cm-lint-marker-error')`)

	// Unsaved edits of the open stack survive what happens to other stacks: a failed deploy of blog does not jump to its line, creating
	// a stack does not open it, and "Show line" asks before it replaces the open stack.
	const shopKept = `()=>!document.querySelector('dialog[open]') && document.querySelector('[data-stack-pane] h2')?.textContent==='shop' && !!document.querySelector('[data-ed-surface="compose"] .cm-content')?.textContent.includes('# keep me') && document.querySelector('[data-action="save-file"][data-file="compose"]')?.disabled===false`
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/shop"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-stack-pane] h2')?.textContent==='shop' && document.querySelectorAll('[data-stack-pane] .dm-svc').length===2`)
	page.MustEval(`()=>document.querySelector('[data-action="stack-tab"][data-tab="compose"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-ed-surface="compose"] .cm-editor') && document.querySelector('[data-ed-surface="compose"] .cm-content').textContent.includes('nginx')`)
	page.MustElement(`[data-ed-surface="compose"] .cm-content`).MustClick()
	page.MustInsertText("# keep me\n")
	page.MustWait(`()=>!document.querySelector('[data-action="save-file"][data-file="compose"]').disabled`)
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/shop"]').focus()`)
	dockerManagerPress(page, 'k')
	page.MustWait(`()=>!!document.querySelector('[data-palette] input')`)
	page.MustElement(`[data-palette] input`).MustInput("blog bereitstellen")
	page.MustWait(`()=>{const items=[...document.querySelectorAll('[data-palette] .dm-palette-item')];return items.length===1 && items[0].textContent.includes('blog')}`)
	if err := page.Keyboard.Type(input.Enter); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>document.querySelectorAll('li.dm-op[data-state="failed"]').length===2`)
	time.Sleep(500 * time.Millisecond)
	if !page.MustEval(shopKept).Bool() {
		t.Fatal("a failed deploy of blog took the screen from shop and its unsaved edit")
	}
	page.MustEval(`()=>document.querySelector('[data-new-stack]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-newstack]').hidden && !!document.querySelector('[data-ns-compose] .cm-editor')`)
	page.MustEval(`()=>{const i=document.querySelector('[data-newstack] input[name="name"]');i.value='kept';i.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustElement(`[data-ns-compose] .cm-content`).MustClick()
	page.MustInsertText("services:\n  web:\n    image: nginx:1.27\n")
	page.MustWait(`()=>[...document.querySelectorAll('[data-ns-preview] .dm-pv-list strong')].some(e=>e.textContent==='web')`)
	page.MustEval(`()=>document.querySelector('[data-ns="save"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="no"]').click()`)
	page.MustWait(shopKept)
	if got := page.MustEval(`()=>document.querySelector('[data-newstack]').hidden`).Bool(); !got || !strings.Contains(fixture.body("POST /hosts/local/stacks"), `"name":"kept"`) {
		t.Fatalf("creating a stack with another stack's edits open must create it and leave the open stack alone: %s", fixture.body("POST /hosts/local/stacks"))
	}
	page.MustEval(`()=>document.querySelector('[data-action="op-goto-line"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="no"]').click()`)
	page.MustWait(shopKept)
	page.MustEval(`()=>document.querySelector('[data-action="op-goto-line"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="yes"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]') && document.querySelector('[data-stack-pane] h2')?.textContent==='blog' && document.querySelector('[data-panel="compose"]')?.hidden===false && !!document.querySelector('[data-ed-surface="compose"] .cm-lint-marker-error')`)

	// Delete a stack: the dialog names the folder and the volumes, and needs the stack name typed.
	page.MustWait(`()=>!!document.querySelector('[data-stack-pane] [data-action="stack-more"]')`)
	page.MustEval(`()=>document.querySelector('[data-stack-pane] [data-action="stack-more"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.vd-context-menu [data-context-action]').length>=2`)
	page.MustEval(`()=>[...document.querySelectorAll('.vd-context-menu [data-context-action]')].pop().click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [name="confirm-name"]') && document.querySelector('dialog[open] .dm-lose-list').textContent.includes('/opt/stacks/blog') && document.querySelector('dialog[open] [data-volumes]').textContent.includes('blog_data')`)
	if !page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="yes"]').disabled`).Bool() {
		t.Fatal("deleting a stack must wait for the typed name")
	}
	page.MustElement(`dialog[open] [name="confirm-name"]`).MustInput("blog")
	page.MustWait(`()=>!document.querySelector('dialog[open] [data-dact="yes"]').disabled`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="yes"]').click()`)
	page.MustWait(`()=>!document.querySelector('.dm-srow-main[data-stack-key="local/blog"]') && document.querySelector('[data-stack-pane]').hidden`)
	if !strings.Contains(fixture.joined(), "DELETE /api/docker-manager/hosts/local/stacks/blog") || strings.Contains(fixture.joined(), "volumes=1") {
		t.Fatalf("delete request missing or asked for the volumes: %s", fixture.joined())
	}

	// New stack by paste: the name is checked, the preview follows the code, Save draft creates the stack and opens it.
	page.MustEval(`()=>document.querySelector('[data-new-stack]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-newstack]').hidden && !!document.querySelector('[data-ns-compose] .cm-editor')`)
	page.MustElement(`[data-newstack] input[name="name"]`).MustInput("Bad Name")
	page.MustWait(`()=>document.querySelector('[data-newstack] input[name="name"]').getAttribute('aria-invalid')==='true'`)
	page.MustEval(`()=>{const i=document.querySelector('[data-newstack] input[name="name"]');i.value='notes';i.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustElement(`[data-ns-compose] .cm-content`).MustClick()
	page.MustInsertText("services:\n  web:\n    image: nginx:1.27\n")
	page.MustWait(`()=>[...document.querySelectorAll('[data-ns-preview] .dm-pv-list strong')].some(e=>e.textContent==='web')`)
	page.MustEval(`()=>document.querySelector('[data-ns="save"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-newstack]').hidden && document.querySelector('[data-stack-pane] h2')?.textContent==='notes'`)
	if body := fixture.body("POST /hosts/local/stacks"); !strings.Contains(body, `"name":"notes"`) || strings.Contains(body, "deploy") || !strings.Contains(body, "nginx:1.27") {
		t.Fatalf("create request = %s, want the draft of notes without deploy", body)
	}

	// A deploy that could not start answers 201 with deploy_error and no operation: the stack opens as a draft and the toast names
	// the reason in the language of the page, not the server's English message.
	page.MustEval(`()=>document.querySelector('[data-new-stack]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-newstack]').hidden && !!document.querySelector('[data-ns-compose] .cm-editor')`)
	page.MustEval(`()=>{const i=document.querySelector('[data-newstack] input[name="name"]');i.value='oops';i.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustElement(`[data-ns-compose] .cm-content`).MustClick()
	page.MustInsertText("services:\n  web:\n    image: nginx:1.27\n")
	page.MustWait(`()=>[...document.querySelectorAll('[data-ns-preview] .dm-pv-list strong')].some(e=>e.textContent==='web')`)
	page.MustEval(`()=>document.querySelector('[data-ns="deploy"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-newstack]').hidden && document.querySelector('[data-stack-pane] h2')?.textContent==='oops' && [...document.querySelectorAll('.dm-toast[data-tone="error"]')].some(e=>e.textContent.includes('oops'))`)
	oops := page.MustEval(`()=>[...document.querySelectorAll('.dm-toast[data-tone="error"]')].find(e=>e.textContent.includes('oops')).textContent`).String()
	if !strings.Contains(oops, "nicht gestartet werden") || !strings.Contains(oops, "interner Fehler") || strings.Contains(oops, "The deployment") {
		t.Fatalf("deploy_error toast = %q, want the saved-as-draft sentence with the translated reason", oops)
	}
	if body := fixture.body("POST /hosts/local/stacks"); !strings.Contains(body, `"name":"oops"`) || !strings.Contains(body, `"deploy":true`) {
		t.Fatalf("create request = %s, want oops with deploy", body)
	}

	// New stack by converting docker run: the compose file arrives in the editor with the warnings listed; a dropped compose file
	// fills the editor too and suggests the name; closing a filled view asks first.
	page.MustEval(`()=>document.querySelector('[data-new-stack]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-newstack]').hidden && !!document.querySelector('[data-ns-compose] .cm-editor')`)
	page.MustEval(`()=>document.querySelector('[data-ns-tab="convert"]').click()`)
	page.MustEval(`()=>{const c=document.querySelector('[data-newstack] [name="command"]');c.value='docker run -d --rm --name web -p 8080:80 nginx:1.27';c.dispatchEvent(new Event('input',{bubbles:true}));document.querySelector('[data-ns="convert"]').click()}`)
	page.MustWait(`()=>document.querySelector('[data-ns-compose] .cm-content')?.textContent.includes('nginx:1.27') && document.querySelectorAll('[data-ns-notes] li').length===1 && document.querySelector('[data-newstack] input[name="name"]').value==='web'`)
	page.MustEval(`()=>{const dt=new DataTransfer();dt.items.add(new File(['services:\n  immich:\n    image: ghcr.io/immich-app/immich-server:release\n'],'compose.yaml'));const root=document.querySelector('[data-dm-app]');root.dispatchEvent(new DragEvent('dragover',{dataTransfer:dt,bubbles:true,cancelable:true}));root.dispatchEvent(new DragEvent('drop',{dataTransfer:dt,bubbles:true,cancelable:true}))}`)
	page.MustWait(`()=>document.querySelector('[data-ns-compose] .cm-content').textContent.includes('immich-server')`)
	page.MustEval(`()=>document.querySelector('[data-ns="cancel"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="yes"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="yes"]').click()`)
	page.MustWait(`()=>document.querySelector('[data-newstack]').hidden`)

	// An external compose project: start, stop and restart go through the project route.
	page.MustEval(`()=>document.querySelector('[data-action="project-action"][data-project="legacy"][data-act="restart"]').click()`)
	page.MustWait(`()=>true`)
	for i := 0; i < 50 && !strings.Contains(fixture.joined(), "POST /api/docker-manager/hosts/local/projects/legacy/restart"); i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(fixture.joined(), "POST /api/docker-manager/hosts/local/projects/legacy/restart") {
		t.Fatalf("the project restart never reached the server: %s", fixture.joined())
	}

	// Containers section.
	page.MustEval(`()=>document.querySelector('[data-action="pick-section"][data-section="containers"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===5`)

	// The menu button of a row opens the row menu and the menu stays open: the desktop closes menus on every click it sees, so the
	// click that opens one must not reach it.
	page.MustElement(`.dm-row [data-action="more"]`).MustClick()
	page.MustWait(`()=>document.querySelectorAll('.vd-context-menu [data-context-action]').length>=3`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('.vd-context-menu')`)

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
	if got := page.MustEval(`()=>document.querySelectorAll('.dm-row .dm-chip').length`).Int(); got != 5 {
		t.Fatalf("host chips = %d, want one per row", got)
	}
	if page.MustEval(`()=>[...document.querySelectorAll('.dm-row')].find(r=>r.querySelector('.dm-name').textContent==='shop-db').dataset.tone`).String() != "warn" {
		t.Fatal("an unhealthy container must be amber")
	}

	// State filter and search.
	page.MustEval(`()=>document.querySelector('[data-filter="problem"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===1`)
	page.MustEval(`()=>document.querySelector('[data-filter="all"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===5`)
	page.MustEval(`()=>{const s=document.querySelector('[data-search]');s.value='plex';s.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===1`)
	page.MustEval(`()=>{const s=document.querySelector('[data-search]');s.value='';s.dispatchEvent(new Event('input',{bubbles:true}))}`)
	page.MustWait(`()=>document.querySelectorAll('.dm-row').length===5`)

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
	// A form with input asks before any way of closing it takes the input away, inside the form: the form element stays, nothing
	// is repainted, and "Keep editing" (a button, or a second Escape that needs no click in between) leaves it as it was typed.
	const (
		asking     = `()=>{const a=document.querySelector('dialog[open] [data-discard-ask]');return !!a&&!a.hidden&&document.activeElement===a.querySelector('[data-dact="keep"]')}`
		notAsked   = `()=>{const a=document.querySelector('dialog[open] [data-discard-ask]');return !!a&&a.hidden}`
		stillTyped = `()=>{const f=document.querySelector('dialog[open] [data-host-form]');return !!f&&f.dataset.same==='1'&&f.elements.ssh_password.value==='hunter2-secret'&&f.elements.ssh_key.value==='pem-secret'}`
	)
	openHostForm := func() {
		page.MustEval(`()=>document.querySelector('[data-action="manage-hosts"]').click()`)
		page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="add"]')`)
		page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="add"]').click()`)
		page.MustWait(`()=>!!document.querySelector('dialog[open] [data-host-form]')`)
		page.MustEval(`()=>{const f=document.querySelector('[data-host-form]');f.dataset.same='1';f.elements.ssh_password.value='hunter2-secret';f.elements.ssh_key.value='pem-secret';f.elements.ssh_key.dispatchEvent(new Event('input',{bubbles:true}))}`)
	}
	openHostForm()
	page.MustElement("dialog[open] [data-host-form] input[name=name]").MustClick()
	pressEscape()
	page.MustWait(asking)
	pressEscape()
	page.MustWait(notAsked)
	if !page.MustEval(stillTyped).Bool() {
		t.Fatal("a second Escape on the discard question must keep the form and what was typed into it")
	}
	for name, ask := range map[string]string{
		"Back":     `()=>document.querySelector('dialog[open] [data-dact="back"]').click()`,
		"close":    `()=>document.querySelector('dialog[open] [data-dialog-dismiss]').click()`,
		"backdrop": `()=>document.querySelector('dialog[open]').click()`,
	} {
		page.MustEval(ask)
		page.MustWait(asking)
		page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="keep"]').click()`)
		page.MustWait(notAsked)
		if !page.MustEval(stillTyped).Bool() {
			t.Fatalf("Keep editing after %s must leave the form with what was typed into it", name)
		}
	}
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dialog-dismiss]').click()`)
	page.MustWait(asking)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="discard"]').click()`)
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)

	closers := []struct {
		name string
		run  func()
	}{
		{"Escape twice", func() {
			// No click is needed between the Escapes: the form asks, Discard leaves it, the next Escape closes the host list.
			pressEscape()
			page.MustWait(asking)
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="discard"]').click()`)
			page.MustWait(`()=>!document.querySelector('dialog[open] [data-host-form]')&&!!document.querySelector('dialog[open] .dm-hostlist')`)
			pressEscape()
		}},
		{"Back and Close", func() {
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="back"]').click()`)
			page.MustWait(asking)
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="discard"]').click()`)
			page.MustWait(`()=>!!document.querySelector('dialog[open] .dm-hostlist')`)
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="close"]').click()`)
		}},
		{"close button", func() {
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dialog-dismiss]').click()`)
			page.MustWait(asking)
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="discard"]').click()`)
		}},
		{"backdrop", func() {
			// A click beside the dialog with typed input asks before it throws the input away.
			page.MustEval(`()=>document.querySelector('dialog[open]').click()`)
			page.MustWait(asking)
			page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="discard"]').click()`)
		}},
	}
	for _, closer := range closers {
		openHostForm()
		closer.run()
		page.MustWait(`()=>!document.querySelector('dialog[open]')`)
		if !page.MustEval(secretsGone).Bool() {
			t.Fatalf("closing the host form with %s left its content or the typed secrets in the page", closer.name)
		}
	}

	// A toast raised while a dialog is open goes inside it; once the dialog closes it is back in the app.
	page.MustEval(`()=>document.querySelector('[data-action="manage-hosts"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] [data-dact="test"][data-host-id="local"]')`)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="test"][data-host-id="local"]').click()`)
	page.MustWait(`()=>!!document.querySelector('dialog[open] .dm-toast')`)
	if err := page.Keyboard.Type(input.Escape); err != nil {
		t.Fatal(err)
	}
	page.MustWait(`()=>!document.querySelector('dialog[open]') && document.querySelector('[data-toasts]').parentNode===document.querySelector('[data-dm-app]')`)

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
	page.MustWait(asking)
	page.MustEval(`()=>document.querySelector('dialog[open] [data-dact="discard"]').click()`)
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
	if !page.MustEval(`()=>document.querySelector('[data-new-stack]').disabled`).Bool() {
		t.Fatal("a read-only window must not offer New stack")
	}
	fixture.setReadonly(false)
	page.MustEval(`()=>document.querySelector('[data-refresh]').click()`)
	page.MustWait(`()=>document.querySelector('[data-dm-app]').dataset.readonly==='false'&&document.querySelector('[data-readonly-chip]').hidden`)

	if errors := page.MustEval(`()=>window.fixtureErrors`).Arr(); len(errors) != 0 {
		t.Fatalf("page errors: %v", errors)
	}
}

// When the CodeMirror bundle cannot be loaded the editor stays a plain text area: it saves with Ctrl+S and lists the diagnostics below.
func TestDesktopDockerManagerEditorFallbackBrowser(t *testing.T) {
	fixture := newDockerManagerFixture()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{"/api/docker-manager/": fixture, "/js/vendor/codemirror-bundle.esm.js": http.NotFoundHandler()})
	page.MustEval(`()=>auditTest.openApp('docker-manager')`)
	page.MustWait(`()=>document.querySelectorAll('.dm-srow-main[data-stack-key]').length===2`)
	page.MustEval(`()=>document.querySelector('.dm-srow-main[data-stack-key="local/shop"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-stack-pane] .dm-svc').length===2`)
	page.MustEval(`()=>document.querySelector('[data-action="stack-tab"][data-tab="compose"]').click()`)
	page.MustWait(`()=>{const a=document.querySelector('[data-ed-surface="compose"] textarea.dm-ed-text');return !!a && !a.hidden && a.value.includes('nginx:latest') && !document.querySelector('[data-ed-surface="compose"] .cm-editor')}`)
	page.MustWait(`()=>!!document.querySelector('[data-ed-surface="compose"] .dm-problem[data-severity="hint"]')`)
	page.MustEval(`()=>document.querySelector('[data-ed-surface="compose"] .dm-problem-go').click()`)
	page.MustWait(`()=>{const a=document.querySelector('[data-ed-surface="compose"] textarea');return document.activeElement===a && a.value.slice(0,a.selectionStart).split('\n').length===3}`)
	page.MustEval(`()=>{const a=document.querySelector('[data-ed-surface="compose"] textarea');a.setSelectionRange(0,0)}`)
	page.MustInsertText("# fallback\n")
	page.MustWait(`()=>!document.querySelector('[data-action="save-file"][data-file="compose"]').disabled`)
	dockerManagerPress(page, 's')
	page.MustWait(`()=>document.querySelector('[data-action="save-file"][data-file="compose"]').disabled`)
	if body := fixture.body("PUT /hosts/local/stacks/shop/files"); !strings.Contains(body, "# fallback") || !strings.Contains(body, `"revision":"r1"`) {
		t.Fatalf("saved files = %s, want the text typed into the plain text area", body)
	}
	if errors := page.MustEval(`()=>window.fixtureErrors`).Arr(); len(errors) != 0 {
		t.Fatalf("page errors: %v", errors)
	}
}
