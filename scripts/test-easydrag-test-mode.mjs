#!/usr/bin/env node
// Test opens in Simulation when the steps it runs have outward effects (named or unknown);
// flows without effects and durable flows keep the real test.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const box = vm.createContext({ window: {}, console, Intl, Date, Map, Set, JSON });
const file = new URL('../ui/js/desktop/apps/easydrag-testing.js', import.meta.url);
vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
const ED = box.window.EasyDrag;

const infos = new Map([
    ['trigger.manual', { type: 'trigger.manual', trigger: true }],
    ['notify.telegram', { type: 'notify.telegram', effects: ['sends_message'] }],
    ['logic.set', { type: 'logic.set' }],
    ['tool.mystery', { type: 'tool.mystery', risky: true }],
    ['logic.durable_wait', { type: 'logic.durable_wait' }],
    ['logic.durable_approval', { type: 'logic.durable_approval' }]
]);
// edFor builds a chain n0 -> n1 -> ... of the given types.
const edFor = types => {
    const nodes = types.map((type, i) => ({ id: 'n' + i, type, label: type, settings: {} }));
    const edges = nodes.slice(1).map((n, i) => ({ id: 'e' + i, source: { node: nodes[i].id, port: 'out' }, target: { node: n.id, port: 'in' } }));
    return { model: { doc: { nodes, edges }, info: type => infos.get(type), node: id => nodes.find(n => n.id === id) } };
};
const pick = (types, onlyNode, real) => ED.testing.simulationDefault(edFor(types), onlyNode || null, 'n0', real || null);

assert.equal(pick(['trigger.manual', 'notify.telegram']), true, 'a named effect selects Simulation');
assert.equal(pick(['trigger.manual', 'logic.set']), false, 'no effect keeps the real test');
assert.equal(pick(['trigger.manual', 'tool.mystery']), true, 'an unknown effect selects Simulation');
assert.equal(pick(['trigger.manual', 'logic.set', 'notify.telegram'], 'n1'), false, 'a step test counts only what it runs');
assert.equal(pick(['trigger.manual', 'logic.set'], null, new Map([['n1', ['sends_message']]])), true, 'effects of the real settings count');
assert.equal(pick(['trigger.manual', 'notify.telegram', 'logic.durable_wait']), false, 'a durable wait keeps the real test');
assert.equal(pick(['trigger.manual', 'notify.telegram', 'logic.durable_approval']), false, 'a durable approval keeps the real test');
console.log('All EasyDrag test-mode checks passed.');
