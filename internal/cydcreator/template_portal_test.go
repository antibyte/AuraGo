package cydcreator

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// portalPasswordScript returns the JavaScript of the setup portal's password
// check (the raw string R"JS(...)JS" in cyd_portal.cpp) and the C text in
// front of it.
func portalPasswordScript(t *testing.T) (prefix, script string) {
	t.Helper()
	src, err := fs.ReadFile(TemplateFS(), "lib/cyd_template/src/cyd_portal.cpp")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	start := strings.Index(text, `R"JS(`)
	end := strings.Index(text, `)JS"`)
	if start < 0 || end < start {
		t.Fatal(`cyd_portal.cpp: no R"JS(...)JS" password check`)
	}
	lineStart := strings.LastIndex(text[:start], "\n") + 1
	return text[lineStart:start], text[start+len(`R"JS(`) : end]
}

func TestPortalPasswordScriptShape(t *testing.T) {
	prefix, script := portalPasswordScript(t)
	// The localized message comes from the device strings; strings.json keeps
	// them free of quotes and backslashes, so a JS string literal can hold it.
	if !strings.Contains(prefix, `"<script>var cydPwMsg=\"" CYD_S_WIFI_PASSWORD_LENGTH "\";"`) {
		t.Errorf("message prefix changed: %q", prefix)
	}
	// WiFiManager keeps a pointer to the head element and sends it with every
	// page: keep it small.
	if len(script) > 1024 {
		t.Errorf("password check is %d bytes, keep it below 1 KiB", len(script))
	}
	for _, bad := range []string{"</", `\`, "'"} {
		if strings.Contains(script, bad) {
			t.Errorf("password check contains %q (breaks the C string or the page)", bad)
		}
	}
}

// The browser check is manual (phone); this runs the script against a minimal
// fake DOM shaped like WiFiManager 2.0.17's /wifi page: list items
// <a data-ssid> followed by the signal icon, class "l" when locked; form
// fields #s and #p.
const portalScriptHarness = `
const vm = require("vm");
const fs = require("fs");
const code = 'var cydPwMsg="MSG";' + fs.readFileSync(process.argv[2], "utf8");
let listener = null, capture = false;
function makeEl(attrs, cls) {
  return { attrs: attrs || {}, value: "", focused: false, id: (attrs && attrs.id) || "", textContent: "", className: "",
    classList: { contains: c => (cls || []).includes(c) },
    getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; }, focus() { this.focused = true; } };
}
global.document = {
  items: [], elements: {},
  addEventListener(type, fn, c) { if (type === "submit") { listener = fn; capture = c === true; } },
  querySelectorAll(sel) { if (sel !== "[data-ssid]") throw new Error("selector " + sel); return this.items; },
  getElementById(id) { return this.elements[id] || null; },
  createElement() { return makeEl(); },
};
vm.runInThisContext(code);
if (!listener || !capture) throw new Error("no capturing submit listener");
function page(list) {
  document.items = list.map(([ssid, locked]) => {
    const a = makeEl({ "data-ssid": ssid });
    a.nextElementSibling = makeEl({}, locked ? ["q", "l"] : ["q"]);
    return a;
  });
  document.elements = {};
  const s = makeEl({ id: "s" }), p = makeEl({ id: "p" });
  p.parentNode = { inserted: 0, insertBefore(n, ref) { if (ref !== p.nextSibling) throw new Error("not after #p");
    this.inserted++; document.elements[n.id] = n; } };
  const form = { querySelector: sel => (sel === "#s" ? s : sel === "#p" ? p : null) };
  return { s, p, form };
}
function submit(form) { let blocked = false; listener({ target: form, preventDefault() { blocked = true; } }); return blocked; }
const hex64 = "0123456789abcdefABCDEF0123456789abcdefABCDEF0123456789abcdef0123";
const uml = String.fromCharCode(0xe4), emoji = String.fromCodePoint(0x1f600);
const cases = [
  // ssid, password, listed networks, blocked
  ["Home", "", [["Home", true]], true],
  ["Home", "1234567", [["Home", true]], true],
  ["Home", "12345678", [["Home", true]], false],
  ["Home", "x".repeat(63), [["Home", true]], false],
  ["Home", hex64, [["Home", true]], false],
  ["Home", hex64.slice(0, 63) + "g", [["Home", true]], true],
  ["Cafe", "", [["Home", true], ["Cafe", false]], false],
  ["Hidden", "", [["Home", true]], false],
  ["Hidden", "abc", [["Home", true]], true],
  ["Hidden", "abcdefgh", [["Home", true]], false],
  // UTF-8 bytes, as the driver counts them: 60 umlauts are 120 bytes, 7
  // characters with two umlauts are 9 bytes, an emoji is 4 bytes.
  ["Home", uml.repeat(60), [["Home", true]], true],
  ["Home", uml + uml + "abcde", [["Home", true]], false],
  ["Home", emoji + emoji, [["Home", true]], false],
  ["Home", emoji + "abc", [["Home", true]], true],
  ["Home", emoji.repeat(16), [["Home", true]], true],
  ["Home", emoji.repeat(15) + "abc", [["Home", true]], false],
];
for (const [ssid, pw, list, want] of cases) {
  const { s, p, form } = page(list);
  s.value = ssid; p.value = pw;
  const got = submit(form);
  if (got !== want) throw new Error(JSON.stringify([ssid, pw.length, "blocked", got, "want", want]));
  const msg = document.getElementById("cydpe");
  if (want && !(msg && msg.textContent === "MSG" && msg.className === "msg D" && p.focused)) throw new Error("no message for " + ssid);
  if (!want && msg) throw new Error("message for an accepted password");
}
// A disabled field (WiFiManager disables it for an open network) sends
// nothing: its leftover value must not block a typed SSID.
{
  const { s, p, form } = page([["Home", true], ["Cafe", false]]);
  s.value = "Typed"; p.value = "secret"; p.disabled = true;
  if (submit(form)) throw new Error("blocked by a disabled field");
}
// A second rejection reuses the message; other forms are left alone.
const { s, p, form } = page([["Home", true]]);
s.value = "Home"; p.value = "";
submit(form); submit(form);
if (p.parentNode.inserted !== 1) throw new Error("message inserted twice");
if (submit({ querySelector: () => null })) throw new Error("blocked a form without #s/#p");
if (!(cydPwOk("", 0) && !cydPwOk("", 1) && cydPwOk("", 2) && !cydPwOk("1234567", 2))) throw new Error("cydPwOk");
console.log("ok");
`

func TestPortalPasswordScriptBlocksBadPasswords(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required to run the portal password check")
	}
	_, script := portalPasswordScript(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "check.js")
	harnessPath := filepath.Join(dir, "harness.cjs")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(harnessPath, []byte(portalScriptHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harnessPath, scriptPath).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("portal password check: %v\n%s", err, out)
	}
}
