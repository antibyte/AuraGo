// B.4: the credential move UI (name proposal, the move itself against a stub server, the history
// cleared after a move or a removal, the purge offer after a publish) and ED.model.clearHistory.
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

// clearHistory: undo and redo have nothing left, and the model says so to its listeners.
const model = ED.model.create(plain(doc()));
let heard = 0;
model.on(() => { heard++; });
model.setParam('n_aaaaaaaa', 'url', 'https://api.example.com/v1?q=1');
assert.equal(model.canUndo(), true);
const beforeClear = heard, versionBefore = model.version;
model.clearHistory();
assert.equal(heard, beforeClear + 1);
assert.equal(model.version, versionBefore, "clearHistory must not count as an edit (the saver compares versions)");
assert.equal(model.canUndo(), false);
assert.equal(model.canRedo(), false);
assert.equal(model.undo(), false);
assert.equal(model.scrubHistory, undefined, 'scrubHistory is gone');

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
// The move clears undo and redo: no snapshot is left that could hold the value, undo does nothing.
assert.equal(m2.canUndo(), false);
assert.equal(m2.canRedo(), false);
const afterMove = JSON.stringify(m2.toJSON());
assert.equal(m2.undo(), false);
assert.equal(JSON.stringify(m2.toJSON()), afterMove, 'undo changed the document after a move');
assert.ok(!afterMove.includes(TOKEN));
assert.equal(notes.length, 1);
assert.ok(notes[0].message.includes('easydrag.ui.credential_undo_reset'), 'the toast does not say that undo was reset');

// "Remove value" (not exported; it needs the DOM) clears the history as well and says so.
const moveSource = fs.readFileSync(new URL('easydrag-credentials-move.js', apps), 'utf8');
const removeBody = moveSource.slice(moveSource.indexOf('function removeValue'), moveSource.indexOf('function decorate'));
assert.ok(removeBody.includes('ed.model.clearHistory()') && removeBody.includes('credential_undo_reset'), 'removeValue does not clear the history');
box.window.EasyDrag.credentialsPurge = { offer: id => offers.push(id) };
mv.afterPublish();
mv.afterPublish();
assert.deepEqual(offers, ['flow_x'], 'the purge is offered once, after the first publish that follows a move');
console.log('PASS credential move');
