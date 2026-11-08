import { clamp } from './config.js';

const TRANSITIONS = ['left', 'right', 'plunger', 'start'];

export function createInput(root, settings, { active, playing, action, binding }) {
    const controller = new AbortController(), held = new Set(), simulated = new Set(), transitions = [], nudges = [];
    let config = settings, capture = null, padIndex = null, previous = [], previousAxes = [0, 0], sampledPad = {};
    const listen = (target, name, fn) => target.addEventListener(name, fn, { signal: controller.signal });
    function clear() { held.clear(); simulated.clear(); transitions.length = 0; nudges.length = 0; previous = []; previousAxes = [0, 0]; sampledPad = {}; }
    function rebind(device, code) {
        const { key } = capture, map = config[device === 'key' ? 'keys' : 'pad'];
        const other = Object.keys(map).find(name => map[name] === code);
        if (other) map[other] = map[key];
        map[key] = code; capture = null; clear(); binding(config);
    }
    listen(document, 'keydown', event => {
        if (!active()) return;
        if (capture?.device === 'key') { event.preventDefault(); if (event.code === 'Escape') { capture = null; binding(config); } else if (!event.repeat) rebind('key', event.code); return; }
        if (event.target.closest('input,select,textarea,[contenteditable="true"]')) return;
        const key = Object.keys(config.keys).find(name => config.keys[name] === event.code);
        if (!key) return;
        if (!playing() && key !== 'start' && (key !== 'pause' || event.target.closest('button') && ['Space', 'Enter', 'NumpadEnter'].includes(event.code))) return;
        event.preventDefault();
        if (held.has(key) || event.repeat) return;
        held.add(key);
        if (TRANSITIONS.includes(key) && (key !== 'start' || playing())) transitions.push({ key, down: true, time: performance.now() });
        if (key === 'start' && !playing()) action('start');
        if (key.startsWith('nudge')) nudges.push({ x: key === 'nudgeLeft' ? -1 : key === 'nudgeRight' ? 1 : 0, z: key === 'nudgeUp' ? 1 : 0 });
        else if (['pause', 'camera', 'reset'].includes(key)) action(key);
    });
    listen(document, 'keyup', event => {
        const key = Object.keys(config.keys).find(name => config.keys[name] === event.code);
        if (key && held.delete(key) && TRANSITIONS.includes(key) && (key !== 'start' || playing())) transitions.push({ key, down: false, time: performance.now() });
    });
    listen(window, 'blur', () => { clear(); action('focusLost'); });
    listen(document, 'visibilitychange', () => { if (document.hidden) { clear(); action('focusLost'); } });
    listen(window, 'gamepaddisconnected', event => { if (event.gamepad.index === padIndex) { padIndex = null; clear(); action('disconnected'); } });
    function poll() {
        if (!active()) { clear(); return { left: false, right: false, plunger: 0, start: false }; }
        const pads = navigator.getGamepads?.() || [];
        const pad = [...pads].find(p => p?.connected && (padIndex === null || p.index === padIndex));
        let padValues = {};
        if (pad) {
            padIndex = pad.index;
            if (capture?.device === 'pad') {
                const index = pad.buttons.findIndex((b, i) => b.value > .65 && !previous[i]);
                if (index >= 0) rebind('pad', index);
            } else {
                for (const [name, index] of Object.entries(config.pad)) {
                    padValues[name] = pad.buttons[index]?.value || 0;
                    if (padValues[name] > .6 && !previous[index]) {
                        if (['pause', 'camera', 'reset'].includes(name) || name === 'start' && !playing()) action(name);
                        if (name.startsWith('nudge')) nudges.push({ x: name === 'nudgeLeft' ? -1 : name === 'nudgeRight' ? 1 : 0, z: name === 'nudgeUp' ? 1 : 0 });
                    }
                }
                for (const axis of [0, 1]) {
                    const value = pad.axes[axis] || 0;
                    if (Math.abs(value) > .35 && !previousAxes[axis] && (axis === 0 || value < 0)) nudges.push({ x: axis === 0 ? value : 0, z: axis === 1 ? -value : 0 });
                    previousAxes[axis] = Math.abs(value) > .25 ? Math.sign(value) : 0;
                }
            }
            previous = pad.buttons.map(b => b.value > .6);
        } else if (padIndex !== null) { padIndex = null; clear(); action('disconnected'); }
        sampledPad = padValues;
        return { left: held.has('left') || padValues.left > .5, right: held.has('right') || padValues.right > .5, plunger: held.has('plunger') ? 1 : clamp(padValues.plunger || 0, 0, 1), start: held.has('start') || padValues.start > .5 };
    }
    function at(time) {
        while (transitions.length && transitions[0].time <= time) {
            const edge = transitions.shift(); if (edge.down) simulated.add(edge.key); else simulated.delete(edge.key);
        }
        return { left: simulated.has('left') || sampledPad.left > .5, right: simulated.has('right') || sampledPad.right > .5, plunger: simulated.has('plunger') ? 1 : clamp(sampledPad.plunger || 0, 0, 1), start: simulated.has('start') || sampledPad.start > .5 };
    }
    return { poll, at, clear, nudges: () => nudges.splice(0), setSettings(value) { config = value; clear(); }, capture(key, device) { clear(); capture = { key, device }; }, cancelCapture() { capture = null; }, inspect: () => ({ pad: padIndex, capture }), dispose() { controller.abort(); clear(); capture = null; } };
}
