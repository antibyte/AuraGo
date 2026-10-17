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
}

func newInfraAppBrowserFixture() *infraAppBrowserFixture {
	return &infraAppBrowserFixture{
		consumed:          map[string]bool{},
		failCandidateOnce: "docker-candidate",
		capabilities:      map[string]bool{"write": true, "source_edit": true, "control": true},
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

func (f *infraAppBrowserFixture) record(path string) {
	f.requests = append(f.requests, path)
}

func (f *infraAppBrowserFixture) source(kind, instance, key, name string) map[string]any {
	return map[string]any{"kind": kind, "instance": instance, "key": key, "name": name, "state": "current", "observed": map[string]any{}}
}

func (f *infraAppBrowserFixture) snapshot() map[string]any {
	return map[string]any{"nodes": f.nodes, "edges": f.edges, "sources": []any{}, "capabilities": f.capabilities}
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
	switch {
	case r.URL.Path == "/api/desktop/integrations/credentials" && r.Method == http.MethodGet:
		write(http.StatusOK, []any{map[string]any{"id": "credential-lab-1", "name": "Lab SSH", "type": "ssh"}})
	case path == "/snapshot" && r.Method == http.MethodGet:
		write(http.StatusOK, f.snapshot())
	case path == "/targets" && r.Method == http.MethodGet:
		write(http.StatusOK, map[string]any{"targets": []any{
			map[string]any{"app": "network-cameras", "target_id": "stream-front-door", "label": "Front door camera"},
			map[string]any{"app": "software-store", "target_id": "net-tools", "label": "Network Tools"},
			map[string]any{"app": "meshcore", "target_id": "conversation-lab", "label": "Lab mesh conversation"},
		}})
	case path == "/candidates" && r.Method == http.MethodGet:
		candidates := make([]map[string]any, 0, len(f.candidates))
		for _, candidate := range f.candidates {
			if !f.consumed[candidate["id"].(string)] {
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
		if f.failCandidateOnce == request.CandidateID {
			f.failCandidateOnce = ""
			write(http.StatusUnprocessableEntity, map[string]string{"error": "fixture_retryable_candidate"})
			return
		}
		var candidate map[string]any
		for _, item := range f.candidates {
			if item["id"] == request.CandidateID {
				candidate = item
				break
			}
		}
		if candidate == nil {
			write(http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		f.consumed[request.CandidateID] = true
		if request.NodeID != "" {
			var target map[string]any
			for _, item := range f.nodes {
				if item["id"] == request.NodeID {
					target = item
					break
				}
			}
			if target == nil {
				write(http.StatusNotFound, map[string]string{"error": "not_found"})
				return
			}
			source := candidate["source"].(map[string]any)
			target["sources"] = append(target["sources"].([]any), source)
			if source["kind"] == "registry" {
				target["registry_id"] = candidate["registry_id"]
				first := f.source("docker", "engine-a", "protected-container", "Protected sidecar")
				second := f.source("docker", "engine-a", "ordinary-container", "Ordinary container")
				target["sources"] = append(target["sources"].([]any), first, second)
				target["available_actions"] = []any{"start"}
				target["source_actions"] = map[string]any{infraAppSourceID("docker", "engine-a", "protected-container"): []string{}, infraAppSourceID("docker", "engine-a", "ordinary-container"): []string{"start"}}
				target["source_action_reasons"] = map[string]any{infraAppSourceID("docker", "engine-a", "protected-container"): map[string]string{"start": "managed_container"}}
			}
			write(http.StatusOK, map[string]any{"node": target})
			return
		}
		node := candidate["node"].(map[string]any)
		f.nextNode++
		node = cloneInfraBrowserNode(node)
		node["id"], node["version"] = fmt.Sprintf("imported-%d", f.nextNode), 1
		node["sources"] = []any{candidate["source"]}
		node["placements"] = map[string]any{}
		node["available_actions"] = []any{"start", "stop", "restart"}
		source := candidate["source"].(map[string]any)
		node["source_actions"] = map[string]any{infraAppSourceID("docker", source["instance"].(string), source["key"].(string)): []string{"start", "stop", "restart"}}
		node["source_action_reasons"] = map[string]any{}
		f.nodes = append(f.nodes, node)
		write(http.StatusCreated, map[string]any{"node": node})
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
		for _, change := range payload.Nodes {
			for _, node := range f.nodes {
				if node["id"] != change["id"] {
					continue
				}
				placements, _ := node["placements"].(map[string]any)
				if placements == nil {
					placements = map[string]any{}
					node["placements"] = placements
				}
				placements[payload.View] = map[string]any{"x": change["x"], "y": change["y"], "pinned": change["pinned"]}
				node["version"] = float64(2)
			}
		}
		write(http.StatusOK, f.snapshot())
	default:
		write(http.StatusNotFound, map[string]string{"error": "not_found", "path": r.URL.Path})
	}
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
		return !create.hidden&&!create.disabled&&!importExisting.hidden&&!importExisting.disabled&&document.querySelector('.infra-stage-empty img')?.naturalWidth>0;
	}`).Bool() {
		t.Fatal("empty Infra atlas must show both enabled first-run choices and its local illustration")
	}
	page.MustEval(`()=>document.querySelector('[data-action="add-empty"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-new-node]')`)
	page.MustEval(`()=>{const form=document.querySelector('[data-new-node]');form.elements.name.value='Atlas gateway';form.elements.kind.value='device';form.elements.icon.value='router';form.requestSubmit();}`)
	page.MustWait(`()=>!!document.querySelector('[data-place="manual-1"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="import"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-candidate]').length===2`)
	if !page.MustEval(`()=>document.querySelector('[data-bind="registry-candidate"] option[value="manual-1"]')?.textContent==='Atlas gateway'`).Bool() {
		t.Fatal("registry candidate did not offer the existing eligible device")
	}
	page.MustEval(`()=>{
		for(const check of document.querySelectorAll('[data-candidate]')){check.checked=true;check.dispatchEvent(new Event('change',{bubbles:true}));}
		const bind=document.querySelector('[data-bind="registry-candidate"]');bind.value='manual-1';bind.dispatchEvent(new Event('change',{bubbles:true}));
		document.querySelector('[data-import]').click();
	}`)
	page.MustWait(`()=>document.querySelectorAll('[data-candidate]').length===1`)
	if !page.MustEval(`()=>document.querySelector('[data-candidate="docker-candidate"]')?.checked&&!document.querySelector('[data-candidate="registry-candidate"]')`).Bool() {
		t.Fatal("partial import did not retain only the candidate that still needs processing")
	}
	page.MustEval(`()=>document.querySelector('[data-import]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-place="imported-2"]')`)
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
	page.MustWait(`()=>!!document.querySelector('[data-app-target][data-app="software-store"]')`)
	page.MustEval(`()=>{
		const target=document.querySelector('[data-app-target][data-app="software-store"]');target.checked=true;target.dispatchEvent(new Event('change',{bubbles:true}));
		const iface=document.querySelector('[data-list-add="interfaces"]');iface.click();
		const name=document.querySelector('[data-list-row="interfaces"] [data-key="name"]');name.value='eth0';name.dispatchEvent(new Event('input',{bubbles:true}));
		const address=document.querySelector('[data-list-row="interfaces"] [data-key="ip_address"]');address.value='192.0.2.9';address.dispatchEvent(new Event('input',{bubbles:true}));
		document.querySelector('[data-detail-form] button[type="submit"]').click();
	}`)
	page.MustWait(`()=>document.querySelector('[data-detail-form] button[type="submit"]')?.disabled&&!document.querySelector('.infra-draft-banner')`)
	page.MustEval(`()=>{
		const add=document.querySelector('[data-action="add-edge"]');add.click();
		const form=document.querySelector('[data-edge-form]');form.elements.from.value='manual-1';form.elements.to.value='imported-2';form.elements.kind.value='depends_on';form.requestSubmit();
	}`)
	page.MustWait(`()=>document.querySelectorAll('.infra-relation-row').length===1`)
	fixture.mu.Lock()
	manualBindings, _ := fixture.nodes[0]["profile"].(map[string]any)["app_bindings"].([]any)
	manualInterfaces, _ := fixture.nodes[0]["profile"].(map[string]any)["interfaces"].([]any)
	credentialPut := fixture.credentialPut
	fixture.mu.Unlock()
	if len(manualBindings) != 1 || len(manualInterfaces) != 1 || credentialPut == nil {
		t.Fatal("expected confirmed app binding, interface profile, and independent registry source edit")
	}
	page.MustEval(`()=>{
		const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';
		const settings=auditTest.state.bootstrap.settings;settings['appearance.theme']='standard';settings['appearance.fruity_mode']='dark';auditTest.applyDesktopSettings();
	}`)
	assertReadableLayout := func(label string) {
		if !page.MustEval(`()=>{
			const sizes=[['overline','.infra-eyebrow',14],['kicker','.infra-kicker',14],['sidebar count','.infra-count',14],['scope count','.infra-scope-nav button > span:last-child',14],['source status','.infra-sidebar-bottom',14],['map legend','.infra-stage-footer',14],['map help','.infra-stage-footer [data-map-help]',14],['tray heading','.infra-tray-title',14],['tray name','.infra-tray-item strong',15],['tray kind','.infra-tray-item small',14]];
			if(sizes.some(([,selector,min])=>{const el=document.querySelector(selector);return !el||parseFloat(getComputedStyle(el).fontSize)<min;})) return false;
			return ['.infra-toolbar','.infra-filterbar','.infra-stage-head','.infra-stage-footer','.infra-tray-title','.infra-inspector'].every(selector=>{const el=document.querySelector(selector);return !el||getComputedStyle(el).display==='none'||el.scrollWidth<=el.clientWidth+1;});
		}`).Bool() {
			t.Fatalf("%s Infra text is too small or a key region overflows: %s", label, page.MustEval(`()=>JSON.stringify({sizes:['.infra-eyebrow','.infra-kicker','.infra-count','.infra-scope-nav button > span:last-child','.infra-sidebar-bottom','.infra-stage-footer','.infra-stage-footer [data-map-help]','.infra-tray-title','.infra-tray-item strong','.infra-tray-item small'].map(selector=>[selector,document.querySelector(selector)&&getComputedStyle(document.querySelector(selector)).fontSize]),overflow:['.infra-toolbar','.infra-filterbar','.infra-stage-head','.infra-stage-footer','.infra-tray-title','.infra-inspector'].filter(selector=>{const el=document.querySelector(selector);return el&&getComputedStyle(el).display!=='none'&&el.scrollWidth>el.clientWidth+1;})})`).Str())
		}
	}
	page.MustWait(`()=>!!document.querySelector('.infra-tray-item')&&!!document.querySelector('[data-detail-form]')`)
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
	page.MustWait(`()=>getComputedStyle(document.querySelector('[data-inspector]')).display==='none'&&!document.querySelector('[data-tray]').hidden`)
	assertReadableLayout("430px")
	path := infraReviewArtifactPath(t, "430-narrow-populated-tray.png")
	if err := os.WriteFile(path, page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.style.width='1500px';win.style.height='970px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	for _, size := range []int{25, 250, 1000} {
		fixture.mu.Lock()
		fixture.world(size)
		fixture.mu.Unlock()
		page.MustEval(`()=>document.querySelector('[data-action="refresh"]').click()`)
		expectedVisible := size
		if size == 25 {
			expectedVisible-- // The service is hidden until its explicit host expansion.
		}
		page.MustWait(fmt.Sprintf(`()=>document.querySelector('[data-visible-count]').textContent.includes('%d')`, expectedVisible))
		if size == 25 {
			page.MustWait(`()=>{const images=[...document.querySelectorAll('[data-nodes] img')];return images.length>0&&images.every(image=>image.complete&&image.naturalWidth>0);}`)
			if !page.MustEval(`()=>{
				const scene=document.querySelector('[data-scene]'),nodes=[...document.querySelectorAll('[data-nodes] [data-node]')];
				return nodes.length===24&&!document.querySelector('[data-node="fixture-node-0001"]')&&parseFloat(scene.style.width)>1000&&parseFloat(scene.style.width)<2000&&document.querySelector('[data-edges]').getAttribute('viewBox').startsWith('0 0 ');
			}`).Bool() {
				metrics := page.MustEval(`()=>{const scene=document.querySelector('[data-scene]'),nodes=[...document.querySelectorAll('[data-nodes] [data-node]')],images=[...document.querySelectorAll('[data-nodes] img')];return JSON.stringify({nodes:nodes.length,service:!!document.querySelector('[data-node="fixture-node-0001"]'),loaded:images.filter(image=>image.complete&&image.naturalWidth>0).length,imageCount:images.length,width:scene.style.width,height:scene.style.height,viewBox:document.querySelector('[data-edges]').getAttribute('viewBox')});}`).Str()
				t.Fatalf("atlas did not render the compact 5×5 demo cluster with SVG bounds and packaged miniatures: %s", metrics)
			}
			page.MustEval(`()=>document.querySelector('[data-node="fixture-node-0002"]').click()`)
			page.MustWait(`()=>!!document.querySelector('[data-toggle-host-services]')`)
			if !page.MustEval(`()=>{const button=document.querySelector('[data-toggle-host-services]'),list=document.getElementById(button.getAttribute('aria-controls'));return button.getAttribute('aria-expanded')==='false'&&list?.hidden&&!!list.querySelector('[data-hosted-service="fixture-node-0001"]');}`).Bool() {
				t.Fatal("host did not expose a collapsed, accessible hosted-service list")
			}
			page.MustEval(`()=>document.querySelector('[data-toggle-host-services]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===25&&!!document.querySelector('[data-node="fixture-node-0001"]')`)
			constBefore := page.MustEval(`()=>{const n=document.querySelector('[data-node="fixture-node-0001"]');return n.style.left+'|'+n.style.top;}`).Str()
			page.MustEval(`()=>document.querySelector('[data-toggle-host-services]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===24`)
			if !page.MustEval(`()=>['.infra-node-copy strong','.infra-node-copy small'].every(selector=>parseFloat(getComputedStyle(document.querySelector(selector)).fontSize)>=16)`).Bool() {
				t.Fatal("atlas node labels must remain at least 16px")
			}
			page.MustEval(`()=>document.querySelector('[data-toggle-host-services]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===25`)
			if after := page.MustEval(`()=>{const n=document.querySelector('[data-node="fixture-node-0001"]');return n.style.left+'|'+n.style.top;}`).Str(); after != constBefore {
				t.Fatal("expanding the host moved the saved hosted-service placement")
			}
			page.MustEval(`()=>{const search=document.querySelector('[data-search]');search.value='Public status';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===1`)
			page.MustEval(`()=>{const search=document.querySelector('[data-search]');search.value='';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===25`)
			if after := page.MustEval(`()=>{const n=document.querySelector('[data-node="fixture-node-0001"]');return n.style.left+'|'+n.style.top;}`).Str(); after != constBefore {
				t.Fatal("filtering moved the asset from its saved world position")
			}
			page.MustEval(`()=>document.querySelector('[data-action="zoom-in"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-scene]').style.transform==='scale(1)'`)
			page.MustEval(`()=>document.querySelector('[data-action="zoom-reset"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-scene]').style.transform==='scale(0.9)'`)
			page.MustEval(`()=>document.querySelector('[data-view="topology"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-view="topology"]').getAttribute('aria-pressed')==='true'`)
			page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-view="atlas"]').getAttribute('aria-pressed')==='true'`)
			if !page.MustEval(`()=>{
				const left=document.querySelector('[data-node="fixture-node-0003"] .infra-node-copy').getBoundingClientRect();
				const right=document.querySelector('[data-node="fixture-node-0001"] .infra-node-copy').getBoundingClientRect();
				return left.right<=right.left;
			}`).Bool() {
				t.Fatal("larger labels overlap neighboring assets at their saved positions")
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
			if metrics := page.MustEval(`()=>{const root=document.querySelector('.infra-inspector');return JSON.stringify({client:root.clientWidth,scroll:root.scrollWidth,overflow:[...root.querySelectorAll('*')].filter(el=>el.scrollWidth>el.clientWidth+2).map(el=>({tag:el.tagName,class:el.className,client:el.clientWidth,scroll:el.scrollWidth,grid:getComputedStyle(el).gridTemplateColumns})).slice(0,8)});}`).Str(); page.MustEval(`()=>{const root=document.querySelector('.infra-inspector');return root.scrollWidth<=root.clientWidth+1}`).Bool() == false {
				t.Fatalf("selected node inspector has horizontal overflow: %s", metrics)
			}
		} else if size == 250 {
			if count := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length`).Int(); count != 250 {
				t.Fatalf("atlas has %d visible nodes; want 250", count)
			}
			if !page.MustEval(`()=>parseFloat(document.querySelector('[data-scene]').style.width)>5000`).Bool() {
				t.Fatal("large atlas did not expand to contain saved world coordinates")
			}
		} else {
			if count := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length`).Int(); count != 1000 {
				t.Fatalf("expanded grouped atlas has %d nodes; want 1000", count)
			}
			page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
			page.MustWait(`()=>document.querySelector('[data-table-total]').textContent.includes('1000')`)
			if count := page.MustEval(`()=>document.querySelectorAll('[data-table-body] [data-row]').length`).Int(); count >= 100 {
				t.Fatalf("inventory table rendered %d rows at once; want a virtualized slice", count)
			}
			page.MustEval(`()=>document.querySelector('[data-view="atlas"]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===1000`)
			page.MustEval(`()=>document.querySelector('.infra-group-toggle').click()`)
			page.MustWait(`()=>document.querySelector('[data-visible-count]').textContent.includes('1')`)
			if count := page.MustEval(`()=>document.querySelectorAll('[data-nodes] [data-node]').length`).Int(); count != 1 {
				t.Fatalf("collapsed 1000-entry group has %d DOM nodes; want its single group marker", count)
			}
			page.MustEval(`()=>document.querySelector('.infra-group-toggle').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-nodes] [data-node]').length===1000`)
			page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.minWidth='360px';win.style.width='430px';win.style.height='700px';}`)
			if !page.MustEval(`()=>document.querySelector('.infra-app').clientWidth<500&&getComputedStyle(document.querySelector('.infra-sidebar')).display==='none'&&getComputedStyle(document.querySelector('.infra-workspace')).gridTemplateColumns.split(' ').length===1&&document.querySelector('.infra-toolbar').scrollWidth<=document.querySelector('.infra-toolbar').clientWidth+1&&innerWidth>1500`).Bool() {
				metrics := page.MustEval(`()=>{const app=document.querySelector('.infra-app'),win=document.querySelector('.vd-window[data-app-id="infra"]'),workspace=document.querySelector('.infra-workspace'),toolbar=document.querySelector('.infra-toolbar');return JSON.stringify({window:win.clientWidth,app:app.clientWidth,sidebar:getComputedStyle(document.querySelector('.infra-sidebar')).display,columns:getComputedStyle(workspace).gridTemplateColumns,toolbar:{client:toolbar.clientWidth,scroll:toolbar.scrollWidth},viewport:innerWidth});}`).Str()
				t.Fatalf("narrow Infra window on a wide viewport did not switch to its compact container layout: %s", metrics)
			}
			page.MustEval(`()=>document.querySelector('[data-view="table"]').click()`)
			page.MustWait(`()=>document.querySelectorAll('[data-table-body] [data-row]').length<100`)
			if !page.MustEval(`()=>getComputedStyle(document.querySelector('[data-action="import"]')).display!=='none'&&!!document.querySelector('[data-action="close-inspector"]')&&!!document.querySelector('[data-action="close-inspector"]').getAttribute('aria-label')`).Bool() {
				t.Fatal("narrow inventory must retain an accessible import action and a clear inspector control")
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
