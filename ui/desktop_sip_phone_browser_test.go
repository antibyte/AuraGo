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
