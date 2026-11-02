package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The agent tool ecalc opens ECalc pre-filled through an open_app desktop event whose payload carries
// {ecalc: {page, values, choices}}. The core (sdk-events-bootstrap.js) rebuilds it from checked parts and hands it to
// openApp as launch.launch, which reaches the app as ctx.launch (new window) or ECalcApp.launch (window already open,
// window-shell-runtime.js). These tests are source markers plus a Node run of the sanitizer; the browser test
// covers the pre-filled page.

const (
	ecalcLaunchSanitizerStart = "    const ECALC_LAUNCH_PAGE = "
	ecalcLaunchSanitizerEnd   = "    async function handleDesktopEvent("
)

func ecalcLaunchMarkersInOrder(t *testing.T, name, source string, markers []string) {
	t.Helper()
	offset := 0
	for _, marker := range markers {
		index := strings.Index(source[offset:], marker)
		if index < 0 {
			t.Fatalf("%s: marker %q missing after offset %d", name, marker, offset)
		}
		offset += index + len(marker)
	}
}

func TestECalcLaunchBridgeSourceMarkers(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"js/desktop/core/sdk-events-bootstrap.js", "js/desktop/bundles/main.bundle.js"} {
		source := rawDesktopAssetText(t, file)
		ecalcLaunchMarkersInOrder(t, file, source, []string{
			"const ECALC_LAUNCH_PAGE = /^[a-z][a-z0-9_]{0,63}$/;",
			"const ECALC_LAUNCH_NAME = /^[a-z][a-z0-9_]{0,31}$/;",
			"function sanitizeECalcLaunchPairs(raw, valueLimit, valuePattern) {",
			"if (count >= 32) break;",
			"value.length > valueLimit",
			"function sanitizeECalcLaunch(raw) {",
			"values: sanitizeECalcLaunchPairs(raw.values, 256, null),",
			"choices: sanitizeECalcLaunchPairs(raw.choices, 32, ECALC_LAUNCH_NAME)",
			"async function handleDesktopEvent(event) {",
			"if (event.type === 'open_app' && event.payload && event.payload.app_id) {",
			"if (event.payload.app_id === 'ecalc') {",
			"const ecalcLaunch = sanitizeECalcLaunch(event.payload.ecalc);",
			"if (ecalcLaunch) launch.launch = ecalcLaunch;",
			"openApp(event.payload.app_id, Object.keys(launch).length ? launch : undefined);",
		})
	}
	// The launch context is one-shot: a restored session must never overwrite what the user typed since.
	session := rawDesktopAssetText(t, "js/desktop/core/session-runtime.js")
	if start := strings.Index(session, "const SESSION_CONTEXT_KEYS = ["); start < 0 || strings.Contains(session[start:start+strings.Index(session[start:], "]")], "launch") {
		t.Fatal("session-runtime.js must not persist a launch context")
	}
	// The second-open branch sits inside the existing-window branch of openApp, before it returns.
	for _, file := range []string{"js/desktop/core/window-shell-runtime.js", "js/desktop/bundles/main.bundle.js"} {
		shell := strings.ReplaceAll(rawDesktopAssetText(t, file), "\r\n", "\n") // a Windows checkout has CRLF
		ecalcLaunchMarkersInOrder(t, file, shell, []string{
			"if (appId === 'ecalc' && context && context.launch && window.ECalcApp && typeof window.ECalcApp.launch === 'function') window.ECalcApp.launch(existing.id, context.launch);",
		})
		existing := strings.Index(shell, "const existing = (context && context.forceNew) ? null : findExistingAppWindow(appId, context || {});")
		branch := strings.Index(shell, "window.ECalcApp.launch(existing.id, context.launch);")
		ret := strings.Index(shell[existing:], "\n            return;\n        }")
		if existing < 0 || branch < existing || branch > existing+ret {
			t.Fatalf("%s: the ECalc launch must run inside the existing-window branch of openApp", file)
		}
	}
}

func TestECalcLaunchSanitizerRejectsWhatItDoesNotKnow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for the ECalc launch sanitizer check")
	}
	source := rawDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	start := strings.Index(source, ecalcLaunchSanitizerStart)
	end := strings.Index(source, ecalcLaunchSanitizerEnd)
	if start < 0 || end < start {
		t.Fatal("the sanitizer block was not found in sdk-events-bootstrap.js")
	}
	dir := t.TempDir()
	snippet := filepath.Join(dir, "sanitizer.js")
	if err := os.WriteFile(snippet, []byte(source[start:end]), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := filepath.Join(dir, "check.js")
	script := `
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
// Same realm as this script, so that prototypes compare equal.
const { sanitizeECalcLaunch } = vm.runInThisContext(fs.readFileSync(process.argv[2], 'utf8') + '\n({ sanitizeECalcLaunch });');
const plain = value => JSON.parse(JSON.stringify(value));

// A good payload comes out unchanged, with the values in the order they were given.
const good = sanitizeECalcLaunch(JSON.parse('{"page":"voltage_divider","values":{"vin":"12 V","r1":"4k7"},"choices":{"wave":"triangle"}}'));
assert.deepEqual(plain(good), { page: 'voltage_divider', values: { vin: '12 V', r1: '4k7' }, choices: { wave: 'triangle' } });
assert.deepEqual(Object.keys(good.values), ['vin', 'r1']);
assert.deepEqual(plain(sanitizeECalcLaunch({ page: 'ohms_law' })), { page: 'ohms_law', values: {}, choices: {} });
assert.deepEqual(plain(sanitizeECalcLaunch({ page: 'ohms_law', values: [], choices: 'x' })), { page: 'ohms_law', values: {}, choices: {} });

// No page, no launch.
for (const bad of [null, undefined, 'x', 3, [], {}, { page: 7 }, { page: '' }, { page: 'Voltage' }, { page: '1abc' }, { page: '../x' }, { page: 'a-b' }, { page: 'a b' }, { page: '_x' }, { page: 'a'.repeat(65) }]) {
  assert.equal(sanitizeECalcLaunch(bad), null, JSON.stringify(bad));
}
assert.equal(sanitizeECalcLaunch({ page: 'a'.repeat(64) }).page.length, 64);

// Names: lower-case ids that start with a letter, at most 32 characters.
const names = sanitizeECalcLaunch(JSON.parse(JSON.stringify({ page: 'x', values: { vin: '1', Vin: '2', '1a': '3', 'a-b': '4', ['a'.repeat(32)]: '5', ['a'.repeat(33)]: '6', '': '7', ok_1: '8' } })));
assert.deepEqual(Object.keys(names.values), ['vin', 'a'.repeat(32), 'ok_1']);

// Values: strings of at most 256 characters; nothing else.
const vals = sanitizeECalcLaunch({ page: 'x', values: { a: 'x'.repeat(256), b: 'x'.repeat(257), c: 5, d: null, e: { f: 1 }, f: ['1'], g: true, h: '' } });
assert.deepEqual(Object.keys(vals.values), ['a', 'h']);

// Choices: the option is an id as well.
const ch = sanitizeECalcLaunch({ page: 'x', choices: { wave: 'triangle', mode: 'Fast', kind: 'a'.repeat(33), n: 4, ok: 'a'.repeat(32) } });
assert.deepEqual(plain(ch.choices), { wave: 'triangle', ok: 'a'.repeat(32) });

// At most 32 of each, the first ones.
const many = {};
for (let i = 0; i < 40; i++) many['v' + i] = 'o' + i;
const cut = sanitizeECalcLaunch({ page: 'x', values: many, choices: many });
assert.equal(Object.keys(cut.values).length, 32);
assert.equal(Object.keys(cut.values)[31], 'v31');
assert.equal(Object.keys(cut.choices).length, 32);

// A hostile key neither pollutes the prototype nor survives.
const hostile = sanitizeECalcLaunch(JSON.parse('{"page":"x","values":{"__proto__":"x","constructor":"y","vin":"1"},"choices":{"__proto__":"x"}}'));
assert.deepEqual(Object.keys(hostile.values), ['constructor', 'vin']);
assert.equal(Object.keys(hostile.choices).length, 0);
assert.equal(({}).vin, undefined);
assert.equal(Object.getPrototypeOf(hostile.values), Object.prototype);
console.log('ok');
`
	if err := os.WriteFile(runner, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, runner, snippet).CombinedOutput()
	if err != nil {
		t.Fatalf("node check failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("unexpected output: %s", out)
	}
}
