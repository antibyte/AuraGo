package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

func TestDesktopInfraListRecoveryBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1100, 800).Timeout(90 * time.Second)
	defer page.Close()
	page.MustElement(`[data-view="table"]`).MustClick()
	page.MustElement(`[data-search]`).MustInput("no-such-asset")
	page.MustWait(`()=>!document.querySelector('[data-table-empty]').hidden`)
	if !page.MustEval(`()=>document.querySelector('.infra-table').hidden&&document.querySelector('[data-table-check-all]').disabled&&document.querySelector('[data-table-total]').textContent===fixtureWords['desktop.infra_visible_count'].replace('{count}',0)&&document.querySelector('[data-inventory-count]').textContent===fixtureWords['desktop.infra_asset_count'].replace('{count}',25)`).Bool() {
		t.Fatal("an empty search must distinguish shown and inventory counts without offering selection")
	}
	page.MustElement(`[data-table-reset]`).MustClick()
	page.MustWait(`()=>document.activeElement===document.querySelector('[data-search]')&&document.querySelector('[data-table-empty]').hidden`)
	page.MustEval(`()=>{const el=document.querySelector('[data-kind-filter]');el.value='network';el.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelector('[data-table-reset]').dataset.action==='reset-filters'`)
	page.MustElement(`[data-table-reset]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-table-empty]').hidden&&document.activeElement===document.querySelector('[data-view="table"]')`)
	if !page.MustEval(`()=>!document.querySelector('[data-table-check-all]').disabled&&document.querySelector('[data-kind-filter]').value===''`).Bool() {
		t.Fatal("reset must restore rows, selection and the filter form")
	}
	page.MustElement(`[data-search]`).MustInput("no-such-asset")
	page.MustEval(`()=>{const el=document.querySelector('[data-kind-filter]');el.value='device';el.dispatchEvent(new Event('change',{bubbles:true}));}`)
	page.MustWait(`()=>document.querySelector('[data-table-reset]').dataset.action==='clear-search-filters'`)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		t.Fatal(err)
	}
	if !page.MustEval(`()=>matchMedia('(pointer: coarse)').matches`).Bool() {
		t.Fatal("touch emulation did not switch the primary pointer to coarse")
	}
	for _, theme := range infraReviewThemes[:3] {
		applyInfraTheme(page, theme)
		infraPolishResize(page, 360, 700)
		if !page.MustEval(`()=>{const box=document.querySelector('[data-table-empty]'),b=box.querySelector('[data-table-reset]').getBoundingClientRect(),r=box.getBoundingClientRect();return b.height>=44&&box.scrollWidth<=box.clientWidth+1&&b.left>=r.left&&b.right<=r.right&&b.bottom<=r.bottom;}`).Bool() {
			t.Fatal("the empty-state action must fit a narrow list", theme.name)
		}
		if err := os.WriteFile(infraReviewArtifactPath(t, "list-recovery-"+theme.name+".png"), page.MustScreenshot(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	page.MustElement(`[data-table-reset]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-search]').value===''&&document.querySelector('[data-kind-filter]').value===''&&document.querySelector('[data-table-empty]').hidden`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

func TestDesktopInfraListInitialStateBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.nodes = []map[string]any{}
	var first, released sync.Once
	var fail atomic.Bool
	fail.Store(true)
	release := make(chan struct{})
	defer released.Do(func() { close(release) })
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/desktop/infra/snapshot" {
			first.Do(func() {
				select {
				case <-release:
				case <-r.Context().Done():
				}
			})
			if fail.Load() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"error":"infra_unavailable"}`))
				return
			}
		}
		fixture.ServeHTTP(w, r)
	})
	page := infraBrowserPageWithFixture(t, handler).Timeout(90 * time.Second)
	defer page.Close()
	openInfraBrowserApp(page)
	page.MustWait(`()=>!!document.querySelector('[data-view="table"]')`)
	page.MustElement(`[data-view="table"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-table-view]').getAttribute('aria-busy')==='true'`)
	if !page.MustEval(`()=>document.querySelector('[data-table-message]').textContent===fixtureWords['desktop.infra_sync_syncing']&&document.querySelector('[data-table-total]').hidden&&document.querySelector('[data-table-empty] [data-action="add-empty"]').hidden`).Bool() {
		t.Fatal("a pending first snapshot must say loading instead of showing an empty inventory")
	}
	if err := os.WriteFile(infraReviewArtifactPath(t, "06-list-loading.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
	released.Do(func() { close(release) })
	page.MustWait(`()=>document.querySelector('[data-table-view]').getAttribute('aria-busy')==='false'&&!document.querySelector('[data-table-empty] [data-action="refresh"]').hidden`)
	if !page.MustEval(`()=>document.querySelector('[data-table-message]').textContent===fixtureWords['desktop.infra_sync_error']&&document.querySelector('[data-table-total]').hidden`).Bool() {
		t.Fatal("an unavailable first snapshot must offer retry without claiming zero assets")
	}
	if err := os.WriteFile(infraReviewArtifactPath(t, "07-list-unavailable.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
	fail.Store(false)
	page.MustElement(`[data-table-empty] [data-action="refresh"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-live]').dataset.sync==='live'&&!document.querySelector('[data-table-total]').hidden`)
	if !page.MustEval(`()=>document.querySelector('[data-table-message]').textContent===fixtureWords['desktop.infra_list_none']&&['add-empty','import-empty'].every(action=>{const b=document.querySelector('[data-table-empty] [data-action="'+action+'"]');return !b.hidden&&!b.disabled;})`).Bool() {
		t.Fatal("a confirmed empty inventory must offer create and import")
	}
	if err := os.WriteFile(infraReviewArtifactPath(t, "08-list-empty.png"), page.MustScreenshot(), 0644); err != nil {
		t.Fatal(err)
	}
	fixture.mu.Lock()
	fixture.capabilities = map[string]bool{"write": false, "source_edit": false, "control": false}
	fixture.mu.Unlock()
	page.MustElement(`.infra-status-refresh`).MustClick()
	page.MustWait(`()=>document.querySelector('.infra-app').dataset.readonly==='true'`)
	if !page.MustEval(`()=>['add-empty','import-empty'].every(action=>document.querySelector('[data-table-empty] [data-action="'+action+'"]').hidden)`).Bool() {
		t.Fatal("an empty read-only inventory must not offer writes")
	}
	fixture.mu.Lock()
	fixture.capabilities["write"] = true
	fixture.mu.Unlock()
	page.MustElement(`.infra-status-refresh`).MustClick()
	page.MustWait(`()=>document.querySelector('.infra-app').dataset.readonly==='false'`)
	page.MustElement(`[data-table-empty] [data-action="import-empty"]`).MustClick()
	page.MustWait(`()=>!document.querySelector('[data-import-sheet]').hidden`)
	page.MustElement(`[data-action="close-import"]`).MustClick()
	page.MustWait(`()=>document.querySelector('[data-import-sheet]').hidden`)
	page.MustElement(`[data-table-empty] [data-action="add-empty"]`).MustClick()
	page.MustWait(`()=>!!document.querySelector('dialog[open]')`)
	page.Keyboard.MustType(input.Escape)
	page.MustWait(`()=>!document.querySelector('dialog[open]')`)
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}

func TestDesktopInfraInspectorTabVisibilityBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	fixture := newInfraAppBrowserFixture()
	fixture.world(25)
	page := infraPolishOpen(t, fixture, 1000, 800).Timeout(2 * time.Minute)
	defer page.Close()
	page.MustEval(`()=>document.querySelector('.infra-app').style.setProperty('--infra-inspector-w','320px')`)
	if err := (proto.EmulationSetTouchEmulationEnabled{Enabled: true}).Call(page); err != nil {
		t.Fatal(err)
	}
	for index, code := range []string{"de", "en", "cs", "da", "el", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var words map[string]any
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+code+".json")), &words); err != nil {
			t.Fatal(err)
		}
		page.MustEval(`(words,id)=>{Object.assign(fixtureWords,words);document.querySelector('[data-node="'+id+'"]').click();}`, words, []string{"fixture-node-0003", "fixture-node-0004"}[index%2])
		page.MustWait(`()=>!document.querySelector('[data-inspector]').hidden`)
		page.MustElement(`[data-inspector-tab="overview"]`).MustFocus()
		page.Keyboard.MustType(input.End)
		page.MustWait(`()=>document.querySelector('[data-inspector-tab="connections"]').getAttribute('aria-selected')==='true'`)
		const visible = `()=>{const tab=document.querySelector('[data-inspector-tab="connections"]'),bar=tab.parentElement,r=tab.getBoundingClientRect(),b=bar.getBoundingClientRect();return tab.getAttribute('aria-selected')==='true'&&r.height>=44&&r.left>=b.left-1&&r.right<=b.right+1&&document.querySelector('.infra-app').scrollTop===0&&document.documentElement.scrollTop===0;}`
		if !page.MustEval(visible).Bool() {
			t.Fatal("the last tab must be fully visible without scrolling the page", code)
		}
		// A different asset restores the remembered tab and reveals it after repaint, too.
		page.MustEval(`id=>document.querySelector('[data-node="'+id+'"]').click()`, []string{"fixture-node-0004", "fixture-node-0003"}[index%2])
		if !page.MustEval(visible).Bool() {
			t.Fatal("a remembered tab must stay visible after repaint", code)
		}
	}
	closeInfraBrowserWindow(t, page)
	requireInfraBrowserClean(t, page)
}
