import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

// Knob / fader math. The DOM half of the module is covered by the browser test.
const runtime = { console };
runtime.globalThis = runtime;
runtime.window = runtime;
vm.runInNewContext(fs.readFileSync('ui/js/desktop/apps/synth-studio-controls.js', 'utf8'), runtime, { filename: 'synth-studio-controls.js' });
const C = runtime.SynthStudioControls;
assert.ok(C, 'module exports its namespace');

const unit = { min: 0, max: 1, step: 0.01 };
assert.equal(C.snap(0.4999, unit), 0.5);
assert.equal(C.snap(7, { min: 0, max: 5, step: 1 }), 5);
assert.equal(C.snap(-3, { min: -1, max: 1, step: 0.01 }), -1);
assert.equal(C.snap(0.123, { min: 0, max: 1, step: 0.005 }), 0.125);
assert.equal(C.snap(0.3, { min: 0, max: 1, step: 0 }), 0.3);

assert.equal(C.fraction(0.5, { min: 0, max: 2 }), 0.25);
assert.equal(C.fraction(9, { min: 0, max: 2 }), 1);
assert.equal(C.fraction(1, { min: 1, max: 1 }), 0);

assert.equal(C.angle(0), -135);
assert.equal(C.angle(1), 135);

assert.equal(C.arcPath(0, 0), '');
assert.equal(C.arcPath(0.5, 0.5), '');
assert.equal(C.arcPath(0, 0.5), 'M12.44 43.56A22 22 0 0 1 28.00 6.00');
assert.equal(C.arcPath(0, 1), 'M12.44 43.56A22 22 0 1 1 43.56 43.56');
assert.equal(C.arcPath(0.5, 0), C.arcPath(0, 0.5), 'arc is symmetric in its ends');

assert.equal(C.dragValue(0.5, 100, 200, unit, false), 1);
assert.equal(C.dragValue(0.5, 80, 200, unit, true), 0.6, 'shift drags four times finer');
assert.equal(C.dragValue(0.9, -1000, 200, unit, false), 0);

const kind = (attrs, type = 'range') => C.kindFor({
    type,
    dataset: attrs.dataset || {},
    hasAttribute: name => (attrs.has || []).includes(name),
    matches: () => !!attrs.knob
});
assert.equal(kind({ knob: true }), 'knob');
assert.equal(kind({ has: ['data-track-volume'] }), 'hfader');
assert.equal(kind({ dataset: { ssKind: 'hfader' } }), 'hfader');
assert.equal(kind({ knob: true, dataset: { ssPlain: '' } }), null);
assert.equal(kind({ knob: true }, 'number'), null);
assert.equal(kind({}), null);
assert.ok(C.KNOB_SELECTOR.split(',').includes('[data-macro]'), 'performance macros are drawn as knobs');
console.log('synth-studio controls ok');
