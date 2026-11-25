import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs'; import { fakeClock } from './live-fakes.mjs';
const boot = () => loadEngine(['hd-core.js', 'hd-input.js'], { dom: true });
const pe = (type, x, y, extra) => Object.assign({ type, pointerId: 1, isPrimary: true, button: 0, pointerType: 'touch', clientX: x, clientY: y, preventDefault() {} }, extra);
export const tests = [
  ['input: snap anchors the grid at min and never passes max; keys step, page 10 %, Home/End', () => { const I = boot().E.input, ct = { min: 2202, max: 6535, step: 50 };
    assert.equal(I.snap(6535, ct), 6502); assert.equal(I.snap(2230, ct), 2252); assert.equal(I.snap(2220, ct), 2202); assert.equal(I.keyValue('End', 3000, ct), 6502); assert.equal(I.keyValue('Home', 3000, ct), 2202);
    const pct = { min: 0, max: 100, step: 1 }; assert.equal(I.keyValue('ArrowRight', 40, pct), 41); assert.equal(I.keyValue('PageDown', 40, pct), 30); assert.equal(I.keyValue('ArrowUp', 100, pct), 100);
    assert.equal(I.keyValue('ArrowUp', 55, { min: 0, max: 100, step: 5 }), 60); assert.equal(I.snap(0.30000000004, { min: 0, max: 1, step: 0.1 }), 0.3); assert.equal(I.keyValue('a', 1, pct), null); }],
  ['input: fractions for tracks and the dial (15 deg dead zone keeps the last value deep in the gap)', () => { const I = boot().E.input, r = { left: 0, top: 0, width: 200, height: 200 };
    assert.equal(I.fraction(50, 0, r, 'x'), 0.25); assert.equal(I.fraction(0, 50, r, 'y'), 0.75);
    assert.equal(I.dialFraction(100, 0, r, 0.3), 0.5); assert.equal(I.dialFraction(200, 100, r, 0.3), 225 / 270);
    assert.equal(I.dialFraction(100, 200, r, 0.3), 0.3); assert.equal(I.verticalFraction(0.5, -110), 1); }],
  ['input: gesture splits tap and drag at 4 px, holds after 500 ms, swallows the click after a drag', async () => { const { E, document } = boot(), clock = fakeClock(), el = document.createElement('div'), log = [];
    el.addEventListener('click', () => log.push('other'));
    E.input.gesture(el, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, rect: () => ({ left: 0, top: 0, width: 100, height: 44 }),
      onTap: () => log.push('tap'), onHold: () => log.push('hold'), onDragStart: () => log.push('start'), onDrag: p => log.push('drag:' + p.x), onDragEnd: () => log.push('end') });
    el.dispatchEvent(pe('pointerdown', 10, 10)); el.dispatchEvent(pe('pointermove', 13, 10)); el.dispatchEvent(pe('pointerup', 13, 10)); assert.deepEqual(log, ['tap']);
    log.length = 0; el.dispatchEvent(pe('pointerdown', 10, 10)); el.dispatchEvent(pe('pointermove', 15, 10)); el.dispatchEvent(pe('pointermove', 30, 10)); el.dispatchEvent(pe('pointerup', 30, 10));
    assert.deepEqual(log, ['start', 'drag:15', 'drag:30', 'end']); el.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log, ['start', 'drag:15', 'drag:30', 'end']); // swallowed: no tap, no other listener
    el.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log.slice(4), ['other']); // the swallow is one click only
    log.length = 0; el.dispatchEvent(pe('pointerdown', 10, 10)); await clock.advance(500); el.dispatchEvent(pe('pointerup', 10, 10)); assert.deepEqual(log, ['hold']);
    log.length = 0; el.dispatchEvent(pe('pointerdown', 10, 10, { isPrimary: false })); el.dispatchEvent(pe('pointerup', 10, 10, { isPrimary: false })); assert.deepEqual(log, []); }],
  ['input: press activates on click, not after a move or a hold; Shift+Enter runs hold; a disabled control reports blocked', async () => { const { E, document } = boot(), clock = fakeClock(), b = document.createElement('button'), log = []; let ok = true;
    E.input.press(b, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, enabled: () => ok, onBlocked: () => log.push('blocked'), onActivate: () => log.push('act'), onHold: () => log.push('hold') });
    b.dispatchEvent({ type: 'click' }); b.dispatchEvent(pe('pointerdown', 0, 0)); b.dispatchEvent(pe('pointermove', 9, 0)); b.dispatchEvent(pe('pointerup', 9, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} });
    b.dispatchEvent({ type: 'keydown', key: 'Enter', shiftKey: true, preventDefault() {} }); ok = false; b.dispatchEvent({ type: 'click' });
    assert.deepEqual(log, ['act', 'hold', 'blocked']); }],
  ['input: a click listener added after gesture() does not run after a drag', () => { const { E, document } = boot(), clock = fakeClock(), el = document.createElement('div'), log = [];
    E.input.gesture(el, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, onDragEnd: () => log.push('end') }); el.addEventListener('click', () => log.push('late'));
    el.dispatchEvent(pe('pointerdown', 10, 10)); el.dispatchEvent(pe('pointermove', 30, 10)); el.dispatchEvent(pe('pointerup', 30, 10)); el.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log, ['end']); }],
  ['input: gesture pointercancel after a drag starts ends in onDragEnd cancelled and does not swallow the next click', async () => { const { E, document } = boot(), clock = fakeClock(), el = document.createElement('div'), log = [];
    E.input.gesture(el, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, onDragStart: () => log.push('start'), onDragEnd: (p, s, cancelled) => log.push('end:' + cancelled) }); el.addEventListener('click', () => log.push('click'));
    el.dispatchEvent(pe('pointerdown', 10, 10)); el.dispatchEvent(pe('pointermove', 30, 10)); el.dispatchEvent(pe('pointercancel', 0, 0)); await clock.advance(100); el.dispatchEvent({ type: 'click', preventDefault() {} });
    assert.deepEqual(log, ['start', 'end:true', 'click']); }],
  ['input: press and keys on one button run Shift+Enter hold once, in either order', () => { const { E, document } = boot();
    for (const pressFirst of [true, false]) { const b = document.createElement('button'); let n = 0; const hold = () => { n += 1; };
      const bindPress = () => E.input.press(b, { onHold: hold }), bindKeys = () => E.input.keys(b, { onHold: hold }); if (pressFirst) { bindPress(); bindKeys(); } else { bindKeys(); bindPress(); }
      b.dispatchEvent({ type: 'keydown', key: 'Enter', shiftKey: true, defaultPrevented: false, preventDefault() { this.defaultPrevented = true; } }); assert.equal(n, 1, 'pressFirst ' + pressFirst); } }],
  ['input: press pointercancel arms no swallow, so a click 100 ms later activates; a swallowed click stops other listeners', async () => { const { E, document } = boot(), clock = fakeClock(), b = document.createElement('button'), log = [];
    b.addEventListener('click', () => log.push('other')); E.input.press(b, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, onActivate: () => log.push('act') });
    b.dispatchEvent(pe('pointerdown', 0, 0)); b.dispatchEvent(pe('pointercancel', 0, 0)); await clock.advance(100); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log, ['act', 'other']);
    log.length = 0; b.dispatchEvent(pe('pointerdown', 0, 0)); b.dispatchEvent(pe('pointermove', 9, 0)); b.dispatchEvent(pe('pointerup', 9, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log, []); }],
  ['input: repeat pointercancel arms no swallow; a swallowed click after a repeat stops other listeners', async () => { const { E, document } = boot(), clock = fakeClock(), b = document.createElement('button'), log = [];
    b.addEventListener('click', () => log.push('other')); E.input.repeat(b, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, onStep: () => log.push('step') });
    b.dispatchEvent(pe('pointerdown', 0, 0)); await clock.advance(400); b.dispatchEvent(pe('pointercancel', 0, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log, ['step', 'step', 'other']);
    log.length = 0; b.dispatchEvent(pe('pointerdown', 0, 0)); await clock.advance(400); b.dispatchEvent(pe('pointerup', 0, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.deepEqual(log, ['step']); }],
  ['input: repeat steps once per click and 400 ms then every 80 ms while held', async () => { const { E, document } = boot(), clock = fakeClock(), b = document.createElement('button'); let n = 0;
    E.input.repeat(b, { setTimeout: clock.setTimeout, clearTimeout: clock.clearTimeout, now: clock.now, onStep: () => n++ });
    b.dispatchEvent({ type: 'click' }); assert.equal(n, 1); b.dispatchEvent(pe('pointerdown', 0, 0)); await clock.advance(400 + 80 * 3); b.dispatchEvent(pe('pointerup', 0, 0)); b.dispatchEvent({ type: 'click', preventDefault() {} }); assert.equal(n, 5); }]
];
