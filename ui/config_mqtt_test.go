package ui

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMQTTConfigModuleUsesTypedControlsAndPreservesSecrets(t *testing.T) {
	t.Parallel()
	source := string(mustReadUIFile(t, "cfg/mqtt.js"))
	for _, marker := range []string{
		`data-type="array" data-path="mqtt.topics"`,
		`data-type="array" data-path="mqtt.relay_topics"`,
		`data-path="mqtt.relay_grants" data-type="json"`,
		`data-path="mqtt.buffer.max_bytes"`,
		`data-mqtt-grant-action`,
		`data-mqtt-grant-target`,
		`res.relay_pause_reason`,
		`m.payload_binary`,
		`data-type="number" data-path="mqtt.qos"`,
		`data-type="number" data-path="mqtt.availability.qos"`,
		"mqtt-tls-transport-message",
		"config.mqtt.tls_plaintext_error",
		`data-path="mqtt.allow_unauthenticated_relay"`,
		"config.mqtt.allow_unauthenticated_relay_label",
		"help.mqtt.allow_unauthenticated_relay",
		`data-path="mqtt.allow_untrusted_transport"`,
		"config.mqtt.allow_untrusted_transport_label",
		"help.mqtt.allow_untrusted_transport",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mqtt.js missing %q", marker)
		}
	}
	if strings.Contains(source, "input.value.trim()") {
		t.Fatal("MQTT password must preserve leading and trailing whitespace")
	}
	englishConfig := mustReadJSONMap(t, "lang/config/mqtt/en.json")
	englishHelp := mustReadJSONMap(t, "lang/help/en.json")
	for _, lang := range []string{"en", "de", "fr", "es", "it", "ja", "zh", "nl", "pl", "cs", "da", "el", "hi", "no", "pt", "sv"} {
		values := mustReadJSONMap(t, "lang/config/mqtt/"+lang+".json")
		for _, key := range []string{"relay_topics_label", "relay_topics_help", "relay_grants_label", "relay_grants_help", "grant_add", "grant_remove", "grant_action", "grant_target", "action_publish", "action_ha_control", "relay_limits_help", "buffer_max_bytes_label", "buffer_max_bytes_help", "pause_credentials", "pause_transport", "pause_topics", "pause_grants", "relay_ready", "binary_payload", "queue_label", "dropped_label", "quarantined_label"} {
			full := "config.mqtt." + key
			if strings.TrimSpace(values[full]) == "" {
				t.Fatalf("%s missing %s", lang, full)
			}
			if lang != "en" && strings.HasSuffix(key, "_help") && values[full] == englishConfig[full] {
				t.Fatalf("%s copies English help %s", lang, full)
			}
		}
		if strings.TrimSpace(values["config.mqtt.tls_plaintext_error"]) == "" {
			t.Fatalf("lang/config/mqtt/%s.json missing TLS transport message", lang)
		}
		if _, err := json.Marshal(values); err != nil {
			t.Fatalf("lang/config/mqtt/%s.json is not JSON: %v", lang, err)
		}
		help := mustReadJSONMap(t, "lang/help/"+lang+".json")
		for _, entry := range []struct {
			file string
			key  string
			text string
		}{
			{"lang/config/mqtt/" + lang + ".json", "config.mqtt.allow_unauthenticated_relay_label", values["config.mqtt.allow_unauthenticated_relay_label"]},
			{"lang/help/" + lang + ".json", "help.mqtt.allow_unauthenticated_relay", help["help.mqtt.allow_unauthenticated_relay"]},
			{"lang/config/mqtt/" + lang + ".json", "config.mqtt.allow_untrusted_transport_label", values["config.mqtt.allow_untrusted_transport_label"]},
			{"lang/help/" + lang + ".json", "help.mqtt.allow_untrusted_transport", help["help.mqtt.allow_untrusted_transport"]},
		} {
			file, key, text := entry.file, entry.key, entry.text
			if strings.TrimSpace(text) == "" || text == key {
				t.Fatalf("%s missing %s", file, key)
			}
			english := englishConfig[key]
			if english == "" {
				english = englishHelp[key]
			}
			if lang != "en" && text == english {
				t.Fatalf("%s copies the English %s instead of translating it", file, key)
			}
		}
	}
}

func TestMQTTConfigRelayPermissionsSaveReloadBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "de", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => { await selectSection('mqtt', {scrollBehavior:'auto'}); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-path="mqtt.relay_topics"]')`)
	const want = `[{"tool":"mqtt_publish","operation":"publish","target":"mqtt:home/light/set"},{"tool":"home_assistant","operation":"call_service","target":"ha:light.desk/light.turn_on"}]`
	assertGrants := func(got string) {
		t.Helper()
		var actual, expected []map[string]string
		if json.Unmarshal([]byte(got), &actual) != nil || json.Unmarshal([]byte(want), &expected) != nil || !reflect.DeepEqual(actual, expected) {
			t.Fatalf("relay permissions=%s want=%s", got, want)
		}
	}
	got := page.MustEval(`async () => {
        const topics = document.querySelector('[data-path="mqtt.relay_topics"]');
        topics.value = 'home/input, frigate/events';
        topics.dispatchEvent(new Event('input', {bubbles:true}));
        for (const [action, target] of [['mqtt_publish|publish','mqtt:home/light/set'],['home_assistant|call_service','ha:light.desk/light.turn_on']]) {
            mqttAddGrant();
            const row = Array.from(document.querySelectorAll('[data-mqtt-relay-grant]')).at(-1);
            const select = row.querySelector('[data-mqtt-grant-action]');
            select.value = action;
            select.dispatchEvent(new Event('change', {bubbles:true}));
            const input = row.querySelector('[data-mqtt-grant-target]');
            input.value = target;
            input.dispatchEvent(new Event('input', {bubbles:true}));
        }
        await saveConfig();
        const response = await fetch('/api/config');
        const saved = (await response.json()).mqtt;
        if (JSON.stringify(saved.relay_topics) !== '["home/input","frigate/events"]') return 'topics missing';
        return JSON.stringify(saved.relay_grants);
    }`).String()
	assertGrants(got)
	page.MustNavigate(page.MustInfo().URL).MustWaitLoad()
	waitForJSBool(t, page, `() => document.querySelectorAll('[data-mqtt-relay-grant]').length === 2`)
	assertGrants(page.MustEval(`() => document.getElementById('mqtt-relay-grants-value').value`).String())
	page.MustEval(`() => mqttRemoveGrant(document.querySelector('[data-mqtt-relay-grant] button'))`)
	if !page.MustEval(`() => AuraConfigState.get('mqtt.relay_grants').length === 1 && hasUnsavedConfigChanges()`).Bool() {
		t.Fatal("removing a grant did not become a pending configuration change")
	}
}

func TestMQTTConfigTopicsEditSaveReloadBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshFixtureOrigin(t, "en", false) + "/config#overview")
	defer page.MustClose()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustEval(`async () => { await selectSection('mqtt', {scrollBehavior:'auto'}); }`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-path="mqtt.topics"]')`)

	if !page.MustEval(`() => document.querySelector('[data-path="mqtt.topics"]').dataset.type === 'array'`).Bool() {
		t.Fatal("MQTT topics control is not typed as an array")
	}
	if !page.MustEval(`() => document.querySelector('[data-path="mqtt.qos"]').dataset.type === 'number'`).Bool() {
		t.Fatal("MQTT QoS control is not typed as a number")
	}
	setTopicsAndSave := func(value string) string {
		return page.MustEval(`async value => {
            const input = document.querySelector('[data-path="mqtt.topics"]');
            input.value = value;
            input.dispatchEvent(new Event('input', {bubbles:true}));
            await saveConfig();
            const response = await fetch('/api/config');
            return JSON.stringify((await response.json()).mqtt.topics);
        }`, value).String()
	}
	if got := setTopicsAndSave("home/#, sensors/+"); got != `["home/#","sensors/+"]` {
		t.Fatalf("saved topics after add = %s", got)
	}
	if got := setTopicsAndSave("home/#"); got != `["home/#"]` {
		t.Fatalf("saved topics after remove = %s", got)
	}
	if got := setTopicsAndSave(""); got != `[]` {
		t.Fatalf("saved topics after clear = %s", got)
	}
	if got := page.MustEval(`async () => {
        const qos = document.querySelector('[data-path="mqtt.qos"]');
        qos.value = '0';
        qos.dispatchEvent(new Event('change', {bubbles:true}));
        await saveConfig();
        const response = await fetch('/api/config');
        return JSON.stringify((await response.json()).mqtt.qos);
	}`).String(); got != `0` {
		t.Fatalf("saved QoS0 = %s", got)
	}
	if got := page.MustEval(`async () => {
        const toggle = document.querySelector('[data-path="mqtt.allow_unauthenticated_relay"]');
        if (!toggle) return 'missing';
        if (toggle.classList.contains('on')) return 'on before opt-in';
        const label = toggle.closest('.field-group').querySelector('.field-label').textContent;
        if (label.includes('config.mqtt.')) return 'untranslated label ' + label;
        toggle.click();
        await saveConfig();
        const response = await fetch('/api/config');
        return JSON.stringify((await response.json()).mqtt.allow_unauthenticated_relay);
    }`).String(); got != `true` {
		t.Fatalf("saved allow_unauthenticated_relay = %s", got)
	}
	page.MustEval(`async () => {
        configData.mqtt.availability.enabled = true;
        await renderMQTTSection();
    }`)
	waitForJSBool(t, page, `() => !!document.querySelector('[data-path="mqtt.availability.qos"]')`)
	if !page.MustEval(`() => document.querySelector('[data-path="mqtt.availability.qos"]').dataset.type === 'number'`).Bool() {
		t.Fatal("MQTT availability QoS control is not typed as a number")
	}
	if got := page.MustEval(`async () => {
        const qos = document.querySelector('[data-path="mqtt.availability.qos"]');
        qos.value = '0';
        qos.dispatchEvent(new Event('change', {bubbles:true}));
        await saveConfig();
        const response = await fetch('/api/config');
        return JSON.stringify((await response.json()).mqtt.availability.qos);
    }`).String(); got != `0` {
		t.Fatalf("saved availability QoS0 = %s", got)
	}
	// The broker password joins the save dock (the server writes it to the vault verbatim).
	page.MustEval(`() => {
        const input = document.getElementById('mqtt-password');
        input.value = '  password with spaces  ';
        input.dispatchEvent(new Event('input', {bubbles:true}));
    }`)
	if !page.MustEval(`() => document.getElementById('mqtt-password').dataset.path === 'mqtt.password' && AuraConfigState.get('mqtt.password') === '  password with spaces  ' && hasUnsavedConfigChanges() && !document.querySelector('[onclick*="mqttSavePassword"]')`).Bool() {
		t.Fatal("mqtt password did not join the save dock")
	}
	page.MustEval(`() => {
        const broker = document.querySelector('[data-path="mqtt.broker"]');
        broker.value = 'tcp://localhost:1883';
        broker.dispatchEvent(new Event('input', {bubbles:true}));
        const toggle = document.querySelector('[data-path="mqtt.tls.enabled"]');
        if (!toggle.classList.contains('on')) toggle.click();
    }`)
	if !page.MustEval(`() => !document.getElementById('mqtt-tls-transport-message').hidden`).Bool() {
		t.Fatal("TLS/plaintext warning did not appear immediately")
	}
	page.MustNavigate(page.MustInfo().URL).MustWaitLoad()
	waitForJSBool(t, page, `() => !!document.querySelector('[data-path="mqtt.topics"]')`)
	if got := page.MustEval(`() => JSON.stringify(AuraConfigState.get('mqtt.topics', {saved:true}))`).String(); got != `[]` {
		t.Fatalf("reloaded topics = %s", got)
	}
}
