package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func infraImportFixtureCandidate(id, kind, name, icon string, source, profile map[string]any) map[string]any {
	return map[string]any{"id": id, "source": source, "node": map[string]any{"kind": kind, "name": name, "icon": icon, "profile": profile}}
}

// TestDesktopInfraImportSheetBrowser covers the non-modal import sheet: relevance groups with reasons, the
// search filter, Escape and focus return, Ignore, dragging a discovery onto a place of the map (one batch
// call with a placement and the place as parent), dragging a guest (no placement, drop label names the host)
// and importing a duplicate onto the asset it duplicates.
func TestDesktopInfraImportSheetBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.failCandidateOnce = ""
	fixture.nodes = []map[string]any{
		infraCanvasFixtureNode("room-1", "room", "Living room", "room", map[string]any{"x": 560, "y": 120, "w": 720, "h": 480, "pinned": true}, ""),
		infraCanvasFixtureNode("dev-a", "device", "Edge router", "router", map[string]any{"x": 160, "y": 200, "pinned": true}, ""),
		infraCanvasFixtureNode("dup-a", "device", "Backup NAS", "nas", map[string]any{"x": 160, "y": 420, "pinned": true}, ""),
	}
	fixture.nodes[2]["profile"] = map[string]any{"ip_address": "192.0.2.77", "tags": []any{}, "interfaces": []any{}, "links": []any{}, "app_bindings": []any{}, "custom": map[string]any{}}
	src := func(kind, instance, key, name string) map[string]any {
		return map[string]any{"kind": kind, "instance": instance, "key": key, "name": name, "state": "current"}
	}
	fixture.candidates = []map[string]any{
		infraImportFixtureCandidate("reg-printer", "device", "Studio printer", "printer", src("registry", "local", "reg-printer", "Device registry"), map[string]any{"vendor": "Brother", "ip_address": "192.0.2.50", "interfaces": []any{}}),
		infraImportFixtureCandidate("docker-web", "container", "web-front", "container", src("docker", "engine-a", "web-1", "Docker Engine"), map[string]any{"vendor": "Docker", "model": "nginx:stable", "interfaces": []any{}}),
		infraImportFixtureCandidate("fritz-nas", "device", "FRITZ NAS", "nas", src("fritzbox", "fritz", "fritz-nas", "FRITZ!Box"), map[string]any{"ip_address": "192.0.2.77", "interfaces": []any{}}),
		infraImportFixtureCandidate("mdns-generic", "device", "a1b2c3d4e5f6", "generic-device", src("registry", "local", "mdns-1", "Device registry"), map[string]any{"role": "mdns-device", "tags": []any{"network-scan"}, "ip_address": "192.0.2.90", "interfaces": []any{}}),
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-a"]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="fit"]').click()`)
	// The audit page speaks German: texts are compared with the dictionary the page loaded.
	page.MustEval(`()=>{window.w=(key,vars={})=>Object.entries(vars).reduce((s,[k,v])=>s.replaceAll('{'+k+'}',v),window.fixtureWords[key]);}`)
	settle := func() {
		page.MustWait(`()=>!document.querySelector('[data-scene]').classList.contains('is-animating')`)
		page.MustEval(`()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
	}
	settle()
	openSheet := func() {
		page.MustEval(`()=>document.querySelector('[data-action="add-menu"]').click()`)
		page.MustWait(`()=>!document.querySelector('[data-menu="add"]').hidden`)
		page.MustEval(`()=>document.querySelector('[data-menu="add"] [data-action="import"]').click()`)
		page.MustWait(`()=>!document.querySelector('[data-import-sheet]').hidden&&!document.querySelector('[data-import-loading]').offsetParent`)
	}

	// Three relevance groups; the recommended rows start selected, the duplicate carries its reason and a
	// preselected link, and the "probably not homelab" group starts folded.
	openSheet()
	page.MustWait(`()=>document.querySelectorAll('[data-candidate-row]').length===3`)
	got := page.MustEval(`()=>JSON.stringify({
		counts:['recommended','duplicate','unlikely'].map(g=>document.querySelector('[data-import-group-count="'+g+'"]').textContent),
		folded:document.querySelector('[data-import-list="unlikely"]').hidden,
		selected:document.querySelector('[data-import-count]').textContent===w('desktop.infra_selected_count',{count:2}),
		checked:[...document.querySelectorAll('[data-candidate]')].filter(box=>box.checked).map(box=>box.dataset.candidate).sort(),
		reason:document.querySelector('[data-candidate-row="fritz-nas"] [data-candidate-reason]')?.textContent===w('desktop.infra_relevance_duplicate_ip',{name:'Backup NAS'}),
		bound:document.querySelector('[data-bind="fritz-nas"]')?.value||'',
		modal:document.querySelector('[data-dialog]').open,
	})`).Str()
	if want := `{"counts":["2","1","1"],"folded":true,"selected":true,"checked":["docker-web","reg-printer"],"reason":true,"bound":"dup-a","modal":false}`; got != want {
		t.Fatalf("import sheet state %s; want %s", got, want)
	}

	// The search filter narrows the rows; Escape closes the sheet and returns focus to the trigger.
	page.MustEval(`()=>{const search=document.querySelector('[data-import-search]');search.value='printer';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelectorAll('[data-candidate-row]').length===1&&!!document.querySelector('[data-candidate-row="reg-printer"]')`)
	page.MustEval(`()=>{const search=document.querySelector('[data-import-search]');search.value='';search.dispatchEvent(new Event('input',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelectorAll('[data-candidate-row]').length===3`)
	page.MustEval(`()=>document.querySelector('[data-import-search]').focus()`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>document.querySelector('[data-import-sheet]').hidden`)
	if !page.MustEval(`()=>document.activeElement===document.querySelector('[data-action="add-menu"]')`).Bool() {
		t.Fatal("closing the import sheet must return focus to the control that opened it")
	}
	openSheet()
	page.MustWait(`()=>document.querySelectorAll('[data-candidate-row]').length===3`)

	// Ignoring a discovery writes the ignore, removes the row and lists the source under Ignored.
	page.MustEval(`()=>document.querySelector('[data-import-group-toggle="unlikely"]').click()`)
	page.MustWait(`()=>!!document.querySelector('[data-candidate-row="mdns-generic"] [data-candidate-reason]')`)
	if !page.MustEval(`()=>document.querySelector('[data-candidate-row="mdns-generic"] [data-candidate-reason]').textContent===w('desktop.infra_relevance_generic_discovery')`).Bool() {
		t.Fatal("a generic scan result must say why it is probably not homelab")
	}
	page.MustEval(`()=>document.querySelector('[data-ignore-candidate="mdns-generic"]').click()`)
	page.MustWait(`()=>!document.querySelector('[data-candidate-row="mdns-generic"]')&&document.querySelector('[data-import-group="unlikely"]').hidden`)
	page.MustWait(`()=>document.querySelector('[data-section-count="ignored"]')?.textContent==='1'`)
	fixture.mu.Lock()
	ignores := append([]map[string]any(nil), fixture.ignoreCalls...)
	fixture.mu.Unlock()
	if len(ignores) != 1 || ignores[0]["ignored"] != true || fmt.Sprint(ignores[0]["sources"]) != "[map[instance:local key:mdns-1 kind:registry]]" {
		t.Fatalf("ignoring the discovery wrote %v", ignores)
	}

	// A discovery dragged onto a place of the map is imported with a placement inside that place.
	centreOf := func(selector string) (float64, float64) {
		settle()
		point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return [r.left+r.width/2,r.top+r.height/2];}`, selector).Arr()
		return point[0].Num(), point[1].Num()
	}
	batchCalls := func() []map[string]any {
		fixture.mu.Lock()
		defer fixture.mu.Unlock()
		return append([]map[string]any(nil), fixture.batchCalls...)
	}
	waitBatches := func(want int) []map[string]any {
		deadline := time.Now().Add(8 * time.Second)
		for {
			if calls := batchCalls(); len(calls) >= want {
				return calls
			}
			if time.Now().After(deadline) {
				t.Fatalf("expected %d import batches, have %d", want, len(batchCalls()))
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	// A point inside the place that is neither under the sheet nor under the inspector (it opens when an asset is selected).
	dropAt := func() (float64, float64) {
		settle()
		point := page.MustEval(`()=>{const r=document.querySelector('[data-region="room-1"]').getBoundingClientRect(),sheet=document.querySelector('[data-import-sheet]').getBoundingClientRect(),map=document.querySelector('.infra-canvas').getBoundingClientRect();return [Math.min(Math.max(r.left+r.width*0.7,sheet.right+40),map.right-40),r.top+r.height*0.8];}`).Arr()
		return point[0].Num(), point[1].Num()
	}
	dropX, dropY := dropAt()
	rowX, rowY := centreOf(`[data-candidate-row="reg-printer"] .infra-candidate-copy strong`)
	page.Mouse.MustMoveTo(rowX, rowY).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo((rowX+dropX)/2, (rowY+dropY)/2)
	page.Mouse.MustMoveTo(dropX, dropY)
	page.MustWait(`()=>document.querySelector('.infra-drag-ghost')?.dataset.dropLabel===w('desktop.infra_drop_into',{name:'Living room'})`)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	calls := waitBatches(1)
	items, _ := calls[0]["items"].([]map[string]any)
	placement, _ := items[0]["placement"].(map[string]any)
	if len(items) != 1 || items[0]["candidate_id"] != "reg-printer" || items[0]["parent_id"] != "room-1" || placement["view"] != "atlas" || placement["x"] == nil || placement["y"] == nil || placement["pinned"] != true {
		t.Fatalf("dropping a discovery on a place posted %v; want one item with a pinned placement and parent_id room-1", calls[0])
	}
	page.MustWait(`()=>!!document.querySelector('[data-node="imported-1"]')&&!document.querySelector('[data-candidate-row="reg-printer"]')&&document.querySelector('[data-inspector] h2')?.textContent==='Studio printer'`)
	if page.MustEval(`()=>document.querySelector('[data-import-sheet]').hidden`).Bool() {
		t.Fatal("the sheet must stay open after a drop so more discoveries can follow")
	}

	// A guest is imported without a placement; while it is dragged the drop label names its host.
	rowX, rowY = centreOf(`[data-candidate-row="docker-web"] .infra-candidate-copy strong`)
	dropX, dropY = dropAt()
	page.Mouse.MustMoveTo(rowX, rowY).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo((rowX+dropX)/2, (rowY+dropY)/2)
	page.Mouse.MustMoveTo(dropX, dropY)
	page.MustWait(`()=>document.querySelector('.infra-drag-ghost')?.dataset.dropLabel===w('desktop.infra_drop_guest')`)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	calls = waitBatches(2)
	items, _ = calls[1]["items"].([]map[string]any)
	if len(items) != 1 || items[0]["candidate_id"] != "docker-web" || items[0]["placement"] != nil || items[0]["parent_id"] != nil {
		t.Fatalf("dropping a guest posted %v; want no placement and no parent", calls[1])
	}
	page.MustWait(`()=>!document.querySelector('[data-candidate-row="docker-web"]')`)

	// The duplicate imports onto the asset it duplicates: its source joins Backup NAS and the sheet closes.
	page.MustEval(`()=>{const box=document.querySelector('[data-candidate="fritz-nas"]');box.checked=true;box.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelector('[data-import-count]').textContent===w('desktop.infra_selected_count',{count:1})&&!document.querySelector('[data-import]').disabled`)
	page.MustEval(`()=>document.querySelector('[data-import]').click()`)
	page.MustWait(`()=>document.querySelector('[data-import-sheet]').hidden`)
	calls = waitBatches(3)
	items, _ = calls[2]["items"].([]map[string]any)
	if len(items) != 1 || items[0]["candidate_id"] != "fritz-nas" || items[0]["node_id"] != "dup-a" {
		t.Fatalf("importing the duplicate posted %v; want node_id dup-a", calls[2])
	}
	fixture.mu.Lock()
	sources, _ := fixture.nodes[2]["sources"].([]any)
	fixture.mu.Unlock()
	if len(sources) != 1 {
		t.Fatalf("Backup NAS has %d sources after the import, want the FRITZ!Box source", len(sources))
	}

	// A duplicate dropped on the map only links the asset it duplicates: Backup NAS is already placed, so the drop
	// sends no placement and no parent_id (the import is not undoable and would move it into the place) and selects it.
	fixture.mu.Lock()
	fixture.candidates = append(fixture.candidates, infraImportFixtureCandidate("fritz-nas-2", "device", "FRITZ NAS 2", "nas", src("fritzbox", "fritz", "fritz-nas-2", "FRITZ!Box"), map[string]any{"ip_address": "192.0.2.77", "interfaces": []any{}}))
	fixture.mu.Unlock()
	openSheet()
	page.MustWait(`()=>document.querySelector('[data-bind="fritz-nas-2"]')?.value==='dup-a'`)
	rowX, rowY = centreOf(`[data-candidate-row="fritz-nas-2"] .infra-candidate-copy strong`)
	dropX, dropY = dropAt()
	page.Mouse.MustMoveTo(rowX, rowY).MustDown(proto.InputMouseButtonLeft)
	page.Mouse.MustMoveTo((rowX+dropX)/2, (rowY+dropY)/2)
	page.Mouse.MustMoveTo(dropX, dropY)
	page.MustWait(`()=>document.querySelector('.infra-drag-ghost')?.dataset.dropLabel===w('desktop.infra_drop_into',{name:'Living room'})`)
	page.Mouse.MustUp(proto.InputMouseButtonLeft)
	calls = waitBatches(4)
	items, _ = calls[3]["items"].([]map[string]any)
	if len(items) != 1 || items[0]["candidate_id"] != "fritz-nas-2" || items[0]["node_id"] != "dup-a" || items[0]["placement"] != nil || items[0]["parent_id"] != nil {
		t.Fatalf("dropping a duplicate of a placed asset posted %v; want node_id dup-a and no placement or parent", calls[3])
	}
	page.MustWait(`()=>document.querySelector('[data-inspector] h2')?.textContent==='Backup NAS'&&!document.querySelector('[data-candidate-row="fritz-nas-2"]')`)
	page.MustEval(`()=>document.querySelector('[data-action="close-import"]').click()`)

	// An asset with sources can be rediscovered: its removal confirmation points at "Remove and ignore".
	page.MustEval(`()=>document.querySelector('[data-library-item="dup-a"] [data-library-select]').focus()`)
	page.Keyboard.MustType(input.Delete)
	page.MustWait(`()=>!!document.querySelector('.vd-modal [data-cancel]')`)
	if !page.MustEval(`()=>document.querySelector('.vd-modal .vd-modal-copy').textContent===w('desktop.infra_remove_node_confirm_sources',{name:'Backup NAS'})`).Bool() {
		t.Fatal("removing an asset with sources must point at Remove and ignore")
	}
	page.MustEval(`()=>document.querySelector('.vd-modal [data-cancel]').click()`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

// TestDesktopInfraImportSourceChipsBrowser covers how the import sheet names sources: the kind only while a kind
// has one instance, a numbered name ("Docker 1") once it has several, and one chip per source however many rows it has.
func TestDesktopInfraImportSourceChipsBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.failCandidateOnce = ""
	fixture.nodes = []map[string]any{infraCanvasFixtureNode("dev-a", "device", "Edge router", "router", map[string]any{"x": 160, "y": 200, "pinned": true}, "")}
	src := func(kind, instance, key, name string) map[string]any {
		return map[string]any{"kind": kind, "instance": instance, "key": key, "name": name, "state": "current"}
	}
	fixture.candidates = []map[string]any{
		infraImportFixtureCandidate("reg-printer", "device", "Studio printer", "printer", src("registry", "inventory", "reg-printer", "Studio printer"), map[string]any{"ip_address": "192.0.2.50", "interfaces": []any{}}),
		infraImportFixtureCandidate("web-a", "container", "web-front", "container", src("docker", "engine-a", "web-1", "web-front"), map[string]any{"model": "nginx:stable", "interfaces": []any{}}),
	}
	stale := src("docker", "engine-b", "db-1", "db")
	stale["state"] = "stale"
	fixture.candidateSources = []any{
		src("registry", "inventory", "reg-printer", "Studio printer"), src("registry", "inventory", "reg-tv", "Living room TV"), src("registry", "inventory", "reg-nas", "NAS"),
		src("docker", "engine-a", "web-1", "web-front"), src("docker", "engine-a", "web-2", "web-back"), stale,
	}
	page := infraBrowserPageWithFixture(t, fixture)
	defer page.Close()
	page.MustSetViewport(1920, 1200, 1, false)
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-node="dev-a"]')`)
	page.MustEval(`()=>{const win=document.querySelector('.vd-window[data-app-id="infra"]');win.classList.remove('maximized','vd-mobile-forced-maximized');win.style.right='auto';win.style.width='1500px';win.style.height='970px';win.style.left='210px';win.style.top='70px';window.w=(key,vars={})=>Object.entries(vars).reduce((s,[k,v])=>s.replaceAll('{'+k+'}',v),window.fixtureWords[key]);}`)
	page.MustWait(`()=>document.querySelector('.infra-app').clientWidth>1000`)
	page.MustEval(`()=>document.querySelector('[data-action="add-menu"]').click()`)
	page.MustEval(`()=>document.querySelector('[data-menu="add"] [data-action="import"]').click()`)
	page.MustWait(`()=>document.querySelectorAll('[data-candidate-row]').length===2&&document.querySelectorAll('.infra-source-chip').length===3`)
	got := page.MustEval(`()=>JSON.stringify({
		chips:[...document.querySelectorAll('.infra-source-chip')].map(chip=>chip.textContent+':'+chip.dataset.state),
		rows:[...document.querySelectorAll('[data-candidate-row]')].map(row=>row.dataset.candidateRow+'='+row.querySelector('.infra-candidate-copy > span').textContent),
	})`).Str()
	registry := page.MustEval(`()=>window.fixtureWords['desktop.infra_source_registry']`).Str()
	docker := page.MustEval(`()=>window.fixtureWords['desktop.infra_source_docker']`).Str()
	want := fmt.Sprintf(`{"chips":["%s:current","%s 1:current","%s 2:stale"],"rows":["reg-printer=%s","web-a=%s 1"]}`, registry, docker, docker, registry, docker)
	if got != want {
		t.Fatalf("import sources = %s; want %s", got, want)
	}
	// The opaque instance id never shows in the text; it only goes into the tooltip.
	titles := page.MustEval(`()=>JSON.stringify([...document.querySelectorAll('.infra-source-chip')].map(chip=>chip.title))`).Str()
	if wantTitles := fmt.Sprintf(`["%s · inventory","%s · engine-a","%s · engine-b"]`, registry, docker, docker); titles != wantTitles {
		t.Fatalf("source chip tooltips = %s; want %s", titles, wantTitles)
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
