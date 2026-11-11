import assert from 'node:assert/strict';
import { createInput } from '../ui/js/desktop/apps/pinball/input.js';
import { normalizeSettings } from '../ui/js/desktop/apps/pinball/config.js';
import { normalizeSettings as normalizeNew } from '../ui/js/desktop/apps/pinball/config.js';

const doc = new EventTarget(), win = new EventTarget();
let now = 0, playing = true, onButton = false, pads = [], bound = 0;
doc.hidden = false; doc.closest = selector => onButton && selector === 'button' ? {} : null;
globalThis.document = doc; globalThis.window = win;
Object.defineProperty(globalThis, 'performance', { value: { now: () => now }, configurable: true });
Object.defineProperty(globalThis, 'navigator', { value: { getGamepads: () => pads }, configurable: true });
const actions = [], settings = normalizeSettings();
const input = createInput({}, settings, { active: () => true, playing: () => playing, action: name => actions.push(name), binding: () => bound++ });
function key(type, code, time, extra = {}) { now = time; const event = new Event(type, { cancelable: true }); Object.assign(event, { code, repeat: false }, extra); doc.dispatchEvent(event); return event; }

key('keydown', 'ShiftLeft', 10); key('keyup', 'ShiftLeft', 12); input.poll();
assert.equal(input.at(9).left, false); assert.equal(input.at(11).left, true); assert.equal(input.at(13).left, false, 'a tap between render frames keeps both edges');
key('keydown', 'Space', 14); assert.equal(input.at(14).plunger, 1); input.clear(); assert.equal(input.at(20).plunger, 0);
input.capture('left', 'key'); key('keydown', 'KeyA', 21); assert.equal(bound, 1); assert.equal(settings.keys.left, 'KeyA');
key('keydown', 'KeyA', 22); assert.equal(input.at(22).left, true); key('keyup', 'KeyA', 23); assert.equal(input.at(23).left, false);
playing = false; onButton = true;
assert.equal(key('keydown', 'Space', 24).defaultPrevented, false, 'Space keeps native menu-button activation');
onButton = false; playing = true;
const buttons = Array.from({ length: 17 }, () => ({ value: 0 }));
pads = [{ index: 0, connected: true, buttons, axes: [0, 0] }];
buttons[4].value = 1; input.poll(); assert.equal(input.at(30).left, true);
buttons[9].value = 1; input.poll(); input.poll(); assert.equal(actions.filter(a => a === 'pause').length, 1);
buttons[2].value = 1; input.poll(); input.poll(); assert.equal(actions.filter(a => a === 'reset').length, 1, 'training reset is a remappable edge-triggered gamepad action');
pads[0].axes[0] = -.5; input.poll(); assert.equal(input.nudges()[0].x, -.5); input.poll(); assert.equal(input.nudges().length, 0);
const disconnect = new Event('gamepaddisconnected'); disconnect.gamepad = { index: 0 }; win.dispatchEvent(disconnect); pads = [];
assert.ok(actions.includes('disconnected')); assert.equal(input.at(40).left, false);
input.dispose(); assert.equal(key('keydown', 'KeyA', 50).defaultPrevented, false);
// T18: start action
{
    const cfg = normalizeNew(); const acts = []; let play = true;
    const inp = createInput({}, cfg, { active: () => true, playing: () => play, action: n => acts.push(n), binding: () => {} });
    key('keydown', 'Digit1', 100); key('keyup', 'Digit1', 102);
    assert.equal(inp.at(99).start, false); assert.equal(inp.at(101).start, true); assert.equal(inp.at(103).start, false, 'start tap between frames keeps both edges');
    assert.equal(acts.includes('start'), false, 'start is a game button while playing');
    play = false; key('keydown', 'Digit1', 110); key('keyup', 'Digit1', 111); assert.deepEqual(acts, ['start'], 'start action when not playing');
    play = true; const b = Array.from({ length: 17 }, () => ({ value: 0 }));
    pads = [{ index: 1, connected: true, buttons: b, axes: [0, 0] }];
    b[8].value = 1; assert.equal(inp.poll().start, true); assert.equal(inp.at(120).start, true, 'pad button 8 maps to start');
    inp.dispose(); pads = [];
}
// Windows unifies a left/right modifier pair and can drop one keyup when both keys are released
// together. Without reconciliation one bat stays latched until that key is pressed again.
{
    const fresh = () => createInput({}, normalizeSettings(), { active: () => true, playing: () => true, action: () => {}, binding: () => {} });
    const shiftDown = { getModifierState: name => name === 'Shift' };
    const shiftUp = { getModifierState: () => false };
    const inp = fresh();
    key('keydown', 'ShiftLeft', 200);
    key('keydown', 'ShiftRight', 201);
    assert.equal(inp.at(202).left, true, 'left coil follows ShiftLeft');
    assert.equal(inp.at(202).right, true, 'right coil follows ShiftRight');
    key('keyup', 'ShiftLeft', 203, shiftUp);   // the sibling keyup never arrives
    assert.equal(inp.poll().left, false); assert.equal(inp.poll().right, false);
    assert.equal(inp.at(204).left, false, 'left coil releases');
    assert.equal(inp.at(204).right, false, 'right coil releases without its own keyup');
    inp.dispose();
    const pair = fresh();
    key('keydown', 'ShiftLeft', 210); key('keydown', 'ShiftRight', 211);
    key('keyup', 'ShiftLeft', 212, shiftDown);   // ShiftRight is still physically held
    assert.equal(pair.at(213).left, false, 'released coil drops');
    assert.equal(pair.at(213).right, true, 'a still-held Shift keeps its bat up');
    key('keyup', 'ShiftRight', 214, shiftUp);
    assert.equal(pair.at(215).right, false, 'the last Shift release drops the remaining bat');
    pair.dispose();
    const fallback = fresh();
    key('keydown', 'ShiftLeft', 220); key('keydown', 'ShiftRight', 221);
    key('keyup', 'ShiftLeft', 222, { shiftKey: false });   // getModifierState unavailable
    assert.equal(fallback.at(223).right, false, 'the shiftKey flag is a valid fallback signal');
    fallback.dispose();
    const repeated = fresh();
    key('keydown', 'ShiftLeft', 230);
    key('keydown', 'ShiftRight', 231, { repeat: true });
    assert.equal(repeated.at(232).right, true, 'a repeat-flagged second Shift is still a distinct press');
    repeated.dispose();
    const repeats = fresh();
    key('keydown', 'ArrowLeft', 240); key('keydown', 'ArrowLeft', 241, { repeat: true });
    assert.equal(repeats.nudges().length, 1, 'ordinary key auto-repeat stays deduplicated');
    repeats.dispose();
}
console.log('Pinball input: sub-frame edges, paired Shift release, remapping, native menu keys, analogue nudge, controller disconnect and disposal passed');
