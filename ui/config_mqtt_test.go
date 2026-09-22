package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMQTTConfigModuleUsesTypedControlsAndPreservesSecrets(t *testing.T) {
	t.Parallel()
	source := string(mustReadUIFile(t, "cfg/mqtt.js"))
	for _, marker := range []string{
		`data-type="array" data-path="mqtt.topics"`,
		`data-type="number" data-path="mqtt.qos"`,
		`data-type="number" data-path="mqtt.availability.qos"`,
		"mqtt-tls-transport-message",
		"config.mqtt.tls_plaintext_error",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("mqtt.js missing %q", marker)
		}
	}
	if strings.Contains(source, "input.value.trim()") {
		t.Fatal("MQTT password must preserve leading and trailing whitespace")
	}
	for _, lang := range []string{"en", "de", "fr", "es", "it", "ja", "zh", "nl", "pl", "cs", "da", "el", "hi", "no", "pt", "sv"} {
		values := mustReadJSONMap(t, "lang/config/mqtt/"+lang+".json")
		if strings.TrimSpace(values["config.mqtt.tls_plaintext_error"]) == "" {
			t.Fatalf("lang/config/mqtt/%s.json missing TLS transport message", lang)
		}
		if _, err := json.Marshal(values); err != nil {
			t.Fatalf("lang/config/mqtt/%s.json is not JSON: %v", lang, err)
		}
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
	page.MustEval(`async () => {
        const originalFetch = window.fetch;
        window.__mqttPasswordBody = '';
        window.fetch = async (input, init) => {
            if (String(input).endsWith('/api/vault/secrets') && init && init.method === 'POST') {
                window.__mqttPasswordBody = init.body;
                return new Response(JSON.stringify({status:'ok'}), {status:200, headers:{'Content-Type':'application/json'}});
            }
            return originalFetch(input, init);
        };
        const input = document.getElementById('mqtt-password');
        input.value = '  password with spaces  ';
        mqttSavePassword();
        await new Promise(resolve => setTimeout(resolve, 0));
    }`)
	if got := page.MustEval(`() => JSON.parse(window.__mqttPasswordBody).value`).String(); got != `  password with spaces  ` {
		t.Fatalf("password whitespace was changed: %q", got)
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
