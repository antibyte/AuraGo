package ui

import "testing"

func TestDesktopSIPPhoneKeypadFocusBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	for _, theme := range []string{"standard", "fruity"} {
		t.Run(theme, func(t *testing.T) {
			page := browser.MustPage()
			defer page.MustClose()
			page.MustSetViewport(390, 844, 1, true)
			page.MustEval(`theme => {
				document.body.dataset.theme = theme;
				document.body.innerHTML = '<div id="phone"></div>';
				window.phoneState = {
					appState: {status: {enabled: true, registered: true}, capabilities: {dial: true}},
					preferences: {favorites: []}, call: null, secureContext: true
				};
				window.dialed = [];
				window.SipPhoneRuntime = {
					getState: () => phoneState,
					subscribe: fn => { fn(phoneState); return () => {}; },
					initialize: () => {}, refreshDevices: async () => {},
					dial: async target => { dialed.push(target); }
				};
			}`, theme)
			page.MustEval(`() => {` + readDesktopAssetText(t, "js/desktop/apps/sip-phone.js") + `}`)
			page.MustEval(`async () => {
				SipPhoneApp.render(document.querySelector('#phone'), 'test-phone', {
					t: key => key, esc: value => String(value || ''), iconMarkup: () => '',
					api: async () => []
				});
				await Promise.resolve();
				window.targetFocusCount = 0;
				document.querySelector('[data-sip-phone="target"]').addEventListener('focus', () => targetFocusCount++);
			}`)
			page.MustElement(`[data-sip-digit="1"]`).MustClick()
			page.MustElement(`[data-sip-digit="2"]`).MustClick()
			page.MustElement(`[data-sip-phone-action="backspace"]`).MustClick()
			page.MustEval(`async () => {
				const zero = document.querySelector('[data-sip-digit="0"]');
				zero.dispatchEvent(new PointerEvent('pointerdown', {pointerType: 'touch'}));
				await new Promise(resolve => setTimeout(resolve, 600));
				zero.dispatchEvent(new PointerEvent('pointerup', {pointerType: 'touch'}));
				zero.click();
			}`)
			if !page.MustEval(`() => document.querySelector('[data-sip-phone="target"]').value === '1+' && targetFocusCount === 0`).Bool() {
				t.Fatal("keypad digits, backspace or long-press focused the number field or changed the wrong digits")
			}
			page.MustElement(`[data-sip-phone-action="dial"]`).MustClick()
			if !page.MustEval(`() => dialed[0] === '1+' && targetFocusCount === 0`).Bool() {
				t.Fatal("dial must use the keypad value without focusing the number field")
			}
			page.MustElement(`[data-sip-phone="target"]`).MustClick().MustInput("sip:alice@example.test")
			page.MustEval(`() => document.querySelector('[data-sip-phone="target"]').dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter'}))`)
			if !page.MustEval(`() => targetFocusCount > 0 && dialed[1].includes('sip:alice@example.test')`).Bool() {
				t.Fatal("explicit number-field editing and Enter dialing must remain available")
			}
			page.MustEval(`() => SipPhoneApp.dispose('test-phone')`)
		})
	}
}

// TestDesktopSIPPhoneLaunchDialBrowser covers the People handoff: a launch
// context with a dial value preselects the number on the keypad, the app's
// open() handler re-selects it in an already mounted phone without dialing,
// and the keypad view is reachable again after the preselect.
func TestDesktopSIPPhoneLaunchDialBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage()
	defer page.MustClose()
	page.MustSetViewport(390, 844, 1, true)
	page.MustEval(`() => {
		document.body.innerHTML = '<div id="phone"></div>';
		window.phoneState = {
			appState: {status: {enabled: true, registered: true}, capabilities: {dial: true}},
			preferences: {favorites: []}, call: null, secureContext: true
		};
		window.dialed = [];
		window.SipPhoneRuntime = {
			getState: () => phoneState,
			subscribe: fn => { fn(phoneState); return () => {}; },
			initialize: () => {}, refreshDevices: async () => {},
			dial: async target => { dialed.push(target); }
		};
	}`)
	page.MustEval(`() => {` + readDesktopAssetText(t, "js/desktop/apps/sip-phone.js") + `}`)
	page.MustEval(`async () => {
		SipPhoneApp.render(document.querySelector('#phone'), 'test-phone', {
			t: key => key, esc: value => String(value || ''), iconMarkup: () => '',
			api: async () => [], dial: '+49 30 123456'
		});
		await Promise.resolve();
	}`)
	if !page.MustEval(`() => document.querySelector('[data-sip-phone="target"]').value === '+49 30 123456' && document.querySelector('[data-sip-tab="keypad"]').classList.contains('is-active')`).Bool() {
		t.Fatal("a dial launch context must preselect the number on the keypad view")
	}
	page.MustEval(`() => SipPhoneApp.open('test-phone', { dial: 'sip:ada@example.test' })`)
	if !page.MustEval(`() => document.querySelector('[data-sip-phone="target"]').value === 'sip:ada@example.test' && dialed.length === 0`).Bool() {
		t.Fatal("open() must re-select the number without dialing it")
	}
	page.MustEval(`() => {
		const contactsTab = document.querySelector('[data-sip-tab="contacts"]');
		contactsTab.click();
		SipPhoneApp.open('test-phone', { dial: '102' });
	}`)
	if !page.MustEval(`() => document.querySelector('[data-sip-phone="target"]').value === '102' && document.querySelector('[data-sip-tab="keypad"]').classList.contains('is-active')`).Bool() {
		t.Fatal("open() must return from any tab to the keypad with the dial value")
	}
	page.MustEval(`() => {
		SipPhoneApp.open('test-phone', {});
		SipPhoneApp.open('missing-window', { dial: '103' });
	}`)
	if !page.MustEval(`() => document.querySelector('[data-sip-phone="target"]').value === '102' && dialed.length === 0`).Bool() {
		t.Fatal("open() without a dial value must keep the current keypad value and never dial")
	}
	page.MustEval(`() => SipPhoneApp.dispose('test-phone')`)
	page.MustEval(`async () => {
		SipPhoneApp.render(document.querySelector('#phone'), 'test-phone', {
			t: key => key, esc: value => String(value || ''), iconMarkup: () => '',
			api: async () => []
		});
		await Promise.resolve();
	}`)
	if !page.MustEval(`() => document.querySelector('[data-sip-phone="target"]').value === ''`).Bool() {
		t.Fatal("mounting without a dial context must open with an empty keypad value")
	}
	page.MustEval(`() => SipPhoneApp.dispose('test-phone')`)
}
