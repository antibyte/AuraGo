#!/usr/bin/env node
// Proposal replacement is one undo command and never overwrites another local generation.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

const box = vm.createContext({ window: {}, crypto: webcrypto, console, Intl, Date, Map, Set, JSON });
for (const name of ['easydrag-template.js', 'easydrag-model.js', 'easydrag-authoring.js']) {
    const file = new URL('../ui/js/desktop/apps/' + name, import.meta.url);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const ED = box.window.EasyDrag;
const original = { schema: 1, id: 'flow_original', name: 'Original', description: 'Keep this', settings: { concurrency: 'queue' }, nodes: [
    { id: 'n_old', key: 'start', label: 'Start', type: 'trigger.manual', position: { x: 0, y: 0 }, params: {}, settings: {} }
], edges: [] };
const proposal = { schema: 1, id: 'flow_untrusted', name: 'Proposal', settings: { concurrency: 'skip' }, nodes: [
    { id: 'n_new', key: 'start', label: 'Start', type: 'trigger.manual', position: { x: 80, y: 80 }, params: {}, settings: {} },
    { id: 'n_result', key: 'result', label: 'Result', type: 'logic.set', position: { x: 380, y: 80 }, params: { fields: [{ name: 'text', value: 'Hello' }] }, settings: {} }
], edges: [{ id: 'e_new', source: { node: 'n_new', port: 'out' }, target: { node: 'n_result', port: 'in' } }] };
const model = ED.model.create(original, { types: new Map() });
const before = JSON.stringify(model.toJSON());
assert.equal(ED.authoring.applyProposal(model, proposal, model.version, 'Apply'), true);
assert.equal(model.version, 1);
assert.equal(model.doc.id, 'flow_original');
assert.equal(model.doc.name, 'Proposal');
assert.equal(model.doc.nodes.length, 2);
assert.equal(model.doc.edges.length, 1);
assert.equal(model.undo(), true);
assert.equal(JSON.stringify(model.toJSON()), JSON.stringify(Object.assign(JSON.parse(before), { groups: [] })));
assert.equal(model.canUndo(), false);
assert.equal(model.redo(), true);
model.setLabel('n_new', 'A newer local edit');
const newer = JSON.stringify(model.toJSON());
assert.equal(ED.authoring.applyProposal(model, proposal, 1, 'Apply'), false);
assert.equal(JSON.stringify(model.toJSON()), newer);
const grouped = ED.model.create(Object.assign({}, original, { groups: [{ id: 'g_abcdefgh', title: 'Original group', color: 'neutral', members: ['n_old'] }] }), { types: new Map() });
const groupedBefore = JSON.stringify(grouped.toJSON());
assert.equal(ED.authoring.applyProposal(grouped, proposal, grouped.version, 'Apply'), true);
assert.equal(JSON.stringify(grouped.doc.groups), '[]');
grouped.undo();
assert.equal(JSON.stringify(grouped.toJSON()), groupedBefore);
assert.equal(ED.authoring.applyProposal(model, { nodes: [proposal.nodes[0], proposal.nodes[0]], edges: [] }, model.version, 'Apply'), false);
assert.equal(JSON.stringify(model.toJSON()), newer);
console.log('All EasyDrag proposal checks passed.');
