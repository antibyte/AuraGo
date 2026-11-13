#!/usr/bin/env node
// W1a D.3: the motion gate. allow() is false when the user chose Ruhig, when the system asks for reduced
// motion, and when the desktop turned animations off; data-motion on a bound root follows all three.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const KEY = 'aurago.easydrag.liveliness';
const source = fs.readFileSync(new URL('easydrag-delight.js', apps), 'utf8');

// load runs a fresh module in its own page stub. state.reduced and state.animations stand for the system
// and the desktop setting; change() calls the listeners that the module registered, like the browser.
function load(start = {}) {
    const store = new Map(start.stored ? [[KEY, start.stored]] : []);
    const state = { reduced: !!start.reduced, animations: start.animations };
    const listeners = [];
    const body = { dataset: {} };
    Object.defineProperty(body.dataset, 'animations', { get: () => state.animations, enumerable: true });
    const box = vm.createContext({
        window: {
            matchMedia: query => ({ matches: state.reduced && query.includes('prefers-reduced-motion'), addEventListener: (name, fn) => { if (name === 'change') listeners.push(fn); } }),
            addEventListener() {}
        },
        document: { body }, console, Map, Set,
        localStorage: { getItem: k => (store.has(k) ? store.get(k) : null), setItem: (k, v) => { store.set(k, String(v)); }, removeItem: k => { store.delete(k); } }
    });
    vm.runInContext(source, box, { filename: 'easydrag-delight.js' });
    const root = { isConnected: true, attrs: {}, getAttribute(name) { return this.attrs[name] ?? null; }, setAttribute(name, value) { this.attrs[name] = value; } };
    return { delight: box.window.EasyDrag.delight, state, store, root, change: () => listeners.forEach(fn => fn()) };
}

// Nothing asks for calm: normal, and everything may move.
let h = load({ animations: 'true' });
assert.equal(h.delight.level(), 'normal');
assert.equal(h.delight.choice(), 'normal');
for (const kind of ['run', 'drag', 'insert', 'loading']) assert.equal(h.delight.allow(kind), true, kind + ' is allowed in normal');

// Each of the three conditions alone makes allow() false for every kind.
h = load({ animations: 'true', stored: 'calm' });
assert.equal(h.delight.level(), 'calm');
assert.equal(h.delight.choice(), 'calm');
assert.equal(h.delight.allow('run'), false, 'Ruhig blocks motion');

h = load({ animations: 'true', reduced: true });
assert.equal(h.delight.level(), 'calm');
assert.equal(h.delight.choice(), 'normal', 'the system setting does not change the stored choice');
assert.equal(h.delight.allow('run'), false, 'reduced motion blocks motion');

h = load({ animations: 'false' });
assert.equal(h.delight.level(), 'calm');
assert.equal(h.delight.allow('drag'), false, 'data-animations=false blocks motion');
h = load({});
assert.equal(h.delight.level(), 'normal', 'a body without data-animations is not calm');

// setLevel stores the choice per device and ignores anything but the two values.
h = load({ animations: 'true' });
assert.equal(h.delight.setLevel('calm'), 'calm');
assert.equal(h.store.get(KEY), 'calm');
assert.equal(h.delight.allow('run'), false);
assert.equal(h.delight.setLevel('lively'), 'calm', 'an unknown value changes nothing');
assert.equal(h.store.get(KEY), 'calm');
assert.equal(h.delight.setLevel('normal'), 'normal');
assert.equal(h.delight.allow('run'), true);

// data-motion on a bound root follows the three conditions as they change.
h = load({ animations: 'true' });
const unbind = h.delight.bind(h.root);
assert.equal(h.root.getAttribute('data-motion'), 'normal');
h.delight.setLevel('calm');
assert.equal(h.root.getAttribute('data-motion'), 'calm');
h.delight.setLevel('normal');
assert.equal(h.root.getAttribute('data-motion'), 'normal');
h.state.reduced = true; h.change();
assert.equal(h.root.getAttribute('data-motion'), 'calm', 'the media query change reaches the root');
h.state.reduced = false; h.change();
assert.equal(h.root.getAttribute('data-motion'), 'normal');
h.state.animations = 'false'; h.delight.refresh();
assert.equal(h.root.getAttribute('data-motion'), 'calm', 'the desktop setting reaches the root');
h.state.animations = 'true'; h.delight.refresh();
assert.equal(h.root.getAttribute('data-motion'), 'normal');
unbind();
h.delight.setLevel('calm');
assert.equal(h.root.getAttribute('data-motion'), 'normal', 'an unbound root is left alone');
const gone = { isConnected: false, attrs: {}, getAttribute() { return null; }, setAttribute() { throw new Error('a disconnected root must not be written'); } };
h.delight.bind(gone);
h.delight.setLevel('normal');

// The menu entry: one parent item with two choices; the current one carries the check mark.
h = load({ animations: 'true', stored: 'calm' });
const words = { 'easydrag.ui.liveliness': 'Lebendigkeit', 'easydrag.ui.liveliness_normal': 'Normal', 'easydrag.ui.liveliness_calm': 'Ruhig' };
const items = h.delight.menuItems(key => words[key] || key);
assert.equal(items.length, 1);
assert.equal(items[0].label, 'Lebendigkeit');
assert.deepEqual([...items[0].items.map(item => item.label)], ['Normal', 'Ruhig']);
assert.deepEqual([...items[0].items.map(item => !!item.shortcutHint)], [false, true], 'the check mark marks the current choice');
items[0].items[0].action();
assert.equal(h.store.get(KEY), 'normal');
assert.equal(h.delight.menuItems(key => key)[0].items[0].shortcutHint.length > 0, true);
console.log('PASS motion gate: three conditions, data-motion binding, menu entry');
