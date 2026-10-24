package ui

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type infraAppBrowserFixture struct {
	mu                sync.Mutex
	nodes             []map[string]any
	edges             []map[string]any
	candidates        []map[string]any
	consumed          map[string]bool
	credentialPut     map[string]any
	failCandidateOnce string
	actionPosts       []map[string]any
	requests          []string
	nextNode          int
	nextEdge          int
	capabilities      map[string]bool
	worldSize         int
	layoutCalls       []map[string]any          // every PUT /layout body, in order
	fail409Once       bool                      // the next PUT /layout bumps its first node's version and answers 409
	sourceHosts       map[string]map[string]any // Docker instance -> mapped device (desktop_infra_fixture_extras_test.go)
	sourceHostPuts    []map[string]any          // every PUT /source-hosts body, in order
	ignoredRows       map[string]map[string]any // ignored sources by identity
	nodeDeletes       []string                  // every DELETE /nodes/{id} as id?query
	batchCalls        []map[string]any          // every POST /import/batch body, in order
	ignoreCalls       []map[string]any          // every PUT /sources/ignore body, in order
	nodePuts          []map[string]any          // every PUT /nodes/{id} body, in order
	targets           []map[string]any          // GET /targets answer (app link candidates)
}

func newInfraAppBrowserFixture() *infraAppBrowserFixture {
	return &infraAppBrowserFixture{
		consumed:          map[string]bool{},
		failCandidateOnce: "docker-candidate",
		capabilities:      map[string]bool{"write": true, "source_edit": true, "control": true},
		targets: []map[string]any{
			{"app": "network-cameras", "target_id": "stream-front-door", "label": "Front door camera"},
			{"app": "software-store", "target_id": "net-tools", "label": "Network Tools"},
			{"app": "meshcore", "target_id": "conversation-lab", "label": "Lab mesh conversation"},
		},
		candidates: []map[string]any{
			{"id": "registry-candidate", "source": map[string]any{"kind": "registry", "instance": "local", "key": "registry-device-9", "name": "Device registry", "state": "current"}, "registry_id": "registry-device-9", "node": map[string]any{"kind": "device", "name": "Atlas gateway", "icon": "router", "profile": map[string]any{"ip_address": "192.0.2.9", "interfaces": []any{}}}},
			{"id": "docker-candidate", "source": map[string]any{"kind": "docker", "instance": "engine-a", "key": "container-full-id-2", "name": "Docker Engine", "state": "current"}, "node": map[string]any{"kind": "container", "name": "runtime-worker", "icon": "container", "profile": map[string]any{"vendor": "Docker", "model": "nginx:stable", "interfaces": []any{}}}},
		},
	}
}

func infraAppSourceID(kind, instance, key string) string {
	return "infra_" + base64.RawURLEncoding.EncodeToString([]byte(kind+"\x00"+instance+"\x00"+key))
}

func infraReviewArtifactPath(t *testing.T, name string) string {
	t.Helper()
	dir := os.Getenv("AURAGO_INFRA_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join("..", "reports", "infra-ui-review")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name)
}

func infraFixtureNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

func (f *infraAppBrowserFixture) record(path string) {
	f.requests = append(f.requests, path)
}

func (f *infraAppBrowserFixture) source(kind, instance, key, name string) map[string]any {
	return map[string]any{"kind": kind, "instance": instance, "key": key, "name": name, "state": "current", "observed": map[string]any{}}
}

func (f *infraAppBrowserFixture) snapshot() map[string]any {
	return map[string]any{"nodes": f.nodes, "edges": f.edges, "sources": f.sourceRows(), "source_hosts": f.sourceHostRows(), "capabilities": f.capabilities}
}

func (f *infraAppBrowserFixture) world(size int) {
	f.worldSize = size
	f.nodes = make([]map[string]any, 0, size)
	f.edges = nil
	group := map[string]any{"id": "fixture-group", "version": 1, "kind": "group", "name": "Lab floor", "icon": "group", "profile": map[string]any{"tags": []any{}, "interfaces": []any{}, "links": []any{}, "custom": map[string]any{}}, "placements": map[string]any{"atlas": map[string]any{"x": -110, "y": -80, "pinned": true}, "topology": map[string]any{"x": -110, "y": -80, "pinned": true}}, "sources": []any{}, "available_actions": []any{}}
	demo := []struct {
		name, kind, icon string
	}{
		{"Public status", "service", "web-service"},
		{"Atlas hypervisor", "vm", "vm"},
		{"Cold-store NAS", "device", "nas"},
		{"Core switch", "device", "managed-switch"},
		{"North access point", "device", "access-point"},
		{"UPS bank", "device", "ups"},
		{"Entry camera", "device", "camera"},
		{"Camera recorder", "device", "nvr"},
		{"Build workstation", "device", "workstation"},
		{"Admin laptop", "device", "laptop"},
		{"Office printer", "device", "printer"},
		{"Gateway firewall", "device", "firewall"},
		{"Edge router", "device", "router"},
		{"Archive shelf", "device", "disk-shelf"},
		{"Prototype printer", "device", "3d-printer"},
		{"Mesh relay", "device", "mesh-node"},
		{"Power distribution", "device", "pdu"},
		{"Fiber ONT", "device", "modem-ont"},
		{"Field tablet", "device", "tablet"},
		{"Room controller", "device", "smart-hub"},
		{"Remote access", "device", "vpn"},
		{"Metrics database", "device", "database"},
		{"Rack server", "device", "rack-server"},
		{"Lab mini PC", "device", "mini-pc"},
	}
	f.nodes = append(f.nodes, group)
	for i := 1; i < size; i++ {
		kind, icon := "device", "mini-pc"
		name := fmt.Sprintf("Lab asset %04d", i)
		if i <= len(demo) {
			name, kind, icon = demo[i-1].name, demo[i-1].kind, demo[i-1].icon
		}
		id := fmt.Sprintf("fixture-node-%04d", i)
		index := i - 1
		column, row := index%5, index/5
		if index >= 25 {
			extra := index - 25
			column, row = 5+extra%20, extra/20
		}
		x := 110 + column*220
		y := 100 + row*155
		if i == 3 {
			x = -10 // Retain a negative saved coordinate outside the group marker.
		}
		f.nodes = append(f.nodes, map[string]any{"id": id, "version": 1, "kind": kind, "name": name, "icon": icon, "parent_id": "fixture-group", "profile": map[string]any{"vendor": "AuraGo Labs", "model": "Atlas Mini", "ip_address": fmt.Sprintf("192.0.%d.%d", (i/250)+1, (i%250)+1), "interfaces": []any{}, "links": []any{}, "tags": []any{}, "custom": map[string]any{}}, "placements": map[string]any{"atlas": map[string]any{"x": x, "y": y, "pinned": false}, "topology": map[string]any{"x": x, "y": y, "pinned": false}}, "sources": []any{}, "available_actions": []any{}})
	}
	if size >= 3 {
		f.edges = append(f.edges, map[string]any{"id": "fixture-edge-runs-on", "version": 1, "from": "fixture-node-0001", "to": "fixture-node-0002", "kind": "runs_on", "label": "Application host"})
	}
	if size >= 7 {
		f.edges = append(f.edges,
			map[string]any{"id": "fixture-edge-lan", "version": 1, "from": "fixture-node-0003", "to": "fixture-node-0004", "kind": "ethernet", "from_port": "LAN 1", "to_port": "eth0", "speed": "2.5 Gbps"},
			map[string]any{"id": "fixture-edge-wifi", "version": 1, "from": "fixture-node-0004", "to": "fixture-node-0005", "kind": "wifi", "label": "Telemetry Wi-Fi"},
			map[string]any{"id": "fixture-edge-fiber", "version": 1, "from": "fixture-node-0005", "to": "fixture-node-0006", "kind": "fiber", "label": "Backbone"},
		)
	}
}

func (f *infraAppBrowserFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(r.URL.Path)

	write := func(status int, body any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	decode := func(dst any) bool { return json.NewDecoder(r.Body).Decode(dst) == nil }
	path := strings.TrimPrefix(r.URL.Path, "/api/desktop/infra")
	if f.serveExtras(path, r, write, decode) {
		return
	}
	switch {
	case r.URL.Path == "/api/desktop/integrations/credentials" && r.Method == http.MethodGet:
		write(http.StatusOK, []any{map[string]any{"id": "credential-lab-1", "name": "Lab SSH", "type": "ssh"}})
	case path == "/snapshot" && r.Method == http.MethodGet:
		write(http.StatusOK, f.snapshot())
	case path == "/targets" && r.Method == http.MethodGet:
		write(http.StatusOK, map[string]any{"targets": f.targets})
	case path == "/candidates" && r.Method == http.MethodGet:
		candidates := make([]map[string]any, 0, len(f.candidates))
		for _, candidate := range f.candidates {
			if !f.consumed[candidate["id"].(string)] && !f.candidateIgnored(candidate) {
				candidates = append(candidates, candidate)
			}
		}
		write(http.StatusOK, map[string]any{"candidates": candidates, "sources": []any{f.source("registry", "local", "registry", "Device registry"), f.source("docker", "engine-a", "engine-a", "Docker Engine")}})
	case path == "/nodes" && r.Method == http.MethodPost:
		var node map[string]any
		if !decode(&node) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.nextNode++
		node["id"], node["version"] = fmt.Sprintf("manual-%d", f.nextNode), 1
		if node["placements"] == nil {
			node["placements"] = map[string]any{}
		}
		if node["sources"] == nil {
			node["sources"] = []any{}
		}
		if node["available_actions"] == nil {
			node["available_actions"] = []any{}
		}
		f.nodes = append(f.nodes, node)
		write(http.StatusCreated, map[string]any{"node": node})
	case path == "/import" && r.Method == http.MethodPost:
		var request struct {
			CandidateID string `json:"candidate_id"`
			NodeID      string `json:"node_id"`
		}
		if !decode(&request) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		node, status, code := f.importCandidate(request.CandidateID, request.NodeID)
		if code != "" {
			write(status, map[string]string{"error": code})
			return
		}
		write(status, map[string]any{"node": node})
	case path == "/import/batch" && r.Method == http.MethodPost:
		var payload struct {
			Items []map[string]any `json:"items"`
		}
		if !decode(&payload) || len(payload.Items) == 0 || len(payload.Items) > 500 {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.batchCalls = append(f.batchCalls, map[string]any{"items": payload.Items})
		nodes, failures := []map[string]any{}, []map[string]any{}
		for _, item := range payload.Items {
			candidateID, _ := item["candidate_id"].(string)
			nodeID, _ := item["node_id"].(string)
			node, _, code := f.importCandidate(candidateID, nodeID)
			if code != "" {
				failures = append(failures, map[string]any{"candidate_id": candidateID, "error": code, "message": code})
				continue
			}
			if parent, ok := item["parent_id"].(string); ok {
				if parent == "" {
					delete(node, "parent_id")
				} else {
					node["parent_id"] = parent
				}
			}
			if placement, ok := item["placement"].(map[string]any); ok {
				placements, _ := node["placements"].(map[string]any)
				if placements == nil {
					placements = map[string]any{}
					node["placements"] = placements
				}
				view, _ := placement["view"].(string)
				placements[view] = map[string]any{"x": placement["x"], "y": placement["y"], "pinned": true}
			}
			nodes = append(nodes, node)
		}
		write(http.StatusOK, map[string]any{"nodes": nodes, "errors": failures})
	case strings.HasPrefix(path, "/nodes/") && strings.HasSuffix(path, "/source-edit") && r.Method == http.MethodGet:
		write(http.StatusOK, map[string]any{"source": "registry", "fields": map[string]any{"name": "Atlas gateway", "type": "server", "protocol": "ssh", "ip_address": "192.0.2.9", "port": 22, "username": "andigo", "credential_id": "credential-lab-1", "description": "Lab entry", "tags": []any{"lab"}}, "version": 1, "editable": true})
	case strings.HasPrefix(path, "/nodes/") && strings.HasSuffix(path, "/source-edit") && r.Method == http.MethodPut:
		var payload map[string]any
		if !decode(&payload) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.credentialPut = payload
		write(http.StatusOK, map[string]any{"source": "registry", "fields": payload["fields"], "version": 2, "editable": true, "status": "completed"})
	case strings.HasPrefix(path, "/nodes/") && strings.HasSuffix(path, "/actions") && r.Method == http.MethodPost:
		var payload map[string]any
		if !decode(&payload) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.actionPosts = append(f.actionPosts, payload)
		write(http.StatusOK, map[string]any{"status": "completed", "action": payload["action"], "request_id": payload["request_id"]})
	case path == "/edges" && r.Method == http.MethodPost:
		var edge map[string]any
		if !decode(&edge) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		if edge["from"] == edge["to"] {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		for _, existing := range f.edges {
			if existing["from"] == edge["from"] && existing["to"] == edge["to"] && existing["kind"] == edge["kind"] {
				write(http.StatusConflict, map[string]string{"error": "edge_exists"})
				return
			}
		}
		f.nextEdge++
		edge["id"], edge["version"] = fmt.Sprintf("edge-%d", f.nextEdge), 1
		f.edges = append(f.edges, edge)
		write(http.StatusCreated, map[string]any{"edge": edge})
	case strings.HasPrefix(path, "/nodes/") && r.Method == http.MethodPut:
		var updated map[string]any
		if !decode(&updated) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		id := strings.Split(strings.TrimPrefix(path, "/nodes/"), "/")[0]
		f.nodePuts = append(f.nodePuts, updated)
		for index, node := range f.nodes {
			if node["id"] == id {
				updated["version"] = float64(2)
				f.nodes[index] = updated
				write(http.StatusOK, map[string]any{"node": updated})
				return
			}
		}
		write(http.StatusNotFound, map[string]string{"error": "not_found"})
	case path == "/layout" && r.Method == http.MethodPut:
		var payload struct {
			View  string           `json:"view"`
			Nodes []map[string]any `json:"nodes"`
		}
		if !decode(&payload) {
			write(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}
		f.layoutCalls = append(f.layoutCalls, map[string]any{"view": payload.View, "nodes": payload.Nodes})
		byID := map[string]map[string]any{}
		for _, node := range f.nodes {
			byID[fmt.Sprint(node["id"])] = node
		}
		if f.fail409Once && len(payload.Nodes) > 0 {
			f.fail409Once = false
			if node := byID[fmt.Sprint(payload.Nodes[0]["id"])]; node != nil {
				node["version"] = infraFixtureNumber(node["version"]) + 5 // another client changed it
			}
			write(http.StatusConflict, map[string]string{"error": "conflict"})
			return
		}
		for _, change := range payload.Nodes { // the server validates the whole batch first
			node := byID[fmt.Sprint(change["id"])]
			if node == nil {
				write(http.StatusNotFound, map[string]string{"error": "not_found"})
				return
			}
			if infraFixtureNumber(change["version"]) != infraFixtureNumber(node["version"]) {
				write(http.StatusConflict, map[string]string{"error": "conflict"})
				return
			}
		}
		patched := make([]map[string]any, 0, len(payload.Nodes))
		for _, change := range payload.Nodes {
			node := byID[fmt.Sprint(change["id"])]
			placements, _ := node["placements"].(map[string]any)
			if placements == nil {
				placements = map[string]any{}
				node["placements"] = placements
			}
			if change["unplace"] == true { // the amendment: the asset leaves this map view
				delete(placements, payload.View)
				node["version"] = infraFixtureNumber(node["version"]) + 1
				patched = append(patched, node)
				continue
			}
			placement := map[string]any{"x": change["x"], "y": change["y"], "pinned": change["pinned"]}
			if w, ok := change["w"]; ok {
				placement["w"], placement["h"] = w, change["h"]
			} else if previous, ok := placements[payload.View].(map[string]any); ok && previous["w"] != nil {
				placement["w"], placement["h"] = previous["w"], previous["h"]
			}
			placements[payload.View] = placement
			if parent, ok := change["parent_id"]; ok {
				if value, _ := parent.(string); value == "" {
					delete(node, "parent_id")
				} else {
					node["parent_id"] = value
				}
			}
			node["version"] = infraFixtureNumber(node["version"]) + 1
			patched = append(patched, node)
		}
		// Mirror the server: only the nodes this request changed are returned.
		write(http.StatusOK, map[string]any{"nodes": patched})
	default:
		write(http.StatusNotFound, map[string]string{"error": "not_found", "path": r.URL.Path})
	}
}

// candidateIgnored tells whether the discovery's source was ignored (PUT /sources/ignore).
func (f *infraAppBrowserFixture) candidateIgnored(candidate map[string]any) bool {
	source, _ := candidate["source"].(map[string]any)
	return source != nil && f.ignoredRows[infraFixtureSourceKey(source)] != nil
}

// importCandidate mirrors POST /import for one discovery: the node, the HTTP status and an error code.
func (f *infraAppBrowserFixture) importCandidate(candidateID, nodeID string) (map[string]any, int, string) {
	if f.failCandidateOnce == candidateID {
		f.failCandidateOnce = ""
		return nil, http.StatusUnprocessableEntity, "fixture_retryable_candidate"
	}
	var candidate map[string]any
	for _, item := range f.candidates {
		if item["id"] == candidateID && !f.consumed[candidateID] && !f.candidateIgnored(item) {
			candidate = item
			break
		}
	}
	if candidate == nil {
		return nil, http.StatusNotFound, "not_found"
	}
	source := candidate["source"].(map[string]any)
	if nodeID != "" {
		var target map[string]any
		for _, item := range f.nodes {
			if item["id"] == nodeID {
				target = item
				break
			}
		}
		if target == nil {
			return nil, http.StatusNotFound, "not_found"
		}
		f.consumed[candidateID] = true
		sources, _ := target["sources"].([]any)
		target["sources"] = append(sources, source)
		if source["kind"] == "registry" {
			target["registry_id"] = candidate["registry_id"]
			first := f.source("docker", "engine-a", "protected-container", "Protected sidecar")
			second := f.source("docker", "engine-a", "ordinary-container", "Ordinary container")
			target["sources"] = append(target["sources"].([]any), first, second)
			target["available_actions"] = []any{"start"}
			target["source_actions"] = map[string]any{infraAppSourceID("docker", "engine-a", "protected-container"): []string{}, infraAppSourceID("docker", "engine-a", "ordinary-container"): []string{"start"}}
			target["source_action_reasons"] = map[string]any{infraAppSourceID("docker", "engine-a", "protected-container"): map[string]string{"start": "managed_container"}}
		}
		return target, http.StatusOK, ""
	}
	f.consumed[candidateID] = true
	node := cloneInfraBrowserNode(candidate["node"].(map[string]any))
	f.nextNode++
	node["id"], node["version"] = fmt.Sprintf("imported-%d", f.nextNode), 1
	node["sources"] = []any{candidate["source"]}
	node["placements"] = map[string]any{}
	node["available_actions"] = []any{"start", "stop", "restart"}
	node["source_actions"] = map[string]any{infraAppSourceID(fmt.Sprint(source["kind"]), fmt.Sprint(source["instance"]), fmt.Sprint(source["key"])): []string{"start", "stop", "restart"}}
	node["source_action_reasons"] = map[string]any{}
	f.nodes = append(f.nodes, node)
	return node, http.StatusCreated, ""
}

func cloneInfraBrowserNode(node map[string]any) map[string]any {
	encoded, _ := json.Marshal(node)
	var cloned map[string]any
	_ = json.Unmarshal(encoded, &cloned)
	if cloned["profile"] == nil {
		cloned["profile"] = map[string]any{"tags": []any{}, "interfaces": []any{}, "links": []any{}, "custom": map[string]any{}}
	}
	return cloned
}

func TestDesktopInfraAppBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	page := desktopAuditBrowserWithAPI(t, map[string]http.Handler{
		"/api/desktop/infra/":                   fixture,
		"/api/desktop/integrations/credentials": fixture,
	})
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	page.MustEval(`()=>{
		window.infraTimers={created:[],cleared:[]};
		const set=window.setTimeout.bind(window),clear=window.clearTimeout.bind(window);
		window.setTimeout=(fn,delay,...args)=>{const id=set(fn,delay,...args);if(Number(delay)===30000)infraTimers.created.push(id);return id;};
		window.clearTimeout=id=>{if(infraTimers.created.includes(id))infraTimers.cleared.push(id);return clear(id);};
		auditTest.openApp('infra');
	}`)
	page.MustWait(`()=>!!document.querySelector('.infra-app') && !document.querySelector('[data-action="add-empty"]').disabled`)
	if !page.MustEval(`()=>{
		const create=document.querySelector('[data-action="add-empty"]'),importExisting=document.querySelector('[data-action="import-empty"]');
		return !create.hidden&&!create.disabled&&!importExisting.hidden&&!importExisting.disabled&&document.querySelector('[data-empty] img')?.naturalWidth>0;
	}`).Bool() {
		t.Fatal("empty Infra atlas must show both enabled first-run choices and its local illustration")
	}
	page.MustEval(`()=>document.querySelector('[data-action="add-empty"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-new-node]')`)
	page.MustEval(`()=>{const form=document.querySelector('[data-new-node]');form.elements.name.value='Atlas gateway';form.elements.kind.value='device';form.elements.icon.value='router';form.requestSubmit();}`)
	page.MustWait(`()=>!!document.querySelector('[data-place="manual-1"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="import"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-candidate]').length===2`)
	if !page.MustEval(`()=>{const sheet=document.querySelector('[data-import-sheet]');return !sheet.hidden&&sheet.getAttribute('aria-modal')==='false'&&!document.querySelector('[data-dialog]').open;}`).Bool() {
		t.Fatal("the import sheet must open beside the map, not as a modal dialog")
	}
	if !page.MustEval(`()=>document.querySelector('[data-bind="registry-candidate"] option[value="manual-1"]')?.textContent==='Atlas gateway'`).Bool() {
		t.Fatal("registry candidate did not offer the existing eligible device")
	}
	page.MustEval(`()=>{
		for(const check of document.querySelectorAll('[data-candidate]')){check.checked=true;check.dispatchEvent(new Event('change',{bubbles:true}));}
		const bind=document.querySelector('[data-bind="registry-candidate"]');bind.value='manual-1';bind.dispatchEvent(new Event('change',{bubbles:true}));
		document.querySelector('[data-import]').click();
	}`)
	page.MustWait(`()=>document.querySelectorAll('[data-candidate]').length===1`)
	if !page.MustEval(`()=>document.querySelector('[data-candidate="docker-candidate"]')?.checked&&!document.querySelector('[data-candidate="registry-candidate"]')&&!document.querySelector('[data-candidate-row="docker-candidate"] [data-candidate-error]').hidden`).Bool() {
		t.Fatal("partial import did not retain only the candidate that still needs processing, with its inline error")
	}
	fixture.mu.Lock()
	batches := append([]map[string]any(nil), fixture.batchCalls...)
	fixture.mu.Unlock()
	if len(batches) != 1 || len(batches[0]["items"].([]map[string]any)) != 2 || batches[0]["items"].([]map[string]any)[0]["node_id"] != "manual-1" {
		t.Fatalf("the first import posted %v; want one batch of both candidates, the registry one bound to manual-1", batches)
	}
	page.MustEval(`()=>document.querySelector('[data-import]').click()`)
	page.MustWait(`()=>document.querySelector('[data-import-sheet]').hidden`)
	page.MustWait(`()=>!!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')`)
	page.MustWait(`()=>document.querySelectorAll('.infra-source-row').length===3`)
	if !page.MustEval(`()=>{
		const rows=[...document.querySelectorAll('.infra-source-row')];
		const protectedRow=rows.find(row=>row.textContent.includes('Protected sidecar'));
		const ordinaryRow=rows.find(row=>row.textContent.includes('Ordinary container'));
		return protectedRow&&!protectedRow.querySelector('[data-source-action]')&&!!protectedRow.querySelector('.infra-action-reason')&&ordinaryRow?.querySelector('[data-source-action]')?.dataset.sourceIdentity==='infra_ZG9ja2VyAGVuZ2luZS1hAG9yZGluYXJ5LWNvbnRhaW5lcg';
	}`).Bool() {
		t.Fatalf("source actions were not gated against their exact provider identity: %s", page.MustEval(`()=>JSON.stringify({rows:[...document.querySelectorAll('.infra-source-row')].map(row=>({text:row.innerText,identity:row.querySelector('[data-source-action]')?.dataset.sourceIdentity})),nodes:auditTest.state?.infra?.nodes})`).Str())
	}
	page.MustEval(`()=>document.querySelector('.infra-source-row:has([data-source-action]) [data-source-action]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-run-action="start"]')`)
	page.MustEval(`()=>document.querySelector('[data-run-action="start"]').click()`)
	page.MustWait(`()=>!!document.querySelector('.infra-live-status.is-error')||!document.querySelector('[data-dialog]').open`)
	page.MustEval(`()=>document.querySelector('[data-edit-source="registry"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-source-field="credential_id"] option[value="credential-lab-1"]')`)
	page.MustEval(`()=>{
		const name=document.querySelector('[data-source-field="name"]');name.value='Gateway from Infra';name.dispatchEvent(new Event('input',{bubbles:true}));
		document.querySelector('[data-action="save-source"]').click();
	}`)
	page.MustWait(`()=>!document.querySelector('.infra-source-edit')`)
	page.MustWait(`()=>!!document.querySelector('[data-action="link-app"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="link-app"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-picker] [data-app-target][data-app="software-store"]')`)
	page.MustEval(`()=>document.querySelector('[data-app-picker] [data-app-target][data-app="software-store"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-app-binding="software-store:net-tools"]')`)
	page.MustEval(`()=>{
		const iface=document.querySelector('[data-list-add="interfaces"]');iface.click();
		const name=document.querySelector('[data-list-row="interfaces"] [data-key="name"]');name.value='eth0';name.dispatchEvent(new Event('input',{bubbles:true}));
		const address=document.querySelector('[data-list-row="interfaces"] [data-key="ip_address"]');address.value='192.0.2.9';address.dispatchEvent(new Event('input',{bubbles:true}));
		document.querySelector('[data-detail-form] button[type="submit"]').click();
	}`)
	page.MustWait(`()=>document.querySelector('[data-detail-form] button[type="submit"]')?.disabled&&!document.querySelector('.infra-draft-banner')`)
	page.MustEval(`()=>document.querySelector('[data-action="add-edge"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-node-option="imported-2"]')`)
	page.MustEval(`()=>document.querySelector('[data-picker] [data-node-option="imported-2"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-edge-form]')`)
	page.MustEval(`()=>{const form=document.querySelector('[data-edge-form]');if(form.elements.from.value!=='manual-1'||form.elements.to.value!=='imported-2')throw new Error('connection ends were not prefilled');form.elements.kind.value='depends_on';form.requestSubmit();}`)
	page.MustWait(`()=>document.querySelectorAll('.infra-relation-row').length===1`)
	fixture.mu.Lock()
	manualBindings, _ := fixture.nodes[0]["profile"].(map[string]any)["app_bindings"].([]any)
	manualInterfaces, _ := fixture.nodes[0]["profile"].(map[string]any)["interfaces"].([]any)
	credentialPut := fixture.credentialPut
	fixture.mu.Unlock()
	if len(manualBindings) != 1 || len(manualInterfaces) != 1 || credentialPut == nil {
		t.Fatal("expected confirmed app binding, interface profile, and independent registry source edit")
	}
	// Cleanup: a dismissed suggestion hides and comes back through "show dismissed"; the hostless Docker
	// guest asks for its engine host, the picker offers devices, and the choice is written with the stored version.
	page.MustEval(`()=>document.querySelector('[data-cleanup-dismiss="engine-host:docker:engine-a"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')&&!document.querySelector('[data-action="cleanup-reset"]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-action="cleanup-reset"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')&&document.querySelector('[data-action="cleanup-reset"]').hidden`)
	page.MustEval(`()=>document.querySelector('[data-cleanup-action="engine-host:docker:engine-a"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-picker] [data-node-option="manual-1"]')`)
	if !page.MustEval(`()=>document.querySelector('[data-picker-input]')===document.activeElement&&document.querySelector('[data-picker-input]').getAttribute('role')==='combobox'&&document.querySelector('[data-picker] [role="listbox"] [role="option"]')!==null`).Bool() {
		t.Fatal("the asset picker must be a focused combobox with a listbox of options")
	}
	page.MustEval(`()=>document.querySelector('[data-picker] [data-node-option="manual-1"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-cleanup="engine-host:docker:engine-a"]')`)
	fixture.mu.Lock()
	hostPuts := append([]map[string]any(nil), fixture.sourceHostPuts...)
	fixture.mu.Unlock()
	if len(hostPuts) != 1 || hostPuts[0]["instance"] != "engine-a" || hostPuts[0]["node_id"] != "manual-1" || infraFixtureNumber(hostPuts[0]["version"]) != 0 {
		t.Fatalf("assigning the engine host wrote %#v", hostPuts)
	}
	page.MustEval(`()=>{
		const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';
		const settings=auditTest.state.bootstrap.settings;settings['appearance.theme']='standard';settings['appearance.fruity_mode']='dark';auditTest.applyDesktopSettings();
	}`)
	assertReadableLayout := func(label string) {
		got := page.MustEval(`()=>{
			const sizes=[['brand','.infra-brand-name',14],['view switch','.infra-view-switch button',14],['search','.infra-search input',14],['library title','.infra-library-title',13],['library row','.infra-library-row-name',14],['library meta','.infra-library-row-meta',14],['zoom','.infra-zoom-value',14],['visible count','[data-visible-count]',14]];
			const small=sizes.filter(([,selector,min])=>{const el=document.querySelector(selector);return !el||parseFloat(getComputedStyle(el).fontSize)<min;}).map(([name])=>name);
			const overflow=['.infra-commandbar','.infra-library','.infra-inspector'].filter(selector=>{const el=document.querySelector(selector);return el&&getComputedStyle(el).display!=='none'&&el.scrollWidth>el.clientWidth+1;});
			return JSON.stringify({small,overflow});
		}`).Str()
		if got != `{"small":[],"overflow":[]}` {
			t.Fatalf("%s Infra text is too small or a key region overflows: %s", label, got)
		}
	}
	page.MustWait(`()=>!!document.querySelector('[data-library-item]')&&!!document.querySelector('[data-detail-form]')`)
	assertReadableLayout("wide")
	for _, theme := range []string{"standard", "fruity"} {
		for _, mode := range []string{"light", "dark"} {
			page.MustEval(`(theme,mode)=>{const settings=auditTest.state.bootstrap.settings;settings['appearance.theme']=theme;settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;auditTest.applyDesktopSettings();}`, theme, mode)
			page.MustWait(`(theme,mode)=>document.body.dataset.theme===theme&&document.body.dataset.fruityMode===mode`, theme, mode)
			path := infraReviewArtifactPath(t, theme+"-"+mode+"-populated-wide.png")
			if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='430px';win.style.height='700px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth<500&&!!document.querySelector('[data-action="close-inspector"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]').click()`)
	page.MustWait(`()=>getComputedStyle(document.querySelector('[data-inspector]')).display==='none'&&getComputedStyle(document.querySelector('.infra-library')).display==='none'`)
	if !page.MustEval(`()=>{const app=document.querySelector('.infra-app'),canvas=document.querySelector('[data-canvas]');return canvas.getBoundingClientRect().width>=app.clientWidth*0.95;}`).Bool() {
		t.Fatal("below 760px the canvas must span the whole window")
	}
	page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').click()`)
	page.MustWait(`()=>{const place=document.querySelector('[data-place="manual-1"]');return getComputedStyle(document.querySelector('.infra-library')).display!=='none'&&!!place&&place.offsetParent!==null;}`)
	assertReadableLayout("430px")
	path := infraReviewArtifactPath(t, "430-narrow-library-sheet.png")
	if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>document.querySelector('[data-action="toggle-library"]').click()`)
	page.MustWait(`()=>getComputedStyle(document.querySelector('.infra-library')).display==='none'`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='1500px';win.style.height='970px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	for _, size := range []int{25, 250, 1000} {
		fixture.mu.Lock()
		fixture.world(size)
		fixture.mu.Unlock()
		page.MustEval(`()=>document.querySelector('[data-action="refresh"]').click()`)
		expectedVisible := map[int]int{25: 23, 250: 248, 1000: 998}[size]
		page.MustWait(fmt.Sprintf(`()=>document.querySelector('[data-visible-count]').textContent.includes('%d')`, expectedVisible))
		if size == 25 {
			page.MustWait(`()=>{const images=[...document.querySelectorAll('[data-nodes] img')];return images.length>0&&images.every(image=>image.complete&&image.naturalWidth>0);}`)
			if got := page.MustEval(`()=>JSON.stringify({cards:document.querySelectorAll('[data-nodes] [data-node]').length,guestCard:!!document.querySelector('[data-node="fixture-node-0001"]'),region:!!document.querySelector('[data-regions] [data-region="fixture-group"]'),viewBox:document.querySelector('[data-edges]').getAttribute('viewBox').slice(0,4),transform:document.querySelector('[data-scene]').style.transform.slice(0,10)})`).Str(); got != `{"cards":23,"guestCard":false,"region":true,"viewBox":"0 0 ","transform":"translate("}` {
				t.Fatalf("map did not render 23 cards, the group region and a translate+scale scene: %s", got)
			}
			if !page.MustEval(`()=>{const button=document.querySelector('[data-toggle-host-services="fixture-node-0002"]'),shelf=button&&document.getElementById(button.getAttribute('aria-controls'));return button?.getAttribute('aria-expanded')==='false'&&shelf?.hidden&&!!shelf.querySelector('[data-hosted-service="fixture-node-0001"]');}`).Bool() {
				t.Fatal("the host card did not expose a collapsed, accessible guest shelf")
			}
			// One guest that does not run reads "1 guest · stopped" (singular), and the tooltip lists the guest names.
			if !page.MustEval(`()=>{const badge=document.querySelector('[data-toggle-host-services="fixture-node-0002"]'),words=window.fixtureWords;return badge.textContent.replace('▣','').trim()===words['desktop.infra_guests_badge_stopped_one']&&badge.title===words['desktop.infra_guests_badge_title'].replace('{names}','Public status');}`).Bool() {
				t.Fatalf("the host badge must use the singular form and name its guest: %s", page.MustEval(`()=>{const badge=document.querySelector('[data-toggle-host-services="fixture-node-0002"]');return JSON.stringify([badge.textContent,badge.title]);}`).Str())
			}
			page.MustEval(`()=>document.querySelector('[data-toggle-host-services="fixture-node-0002"]').click()`)
			page.MustWait(`()=>{const button=document.querySelector('[data-toggle-host-services="fixture-node-0002"]');return button.getAttribute('aria-expanded')==='true'&&!document.getElementById(button.getAttribute('aria-controls')).hidden;}`)
			if count := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length`).Int(); count != 23 {
				t.Fatalf("expanding a host must not turn guests into cards; got %d cards", count)
			}
			page.MustEval(`()=>document.querySelector('[data-hosted-service="fixture-node-0001"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Public status'`)
			// The header names kind and place only (no internal version) and the tabs follow the tablist pattern.
			if !page.MustEval(`()=>{const head=document.querySelector('[data-inspector-head]'),version=window.fixtureWords['desktop.infra_version'].replace('{version}','1'),tabs=[...document.querySelectorAll('[role="tab"][data-inspector-tab]')];return !head.textContent.includes(version)&&!!document.querySelector('[role="tablist"]')&&tabs.length===4&&tabs.filter(tab=>tab.getAttribute('aria-selected')==='true').length===1&&tabs.every(tab=>document.getElementById(tab.getAttribute('aria-controls'))?.getAttribute('role')==='tabpanel');}`).Bool() {
				t.Fatal("the inspector header must not show the version and its tabs must be a tablist with tabpanels")
			}
			if !page.MustEval(`()=>parseFloat(getComputedStyle(document.querySelector('.infra-card-name')).fontSize)>=16&&parseFloat(getComputedStyle(document.querySelector('.infra-card-meta')).fontSize)>=14`).Bool() {
				t.Fatal("card names must stay at least 16px and their meta line at least 14px")
			}
			before := page.MustEval(`()=>{const n=document.querySelector('[data-node="fixture-node-0003"]');return n.style.left+'|'+n.style.top;}`).Str()
			page.MustEval(`()=>{const search=document.querySelector('[data-search]');search.value='Cold-store NAS';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]:not(.is-dimmed)').length===1&&!document.querySelector('[data-node="fixture-node-0003"]').classList.contains('is-dimmed')&&document.querySelectorAll('[data-nodes] [data-node]').length===23`)
			page.MustEval(`()=>{const search=document.querySelector('[data-search]');search.value='';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
			page.MustWait(`()=>!document.querySelector('[data-nodes] [data-node].is-dimmed')`)
			if after := page.MustEval(`()=>{const n=document.querySelector('[data-node="fixture-node-0003"]');return n.style.left+'|'+n.style.top;}`).Str(); after != before {
				t.Fatal("filtering moved the asset from its saved world position")
			}
			page.MustEval(`()=>{window.infraZoomOf=()=>Number(/scale\(([^)]+)\)/.exec(document.querySelector('[data-scene]').style.transform)[1]);window.infraZoomBefore=infraZoomOf();document.querySelector('[data-action="zoom-in"]').click();}`)
			page.MustWait(`()=>infraZoomOf()>infraZoomBefore`)
			page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
			page.MustWait(`()=>infraZoomOf()===1&&document.querySelector('[data-zoom]').textContent.trim()==='100%'`)
			page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-view="topology"]').getAttribute('aria-pressed')==='true'`)
			page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-view="atlas"]').getAttribute('aria-pressed')==='true'`)
			if !page.MustEval(`()=>{const left=document.querySelector('[data-node="fixture-node-0003"]').getBoundingClientRect(),right=document.querySelector('[data-node="fixture-node-0002"]').getBoundingClientRect();return left.right<=right.left;}`).Bool() {
				t.Fatal("cards overlap neighbouring assets at their saved positions")
			}
			for _, theme := range []string{"standard", "fruity"} {
				for _, mode := range []string{"light", "dark"} {
					page.MustEval(`(theme,mode)=>{const settings=auditTest.state.bootstrap.settings;settings['appearance.theme']=theme;settings['appearance.fruity_mode']=mode;document.documentElement.dataset.theme=mode;auditTest.applyDesktopSettings();}`, theme, mode)
					page.MustWait(`(theme,mode)=>document.body.dataset.theme===theme&&document.body.dataset.fruityMode===mode`, theme, mode)
					if theme == "standard" && mode == "dark" || theme == "fruity" && mode == "light" {
						path := infraReviewArtifactPath(t, theme+"-"+mode+"-wide.png")
						if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0002"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Atlas hypervisor'`)
			if !page.MustEval(`()=>{const root=document.querySelector('.infra-inspector');return root.scrollWidth<=root.clientWidth+1}`).Bool() {
				t.Fatal("selected node inspector has horizontal overflow")
			}
		} else if size == 250 {
			if count := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length`).Int(); count != 248 {
				t.Fatalf("map has %d cards; want 248 (249 assets minus one guest)", count)
			}
			if left := page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0045"]').style.left`).Str(); left != "5390px" {
				t.Fatalf("a far saved world coordinate changed: %s", left)
			}
		} else {
			if got := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length+document.querySelectorAll('[data-regions] [data-region]').length+document.querySelectorAll('[data-guest-shelf] [data-hosted-service]').length`).Int(); got != 1000 {
				t.Fatalf("1000-asset map rendered %d cards, regions and guest chips; want 1000", got)
			}
			page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-table-total]').textContent.includes('1000')`)
			if count := page.MustEval(`()=>document.querySelectorAll('[data-table-body] [data-row]').length`).Int(); count >= 100 {
				t.Fatalf("inventory table rendered %d rows at once; want a virtualized slice", count)
			}
			page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===998`)
			// Switching back from the list schedules one forced canvas pass; let it finish before the card is captured.
			page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
			page.MustEval(`()=>{window.infraKeyedCard=document.querySelector('[data-node="fixture-node-0500"]');window.infraStamp=document.querySelector('[data-live]').dataset.refreshedAt;document.querySelector('[data-action="refresh"]').click();}`)
			page.MustWait(`()=>document.querySelector('[data-live]').dataset.refreshedAt!==window.infraStamp`)
			if !page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0500"]')===window.infraKeyedCard`).Bool() {
				t.Fatal("a refresh with unchanged data rebuilt the card instead of patching it")
			}
			page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.minWidth='360px';win.style.width='430px';win.style.height='700px';}`)
			if !page.MustEval(`()=>{const app=document.querySelector('.infra-app'),bar=document.querySelector('.infra-commandbar'),toggle=document.querySelector('[data-action="toggle-library"]');return app.clientWidth<500&&getComputedStyle(document.querySelector('.infra-library')).display==='none'&&getComputedStyle(toggle).display!=='none'&&bar.scrollWidth<=bar.clientWidth+1&&innerWidth>1500;}`).Bool() {
				t.Fatalf("narrow Infra window did not switch to its compact layout: %s", page.MustEval(`()=>{const bar=document.querySelector('.infra-commandbar');return JSON.stringify({app:document.querySelector('.infra-app').clientWidth,library:getComputedStyle(document.querySelector('.infra-library')).display,bar:[bar.clientWidth,bar.scrollWidth]});}`).Str())
			}
			page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length<100`)
			if !page.MustEval(`()=>{const trigger=document.querySelector('[data-action="add-menu"]');trigger.click();const item=document.querySelector('[data-action="import"]');const ok=!!item&&item.offsetParent!==null&&!item.disabled;trigger.click();return ok&&!!document.querySelector('[data-action="close-inspector"]')?.getAttribute('aria-label');}`).Bool() {
				t.Fatal("narrow inventory must keep import reachable through the Add menu and a labelled inspector close control")
			}
			page.MustEval(`()=>{const field=document.querySelector('[data-node-field="name"]');field.value='Unsaved narrow draft';field.dispatchEvent(new Event('input',{bubbles:true}));}`)
			page.MustWait(`()=>!!document.querySelector('.infra-draft-banner')`)
			page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]').click()`)
			page.MustWait(`()=>getComputedStyle(document.querySelector('[data-inspector]')).display==='none'`)
			if count := page.MustEval(`()=>document.querySelectorAll('[data-table-body] [data-row]').length`).Int(); count == 0 || count >= 100 {
				t.Fatalf("closing the inspector did not leave the virtualized inventory usable: %d rows", count)
			}
			page.MustEval(`()=>document.querySelector('[data-row="fixture-node-0002"] [data-select]').click()`)
			page.MustWait(`()=>document.querySelector('[data-node-field="name"]')?.value==='Unsaved narrow draft'`)
			page.MustEval(`()=>document.querySelector('[data-action="close-inspector"]').click()`)
			page.MustWait(`()=>getComputedStyle(document.querySelector('[data-inspector]')).display==='none'`)
			page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-view="atlas"]').getAttribute('aria-pressed')==='true'`)
			if !page.MustEval(`()=>getComputedStyle(document.querySelector('[data-inspector]')).display==='none'`).Bool() {
				t.Fatal("empty inspector should not cover the narrow atlas")
			}
			page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length<100`)
			path := infraReviewArtifactPath(t, "1000-grouped-narrow-window.png")
			if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !page.MustEval(`()=>infraTimers.created.length>0`).Bool() {
		t.Fatal("visible Infra window did not schedule its 30 second poll")
	}
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.display='none';}`)
	page.MustWait(`()=>infraTimers.cleared.length>0`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.display='';document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{enabled:false}}));}`)
	page.MustWait(`()=>document.querySelector('[data-action="add"]').disabled`)
	page.MustEval(`()=>document.dispatchEvent(new CustomEvent('aurago:desktop-policy',{detail:{enabled:true}}))`)
	page.MustWait(`()=>!document.querySelector('[data-action="add"]').disabled`)
	page.MustEval(`()=>{
		const win=document.querySelector('.vd-window[data-app-id="infra"]');
		window.infraCloseAttempt=auditTest.closeWindow(win.dataset.windowId);
	}`)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-choice="discard"]')`)
	page.MustEval(`async()=>{
		document.querySelector('.vd-modal [data-choice="discard"]').click();
		await window.infraCloseAttempt;
	}`)
	page.MustWait(`()=>!document.querySelector('.vd-window[data-app-id="infra"]')`)
	if count := page.MustEval(`()=>fixtureErrors.length`).Int(); count != 0 {
		t.Fatalf("browser reported uncaught errors: %s", page.MustEval(`()=>JSON.stringify(fixtureErrors)`).Str())
	}
}

func TestDesktopInfraBrowserFixtureHasAllFleetSizes(t *testing.T) {
	fixture := newInfraAppBrowserFixture()
	for _, size := range []int{25, 250, 1000} {
		fixture.world(size)
		if len(fixture.nodes) != size {
			t.Fatalf("world(%d) created %d nodes", size, len(fixture.nodes))
		}
		if size > 2 && len(fixture.edges) == 0 {
			t.Fatalf("world(%d) omitted grouped service relationships", size)
		}
	}
	if infraAppSourceID("docker", "engine-a", "protected-container") != "infra_ZG9ja2VyAGVuZ2luZS1hAHByb3RlY3RlZC1jb250YWluZXI" {
		t.Fatal("fixture source identity must match the store CandidateID encoding")
	}
}
