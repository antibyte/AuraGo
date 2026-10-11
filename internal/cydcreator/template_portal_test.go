package cydcreator

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// portalScript returns the JavaScript of the setup portal's head script (the
// raw string R"JS(...)JS" of kPortalScript in cyd_portal.cpp) and the C text
// in front of it.
func portalScript(t *testing.T) (prefix, script string) {
	t.Helper()
	src, err := fs.ReadFile(TemplateFS(), "lib/cyd_template/src/cyd_portal.cpp")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	decl := strings.Index(text, "const char kPortalScript[] =")
	start := strings.Index(text, `R"JS(`)
	end := strings.Index(text, `)JS"`)
	if decl < 0 || start < decl || end < start {
		t.Fatal(`cyd_portal.cpp: no kPortalScript with R"JS(...)JS"`)
	}
	return text[decl:start], text[start+len(`R"JS(`) : end]
}

func TestPortalScriptShape(t *testing.T) {
	prefix, script := portalScript(t)
	// The localized texts come from the device strings; strings.json keeps
	// them free of quotes and backslashes, so JS string literals can hold them.
	compact := strings.Join(strings.Fields(prefix), " ")
	want := `"<script>var cydPwMsg=\"" CYD_S_WIFI_PASSWORD_LENGTH "\",cydScanMsg=\"" CYD_S_WIFI_SCANNING "\";"`
	if !strings.Contains(compact, want) {
		t.Errorf("text prefix changed: %q", prefix)
	}
	// WiFiManager keeps a pointer to the head element and sends it with every
	// page: keep it small. 1.75 KiB holds the password check and the scan
	// overlay with its timeout, tap-to-dismiss and status role (the overlay's
	// look lives in the theme CSS); 2 KiB is the hard ceiling.
	if len(script) > 1792 {
		t.Errorf("portal script is %d bytes, keep it below 1.75 KiB", len(script))
	}
	for _, bad := range []string{"</", `\`, "'"} {
		if strings.Contains(script, bad) {
			t.Errorf("portal script contains %q (breaks the C string or the page)", bad)
		}
	}
}

// The browser check is manual (phone); this runs the script against a minimal
// fake DOM shaped like WiFiManager 2.0.17's pages: list items <a data-ssid>
// followed by the signal icon (class "l" when locked), form fields #s and #p,
// and the forms that go to /wifi (menu "configure", "refresh").
const portalScriptHarness = `
const vm = require("vm");
const fs = require("fs");
const code = 'var cydPwMsg="MSG",cydScanMsg="SCAN";' + fs.readFileSync(process.argv[2], "utf8");
let listener = null, capture = false, pageshow = null;
function makeEl(attrs, cls) {
  return { attrs: attrs || {}, value: "", focused: false, disabled: false, id: (attrs && attrs.id) || "",
    textContent: "", className: "", style: {}, children: [],
    classList: { contains: c => (cls || []).includes(c) },
    getAttribute(n) { return n in this.attrs ? this.attrs[n] : null; }, setAttribute(n, v) { this.attrs[n] = String(v); },
    focus() { this.focused = true; },
    appendChild(c) { this.children.push(c); if (c.id) document.elements[c.id] = c; return c; } };
}
global.window = { addEventListener(type, fn) { if (type === "pageshow") pageshow = fn; } };
// Fake timers: the overlay's safety timeout runs only when the test says so.
const timers = [];
global.setTimeout = (fn, ms) => { timers.push({ fn, ms }); return timers.length; };
global.clearTimeout = id => { if (id) timers[id - 1].fn = null; };
const pending = () => timers.filter(t => t.fn);
global.document = {
  items: [], elements: {},
  addEventListener(type, fn, c) { if (type === "submit") { listener = fn; capture = c === true; } },
  querySelectorAll(sel) { if (sel !== "[data-ssid]") throw new Error("selector " + sel); return this.items; },
  getElementById(id) { return this.elements[id] || null; },
  createElement() { return makeEl(); },
};
document.body = makeEl();
vm.runInThisContext(code);
if (!listener || !capture || !pageshow) throw new Error("listeners missing");
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
const uml = String.fromCharCode(0xe4), euro = String.fromCharCode(0x20ac), emoji = String.fromCodePoint(0x1f600);
const hi = String.fromCharCode(0xd83d), lo = String.fromCharCode(0xde00);  // unpaired: U+FFFD, 3 bytes
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
  // UTF-8 bytes as the browser sends them (the driver counts the same).
  ["Home", uml.repeat(60), [["Home", true]], true],
  ["Home", uml + uml + "abcde", [["Home", true]], false],
  ["Home", uml.repeat(31) + "a", [["Home", true]], false],
  ["Home", uml.repeat(32), [["Home", true]], true],
  ["Home", euro.repeat(21), [["Home", true]], false],
  ["Home", euro.repeat(22), [["Home", true]], true],
  ["Home", emoji + emoji, [["Home", true]], false],
  ["Home", emoji + "abc", [["Home", true]], true],
  ["Home", emoji.repeat(16), [["Home", true]], true],
  ["Home", emoji.repeat(15) + "abc", [["Home", true]], false],
  ["Home", hi + "abcd", [["Home", true]], true],
  ["Home", hi + "abcde", [["Home", true]], false],
  ["Home", hi + hi + "ab", [["Home", true]], false],
  ["Home", lo.repeat(21), [["Home", true]], false],
  ["Home", lo.repeat(22), [["Home", true]], true],
];
function passwordCases(label) {
  for (const [ssid, pw, list, want] of cases) {
    const { s, p, form } = page(list);
    s.value = ssid; p.value = pw;
    const got = submit(form);
    if (got !== want) throw new Error(label + " " + JSON.stringify([ssid, pw.length, "blocked", got, "want", want]));
    const msg = document.getElementById("cydpe");
    if (want && !(msg && msg.textContent === "MSG" && msg.className === "msg D" && p.focused)) throw new Error("no message for " + ssid);
    if (!want && msg) throw new Error("message for an accepted password");
  }
}
passwordCases("TextEncoder");
const encoder = global.TextEncoder;
global.TextEncoder = undefined;  // the fallback count must agree
passwordCases("fallback");
global.TextEncoder = encoder;
// A disabled field (WiFiManager disables it for an open network) sends
// nothing: its leftover value must not block a typed SSID.
{
  const { s, p, form } = page([["Home", true], ["Cafe", false]]);
  s.value = "Typed"; p.value = "secret"; p.disabled = true;
  if (submit(form)) throw new Error("blocked by a disabled field");
}
// A second rejection reuses the message; other forms are left alone.
{
  const { s, p, form } = page([["Home", true]]);
  s.value = "Home"; p.value = "";
  submit(form); submit(form);
  if (p.parentNode.inserted !== 1) throw new Error("message inserted twice");
  if (submit({ querySelector: () => null })) throw new Error("blocked a form without #s/#p");
}
// Forms going to /wifi show the scanning overlay at once and never block;
// the back-forward cache restore hides it; other forms do not show it.
const action = a => ({ getAttribute: n => (n === "action" ? a : null), querySelector: () => null });
document.elements = {};
for (const a of ["/wifi", "/wifi?refresh=1"]) {
  if (submit(action(a))) throw new Error("blocked " + a);
  const o = document.getElementById("cydsc");
  if (!o || o.style.display !== "" || o.children.length !== 2 || o.children[1].textContent !== "SCAN") throw new Error("no overlay for " + a);
  if (o.getAttribute("role") !== "status") throw new Error("overlay without role status");
  pageshow();
  if (o.style.display !== "none") throw new Error("overlay stays after pageshow");
  if (pending().length) throw new Error("pageshow left the safety timer running");
}
if (document.body.children.length !== 1) throw new Error("overlay created twice");
// A stopped load: the overlay goes after 30 s; a tap hides it at once.
{
  submit(action("/wifi"));
  const o = document.getElementById("cydsc"), t = pending();
  if (t.length !== 1 || t[0].ms !== 30000) throw new Error("no 30 s safety timer");
  t[0].fn();
  if (o.style.display !== "none") throw new Error("overlay stays after the timeout");
  submit(action("/wifi"));
  o.onclick();
  if (o.style.display !== "none" || pending().length) throw new Error("tap does not dismiss the overlay");
}
for (const a of ["wifisave", "/wifisave", "/0wifi", "/exit"]) {
  if (submit(action(a))) throw new Error("blocked " + a);
  if (document.getElementById("cydsc").style.display !== "none") throw new Error("overlay for " + a);
}
if (!(cydPwOk("", 0) && !cydPwOk("", 1) && cydPwOk("", 2) && !cydPwOk("1234567", 2))) throw new Error("cydPwOk");
console.log("ok");
`

func TestPortalScriptBehaviour(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required to run the portal script")
	}
	_, script := portalScript(t)
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "portal.js")
	harnessPath := filepath.Join(dir, "harness.cjs")
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(harnessPath, []byte(portalScriptHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harnessPath, scriptPath).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("portal script: %v\n%s", err, out)
	}
}
