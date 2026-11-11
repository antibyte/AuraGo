// B.4: the credential move UI (name proposal, the move itself against a stub server, history scrub,
// the purge offer after a publish) and ED.model.scrubHistory.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const box = vm.createContext({ window: {}, console, Intl, Date, Set, Map, URL, crypto: webcrypto });
for (const name of ['easydrag-template.js', 'easydrag-credentials.js', 'easydrag-model.js', 'easydrag-credentials-move.js']) {
    const file = new URL(name, apps);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const ED = box.window.EasyDrag;
const plain = value => JSON.parse(JSON.stringify(value));
const TOKEN = 'tok-secret-1234567890';
const doc = () => ({ schema: 1, name: 'x', nodes: [{ id: 'n_aaaaaaaa', key: 'call', type: 'http.request', label: 'Call', position: { x: 0, y: 0 },
    params: { url: 'https://api.example.com/v1?api_key=' + TOKEN + '&q=1' }, settings: {} }], edges: [] });

// The proposal is built from the host and the key, cut to 40 characters.
const node = { id: 'n1', type: 'http.request', params: { url: 'https://api.example.com/v1?api_key=x' } };
assert.equal(ED.credentialsMove.proposeName(node, { location: 'url_query', path: 'api_key' }), 'api_example_com_api_key');
assert.equal(ED.credentialsMove.proposeName({ params: { url: '{{trigger.data.u}}' } }, { location: 'header', path: 'Authorization' }), 'authorization');
assert.equal(ED.credentialsMove.proposeName({ params: { url: 'https://' + 'a'.repeat(60) + '.example/x' } }, { location: 'url_userinfo', path: 'userinfo' }).length <= 40, true);

// scrubHistory: after it, undo and redo cannot bring the typed value back; a short value only
// matches a whole text.
const model = ED.model.create(plain(doc()));
model.setParam('n_aaaaaaaa', 'url', 'https://api.example.com/v1?q=1');
assert.equal(model.canUndo(), true);
assert.ok(model.scrubHistory(TOKEN) >= 1);
model.undo();
assert.ok(!JSON.stringify(model.toJSON()).includes(TOKEN), 'undo brought the token back');
assert.ok(JSON.stringify(model.toJSON()).includes('[redacted]'));
model.redo();
assert.ok(!JSON.stringify(model.toJSON()).includes(TOKEN));
assert.equal(model.scrubHistory('ab'), 0);

// The move against a stub server: the request carries no value, the model takes the returned
// params as one change, the history is clean, and only a publish after a move offers the purge.
const sent = [], notes = [], offers = [];
const m2 = ED.model.create(plain(doc()));
const served = doc(); served.nodes[0].params = { url: 'https://api.example.com/v1?q=1', auth_secret: 'my_key', auth_in: 'query', auth_query: 'api_key' };
m2.setParam('n_aaaaaaaa', 'q_test', 'x'); // some earlier history
const ed = {
    t: key => key, esc: s => s, model: m2, flow: { id: 'flow_x' }, issues: [], bus: { emit() {} }, readonly: false,
    ctx: { notify: n => notes.push(n), confirmDialog: async () => true },
    flushPendingEdits: () => true,
    saver: { revision: 1, flush: async () => true, beginExternal: () => ({}), releaseExternal() {}, recoverExternal: async () => false,
        acknowledgeExternal: (ticket, rev, apply) => { assert.equal(rev, 2); return apply() !== false; } },
    api: { moveCredential: async (id, body) => { sent.push(body); return { draft_revision: 2, doc: served, moved: ['n_aaaaaaaa'], not_movable: [], issues: [] }; } }
};
const mv = ED.credentialsMove.create(ed);
const finding = ED.credentials.findings(m2.node('n_aaaaaaaa'))[0];
assert.equal(finding.location, 'url_query');
assert.equal(await mv.move(finding, 'n_aaaaaaaa', 'my_key', false), true);
assert.ok(!JSON.stringify(sent).includes(TOKEN), 'the request carries the value');
assert.deepEqual(plain(sent[0]), { revision: 1, node_id: 'n_aaaaaaaa', location: 'url_query', path: 'api_key', secret_name: 'my_key', replace: false });
assert.equal(m2.node('n_aaaaaaaa').params.auth_secret, 'my_key');
for (let i = 0; i < 5 && m2.canUndo(); i++) { m2.undo(); assert.ok(!JSON.stringify(m2.toJSON()).includes(TOKEN), 'undo step ' + i + ' brought the token back'); }
assert.equal(notes.length, 1);
box.window.EasyDrag.credentialsPurge = { offer: id => offers.push(id) };
mv.afterPublish();
mv.afterPublish();
assert.deepEqual(offers, ['flow_x'], 'the purge is offered once, after the first publish that follows a move');
console.log('PASS credential move');
