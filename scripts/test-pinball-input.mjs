import assert from 'node:assert/strict';
import { createInput } from '../ui/js/desktop/apps/pinball/input.js';
import { normalizeSettings } from '../ui/js/desktop/apps/pinball/tables.js';

const doc = new EventTarget(), win = new EventTarget();
let now = 0, playing = true, onButton = false, pads = [], bound = 0;
doc.hidden = false; doc.closest = selector => onButton && selector === 'button' ? {} : null;
globalThis.document = doc; globalThis.window = win;
Object.defineProperty(globalThis, 'performance', { value: { now: () => now }, configurable: true });
Object.defineProperty(globalThis, 'navigator', { value: { getGamepads: () => pads }, configurable: true });
const actions = [], settings = normalizeSettings();
const input = createInput({}, settings, { active: () => true, playing: () => playing, action: name => actions.push(name), binding: () => bound++ });
function key(type, code, time) { now = time; const event = new Event(type, { cancelable: true }); Object.assign(event, { code, repeat: false }); doc.dispatchEvent(event); return event; }

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
console.log('Pinball input: sub-frame edges, remapping, native menu keys, analogue nudge, controller disconnect and disposal passed');
