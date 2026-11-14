#!/usr/bin/env node
// A2 (S0c): stoppedBy names the logic.stop step that ended a successful run, and nothing else.
// The stop step itself is "success" for both stop choices (engine_state.go), so the run's own
// status decides: a stop with status "error" and a failure inside the stop's grace end the run
// as "error" and get no hint.
// W1a A: the run shows the status "stopped" instead of a hint line; statusOf is checked at the end.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const box = vm.createContext({ window: {}, console });
for (const name of ['easydrag-status.js', 'easydrag-durable.js', 'easydrag-testing.js']) {
    const file = new URL('../ui/js/desktop/apps/' + name, import.meta.url);
    vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
}
const { stoppedBy } = box.window.EasyDrag.testing;

const TRIGGER = 'n_trigger1', WORK = 'n_workwork', STOP = 'n_stopstop';
const doc = { nodes: [
    { id: TRIGGER, type: 'trigger.manual', label: 'Start' },
    { id: WORK, type: 'web.search', label: 'Suche' },
    { id: STOP, type: 'logic.stop', label: 'Fertig' }
] };
const step = (id, status) => ({ node_id: id, status });
const model = { node: id => doc.nodes.find(n => n.id === id) || null, info: type => (type === 'logic.stop' ? { label: 'Stopp' } : null) };

// 1. A stop run: the stop step succeeded, the rest was cut off or skipped, the run is a success.
const stopRun = { run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(STOP, 'success'), step(WORK, 'skipped')], doc };
const hit = stoppedBy(stopRun, model);
assert.equal(hit.step.node_id, STOP, 'the hint names the stop step');
assert.equal(hit.label, 'Fertig', 'with the label of its node in the run document');
assert.equal(stoppedBy(Object.assign({}, stopRun, { doc: { nodes: [{ id: STOP, type: 'logic.stop' }] } }), model).label, 'Stopp', 'a node without a label takes the type label');

// 2. A stop run with a failed step after the stop: the run is an error, no hint. Same for a stop
// with status "error" (FLOW_STOPPED), whose step is a success as well.
const failedAfter = { run: { status: 'error' }, steps: [step(TRIGGER, 'success'), step(STOP, 'success'), step(WORK, 'error')], doc };
assert.equal(stoppedBy(failedAfter, model), null, 'a failure after the stop is not hidden');
assert.equal(stoppedBy({ run: { status: 'error' }, steps: [step(STOP, 'success')], doc }, model), null, 'a stop that ends the run as error shows no hint');

// 3. An ordinary run: no stop step ran (the stop node was skipped, or the flow has none).
const ordinary = { run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(WORK, 'success'), step(STOP, 'skipped')], doc };
assert.equal(stoppedBy(ordinary, model), null, 'a skipped stop node is no hint');
const noStopDoc = { nodes: doc.nodes.filter(n => n.id !== STOP) };
assert.equal(stoppedBy({ run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(WORK, 'success')], doc: noStopDoc }, model), null, 'a flow without a stop node');
assert.equal(stoppedBy({ run: { status: 'success' }, steps: [step(STOP, 'success')], doc: noStopDoc }, model), null, 'a document that has no such node: the model is not asked');

// 4. The document is missing (a pruned version): fall back to the editor model, else no hint.
const pruned = { run: { status: 'success' }, steps: [step(TRIGGER, 'success'), step(STOP, 'success')] };
assert.equal(stoppedBy(pruned, model).label, 'Fertig', 'the model node with the same id and type stands in');
assert.equal(stoppedBy(pruned, { node: () => null }), null, 'the model does not know the node');
assert.equal(stoppedBy(pruned, { node: () => ({ id: STOP, type: 'web.search' }) }), null, 'the model node has another type');
assert.equal(stoppedBy(pruned, null), null, 'no model at all');
assert.equal(stoppedBy(null, model), null, 'no detail');
assert.equal(stoppedBy({ run: { status: 'success' } }, model), null, 'no steps');
console.log('easydrag stop hint: ok');

// 5. (S0c review A) A run that never started has no failed step, only a run-level error_code: the
// results panel still says why. createResults runs on a minimal stub of the core and the canvas.
{
    const body = { innerHTML: '' }, summary = { textContent: '' };
    const el = { hidden: true, open: false, querySelector: sel => (sel === '.ed-disclosure-head' ? summary : body), setAttribute() {}, addEventListener() {} };
    box.window.EasyDrag.core = {
        bag: () => ({ listen() {}, add() {}, dispose() {} }), el: () => el, tr: (t, key, fallback) => fallback,
        stepErrorText: (t, s) => (s && s.error_code ? 'ERR ' + s.error_code + (s.error_message ? ': ' + s.error_message : '') : ''),
        fmt: { duration: String }
    };
    box.window.EasyDrag.runs = { isFinal: () => true }; box.window.EasyDrag.kit = { disclosure: () => el, setOpen() {}, setTitle: (_, text) => { summary.textContent = text; } };
    let onRun = null;
    const ed = { t: key => key, esc: s => String(s), model: model, isBlock: false, bus: { on: (name, fn) => { onRun = fn; return () => {}; } }, run: null };
    box.window.EasyDrag.testing.createResults(ed, { el: { appendChild() {} } });
    const paint = record => { ed.run = { id: 'r1', mode: 'live', status: 'error', steps: new Map(), record }; onRun(); return body.innerHTML; };
    const failedStart = paint({ id: 'r1', status: 'error', error_code: 'FLOW_START_FAILED', error_message: 'the flow is not published' });
    assert.ok(failedStart.includes('<p class="ed-hint">ERR FLOW_START_FAILED: the flow is not published</p>'), 'a run-level error without a failed step is shown');
    assert.ok(!paint({ id: 'r2', status: 'error' }).includes('ERR '), 'no error_code, no hint');
}
console.log('easydrag start failure results: ok');

// 6. (W1a A) statusOf and statusLabel: a run a stop step ended is "stopped".
{
    const T = box.window.EasyDrag.testing, D = box.window.EasyDrag.durable;
    const stopRunFull = { id: 'r1', status: 'success', steps: new Map([[STOP, step(STOP, 'success')]]), record: { id: 'r1', status: 'success' } };
    assert.equal(T.statusOf({ model }, stopRunFull), 'stopped', 'a successful run a stop step ended is "stopped"');
    assert.equal(T.statusOf({ model }, { id: 'r2', status: 'error', steps: new Map([[STOP, step(STOP, 'success')]]), record: { id: 'r2', status: 'error' } }), 'failed', 'an error run is not "stopped"');
    assert.equal(T.statusOf({ model }, { id: 'r3', status: 'cancelled', steps: new Map(), record: { id: 'r3', status: 'cancelled' } }), 'cancelled');
    assert.equal(T.statusOf({ model }, { id: 'r4', status: 'success', steps: new Map(), record: { id: 'r4', status: 'success' } }), 'ok');
    assert.equal(D.value({ id: 'r5', status: 'running', record: { id: 'r5', status: 'running' } }), 'running');
    assert.equal(D.value({ id: 'r6', status: 'queued', record: { id: 'r6', status: 'queued' } }), 'waiting');
    assert.equal(D.value({ status: 'error', record: { status: 'running' } }), 'failed', 'a finished live run beats a stale record');
    assert.equal(D.value({ status: 'success', record: { status: 'running' } }), 'ok');
    assert.equal(T.statusLabel(key => key, { model }, stopRunFull), 'stopped', 'the word of a stopped run (the harness t returns the key)');
}
console.log('easydrag stopped status: ok');
