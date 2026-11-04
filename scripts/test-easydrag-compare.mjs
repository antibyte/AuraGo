#!/usr/bin/env node
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const box = vm.createContext({ window: {}, Map, Set, WeakMap, Object, JSON });
vm.runInContext(fs.readFileSync(new URL('../ui/js/desktop/apps/easydrag-compare.js', import.meta.url), 'utf8'), box);
const { compare, diagnostic } = box.window.EasyDrag.compare;
const left = { run: { id: 'run_a', flow_id: 'flow_a', revision: 2, status: 'success', trigger_data: { private: 'payload' }, error_message: 'sensitive detail' }, steps: [
    { node_id: 'n_a', status: 'success', output: { name: 'sample', count: 1 }, duration_ms: 10, error_message: 'never export this' }
] };
const reordered = JSON.parse(JSON.stringify(left));
reordered.run.id = 'run_b';
reordered.steps[0].output = { count: 1, name: 'sample' };
reordered.steps[0].duration_ms = 12;
assert.equal(compare(left, reordered).rows.length, 0, 'key order and duration do not imply semantic changes');
reordered.steps[0].output.count = 2;
let result = compare(left, reordered);
assert.equal(result.rows.length, 1);
assert.equal(result.rows[0].field, 'output');
assert.equal(result.incomplete, false);
const deep = {};
let branch = deep;
for (let i = 0; i < 20; i++) { branch.next = {}; branch = branch.next; }
result = compare({ run: { trigger_data: deep } }, { run: { trigger_data: JSON.parse(JSON.stringify(deep)) } });
assert.equal(result.incomplete, true, 'bounded comparison never claims equality for omitted depth');
assert.ok(result.rows.length > 0);
result = compare({ run: { trigger_data: { _preview: 'partial' } } }, { run: { trigger_data: { _preview: 'partial' } } });
assert.equal(result.incomplete, true, 'equal preview markers cannot prove equal original input');
result = compare({ run: { trigger_data: { token: '[redacted]' } } }, { run: { trigger_data: { token: '[redacted]' } } });
assert.equal(result.incomplete, true, 'equal redactions cannot prove equal original input');
result = compare({ steps: [{ node_id: 'n_a', params: { text: 'old' } }] }, { steps: [{ node_id: 'n_a', params: { text: 'new' } }] });
assert.equal(result.rows[0].field, 'params', 'stored resolved parameters are the input contract');
const serialized = JSON.stringify(diagnostic([left, reordered]));
for (const privateText of ['payload', 'sensitive', 'never export', 'sample', 'trigger_data', 'output', 'error_message']) assert.equal(serialized.includes(privateText), false);
assert.ok(serialized.includes('run_a'));
assert.ok(serialized.includes('n_a'));
const massive = { steps: Array.from({ length: 600 }, (_, i) => ({ node_id: 'n_' + i, status: 'success' })) };
assert.equal(diagnostic([massive]).runs[0].steps.length, 500);
assert.equal(compare(massive, { steps: [] }).incomplete, true);
assert.ok(compare(massive, { steps: [] }).rows.length <= 100);
console.log('All EasyDrag comparison checks passed.');
