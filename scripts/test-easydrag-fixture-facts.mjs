// W1c Part A: the shared browser fixture answers GET {flow} and the save with "tested", "changes" and
// "next_run", controlled by edFixture.state.tested / .changes / .nextRun (the names Parts B and D write
// against). The defaults are neutral: tested false, no changes, no next run. The fixture is loaded in a vm
// with the few browser globals it touches.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = fs.readFileSync(path.join(root, 'ui', 'testdata', 'easydrag-fixture.js'), 'utf8');

const window = { fixtureReady: Promise.resolve(), fetch: async () => new Response('{}', { status: 404 }), I18N: {} };
window.window = window;
const sandbox = {
    window, Response, Headers, URL, JSON, Date, Map, Set, Promise, setTimeout, clearTimeout, console,
    location: { origin: 'http://fixture.test' },
    document: { dispatchEvent() { return true; }, addEventListener() {} },
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init && init.detail; } },
    EventSource: class {}
};
vm.createContext(sandbox);
vm.runInContext(source, sandbox, { filename: 'easydrag-fixture.js' });

const fx = window.edFixture;
assert.ok(fx && fx.state, 'edFixture.state exists');
const call = async (method, url, body) => {
    const res = await window.fetch(url, { method, body: body ? JSON.stringify(body) : undefined });
    return { status: res.status, body: await res.json() };
};
const flowPath = '/api/desktop/flows/' + fx.seededID;
const has = (body, key) => Object.prototype.hasOwnProperty.call(body, key);

// Neutral defaults: the answers carry tested:false and nothing else of the three.
assert.equal(fx.state.tested, false);
assert.equal(fx.state.changes, null);
assert.equal(fx.state.nextRun, null);
let got = await call('GET', flowPath);
assert.equal(got.status, 200);
assert.equal(got.body.tested, false);
assert.ok(!has(got.body, 'changes') && !has(got.body, 'next_run'), 'by default there is no changes and no next_run');
const doc = JSON.parse(JSON.stringify(fx.flow().rec.draft));
let saved = await call('PUT', flowPath, { doc, base_revision: fx.flow().rec.draft_revision });
assert.equal(saved.status, 200);
assert.equal(saved.body.tested, false);
assert.ok(!has(saved.body, 'changes') && !has(saved.body, 'next_run'), 'the default save answer has neither');

// The three names control the answer, set explicitly.
fx.state.tested = true;
fx.state.changes = { steps: 5, connections: 0 };
fx.state.nextRun = { at: '2031-03-03T07:00:00+01:00', node: 'n_start' };
got = await call('GET', flowPath);
assert.equal(got.body.tested, true);
assert.deepEqual(got.body.changes, { steps: 5, connections: 0 });
assert.deepEqual(got.body.next_run, { at: '2031-03-03T07:00:00+01:00', node: 'n_start' });
fx.state.changes = null;
fx.state.nextRun = null;
got = await call('GET', flowPath);
assert.equal(got.body.tested, true);
assert.ok(!has(got.body, 'changes') && !has(got.body, 'next_run'), 'null leaves the fields out');

// The save answer carries tested and changes (not next_run), and the fixture does not reset tested by itself.
fx.state.changes = { steps: 1, connections: 2 };
fx.state.nextRun = { at: '2031-03-03T07:00:00+01:00', node: 'n_start' };
saved = await call('PUT', flowPath, { doc, base_revision: fx.flow().rec.draft_revision });
assert.equal(saved.status, 200);
assert.equal(saved.body.tested, true);
assert.deepEqual(saved.body.changes, { steps: 1, connections: 2 });
assert.ok(!has(saved.body, 'next_run'), 'the save answer has no next_run');

// Like the server, a flow without a live version has no changes.
const draftOnly = fx.addDraft('Entwurf');
got = await call('GET', '/api/desktop/flows/' + draftOnly);
assert.ok(!has(got.body, 'changes'), 'a never published flow has no changes');
assert.equal(got.body.tested, true);

console.log('PASS fixture facts (tested, changes, next_run)');
