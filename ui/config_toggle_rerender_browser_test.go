package ui

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Some section switches re-render #content from their click handler. State
// notifications and the bubbling AuraConfigState click listener still read the
// clicked switch, so it must already show the new value.

// rod treats a switch's ::after knob as covering the switch; click its center directly.
func clickConfigSwitch(page *rod.Page, selector string) {
	page.MustElement(selector).MustScrollIntoView()
	point := page.MustEval(`selector=>{const r=document.querySelector(selector).getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2];}`, selector).Arr()
	page.Mouse.MustMoveTo(point[0].Num(), point[1].Num()).MustClick(proto.InputMouseButtonLeft)
}

const configSwitchState = `(selector,path)=>{
    const toggle=document.querySelector(selector);
    return {dirty:AuraConfigState.isDirty(), value:AuraConfigState.get(path),
        paths:AuraConfigState.dirtyPaths().join(','), tracked:AuraConfigState.dirtyPaths().includes(path),
        on:toggle.classList.contains('on'), aria:toggle.getAttribute('aria-checked'),
        save:!document.getElementById('btnSave').disabled};
}`

// A dirty config page would otherwise block navigation and closing.
const dropConfigUnloadGuard = `()=>{if(typeof handleConfigBeforeUnload==='function')window.removeEventListener('beforeunload',handleConfigBeforeUnload);}`

func openConfigSection(t *testing.T, page *rod.Page, section string) {
	t.Helper()
	waitForJSBool(t, page, `()=>!!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async section=>{await selectSection(section,{scrollBehavior:'auto'}); resetDirtySnapshot();}`, section)
	if page.MustEval(`()=>AuraConfigState.isDirty()`).Bool() {
		t.Fatalf("%s: fixture must start with a clean draft", section)
	}
}

func TestGameMakerConfigEnableToggleKeepsDraftDirtyBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview").Timeout(60 * time.Second)
	defer func() {
		page.MustEval(dropConfigUnloadGuard)
		page.MustClose()
	}()
	openConfigSection(t, page, "game_maker")
	const path = "game_maker.enabled"
	const selector = `#content .toggle[data-path="game_maker.enabled"]`
	if page.MustEval(`path=>AuraConfigState.get(path)!==false`, path).Bool() {
		t.Fatal("fixture must start with Game Maker disabled")
	}

	clickConfigSwitch(page, selector)
	on := page.MustEval(configSwitchState, selector, path)
	if !on.Get("dirty").Bool() || on.Get("value").Str() != "true" || on.Get("paths").Str() != path ||
		!on.Get("on").Bool() || on.Get("aria").Str() != "true" || !on.Get("save").Bool() ||
		!page.MustEval(`()=>!!document.querySelector('#content [data-path="game_maker.readonly"]')`).Bool() {
		t.Fatalf("enabling Game Maker lost the draft change: %v", on)
	}

	clickConfigSwitch(page, selector)
	off := page.MustEval(configSwitchState, selector, path)
	if off.Get("dirty").Bool() || off.Get("value").Str() != "false" || off.Get("paths").Str() != "" ||
		off.Get("on").Bool() || off.Get("aria").Str() != "false" || off.Get("save").Bool() {
		t.Fatalf("disabling Game Maker again did not return to the saved state: %v", off)
	}
}

func TestConfigRerenderingTogglesKeepDraftBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	switches := []struct{ section, path string }{
		{"manifest", "manifest.enabled"},
		{"dograh", "dograh.enabled"},
		{"space_agent", "space_agent.enabled"},
		{"omniroute", "omniroute.enabled"},
		{"virtual_desktop", "virtual_desktop.enabled"},
		{"virtual_desktop", "virtual_desktop.allow_agent_control"},
		{"virtual_computers", "virtual_computers.enabled"},
		{"game_maker", "game_maker.enabled"},
	}
	for _, populated := range []bool{false, true} {
		origin := configRefreshFixtureOrigin(t, "en", populated)
		browser := newSmokeBrowser(t)
		page := browser.MustPage("").Timeout(150 * time.Second)
		for i, sw := range switches {
			selector := `#content .toggle[data-path="` + sw.path + `"]`
			// A query keeps each case a full page load; a fragment-only change would not reload.
			page.MustEval(dropConfigUnloadGuard)
			page.MustNavigate(fmt.Sprintf("%s/config?case=%d#overview", origin, i)).MustWaitLoad()
			openConfigSection(t, page, sw.section)
			was := page.MustEval(`selector=>document.querySelector(selector).classList.contains('on')`, selector).Bool()

			clickConfigSwitch(page, selector)
			first := page.MustEval(configSwitchState, selector, sw.path)
			if first.Get("value").Bool() != !was || first.Get("on").Bool() != !was || first.Get("aria").Str() != strconv.FormatBool(!was) ||
				!first.Get("dirty").Bool() || !first.Get("save").Bool() || !first.Get("tracked").Bool() {
				t.Errorf("populated=%v %s: first click lost the draft change: %v", populated, sw.path, first)
				continue
			}

			clickConfigSwitch(page, selector)
			second := page.MustEval(configSwitchState, selector, sw.path)
			if second.Get("value").Bool() != was || second.Get("on").Bool() != was || second.Get("tracked").Bool() {
				t.Errorf("populated=%v %s: second click did not restore the saved value: %v", populated, sw.path, second)
			}
		}
		page.MustEval(dropConfigUnloadGuard)
		page.MustClose()
	}
}
