package ui

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
)

// A locked Config switch is not an edit. The Firewall section locks its switch
// on hosts that are not Linux; the fixture has no /api/system/os, so the section
// treats the host as one. Neither a click on the locked switch nor a save made
// while it is shown may change the saved firewall.enabled value.

const lockedFirewallSelector = `#content .toggle[data-path="firewall.enabled"]`

// AuraConfigState uses syncToggleA11y's definition of a locked switch. Each
// locked switch here shows a value that differs from the draft; a click on it
// must not copy that value into the draft, while an unlocked switch still syncs.
func TestConfigStateIgnoresClicksOnLockedSwitchesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage("")
	defer page.MustClose()
	page.MustSetDocumentContent(`
		<div class="toggle" id="expando" data-path="locks.expando"></div>
		<div class="toggle" id="aria" data-path="locks.aria" aria-disabled="true"></div>
		<div class="toggle" id="data" data-path="locks.data" data-disabled="true"></div>
		<div class="toggle cfg-toggle-disabled" id="css" data-path="locks.css"></div>
		<div class="toggle" id="open" data-path="locks.open"></div>`)
	if err := page.AddScriptTag("", normalizeAssetText(mustReadUIFile(t, "js/config/state.js"))); err != nil {
		t.Fatalf("load config state: %v", err)
	}
	result := page.MustEval(`()=>{
		const s=window.AuraConfigState;
		s.init({locks:{expando:true, aria:true, data:true, css:true, open:true}});
		s.bind(document);
		document.getElementById('expando').disabled=true;
		document.querySelectorAll('.toggle').forEach(toggle=>toggle.dispatchEvent(new MouseEvent('click',{bubbles:true})));
		return {draft:s.get('locks'), paths:s.dirtyPaths().join(',')};
	}`)
	draft := result.Get("draft")
	for _, key := range []string{"expando", "aria", "data", "css"} {
		if draft.Get(key).Str() != "true" {
			t.Errorf("click on the %s-locked switch changed the draft: %v", key, result)
		}
	}
	if draft.Get("open").Str() != "false" || result.Get("paths").Str() != "locks.open" {
		t.Errorf("click on the unlocked switch must still sync it: %v", result)
	}
}

func waitForLockedFirewallSwitch(t *testing.T, page *rod.Page) {
	t.Helper()
	waitForJSBool(t, page, `()=>document.getElementById('fwEnabledToggle')?.disabled===true`)
	if !page.MustEval(`()=>AuraConfigState.get('firewall.enabled',{saved:true})===true`).Bool() {
		t.Fatal("fixture must save the firewall as enabled")
	}
}

func TestConfigLockedFirewallSwitchIgnoresClicksBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", true) + "/config#overview").Timeout(90 * time.Second)
	defer func() {
		page.MustEval(dropConfigUnloadGuard)
		page.MustClose()
	}()
	openConfigSection(t, page, "firewall")
	waitForLockedFirewallSwitch(t, page)

	clickConfigSwitch(page, lockedFirewallSelector)
	state := page.MustEval(configSwitchState, lockedFirewallSelector, "firewall.enabled")
	if state.Get("dirty").Bool() || state.Get("value").Str() != "true" || state.Get("paths").Str() != "" ||
		state.Get("save").Bool() {
		t.Fatalf("clicking the locked firewall switch changed the draft: %v", state)
	}
	// The locked switch keeps showing the saved value, so a later sync of the
	// section cannot read a different one.
	shown := page.MustEval(`selector=>{const toggle=document.querySelector(selector);
		return {on:toggle.classList.contains('on'), aria:toggle.getAttribute('aria-checked'), locked:toggle.getAttribute('aria-disabled')};}`,
		lockedFirewallSelector)
	if !shown.Get("on").Bool() || shown.Get("aria").Str() != "true" || shown.Get("locked").Str() != "true" {
		t.Fatalf("locked firewall switch must show the saved value and announce the lock: %v", shown)
	}
}

func TestConfigSaveKeepsLockedFirewallValueBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", true) + "/config#overview").Timeout(90 * time.Second)
	defer func() {
		page.MustEval(dropConfigUnloadGuard)
		page.MustClose()
	}()
	// An unrelated edit in another section leaves the draft dirty.
	openConfigSection(t, page, "heartbeat")
	clickConfigSwitch(page, `#content .toggle[data-path="heartbeat.enabled"]`)
	if !page.MustEval(`()=>{const paths=AuraConfigState.dirtyPaths();
		return paths.includes('heartbeat.enabled')&&!paths.some(path=>path.startsWith('firewall.'));}`).Bool() {
		t.Fatalf("expected a heartbeat draft edit, got %q", page.MustEval(`()=>AuraConfigState.dirtyPaths().join(',')`).Str())
	}

	// Saving from the Firewall section reads every switch it shows.
	page.MustEval(`async()=>{await selectSection('firewall',{scrollBehavior:'auto'});}`)
	waitForLockedFirewallSwitch(t, page)
	result := page.MustEval(`async()=>{
		const puts=[]; const original=window.fetch;
		window.fetch=function(url,options){
			if(String(url).endsWith('/api/config')&&options&&options.method==='PUT') puts.push(JSON.parse(options.body));
			return original.apply(this,arguments);
		};
		try { await saveConfig(); } finally { window.fetch=original; }
		return {puts:puts, firewall:AuraConfigState.get('firewall.enabled'), dirty:AuraConfigState.isDirty()};
	}`)
	puts := result.Get("puts").Arr()
	if len(puts) != 1 {
		t.Fatalf("expected one config save, got %v", result)
	}
	patch := puts[0]
	if patch.Has("firewall") || patch.Get("heartbeat.enabled").Str() != "false" {
		t.Fatalf("save must carry the heartbeat edit and no firewall value, sent %v", patch)
	}
	if result.Get("firewall").Str() != "true" || result.Get("dirty").Bool() {
		t.Fatalf("after the save the firewall must stay enabled and the draft clean: %v", result)
	}
}
