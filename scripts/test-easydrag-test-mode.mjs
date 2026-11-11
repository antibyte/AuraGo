#!/usr/bin/env node
// Test opens in Simulation when the steps it runs have outward effects (named or unknown);
// flows without effects and durable flows keep the real test.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const box = vm.createContext({ window: {}, console, Intl, Date, Map, Set, JSON, AbortController });
const file = new URL('../ui/js/desktop/apps/easydrag-testing.js', import.meta.url);
vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
const ED = box.window.EasyDrag;

const infos = new Map([
    ['trigger.manual', { type: 'trigger.manual', trigger: true }],
    ['notify.telegram', { type: 'notify.telegram', effects: ['sends_message'] }],
    ['logic.set', { type: 'logic.set' }],
    ['logic.mystery', { type: 'logic.mystery', risky: true }],
    ['tool.x', { type: 'tool.x', effects: ['sends_message'] }],
    ['notify.web', { type: 'notify.web', effects: ['sends_message'], simulatable: false }],
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
assert.equal(pick(['trigger.manual', 'logic.mystery']), true, 'an unknown effect selects Simulation');
assert.equal(pick(['trigger.manual', 'tool.x']), false, 'a generic tool step cannot be simulated');
assert.equal(pick(['trigger.manual', 'notify.telegram', 'notify.web']), false, 'the server flag simulatable=false keeps the real test');
assert.equal(pick(['trigger.manual', 'logic.set', 'notify.telegram'], 'n1'), false, 'a step test counts only what it runs');
assert.equal(pick(['trigger.manual', 'logic.set'], null, new Map([['n1', ['sends_message']]])), true, 'effects of the real settings count');
assert.equal(pick(['trigger.manual', 'notify.telegram', 'logic.durable_wait']), false, 'a durable wait keeps the real test');
assert.equal(pick(['trigger.manual', 'notify.telegram', 'logic.durable_approval']), false, 'a durable approval keeps the real test');
// decorate: the default follows the trigger until the user picks a mode.
{
    // Step test on n2: trigger T1 (n0) does not reach it; T2 (n3) reaches the telegram step.
    const twoTriggers = () => {
        const nodes = [
            { id: 'n0', type: 'trigger.manual', settings: {} }, { id: 'n3', type: 'trigger.manual', settings: {} },
            { id: 'n1', type: 'logic.set', settings: {} }, { id: 'n2', type: 'notify.telegram', settings: {} }];
        const edges = [
            { id: 'e0', source: { node: 'n0', port: 'out' }, target: { node: 'n1', port: 'in' } },
            { id: 'e1', source: { node: 'n3', port: 'out' }, target: { node: 'n2', port: 'in' } }];
        return { model: { doc: { nodes, edges }, info: type => infos.get(type), node: id => nodes.find(n => n.id === id) } };
    };
    const open = ed => {
        const select = { value: 'real', disabled: false, addEventListener(type, fn) { select[type] = fn; } };
        const stub = () => ({ hidden: false, disabled: false, textContent: '', querySelector: () => ({ value: '' }), querySelectorAll: () => [] });
        ED.core = { el: () => Object.assign(stub(), { querySelector: sel => sel === 'select' ? select : { value: '' } }) };
        ed.t = key => key; ed.esc = x => x; ed.api = {};
        const dialog = { body: Object.assign(stub(), { insertBefore() {}, appendChild() {} }), el: { querySelector: () => null }, done: new Promise(() => {}) };
        let trigger = 'n0';
        const handle = ED.testing.decorate(ed, dialog, { trigger: () => trigger, onlyNode: 'n2', real: () => null, isCurrent: () => true });
        return { select, handle, setTrigger: id => { trigger = id; handle.preselect(id, null); } };
    };
    const a = open(twoTriggers());
    assert.equal(a.select.value, 'real', 'T1 has no effects: real');
    a.setTrigger('n3');
    assert.equal(a.select.value, 'simulation', 'T2 sends a message: Simulation follows the trigger');
    a.setTrigger('n0');
    assert.equal(a.select.value, 'simulation', 'never switched back automatically');
    const b = open(twoTriggers());
    b.select.value = 'real';
    b.select.change();
    b.setTrigger('n3');
    assert.equal(b.select.value, 'real', 'a mode the user chose stays');
}
console.log('All EasyDrag test-mode checks passed.');
