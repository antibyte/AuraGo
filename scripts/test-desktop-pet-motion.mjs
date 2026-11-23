import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const root = new URL('../', import.meta.url);
const catalog = JSON.parse(readFileSync(new URL('assets/desktop-pets/catalog.json', root), 'utf8'));

async function fixture(model) {
    let now = 0, next = 0, reduced = false;
    const tasks = new Map(), events = new Map(), mediaEvents = new Map();
    const schedule = (fn, ms) => { const id = ++next; tasks.set(id, { at: now + ms, fn }); return id; };
    const cancel = id => tasks.delete(id);
    const advance = ms => {
        const end = now + ms;
        for (;;) {
            const task = [...tasks].sort((a, b) => a[1].at - b[1].at)[0];
            if (!task || task[1].at > end) break;
            tasks.delete(task[0]); now = task[1].at; task[1].fn(now);
        }
        now = end;
    };
    class Element {
        style = { setProperty() {} };
        dataset = {};
        classList = { add() {}, remove() {} };
        get offsetLeft() { return parseFloat(this.style.left) || 0; }
        get offsetTop() { return parseFloat(this.style.top) || 0; }
        getBoundingClientRect() { return { left: this.offsetLeft, top: this.offsetTop, width: 192, height: 208, right: this.offsetLeft + 192, bottom: this.offsetTop + 208 }; }
        setAttribute() {}
        removeAttribute() {}
        addEventListener() {}
        appendChild() {}
        remove() {}
        querySelector(selector) { return selector === '.vd-pet-sprite' ? sprite : bubble; }
    }
    const layer = new Element(), sprite = new Element(), bubble = new Element(), motions = [];
    bubble.hidden = true;
    sprite.dataset = new Proxy({}, { set(target, key, value) { target[key] = value; if (key === 'state' && value !== 'idle') motions.push(value); return true; } });
    const settings = { 'pet.enabled': 'true', 'pet.scale': '1', 'pet.position_x': '400', 'pet.position_y': '300', 'pet.gravity': 'false' };
    const id = model ? catalog.rexi.id : catalog.rexi.sprite_id;
    const document = {
        hidden: false, body: Object.assign(new Element(), { dataset: { animations: 'true' } }),
        createElement: () => layer, getElementById: () => null, querySelector: () => null,
        addEventListener: (type, fn) => events.set(type, fn)
    };
    const context = vm.createContext({
        document, innerWidth: 1280, innerHeight: 850, AbortController,
        state: { bootstrap: { pets: [{ id, subcategory: model ? '3d' : '', spritesheet: 'spritesheet.webp' }], active_pet_id: id, settings } },
        settingValue: key => settings[key], t: key => key, shellLayout: () => 'taskbar',
        setTimeout: schedule, clearTimeout: cancel, requestAnimationFrame: fn => schedule(fn, 16), cancelAnimationFrame: cancel,
        addEventListener() {}, matchMedia: () => ({ get matches() { return reduced; }, addEventListener: (type, fn) => mediaEvents.set(type, fn) }),
        Date: class extends Date { static now() { return now; } }, performance: { now: () => now },
        Math: Object.assign(Object.create(Math), { random: () => 0 }),
        AuraLazyAssets: { loadScript: async () => {}, versionedURL: url => url },
        AuraPunkPet3D: { create: async () => ({ setState() {}, duration: () => 1500, dispose() {} }) }
    });
    context.window = context;
    for (const file of ['pet-model-catalog.js', 'pet-runtime.js']) vm.runInContext(readFileSync(new URL('ui/js/desktop/core/' + file, root), 'utf8'), context);
    for (let i = 0; i < 12; i++) await Promise.resolve();
    return { layer, sprite, settings, document, context, advance, motions, reduce(value) { reduced = value; mediaEvents.get('change')(); }, visibility(value) { document.hidden = value; events.get('visibilitychange')(); } };
}

for (const model of [false, true]) {
    const f = await fixture(model), pet = f.context.PetRuntime;
    f.advance(3000); pet.load();
    f.advance(3300);
    assert.equal(f.sprite.dataset.state, 'running-left', 'The first stroll must survive desktop refreshes');
    assert.ok(f.layer.offsetLeft < 400, 'A stroll must move across the desktop');
    const x = f.layer.offsetLeft;
    pet.load();
    assert.equal(f.layer.offsetLeft, x, 'Refresh must not snap to the saved position');
    assert.equal(f.sprite.dataset.state, 'running-left', 'Refresh must not cancel the walking animation');
    f.document.body.dataset.animations = 'false'; f.advance(32);
    assert.equal(f.layer.offsetLeft, x, 'Disabling animations must stop an active stroll');
    f.advance(30000); assert.equal(f.layer.offsetLeft, x);
    f.document.body.dataset.animations = 'true'; pet.load();
    f.advance(4000);
    assert.equal(f.sprite.dataset.state, 'idle');
    f.advance(20000);
    assert.deepEqual(f.motions.slice(0, 3), ['running-left', 'waving', 'running-left'], 'At least every second ambient action must be a stroll');
    f.reduce(true);
    const stopped = f.layer.offsetLeft;
    f.advance(30000); assert.equal(f.layer.offsetLeft, stopped, 'Reduced motion must stop roaming');
    f.reduce(false); f.advance(20000);
    f.visibility(true);
    const hiddenX = f.layer.offsetLeft;
    f.advance(30000); assert.equal(f.layer.offsetLeft, hiddenX, 'Hidden tabs must stop roaming');
    f.visibility(false);
    f.settings['pet.position_x'] = '220'; pet.load();
    assert.equal(f.layer.offsetLeft, 220, 'An explicit position change must still apply');
    f.settings['pet.always_on_top'] = 'true'; pet.load();
    assert.equal(f.layer.dataset.alwaysOnTop, 'true');
    f.settings['pet.enabled'] = 'false'; pet.load();
    assert.equal(f.context.PetRuntime.inspect3D(), null);
}

// Verify 3D activity hooks: expanded ambient pool (no duplicate running), 2D roam interpolation, and onActivity wakeup.
{
    let now = 0, next = 0, observedStates = null, onActivityCb = null, lastState = 'idle';
    const tasks = new Map();
    const schedule = (fn, ms) => { const id = ++next; tasks.set(id, { at: now + ms, fn }); return id; };
    const cancel = id => tasks.delete(id);
    const advance = ms => {
        const end = now + ms;
        for (;;) {
            const task = [...tasks].sort((a, b) => a[1].at - b[1].at)[0];
            if (!task || task[1].at > end) break;
            tasks.delete(task[0]); now = task[1].at; task[1].fn(now);
        }
        now = end;
    };
    class Element {
        style = { setProperty() {} };
        dataset = {};
        classList = { add() {}, remove() {} };
        get offsetLeft() { return parseFloat(this.style.left) || 0; }
        get offsetTop() { return parseFloat(this.style.top) || 0; }
        getBoundingClientRect() { return { left: this.offsetLeft, top: this.offsetTop, width: 192, height: 208, right: this.offsetLeft + 192, bottom: this.offsetTop + 208 }; }
        setAttribute() {}
        removeAttribute() {}
        addEventListener() {}
        appendChild() {}
        remove() {}
        querySelector(selector) { return selector === '.vd-pet-sprite' ? sprite : bubble; }
    }
    const layer = new Element(), sprite = new Element(), bubble = new Element();
    bubble.hidden = true;
    const settings = { 'pet.enabled': 'true', 'pet.scale': '1', 'pet.position_x': '400', 'pet.position_y': '300', 'pet.gravity': 'false' };
    const id = catalog.rexi.id;
    const document = {
        hidden: false, body: Object.assign(new Element(), { dataset: { animations: 'true' } }),
        createElement: () => layer, getElementById: () => null, querySelector: () => null, addEventListener() {}
    };
    const context = vm.createContext({
        document, innerWidth: 1280, innerHeight: 850, AbortController,
        state: { bootstrap: { pets: [{ id, subcategory: '3d', spritesheet: 'spritesheet.webp' }], active_pet_id: id, settings } },
        settingValue: key => settings[key], t: key => key, shellLayout: () => 'taskbar',
        setTimeout: schedule, clearTimeout: cancel, requestAnimationFrame: fn => schedule(fn, 16), cancelAnimationFrame: cancel,
        addEventListener() {}, matchMedia: () => ({ matches: false, addEventListener() {} }),
        Date: class extends Date { static now() { return now; } }, performance: { now: () => now },
        Math: Object.assign(Object.create(Math), { random: () => 0.4 }),
        AuraLazyAssets: { loadScript: async () => {}, versionedURL: url => url },
        AuraPunkPet3D: {
            create: async (_host, opts) => {
                onActivityCb = opts.onActivity;
                return {
                    setState(s) { lastState = s; },
                    duration: () => 1800,
                    dispose() {},
                    pickAmbientState(states) { observedStates = [...states]; return 'dancing'; },
                    planRoam({ startX, startY }) { return { startX, startY, targetX: startX + 180, targetY: startY - 60, deltaX: 180 }; }
                };
            }
        }
    });
    context.window = context;
    for (const file of ['pet-model-catalog.js', 'pet-runtime.js']) vm.runInContext(readFileSync(new URL('ui/js/desktop/core/' + file, root), 'utf8'), context);
    for (let i = 0; i < 12; i++) await Promise.resolve();
    advance(12000);
    assert.equal(layer.offsetLeft, 580, '3D planRoam horizontal destination must be reached');
    assert.equal(layer.offsetTop, 240, '3D planRoam vertical destination must interpolate when gravity is off');
    advance(12000);
    assert.ok(Array.isArray(observedStates) && !observedStates.includes('running'), '3D odd ambient actions must avoid duplicate running entries');
    for (const expected of ['dancing', 'playing', 'celebrating', 'sitting', 'stretching', 'thinking', 'working', 'editing']) {
        assert.ok(observedStates.includes(expected), '3D ambient pool must include ' + expected);
    }
    onActivityCb('celebrating');
    assert.equal(lastState, 'celebrating', '3D petting/activity callback must trigger the requested motion');
}
console.log('Desktop pet roaming: sprite and 3D cadence, refresh continuity, preferences, settings and teardown passed.');
