// B.5: the client credential rule against the same vectors as the Go rule
// (internal/flows/testdata/credential_cases.json), and the masking sites (emergency copy, clipboard).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const store = new Map();
const box = vm.createContext({
    window: {}, console, Intl, Date, Set, Map, crypto: webcrypto, navigator: { platform: 'Linux' },
    localStorage: { get length() { return store.size; }, key: i => Array.from(store.keys())[i] ?? null, getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } },
    setTimeout: () => 1, clearTimeout: () => {}
});
for (const name of ['easydrag-core.js', 'easydrag-format.js', 'easydrag-template.js', 'easydrag-credentials.js', 'easydrag-saver.js']) {
    const file = new URL(name, apps);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const ED = box.window.EasyDrag;
const plain = value => JSON.parse(JSON.stringify(value));

const cases = JSON.parse(fs.readFileSync(new URL('../internal/flows/testdata/credential_cases.json', import.meta.url), 'utf8'));
assert.ok(cases.length >= 18, 'the shared vectors shrank');
for (const c of cases) {
    const node = Object.assign({}, c.node, { params: plain(c.node.params) });
    const found = plain(ED.credentials.findings(node)).map(f => ({ param: f.param, location: f.location, path: f.path, movable: f.movable }));
    assert.deepEqual(found, c.findings, c.name + ': findings');
    assert.ok(ED.credentials.findings(node).every(f => f.value), c.name + ': every finding holds its value');
    assert.deepEqual(plain(ED.credentials.maskParams(node.type, node.params)), c.masked_params, c.name + ': masked params');
    assert.deepEqual(plain(node.params), plain(c.node.params), c.name + ': the input was changed');
}

// remove clears exactly one finding (the warning's "remove value"), mask handles a whole document.
const call = { id: 'n1', type: 'http.request', params: { url: 'https://u:pw1234@api.example.com/v1?api_key=abc123&q=1', headers: { Authorization: 'Bearer abc123def456' } } };
const urlHit = ED.credentials.findings(call).find(f => f.location === 'url_query');
assert.equal(plain(ED.credentials.remove(call, urlHit)).url, 'https://u:pw1234@api.example.com/v1?api_key=&q=1');
const userHit = ED.credentials.findings(call).find(f => f.location === 'url_userinfo');
assert.equal(plain(ED.credentials.remove(call, userHit)).url, 'https://api.example.com/v1?api_key=abc123&q=1');
const headerHit = ED.credentials.findings(call).find(f => f.location === 'header');
assert.equal(plain(ED.credentials.remove(call, headerHit)).headers.Authorization, '');
const masked = plain(ED.credentials.mask({ name: 'x', nodes: [call], edges: [] }));
assert.ok(!JSON.stringify(masked).includes('abc123') && !JSON.stringify(masked).includes('pw1234'));
assert.equal(ED.credentials.hasFindings({ nodes: [call] }), true);
assert.equal(ED.credentials.hasFindings({ nodes: [{ id: 'n2', type: 'logic.set', params: {} }] }), false);

// The emergency copy never holds a token, says so, and offers the flag to the restore dialog.
let version = 1;
const doc = { schema: 1, name: 'x', nodes: [Object.assign({ key: 'call', label: 'Call', position: { x: 0, y: 0 }, settings: {} }, plain(call))], edges: [] };
const model = { get version() { return version; }, toJSON: () => plain(doc), replaceDoc() {} };
const saver = ED.saver.create({ api: { save: () => new Promise(() => {}) }, flowId: 'f1', model, revision: 1, onState() {}, onSaved() {}, onInvalid() {}, onConflict: () => new Promise(() => {}) });
version++;
saver.dispose(); // writes the unsaved draft
const copy = JSON.parse(store.get('aurago.easydrag.draft.f1'));
for (const secret of ['abc123def456', 'pw1234', 'api_key=abc123']) assert.ok(!JSON.stringify(copy).includes(secret), 'the emergency copy holds ' + secret);
assert.equal(copy.masked, true);
assert.equal(ED.saver.emergencyCopy('f1', 1).masked, true);

// The clipboard fragment is masked where it is written (the interact module needs a DOM, so the
// call is pinned in the source).
const interact = fs.readFileSync(new URL('easydrag-interact.js', apps), 'utf8');
assert.ok(interact.includes('const masked = ED.credentials.mask(frag)'), 'the clipboard fragment is not masked');
assert.ok(interact.includes('const text = JSON.stringify(masked)') && interact.includes('writeText(text)') && interact.includes('frag: masked'),
    'the system clipboard text or the storage write does not come from the masked fragment');
assert.ok(!/JSON\.stringify\(frag\)/.test(interact), 'the raw fragment is serialised');
// What the masked fragment serialises to holds no literal.
const maskedFrag = ED.credentials.mask({ easydrag: 1, nodes: [call], edges: [] });
for (const secret of ['abc123def456', 'pw1234', 'api_key=abc123']) assert.ok(!JSON.stringify(maskedFrag).includes(secret), 'the clipboard text holds ' + secret);
console.log('PASS credential rule and masking');
