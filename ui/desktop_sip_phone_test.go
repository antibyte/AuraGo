package ui

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestDesktopSIPPhoneRegistrationAndLazyLifecycle(t *testing.T) {
	t.Parallel()

	desktopTypes, err := os.ReadFile("../internal/desktop/types.go")
	if err != nil {
		t.Fatalf("read desktop app registry: %v", err)
	}
	if !strings.Contains(string(desktopTypes), `{ID: "sip-phone", Name: "Phone"`) {
		t.Fatal("desktop app registry must expose the built-in sip-phone app")
	}

	checks := map[string][]string{
		"js/desktop/core/module-loader.js": {
			"'sip-phone': {",
			"styles: appStyles('/css/desktop-app-sip-phone.css')",
			"scripts: ['/js/desktop/apps/sip-phone.js']",
			"'sip-phone': ['sip_phone']",
		},
		"js/desktop/core/menus-and-routing.js": {
			"if (appId === 'sip-phone')",
			"window.AuraDesktopModules.loadAppScript('sip-phone')",
			"window.SipPhoneApp.render",
		},
		"js/desktop/apps/sip-phone.js": {
			"const instances = new Map()",
			"function mount(host, windowId, context)",
			"dispose(windowId);",
			"function dispose(windowId)",
			"window.SipPhoneApp = { render: mount, dispose }",
		},
		"js/desktop/core/window-shell-runtime.js": {
			"'sip-phone': { width: 1120, height: 720 }",
			"'sip-phone': { width: 720, height: 560 }",
		},
	}
	for path, markers := range checks {
		source := readDesktopAssetText(t, path)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing SIP phone contract %q", path, marker)
			}
		}
	}
}

func TestDesktopSIPPhoneBrowserMediaAndShellContracts(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/sip-phone-runtime.js")
	for _, marker := range []string{
		"new RTCPeerConnection({ iceServers: [] })",
		"codec.mimeType || '').toLowerCase() === 'audio/pcmu'",
		"Number(codec.clockRate) === 8000",
		"media_mode: 'browser'",
		"browser_session_id: sipPhoneShellState.browserSessionID",
		"new EventSource('/api/sip/events'",
		"sipPhoneTerminalEventMessage(event.data, sipPhoneShellState.callID)",
		"describeEndReason: sipPhoneEndReasonMessage",
		"await refreshSIPPhoneState()",
		"await audioLease.acquire('sip-phone'",
		"track.enabled = false",
		"call.state === 'active'",
		"disposeSIPPhoneMediaResources",
		"startSIPPhoneRinging()",
		"startSIPPhoneOutgoingRingback()",
		"call.direction === 'inbound' && call.state === 'ringing'",
		"sipPhoneText('outgoing_ringing', 'The other phone is ringing')",
		"document.createElement('audio')",
		"document.body.appendChild(pendingRemoteAudio)",
		"new MediaStream([event.track])",
		"startSIPPhoneRemoteAudio(pendingRemoteAudio)",
		"enableAudio: enableSIPPhoneAudio",
		"window.SipPhoneRuntime = {",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("SIP phone runtime missing media or shell contract %q", marker)
		}
	}
	bootstrap := readDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	if !strings.Contains(bootstrap, "window.addEventListener('beforeunload', cleanupDesktopShellRuntime)") ||
		!strings.Contains(bootstrap, "closeSIPPhoneShellRuntime()") {
		t.Fatal("desktop shell must end SIP browser media when the tab closes")
	}

	lease := readDesktopAssetText(t, "js/shared/browser-audio-lease.js")
	for _, marker := range []string{
		"new BroadcastChannel",
		"localStorage.setItem",
		"navigator.locks.request",
		"ifAvailable: true",
		"audio_session_busy",
		"window.addEventListener('pagehide'",
	} {
		if !strings.Contains(lease, marker) {
			t.Fatalf("shared browser audio lease missing %q", marker)
		}
	}

	speech := readDesktopAssetText(t, "js/realtime-speech/core.js")
	if !strings.Contains(speech, "await window.AuraBrowserAudioLease.acquire") ||
		!strings.Contains(speech, "window.AuraBrowserAudioLease.release") {
		t.Fatal("Realtime Speech must participate in the shared browser audio lease")
	}
}

func TestDesktopSIPPhoneComfortAndPrivacyContracts(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/sip-phone.js")
	for _, marker := range []string{
		"'/api/contacts'",
		"favorites.length >= 24",
		"data-sip-redial",
		"data-sip-copy",
		"snapshot.audioPlaybackBlocked",
		"data-sip-phone-action=\"enable-audio\"",
		"runtime.describeEndReason(call.end_reason)",
		`data-sip-phone="input-device"`,
		`data-sip-phone="output-device"`,
		`data-sip-phone="ringtone"`,
		"runtime.setMuted",
		"runtime.sendDTMF",
		"text(instance, 'outgoing_ringing', 'The other phone is ringing')",
		`<p aria-live="polite">`,
		"blockers.includes('outbound_disabled')",
		"text(instance, 'unavailable', 'Calling is unavailable')",
	} {
		if !strings.Contains(app, marker) {
			t.Fatalf("SIP phone app missing comfort contract %q", marker)
		}
	}
	if strings.Contains(app, "text(instance, 'setup_required'") {
		t.Fatal("registered-but-locked SIP must show the concrete calling blocker, not a generic setup-required title")
	}

	combined := strings.ToLower(app + readDesktopAssetText(t, "css/desktop-app-sip-phone.css"))
	if strings.Contains(combined, "voicemail") || strings.Contains(combined, "mailbox") {
		t.Fatal("SIP phone must not expose a voicemail or mailbox UI without backend support")
	}
}

func TestDesktopSIPPhoneReloadObserverCanEndCall(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/sip-phone.js")
	runtime := readDesktopAssetText(t, "js/desktop/core/sip-phone-runtime.js")

	if !strings.Contains(runtime, "sipPhoneShellState.callID || (sipPhoneShellState.appState && sipPhoneShellState.appState.active_call && sipPhoneShellState.appState.active_call.id)") {
		t.Fatal("SIP hangup must fall back to the server-reported active call after a page reload")
	}

	const hangupMarker = `class="sip-phone-hangup" data-sip-phone-action="hangup"`
	hangupAt := strings.Index(app, hangupMarker)
	if hangupAt < 0 {
		t.Fatal("SIP phone is missing its hangup control")
	}
	hangupLine := app[hangupAt:]
	if lineEnd := strings.IndexByte(hangupLine, '\n'); lineEnd >= 0 {
		hangupLine = hangupLine[:lineEnd]
	}
	if strings.Contains(hangupLine, "snapshot.observer") || strings.Contains(hangupLine, "disabled") {
		t.Fatal("an observer created by a page reload must still be able to end the active call")
	}

	for _, marker := range []string{
		`data-sip-phone-action="mute" class="${snapshot.muted ? 'is-active' : ''}" ${snapshot.observer ? 'disabled' : ''}`,
		`data-sip-phone-action="toggle-keypad" ${snapshot.observer ? 'disabled' : ''}`,
		`data-sip-phone="volume" min="0" max="1" step="0.05" value="${Number(snapshot.preferences.volume || 0)}" ${snapshot.observer ? 'disabled' : ''}`,
	} {
		if !strings.Contains(app, marker) {
			t.Fatalf("observer must not regain browser media control after reload; missing contract %q", marker)
		}
	}
}

func TestDesktopSIPPhoneFloatingGadgetContracts(t *testing.T) {
	t.Parallel()

	bundler, err := os.ReadFile("../scripts/build-ui-bundles.js")
	if err != nil {
		t.Fatalf("read bundle build script: %v", err)
	}
	if !strings.Contains(string(bundler), "'ui/js/desktop/core/sip-phone-gadget-runtime.js'") {
		t.Fatal("desktop main bundle must include the floating phone gadget runtime")
	}

	desktopTypes, err := os.ReadFile("../internal/desktop/types.go")
	if err != nil {
		t.Fatalf("read desktop types: %v", err)
	}
	for _, marker := range []string{
		`{Key: "phone_gadget.enabled", Default: "false", Values: []string{"true", "false"}}`,
		`{Key: "phone_gadget.position_x", Default: ""}`,
		`{Key: "phone_gadget.position_y", Default: ""}`,
		`{Key: "phone_gadget.always_on_top", Default: "false", Values: []string{"true", "false"}}`,
	} {
		if !strings.Contains(string(desktopTypes), marker) {
			t.Fatalf("backend desktop setting definitions missing %q", marker)
		}
	}

	checks := map[string][]string{
		"js/desktop/core/sip-phone-gadget-runtime.js": {
			"settingValue('phone_gadget.enabled')",
			"settingValue('phone_gadget.always_on_top')",
			"'phone_gadget.position_x'",
			"modules.loadAppScript('sip-phone')",
			"window.SipPhoneApp.render(host, GADGET_WINDOW_ID, {",
			"window.SipPhoneApp.dispose(GADGET_WINDOW_ID)",
			"showPhoneGadgetContextMenu",
			"openApp('sip-phone')",
			"window.SipPhoneGadget = {",
		},
		"js/desktop/bundles/main.bundle.js": {
			"window.SipPhoneGadget = {",
		},
		"js/desktop/core/desktop-foundation.js": {
			"'phone_gadget.enabled': 'false'",
			"window.SipPhoneGadget.sync()",
		},
		"js/desktop/core/sdk-events-bootstrap.js": {
			"window.SipPhoneGadget.init()",
		},
		"js/desktop/apps/settings.js": {
			"settingToggle('phone_gadget.enabled', 'desktop.settings_phone_gadget', 'desktop.settings_phone_gadget_desc')",
		},
		"css/desktop-sip-phone-shell.css": {
			".vd-sip-phone-gadget {",
			".vd-sip-phone-gadget[data-always-on-top=\"true\"]",
		},
		"css/desktop-app-sip-phone.css": {
			".vd-sip-phone-gadget .sip-phone-statusbar",
		},
	}
	for path, markers := range checks {
		source := readDesktopAssetText(t, path)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing floating phone gadget contract %q", path, marker)
			}
		}
	}

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"desktop.settings_phone_gadget",
		"desktop.settings_phone_gadget_desc",
		"desktop.phone_gadget_open_window",
		"desktop.phone_gadget_always_on_top",
		"desktop.phone_gadget_remove",
	}
	for _, locale := range locales {
		values := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+locale+".json")), &values); err != nil {
			t.Fatalf("parse lang/desktop/%s.json: %v", locale, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("lang/desktop/%s.json is missing floating phone gadget key %q", locale, key)
			}
		}
	}
}

func TestDesktopSIPPhoneTranslationsAreComplete(t *testing.T) {
	t.Parallel()

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	var baseKeys []string
	var english map[string]string
	for _, locale := range locales {
		path := "lang/sip_phone/" + locale + ".json"
		values := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if len(values) < 64 {
			t.Fatalf("%s has only %d SIP phone translations", path, len(values))
		}
		configValues := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/config/"+locale+".json")), &configValues); err != nil {
			t.Fatalf("parse %s SIP config translations: %v", locale, err)
		}
		if strings.TrimSpace(configValues["config.sip.restart_required"]) == "" {
			t.Fatalf("lang/config/%s.json is missing the browser media restart notice", locale)
		}
		desktopValues := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+locale+".json")), &desktopValues); err != nil {
			t.Fatalf("parse %s desktop translations: %v", locale, err)
		}
		if strings.TrimSpace(desktopValues["desktop.app_sip_phone"]) == "" {
			t.Fatalf("lang/desktop/%s.json is missing the SIP phone app name", locale)
		}
		keys := make([]string, 0, len(values))
		for key, value := range values {
			if !strings.HasPrefix(key, "desktop.sip_phone_") {
				t.Fatalf("%s contains unrelated key %q", path, key)
			}
			if strings.TrimSpace(value) == "" {
				t.Fatalf("%s contains an empty translation for %q", path, key)
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if locale == "en" {
			baseKeys = keys
			english = values
			continue
		}
		if baseKeys == nil {
			continue
		}
		if !reflect.DeepEqual(keys, baseKeys) {
			t.Fatalf("%s does not match the English SIP phone key set", path)
		}
	}

	if english == nil {
		t.Fatal("English SIP phone translations were not loaded")
	}
	for _, locale := range locales {
		if locale == "en" {
			continue
		}
		values := map[string]string{}
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/sip_phone/"+locale+".json")), &values); err != nil {
			t.Fatalf("parse %s translations: %v", locale, err)
		}
		if reflect.DeepEqual(values, english) {
			t.Fatalf("%s must contain localized SIP phone text, not the English file", locale)
		}
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, baseKeys) {
			t.Fatalf("%s does not match the English SIP phone key set", locale)
		}
	}
}
