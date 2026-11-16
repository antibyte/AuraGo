#!/usr/bin/env node
// W1c C.1: the one Stop. ED.runStop.focusRun names the run the header's Stop stops: the viewed run while it is
// active and can be cancelled (durable runs by their checkpoint), else the editor's own run, also while a past
// run is open (the parked run). paint gives the button its words, sync shows or hides it and hands on the focus.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const read = name => fs.readFileSync(new URL(name, apps), 'utf8');

// A button just big enough for paint and sync: attributes, classes, a span child, focus.
function button(text) {
    const attrs = new Map();
    const span = { textContent: text };
    const b = {
        hidden: false, disabled: false, focused: false, span,
        setAttribute: (k, v) => attrs.set(k, String(v)), getAttribute: k => (attrs.has(k) ? attrs.get(k) : null), removeAttribute: k => attrs.delete(k),
        classList: { on: new Set(), toggle(c, on) { if (on) this.on.add(c); else this.on.delete(c); } },
        querySelector: sel => (sel === 'span' ? span : null),
        focus() { box.document.activeElement = b; b.focused = true; }
    };
    return b;
}
const box = vm.createContext({ window: {}, console, document: { activeElement: null }, Set, Map, Object, Array });
for (const file of ['easydrag-durable.js', 'easydrag-run-stop.js']) vm.runInContext(read(file), box, { filename: file });
const ED = box.window.EasyDrag;
const t = key => key.replace('easydrag.ui.', '');

const own = (status, extra) => Object.assign({ id: 'r_own', status, mode: 'test', steps: new Map() }, extra);
const viewed = (status, extra) => Object.assign({ id: 'r_view', status, mode: 'live', started_at: '2026-11-16T10:00:00Z' }, extra);
const host = (patch) => Object.assign({ run: null, runView: null, parkedRun: null }, patch);
const ids = ed => { const f = ED.runStop.focusRun(ed); return f ? [f.id, f.mode, f.status] : null; };

// 1. Nothing runs: no run in focus.
assert.equal(ED.runStop.focusRun(host()), null, 'idle: no run in focus');

// 2. The editor's own run: queued, waiting and running count, a final one does not; a stale run only when durable.
assert.deepEqual(ids(host({ run: own('running') })), ['r_own', 'test', 'running']);
assert.deepEqual(ids(host({ run: own('queued', { mode: 'live' }) })), ['r_own', 'live', 'queued']);
assert.equal(ED.runStop.focusRun(host({ run: own('success') })), null, 'a finished own run has no Stop');
assert.equal(ED.runStop.focusRun(host({ run: own('running', { stale: true }) })), null, 'a run whose stream was given up has no Stop');
assert.deepEqual(ids(host({ run: own('running', { stale: true, durableKnown: true }) })), ['r_own', 'test', 'running'], 'a durable run stays stoppable');
const record = ED.runStop.focusRun(host({ run: own('running', { record: { id: 'r_own', started_at: '2026-11-16T09:00:00Z', durable: true } }) }));
assert.equal(record.started_at, '2026-11-16T09:00:00Z', 'the run record travels with the focus (the confirmation names its start)');
assert.equal(record.durable, true);

// 3. The review of a run that is still active: the viewed run is in focus (it is ed.run too, as a review).
const review = (status, extra, ed) => host(Object.assign({ runView: { run: viewed(status, extra) }, run: own(status, { id: 'r_view', view: true }) }, ed));
assert.deepEqual(ids(review('waiting')), ['r_view', 'live', 'waiting']);
assert.equal(ED.runStop.focusRun(review('success')), null, 'a finished viewed run has no Stop');
assert.equal(ED.runStop.focusRun(review('waiting', { durable: true })), null, 'a durable run without a checkpoint cannot be stopped from here');
const checkpoint = (generation, cancel) => ({ run_id: 'r_view', state: 'waiting', generation, cancel_available: cancel });
assert.deepEqual(ids(review('waiting', { durable: true }, { run: own('waiting', { id: 'r_view', view: true, record: { id: 'r_view', durable: true, status: 'waiting' }, durableStatus: checkpoint(2, true) }) })), ['r_view', 'live', 'waiting'], 'a checkpoint that allows the cancel puts the run in focus');
assert.equal(ED.runStop.focusRun(review('waiting', { durable: true }, { run: own('waiting', { id: 'r_view', view: true, record: { id: 'r_view', durable: true, status: 'waiting' }, durableStatus: checkpoint(2, false) }) })), null, 'a checkpoint that forbids the cancel takes the run out of focus');
assert.equal(ED.runStop.focusRun(review('waiting', {}, { run: own('waiting', { id: 'r_view', view: true, durableStatusError: true }) })), null, 'a lost checkpoint read takes the proof away');
// A failed run whose legacy checkpoint still offers the cleanup keeps its Stop (the banner's rule).
assert.deepEqual(ids(review('error', { durable: true }, { run: own('error', { id: 'r_view', view: true, record: { id: 'r_view', durable: true, status: 'error' }, durableStatus: { run_id: 'r_view', state: 'blocked', generation: 2, cancel_available: true, blocked_code: 'incompatible' } }) })), ['r_view', 'live', 'error']);

// 4. A past run is open and the editor's own run is parked behind it: the Stop stops the own run.
const parked = { id: 'r_own', mode: 'test', record: { id: 'r_own', status: 'running', started_at: '2026-11-16T09:30:00Z' } };
const past = review('success', {}, { parkedRun: parked });
assert.deepEqual(ids(past), ['r_own', 'test', 'running']);
assert.equal(ED.runStop.focusRun(past).started_at, '2026-11-16T09:30:00Z');
// An active viewed run wins over the parked one; the parked run that is the viewed run adds nothing.
assert.deepEqual(ids(review('running', {}, { parkedRun: parked })), ['r_view', 'live', 'running']);
assert.equal(ED.runStop.focusRun(review('success', { id: 'r_own' }, { parkedRun: parked })), null, 'the parked run itself, shown as finished, is nothing to stop');

// 5. paint: the words, the busy state, the name.
const b = button('Stop');
ED.runStop.paint(b, t, true);
assert.deepEqual([b.span.textContent, b.disabled, b.getAttribute('aria-busy'), b.getAttribute('aria-label'), b.getAttribute('title'), b.classList.on.has('is-busy')], ['run_stopping', false, 'true', 'run_stopping', 'run_stopping', true]);
ED.runStop.paint(b, t, false);
assert.deepEqual([b.span.textContent, b.disabled, b.getAttribute('aria-busy'), b.getAttribute('aria-label'), b.classList.on.has('is-busy')], ['run_cancel', false, null, 'run_cancel', false], 'the label comes back');

// 6. sync: shown for a run in focus, hidden without; pending follows the run in focus; the focus is handed on.
const exit = button('Back'), canvas = button('');
const root = { querySelector: sel => (sel === '[data-ed-cmd="exit-run-view"]' ? exit : sel === '.ed-canvas' ? canvas : null) };
const stop = button('Stop');
let ed = host({ root, run: own('running') });
assert.deepEqual([ED.runStop.sync(ed, stop, t, () => false) !== null, stop.hidden, stop.getAttribute('aria-busy')], [true, false, null]);
ED.runStop.sync(ed, stop, t, id => id === 'r_own');
// The registry turns aria-busy into aria-disabled (not loaded here; the keyboard browser test pins it); the button is never disabled.
assert.deepEqual([stop.span.textContent, stop.getAttribute('aria-busy'), stop.disabled], ['run_stopping', 'true', false], 'a stop in flight shows on the Stop of that run');
ED.runStop.sync(host({ root, runView: { run: viewed('waiting') }, run: own('waiting', { id: 'r_view', view: true }) }), stop, t, id => id === 'r_own');
assert.deepEqual([stop.span.textContent, stop.getAttribute('aria-busy'), stop.disabled], ['run_cancel', null, false], 'the pending state of another run does not show');
stop.focus();
ED.runStop.sync(host({ root, runView: { run: viewed('success') }, run: own('success', { id: 'r_view', view: true }) }), stop, t, () => false);
assert.deepEqual([stop.hidden, box.document.activeElement === exit], [true, true], 'a Stop that goes away in the review hands the focus to "Back to draft"');
stop.hidden = false; stop.focus();
ED.runStop.sync(host({ root, run: own('success') }), stop, t, () => false);
assert.deepEqual([stop.hidden, box.document.activeElement === canvas], [true, true], 'else the focus goes to the canvas');
canvas.focused = false;
stop.hidden = false; box.document.activeElement = exit;
ED.runStop.sync(host({ root, run: own('success') }), stop, t, () => false);
assert.equal(canvas.focused, false, 'a focus elsewhere stays');

console.log('ok   the one Stop: focus rules, words, pending, focus hand-over');
