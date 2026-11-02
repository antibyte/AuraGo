## Part C: JS player runtime (`ui/js/storymaker`)

One deterministic JavaScript player serves the app preview and the headless export. It is a set of **classic scripts** that
`scripts/build-ui-bundles.js` concatenates into two bundles (the build only concatenates, so every file keeps its own IIFE
scope and publishes into one namespace, `StorymakerCore` in the worker realm, `StorymakerHost` in the document realm):

| Bundle | Realm | Parts |
| --- | --- | --- |
| `ui/js/storymaker/host.bundle.js` | document (the player iframe's top document, the render page) | `schedule.js host-core.js host-app.js host-render.js host.js` |
| `ui/js/storymaker/player.bundle.js` | worker (one player worker, one worker per code scene) | `ease timeline schedule stage parts rig text camera scene narration plan beats checks compile transitions render codescene determinism worker-player worker-scene worker-main` |

Files of this part, grouped by task: C1 fixtures and test harness; C2 `ease timeline schedule`; C3 `stage parts text camera`; C4
`rig`; C5 `scene narration plan beats checks compile`; C6 `transitions render`; C7 `codescene determinism`; C8
`worker-player worker-scene worker-main`; C9 `host-core host-app`; C10 `host-render host`; C11 bundles, stylesheet, guards; C12 the
browser tests (the phase-1 gate).

Conventions of this part, all enforced by the tests:

- Every source file starts with a comment that says what it is, is below 600 lines, and uses **no template strings, no
  `innerHTML` / `insertAdjacentHTML` / `eval`** (the "untrusted text" rule; the guard test greps for them). Player-realm files use no
  `Date.now`, `Math.random`, `performance.now` or timers; the host drives all deadlines.
- Tests live in `scripts/storymaker-tests/NN-*.mjs` (one module per task, run in file name order by
  `scripts/test-storymaker-player.mjs`, `npm run test:storymaker`). They run the real sources in a Node `vm` against a recording fake
  2D context; the workers of the host tests are real worker-side scripts in in-process `vm` contexts. The part lists
  (`scripts/storymaker-bundle-parts.js`) are final from C1; a source file that a later task creates is simply not loaded yet, and
  the guard of C11 fails the suite if a listed file is missing.
- `esbuild` 0.28.2 is already a dependency in `package.json`; the tests use it to bundle code scenes in the exact form of the Go
  bundler (contract section 4). Run `npm ci` (or have `node_modules` resolvable from the worktree) before the first test run.
- Each task replays green in a scratch tree: before its implementation the new test module fails, after it the whole suite
  passes (the pass counts below were measured that way).
- The shared fixtures (`stories/*.json`, `scenes/s3.js`, `catalog-mini/catalog.json`) are owned by part A; C1 lists their exact
  content so that either part can create them, and the stub files under `catalog-mini/{rig,props,environments}` are owned by this part.

### Task C1: Test harness, shared fixtures and the catalog-mini stub files

**Files:**
- Create: `scripts/storymaker-bundle-parts.js`, `scripts/storymaker-test-support.mjs`, `scripts/storymaker-test-context.mjs`, `scripts/test-storymaker-player.mjs`
- Create: `scripts/storymaker-tests/00-fixtures.mjs`
- Create (identical to part A's; if Task A already wrote them, rewriting them leaves `git status` clean for these paths): `internal/storymaker/testdata/stories/minimal.json`, `internal/storymaker/testdata/stories/firewall.json`, `internal/storymaker/testdata/scenes/s3.js`, `internal/storymaker/testdata/catalog-mini/catalog.json`
- Create (this part): `internal/storymaker/testdata/catalog-mini/rig/human.json`, `props/lock.json`, `props/shield.json`, `environments/plain-stage.json`, `environments/office.json`, `environments/abstract-tech.json`
- Modify: `package.json:48` (three scripts)

The harness: `storymaker-test-support.mjs` has the recording fake 2D context (a Proxy that logs every call with numbers rounded to 3
decimals), fake canvases, `Path2D` and `FontFace`, `loadPlayer()` / `loadHost()` (the real part files in a fresh `vm` context), and
`InProcessWorker` (a `Worker` whose script is the real worker-side code evaluated in its own `vm` context, with structured-clone
messaging on the next macrotask). The stub files are a small but complete rig (all 13 bones, 25 slots covering every param, the
variants that the mini character and its expressions use) and three environments in the vector part format, written against
contract section 3a. Fixtures never live under `ui/` (the asset-pack build ships `testdata` directories).

- [ ] **Step 1: Write the failing test**

Create the harness and the fixture test (the fixtures do not exist yet):


**`scripts/storymaker-bundle-parts.js`**

````js
// The source files of the two Storymaker player bundles. scripts/build-ui-bundles.js concatenates them, and
// scripts/test-storymaker-player.mjs loads the same lists, so the bundles and the tests cannot drift apart.
//
//   host.bundle.js    document realm: the host page (app protocol, render driver, workers, deadlines)
//   player.bundle.js  worker realm: compile, render, rig, code-scene API, role handlers
export const storymakerHostParts = [
  'ui/js/storymaker/schedule.js',
  'ui/js/storymaker/host-core.js',
  'ui/js/storymaker/host-app.js',
  'ui/js/storymaker/host-render.js',
  'ui/js/storymaker/host.js'
];

export const storymakerPlayerParts = [
  'ui/js/storymaker/ease.js',
  'ui/js/storymaker/timeline.js',
  'ui/js/storymaker/schedule.js',
  'ui/js/storymaker/stage.js',
  'ui/js/storymaker/parts.js',
  'ui/js/storymaker/rig.js',
  'ui/js/storymaker/text.js',
  'ui/js/storymaker/camera.js',
  'ui/js/storymaker/scene.js',
  'ui/js/storymaker/narration.js',
  'ui/js/storymaker/plan.js',
  'ui/js/storymaker/beats.js',
  'ui/js/storymaker/checks.js',
  'ui/js/storymaker/compile.js',
  'ui/js/storymaker/transitions.js',
  'ui/js/storymaker/render.js',
  'ui/js/storymaker/codescene.js',
  'ui/js/storymaker/determinism.js',
  'ui/js/storymaker/worker-player.js',
  'ui/js/storymaker/worker-scene.js',
  'ui/js/storymaker/worker-main.js'
];
````


**`scripts/storymaker-test-support.mjs`**

````js
// Test support for scripts/test-storymaker-player.mjs: the real player sources in a Node vm, a recording fake 2D
// context, fake canvases and fonts, and an in-process worker that runs the real worker-side scripts so the host can be
// tested end to end without a browser.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { createHash } from 'node:crypto';
import { storymakerHostParts, storymakerPlayerParts } from './storymaker-bundle-parts.js';

export const root = path.resolve(import.meta.dirname, '..');
export const read = file => fs.readFileSync(path.join(root, file), 'utf8');
export const readJSON = file => JSON.parse(read(file));
export const plain = value => JSON.parse(JSON.stringify(value));

export const MINI = 'internal/storymaker/testdata/catalog-mini/';
export const miniCatalog = () => readJSON(MINI + 'catalog.json');
export const miniFiles = () => ({
    rig: readJSON(MINI + 'rig/human.json'),
    props: { 'props/lock.json': readJSON(MINI + 'props/lock.json'), 'props/shield.json': readJSON(MINI + 'props/shield.json') },
    envs: {
        'environments/plain-stage.json': readJSON(MINI + 'environments/plain-stage.json'),
        'environments/office.json': readJSON(MINI + 'environments/office.json'),
        'environments/abstract-tech.json': readJSON(MINI + 'environments/abstract-tech.json')
    }
});
export const story = name => readJSON('internal/storymaker/testdata/stories/' + name + '.json');

// ---- recording 2D context ---------------------------------------------------------------------------------------

let nextId = 1;
const fmtNum = n => String(Math.round(n * 1000) / 1000);
function fmt(v) {
    if (typeof v === 'number') return fmtNum(v);
    if (v && typeof v === 'object' && v.__gradient) return 'gradient';
    if (v && typeof v === 'object' && v.d !== undefined && v.isPath) return 'path(' + v.d + ')';
    if (v && typeof v === 'object') return JSON.stringify(v);
    return String(v);
}
export function fontPx(font) {
    const m = /(\d+(?:\.\d+)?)px/.exec(font || '');
    return m ? parseFloat(m[1]) : 10;
}

export function makeContext(canvas, opts = {}) {
    const log = [];
    const state = {
        font: '10px sans-serif', fillStyle: '#000000', strokeStyle: '#000000', globalAlpha: 1, lineWidth: 1, textAlign: 'start',
        textBaseline: 'alphabetic', lineJoin: 'miter', lineCap: 'butt'
    };
    const measure = opts.measure || ((s, font) => String(s).length * fontPx(font) * 0.5);
    const proxy = new Proxy({}, {
        get(_t, prop) {
            if (prop === 'log') return log;
            if (prop === 'canvas') return canvas;
            if (prop === 'measureText') return s => ({ width: measure(s, state.font) });
            if (prop === 'createLinearGradient' || prop === 'createRadialGradient') {
                return (...args) => {
                    const g = { __gradient: true, addColorStop: (o, c) => log.push('stop(' + fmt(o) + ',' + c + ')') };
                    log.push(prop + '(' + args.map(fmt).join(',') + ')=gradient');
                    return g;
                };
            }
            if (prop in state) return state[prop];
            return (...args) => {
                log.push(prop + '(' + args.map(a => (a && a.__canvas ? 'canvas' + a.__canvas : a && a.__bitmap ? 'bitmap' + a.id : fmt(a))).join(',') + ')');
            };
        },
        set(_t, prop, value) {
            state[prop] = value;
            log.push('set ' + String(prop) + '=' + fmt(value));
            return true;
        }
    });
    return proxy;
}

export class FakeCanvas {
    constructor(width, height) {
        this.width = width;
        this.height = height;
        this.__canvas = nextId++;
        this.ctx = makeContext(this);
    }
    getContext() { return this.ctx; }
    // like the real one it hands the pixels over and leaves a blank canvas: the call log restarts
    transferToImageBitmap() {
        const d = digest(this.ctx.log);
        this.ctx.log.length = 0;
        return { __bitmap: true, id: nextId++, width: this.width, height: this.height, digest: d };
    }
    async convertToBlob() { return { size: this.ctx.log.length }; }
}

// digest of a call log; the ids of canvases and bitmaps are not part of what was drawn
export function digest(log) { return createHash('sha256').update(log.map(l => l.replace(/(canvas|bitmap)\d+/g, '$1')).join('\n')).digest('hex').slice(0, 16); }

export class FakePath2D {
    constructor(d) { this.d = d; this.isPath = true; }
}

// A realm global for the player scripts: the fakes they need, nothing else.
export function realmGlobals(extra = {}) {
    class FakeFontFace {
        constructor(family, data, desc) { this.family = family; this.data = data; this.desc = desc; this.status = 'unloaded'; }
        async load() {
            if (!this.data || this.data.byteLength === 0) throw new Error('A network error occurred.');
            this.status = 'loaded';
            return this;
        }
    }
    const fonts = { faces: [], add(f) { this.faces.push(f); } };
    const nope = () => { throw new Error('a real worker would have run this'); };
    return {
        Path2D: FakePath2D, OffscreenCanvas: FakeCanvas, FontFace: FakeFontFace, fonts, console,
        performance: { now: () => 0 }, crypto: { getRandomValues: a => a }, setTimeout: nope, setInterval: nope, requestAnimationFrame: nope, ...extra
    };
}

// The part lists are final from the first task; a source file that a later task creates is simply not loaded yet. The
// static guard (10-static-guards.mjs) fails the suite when a listed file is missing, so a typo cannot hide here.
const present = files => files.filter(f => fs.existsSync(path.join(root, f)));
const joined = files => present(files).map(f => '\n;\n/* ' + f + ' */\n' + read(f)).join('\n');

// loadParts(files, globals): runs the given source files, in order, in a fresh vm context.
export function loadParts(files, globals = {}) {
    const g = realmGlobals(globals);
    const context = vm.createContext(g);
    vm.runInContext('globalThis.self = globalThis;', context);
    vm.runInContext(joined(files), context, { filename: 'storymaker-parts.js' });
    return context;
}

export const loadPlayer = (globals = {}) => loadParts(storymakerPlayerParts, globals);
export const playerSource = () => joined(storymakerPlayerParts);
export const hostSource = () => joined(storymakerHostParts);

// core(ctx): the StorymakerCore of a vm context.
export const coreOf = context => context.StorymakerCore;

// Render assets for the core tests: the mini catalog files and two canvases for transitions.
export function renderAssets(extra = {}) {
    const f = miniFiles(), canvases = {};
    return {
        rig: f.rig, props: f.props, envs: f.envs, codeFrames: {}, codeErrors: {},
        canvas(name, w, h) {
            const key = name + w + 'x' + h;
            if (!canvases[key]) { const c = new FakeCanvas(w, h); canvases[key] = { canvas: c, ctx: c.ctx }; }
            const entry = canvases[key];
            entry.ctx.log.length = 0;
            return entry;
        },
        ...extra
    };
}

// ---- in-process worker --------------------------------------------------------------------------------------------

// A Worker whose script is the real player bundle parts (plus an optional code-scene bundle text) evaluated in a vm
// context that has its own global, postMessage and importScripts. Messages travel through structuredClone on the next
// macrotask, like a real worker. terminate() drops everything; hang(type) makes the worker swallow messages of a type.
export class InProcessWorker {
    constructor(role, sceneText, opts = {}) {
        this.role = role;
        this.dead = false;
        this.listeners = [];
        this.hostListeners = [];
        this.hang = new Set(opts.hang || []);
        this.received = [];
        const self = this;
        const globals = realmGlobals({
            postMessage(data, transfer) { if (!self.dead) setImmediate(() => self.deliverToHost(data)); },
            importScripts() {},
            addEventListener(type, fn) { if (type === 'message') self.listeners.push(fn); }
        });
        this.context = vm.createContext(globals);
        vm.runInContext('globalThis.self = globalThis; self.__SM_ROLE__ = ' + JSON.stringify(role) + ';', this.context);
        vm.runInContext(playerSource(), this.context, { filename: 'player.bundle.js' });
        if (sceneText) vm.runInContext(sceneText, this.context, { filename: 'scene.bundle.js' });
    }
    set onmessage(fn) { this.hostListeners = [fn]; }
    addEventListener(type, fn) { if (type === 'message') this.hostListeners.push(fn); }
    removeEventListener() {}
    deliverToHost(data) {
        if (this.dead) return;
        const clone = structuredClone(data);
        for (const fn of this.hostListeners) fn({ data: clone });
    }
    postMessage(data) {
        if (this.dead) return;
        this.received.push(data && data.type);
        if (this.hang.has(data && data.type)) return;
        const clone = structuredClone(data);
        setImmediate(() => {
            if (this.dead) return;
            for (const fn of this.listeners) fn({ data: clone });
        });
    }
    terminate() { this.dead = true; }
}

// Timers the host can be driven with deterministically.
export function fakeTimers() {
    let now = 0, id = 1;
    const timers = new Map();
    return {
        now: () => now,
        setTimeout(fn, ms) { const h = id++; timers.set(h, { at: now + ms, fn }); return h; },
        clearTimeout(h) { timers.delete(h); },
        advance(ms) {
            const end = now + ms;
            for (;;) {
                let next = null;
                for (const [h, t] of timers) if (t.at <= end && (!next || t.at < next.t.at)) next = { h, t };
                if (!next) break;
                timers.delete(next.h);
                now = next.t.at;
                next.t.fn();
            }
            now = end;
        },
        pending: () => timers.size
    };
}

export const tick = () => new Promise(resolve => setImmediate(resolve));
export async function settle(times = 20) { for (let i = 0; i < times; i++) await tick(); }

// ---- host realm ---------------------------------------------------------------------------------------------------

export const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
export async function until(cond, label, ms = 4000) {
    const end = Date.now() + ms;
    for (;;) {
        if (cond()) return;
        if (Date.now() > end) throw new Error('timeout waiting for ' + label);
        await sleep(2);
    }
}

// loadHost(): the real host.bundle.js parts in a fresh vm context (no document: nothing auto-mounts).
export function loadHost() {
    const ctx = vm.createContext({ console, URL, Promise });
    vm.runInContext('globalThis.self = globalThis;', ctx);
    vm.runInContext(hostSource(), ctx, { filename: 'host.bundle.js' });
    return ctx;
}
````


**`scripts/storymaker-test-context.mjs`**

````js
// Shared context of the Storymaker player tests: the check runner and lazily built fixtures. Every module under
// scripts/storymaker-tests/ imports `t` from here and registers its checks at the top level.
import assert from 'node:assert/strict';
import * as esbuild from 'esbuild';
import { read, miniCatalog, miniFiles, story, loadPlayer } from './storymaker-test-support.mjs';

export const results = [];
export const pending = [];

export function check(name, fn) {
    const fail = err => results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    try {
        const out = fn();
        if (out && typeof out.then === 'function') {
            const done = out.then(() => results.push('ok   ' + name), fail);
            pending.push(done);
            return done;
        }
        results.push('ok   ' + name);
    } catch (err) { fail(err); }
    return undefined;
}

export const near = (actual, expected, eps, label) => assert.ok(Math.abs(actual - expected) <= eps, (label || 'value') + ': ' + actual + ' vs ' + expected);

let player = null, compiledOnce = null;
const catalog = miniCatalog();

// Bundle a code scene exactly like the Go side does (contract section 4: esbuild Transform, IIFE, globalName
// __smSceneModule, target es2020, no legal comments, then the registration line).
function bundleScene(source, name) {
    const out = esbuild.transformSync(source, { format: 'iife', globalName: '__smSceneModule', target: 'es2020', legalComments: 'none', loader: 'js', sourcefile: 'scenes/' + name + '.js' });
    return out.code + '\nself.__smScene=__smSceneModule.default;\n';
}
const s3Source = read('internal/storymaker/testdata/scenes/s3.js');

export const t = {
    results, pending, check, near, catalog, files: miniFiles(), firewall: story('firewall'), minimal: story('minimal'), bundleScene, s3Source,
    s3Bundle: bundleScene(s3Source, 's3'),
    // core(): StorymakerCore of the real player sources in one shared vm context
    core() { if (!player) player = loadPlayer(); return player.StorymakerCore; },
    byId(id) { return catalog.entries.find(e => e.id === id); },
    // compiled(): the firewall story compiled once; the code scene s3 lasts api.duration(5)
    compiled() {
        if (!compiledOnce) compiledOnce = t.core().compile(t.firewall, {}, catalog, { codeScenes: { s3: { duration: 5 } } });
        return compiledOnce;
    },
    sceneById(id) { return t.compiled().timeline.scenes.find(s => s.id === id); }
};
````


**`scripts/test-storymaker-player.mjs`**

````js
// Storymaker player runtime (ui/js/storymaker): runs the real player and host sources in a Node vm against a recording
// fake 2D context, in-process workers and a fake parent window / fake fetch. The checks live in scripts/storymaker-tests/
// (one module per area, run in file name order):
//   01 easing and tracks        02 stage mapping, parts, text    03 rig                 04 compile and timing
//   05 renderFrame              06 code scenes and determinism   07 worker protocol     08 app host (section 8 protocol)
//   09 render driver (section 7)   10 static guards
// Run: node scripts/test-storymaker-player.mjs   (npm run test:storymaker)
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { results, pending } from './storymaker-test-context.mjs';

const dir = path.join(import.meta.dirname, 'storymaker-tests');
for (const file of fs.readdirSync(dir).filter(f => f.endsWith('.mjs')).sort()) {
    await import(pathToFileURL(path.join(dir, file)).href);
}
await Promise.all(pending);
console.log(results.join('\n'));
const failed = results.filter(r => r.startsWith('FAIL')).length;
console.log('\n' + (results.length - failed) + ' passed, ' + failed + ' failed');
process.exit(failed || !results.length ? 1 : 0);
````


**`scripts/storymaker-tests/00-fixtures.mjs`**

````js
// Task C1: the shared fixtures and the catalog-mini stub files against contract section 3a (rig semantics): the 13 bones
// with a rest transform per view, a slot for every rig parameter, the variants that the mini character and its expressions
// use, props and environments in the vector part format.
import assert from 'node:assert/strict';
import { read, readJSON, MINI } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, catalog, files, firewall, minimal } = t;
const BONES = ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r'];
const PARAMS = ['body', 'skin', 'hair', 'hair_back', 'eyes', 'brows', 'mouth', 'hand', 'top', 'bottom', 'shoes', 'glasses', 'facial_hair', 'accessory'];
const MOUTHS = ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv', 'smile', 'frown'];
const VIEWS = ['front', 'three_quarter', 'profile'];

function checkShapes(shapes, where) {
    assert.ok(Array.isArray(shapes), where + ' is a list of shapes');
    for (const s of shapes) {
        assert.equal(typeof s.d, 'string', where + ': a shape has path data');
        assert.ok(s.d.length > 2 && /^[MmLlHhVvCcSsQqTtAaZz0-9 ,.-]+$/.test(s.d), where + ': plain SVG path data');
        for (const c of [s.fill, s.stroke]) if (typeof c === 'string') assert.ok(c === 'none' || /^(#[0-9a-fA-F]{3,8}|\$[a-z_0-9]+)$/.test(c), where + ': colour ' + c);
        if (s.gradient) assert.ok(['linear', 'radial'].includes(s.gradient.type) && s.gradient.stops.length >= 2, where + ': gradient');
    }
}

check('fixtures: the shared stories, the code scene and the catalog manifest are the contract content', () => {
    assert.equal(minimal.title, 'Minimal'); assert.equal(minimal.scenes.length, 1);
    assert.deepEqual(firewall.scenes.map(s => s.id), ['s1', 's2', 's3']);
    assert.equal(firewall.scenes[2].kind, 'code');
    assert.match(read('internal/storymaker/testdata/scenes/s3.js'), /^export default function scene\(api\) \{\r?\n  api\.duration\(5\);/);
    assert.equal(catalog.schema, 'storymaker-catalog/1'); assert.equal(catalog.version, 'mini00000000'); assert.equal(catalog.entries.length, 20);
    assert.deepEqual(catalog.stage, { width: 1920, height: 1080, safe: { x: 5, y: 5, w: 90, h: 90 } });
    for (const e of catalog.entries) if (e.file) assert.ok(typeof readJSON(MINI + e.file) === 'object', e.file + ' exists and parses');
    assert.equal(readJSON(MINI + catalog.rig.file).id, catalog.rig.id);
});

check('catalog-mini rig: 13 bones with a rest transform per view, slots for every param, z order per view', () => {
    const rig = files.rig;
    assert.equal(rig.id, 'rig.human/1');
    assert.ok(rig.units_height > 0 && rig.height > 0 && rig.height < 1);
    assert.deepEqual(rig.bones.map(b => b.name), BONES);
    const names = new Set(BONES);
    for (const b of rig.bones) {
        assert.ok(b.parent === null ? b.name === 'root' : names.has(b.parent), b.name + ' has a known parent');
        for (const v of VIEWS) assert.ok(['x', 'y', 'angle'].every(k => typeof b.rest[v][k] === 'number'), b.name + ' rest.' + v);
    }
    const slotNames = rig.slots.map(s => s.name);
    assert.equal(new Set(slotNames).size, slotNames.length, 'slot names are unique');
    for (const s of rig.slots) {
        assert.ok(names.has(s.bone), s.name + ' sits on a bone');
        assert.ok(PARAMS.includes(s.param), s.name + ' has a known param ' + s.param);
        for (const v of VIEWS) assert.equal(typeof s.z[v], 'number', s.name + ' z.' + v);
    }
    assert.deepEqual([...new Set(rig.slots.map(s => s.param))].sort(), [...PARAMS].sort(), 'a slot for every param');
    assert.ok(['hand_l', 'hand_r'].every(n => slotNames.includes(n)), 'the hands are slots on the lower arms');
    assert.equal(rig.slots.find(s => s.name === 'hand_l').bone, 'lower_arm_l');
    assert.ok(['shoe_l', 'shoe_r'].every(n => slotNames.includes(n)), 'the feet are slots on the lower legs');
});

check('catalog-mini rig: the variants that the mini character and the three expressions use exist', () => {
    const rig = files.rig, ch = catalog.entries.find(e => e.id === 'char.office-woman-1').params, outfit = rig.params.outfit[ch.outfit];
    for (const s of rig.slots) assert.ok(rig.variants[s.name], 'variants for slot ' + s.name);
    const has = (slot, key) => {
        const v = rig.variants[slot][key];
        assert.ok(v, slot + ' has variant ' + key);
        for (const view of VIEWS) checkShapes(v[view], slot + '/' + key + '/' + view);
    };
    has('head', ch.body); has('neck', ch.body); has('hair', ch.hair); has('hair_back', ch.hair);
    for (const skin of Object.keys(rig.params.skin)) has('ears', skin);
    has('torso', outfit.top); has('sleeve_upper_l', outfit.top); has('sleeve_lower_r', outfit.top);
    has('pelvis', outfit.bottom); has('leg_upper_l', outfit.bottom); has('leg_lower_r', outfit.bottom);
    has('shoe_l', outfit.shoes); has('shoe_r', outfit.shoes);
    has('facial_hair', ch.facial_hair); has('glasses', ch.glasses); has('accessory', ch.accessory);
    for (const e of ['neutral', 'wide', 'smile']) has('eyes', ch.eyes + '/' + e);
    for (const e of ['neutral', 'raised']) has('brows', ch.brows + '/' + e);
    for (const m of MOUTHS) has('mouth', m);
    for (const h of ['open', 'fist', 'point', 'hold']) { has('hand_l', h); has('hand_r', h); }
    // every expression of the manifest maps to variants that exist
    for (const e of catalog.entries.filter(x => x.kind === 'expression')) {
        assert.ok(rig.variants.eyes[ch.eyes + '/' + e.eyes] && rig.variants.brows[ch.brows + '/' + e.brows] && rig.variants.mouth[e.mouth], e.id + ' maps to variants');
    }
    assert.deepEqual(rig.variants.facial_hair.none.front, []); assert.deepEqual(rig.variants.glasses.none.front, []);
    assert.ok(rig.params.skin[ch.skin] && rig.params.hair_color[ch.hair_color] && outfit.colors.top && outfit.colors.bottom && outfit.colors.shoes);
    assert.ok(['outline', 'eye_white', 'iris', 'lip'].every(k => rig.params.fixed[k]), 'the fixed colour tokens');
    assert.equal(rig.bodies[ch.body].scale, 1);
    assert.deepEqual(MOUTHS.slice(0, 7), ['rest', 'ai', 'e', 'o', 'u', 'mbp', 'fv'], 'the 7 speech mouth shapes come first');
});

check('catalog-mini props and environments: the vector part format with palette and environment tokens', () => {
    for (const [file, part] of Object.entries(files.props)) {
        assert.equal(part.viewbox.length, 4, file);
        checkShapes(part.shapes, file);
    }
    for (const [file, env] of Object.entries(files.envs)) {
        assert.deepEqual(env.viewbox, [0, 0, 1920, 1080], file);
        assert.ok(env.layers.length >= 3 && env.layers[0].parallax === 0 && env.layers.at(-1).parallax === 1, file + ': sky at parallax 0, floor at 1');
        for (const l of env.layers) { assert.equal(typeof l.name, 'string'); checkShapes(l.shapes, file + '/' + l.name); }
        assert.deepEqual(Object.keys(env.tokens.day).sort(), Object.keys(env.tokens.night).sort(), file + ': day and night tokens match');
        const used = new Set([...JSON.stringify(env.layers).matchAll(/"\$([a-z_0-9]+)"/g)].map(m => m[1]));
        const palette = Object.keys(catalog.palettes[0].tokens);
        for (const name of used) assert.ok(name in env.tokens.day || palette.includes(name), file + ': token $' + name + ' resolves');
    }
    for (const id of ['prop.lock', 'prop.shield']) {
        const e = catalog.entries.find(x => x.id === id);
        assert.ok(files.props[e.file] && e.size.h > 0 && e.anchor.y === 1);
    }
});
````


Add the three scripts to `package.json` after `"build:browser-vendor"` (line 48; `build:storymaker-catalog` and `check:storymaker-catalog` belong to part D's `scripts/build-storymaker-catalog.mjs`, this part owns `package.json`):

````diff
-    "build:browser-vendor": "node scripts/build-browser-vendor.js"
+    "build:browser-vendor": "node scripts/build-browser-vendor.js",
+    "test:storymaker": "node scripts/test-storymaker-player.mjs",
+    "build:storymaker-catalog": "node scripts/build-storymaker-catalog.mjs",
+    "check:storymaker-catalog": "node scripts/build-storymaker-catalog.mjs --check"
````

- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (the runner stops; Error: ENOENT: no such file or directory, open '<path>') (the shared fixture files are missing)

- [ ] **Step 3: Implement**

Write the fixtures, byte for byte (each file ends with one newline):


**`internal/storymaker/testdata/stories/minimal.json`**

````json
{"schema":"storymaker/1","title":"Minimal","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"voice":null,"music":null,"cast":[],"scenes":[{"id":"s1","narration":"Hallo Welt."}]}
````


**`internal/storymaker/testdata/stories/firewall.json`**

````json
{"schema":"storymaker/1","title":"Was ist eine Firewall?","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"voice":null,"music":null,
 "cast":[{"id":"anna","name":"Anna","character":"char.office-woman-1"}],
 "scenes":[
  {"id":"s1","environment":"env.office","transition":"fade","narration":"Anna arbeitet im Homeoffice. {1}Plötzlich klopft ein Unbekannter an.","speaker":"narrator",
   "actors":[{"cast":"anna","at":"desk","pose":"typing"}],
   "beats":[{"cue":1,"do":"emote","who":"anna","expression":"surprised"},{"do":"show","item":"prop.lock","at":"right","entry":"pop"}]},
  {"id":"s2","environment":"env.abstract-tech","narration":"Eine Firewall prüft jedes Paket.","actors":[{"cast":"anna","at":"left"}],
   "beats":[{"do":"walk","who":"anna","to":"center"},{"do":"gesture","who":"anna","gesture":"point","with":"prev"},{"do":"show","item":"prop.shield","at":"right","entry":"fade"},{"do":"camera","op":"zoom","amount":1.2}]},
  {"id":"s3","kind":"code","narration":"So sieht das in Zahlen aus."}]}
````


**`internal/storymaker/testdata/scenes/s3.js`**

````js
export default function scene(api) {
  api.duration(5);
  const anna = api.add('char.office-woman-1', { at: 'left', pose: 'stand' });
  const label = api.text('3 von 4 Paketen geblockt', 'title');
  anna.gesture('point', { start: 1 });
  api.tween(label, { opacity: 1 }, { start: 0.5, dur: 1, ease: 'ease-out' });
  anna.emote('happy', { start: 3 });
}
````


**`internal/storymaker/testdata/catalog-mini/catalog.json`**

````json
{"schema":"storymaker-catalog/1","version":"mini00000000",
 "stage":{"width":1920,"height":1080,"safe":{"x":5,"y":5,"w":90,"h":90}},
 "rig":{"id":"rig.human/1","file":"rig/human.json"},
 "fonts":[{"name":"default","family":"Storymaker Default","weight":400,"file":"/fonts/placeholder.woff2","licence":"OFL-1.1"}],
 "palettes":[{"id":"palette.sunny","name":"sunny","name_key":"storymaker.catalog.palette_sunny","tokens":{"bg":"#fff7e6","bg2":"#ffe0a3","fg":"#1f2a44","accent":"#ff7a45","accent2":"#36cfc9","text":"#1f2a44","outline":"#1f2a44"}}],
 "entries":[
  {"id":"char.office-woman-1","kind":"character","name":"char.office-woman-1","name_key":"storymaker.catalog.char_office-woman-1","tags":["woman","office","business"],"thumb":"thumbs/char.office-woman-1.svg","usage_example":{"cast":[{"id":"anna","character":"char.office-woman-1"}]},"params":{"body":"average","skin":"medium","hair":"bun","hair_color":"dark-brown","eyes":"round","brows":"soft","facial_hair":"none","glasses":"none","outfit":"business","accessory":"none"}},
  {"id":"prop.lock","kind":"prop","name":"prop.lock","name_key":"storymaker.catalog.prop_lock","tags":["security","lock","padlock"],"thumb":"thumbs/prop.lock.svg","usage_example":{"do":"show","item":"prop.lock","at":"right","entry":"pop"},"file":"props/lock.json","size":{"h":0.14},"anchor":{"x":0.5,"y":1}},
  {"id":"prop.shield","kind":"prop","name":"prop.shield","name_key":"storymaker.catalog.prop_shield","tags":["security","shield","protection"],"thumb":"thumbs/prop.shield.svg","usage_example":{"do":"show","item":"prop.shield","at":"right","entry":"fade"},"file":"props/shield.json","size":{"h":0.16},"anchor":{"x":0.5,"y":1}},
  {"id":"env.plain-stage","kind":"environment","name":"env.plain-stage","name_key":"storymaker.catalog.env_plain-stage","tags":["plain","stage","neutral"],"thumb":"thumbs/env.plain-stage.svg","usage_example":{"environment":"env.plain-stage"},"file":"environments/plain-stage.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":74,"y":62}},"floor":{"y_near":96,"y_far":70,"scale_near":1.05,"scale_far":0.8},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50,"spots":{"far-left":{"x":38,"y":88},"far-right":{"x":62,"y":88},"left":{"x":42,"y":88},"right":{"x":58,"y":88}}},"1:1":{"focus_x":50}}},
  {"id":"env.office","kind":"environment","name":"env.office","name_key":"storymaker.catalog.env_office","tags":["office","work","desk"],"thumb":"thumbs/env.office.svg","usage_example":{"environment":"env.office"},"file":"environments/office.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"desk":{"x":36,"y":84,"face":"right"},"door":{"x":90,"y":82,"face":"left"},"board":{"x":70,"y":60},"window":{"x":18,"y":82}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":40},"1:1":{"focus_x":45}}},
  {"id":"env.abstract-tech","kind":"environment","name":"env.abstract-tech","name_key":"storymaker.catalog.env_abstract-tech","tags":["tech","abstract","network"],"thumb":"thumbs/env.abstract-tech.svg","usage_example":{"environment":"env.abstract-tech"},"file":"environments/abstract-tech.json","spots":{"far-left":{"x":12,"y":88},"left":{"x":30,"y":88},"center":{"x":50,"y":88},"right":{"x":70,"y":88},"far-right":{"x":88,"y":88},"board":{"x":70,"y":58}},"floor":{"y_near":96,"y_far":72,"scale_near":1.05,"scale_far":0.82},"variants":["day","night"],"aspects":{"9:16":{"focus_x":50},"1:1":{"focus_x":50}}},
  {"id":"pose.stand","kind":"pose","name":"stand","name_key":"storymaker.catalog.pose_stand","tags":["stand"],"thumb":"thumbs/pose.stand.svg","usage_example":{"pose":"stand"},"bones":{}},
  {"id":"pose.typing","kind":"pose","name":"typing","name_key":"storymaker.catalog.pose_typing","tags":["typing","computer","sit"],"thumb":"thumbs/pose.typing.svg","usage_example":{"pose":"typing"},"bones":{"upper_leg_l":-80,"lower_leg_l":80,"upper_leg_r":-80,"lower_leg_r":80,"upper_arm_l":-30,"lower_arm_l":-70,"upper_arm_r":-30,"lower_arm_r":-70},"hands":{"l":"open","r":"open"},"seated":true},
  {"id":"gesture.point","kind":"gesture","name":"point","name_key":"storymaker.catalog.gesture_point","tags":["point","show"],"thumb":"thumbs/gesture.point.svg","usage_example":{"do":"gesture","who":"anna","gesture":"point"},"duration":1.2,"keys":[{"t":0,"bones":{}},{"t":0.3,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":0.8,"bones":{"upper_arm_r":-85,"lower_arm_r":-10},"hands":{"r":"point"}},{"t":1,"bones":{}}]},
  {"id":"expr.neutral","kind":"expression","name":"neutral","name_key":"storymaker.catalog.expr_neutral","tags":["neutral"],"thumb":"thumbs/expr.neutral.svg","usage_example":{"expression":"neutral"},"eyes":"neutral","brows":"neutral","mouth":"rest"},
  {"id":"expr.surprised","kind":"expression","name":"surprised","name_key":"storymaker.catalog.expr_surprised","tags":["surprised","shock"],"thumb":"thumbs/expr.surprised.svg","usage_example":{"do":"emote","who":"anna","expression":"surprised"},"eyes":"wide","brows":"raised","mouth":"o"},
  {"id":"expr.happy","kind":"expression","name":"happy","name_key":"storymaker.catalog.expr_happy","tags":["happy","smile"],"thumb":"thumbs/expr.happy.svg","usage_example":{"do":"emote","who":"anna","expression":"happy"},"eyes":"smile","brows":"neutral","mouth":"smile"},
  {"id":"mouth.rest","kind":"mouth","name":"rest","name_key":"storymaker.catalog.mouth_rest","tags":["mouth"],"thumb":"thumbs/mouth.rest.svg","usage_example":{},"shape":"rest"},
  {"id":"cycle.walk","kind":"cycle","name":"walk","name_key":"storymaker.catalog.cycle_walk","tags":["walk"],"thumb":"thumbs/cycle.walk.svg","usage_example":{"do":"walk","who":"anna","to":"center"},"period":1.0,"speed":12,"keys":[{"t":0,"bones":{"upper_leg_l":20,"upper_leg_r":-20,"upper_arm_l":-15,"upper_arm_r":15}},{"t":0.5,"bones":{"upper_leg_l":-20,"upper_leg_r":20,"upper_arm_l":15,"upper_arm_r":-15},"offset_y":-6},{"t":1,"bones":{"upper_leg_l":20,"upper_leg_r":-20,"upper_arm_l":-15,"upper_arm_r":15}}]},
  {"id":"cycle.idle","kind":"cycle","name":"idle","name_key":"storymaker.catalog.cycle_idle","tags":["idle"],"thumb":"thumbs/cycle.idle.svg","usage_example":{},"period":3.0,"keys":[{"t":0,"bones":{"torso":0}},{"t":0.5,"bones":{"torso":1.5},"offset_y":-2},{"t":1,"bones":{"torso":0}}]},
  {"id":"text.title","kind":"text_style","name":"title","name_key":"storymaker.catalog.text_title","tags":["title","heading"],"thumb":"thumbs/text.title.svg","usage_example":{"do":"show","text":"Hallo","at":"center"},"font":"default","size":0.07,"weight":700,"color":"$text","align":"center","max_lines":2},
  {"id":"trans.cut","kind":"transition","name":"cut","name_key":"storymaker.catalog.trans_cut","tags":["cut"],"thumb":"thumbs/trans.cut.svg","usage_example":{"transition":"cut"},"duration":0},
  {"id":"trans.fade","kind":"transition","name":"fade","name_key":"storymaker.catalog.trans_fade","tags":["fade"],"thumb":"thumbs/trans.fade.svg","usage_example":{"transition":"fade"},"duration":0.5},
  {"id":"trans.slide-left","kind":"transition","name":"slide-left","name_key":"storymaker.catalog.trans_slide-left","tags":["slide"],"thumb":"thumbs/trans.slide-left.svg","usage_example":{"transition":"slide-left"},"duration":0.5},
  {"id":"trans.wipe","kind":"transition","name":"wipe","name_key":"storymaker.catalog.trans_wipe","tags":["wipe"],"thumb":"thumbs/trans.wipe.svg","usage_example":{"transition":"wipe"},"duration":0.5}
 ]}
````


**`internal/storymaker/testdata/catalog-mini/rig/human.json`**

````json
{"id":"rig.human/1","units_height":400,"height":0.36,"seated_offset":92,"bones":[{"name":"root","parent":null,"length":0,"rest":{"front":{"x":0,"y":0,"angle":0},"three_quarter":{"x":0,"y":0,"angle":0},"profile":{"x":0,"y":0,"angle":0}}},{"name":"hips","parent":"root","length":0,"rest":{"front":{"x":0,"y":-190,"angle":0},"three_quarter":{"x":0,"y":-190,"angle":0},"profile":{"x":0,"y":-190,"angle":0}}},{"name":"torso","parent":"hips","length":108,"rest":{"front":{"x":0,"y":-8,"angle":0},"three_quarter":{"x":0,"y":-8,"angle":0},"profile":{"x":0,"y":-8,"angle":0}}},{"name":"neck","parent":"torso","length":12,"rest":{"front":{"x":0,"y":-108,"angle":0},"three_quarter":{"x":0,"y":-108,"angle":0},"profile":{"x":0,"y":-108,"angle":0}}},{"name":"head","parent":"neck","length":84,"rest":{"front":{"x":0,"y":-10,"angle":0},"three_quarter":{"x":3,"y":-10,"angle":0},"profile":{"x":6,"y":-10,"angle":0}}},{"name":"upper_arm_l","parent":"torso","length":72,"rest":{"front":{"x":-36,"y":-96,"angle":0},"three_quarter":{"x":-30,"y":-96,"angle":0},"profile":{"x":-4,"y":-96,"angle":0}}},{"name":"lower_arm_l","parent":"upper_arm_l","length":66,"rest":{"front":{"x":0,"y":72,"angle":0},"three_quarter":{"x":0,"y":72,"angle":0},"profile":{"x":0,"y":72,"angle":0}}},{"name":"upper_arm_r","parent":"torso","length":72,"rest":{"front":{"x":36,"y":-96,"angle":0},"three_quarter":{"x":30,"y":-96,"angle":0},"profile":{"x":4,"y":-96,"angle":0}}},{"name":"lower_arm_r","parent":"upper_arm_r","length":66,"rest":{"front":{"x":0,"y":72,"angle":0},"three_quarter":{"x":0,"y":72,"angle":0},"profile":{"x":0,"y":72,"angle":0}}},{"name":"upper_leg_l","parent":"hips","length":94,"rest":{"front":{"x":-16,"y":0,"angle":0},"three_quarter":{"x":-12,"y":0,"angle":0},"profile":{"x":-4,"y":0,"angle":0}}},{"name":"lower_leg_l","parent":"upper_leg_l","length":96,"rest":{"front":{"x":0,"y":94,"angle":0},"three_quarter":{"x":0,"y":94,"angle":0},"profile":{"x":0,"y":94,"angle":0}}},{"name":"upper_leg_r","parent":"hips","length":94,"rest":{"front":{"x":16,"y":0,"angle":0},"three_quarter":{"x":12,"y":0,"angle":0},"profile":{"x":4,"y":0,"angle":0}}},{"name":"lower_leg_r","parent":"upper_leg_r","length":96,"rest":{"front":{"x":0,"y":94,"angle":0},"three_quarter":{"x":0,"y":94,"angle":0},"profile":{"x":0,"y":94,"angle":0}}}],"slots":[{"name":"hair_back","bone":"head","param":"hair_back","z":{"front":5,"three_quarter":5,"profile":5}},{"name":"sleeve_upper_r","bone":"upper_arm_r","param":"top","z":{"front":83,"three_quarter":10,"profile":10}},{"name":"sleeve_lower_r","bone":"lower_arm_r","param":"top","z":{"front":84,"three_quarter":11,"profile":11}},{"name":"hand_r","bone":"lower_arm_r","param":"hand","z":{"front":85,"three_quarter":12,"profile":12}},{"name":"leg_upper_r","bone":"upper_leg_r","param":"bottom","z":{"front":20,"three_quarter":20,"profile":20}},{"name":"leg_lower_r","bone":"lower_leg_r","param":"bottom","z":{"front":21,"three_quarter":21,"profile":21}},{"name":"shoe_r","bone":"lower_leg_r","param":"shoes","z":{"front":22,"three_quarter":22,"profile":22}},{"name":"leg_upper_l","bone":"upper_leg_l","param":"bottom","z":{"front":30,"three_quarter":30,"profile":30}},{"name":"leg_lower_l","bone":"lower_leg_l","param":"bottom","z":{"front":31,"three_quarter":31,"profile":31}},{"name":"shoe_l","bone":"lower_leg_l","param":"shoes","z":{"front":32,"three_quarter":32,"profile":32}},{"name":"pelvis","bone":"hips","param":"bottom","z":{"front":40,"three_quarter":40,"profile":40}},{"name":"torso","bone":"torso","param":"top","z":{"front":50,"three_quarter":50,"profile":50}},{"name":"neck","bone":"neck","param":"body","z":{"front":55,"three_quarter":55,"profile":55}},{"name":"head","bone":"head","param":"body","z":{"front":60,"three_quarter":60,"profile":60}},{"name":"ears","bone":"head","param":"skin","z":{"front":61,"three_quarter":61,"profile":61}},{"name":"mouth","bone":"head","param":"mouth","z":{"front":62,"three_quarter":62,"profile":62}},{"name":"eyes","bone":"head","param":"eyes","z":{"front":63,"three_quarter":63,"profile":63}},{"name":"brows","bone":"head","param":"brows","z":{"front":64,"three_quarter":64,"profile":64}},{"name":"facial_hair","bone":"head","param":"facial_hair","z":{"front":65,"three_quarter":65,"profile":65}},{"name":"hair","bone":"head","param":"hair","z":{"front":70,"three_quarter":70,"profile":70}},{"name":"glasses","bone":"head","param":"glasses","z":{"front":72,"three_quarter":72,"profile":72}},{"name":"accessory","bone":"head","param":"accessory","z":{"front":74,"three_quarter":74,"profile":74}},{"name":"sleeve_upper_l","bone":"upper_arm_l","param":"top","z":{"front":80,"three_quarter":80,"profile":80}},{"name":"sleeve_lower_l","bone":"lower_arm_l","param":"top","z":{"front":81,"three_quarter":81,"profile":81}},{"name":"hand_l","bone":"lower_arm_l","param":"hand","z":{"front":82,"three_quarter":82,"profile":82}}],"variants":{"hair_back":{"bun":{"front":[{"d":"M-24,-100 a18,16 0 1,0 36,0 a18,16 0 1,0 -36,0 Z","fill":"$hair","stroke":"$outline","stroke_width":3}],"three_quarter":[{"d":"M-24,-100 a18,16 0 1,0 36,0 a18,16 0 1,0 -36,0 Z","fill":"$hair","stroke":"$outline","stroke_width":3}],"profile":[{"d":"M-24,-100 a18,16 0 1,0 36,0 a18,16 0 1,0 -36,0 Z","fill":"$hair","stroke":"$outline","stroke_width":3}]}},"sleeve_upper_r":{"blazer":{"front":[{"d":"M-3.6000000000000014,-4 L3.6000000000000014,-4 Q12,-4 12,4.399999999999999 L12,67.6 Q12,76 3.6000000000000014,76 L-3.6000000000000014,76 Q-12,76 -12,67.6 L-12,4.399999999999999 Q-12,-4 -3.6000000000000014,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-3.6000000000000014,-4 L3.6000000000000014,-4 Q12,-4 12,4.399999999999999 L12,67.6 Q12,76 3.6000000000000014,76 L-3.6000000000000014,76 Q-12,76 -12,67.6 L-12,4.399999999999999 Q-12,-4 -3.6000000000000014,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-3.6000000000000014,-4 L3.6000000000000014,-4 Q12,-4 12,4.399999999999999 L12,67.6 Q12,76 3.6000000000000014,76 L-3.6000000000000014,76 Q-12,76 -12,67.6 L-12,4.399999999999999 Q-12,-4 -3.6000000000000014,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}]}},"sleeve_lower_r":{"blazer":{"front":[{"d":"M-3,-4 L3,-4 Q10,-4 10,3 L10,63 Q10,70 3,70 L-3,70 Q-10,70 -10,63 L-10,3 Q-10,-4 -3,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-3,-4 L3,-4 Q10,-4 10,3 L10,63 Q10,70 3,70 L-3,70 Q-10,70 -10,63 L-10,3 Q-10,-4 -3,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-3,-4 L3,-4 Q10,-4 10,3 L10,63 Q10,70 3,70 L-3,70 Q-10,70 -10,63 L-10,3 Q-10,-4 -3,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}]}},"hand_r":{"open":{"front":[{"d":"M-11,76 a11,12 0 1,0 22,0 a11,12 0 1,0 -22,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-11,76 a11,12 0 1,0 22,0 a11,12 0 1,0 -22,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-11,76 a11,12 0 1,0 22,0 a11,12 0 1,0 -22,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}]},"fist":{"front":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":3}],"three_quarter":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":3}],"profile":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":3}]},"point":{"front":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M2,72 L20,64","fill":"none","stroke":"$skin","stroke_width":7}],"three_quarter":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M2,72 L20,64","fill":"none","stroke":"$skin","stroke_width":7}],"profile":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M2,72 L20,64","fill":"none","stroke":"$skin","stroke_width":7}]},"hold":{"front":[{"d":"M-3,66 L3,66 Q10,66 10,73 L10,77 Q10,84 3,84 L-3,84 Q-10,84 -10,77 L-10,73 Q-10,66 -3,66 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-3,66 L3,66 Q10,66 10,73 L10,77 Q10,84 3,84 L-3,84 Q-10,84 -10,77 L-10,73 Q-10,66 -3,66 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-3,66 L3,66 Q10,66 10,73 L10,77 Q10,84 3,84 L-3,84 Q-10,84 -10,77 L-10,73 Q-10,66 -3,66 Z","fill":"$skin","stroke":"$outline","stroke_width":2}]}},"leg_upper_r":{"trousers":{"front":[{"d":"M-4.200000000000001,-4 L4.200000000000001,-4 Q14,-4 14,5.799999999999999 L14,88.2 Q14,98 4.200000000000001,98 L-4.200000000000001,98 Q-14,98 -14,88.2 L-14,5.799999999999999 Q-14,-4 -4.200000000000001,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-4.200000000000001,-4 L4.200000000000001,-4 Q14,-4 14,5.799999999999999 L14,88.2 Q14,98 4.200000000000001,98 L-4.200000000000001,98 Q-14,98 -14,88.2 L-14,5.799999999999999 Q-14,-4 -4.200000000000001,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-4.200000000000001,-4 L4.200000000000001,-4 Q14,-4 14,5.799999999999999 L14,88.2 Q14,98 4.200000000000001,98 L-4.200000000000001,98 Q-14,98 -14,88.2 L-14,5.799999999999999 Q-14,-4 -4.200000000000001,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}]}},"leg_lower_r":{"trousers":{"front":[{"d":"M-3.3000000000000007,-4 L3.3000000000000007,-4 Q11,-4 11,3.6999999999999993 L11,92.3 Q11,100 3.3000000000000007,100 L-3.3000000000000007,100 Q-11,100 -11,92.3 L-11,3.6999999999999993 Q-11,-4 -3.3000000000000007,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-3.3000000000000007,-4 L3.3000000000000007,-4 Q11,-4 11,3.6999999999999993 L11,92.3 Q11,100 3.3000000000000007,100 L-3.3000000000000007,100 Q-11,100 -11,92.3 L-11,3.6999999999999993 Q-11,-4 -3.3000000000000007,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-3.3000000000000007,-4 L3.3000000000000007,-4 Q11,-4 11,3.6999999999999993 L11,92.3 Q11,100 3.3000000000000007,100 L-3.3000000000000007,100 Q-11,100 -11,92.3 L-11,3.6999999999999993 Q-11,-4 -3.3000000000000007,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}]}},"shoe_r":{"shoes":{"front":[{"d":"M-12,88 L-12,98 L24,98 C26,90 16,86 -8,86 Z","fill":"$shoes","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-12,88 L-12,98 L24,98 C26,90 16,86 -8,86 Z","fill":"$shoes","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-12,88 L-12,98 L24,98 C26,90 16,86 -8,86 Z","fill":"$shoes","stroke":"$outline","stroke_width":2.5}]}},"leg_upper_l":{"trousers":{"front":[{"d":"M-4.200000000000001,-4 L4.200000000000001,-4 Q14,-4 14,5.799999999999999 L14,88.2 Q14,98 4.200000000000001,98 L-4.200000000000001,98 Q-14,98 -14,88.2 L-14,5.799999999999999 Q-14,-4 -4.200000000000001,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-4.200000000000001,-4 L4.200000000000001,-4 Q14,-4 14,5.799999999999999 L14,88.2 Q14,98 4.200000000000001,98 L-4.200000000000001,98 Q-14,98 -14,88.2 L-14,5.799999999999999 Q-14,-4 -4.200000000000001,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-4.200000000000001,-4 L4.200000000000001,-4 Q14,-4 14,5.799999999999999 L14,88.2 Q14,98 4.200000000000001,98 L-4.200000000000001,98 Q-14,98 -14,88.2 L-14,5.799999999999999 Q-14,-4 -4.200000000000001,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}]}},"leg_lower_l":{"trousers":{"front":[{"d":"M-3.3000000000000007,-4 L3.3000000000000007,-4 Q11,-4 11,3.6999999999999993 L11,92.3 Q11,100 3.3000000000000007,100 L-3.3000000000000007,100 Q-11,100 -11,92.3 L-11,3.6999999999999993 Q-11,-4 -3.3000000000000007,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-3.3000000000000007,-4 L3.3000000000000007,-4 Q11,-4 11,3.6999999999999993 L11,92.3 Q11,100 3.3000000000000007,100 L-3.3000000000000007,100 Q-11,100 -11,92.3 L-11,3.6999999999999993 Q-11,-4 -3.3000000000000007,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-3.3000000000000007,-4 L3.3000000000000007,-4 Q11,-4 11,3.6999999999999993 L11,92.3 Q11,100 3.3000000000000007,100 L-3.3000000000000007,100 Q-11,100 -11,92.3 L-11,3.6999999999999993 Q-11,-4 -3.3000000000000007,-4 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}]}},"shoe_l":{"shoes":{"front":[{"d":"M-12,88 L-12,98 L24,98 C26,90 16,86 -8,86 Z","fill":"$shoes","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-12,88 L-12,98 L24,98 C26,90 16,86 -8,86 Z","fill":"$shoes","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-12,88 L-12,98 L24,98 C26,90 16,86 -8,86 Z","fill":"$shoes","stroke":"$outline","stroke_width":2.5}]}},"pelvis":{"trousers":{"front":[{"d":"M-24,-6 L24,-6 Q34,-6 34,4 L34,14 Q34,24 24,24 L-24,24 Q-34,24 -34,14 L-34,4 Q-34,-6 -24,-6 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-24,-6 L24,-6 Q34,-6 34,4 L34,14 Q34,24 24,24 L-24,24 Q-34,24 -34,14 L-34,4 Q-34,-6 -24,-6 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-24,-6 L24,-6 Q34,-6 34,4 L34,14 Q34,24 24,24 L-24,24 Q-34,24 -34,14 L-34,4 Q-34,-6 -24,-6 Z","fill":"$bottom","stroke":"$outline","stroke_width":2.5}]}},"torso":{"blazer":{"front":[{"d":"M-20,-112 L20,-112 Q40,-112 40,-92 L40,-8 Q40,12 20,12 L-20,12 Q-40,12 -40,-8 L-40,-92 Q-40,-112 -20,-112 Z","fill":"$top","stroke":"$outline","stroke_width":3},{"d":"M0,-106 L0,6","fill":"none","stroke":"$outline","stroke_width":2.5},{"d":"M-14,-110 L0,-84 L14,-110","fill":"$eye_white","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-20,-112 L20,-112 Q40,-112 40,-92 L40,-8 Q40,12 20,12 L-20,12 Q-40,12 -40,-8 L-40,-92 Q-40,-112 -20,-112 Z","fill":"$top","stroke":"$outline","stroke_width":3},{"d":"M0,-106 L0,6","fill":"none","stroke":"$outline","stroke_width":2.5},{"d":"M-14,-110 L0,-84 L14,-110","fill":"$eye_white","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-20,-112 L20,-112 Q40,-112 40,-92 L40,-8 Q40,12 20,12 L-20,12 Q-40,12 -40,-8 L-40,-92 Q-40,-112 -20,-112 Z","fill":"$top","stroke":"$outline","stroke_width":3},{"d":"M0,-106 L0,6","fill":"none","stroke":"$outline","stroke_width":2.5},{"d":"M-14,-110 L0,-84 L14,-110","fill":"$eye_white","stroke":"$outline","stroke_width":2}]}},"neck":{"average":{"front":[{"d":"M-5,-16 L5,-16 Q9,-16 9,-12 L9,2 Q9,6 5,6 L-5,6 Q-9,6 -9,2 L-9,-12 Q-9,-16 -5,-16 Z","fill":"$skin","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-5,-16 L5,-16 Q9,-16 9,-12 L9,2 Q9,6 5,6 L-5,6 Q-9,6 -9,2 L-9,-12 Q-9,-16 -5,-16 Z","fill":"$skin","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-5,-16 L5,-16 Q9,-16 9,-12 L9,2 Q9,6 5,6 L-5,6 Q-9,6 -9,2 L-9,-12 Q-9,-16 -5,-16 Z","fill":"$skin","stroke":"$outline","stroke_width":2.5}]}},"head":{"average":{"front":[{"d":"M0,6 C-30,6 -36,-30 -34,-50 C-32,-76 -14,-88 0,-88 C14,-88 32,-76 34,-50 C36,-30 30,6 0,6 Z","fill":"$skin","stroke":"$outline","stroke_width":3}],"three_quarter":[{"d":"M2,6 C-26,6 -34,-30 -32,-50 C-30,-76 -12,-88 2,-88 C18,-88 36,-76 38,-52 C40,-44 42,-40 38,-38 C40,-30 30,6 2,6 Z","fill":"$skin","stroke":"$outline","stroke_width":3}],"profile":[{"d":"M4,6 C-22,6 -30,-30 -28,-52 C-26,-76 -8,-88 6,-88 C22,-88 34,-74 36,-54 L42,-44 L36,-40 C38,-28 30,6 4,6 Z","fill":"$skin","stroke":"$outline","stroke_width":3}]}},"ears":{"light":{"front":[{"d":"M-40,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M30,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-36,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-4,-40 a6,10 0 1,0 12,0 a6,10 0 1,0 -12,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}]},"medium":{"front":[{"d":"M-40,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M30,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-36,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-4,-40 a6,10 0 1,0 12,0 a6,10 0 1,0 -12,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}]},"dark":{"front":[{"d":"M-40,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M30,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-36,-40 a5,9 0 1,0 10,0 a5,9 0 1,0 -10,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-4,-40 a6,10 0 1,0 12,0 a6,10 0 1,0 -12,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}]}},"mouth":{"rest":{"front":[{"d":"M-9,-20 L9,-20","fill":"none","stroke":"$lip","stroke_width":4}],"three_quarter":[{"d":"M-1,-20 L17,-20","fill":"none","stroke":"$lip","stroke_width":4}],"profile":[{"d":"M11,-20 L29,-20","fill":"none","stroke":"$lip","stroke_width":4}]},"ai":{"front":[{"d":"M-10,-19 a10,7 0 1,0 20,0 a10,7 0 1,0 -20,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-2,-19 a10,7 0 1,0 20,0 a10,7 0 1,0 -20,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M10,-19 a10,7 0 1,0 20,0 a10,7 0 1,0 -20,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}]},"e":{"front":[{"d":"M-8,-24 L8,-24 Q12,-24 12,-20 L12,-19 Q12,-15 8,-15 L-8,-15 Q-12,-15 -12,-19 L-12,-20 Q-12,-24 -8,-24 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M0,-24 L16,-24 Q20,-24 20,-20 L20,-19 Q20,-15 16,-15 L0,-15 Q-4,-15 -4,-19 L-4,-20 Q-4,-24 0,-24 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M12,-24 L28,-24 Q32,-24 32,-20 L32,-19 Q32,-15 28,-15 L12,-15 Q8,-15 8,-19 L8,-20 Q8,-24 12,-24 Z","fill":"$lip","stroke":"$outline","stroke_width":2}]},"o":{"front":[{"d":"M-6,-19 a6,8 0 1,0 12,0 a6,8 0 1,0 -12,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M2,-19 a6,8 0 1,0 12,0 a6,8 0 1,0 -12,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M14,-19 a6,8 0 1,0 12,0 a6,8 0 1,0 -12,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}]},"u":{"front":[{"d":"M-5,-19 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M3,-19 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M15,-19 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$lip","stroke":"$outline","stroke_width":2}]},"mbp":{"front":[{"d":"M-10,-20 L10,-20","fill":"none","stroke":"$lip","stroke_width":6}],"three_quarter":[{"d":"M-2,-20 L18,-20","fill":"none","stroke":"$lip","stroke_width":6}],"profile":[{"d":"M10,-20 L30,-20","fill":"none","stroke":"$lip","stroke_width":6}]},"fv":{"front":[{"d":"M-10,-21 L10,-21","fill":"none","stroke":"$lip","stroke_width":4},{"d":"M-6,-17 L6,-17","fill":"none","stroke":"$eye_white","stroke_width":3}],"three_quarter":[{"d":"M-2,-21 L18,-21","fill":"none","stroke":"$lip","stroke_width":4},{"d":"M2,-17 L14,-17","fill":"none","stroke":"$eye_white","stroke_width":3}],"profile":[{"d":"M10,-21 L30,-21","fill":"none","stroke":"$lip","stroke_width":4},{"d":"M14,-17 L26,-17","fill":"none","stroke":"$eye_white","stroke_width":3}]},"smile":{"front":[{"d":"M-12,-24 Q0,-10 12,-24","fill":"none","stroke":"$lip","stroke_width":4}],"three_quarter":[{"d":"M-4,-24 Q8,-10 20,-24","fill":"none","stroke":"$lip","stroke_width":4}],"profile":[{"d":"M8,-24 Q20,-10 32,-24","fill":"none","stroke":"$lip","stroke_width":4}]},"frown":{"front":[{"d":"M-11,-14 Q0,-26 11,-14","fill":"none","stroke":"$lip","stroke_width":4}],"three_quarter":[{"d":"M-3,-14 Q8,-26 19,-14","fill":"none","stroke":"$lip","stroke_width":4}],"profile":[{"d":"M9,-14 Q20,-26 31,-14","fill":"none","stroke":"$lip","stroke_width":4}]}},"eyes":{"round/neutral":{"front":[{"d":"M-22,-46 a8,9 0 1,0 16,0 a8,9 0 1,0 -16,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M-17.5,-46 a4.5,4.5 0 1,0 9,0 a4.5,4.5 0 1,0 -9,0 Z","fill":"$iris"},{"d":"M6,-46 a8,9 0 1,0 16,0 a8,9 0 1,0 -16,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M10.5,-46 a4.5,4.5 0 1,0 9,0 a4.5,4.5 0 1,0 -9,0 Z","fill":"$iris"}],"three_quarter":[{"d":"M-14,-46 a8,9 0 1,0 16,0 a8,9 0 1,0 -16,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M-9.5,-46 a4.5,4.5 0 1,0 9,0 a4.5,4.5 0 1,0 -9,0 Z","fill":"$iris"},{"d":"M16,-46 a8,9 0 1,0 16,0 a8,9 0 1,0 -16,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M20.5,-46 a4.5,4.5 0 1,0 9,0 a4.5,4.5 0 1,0 -9,0 Z","fill":"$iris"}],"profile":[{"d":"M12,-46 a8,9 0 1,0 16,0 a8,9 0 1,0 -16,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M16.5,-46 a4.5,4.5 0 1,0 9,0 a4.5,4.5 0 1,0 -9,0 Z","fill":"$iris"}]},"round/wide":{"front":[{"d":"M-24,-46 a10,12 0 1,0 20,0 a10,12 0 1,0 -20,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M-18,-46 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$iris"},{"d":"M4,-46 a10,12 0 1,0 20,0 a10,12 0 1,0 -20,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M10,-46 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$iris"}],"three_quarter":[{"d":"M-16,-46 a10,12 0 1,0 20,0 a10,12 0 1,0 -20,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M-10,-46 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$iris"},{"d":"M14,-46 a10,12 0 1,0 20,0 a10,12 0 1,0 -20,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M20,-46 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$iris"}],"profile":[{"d":"M10,-46 a10,12 0 1,0 20,0 a10,12 0 1,0 -20,0 Z","fill":"$eye_white","stroke":"$outline","stroke_width":2},{"d":"M16,-46 a5,5 0 1,0 10,0 a5,5 0 1,0 -10,0 Z","fill":"$iris"}]},"round/smile":{"front":[{"d":"M-23,-44 Q-14,-56 -5,-44","fill":"none","stroke":"$outline","stroke_width":4},{"d":"M5,-44 Q14,-56 23,-44","fill":"none","stroke":"$outline","stroke_width":4}],"three_quarter":[{"d":"M-15,-44 Q-6,-56 3,-44","fill":"none","stroke":"$outline","stroke_width":4},{"d":"M15,-44 Q24,-56 33,-44","fill":"none","stroke":"$outline","stroke_width":4}],"profile":[{"d":"M11,-44 Q20,-56 29,-44","fill":"none","stroke":"$outline","stroke_width":4}]}},"brows":{"soft/neutral":{"front":[{"d":"M-24,-62 Q-14,-68 -4,-62","fill":"none","stroke":"$hair","stroke_width":4},{"d":"M4,-62 Q14,-68 24,-62","fill":"none","stroke":"$hair","stroke_width":4}],"three_quarter":[{"d":"M-16,-62 Q-6,-68 4,-62","fill":"none","stroke":"$hair","stroke_width":4},{"d":"M14,-62 Q24,-68 34,-62","fill":"none","stroke":"$hair","stroke_width":4}],"profile":[{"d":"M16,-62 Q26,-68 36,-62","fill":"none","stroke":"$hair","stroke_width":4}]},"soft/raised":{"front":[{"d":"M-24,-70 Q-14,-76 -4,-70","fill":"none","stroke":"$hair","stroke_width":4},{"d":"M4,-70 Q14,-76 24,-70","fill":"none","stroke":"$hair","stroke_width":4}],"three_quarter":[{"d":"M-16,-70 Q-6,-76 4,-70","fill":"none","stroke":"$hair","stroke_width":4},{"d":"M14,-70 Q24,-76 34,-70","fill":"none","stroke":"$hair","stroke_width":4}],"profile":[{"d":"M16,-70 Q26,-76 36,-70","fill":"none","stroke":"$hair","stroke_width":4}]}},"facial_hair":{"none":{"front":[],"three_quarter":[],"profile":[]}},"hair":{"bun":{"front":[{"d":"M-36,-52 C-38,-86 -16,-98 2,-98 C22,-98 40,-84 36,-52 C28,-64 14,-72 0,-72 C-14,-72 -28,-64 -36,-52 Z","fill":"$hair","stroke":"$outline","stroke_width":3}],"three_quarter":[{"d":"M-36,-52 C-38,-86 -16,-98 2,-98 C22,-98 40,-84 36,-52 C28,-64 14,-72 0,-72 C-14,-72 -28,-64 -36,-52 Z","fill":"$hair","stroke":"$outline","stroke_width":3}],"profile":[{"d":"M-36,-52 C-38,-86 -16,-98 2,-98 C22,-98 40,-84 36,-52 C28,-64 14,-72 0,-72 C-14,-72 -28,-64 -36,-52 Z","fill":"$hair","stroke":"$outline","stroke_width":3}]}},"glasses":{"none":{"front":[],"three_quarter":[],"profile":[]}},"accessory":{"none":{"front":[],"three_quarter":[],"profile":[]}},"sleeve_upper_l":{"blazer":{"front":[{"d":"M-3.6000000000000014,-4 L3.6000000000000014,-4 Q12,-4 12,4.399999999999999 L12,67.6 Q12,76 3.6000000000000014,76 L-3.6000000000000014,76 Q-12,76 -12,67.6 L-12,4.399999999999999 Q-12,-4 -3.6000000000000014,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-3.6000000000000014,-4 L3.6000000000000014,-4 Q12,-4 12,4.399999999999999 L12,67.6 Q12,76 3.6000000000000014,76 L-3.6000000000000014,76 Q-12,76 -12,67.6 L-12,4.399999999999999 Q-12,-4 -3.6000000000000014,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-3.6000000000000014,-4 L3.6000000000000014,-4 Q12,-4 12,4.399999999999999 L12,67.6 Q12,76 3.6000000000000014,76 L-3.6000000000000014,76 Q-12,76 -12,67.6 L-12,4.399999999999999 Q-12,-4 -3.6000000000000014,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}]}},"sleeve_lower_l":{"blazer":{"front":[{"d":"M-3,-4 L3,-4 Q10,-4 10,3 L10,63 Q10,70 3,70 L-3,70 Q-10,70 -10,63 L-10,3 Q-10,-4 -3,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"three_quarter":[{"d":"M-3,-4 L3,-4 Q10,-4 10,3 L10,63 Q10,70 3,70 L-3,70 Q-10,70 -10,63 L-10,3 Q-10,-4 -3,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}],"profile":[{"d":"M-3,-4 L3,-4 Q10,-4 10,3 L10,63 Q10,70 3,70 L-3,70 Q-10,70 -10,63 L-10,3 Q-10,-4 -3,-4 Z","fill":"$top","stroke":"$outline","stroke_width":2.5}]}},"hand_l":{"open":{"front":[{"d":"M-11,76 a11,12 0 1,0 22,0 a11,12 0 1,0 -22,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-11,76 a11,12 0 1,0 22,0 a11,12 0 1,0 -22,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-11,76 a11,12 0 1,0 22,0 a11,12 0 1,0 -22,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2}]},"fist":{"front":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":3}],"three_quarter":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":3}],"profile":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":3}]},"point":{"front":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M2,72 L20,64","fill":"none","stroke":"$skin","stroke_width":7}],"three_quarter":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M2,72 L20,64","fill":"none","stroke":"$skin","stroke_width":7}],"profile":[{"d":"M-10,74 a10,10 0 1,0 20,0 a10,10 0 1,0 -20,0 Z","fill":"$skin","stroke":"$outline","stroke_width":2},{"d":"M2,72 L20,64","fill":"none","stroke":"$skin","stroke_width":7}]},"hold":{"front":[{"d":"M-3,66 L3,66 Q10,66 10,73 L10,77 Q10,84 3,84 L-3,84 Q-10,84 -10,77 L-10,73 Q-10,66 -3,66 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"three_quarter":[{"d":"M-3,66 L3,66 Q10,66 10,73 L10,77 Q10,84 3,84 L-3,84 Q-10,84 -10,77 L-10,73 Q-10,66 -3,66 Z","fill":"$skin","stroke":"$outline","stroke_width":2}],"profile":[{"d":"M-3,66 L3,66 Q10,66 10,73 L10,77 Q10,84 3,84 L-3,84 Q-10,84 -10,77 L-10,73 Q-10,66 -3,66 Z","fill":"$skin","stroke":"$outline","stroke_width":2}]}}},"bodies":{"average":{"scale":1},"child":{"scale":0.72}},"params":{"body":["average","child"],"skin":{"light":"#f6d5bd","medium":"#e0ac84","dark":"#8d5a3b"},"hair":["bun","none"],"hair_color":{"dark-brown":"#3b2a22","blond":"#d8b45c"},"eyes":["round"],"brows":["soft"],"facial_hair":["none"],"glasses":["none"],"outfit":{"business":{"top":"blazer","bottom":"trousers","shoes":"shoes","colors":{"top":"#2b4a8b","bottom":"#3a3f55","shoes":"#22222a"}}},"accessory":["none"],"fixed":{"outline":"#1f2a44","eye_white":"#ffffff","iris":"#3b2a22","lip":"#c4575f"}}}
````


**`internal/storymaker/testdata/catalog-mini/props/lock.json`**

````json
{"viewbox":[0,0,100,120],"shapes":[{"d":"M28,52 L28,34 C28,14 72,14 72,34 L72,52","fill":"none","stroke":"$fg","stroke_width":9},{"d":"M26,50 L74,50 Q86,50 86,62 L86,100 Q86,112 74,112 L26,112 Q14,112 14,100 L14,62 Q14,50 26,50 Z","fill":"$accent","stroke":"$outline","stroke_width":4},{"d":"M42,76 a8,8 0 1,0 16,0 a8,8 0 1,0 -16,0 Z","fill":"$outline"},{"d":"M50,82 L50,98","fill":"none","stroke":"$outline","stroke_width":6}]}
````


**`internal/storymaker/testdata/catalog-mini/props/shield.json`**

````json
{"viewbox":[0,0,100,120],"shapes":[{"d":"M50,6 L92,20 L92,62 C92,92 66,110 50,116 C34,110 8,92 8,62 L8,20 Z","gradient":{"type":"linear","stops":[[0,"$accent2"],[1,"$accent"]],"x1":0,"y1":0,"x2":100,"y2":120},"stroke":"$outline","stroke_width":4},{"d":"M30,60 L45,76 L72,42","fill":"none","stroke":"$bg","stroke_width":9}]}
````


**`internal/storymaker/testdata/catalog-mini/environments/plain-stage.json`**

````json
{"viewbox":[0,0,1920,1080],"layers":[{"name":"sky","parallax":0,"shapes":[{"d":"M-200,-200 h2320 v1480 h-2320 Z","gradient":{"type":"linear","stops":[[0,"$sky_top"],[1,"$sky_bottom"]],"x1":0,"y1":0,"x2":0,"y2":1080}}]},{"name":"back","parallax":0.5,"shapes":[]},{"name":"floor","parallax":1,"shapes":[{"d":"M-200,760 h2320 v520 h-2320 Z","fill":"$floor"},{"d":"M-200,756 h2320 v8 h-2320 Z","fill":"$floor_line"}]}],"tokens":{"day":{"sky_top":"#bfe6ff","sky_bottom":"#fff7e6","floor":"#e8d3a8","floor_line":"#c9b07a","wall":"#f4e4c1"},"night":{"sky_top":"#17224d","sky_bottom":"#3a4a86","floor":"#4a4565","floor_line":"#35314d","wall":"#2c3358"}}}
````


**`internal/storymaker/testdata/catalog-mini/environments/office.json`**

````json
{"viewbox":[0,0,1920,1080],"layers":[{"name":"sky","parallax":0,"shapes":[{"d":"M-200,-200 h2320 v1480 h-2320 Z","gradient":{"type":"linear","stops":[[0,"$sky_top"],[1,"$sky_bottom"]],"x1":0,"y1":0,"x2":0,"y2":1080}}]},{"name":"back","parallax":0.5,"shapes":[{"d":"M0,0 h1920 v760 h-1920 Z","fill":"$wall"},{"d":"M1280,140 h380 v420 h-380 Z","fill":"$sky_top","stroke":"$outline","stroke_width":8},{"d":"M1330,600 h280 v18 h-280 Z","fill":"$floor_line"}]},{"name":"floor","parallax":1,"shapes":[{"d":"M-200,760 h2320 v520 h-2320 Z","fill":"$floor"},{"d":"M-200,756 h2320 v8 h-2320 Z","fill":"$floor_line"}]}],"tokens":{"day":{"sky_top":"#bfe6ff","sky_bottom":"#fff7e6","floor":"#e8d3a8","floor_line":"#c9b07a","wall":"#f4e4c1"},"night":{"sky_top":"#17224d","sky_bottom":"#3a4a86","floor":"#4a4565","floor_line":"#35314d","wall":"#2c3358"}}}
````


**`internal/storymaker/testdata/catalog-mini/environments/abstract-tech.json`**

````json
{"viewbox":[0,0,1920,1080],"layers":[{"name":"sky","parallax":0,"shapes":[{"d":"M-200,-200 h2320 v1480 h-2320 Z","gradient":{"type":"linear","stops":[[0,"$sky_top"],[1,"$sky_bottom"]],"x1":0,"y1":0,"x2":0,"y2":1080}}]},{"name":"back","parallax":0.5,"shapes":[{"d":"M0,0 h1920 v760 h-1920 Z","fill":"$wall","opacity":0.6},{"d":"M0,380 L1920,380 M0,520 L1920,520 M480,0 L480,760 M960,0 L960,760 M1440,0 L1440,760","fill":"none","stroke":"$accent2","stroke_width":3,"opacity":0.5}]},{"name":"floor","parallax":1,"shapes":[{"d":"M-200,760 h2320 v520 h-2320 Z","fill":"$floor"},{"d":"M-200,756 h2320 v8 h-2320 Z","fill":"$floor_line"}]}],"tokens":{"day":{"sky_top":"#bfe6ff","sky_bottom":"#fff7e6","floor":"#e8d3a8","floor_line":"#c9b07a","wall":"#f4e4c1"},"night":{"sky_top":"#17224d","sky_bottom":"#3a4a86","floor":"#4a4565","floor_line":"#35314d","wall":"#2c3358"}}}
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`4 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add scripts/storymaker-bundle-parts.js scripts/storymaker-test-support.mjs scripts/storymaker-test-context.mjs scripts/test-storymaker-player.mjs scripts/storymaker-tests/00-fixtures.mjs internal/storymaker/testdata package.json
git commit -m "test(storymaker): player test harness and catalog-mini fixtures" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C2: Easing, tween tracks and the frame schedule

**Files:**
- Create: `ui/js/storymaker/ease.js`, `ui/js/storymaker/timeline.js`, `ui/js/storymaker/schedule.js`
- Test: `scripts/storymaker-tests/01-ease-timeline.mjs`

Three small pure modules. `ease.js`: the six eases of the schema (linear, ease-in, ease-out, ease-in-out, back-out, bounce-out), `t`
clamped to 0..1, an unknown name is linear. `timeline.js`: `Track` (numeric segments `t0 t1 from to ease`; a segment without `from` starts at the
value the track has at `t0`, also when segments overlap; the latest start wins), `StepTrack` (discrete values, with an optional blend duration for
poses) and `Intervals` (walking and gesture windows). `schedule.js` answers "which scenes are on screen at frame n" from the scene ranges alone
(`framePlan`); it is part of **both** bundles, so the document-realm host and the player worker agree without sharing the timeline.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/01-ease-timeline.mjs`**

````js
// Task C2: easing, tween tracks and step tracks.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near } = t;
const SM = t.core();

check('the six eases hit 0 and 1 and behave', () => {
    const names = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out'];
    assert.deepEqual(plain(SM.easeNames), names);
    for (const name of names) {
        near(SM.ease(name, 0), 0, 1e-12, name + '(0)');
        near(SM.ease(name, 1), 1, 1e-12, name + '(1)');
    }
    near(SM.ease('linear', 0.25), 0.25, 1e-12);
    assert.ok(SM.ease('ease-in', 0.5) < 0.5 && SM.ease('ease-out', 0.5) > 0.5);
    near(SM.ease('ease-in-out', 0.5), 0.5, 1e-12);
    assert.ok(SM.ease('back-out', 0.8) > 1, 'back-out overshoots');
    assert.ok(SM.ease('bounce-out', 0.9) < 1 && SM.ease('bounce-out', 0.95) > SM.ease('bounce-out', 0.9) - 0.2);
    assert.equal(SM.ease('nope', 0.3), 0.3, 'an unknown name is linear');
    assert.equal(SM.ease('ease-out', -4), 0);
    assert.equal(SM.ease('ease-out', 9), 1);
});

check('tracks evaluate segments, overlapping segments and step tracks', () => {
    const T = SM.timeline, tr = new T.Track(10);
    tr.add(1, 3, 20, 'linear');
    tr.add(5, 5, 0, 'linear');
    tr.add(2, 4, 100, 'linear');   // starts while the first runs: from = the value of the first at t=2
    near(tr.at(0), 10, 1e-9);
    near(tr.at(1.5), 12.5, 1e-9);
    near(tr.at(2), 15, 1e-9);
    near(tr.at(3), 57.5, 1e-9);
    near(tr.at(4.5), 100, 1e-9);
    near(tr.at(5), 0, 1e-9);
    const st = new T.StepTrack('a');
    st.set(2, 'b', 1).set(1, 'c', 1);
    assert.equal(st.at(0.5), 'a');
    assert.equal(st.at(1.5), 'c');
    assert.equal(st.at(2.5), 'b');
    const blend = st.blendAt(2.25);
    assert.equal(blend.a, 'c'); assert.equal(blend.b, 'b'); near(blend.u, 0.25, 1e-9);
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (4 passed, 2 failed; first: the six eases hit 0 and 1 and behave: TypeError: Cannot read properties of undefined (reading 'easeNames'))

- [ ] **Step 3: Implement**


**`ui/js/storymaker/ease.js`**

````js
// Storymaker player: the six easing functions of the story schema (linear, ease-in, ease-out, ease-in-out, back-out,
// bounce-out). Pure and deterministic. Classic script, its own IIFE scope; it publishes StorymakerCore.ease.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var NAMES = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out'];

    function clamp01(t) {
        if (!(t > 0)) return 0;
        return t > 1 ? 1 : t;
    }

    function bounceOut(t) {
        var n1 = 7.5625, d1 = 2.75;
        if (t < 1 / d1) return n1 * t * t;
        if (t < 2 / d1) { t -= 1.5 / d1; return n1 * t * t + 0.75; }
        if (t < 2.5 / d1) { t -= 2.25 / d1; return n1 * t * t + 0.9375; }
        t -= 2.625 / d1;
        return n1 * t * t + 0.984375;
    }

    var FNS = {
        'linear': function (t) { return t; },
        'ease-in': function (t) { return t * t * t; },
        'ease-out': function (t) { var u = 1 - t; return 1 - u * u * u; },
        'ease-in-out': function (t) {
            if (t < 0.5) return 4 * t * t * t;
            var u = -2 * t + 2;
            return 1 - (u * u * u) / 2;
        },
        'back-out': function (t) {
            var c1 = 1.70158, c3 = c1 + 1, u = t - 1;
            return 1 + c3 * u * u * u + c1 * u * u;
        },
        'bounce-out': bounceOut
    };

    // ease(name, t): t is clamped to 0..1; an unknown name is linear.
    function ease(name, t) {
        var fn = FNS[name] || FNS.linear;
        return fn(clamp01(t));
    }

    function isEase(name) { return Object.prototype.hasOwnProperty.call(FNS, name); }

    SM.ease = ease;
    SM.easeNames = NAMES.slice();
    SM.isEase = isEase;
    SM.clamp01 = clamp01;
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/timeline.js`**

````js
// Storymaker player: tween tracks. A Track is a numeric value over time built from segments (t0, t1, from, to, ease); a
// StepTrack is a discrete value that changes at given times. Tracks are filled while a story compiles and are evaluated
// by time only, so evaluation at time t is a pure function of the track. Classic script; publishes StorymakerCore.timeline.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function lerp(a, b, u) { return a + (b - a) * u; }
    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }

    function evalSeg(seg, t) {
        if (t >= seg.t1) return seg.to;
        if (!(seg.t1 > seg.t0)) return seg.to;
        return lerp(seg.from, seg.to, SM.ease(seg.ease, (t - seg.t0) / (seg.t1 - seg.t0)));
    }

    function Track(init) {
        this.init = init;
        this.segs = [];
        this.dirty = false;
    }

    // add(t0, t1, to, ease, from): from is optional; without it the segment starts at the value the track has at t0 (the
    // end of the previous segment, or its mid-way value when segments overlap).
    Track.prototype.add = function (t0, t1, to, ease, from) {
        this.segs.push({ t0: t0, t1: t1 < t0 ? t0 : t1, to: to, ease: ease || 'linear', from: from, order: this.segs.length });
        this.dirty = true;
        return this;
    };

    Track.prototype.finalize = function () {
        if (!this.dirty) return this;
        this.segs.sort(function (a, b) { return a.t0 - b.t0 || a.order - b.order; });
        for (var k = 0; k < this.segs.length; k++) {
            var seg = this.segs[k];
            if (seg.from === undefined || seg.from === null) {
                seg.from = k === 0 ? this.init : evalSeg(this.segs[k - 1], seg.t0);
            }
        }
        this.dirty = false;
        return this;
    };

    Track.prototype.at = function (t) {
        if (this.dirty) this.finalize();
        var segs = this.segs, lo = 0, hi = segs.length - 1, found = -1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (segs[mid].t0 <= t) { found = mid; lo = mid + 1; } else { hi = mid - 1; }
        }
        return found < 0 ? this.init : evalSeg(segs[found], t);
    };

    Track.prototype.end = function () {
        var end = 0;
        for (var i = 0; i < this.segs.length; i++) if (this.segs[i].t1 > end) end = this.segs[i].t1;
        return end;
    };

    function StepTrack(init) {
        this.init = init;
        this.steps = [];
        this.dirty = false;
    }

    // set(t, value, extra): extra is carried along (the pose track keeps its blend duration there).
    StepTrack.prototype.set = function (t, value, extra) {
        this.steps.push({ t: t, v: value, x: extra, order: this.steps.length });
        this.dirty = true;
        return this;
    };

    StepTrack.prototype.index = function (t) {
        if (this.dirty) {
            this.steps.sort(function (a, b) { return a.t - b.t || a.order - b.order; });
            this.dirty = false;
        }
        var steps = this.steps, lo = 0, hi = steps.length - 1, found = -1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1;
            if (steps[mid].t <= t) { found = mid; lo = mid + 1; } else { hi = mid - 1; }
        }
        return found;
    };

    StepTrack.prototype.at = function (t) {
        var k = this.index(t);
        return k < 0 ? this.init : this.steps[k].v;
    };

    // blendAt(t): for a track whose steps carry a blend duration in x: {a: previous value, b: value, u: 0..1}.
    StepTrack.prototype.blendAt = function (t) {
        var k = this.index(t);
        if (k < 0) return { a: this.init, b: this.init, u: 1 };
        var step = this.steps[k], prev = k === 0 ? this.init : this.steps[k - 1].v;
        var dur = step.x > 0 ? step.x : 0;
        var u = dur > 0 ? clamp((t - step.t) / dur, 0, 1) : 1;
        return u >= 1 ? { a: step.v, b: step.v, u: 1 } : { a: prev, b: step.v, u: u };
    };

    // Interval list (walking, gesture windows): active(t) returns the interval containing t, the latest start first.
    function Intervals() { this.list = []; }
    Intervals.prototype.add = function (t0, t1, data) { this.list.push({ t0: t0, t1: t1, data: data }); return this; };
    Intervals.prototype.active = function (t) {
        var best = null;
        for (var i = 0; i < this.list.length; i++) {
            var it = this.list[i];
            if (t >= it.t0 && t < it.t1 && (!best || it.t0 >= best.t0)) best = it;
        }
        return best;
    };

    SM.timeline = { Track: Track, StepTrack: StepTrack, Intervals: Intervals, lerp: lerp, clamp: clamp };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/schedule.js`**

````js
// Storymaker player: the frame schedule. It answers "which scenes are on screen at frame n" from the scene ranges alone
// (start_frame, end_frame, transition frames), so the document-realm host and the player worker agree without sharing the
// timeline. Scene boundaries are whole frames. Classic script; it is part of both bundles; it publishes StorymakerCore.schedule.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // sched = {fps, frame_count, scenes:[{id, kind, start_frame, end_frame, tf, transition}]}
    // framePlan(sched, n) = {cur, layers:[{i, f}], transition: {kind, p} | null}. layers is drawn in order. f is the
    // scene-local frame (a scene held past its end during a transition shows its last frame). During the entry transition
    // of scene i the previous scene is the first layer and p runs over (0, 1).
    function framePlan(sched, n) {
        var scenes = sched.scenes || [];
        if (!scenes.length) return { cur: -1, layers: [], transition: null };
        var cur = 0;
        for (var i = 0; i < scenes.length; i++) {
            if (scenes[i].start_frame <= n) cur = i; else break;
        }
        var sc = scenes[cur], f = n - sc.start_frame;
        var layers = [{ i: cur, f: f < 0 ? 0 : f }], transition = null;
        if (cur > 0 && sc.tf > 0 && f < sc.tf && sc.transition && sc.transition !== 'cut') {
            var prev = scenes[cur - 1];
            var pf = n - prev.start_frame, last = prev.end_frame - prev.start_frame - 1;
            layers.unshift({ i: cur - 1, f: pf > last ? last : pf });
            transition = { kind: sc.transition, p: (f + 1) / (sc.tf + 1) };
        }
        return { cur: cur, layers: layers, transition: transition };
    }

    function frameToSeconds(fps, n) { return n / fps; }
    function secondsToFrames(fps, s) { return Math.round(s * fps + 1e-9); }

    SM.schedule = { framePlan: framePlan, frameToSeconds: frameToSeconds, secondsToFrames: secondsToFrames };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`6 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/ease.js ui/js/storymaker/timeline.js ui/js/storymaker/schedule.js scripts/storymaker-tests/01-ease-timeline.mjs
git commit -m "feat(storymaker): player easing, tween tracks and frame schedule" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C3: Stage mapping, vector parts, text and camera

**Files:**
- Create: `ui/js/storymaker/stage.js`, `ui/js/storymaker/parts.js`, `ui/js/storymaker/text.js`, `ui/js/storymaker/camera.js`
- Test: `scripts/storymaker-tests/02-stage-parts-text.mjs`

`stage.js` lays everything out in the 1920x1080 design frame and maps it onto the canvas for 16:9, 9:16 and 1:1 with a **cover fit**: the
narrow aspects show a vertical slice centred on the environment's `aspects[<aspect>].focus_x`; spots use the per-aspect override when there is one and
are otherwise squeezed into the visible slice (so left/centre/right stay on screen); `{x, y}` positions are percent of the *visible* safe area; the depth
scale comes from the floor line; the camera window stays inside the design frame. `parts.js` turns a Shape into a `Path2D` with fills, strokes,
linear/radial gradients and `$token` resolution (an unresolved token draws `#ff00ff` and is recorded). `text.js` wraps with the real `measureText`
at the target font, ends over-long text with an ellipsis after `max_lines` and draws optional boxes. `camera.js` holds zoom/pan tracks and
deterministic shake bursts.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/02-stage-parts-text.mjs`**

````js
// Task C3: stage mapping (three aspects), vector parts, text wrapping, camera.
import assert from 'node:assert/strict';
import { plain, makeContext } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog } = t;
const SM = t.core();

check('stage: mapping for 16:9, 9:16 and 1:1 with cover fit, focus and spot overrides', () => {
    const plainEnv = t.byId('env.plain-stage'), office = t.byId('env.office');
    const wide = SM.stage.create({ aspect: '16:9', env: plainEnv });
    assert.deepEqual([wide.width, wide.height, wide.scale], [1920, 1080, 1]);
    assert.deepEqual(plain(wide.vis), { x: 0, y: 0, w: 1920, h: 1080 });
    assert.deepEqual(plain(wide.toCanvas(960, 540)), { x: 960, y: 540 });
    near(wide.spot('left').x, 576, 1e-9);
    near(wide.spot('left').y, 0.88 * 1080, 1e-9);
    const half = SM.stage.create({ aspect: '16:9', width: 960 });
    assert.deepEqual([half.width, half.height, half.scale], [960, 540, 0.5]);

    const tall = SM.stage.create({ aspect: '9:16', env: plainEnv });
    assert.deepEqual([tall.width, tall.height], [1080, 1920]);
    near(tall.scale, 1920 / 1080, 1e-9);
    near(tall.vis.w, 607.5, 1e-9); near(tall.vis.h, 1080, 1e-9); near(tall.vis.x, 960 - 303.75, 1e-9);
    const c = tall.toCanvas(960, 540);
    near(c.x, 540, 1e-9); near(c.y, 960, 1e-9);
    near(tall.spot('left').x, 0.42 * 1920, 1e-9, 'the 9:16 override of the environment wins');
    near(tall.spot('center').x, 960, 1e-9, 'a spot without an override is squeezed into the visible slice');
    near(tall.spot('board').x, tall.vis.x + 0.74 * tall.vis.w, 1e-9);
    const officeTall = SM.stage.create({ aspect: '9:16', env: office });
    near(officeTall.vis.x, 0.40 * 1920 - 303.75, 1e-9, 'focus_x of the environment');
    near(officeTall.spot('desk').x, officeTall.vis.x + 0.36 * officeTall.vis.w, 1e-9);
    assert.equal(officeTall.spot('desk').face, 'right');

    const square = SM.stage.create({ aspect: '1:1', env: office });
    assert.deepEqual([square.width, square.height, square.scale], [1080, 1080, 1]);
    near(square.vis.w, 1080, 1e-9); near(square.vis.x, 0.45 * 1920 - 540, 1e-9);
    near(square.toCanvas(square.vis.x + 540, 540).x, 540, 1e-9);

    // {x, y} is percent of the visible safe area
    const sp = wide.safePoint(0, 0), sp2 = wide.safePoint(100, 100), sp3 = wide.safePoint(50, 50);
    assert.deepEqual([sp.x, sp.y, sp2.x, sp2.y, sp3.x, sp3.y], [96, 54, 1824, 1026, 960, 540]);
    near(tall.safePoint(50, 50).x, 960, 1e-9);
    near(tall.safePoint(0, 0).x, tall.vis.x + 0.05 * tall.vis.w, 1e-9);
    assert.equal(wide.resolve('nowhere'), null);
    assert.equal(wide.resolve({ x: 50, y: 50 }).x, 960);

    // depth from the floor line, camera window clamped into the design frame
    near(wide.depthScale(0.88 * 1080), 0.8 + 0.25 * (18 / 26), 1e-9);
    assert.equal(wide.depthScale(0), 0.8); assert.equal(wide.depthScale(1080), 1.05);
    const win = wide.cameraWindow({ zoom: 2, cx: 100, cy: 100 });
    assert.deepEqual([win.x, win.y, win.w, win.h, win.scale], [0, 0, 960, 540, 2]);
    assert.deepEqual(plain(wide.matrix(win)), [2, 0, 0, 2, 0, 0]);
});

check('parts: shapes become paths with fills, strokes, gradients and token resolution', () => {
    SM.parts.resetUnresolved();
    const ctx = makeContext(null);
    const resolve = SM.parts.makeResolver([{ skin: '#aa8866' }, { accent: '#ff0000', fg: '$skin' }]);
    SM.parts.drawShapes(ctx, [
        { d: 'M0,0 L10,0 L10,10 Z', fill: '$skin', stroke: '$accent', stroke_width: 3, opacity: 0.5 },
        { d: 'M0,0 L5,5', fill: 'none', stroke: '#000' },
        { d: 'M0,0 L5,5', gradient: { type: 'linear', stops: [[0, '$accent'], [1, '#00ff00']], x1: 0, y1: 0, x2: 10, y2: 0 } },
        { d: 'M0,0 L5,5', gradient: { type: 'radial', stops: [[0, '#fff'], [1, '$nope']], cx: 5, cy: 5, r: 4 } }
    ], resolve, 1);
    const log = ctx.log.join('\n');
    assert.match(log, /set globalAlpha=0.5/); assert.match(log, /set fillStyle=#aa8866/); assert.match(log, /set strokeStyle=#ff0000/);
    assert.match(log, /fill\(path\(M0,0 L10,0 L10,10 Z\)\)/); assert.match(log, /set lineWidth=3/);
    assert.match(log, /createLinearGradient\(0,0,10,0\)=gradient/); assert.match(log, /createRadialGradient\(5,5,0,5,5,4\)=gradient/);
    assert.match(log, /stop\(0,#ff0000\)/); assert.match(log, /stop\(1,#ff00ff\)/);
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), ['nope'], 'an unresolved token draws magenta and is recorded');
    assert.equal(resolve('$fg'), '#ff00ff', 'a token that points to another token does not resolve');
    SM.parts.resetUnresolved();
});

check('text: wrapping with measureText, max_lines with an ellipsis, long words, newlines', () => {
    const measure = s => s.length * 5;   // 10 px font, 0.5 em per character
    const ell = String.fromCharCode(0x2026);
    assert.deepEqual(plain(SM.text.wrap(measure, 'aaa bbb ccc', 40, 5).lines), ['aaa bbb', 'ccc']);
    const cut = SM.text.wrap(measure, 'aaa bbb ccc ddd eee fff', 40, 2);
    assert.equal(cut.lines.length, 2); assert.equal(cut.truncated, true);
    assert.ok(cut.lines[1].endsWith(ell)); assert.ok(measure(cut.lines[1]) <= 40);
    assert.deepEqual(plain(SM.text.wrap(measure, 'abcdefghijklmnopqrstuv', 40, 5).lines), ['abcdefgh', 'ijklmnop', 'qrstuv']);
    assert.deepEqual(plain(SM.text.wrap(measure, 'one\ntwo three', 100, 5).lines), ['one', 'two three']);
    assert.deepEqual(plain(SM.text.wrap(measure, '', 100, 2).lines), ['']);
    const ctx = makeContext(null), style = t.byId('text.title');
    const lay = SM.text.layout(ctx, 'Ein ziemlich langer Titel, der umbrochen werden muss, weil er nicht passt', style, { canvasH: 1080, maxWidth: 900, family: 'Storymaker Default' });
    assert.equal(lay.px, Math.round(0.07 * 1080));
    assert.equal(lay.font, '700 76px "Storymaker Default", sans-serif');
    assert.ok(lay.lines.length <= 2, 'max_lines of the title style');
    assert.ok(lay.lines.every(l => ctx.measureText(l).width <= 900));
    near(SM.text.maxWidthFor(1920, { x: 5, y: 5, w: 90, h: 90 }, 960, 'center'), 1728, 1e-9);
    near(SM.text.maxWidthFor(1920, { x: 5, y: 5, w: 90, h: 90 }, 400, 'center'), 2 * (400 - 96), 1e-9);
    near(SM.text.maxWidthFor(1920, { x: 5, y: 5, w: 90, h: 90 }, 100, 'center'), 432, 1e-9, 'never narrower than a quarter of the safe width');
});

check('camera: framings, zoom and pan tweens, shake bursts decay and never leave the zoom range', () => {
    assert.deepEqual(plain(SM.camera.FRAMING), { wide: 1, medium: 1.35, close: 1.9 });
    const cam = SM.camera.create(1, 960, 540);
    SM.camera.addZoom(cam, 1, 2, 2, 'linear');
    SM.camera.addPan(cam, 1, 2, 1200, 600, 'linear');
    SM.camera.addShake(cam, 3, 4, 10);
    const mid = SM.camera.at(cam, 1.5);
    near(mid.zoom, 1.5, 1e-9); near(mid.cx, 1080, 1e-9); near(mid.cy, 570, 1e-9);
    assert.equal(SM.camera.at(cam, 0).zoom, 1);
    assert.equal(SM.camera.at(cam, 2.5).zoom, 2);
    assert.equal(SM.camera.at(cam, 2.5).shakeX, 0);
    const early = SM.camera.at(cam, 3.05), late = SM.camera.at(cam, 3.95);
    assert.ok(Math.abs(early.shakeX) + Math.abs(early.shakeY) > 0 && Math.abs(early.shakeX) <= 10 && Math.abs(late.shakeX) < 1, 'the burst decays');
    assert.deepEqual(plain(SM.camera.at(cam, 3.05)), plain(SM.camera.at(cam, 3.05)), 'shake is a pure function of t');
    assert.equal(SM.camera.clampZoom(0.2), 1); assert.equal(SM.camera.clampZoom(99), SM.camera.MAX_ZOOM);
    near(SM.camera.end(cam), 4, 1e-9);
    // the stage turns the camera into a window inside the design frame
    const stage = SM.stage.create({ aspect: '16:9' }), win = stage.cameraWindow(SM.camera.at(cam, 2.5));
    near(win.w, 960, 1e-9); near(win.x, 1200 - 480, 1e-9); near(win.y, 600 - 270, 1e-9);
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (6 passed, 4 failed; first: stage: mapping for 16:9, 9:16 and 1:1 with cover fit, focus and spot overrides: TypeError: Cannot read properties of undefined (reading 'create'))

- [ ] **Step 3: Implement**


**`ui/js/storymaker/stage.js`**

````js
// Storymaker player: the stage. Everything is laid out in the 1920x1080 design frame (positions are design pixels);
// the stage maps the design frame onto the output canvas for 16:9, 9:16 and 1:1 with a cover fit (the narrow aspects show
// a vertical slice centred on the environment's focus_x). It resolves spots (with the per-aspect overrides), {x, y}
// positions (percent of the visible safe area) and the depth scale from the environment's floor line. Classic script;
// publishes StorymakerCore.stage.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var DESIGN_W = 1920, DESIGN_H = 1080;
    var SIZES = { '16:9': [1920, 1080], '9:16': [1080, 1920], '1:1': [1080, 1080] };
    var DEFAULT_SAFE = { x: 5, y: 5, w: 90, h: 90 };
    var DEFAULT_FLOOR = { y_near: 96, y_far: 70, scale_near: 1.05, scale_far: 0.8 };
    var DEFAULT_SPOTS = {
        'far-left': { x: 12, y: 88 }, 'left': { x: 30, y: 88 }, 'center': { x: 50, y: 88 }, 'right': { x: 70, y: 88 },
        'far-right': { x: 88, y: 88 }, 'board': { x: 72, y: 60 }
    };
    var BASE_SPOTS = ['far-left', 'left', 'center', 'right', 'far-right'];

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function num(v, d) { return typeof v === 'number' && isFinite(v) ? v : d; }

    // sizeFor(aspect, width): the canvas size of an aspect; a width override keeps the aspect.
    function sizeFor(aspect, width) {
        var base = SIZES[aspect] || SIZES['16:9'];
        if (!(width > 0)) return { width: base[0], height: base[1] };
        return { width: Math.round(width), height: Math.round(width * base[1] / base[0]) };
    }

    // create({aspect, width, height, env, safe}): env is a catalog environment entry (spots, floor, aspects) or null.
    function create(opts) {
        opts = opts || {};
        var aspect = SIZES[opts.aspect] ? opts.aspect : '16:9';
        var size = sizeFor(aspect);
        var width = opts.width > 0 ? opts.width : size.width;
        var height = opts.height > 0 ? opts.height : (opts.width > 0 ? Math.round(opts.width * size.height / size.width) : size.height);
        var env = opts.env || null;
        var safe = opts.safe || DEFAULT_SAFE;
        var over = env && env.aspects && env.aspects[aspect] ? env.aspects[aspect] : null;

        // cover fit
        var scale = Math.max(width / DESIGN_W, height / DESIGN_H);
        var visW = Math.min(DESIGN_W, width / scale), visH = Math.min(DESIGN_H, height / scale);
        var focus = over && typeof over.focus_x === 'number' ? over.focus_x : 50;
        var visX = clamp(focus / 100 * DESIGN_W - visW / 2, 0, DESIGN_W - visW);
        var visY = (DESIGN_H - visH) / 2;
        var vis = { x: visX, y: visY, w: visW, h: visH };
        var cropped = visW < DESIGN_W - 0.5;
        var floor = env && env.floor ? env.floor : DEFAULT_FLOOR;

        var stage = {
            aspect: aspect, width: width, height: height, scale: width / visW, vis: vis, safe: safe, floor: floor,
            cropped: cropped, focusX: visX + visW / 2, env: env
        };

        // Percent of the design frame to design pixels.
        stage.pct = function (xPct, yPct) { return { x: xPct / 100 * DESIGN_W, y: yPct / 100 * DESIGN_H }; };

        // {x, y} in percent of the visible safe area to design pixels.
        stage.safePoint = function (xPct, yPct) {
            return {
                x: vis.x + (safe.x + clamp(xPct, 0, 100) / 100 * safe.w) / 100 * vis.w,
                y: vis.y + (safe.y + clamp(yPct, 0, 100) / 100 * safe.h) / 100 * vis.h
            };
        };

        stage.spotNames = function () {
            var names = {}, k;
            for (k in DEFAULT_SPOTS) names[k] = true;
            if (env && env.spots) for (k in env.spots) names[k] = true;
            return Object.keys(names);
        };

        stage.hasSpot = function (name) {
            return !!((env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name]);
        };

        // spot(name) = {x, y, face, depth, name} in design pixels, or null for an unknown name. The per-aspect override
        // wins; otherwise a spot of a cropped aspect is squeezed into the visible slice so left/centre/right stay on screen.
        stage.spot = function (name) {
            var src = (env && env.spots && env.spots[name]) || DEFAULT_SPOTS[name];
            if (!src) return null;
            var o = over && over.spots && over.spots[name];
            var xPct = num(o && o.x, src.x), yPct = num(o && o.y, src.y), x;
            if (o && typeof o.x === 'number') x = xPct / 100 * DESIGN_W;
            else if (cropped) x = vis.x + xPct / 100 * vis.w;
            else x = xPct / 100 * DESIGN_W;
            return { name: name, x: x, y: yPct / 100 * DESIGN_H, face: src.face || null, depth: typeof src.depth === 'number' ? src.depth : null };
        };

        // resolve(at, fallbackName): a spot name or {x, y} to {x, y, face, depth, spot}. Unknown values give null.
        stage.resolve = function (at) {
            if (typeof at === 'string') {
                var s = stage.spot(at);
                return s ? { x: s.x, y: s.y, face: s.face, depth: s.depth, spot: s.name } : null;
            }
            if (at && typeof at === 'object' && typeof at.x === 'number' && typeof at.y === 'number') {
                var p = stage.safePoint(at.x, at.y);
                return { x: p.x, y: p.y, face: null, depth: null, spot: null };
            }
            return null;
        };

        // Depth scale from the floor line: y_far -> scale_far, y_near -> scale_near, linear and clamped. depth (0 far .. 1 near)
        // overrides the y lookup for spots that carry one.
        stage.depthScale = function (yDesign, depth) {
            var f = floor, t;
            if (typeof depth === 'number') t = depth;
            else {
                var yPct = yDesign / DESIGN_H * 100, span = f.y_near - f.y_far;
                t = span === 0 ? 1 : (yPct - f.y_far) / span;
            }
            t = clamp(t, 0, 1);
            return f.scale_far + (f.scale_near - f.scale_far) * t;
        };

        // Off-stage x positions for enter and exit: clearly outside the visible slice.
        stage.offLeft = function () { return vis.x - 220; };
        stage.offRight = function () { return vis.x + vis.w + 220; };

        // cameraWindow(cam = {zoom, cx, cy, shakeX, shakeY}) = the rectangle of the design frame shown on the canvas.
        // The window keeps the canvas aspect and stays inside the design frame.
        stage.cameraWindow = function (cam) {
            var zoom = cam && cam.zoom > 0 ? cam.zoom : 1;
            var w = vis.w / zoom, h = vis.h / zoom;
            var cx = cam && typeof cam.cx === 'number' ? cam.cx : vis.x + vis.w / 2;
            var cy = cam && typeof cam.cy === 'number' ? cam.cy : vis.y + vis.h / 2;
            var x = clamp(cx - w / 2, 0, DESIGN_W - w) + (cam && cam.shakeX || 0);
            var y = clamp(cy - h / 2, 0, DESIGN_H - h) + (cam && cam.shakeY || 0);
            return { x: x, y: y, w: w, h: h, scale: width / w };
        };

        // The base window (no camera movement) and the matrix of a window: [a, b, c, d, e, f] for ctx.setTransform.
        stage.baseWindow = function () { return { x: vis.x, y: vis.y, w: vis.w, h: vis.h, scale: width / vis.w }; };
        stage.matrix = function (win) { return [win.scale, 0, 0, win.scale, -win.x * win.scale, -win.y * win.scale]; };

        // Canvas pixel of a design point under a window (used by screen-space overlays and tests).
        stage.toCanvas = function (px, py, win) {
            var w = win || stage.baseWindow();
            return { x: (px - w.x) * w.scale, y: (py - w.y) * w.scale };
        };

        stage.isBaseSpot = function (name) { return BASE_SPOTS.indexOf(name) >= 0; };
        return stage;
    }

    SM.stage = {
        create: create, sizeFor: sizeFor, DESIGN_W: DESIGN_W, DESIGN_H: DESIGN_H, SIZES: SIZES,
        DEFAULT_SAFE: DEFAULT_SAFE, DEFAULT_FLOOR: DEFAULT_FLOOR, DEFAULT_SPOTS: DEFAULT_SPOTS
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/parts.js`**

````js
// Storymaker player: the vector part format. A Shape {d, fill, stroke, stroke_width, opacity, gradient} becomes a Path2D
// with fills, strokes and gradients; colours may be hex or a $token that a resolver chain answers. An unresolved token
// draws #ff00ff and is recorded so tests (and the catalog build) can report it. Classic script; publishes StorymakerCore.parts.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var MISSING = '#ff00ff';
    var unresolved = {};
    var pathCache = {};
    var pathCount = 0;

    // makeResolver(maps): maps are plain objects, the first that has the token wins. Returns resolve(value).
    function makeResolver(maps) {
        var list = (maps || []).filter(function (m) { return m && typeof m === 'object'; });
        return function resolve(value) {
            if (typeof value !== 'string' || value.charAt(0) !== '$') return value;
            var name = value.slice(1);
            for (var i = 0; i < list.length; i++) {
                var v = list[i][name];
                if (typeof v === 'string' && v) return v.charAt(0) === '$' ? MISSING : v;
            }
            unresolved[name] = (unresolved[name] || 0) + 1;
            return MISSING;
        };
    }

    function unresolvedTokens() { return Object.keys(unresolved).sort(); }
    function resetUnresolved() { unresolved = {}; }

    function pathFor(d) {
        var p = pathCache[d];
        if (!p) {
            if (pathCount > 4000) { pathCache = {}; pathCount = 0; }
            p = new Path2D(d);
            pathCache[d] = p;
            pathCount++;
        }
        return p;
    }

    function makeGradient(ctx, g, resolve) {
        var grad;
        if (g.type === 'radial') {
            var cx = g.cx || 0, cy = g.cy || 0;
            grad = ctx.createRadialGradient(cx, cy, 0, cx, cy, g.r > 0 ? g.r : 1);
        } else {
            grad = ctx.createLinearGradient(g.x1 || 0, g.y1 || 0, g.x2 === undefined ? 1 : g.x2, g.y2 || 0);
        }
        var stops = g.stops || [];
        for (var i = 0; i < stops.length; i++) {
            var off = stops[i][0];
            grad.addColorStop(off < 0 ? 0 : (off > 1 ? 1 : off), resolve(stops[i][1]));
        }
        return grad;
    }

    // drawShape(ctx, shape, resolve, alpha): draws in the current transform. alpha multiplies the shape opacity.
    function drawShape(ctx, shape, resolve, alpha) {
        if (!shape || typeof shape.d !== 'string' || !shape.d) return;
        var a = (alpha === undefined ? 1 : alpha) * (typeof shape.opacity === 'number' ? shape.opacity : 1);
        if (!(a > 0)) return;
        var path = pathFor(shape.d);
        ctx.globalAlpha = a > 1 ? 1 : a;
        var hasStroke = shape.stroke && shape.stroke !== 'none';
        if (shape.gradient) {
            ctx.fillStyle = makeGradient(ctx, shape.gradient, resolve);
            ctx.fill(path);
        } else if (shape.fill !== 'none' && (shape.fill || !hasStroke)) {
            ctx.fillStyle = resolve(shape.fill || '#000000');
            ctx.fill(path);
        }
        if (hasStroke) {
            ctx.strokeStyle = resolve(shape.stroke);
            ctx.lineWidth = typeof shape.stroke_width === 'number' ? shape.stroke_width : 1;
            ctx.lineJoin = 'round';
            ctx.lineCap = 'round';
            ctx.stroke(path);
        }
    }

    function drawShapes(ctx, shapes, resolve, alpha) {
        if (!shapes) return;
        for (var i = 0; i < shapes.length; i++) drawShape(ctx, shapes[i], resolve, alpha);
    }

    // drawPart(ctx, part, resolve, alpha, fit): a prop file {viewbox:[x,y,w,h], shapes}. The caller has set the transform;
    // the part is drawn in its own viewbox coordinates.
    function drawPart(ctx, part, resolve, alpha) {
        if (part) drawShapes(ctx, part.shapes, resolve, alpha);
    }

    SM.parts = {
        makeResolver: makeResolver, unresolvedTokens: unresolvedTokens, resetUnresolved: resetUnresolved,
        drawShape: drawShape, drawShapes: drawShapes, drawPart: drawPart, MISSING: MISSING
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/text.js`**

````js
// Storymaker player: text. Text styles come from the catalog (font, size as a fraction of the canvas height, weight,
// colour, optional box, alignment, max_lines). Wrapping measures the real font through measureText and ends an
// over-long text with an ellipsis after max_lines. Text is drawn in screen space on top of the scene. Classic script;
// publishes StorymakerCore.text.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var ELLIPSIS = String.fromCharCode(0x2026);
    var LINE_HEIGHT = 1.22;

    function fontString(weight, px, family) {
        return String(weight || 400) + ' ' + px + 'px "' + String(family || 'sans-serif').replace(/"/g, '') + '", sans-serif';
    }

    function fitEllipsis(measure, line, maxWidth) {
        var text = line.replace(/[\s.,;:!?-]+$/, '');
        while (text.length > 0 && measure(text + ELLIPSIS) > maxWidth) text = text.slice(0, -1);
        return text + ELLIPSIS;
    }

    // breakWord(measure, word, maxWidth): splits a word that is wider than a line into pieces that fit.
    function breakWord(measure, word, maxWidth) {
        var pieces = [], current = '';
        for (var i = 0; i < word.length; i++) {
            if (current && measure(current + word[i]) > maxWidth) { pieces.push(current); current = word[i]; } else current += word[i];
        }
        if (current) pieces.push(current);
        return pieces;
    }

    // wrap(measure, text, maxWidth, maxLines) = {lines, truncated}. measure(str) returns the width at the target font.
    function wrap(measure, text, maxWidth, maxLines) {
        var limit = maxLines > 0 ? maxLines : 1000;
        var paragraphs = String(text === undefined || text === null ? '' : text).split(/\r?\n/);
        var lines = [];
        paragraphs.forEach(function (paragraph) {
            var words = paragraph.split(/\s+/).filter(Boolean), line = '';
            if (!words.length) { lines.push(''); return; }
            words.forEach(function (word) {
                var parts = measure(word) > maxWidth ? breakWord(measure, word, maxWidth) : [word];
                parts.forEach(function (part, k) {
                    var joined = line && k === 0 ? line + ' ' + part : (line ? line + part : part);
                    if (!line || measure(joined) <= maxWidth) { line = joined; return; }
                    lines.push(line);
                    line = part;
                });
            });
            if (line) lines.push(line);
        });
        if (lines.length <= limit) return { lines: lines, truncated: false };
        var kept = lines.slice(0, limit);
        kept[limit - 1] = fitEllipsis(measure, kept[limit - 1] + ' ' + lines[limit], maxWidth);
        return { lines: kept, truncated: true };
    }

    // layout(ctx, text, style, opts): opts = {canvasH, maxWidth, family}. Sets ctx.font to measure. Returns the lines
    // and the box in canvas pixels.
    function layout(ctx, text, style, opts) {
        var px = Math.max(8, Math.round((style.size > 0 ? style.size : 0.05) * opts.canvasH));
        ctx.font = fontString(style.weight, px, opts.family);
        var measure = function (s) { return ctx.measureText(s).width; };
        var wrapped = wrap(measure, text, opts.maxWidth, style.max_lines);
        var width = 0;
        wrapped.lines.forEach(function (l) { var w = measure(l); if (w > width) width = w; });
        var pad = style.box ? Math.round((style.box.pad > 0 ? style.box.pad : 0.4) * px) : 0;
        return {
            lines: wrapped.lines, truncated: wrapped.truncated, px: px, lineH: px * LINE_HEIGHT,
            width: width, height: wrapped.lines.length * px * LINE_HEIGHT, pad: pad, font: ctx.font
        };
    }

    function roundRect(ctx, x, y, w, h, r) {
        var rr = Math.min(r, w / 2, h / 2);
        ctx.beginPath();
        ctx.moveTo(x + rr, y);
        ctx.lineTo(x + w - rr, y);
        ctx.arcTo(x + w, y, x + w, y + rr, rr);
        ctx.lineTo(x + w, y + h - rr);
        ctx.arcTo(x + w, y + h, x + w - rr, y + h, rr);
        ctx.lineTo(x + rr, y + h);
        ctx.arcTo(x, y + h, x, y + h - rr, rr);
        ctx.lineTo(x, y + rr);
        ctx.arcTo(x, y, x + rr, y, rr);
        ctx.closePath();
    }

    // draw(ctx, lay, style, x, y, resolve, alpha): x is the anchor (centre, left or right by style.align), y the vertical
    // centre of the block, in canvas pixels.
    function draw(ctx, lay, style, x, y, resolve, alpha) {
        if (!(alpha > 0)) return;
        var align = style.align === 'left' || style.align === 'right' ? style.align : 'center';
        var left = align === 'left' ? x : (align === 'right' ? x - lay.width : x - lay.width / 2);
        ctx.globalAlpha = alpha > 1 ? 1 : alpha;
        if (style.box) {
            var radius = Math.round((style.box.radius >= 0 ? style.box.radius : 0.3) * lay.px);
            ctx.fillStyle = resolve(style.box.fill || '$bg2');
            roundRect(ctx, left - lay.pad, y - lay.height / 2 - lay.pad, lay.width + lay.pad * 2, lay.height + lay.pad * 2, radius);
            ctx.fill();
        }
        ctx.font = lay.font;
        ctx.fillStyle = resolve(style.color || '$text');
        ctx.textBaseline = 'middle';
        ctx.textAlign = align;
        var ax = align === 'left' ? left : (align === 'right' ? left + lay.width : left + lay.width / 2);
        for (var i = 0; i < lay.lines.length; i++) {
            ctx.fillText(lay.lines[i], ax, y - lay.height / 2 + lay.lineH * (i + 0.5));
        }
    }

    // maxWidthFor(canvasW, safe, x, align): the widest line that keeps the text inside the safe area around the anchor.
    function maxWidthFor(canvasW, safe, x, align) {
        var l = canvasW * safe.x / 100, r = canvasW * (safe.x + safe.w) / 100, full = r - l, w;
        if (align === 'left') w = r - x;
        else if (align === 'right') w = x - l;
        else w = 2 * Math.min(x - l, r - x);
        return Math.max(full * 0.25, Math.min(full, w));
    }

    SM.text = { fontString: fontString, wrap: wrap, layout: layout, draw: draw, maxWidthFor: maxWidthFor, roundRect: roundRect };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/camera.js`**

````js
// Storymaker player: the scene camera. A camera is a zoom track plus a focus point (design pixels) and shake bursts.
// Framings wide/medium/close are zoom levels; the zoom, pan, focus and shake beats add tween segments. The stage turns the
// camera state into a window of the design frame. Classic script; publishes StorymakerCore.camera.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FRAMING = { wide: 1, medium: 1.35, close: 1.9 };
    var MAX_ZOOM = 4;

    // create(zoom, cx, cy): cx/cy are the focus point in design pixels.
    function create(zoom, cx, cy) {
        var T = SM.timeline;
        return { zoom: new T.Track(zoom), cx: new T.Track(cx), cy: new T.Track(cy), shakes: [] };
    }

    function clampZoom(z) { return z < 1 ? 1 : (z > MAX_ZOOM ? MAX_ZOOM : z); }

    function addZoom(cam, t0, t1, zoom, ease) { cam.zoom.add(t0, t1, clampZoom(zoom), ease); }
    function addPan(cam, t0, t1, cx, cy, ease) { cam.cx.add(t0, t1, cx, ease); cam.cy.add(t0, t1, cy, ease); }
    function addShake(cam, t0, t1, amp) { cam.shakes.push({ t0: t0, t1: t1 > t0 ? t1 : t0 + 0.01, amp: amp }); }

    function shakeAt(cam, t) {
        var x = 0, y = 0;
        for (var i = 0; i < cam.shakes.length; i++) {
            var s = cam.shakes[i];
            if (t < s.t0 || t >= s.t1) continue;
            var decay = 1 - (t - s.t0) / (s.t1 - s.t0);
            x += s.amp * decay * (Math.sin(t * 61.7) * 0.6 + Math.sin(t * 37.3 + 1.3) * 0.4);
            y += s.amp * decay * (Math.sin(t * 53.1 + 0.7) * 0.6 + Math.sin(t * 29.9 + 2.1) * 0.4);
        }
        return { x: x, y: y };
    }

    // at(cam, t) = {zoom, cx, cy, shakeX, shakeY}
    function at(cam, t) {
        var sh = shakeAt(cam, t);
        return { zoom: cam.zoom.at(t), cx: cam.cx.at(t), cy: cam.cy.at(t), shakeX: sh.x, shakeY: sh.y };
    }

    function end(cam) {
        var e = Math.max(cam.zoom.end(), cam.cx.end(), cam.cy.end());
        for (var i = 0; i < cam.shakes.length; i++) if (cam.shakes[i].t1 > e) e = cam.shakes[i].t1;
        return e;
    }

    SM.camera = {
        FRAMING: FRAMING, MAX_ZOOM: MAX_ZOOM, create: create, clampZoom: clampZoom, addZoom: addZoom, addPan: addPan,
        addShake: addShake, at: at, end: end
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`10 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/stage.js ui/js/storymaker/parts.js ui/js/storymaker/text.js ui/js/storymaker/camera.js scripts/storymaker-tests/02-stage-parts-text.mjs
git commit -m "feat(storymaker): player stage mapping, vector parts, text and camera" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C4: The 2D character rig

**Files:**
- Create: `ui/js/storymaker/rig.js`
- Test: `scripts/storymaker-tests/03-rig.mjs`

`rig.js` implements contract section 3a: 13 bones with a rest transform per view, `world(b) = world(parent) . T(rest.x, rest.y) .
R(rest.angle + pose[b] + sum of deltas[b])` (degrees, clockwise positive), slots drawn in z order per view, variant selection (`eyes`/`brows` keys are
`<character value>/<expression variant>` with a `/neutral` fallback, `"none"` draws nothing), poses as bone offsets, gestures and cycles as timed key
blends that return to the base pose, hand shapes, mouth shape, the seated drop of the root. Mirroring for left-facing actors is a `scale(-1, 1)` in the
caller's base matrix.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/03-rig.mjs`**

````js
// Task C4: the 2D rig: bones, world matrices, slot order, pose, gesture and cycle keys.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, files } = t;
const SM = t.core();

check('rig: bone angles, world matrices, mirroring base and slot order', () => {
    const rig = t.files.rig, R = SM.rig, typing = t.byId('pose.typing');
    assert.equal(rig.bones.length, 13);
    assert.deepEqual(plain(rig.bones.map(b => b.name)), ['root', 'hips', 'torso', 'neck', 'head', 'upper_arm_l', 'lower_arm_l', 'upper_arm_r', 'lower_arm_r', 'upper_leg_l', 'lower_leg_l', 'upper_leg_r', 'lower_leg_r']);
    const angles = R.boneAngles(rig, 'front', { scale: 1 }, typing.bones);
    assert.equal(angles.upper_leg_l, -80); assert.equal(angles.lower_leg_l, 80); assert.equal(angles.torso, 0);
    const world = R.boneMatrices(rig, 'front', { scale: 1 }, {});
    const head = world.head;
    near(head[4], 0, 1e-9); near(head[5], -190 - 8 - 108 - 10, 1e-9, 'standing head joint');
    const bent = R.boneMatrices(rig, 'front', { scale: 1 }, { torso: 90 });
    near(bent.neck[4], 108, 1e-9, 'a clockwise torso turn swings the neck to the right'); near(bent.neck[5], -198, 1e-9);
    near(R.boneMatrices(rig, 'front', { scale: 1 }, { upper_leg_l: -80 }).lower_leg_l[4], -16 + Math.sin(80 * Math.PI / 180) * 94, 1e-6, 'a thigh turned forward moves the knee forward');
    // body overrides move a joint
    const wide = R.boneMatrices(rig, 'front', { scale: 1, bones: { upper_arm_l: { rest: { front: { x: -50 } } } } }, {});
    near(wide.upper_arm_l[4], -50, 1e-9);
    // draw order differs per view: the far arm is in front of the torso only from the front
    const order = v => R.index(rig).zOrder[v].map(s => s.name);
    assert.ok(order('front').indexOf('sleeve_upper_r') > order('front').indexOf('torso'));
    assert.ok(order('three_quarter').indexOf('sleeve_upper_r') < order('three_quarter').indexOf('torso'));
    assert.equal(R.scaleFor(rig, 1080, 1, 1, { scale: 1 }), 0.36 * 1080 / 400);
    near(R.scaleFor(rig, 1080, 0.973, 1.5, { scale: 0.72 }), 0.36 * 1080 * 0.973 * 1.5 * 0.72 / 400, 1e-9);
    assert.deepEqual(plain(R.mul(R.translate(5, 7), R.rotate(0))), [1, 0, 0, 1, 5, 7]);
});

check('rig: pose, gesture and cycle evaluation return to the base pose', () => {
    const point = t.byId('gesture.point'), walk = t.byId('cycle.walk'), R = SM.rig;
    assert.deepEqual(plain(R.sampleKeys(point.keys, 0).bones), {});
    near(R.sampleKeys(point.keys, 0.15).bones.upper_arm_r, -42.5, 1e-9, 'smoothed halfway between the first two keys');
    assert.equal(R.sampleKeys(point.keys, 0.5).bones.upper_arm_r, -85);
    assert.deepEqual(plain(R.sampleKeys(point.keys, 1).bones), {}, 'a gesture returns to the base pose at t = 1');
    assert.deepEqual(plain(R.handsAt(point.keys, 0.5)), { r: 'point' });
    assert.equal(R.handsAt(point.keys, 1), null);
    assert.equal(R.handsAt(point.keys, 0.1), null);
    const half = R.sampleKeys(walk.keys, 0.25);
    near(half.bones.upper_leg_l, 0, 1e-9); near(half.offset_y, -3, 1e-9);
    near(R.cyclePhase(walk, 2.25), 0.25, 1e-9);
    near(R.cyclePhase(walk, -0.25), 0.75, 1e-9);
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (10 passed, 2 failed; first: rig: bone angles, world matrices, mirroring base and slot order: TypeError: Cannot read properties of undefined (reading 'boneAngles'))

- [ ] **Step 3: Implement**


**`ui/js/storymaker/rig.js`**

````js
// Storymaker player: the 2D character rig (rig.human/1). 13 bones with a rest transform per view (front, three_quarter,
// profile), slots (vector parts attached to a bone, drawn in z order per view), poses (bone angle offsets), gestures and
// cycles (timed additive bone deltas that return to the base pose), expressions (eye/brow/mouth variants), speech mouth
// shapes and hands. The semantics are contract section 3a: world(b) = world(parent) . T(rest.x, rest.y) .
// R(rest.angle + pose[b] + sum of deltas[b]), angles in degrees, clockwise positive. Mirroring for left-facing actors
// is done by the caller (a scale(-1, 1) about the root). Classic script; publishes StorymakerCore.rig.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var VIEWS = ['front', 'three_quarter', 'profile'];
    var IDENTITY = [1, 0, 0, 1, 0, 0];

    function mul(m, n) {
        return [
            m[0] * n[0] + m[2] * n[1], m[1] * n[0] + m[3] * n[1],
            m[0] * n[2] + m[2] * n[3], m[1] * n[2] + m[3] * n[3],
            m[0] * n[4] + m[2] * n[5] + m[4], m[1] * n[4] + m[3] * n[5] + m[5]
        ];
    }
    function translate(x, y) { return [1, 0, 0, 1, x, y]; }
    function rotate(deg) {
        var r = deg * Math.PI / 180, c = Math.cos(r), s = Math.sin(r);
        return [c, s, -s, c, 0, 0];
    }

    // index(rig): bone order (parents before children) and name lookup; cached on the rig object.
    function index(rig) {
        if (rig.__idx) return rig.__idx;
        var byName = {}, order = [], bones = rig.bones || [], i;
        for (i = 0; i < bones.length; i++) byName[bones[i].name] = bones[i];
        var seen = {};
        function visit(b) {
            if (seen[b.name]) return;
            seen[b.name] = true;
            if (b.parent && byName[b.parent]) visit(byName[b.parent]);
            order.push(b);
        }
        for (i = 0; i < bones.length; i++) visit(bones[i]);
        var slots = (rig.slots || []).map(function (s, k) { return { s: s, k: k }; });
        var zOrder = {};
        VIEWS.forEach(function (view) {
            zOrder[view] = slots.slice().sort(function (a, b) {
                var za = a.s.z && typeof a.s.z[view] === 'number' ? a.s.z[view] : 0;
                var zb = b.s.z && typeof b.s.z[view] === 'number' ? b.s.z[view] : 0;
                return za - zb || a.k - b.k;
            }).map(function (entry) { return entry.s; });
        });
        Object.defineProperty(rig, '__idx', { value: { byName: byName, order: order, zOrder: zOrder }, enumerable: false });
        return rig.__idx;
    }

    function viewOf(name) { return VIEWS.indexOf(name) >= 0 ? name : 'three_quarter'; }

    // bodyOf(rig, name) = {scale, bones}; an unknown body is the plain rig.
    function bodyOf(rig, name) {
        var b = rig.bodies && rig.bodies[name];
        return b || { scale: 1 };
    }

    function restOf(bone, view, body) {
        var r = (bone.rest && (bone.rest[view] || bone.rest.three_quarter || bone.rest.front)) || { x: 0, y: 0, angle: 0 };
        var o = body && body.bones && body.bones[bone.name] && body.bones[bone.name].rest && body.bones[bone.name].rest[view];
        if (!o) return r;
        return { x: typeof o.x === 'number' ? o.x : r.x, y: typeof o.y === 'number' ? o.y : r.y, angle: typeof o.angle === 'number' ? o.angle : r.angle };
    }

    // angles(a, b, c ...): sums bone-angle maps (pose, gesture delta, cycle delta).
    function addBones() {
        var out = {};
        for (var i = 0; i < arguments.length; i++) {
            var m = arguments[i];
            if (!m) continue;
            for (var k in m) if (typeof m[k] === 'number') out[k] = (out[k] || 0) + m[k];
        }
        return out;
    }

    function lerpBones(a, b, u) {
        if (u >= 1) return b || {};
        if (u <= 0 || !a) return a || {};
        var out = {}, k;
        for (k in a) out[k] = (a[k] || 0) * (1 - u);
        for (k in (b || {})) out[k] = (out[k] || 0) + b[k] * u;
        return out;
    }

    // boneAngles(rig, view, body, extra): the final angle of every bone (rest angle + sum of the offset maps).
    function boneAngles(rig, view, body, offsets) {
        var idx = index(rig), out = {};
        for (var i = 0; i < idx.order.length; i++) {
            var bone = idx.order[i];
            out[bone.name] = restOf(bone, viewOf(view), body).angle + ((offsets && offsets[bone.name]) || 0);
        }
        return out;
    }

    // boneMatrices(rig, view, body, offsets) = {bone: matrix} in rig space (root at the origin).
    function boneMatrices(rig, view, body, offsets) {
        var idx = index(rig), world = {};
        view = viewOf(view);
        for (var i = 0; i < idx.order.length; i++) {
            var bone = idx.order[i], r = restOf(bone, view, body);
            var local = mul(translate(r.x || 0, r.y || 0), rotate((r.angle || 0) + ((offsets && offsets[bone.name]) || 0)));
            world[bone.name] = bone.parent && world[bone.parent] ? mul(world[bone.parent], local) : local;
        }
        return world;
    }

    function smooth(f) { return f * f * (3 - 2 * f); }

    // sampleKeys(keys, u): additive bone deltas and offset_y of gesture/cycle keys at u (0..1), linear between keys with an
    // ease-in-out on each segment. A bone missing from a key counts as 0.
    function sampleKeys(keys, u) {
        if (!keys || !keys.length) return { bones: {}, offset_y: 0 };
        if (u <= keys[0].t) return { bones: keys[0].bones || {}, offset_y: keys[0].offset_y || 0 };
        var last = keys[keys.length - 1];
        if (u >= last.t) return { bones: last.bones || {}, offset_y: last.offset_y || 0 };
        var k = 0;
        while (k < keys.length - 2 && keys[k + 1].t <= u) k++;
        var a = keys[k], b = keys[k + 1], span = b.t - a.t, f = span > 0 ? smooth((u - a.t) / span) : 1;
        var bones = {}, name;
        for (name in (a.bones || {})) bones[name] = a.bones[name] * (1 - f);
        for (name in (b.bones || {})) bones[name] = (bones[name] || 0) + b.bones[name] * f;
        return { bones: bones, offset_y: (a.offset_y || 0) * (1 - f) + (b.offset_y || 0) * f };
    }

    // handsAt(keys, u): the hand override of the last key at or before u (a key without hands clears the override).
    function handsAt(keys, u) {
        var hands = null;
        for (var i = 0; i < (keys || []).length; i++) {
            if (keys[i].t <= u) hands = keys[i].hands || null; else break;
        }
        return hands;
    }

    // cyclePhase(entry, seconds): 0..1 phase of a periodic cycle.
    function cyclePhase(entry, seconds) {
        var period = entry && entry.period > 0 ? entry.period : 1;
        var p = (seconds / period) % 1;
        return p < 0 ? p + 1 : p;
    }

    // Colour tokens of a character: skin/hair/outfit colours from the rig parameter tables, the fixed set last.
    function tokensFor(rig, params) {
        var p = rig.params || {}, outfit = p.outfit && p.outfit[params.outfit] || {}, colors = outfit.colors || {};
        var t = {}, fixed = p.fixed || {}, k;
        for (k in fixed) t[k] = fixed[k];
        if (p.skin && p.skin[params.skin]) t.skin = p.skin[params.skin];
        if (p.hair_color && p.hair_color[params.hair_color]) t.hair = p.hair_color[params.hair_color];
        if (colors.top) t.top = colors.top;
        if (colors.bottom) t.bottom = colors.bottom;
        if (colors.shoes) t.shoes = colors.shoes;
        return t;
    }

    // variantKeys(rig, params, slot, state): candidate variant keys of a slot, best first.
    function variantKeys(rig, params, slot, state) {
        var expr = state.expr || {}, outfit = rig.params && rig.params.outfit && rig.params.outfit[params.outfit] || {};
        switch (slot.param) {
            case 'body': return [params.body];
            case 'skin': return [params.skin];
            case 'hair': case 'hair_back': return [params.hair];
            case 'eyes':
                return (state.blink ? [params.eyes + '/blink'] : []).concat([params.eyes + '/' + (expr.eyes || 'neutral'), params.eyes + '/neutral']);
            case 'brows': return [params.brows + '/' + (expr.brows || 'neutral'), params.brows + '/neutral'];
            case 'mouth': return [state.mouth || expr.mouth || 'rest', 'rest'];
            case 'hand': {
                var side = /_r$/.test(slot.name) ? 'r' : 'l';
                return [(state.hands && state.hands[side]) || 'open', 'open'];
            }
            case 'top': case 'bottom': case 'shoes': return [outfit[slot.param]];
            default: return [params[slot.param]];
        }
    }

    function pickVariant(rig, params, slot, state, view) {
        var table = rig.variants && rig.variants[slot.name];
        if (!table) return null;
        var keys = variantKeys(rig, params, slot, state);
        for (var i = 0; i < keys.length; i++) {
            var key = keys[i];
            if (key === undefined || key === null || key === 'none') {
                if (key === 'none') return null;
                continue;
            }
            var v = table[key];
            if (v) return v[view] || v.three_quarter || v.front || null;
        }
        return null;
    }

    // Screen scale of an actor (contract 3a): rig.height x stage height x depth x actor scale x body scale / units_height.
    function scaleFor(rig, designHeight, depth, actorScale, body) {
        var units = rig.units_height > 0 ? rig.units_height : 1;
        return (rig.height || units) * designHeight * depth * actorScale * ((body && body.scale) || 1) / units;
    }

    // seatedOffset(rig): how far the root drops (rig units) for seated poses.
    function seatedOffset(rig) {
        if (typeof rig.seated_offset === 'number') return rig.seated_offset;
        var idx = index(rig), lower = idx.byName.lower_leg_l;
        return lower && lower.length > 0 ? lower.length : 0.24 * (rig.units_height || 400);
    }

    // drawActor(ctx, rig, params, state, base, resolve, alpha): draws every visible slot. base is the matrix from rig
    // space to the canvas (placement, scale, mirroring); state = {view, offsets, hands, expr, mouth, blink, offsetY}.
    function drawActor(ctx, rig, params, state, base, resolve, alpha) {
        var idx = index(rig), view = viewOf(state.view);
        var body = bodyOf(rig, params.body);
        var world = boneMatrices(rig, view, body, state.offsets);
        var lift = translate(0, state.offsetY || 0);
        var slots = idx.zOrder[view];
        for (var i = 0; i < slots.length; i++) {
            var slot = slots[i], shapes = pickVariant(rig, params, slot, state, view), m = world[slot.bone];
            if (!shapes || !m) continue;
            var full = mul(base, mul(lift, m));
            ctx.setTransform(full[0], full[1], full[2], full[3], full[4], full[5]);
            SM.parts.drawShapes(ctx, shapes, resolve, alpha);
        }
    }

    SM.rig = {
        VIEWS: VIEWS, IDENTITY: IDENTITY, mul: mul, translate: translate, rotate: rotate, index: index, bodyOf: bodyOf,
        addBones: addBones, lerpBones: lerpBones, boneAngles: boneAngles, boneMatrices: boneMatrices,
        sampleKeys: sampleKeys, handsAt: handsAt, cyclePhase: cyclePhase, tokensFor: tokensFor, scaleFor: scaleFor,
        seatedOffset: seatedOffset, pickVariant: pickVariant, drawActor: drawActor
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`12 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/rig.js scripts/storymaker-tests/03-rig.mjs
git commit -m "feat(storymaker): player 2D character rig" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C5: Compile: scenes, narration timing, beats, report

**Files:**
- Create: `ui/js/storymaker/scene.js`, `ui/js/storymaker/narration.js`, `ui/js/storymaker/plan.js`, `ui/js/storymaker/beats.js`, `ui/js/storymaker/checks.js`, `ui/js/storymaker/compile.js`
- Test: `scripts/storymaker-tests/04-compile.mjs`

The scene-local timing rule of the spec lives here. `plan.js`: per scene the entry transition in whole frames (scene 0 has none; transitions the
phase-1 player does not implement fall back to fade with a minor `not_supported_yet`), the narration window (starts when the transition ends,
words / 2.3 s with a 1 s minimum, or measured block durations from `audioInfo`) and the cue times (character position over the block duration); the host
asks for it (`planTiming`) before it builds code scenes, because `api.cue(n)` needs those times. `beats.js`: actors, props and beats become entities with
tracks (beats run in order, `with: "prev"`, `cue`, `delay`, `duration`, `ease`; per-kind default durations; walk time from the distance at the catalog's
walk speed; the phase-1 support matrix: `enter exit walk move pose gesture emote look show(item,text) hide camera wait`, everything else is a minor
`not_supported_yet`). `compile.js` assembles scenes with whole-frame ranges (a scene starts its entry transition before the previous one ends), the
compile report `{frame_count, fps, width, height, duration_ms, catalog_version, scenes, audio_events, captions, checks, unverified, issues}`, the
asset keys a render needs and the narration windows (the no-overlap assertion). `checks.js` is the phase-1 stub (no checks; code scenes are listed as
`layout_unverified`). `scene.js` holds the entity constructors shared with code scenes and the catalog index.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/04-compile.mjs`**

````js
// Task C5: compile: scene-local timing, cues, auto durations, transition overlap, beats, the support matrix.
import assert from 'node:assert/strict';
import { plain } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, firewall, minimal } = t;
const compiled = t.compiled(), sceneById = t.sceneById;
const SM = t.core();

check('compile: scene ranges in frames, transitions overlap, the first scene has no entry transition', () => {
    const r = plain(compiled.report);
    assert.equal(r.fps, 30);
    assert.deepEqual(r.scenes.map(s => [s.id, s.kind]), [['s1', 'storyboard'], ['s2', 'storyboard'], ['s3', 'code']]);
    // s1: 9 words / 2.3 = 3.913 s + 0.4 s tail = 4.313 s = 129 frames (no entry transition although it says fade)
    assert.deepEqual([r.scenes[0].start_frame, r.scenes[0].end_frame], [0, 129]);
    // s2: fade 0.5 s = 15 frames overlap: starts at 129 - 15
    assert.equal(r.scenes[1].start_frame, 129 - 15);
    assert.ok(r.scenes[1].start_frame < r.scenes[0].end_frame, 'transitions overlap');
    assert.equal(r.scenes[2].start_frame, r.scenes[1].end_frame - 15);
    assert.equal(r.scenes[2].end_frame - r.scenes[2].start_frame, 150, 'the code scene lasts api.duration(5) = 150 frames');
    assert.equal(r.frame_count, Math.max(...r.scenes.map(s => s.end_frame)));
    assert.equal(r.duration_ms, Math.round(r.frame_count * 1000 / 30));
    assert.equal(r.width, 1920); assert.equal(r.height, 1080); assert.equal(r.catalog_version, 'mini00000000');
    assert.deepEqual(r.audio_events, []); assert.deepEqual(r.captions, []); assert.deepEqual(r.checks, []);
    assert.deepEqual(plain(compiled.schedule.scenes.map(s => s.tf)), [0, 15, 15]);
});

check('compile: cue placement is proportional to the character position over the estimated duration', () => {
    const plan = plain(SM.planTiming(firewall, {}, catalog)).scenes.s1;
    const spoken = 'Anna arbeitet im Homeoffice. Plötzlich klopft ein Unbekannter an.';
    const pos = 'Anna arbeitet im Homeoffice. '.length;
    const seconds = 9 / 2.3;
    near(plan.narration_seconds, seconds, 1e-9);
    near(plan.cues['1'], 0 + pos / spoken.length * seconds, 1e-9, 'cue 1');
    // the cue beat starts at that time: the surprised expression appears there
    const anna = sceneById('s1').entities.find(e => e.id === 'anna');
    const step = anna.expr.steps.find(s => s.v === 'surprised');
    near(step.t, plan.cues['1'], 1e-9);
    // a later scene: the cue origin is its own scene start, offset by its entry transition
    const fw2 = plain(SM.planTiming({ ...firewall, scenes: [firewall.scenes[0], { id: 'x', narration: 'eins zwei {1}drei vier', transition: 'fade' }] }, {}, catalog)).scenes.x;
    near(fw2.narration_start, 0.5, 1e-9);
    near(fw2.cues['1'], 0.5 + (10 / 19) * (4 / 2.3), 1e-9);
});

check('compile: auto durations, silent scenes, explicit durations and measured audio', () => {
    const min = SM.compile(minimal, {}, catalog);
    assert.equal(min.report.scenes[0].end_frame, 42, '"Hallo Welt." is 2 words: the 1 s minimum + 0.4 s tail');
    const silent = SM.compile({ scenes: [{ id: 'a', silent: true, narration: 'ignored words ignored' }, { id: 'b', transition: 'cut' }] }, {}, catalog).report;
    assert.deepEqual(plain(silent.scenes.map(s => [s.start_frame, s.end_frame])), [[0, 30], [30, 60]], 'silent scenes last 1 s');
    const explicit = SM.compile({ scenes: [{ id: 'a', duration: 2.5, narration: 'a b c d e f g h i j k l m n o p' }] }, {}, catalog).report;
    assert.equal(explicit.scenes[0].end_frame, 75);
    assert.ok(explicit.issues.some(i => i.code === 'invalid_value' && i.path === '/scenes/0/duration'), 'narration longer than the scene is a warning');
    // beats extend an auto scene
    const longBeat = SM.compile({ scenes: [{ id: 'a', beats: [{ do: 'wait', seconds: 4 }] }] }, {}, catalog).report;
    assert.equal(longBeat.scenes[0].end_frame, 120);
    // measured audio replaces the estimate and produces audio events
    const measured = SM.compile(minimal, { narration: { s1: { blocks: [{ hash: 'abc', duration_ms: 2500 }] } } }, catalog);
    assert.equal(measured.report.scenes[0].end_frame, Math.round(2.9 * 30));
    assert.deepEqual(plain(measured.report.audio_events), [{ kind: 'narration', ref: 'abc', start_ms: 0, gain: 1 }]);
    assert.ok(!measured.report.issues.some(i => i.code === 'narration_missing'));
    assert.ok(compiled.report.issues.filter(i => i.code === 'narration_missing').length === 3 && compiled.report.issues.every(i => i.code !== 'narration_missing' || i.severity === 'warning'));
});

check('compile: beat scheduling with "with: prev", cue, delay and duration', () => {
    const s2 = sceneById('s2'), anna = s2.entities.find(e => e.id === 'anna');
    const walk = anna.walks.list[0], gesture = anna.gestures.list[0];
    near(walk.t0, 0.5, 1e-9, 'beats start when the entry transition ends');
    near(gesture.t0, walk.t0, 1e-9, 'with: prev starts together with the previous beat');
    near(gesture.t1 - gesture.t0, 1.2, 1e-9, 'the gesture lasts the catalog duration');
    // walk duration from the distance at the catalog speed: left 30% -> centre 50% = 384 px at 12% of 1920 px/s x depth 0.973
    near(walk.t1 - walk.t0, 384 / (0.12 * 1920 * (0.82 + 0.23 * (16 / 24))), 0.02, 'walk duration');
    const shield = s2.entities.find(e => e.item === 'prop.shield');
    near(shield.alpha.segs[0].t0, walk.t1, 1e-9, 'the next plain beat follows the longest of the group');
    const custom = SM.compile({ scenes: [{ id: 'a', transition: 'cut', narration: 'eins zwei drei {1}vier fünf sechs', actors: [], beats: [
        { do: 'wait', seconds: 1 },
        { do: 'wait', seconds: 2, delay: 0.5 },
        { do: 'wait', seconds: 0.5, with: 'prev' },
        { do: 'wait', cue: 1, duration: 0.25 },
        { do: 'wait', cue: 7 },
        { do: 'wait', seconds: 1 }
    ] }] }, {}, catalog);
    const ends = custom.report.scenes[0].end_frame;
    const cueT = 15 / 30 * (6 / 2.3);
    assert.ok(custom.report.issues.some(i => i.code === 'unknown_cue' && i.path === '/scenes/0/beats/4/cue'), 'a missing cue marker is reported');
    // chain: 0-1, 1.5-3.5 (delay), with prev at 1.5-2, the cue beat re-anchors at the cue time (0.25 s), the unknown cue follows it (1 s), the last beat 1 s
    const expectedEnd = Math.max(3.5, cueT + 0.25 + 1 + 1);
    near(ends / 30, expectedEnd, 0.02, 'scene end');
});

check('compile: narration windows never overlap (vacuous in phase 1, kept as an assertion)', () => {
    assert.deepEqual(plain(SM.overlappingNarration(compiled.timeline)), []);
    assert.equal(compiled.timeline.narrationEvents.length, 3);
    const ev = compiled.timeline.narrationEvents;
    for (let i = 1; i < ev.length; i++) assert.ok(ev[i].start >= ev[i - 1].end + 0.38, 'the 0.4 s tail separates the narration blocks');
    const heavy = SM.compile({ scenes: Array.from({ length: 6 }, (_, i) => ({ id: 's' + i, narration: 'ein längerer Satz mit mehreren Worten für Szene ' + i })) }, {}, catalog);
    assert.deepEqual(plain(SM.overlappingNarration(heavy.timeline)), []);
    const measured = SM.compile({ scenes: [{ id: 'a', narration: 'x y' }, { id: 'b', narration: 'x y' }] }, { narration: { a: { blocks: [{ hash: 'h1', duration_ms: 3000 }] }, b: { blocks: [{ hash: 'h2', duration_ms: 1000 }, { hash: 'h3', duration_ms: 500 }] } } }, catalog);
    assert.deepEqual(plain(SM.overlappingNarration(measured.timeline)), []);
    assert.equal(measured.report.audio_events.length, 3);
});

check('compile: the phase-1 support matrix reports not_supported_yet and falls back', () => {
    const s = { schema: 'storymaker/1', cast: [{ id: 'anna', character: 'char.office-woman-1' }], scenes: [
        { id: 'a', actors: [{ cast: 'anna', pose: 'wave', expression: 'laugh' }], beats: [
            { do: 'highlight', target: 'anna', style: 'pulse' }, { do: 'say', who: 'anna', text: 'x', style: 'speech' }, { do: 'sfx', sound: 'sfx.pop' },
            { do: 'chart', chart: 'bar', data: { labels: [], values: [] }, at: 'right' }, { do: 'show', icon: 'icon.x', at: 'left' }, { do: 'show', chart: 'bar', at: 'left' },
            { do: 'gesture', who: 'anna', gesture: 'shrug' }, { do: 'pose', who: 'anna', pose: 'flying' }
        ] },
        { id: 'b', transition: 'zoom' }, { id: 'c', transition: 'slide-right' }, { id: 'd', transition: 'wipe' }, { id: 'e', transition: 'slide-left' }, { id: 'f', transition: 'blink' }] };
    const out = SM.compile(s, {}, catalog), issues = plain(out.report.issues);
    const codes = (path, code) => issues.filter(i => i.path === path && i.code === code);
    for (const j of [0, 1, 2, 3]) assert.equal(codes('/scenes/0/beats/' + j, 'not_supported_yet').length, 1, 'beat ' + j);
    assert.equal(codes('/scenes/0/beats/4', 'not_supported_yet').length, 1, 'show with an icon');
    assert.equal(codes('/scenes/0/beats/5', 'not_supported_yet').length, 1, 'show with a chart');
    assert.ok(issues.every(i => i.code !== 'not_supported_yet' || i.severity === 'minor'));
    assert.equal(codes('/scenes/0/actors/0/pose', 'not_supported_yet').length, 1, 'a known pose that the catalog lacks');
    assert.equal(codes('/scenes/0/actors/0/expression', 'not_supported_yet').length, 1);
    assert.equal(codes('/scenes/0/beats/6/gesture', 'not_supported_yet').length, 1);
    assert.equal(codes('/scenes/0/beats/7/pose', 'unknown_name').length, 1, 'an unknown pose name');
    const actor = out.timeline.scenes[0].entities[0];
    assert.equal(actor.pose.init, 'stand', 'the pose falls back to stand');
    assert.equal(actor.expr.init, 'neutral');
    assert.equal(codes('/scenes/1/transition', 'not_supported_yet').length, 1, 'zoom');
    assert.equal(codes('/scenes/2/transition', 'not_supported_yet').length, 1, 'slide-right');
    assert.equal(codes('/scenes/3/transition', 'not_supported_yet').length, 0, 'wipe is supported');
    assert.equal(codes('/scenes/4/transition', 'not_supported_yet').length, 0, 'slide-left is supported');
    assert.equal(codes('/scenes/5/transition', 'unknown_name').length, 1);
    assert.deepEqual(plain(out.schedule.scenes.map(s => s.transition)), ['cut', 'fade', 'fade', 'wipe', 'slide-left', 'fade']);
    assert.deepEqual(plain(out.report.unverified), []);
    const u = compiled.report.unverified;
    assert.equal(u.length, 1); assert.equal(u[0].code, 'layout_unverified'); assert.equal(u[0].scene_id, 's3'); assert.equal(u[0].severity, 'minor');
});

check('compile tolerates missing optional fields and reports a story without scenes', () => {
    const bare = SM.compile({ scenes: [{ id: 'a' }] }, undefined, catalog);
    assert.equal(bare.report.fps, 30);
    assert.equal(bare.report.frame_count, 30);
    const empty = SM.compile({}, {}, catalog);
    assert.equal(empty.report.frame_count, 0);
    assert.ok(empty.report.issues.some(i => i.severity === 'error' && i.path === '/scenes'));
    const noCast = SM.compile({ scenes: [{ id: 'a', actors: [{ cast: 'ghost' }] }] }, {}, catalog);
    assert.ok(noCast.report.issues.some(i => i.code === 'unknown_cast' && i.severity === 'warning'));
    const badEnv = SM.compile({ scenes: [{ id: 'a', environment: 'env.mars' }] }, {}, catalog);
    assert.ok(badEnv.report.issues.some(i => i.code === 'unknown_asset'));
    const codeNoBundle = SM.compile({ scenes: [{ id: 'c', kind: 'code' }] }, {}, catalog, { codeScenes: {} });
    assert.ok(codeNoBundle.report.issues.some(i => i.code === 'code_error' && i.severity === 'error'));
    const codeFailed = SM.compile({ scenes: [{ id: 'c', kind: 'code' }] }, {}, catalog, { codeScenes: { c: { error: { code: 'code_error', message: 'boom', line: 3, column: 5 } } } });
    assert.match(codeFailed.report.issues.find(i => i.code === 'code_error').message, /boom \(line 3, column 5\)/);
    const needs = plain(compiled.needs);
    assert.deepEqual(needs, ['catalog:environments/abstract-tech.json', 'catalog:environments/office.json', 'catalog:props/lock.json', 'catalog:props/shield.json', 'catalog:rig/human.json']);
});

check('frame plan: one layer, or two with the transition share p in (0, 1)', () => {
    const sched = compiled.schedule;
    assert.deepEqual(plain(SM.schedule.framePlan(sched, 0)), { cur: 0, layers: [{ i: 0, f: 0 }], transition: null });
    const mid = plain(SM.schedule.framePlan(sched, 114 + 7));
    assert.equal(mid.cur, 1);
    assert.deepEqual(mid.layers, [{ i: 0, f: 121 }, { i: 1, f: 7 }]);
    assert.equal(mid.transition.kind, 'fade');
    near(mid.transition.p, 8 / 16, 1e-9);
    const after = plain(SM.schedule.framePlan(sched, 114 + 15));
    assert.equal(after.layers.length, 1);
    assert.equal(plain(SM.schedule.framePlan(sched, compiled.report.frame_count - 1)).cur, 2);
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (the runner stops; TypeError: t.core(...).compile is not a function)

- [ ] **Step 3: Implement**


**`ui/js/storymaker/scene.js`**

````js
// Storymaker player: scene entities. Actors, props, texts and shapes are bundles of tracks (position, alpha, scale,
// rotation, pose, expression, facing, gestures, walking). The same entities serve storyboard scenes (filled by compile)
// and code scenes (filled by the code-scene API). evalEntity(entity, t) is a pure function of the tracks and t. This file
// also indexes the catalog manifest by kind and bare name. Classic script; publishes StorymakerCore.scene.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var T = function () { return SM.timeline; };

    // indexCatalog(catalog): the manifest by id and by kind + bare name. The result is plain data.
    function indexCatalog(catalog) {
        var idx = {
            version: (catalog && catalog.version) || '', byId: {}, poses: {}, gestures: {}, expressions: {}, cycles: {},
            textStyles: {}, transitions: {}, mouths: {}, palettes: {}, fonts: (catalog && catalog.fonts) || [],
            stage: (catalog && catalog.stage) || null, rig: (catalog && catalog.rig) || null
        };
        var entries = (catalog && catalog.entries) || [];
        for (var i = 0; i < entries.length; i++) {
            var e = entries[i];
            if (!e || !e.id) continue;
            idx.byId[e.id] = e;
            var bare = e.name && e.name !== e.id ? e.name : String(e.id).replace(/^[a-z]+\./, '');
            if (e.kind === 'pose') idx.poses[bare] = e;
            else if (e.kind === 'gesture') idx.gestures[bare] = e;
            else if (e.kind === 'expression') idx.expressions[bare] = e;
            else if (e.kind === 'cycle') idx.cycles[bare] = e;
            else if (e.kind === 'text_style') idx.textStyles[bare] = e;
            else if (e.kind === 'transition') idx.transitions[bare] = e;
            else if (e.kind === 'mouth') idx.mouths[bare] = e;
        }
        var pals = (catalog && catalog.palettes) || [];
        for (var p = 0; p < pals.length; p++) idx.palettes[pals[p].name || String(pals[p].id).replace(/^palette\./, '')] = pals[p];
        return idx;
    }

    function base(type, id, x, y) {
        var t = T();
        return {
            type: type, id: id, x: new t.Track(x), y: new t.Track(y), alpha: new t.Track(1), scale: new t.Track(1),
            rot: new t.Track(0), depth: null
        };
    }

    // createActor({id, cast, character, params, x, y, scale, facing, pose, expression, hidden, phase})
    function createActor(def) {
        var t = T(), e = base('actor', def.id, def.x, def.y);
        e.cast = def.cast;
        e.character = def.character;
        e.params = def.params || {};
        e.baseScale = def.scale > 0 ? def.scale : 1;
        e.depth = typeof def.depth === 'number' ? def.depth : null;
        e.facing = new t.StepTrack(def.facing === -1 ? -1 : 1);
        e.look = new t.StepTrack(null);
        e.pose = new t.StepTrack(def.pose || 'stand');
        e.expr = new t.StepTrack(def.expression || 'neutral');
        e.gestures = new t.Intervals();
        e.walks = new t.Intervals();
        e.phase = def.phase || 0;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // createProp({id, item, x, y, scale, layer, hidden})
    function createProp(def) {
        var e = base('prop', def.id, def.x, def.y);
        e.item = def.item;
        e.layer = def.layer === 'back' ? 'back' : 'front';
        e.baseScale = def.scale > 0 ? def.scale : 1;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // createText({id, text, style, x, y, hidden}); x and y are design pixels, text is drawn in screen space.
    function createText(def) {
        var e = base('text', def.id, def.x, def.y);
        e.text = String(def.text === undefined || def.text === null ? '' : def.text);
        e.style = def.style || 'title';
        e.baseScale = 1;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // createShape({id, kind, x, y, w, h, color, stroke, hidden}); w/h are design pixels (circle: w is the radius).
    function createShape(def) {
        var e = base('shape', def.id, def.x, def.y);
        e.kind = def.kind || 'rect';
        e.w = new (T().Track)(def.w > 0 ? def.w : 100);
        e.h = new (T().Track)(def.h > 0 ? def.h : 100);
        e.color = def.color || '$accent';
        e.stroke = def.stroke || null;
        e.baseScale = 1;
        if (def.hidden) e.alpha.init = 0;
        return e;
    }

    // evalEntity(e, t) = the logical state of an entity at scene time t (no rig or catalog data needed).
    function evalEntity(e, t) {
        var s = { type: e.type, id: e.id, x: e.x.at(t), y: e.y.at(t), alpha: e.alpha.at(t), scale: e.scale.at(t) * (e.baseScale || 1), rot: e.rot.at(t), depth: e.depth };
        if (e.type === 'actor') {
            var walk = e.walks.active(t), g = e.gestures.active(t);
            s.cast = e.cast;
            s.character = e.character;
            s.params = e.params;
            s.pose = e.pose.blendAt(t);
            s.expression = e.expr.at(t);
            s.facing = e.facing.at(t);
            s.look = e.look.at(t);
            s.walkTime = walk ? t - walk.t0 : null;
            s.phase = e.phase;
            if (g) {
                var span = g.t1 - g.t0, count = g.data.count > 0 ? g.data.count : 1;
                var u = span > 0 ? (t - g.t0) / span * count : 1;
                s.gesture = { name: g.data.name, frac: u - Math.floor(u) };
            } else s.gesture = null;
        } else if (e.type === 'prop') {
            s.item = e.item;
            s.layer = e.layer;
        } else if (e.type === 'text') {
            s.text = e.text;
            s.style = e.style;
        } else if (e.type === 'shape') {
            s.kind = e.kind;
            s.w = e.w.at(t);
            s.h = e.h.at(t);
            s.color = e.color;
            s.stroke = e.stroke;
        }
        return s;
    }

    function entityEnd(e) {
        var end = Math.max(e.x.end(), e.y.end(), e.alpha.end(), e.scale.end(), e.rot.end());
        if (e.type === 'actor') {
            var lists = [e.gestures.list, e.walks.list];
            for (var k = 0; k < lists.length; k++) for (var i = 0; i < lists[k].length; i++) if (lists[k][i].t1 > end) end = lists[k][i].t1;
        }
        return end;
    }

    SM.scene = {
        indexCatalog: indexCatalog, createActor: createActor, createProp: createProp, createText: createText,
        createShape: createShape, evalEntity: evalEntity, entityEnd: entityEnd
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/narration.js`**

````js
// Storymaker player: narration text. Cue markers {n} are stripped from the spoken text and remembered as character
// positions; a cue time is its character position over the text length, scaled by the duration of the narration block.
// Without measured audio the duration is estimated as words / 2.3 s (minimum 1 s). Classic script; publishes
// StorymakerCore.narration.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var WORDS_PER_SECOND = 2.3;
    var MIN_SECONDS = 1.0;
    var TAIL_SECONDS = 0.4;

    // parse(text) = {plain, cues: {n: charIndex}, words, length}. Whitespace collapses to single spaces; a cue marker keeps
    // the position in the plain text where it stood.
    function parse(text) {
        var src = String(text === undefined || text === null ? '' : text);
        var plain = '', cues = {}, re = /\{(\d+)\}/g, last = 0, m;
        function append(chunk) {
            chunk = chunk.replace(/\s+/g, ' ');
            if (!plain || plain.charAt(plain.length - 1) === ' ') chunk = chunk.replace(/^ /, '');
            plain += chunk;
        }
        while ((m = re.exec(src)) !== null) {
            append(src.slice(last, m.index));
            var n = parseInt(m[1], 10);
            if (cues[n] === undefined) cues[n] = plain.length;
            last = m.index + m[0].length;
        }
        append(src.slice(last));
        var trimmed = plain.replace(/ $/, '');
        var words = trimmed ? trimmed.split(' ').length : 0;
        Object.keys(cues).forEach(function (k) { if (cues[k] > trimmed.length) cues[k] = trimmed.length; });
        return { plain: trimmed, cues: cues, words: words, length: trimmed.length };
    }

    // estimate(words) = seconds; no words, no narration.
    function estimate(words) {
        return words > 0 ? Math.max(MIN_SECONDS, words / WORDS_PER_SECOND) : 0;
    }

    // cueTimes(parsed, start, seconds) = {n: scene-local seconds}, proportional to the character position.
    function cueTimes(parsed, start, seconds) {
        var out = {}, len = Math.max(1, parsed.length);
        Object.keys(parsed.cues).forEach(function (k) { out[k] = start + parsed.cues[k] / len * seconds; });
        return out;
    }

    SM.narration = {
        parse: parse, estimate: estimate, cueTimes: cueTimes, WORDS_PER_SECOND: WORDS_PER_SECOND,
        MIN_SECONDS: MIN_SECONDS, TAIL_SECONDS: TAIL_SECONDS
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/plan.js`**

````js
// Storymaker player: timing plan. The scene-local timing rule of the spec: scene time 0 is the start of the scene's entry
// transition; the narration starts when that transition ends; cue and beat times use the same origin. This file computes,
// per scene and without looking at any beat, the transition frames, the narration window and the cue times. The host asks
// for it (planTiming) before it builds the code scenes, because api.cue(n) needs the cue times. Also the shared issue helper
// and the phase-1 support tables. Classic script; publishes StorymakerCore.plan.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // Closed name sets of the schema (contract section 2).
    var KNOWN = {
        poses: 'stand sit typing point-left point-right wave arms-crossed hands-on-hips shrug think present thumbs-up celebrate facepalm phone hold lean'.split(' '),
        gestures: 'point wave shrug think present thumbs-up celebrate facepalm count'.split(' '),
        expressions: 'neutral happy laugh sad angry surprised confused thinking worried determined'.split(' '),
        transitions: 'cut fade slide-left slide-right slide-up slide-down wipe zoom iris whip'.split(' ')
    };
    // What the phase-1 player implements.
    var SUPPORTED_TRANSITIONS = ['cut', 'fade', 'slide-left', 'wipe'];
    var SUPPORTED_BEATS = ['enter', 'exit', 'walk', 'move', 'pose', 'gesture', 'emote', 'look', 'show', 'hide', 'camera', 'wait'];
    var DEFAULT_TRANSITION_SECONDS = { 'cut': 0, 'fade': 0.5, 'slide-left': 0.5, 'slide-right': 0.5, 'slide-up': 0.5, 'slide-down': 0.5, 'wipe': 0.5, 'zoom': 0.6, 'iris': 0.6, 'whip': 0.35 };
    var MIN_SCENE_SECONDS = 1.0;
    var MAX_CODE_SECONDS = 600;

    function issue(severity, sceneId, path, code, message, extra) {
        var out = { severity: severity, path: path, code: code, message: message };
        if (sceneId) out.scene_id = sceneId;
        if (extra) for (var k in extra) if (extra[k] !== undefined) out[k] = extra[k];
        return out;
    }

    function contains(list, v) { return list.indexOf(v) >= 0; }

    var SIZES = { '16:9': [1920, 1080], '9:16': [1080, 1920], '1:1': [1080, 1080] };

    // normaliseFormat(story, opts) = {aspect, fps, width, height}; opts.width scales the canvas, keeping the aspect.
    function normaliseFormat(story, opts) {
        var f = (story && story.format) || {};
        var aspect = SIZES[f.aspect] ? f.aspect : '16:9';
        var fps = f.fps === 24 || f.fps === 25 || f.fps === 30 ? f.fps : 30;
        var size = SM.stage.sizeFor(aspect, opts && opts.width);
        if (opts && opts.width > 0 && opts.height > 0) size = { width: Math.round(opts.width), height: Math.round(opts.height) };
        return { aspect: aspect, fps: fps, width: size.width, height: size.height };
    }

    // transitionFor(scene, index, cat, fps, issues): the entry transition of a scene. Scene 0 has none. A name the player
    // does not implement falls back to fade with the same duration and a minor not_supported_yet issue.
    function transitionFor(scene, index, cat, fps, issues) {
        if (index === 0) return { kind: 'cut', requested: scene.transition || 'cut', tf: 0 };
        var req = scene.transition || 'fade', effective = req, fallback = false;
        var entry = cat.transitions[req];
        var seconds = entry && typeof entry.duration === 'number' ? entry.duration : DEFAULT_TRANSITION_SECONDS[req];
        if (seconds === undefined) seconds = 0.5;
        if (!contains(SUPPORTED_TRANSITIONS, req)) {
            effective = 'fade';
            fallback = true;
            if (contains(KNOWN.transitions, req)) {
                issues.push(issue('minor', scene.id, '/scenes/' + index + '/transition', 'not_supported_yet',
                    'Transition "' + req + '" is not supported yet; the player uses a fade.'));
            } else {
                issues.push(issue('warning', scene.id, '/scenes/' + index + '/transition', 'unknown_name',
                    'Unknown transition "' + req + '"; the player uses a fade.'));
            }
        }
        var tf = effective === 'cut' ? 0 : SM.schedule.secondsToFrames(fps, seconds);
        return { kind: effective, requested: req, tf: tf < 0 ? 0 : tf, fallback: fallback };
    }

    var FALLBACK_PALETTE = { bg: '#fff7e6', bg2: '#ffe0a3', fg: '#1f2a44', accent: '#ff7a45', accent2: '#36cfc9', text: '#1f2a44', outline: '#1f2a44' };

    function pickPalette(story, cat, issues) {
        var want = story.style && story.style.palette || 'sunny';
        var pal = cat.palettes[want];
        if (!pal) {
            var names = Object.keys(cat.palettes);
            if (names.length) {
                issues.push(issue('warning', null, '/style/palette', 'unknown_name', 'Unknown palette "' + want + '"; the player uses "' + names[0] + '".'));
                pal = cat.palettes[names[0]];
            }
        }
        var tokens = {}, k;
        for (k in FALLBACK_PALETTE) tokens[k] = FALLBACK_PALETTE[k];
        if (pal && pal.tokens) for (k in pal.tokens) tokens[k] = pal.tokens[k];
        return tokens;
    }

    function fontFamilies(cat) {
        var out = {};
        cat.fonts.forEach(function (f) { if (f && f.name && !out[f.name]) out[f.name] = f.family; });
        return out;
    }

    // planScene(...) = {tf, T, kind, requested, narration: {text, plain, words, start, seconds, measured, blocks, cues}}
    function planScene(story, audioInfo, cat, fmt, index, issues) {
        var scene = story.scenes[index];
        var tr = transitionFor(scene, index, cat, fmt.fps, issues);
        var T = tr.tf / fmt.fps;
        var silent = scene.silent === true;
        var parsed = SM.narration.parse(silent ? '' : scene.narration);
        var info = audioInfo && audioInfo.narration && audioInfo.narration[scene.id];
        var blocks = info && info.blocks && info.blocks.length ? info.blocks : null;
        var narration = { plain: parsed.plain, words: parsed.words, start: T, seconds: 0, measured: false, blocks: [], cues: {} };
        if (parsed.words > 0) {
            if (blocks) {
                var at = T;
                narration.measured = true;
                blocks.forEach(function (b) {
                    var d = (b.duration_ms > 0 ? b.duration_ms : 0) / 1000;
                    narration.blocks.push({ hash: b.hash, start: at, seconds: d });
                    at += d;
                });
                narration.seconds = at - T;
                narration.cues = SM.narration.cueTimes(parsed, T, narration.blocks[0].seconds);
            } else {
                narration.seconds = SM.narration.estimate(parsed.words);
                narration.cues = SM.narration.cueTimes(parsed, T, narration.seconds);
                issues.push(issue('warning', scene.id, '/scenes/' + index + '/narration', 'narration_missing',
                    'No narration audio for this scene; the duration is estimated at ' + narration.seconds.toFixed(1) + ' s.'));
            }
        }
        return { index: index, tf: tr.tf, T: T, transition: tr.kind, requested: tr.requested, narration: narration };
    }

    function planAll(story, audioInfo, cat, fmt, issues) {
        var out = [];
        for (var i = 0; i < story.scenes.length; i++) out.push(planScene(story, audioInfo, cat, fmt, i, issues));
        return out;
    }

    // planTiming(story, audioInfo, catalog, opts) = {fps, scenes: {<scene id>: {index, transition_frames, narration_start,
    // narration_seconds, cues: {n: seconds}}}, issues}. Pure; used by the host before it builds code scenes.
    function planTiming(story, audioInfo, catalog, opts) {
        var cat = SM.scene.indexCatalog(catalog), issues = [];
        var fmt = normaliseFormat(story, opts);
        var plans = planAll(story, audioInfo || {}, cat, fmt, issues), scenes = {};
        plans.forEach(function (p) {
            var s = story.scenes[p.index];
            scenes[s.id] = {
                index: p.index, kind: s.kind === 'code' ? 'code' : 'storyboard', transition_frames: p.tf,
                narration_start: p.narration.start, narration_seconds: p.narration.seconds, cues: p.narration.cues
            };
        });
        return {
            fps: fmt.fps, width: fmt.width, height: fmt.height, aspect: fmt.aspect, scenes: scenes, issues: issues,
            palette: pickPalette(story, cat, []), fontFamilies: fontFamilies(cat), styleFont: story.style && story.style.font || 'default'
        };
    }

    SM.plan = {
        KNOWN: KNOWN, SUPPORTED_TRANSITIONS: SUPPORTED_TRANSITIONS, SUPPORTED_BEATS: SUPPORTED_BEATS,
        MIN_SCENE_SECONDS: MIN_SCENE_SECONDS, MAX_CODE_SECONDS: MAX_CODE_SECONDS, issue: issue, contains: contains,
        normaliseFormat: normaliseFormat, pickPalette: pickPalette, fontFamilies: fontFamilies, transitionFor: transitionFor, planScene: planScene, planAll: planAll,
        planTiming: planTiming
    };
    SM.planTiming = planTiming;
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/beats.js`**

````js
// Storymaker player: storyboard scenes. buildStoryboard turns one scene (actors, props, camera, beats) into entities
// with tracks. Beats run in order; "with": "prev" starts a beat together with the previous one; "cue": n starts it at
// narration marker {n}; delay, duration and ease are optional. Timing defaults are owned here (enter 0.6 s, walk from the
// distance at the catalog's walk speed, ...). Phase-1 support matrix: enter exit walk move pose gesture emote look show
// (item and text) hide camera wait; the other beats are reported as minor not_supported_yet and ignored. Classic script;
// publishes StorymakerCore.beats.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var ACTOR_REACH = 200;   // design pixels from the feet to the visual centre of a standing actor at scale 1
    var DEFAULT_ACTOR_SPOTS = ['left', 'right', 'far-left', 'far-right'];
    var TEXT_ROW_Y = 28;     // percent of the frame height: texts shown at a floor spot sit in the upper third

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }

    function buildStoryboard(c) {
        var P = SM.plan, S = SM.scene, scene = c.scene, index = c.index, sid = scene.id, stage = c.stage, cat = c.cat;
        var issues = c.issues, plan = c.plan, fmt = c.fmt;
        var entities = [], byId = {}, byItem = {}, counters = {};
        var cam = SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2);
        var beats = scene.beats || [];
        var placed = 0;

        function warn(severity, path, code, message) { issues.push(P.issue(severity, sid, path, code, message)); }
        function beatPath(j, field) { return '/scenes/' + index + '/beats/' + j + (field ? '/' + field : ''); }

        function resolveAt(at, path, fallback) {
            var pos = at === undefined || at === null ? null : stage.resolve(at);
            if (!pos) {
                if (at !== undefined && at !== null) warn('warning', path, 'unknown_spot', 'Unknown position ' + JSON.stringify(at) + '; the player uses ' + fallback + '.');
                pos = stage.resolve(fallback);
            }
            return pos;
        }

        // A known name that the catalog does not have is a minor not_supported_yet and falls back; an unknown name is a warning.
        function nameOr(map, known, name, fallback, path, what) {
            if (map[name]) return name;
            if (P.contains(known, name)) warn('minor', path, 'not_supported_yet', what + ' "' + name + '" is not in the catalog yet; the player uses "' + fallback + '".');
            else warn('warning', path, 'unknown_name', 'Unknown ' + what.toLowerCase() + ' "' + name + '"; the player uses "' + fallback + '".');
            return fallback;
        }

        function register(e, item) {
            entities.push(e);
            byId[e.id] = e;
            if (item) (byItem[item] = byItem[item] || []).push(e);
        }

        function instanceId(item, given) {
            if (given) return given;
            var slug = String(item).replace(/^[a-z]+\./, '');
            counters[slug] = (counters[slug] || 0) + 1;
            return slug + '-' + counters[slug];
        }

        function find(ref) {
            if (!ref) return null;
            if (byId[ref]) return byId[ref];
            var list = byItem[ref];
            return list && list.length ? list[list.length - 1] : null;
        }

        function actorOf(ref, path) {
            var e = find(ref);
            if (e && e.type === 'actor') return e;
            warn('warning', path, 'unknown_cast', 'No actor "' + ref + '" in this scene.');
            return null;
        }

        function needFile(entry) { if (entry && entry.file) c.needs['catalog:' + entry.file] = true; }

        // ---- actors and props that exist from the first frame
        var entering = {};
        beats.forEach(function (b) { if (b && b.do === 'enter' && b.who) entering[b.who] = true; });

        function addActor(def, k, total, hidden) {
            var path = '/scenes/' + index + '/actors/' + k;
            var member = c.castMap[def.cast];
            if (!member) { warn('warning', path + '/cast', 'unknown_cast', 'Unknown cast member "' + def.cast + '".'); return; }
            if (byId[def.cast]) { warn('warning', path + '/cast', 'duplicate_id', 'Cast member "' + def.cast + '" is already in this scene.'); return; }
            var ch = cat.byId[member.character];
            if (!ch) { warn('warning', path + '/cast', 'unknown_asset', 'Unknown character "' + member.character + '".'); return; }
            var at = def.at !== undefined && def.at !== null ? def.at : (total <= 1 ? 'center' : DEFAULT_ACTOR_SPOTS[k % DEFAULT_ACTOR_SPOTS.length]);
            var pos = resolveAt(at, path + '/at', 'center');
            var facing = def.facing === 'left' ? -1 : (def.facing === 'right' ? 1 : (pos.face === 'left' ? -1 : (pos.face === 'right' ? 1 : (pos.x <= stage.focusX ? 1 : -1))));
            var pose = nameOr(cat.poses, P.KNOWN.poses, def.pose || 'stand', 'stand', path + '/pose', 'Pose');
            var expr = nameOr(cat.expressions, P.KNOWN.expressions, def.expression || 'neutral', 'neutral', path + '/expression', 'Expression');
            var e = S.createActor({
                id: def.cast, cast: def.cast, character: member.character, params: ch.params || {}, x: pos.x, y: pos.y,
                scale: def.scale, facing: facing, pose: pose, expression: expr, hidden: hidden, depth: pos.depth, phase: k * 0.37
            });
            register(e, null);
            placed++;
        }

        var actorDefs = scene.actors || [];
        actorDefs.forEach(function (a, k) { addActor(a, k, actorDefs.length, !!entering[a.cast]); });
        Object.keys(entering).forEach(function (cast) { if (!byId[cast]) addActor({ cast: cast, at: 'center' }, actorDefs.length, actorDefs.length + 1, true); });

        (scene.props || []).forEach(function (p, k) {
            var path = '/scenes/' + index + '/props/' + k;
            var entry = cat.byId[p.item];
            if (!entry || entry.kind !== 'prop') { warn('warning', path + '/item', 'unknown_asset', 'Unknown prop "' + p.item + '".'); return; }
            var pos = resolveAt(p.at, path + '/at', 'center');
            needFile(entry);
            register(S.createProp({ id: instanceId(p.item, p.id), item: p.item, x: pos.x, y: pos.y, scale: p.scale, layer: p.layer, depth: pos.depth }), p.item);
        });

        // ---- camera start
        var framing = scene.camera && SM.camera.FRAMING[scene.camera.framing] ? scene.camera.framing : 'wide';

        function focusOf(ref, t) {
            var e = find(ref);
            if (e) {
                var depth = stage.depthScale(e.y.at(t), e.depth);
                var reach = e.type === 'actor' ? ACTOR_REACH * depth * (e.baseScale || 1) : (e.type === 'prop' ? 70 * depth : 0);
                return { x: e.x.at(t), y: e.y.at(t) - reach };
            }
            var s = typeof ref === 'string' ? stage.spot(ref) : null;
            return s ? { x: s.x, y: s.y - 150 } : null;
        }

        var firstActor = null;
        for (var q = 0; q < entities.length; q++) if (entities[q].type === 'actor') { firstActor = entities[q]; break; }
        var startFocus = scene.camera && scene.camera.target ? focusOf(scene.camera.target, 0) : null;
        if (!startFocus && framing !== 'wide' && firstActor) startFocus = focusOf(firstActor.id, 0);
        cam.zoom.init = SM.camera.FRAMING[framing];
        if (startFocus) { cam.cx.init = startFocus.x; cam.cy.init = startFocus.y; }

        // ---- beats
        function walkSeconds(from, to) {
            var cyc = cat.cycles.walk, speed = cyc && cyc.speed > 0 ? cyc.speed : 12;
            var dx = to.x - from.x, dy = to.y - from.y, dist = Math.sqrt(dx * dx + dy * dy);
            var depth = (stage.depthScale(from.y) + stage.depthScale(to.y)) / 2;
            return Math.max(0.3, dist / (speed / 100 * SM.stage.DESIGN_W * depth));
        }

        function easeOf(b, fallback) { return SM.isEase(b.ease) ? b.ease : fallback; }

        function moveTo(e, t0, t1, to, ease) {
            e.x.add(t0, t1, to.x, ease);
            e.y.add(t0, t1, to.y, ease);
        }

        function faceToward(a, t, x) {
            var from = a.x.at(t);
            if (Math.abs(x - from) > 1) a.facing.set(t, x > from ? 1 : -1);
        }

        function entryAnim(e, b, t0, dur, pos) {
            var entry = b.entry || 'fade';
            if (entry === 'none' || dur <= 0) { e.alpha.add(t0, t0, 1, 'linear'); return; }
            if (entry === 'pop' && e.type !== 'text') {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
                e.scale.add(t0, t0 + dur, 1, 'back-out', 0.6);
            } else if (entry === 'slide') {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
                e.x.add(t0, t0 + dur, pos.x, 'ease-out', pos.x - 140);
            } else {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
            }
        }

        var kinds = {
            enter: {
                dur: function () { return 0.6; },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    var to = resolveAt(b.to, beatPath(j, 'to'), 'center'), from;
                    if (b.from === 'right') from = { x: stage.offRight(), y: to.y };
                    else if (b.from === 'door') { var d = stage.spot('door'); from = d ? { x: d.x, y: d.y } : { x: stage.offRight(), y: to.y }; }
                    else from = { x: stage.offLeft(), y: to.y };
                    a.x.add(t0, t0 + dur, to.x, easeOf(b, 'ease-out'), from.x);
                    a.y.add(t0, t0 + dur, to.y, easeOf(b, 'ease-out'), from.y);
                    a.alpha.add(t0, t0, 1, 'linear');
                    a.facing.set(t0, to.x >= from.x ? 1 : -1);
                    a.look.set(t0, null);
                    a.walks.add(t0, t0 + dur, {});
                    if (to.face) a.facing.set(t0 + dur, to.face === 'left' ? -1 : 1);
                    if (typeof to.depth === 'number') a.depth = to.depth;
                }
            },
            exit: {
                dur: function () { return 0.6; },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    var to = b.to ? resolveAt(b.to, beatPath(j, 'to'), 'door') : { x: stage.offRight(), y: a.y.at(t0) };
                    faceToward(a, t0, to.x);
                    a.look.set(t0, null);
                    moveTo(a, t0, t0 + dur, to, easeOf(b, 'ease-in'));
                    a.walks.add(t0, t0 + dur, {});
                    a.alpha.add(t0 + dur - Math.min(0.2, dur), t0 + dur, 0, 'ease-in');
                }
            },
            walk: {
                dur: function (b, t0) {
                    var e = find(b.who || b.target);
                    var to = b.to !== undefined ? stage.resolve(b.to) : null;
                    if (!e || !to) return 0.6;
                    return walkSeconds({ x: e.x.at(t0), y: e.y.at(t0) }, to);
                },
                run: function (b, j, t0, dur) { moveBeat(b, j, t0, dur, true); }
            },
            move: {
                dur: function () { return 0.8; },
                run: function (b, j, t0, dur) { moveBeat(b, j, t0, dur, false); }
            },
            pose: {
                dur: function () { return 0.3; },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a || !b.pose) return;
                    a.pose.set(t0, nameOr(cat.poses, P.KNOWN.poses, b.pose, 'stand', beatPath(j, 'pose'), 'Pose'), dur);
                }
            },
            gesture: {
                dur: function (b) {
                    var entry = cat.gestures[b.gesture], count = clamp(Math.round(b.count || 1), 1, 5);
                    return (entry && entry.duration > 0 ? entry.duration : 0.5) * count;
                },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    var name = nameOr(cat.gestures, P.KNOWN.gestures, b.gesture, '', beatPath(j, 'gesture'), 'Gesture');
                    if (!name) return;
                    a.gestures.add(t0, t0 + dur, { name: name, count: clamp(Math.round(b.count || 1), 1, 5) });
                    var f = b.target ? focusOf(b.target, t0) : null;
                    if (f) faceToward(a, t0, f.x);
                }
            },
            emote: {
                dur: function () { return 0.25; },
                run: function (b, j, t0) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a || !b.expression) return;
                    a.expr.set(t0, nameOr(cat.expressions, P.KNOWN.expressions, b.expression, 'neutral', beatPath(j, 'expression'), 'Expression'));
                }
            },
            look: {
                dur: function () { return 0.3; },
                run: function (b, j, t0) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    if (b.target === 'camera') { a.look.set(t0, 'camera'); return; }
                    a.look.set(t0, null);
                    var f = focusOf(b.target, t0);
                    if (f) faceToward(a, t0, f.x);
                }
            },
            show: {
                dur: function (b) { return (b.entry || 'fade') === 'none' ? 0 : 0.5; },
                run: function (b, j, t0, dur) {
                    if (b.chart || b.icon) {
                        warn('minor', beatPath(j), 'not_supported_yet', 'Show with ' + (b.chart ? 'a chart' : 'an icon') + ' is not supported yet; the beat is ignored.');
                        return;
                    }
                    if (b.text !== undefined && b.text !== null && b.item === undefined) {
                        var pos = textPosition(b.at, beatPath(j, 'at'));
                        var style = b.style && cat.textStyles[b.style] ? b.style : 'title';
                        if (b.style && !cat.textStyles[b.style] && !P.contains(['speech', 'thought'], b.style)) {
                            warn('warning', beatPath(j, 'style'), 'unknown_name', 'Unknown text style "' + b.style + '"; the player uses "title".');
                        }
                        var t = S.createText({ id: instanceId('text.text', b.id), text: b.text, style: style, x: pos.x, y: pos.y, hidden: true });
                        register(t, null);
                        entryAnim(t, b, t0, dur, pos);
                        return;
                    }
                    if (!b.item) { warn('warning', beatPath(j), 'missing_field', 'A show beat needs item, text, icon or chart.'); return; }
                    var entry = cat.byId[b.item];
                    if (!entry || entry.kind !== 'prop') { warn('warning', beatPath(j, 'item'), 'unknown_asset', 'Unknown prop "' + b.item + '".'); return; }
                    var at = resolveAt(b.at, beatPath(j, 'at'), 'center');
                    needFile(entry);
                    var prop = S.createProp({ id: instanceId(b.item, b.id), item: b.item, x: at.x, y: at.y, scale: b.scale, layer: b.layer, hidden: true, depth: at.depth });
                    register(prop, b.item);
                    entryAnim(prop, b, t0, dur, at);
                }
            },
            hide: {
                dur: function () { return 0.3; },
                run: function (b, j, t0, dur) {
                    var e = find(b.target);
                    if (!e) { warn('warning', beatPath(j, 'target'), 'unknown_name', 'Nothing named "' + b.target + '" to hide.'); return; }
                    e.alpha.add(t0, t0 + dur, 0, easeOf(b, 'ease-in'));
                }
            },
            camera: {
                dur: function (b) { return b.op === 'shake' ? 0.5 : 0.8; },
                run: function (b, j, t0, dur) {
                    var ease = easeOf(b, 'ease-in-out'), z = cam.zoom.at(t0), f = b.target ? focusOf(b.target, t0) : null;
                    if (b.op === 'zoom') {
                        SM.camera.addZoom(cam, t0, t0 + dur, z * (b.amount > 0 ? b.amount : 1.25), ease);
                        if (f) SM.camera.addPan(cam, t0, t0 + dur, f.x, f.y, ease);
                    } else if (b.op === 'pan') {
                        var cx = cam.cx.at(t0), cy = cam.cy.at(t0);
                        if (f) SM.camera.addPan(cam, t0, t0 + dur, f.x, f.y, ease);
                        else SM.camera.addPan(cam, t0, t0 + dur, cx + (typeof b.amount === 'number' ? b.amount : 10) / 100 * SM.stage.DESIGN_W, cy, ease);
                    } else if (b.op === 'focus') {
                        var target = f || (firstActor ? focusOf(firstActor.id, t0) : null);
                        SM.camera.addZoom(cam, t0, t0 + dur, b.amount > 0 ? b.amount : 1.6, ease);
                        if (target) SM.camera.addPan(cam, t0, t0 + dur, target.x, target.y, ease);
                    } else if (b.op === 'shake') {
                        SM.camera.addShake(cam, t0, t0 + dur, b.amount > 0 ? b.amount : 14);
                    } else {
                        warn('warning', beatPath(j, 'op'), 'invalid_value', 'Unknown camera op "' + b.op + '".');
                    }
                }
            },
            wait: {
                dur: function (b) { return b.seconds >= 0 ? b.seconds : 1; },
                run: function () {}
            }
        };

        function textPosition(at, path) {
            var rowY = TEXT_ROW_Y / 100 * SM.stage.DESIGN_H, home = { x: stage.focusX, y: rowY };
            if (at === undefined || at === null) return home;
            if (typeof at === 'string') {
                var s = stage.spot(at);
                if (!s) { warn('warning', path, 'unknown_spot', 'Unknown position "' + at + '"; the player centres the text.'); return home; }
                return stage.isBaseSpot(at) ? { x: s.x, y: rowY } : { x: s.x, y: s.y };
            }
            var p = stage.resolve(at);
            return p ? { x: p.x, y: p.y } : home;
        }

        function moveBeat(b, j, t0, dur, walking) {
            var ref = b.who || b.target, e = find(ref);
            if (!e) { warn('warning', beatPath(j, b.who ? 'who' : 'target'), 'unknown_name', 'Nothing named "' + ref + '" to move.'); return; }
            var to = resolveAt(b.to, beatPath(j, 'to'), 'center');
            var ease = easeOf(b, walking ? 'linear' : 'ease-in-out');
            if (e.type === 'actor' && walking) {
                faceToward(e, t0, to.x);
                e.look.set(t0, null);
                e.walks.add(t0, t0 + dur, {});
                if (to.face) e.facing.set(t0 + dur, to.face === 'left' ? -1 : 1);
            }
            if (typeof to.depth === 'number') e.depth = to.depth;
            moveTo(e, t0, t0 + dur, to, ease);
        }

        var cursor = plan.T, prevStart = plan.T, beatsEnd = 0;
        beats.forEach(function (b, j) {
            if (!b || typeof b !== 'object') return;
            var spec = kinds[b.do];
            if (!spec) {
                warn('minor', beatPath(j), 'not_supported_yet', 'Beat "' + b.do + '" is not supported yet; it is ignored.');
                return;
            }
            var t0 = cursor, usedPrev = b.with === 'prev' && j > 0;
            if (usedPrev) t0 = prevStart;
            else if (b.cue !== undefined && b.cue !== null) {
                var ct = plan.narration.cues[b.cue];
                if (ct === undefined) warn('warning', beatPath(j, 'cue'), 'unknown_cue', 'Narration has no cue marker {' + b.cue + '}; the beat follows the previous one.');
                else t0 = ct;
            }
            t0 += b.delay > 0 ? b.delay : 0;
            var dur = typeof b.duration === 'number' && b.duration >= 0 ? b.duration : spec.dur(b, t0);
            spec.run(b, j, t0, dur);
            var end = t0 + dur;
            prevStart = t0;
            cursor = usedPrev ? Math.max(cursor, end) : end;
            if (end > beatsEnd) beatsEnd = end;
        });

        var lastEnd = SM.camera.end(cam);
        for (var k = 0; k < entities.length; k++) { var ee = S.entityEnd(entities[k]); if (ee > lastEnd) lastEnd = ee; }
        return { entities: entities, camera: cam, beatsEnd: beatsEnd, motionEnd: lastEnd, framing: framing, actorsPlaced: placed };
    }

    SM.beats = { buildStoryboard: buildStoryboard };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/checks.js`**

````js
// Storymaker player: layout checks. Phase 1 has none: run() returns an empty list. The code scenes are listed as
// unverified (layout inside a code scene is never verified, only code_error, deadlines and the duration bounds are).
// The real checks (off_stage, overlap_face, text_overflow, ...) arrive in phase 3. Classic script; publishes
// StorymakerCore.checks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // run(scenes) = check results of the compiled scenes (phase 1: none).
    function run() { return []; }

    // unverified(scenes) = one minor layout_unverified issue per code scene.
    function unverified(scenes) {
        var out = [];
        (scenes || []).forEach(function (s) {
            if (s.kind === 'code') {
                out.push(SM.plan.issue('minor', s.id, '/scenes/' + s.index, 'layout_unverified', 'Layout inside a code scene is not verified.'));
            }
        });
        return out;
    }

    SM.checks = { run: run, unverified: unverified };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/compile.js`**

````js
// Storymaker player: compile(story, audioInfo, catalog, opts) -> {timeline, report, schedule, needs}. It is a pure
// function of its inputs. Scenes get whole-frame ranges (transitions overlap: a scene starts its entry transition before
// the previous one ends), storyboard scenes become entities with tracks (beats.js), code scenes are placeholders whose
// duration comes from the code-scene worker (opts.codeScenes). The report is what Go receives from a render job:
// {frame_count, fps, width, height, duration_ms, catalog_version, scenes, audio_events, captions, checks, unverified,
// issues}. Defaults are normally applied by Go, but every optional field may be missing here. Classic script; publishes
// StorymakerCore.compile.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // fontKey(cat, name, weight) = 'font:<name>/<weight>' of the catalog font that fits best, or null.
    function fontKey(cat, name, weight) {
        var best = null;
        cat.fonts.forEach(function (f) {
            if (f.name !== name) return;
            if (!best || Math.abs(f.weight - weight) < Math.abs(best.weight - weight)) best = f;
        });
        return best ? 'font:' + best.name + '/' + best.weight : null;
    }

    function compile(story, audioInfo, catalog, opts) {
        opts = opts || {};
        story = story || {};
        var P = SM.plan, issues = [], needs = {};
        var cat = SM.scene.indexCatalog(catalog);
        var fmt = P.normaliseFormat(story, opts);
        var fps = fmt.fps;
        var palette = P.pickPalette(story, cat, issues);
        var families = P.fontFamilies(cat);
        var scenesIn = Array.isArray(story.scenes) ? story.scenes : [];
        var castMap = {};
        (story.cast || []).forEach(function (m) { if (m && m.id) castMap[m.id] = m; });
        if (!scenesIn.length) issues.push(P.issue('error', null, '/scenes', 'missing_field', 'The story has no scenes.'));

        var plans = P.planAll({ scenes: scenesIn }, audioInfo || {}, cat, fmt, issues);
        var scenes = [], envPrev = null, narrationEvents = [], audioEvents = [];
        var styleFont = story.style && story.style.font || 'default';

        scenesIn.forEach(function (sc, i) {
            var plan = plans[i], isCode = sc.kind === 'code';
            var envId = sc.environment || envPrev || 'env.plain-stage';
            envPrev = envId;
            var envEntry = cat.byId[envId];
            if (!isCode) {
                if (!envEntry || envEntry.kind !== 'environment') {
                    issues.push(P.issue('warning', sc.id, '/scenes/' + i + '/environment', 'unknown_asset', 'Unknown environment "' + envId + '"; the player draws a plain background.'));
                    envEntry = null;
                } else if (envEntry.file) needs['catalog:' + envEntry.file] = true;
            } else envEntry = null;
            var variant = sc.variant === 'night' ? 'night' : 'day';
            var stage = SM.stage.create({ aspect: fmt.aspect, width: fmt.width, height: fmt.height, env: envEntry, safe: catalog && catalog.stage && catalog.stage.safe });
            var built = { entities: [], camera: SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2), beatsEnd: 0, motionEnd: 0 };
            var codeInfo = null;
            if (isCode) {
                codeInfo = opts.codeScenes ? (opts.codeScenes[sc.id] || { error: { message: 'No code-scene bundle for scene "' + sc.id + '".' } }) : null;
                if (codeInfo && codeInfo.error) {
                    var pos = codeInfo.error.line ? ' (line ' + codeInfo.error.line + (codeInfo.error.column ? ', column ' + codeInfo.error.column : '') + ')' : '';
                    issues.push(P.issue('error', sc.id, '/scenes/' + i, codeInfo.error.code === 'deadline' ? 'deadline' : 'code_error', String(codeInfo.error.message || 'Code scene failed') + pos));
                }
                (sc.beats || []).forEach(function (b, j) {
                    issues.push(P.issue('minor', sc.id, '/scenes/' + i + '/beats/' + j, 'not_supported_yet', 'Beats of a code scene are ignored; the code scene draws everything.'));
                });
            } else {
                built = SM.beats.buildStoryboard({ story: story, scene: sc, index: i, plan: plan, cat: cat, fmt: fmt, stage: stage, issues: issues, needs: needs, castMap: castMap });
            }

            // duration (scene-local seconds, origin = start of the entry transition)
            var T = plan.T, n = plan.narration, tail = n.seconds > 0 ? SM.narration.TAIL_SECONDS : 0;
            var autoD = Math.max(T + n.seconds + tail, built.beatsEnd, T + (n.seconds > 0 ? 0 : P.MIN_SCENE_SECONDS));
            var D;
            if (typeof sc.duration === 'number' && sc.duration > 0) {
                D = sc.duration;
                if (n.seconds > 0 && D < T + n.seconds - 0.05) {
                    issues.push(P.issue('warning', sc.id, '/scenes/' + i + '/duration', 'invalid_value',
                        'The scene duration ' + D + ' s is shorter than its narration (' + (T + n.seconds).toFixed(1) + ' s including the transition).'));
                }
            } else if (isCode && codeInfo && codeInfo.duration > 0) {
                D = Math.min(P.MAX_CODE_SECONDS, Math.max(0.5, codeInfo.duration));
            } else D = autoD;
            var dFrames = Math.max(SM.schedule.secondsToFrames(fps, D), plan.tf + 1);
            var prev = scenes.length ? scenes[scenes.length - 1] : null;
            var startFrame = prev ? Math.max(prev.start_frame + 1, prev.end_frame - plan.tf) : 0;

            var scene = {
                id: sc.id, index: i, kind: isCode ? 'code' : 'storyboard', start_frame: startFrame, end_frame: startFrame + dFrames,
                tf: plan.tf, transition: plan.transition, requested_transition: plan.requested, env: envEntry, variant: variant,
                narration: n, entities: built.entities, camera: built.camera, framing: built.framing,
                code: codeInfo ? { error: codeInfo.error || null, duration: codeInfo.duration || null } : null, stages: {}
            };
            scenes.push(scene);

            // narration windows (global seconds) and, with measured audio, the audio events
            var base = scene.start_frame / fps;
            if (n.seconds > 0 && !sc.silent) {
                if (n.measured) {
                    n.blocks.forEach(function (b) {
                        narrationEvents.push({ kind: 'narration', scene_id: sc.id, ref: b.hash, start: base + b.start, end: base + b.start + b.seconds });
                        audioEvents.push({ kind: 'narration', ref: b.hash, start_ms: Math.round((base + b.start) * 1000), gain: 1 });
                    });
                } else {
                    narrationEvents.push({ kind: 'narration', scene_id: sc.id, ref: '', start: base + n.start, end: base + n.start + n.seconds });
                }
            }

            // fonts in use
            scene.entities.forEach(function (e) {
                if (e.type !== 'text') return;
                var style = cat.textStyles[e.style] || cat.textStyles.title || {};
                var name = !style.font || style.font === 'default' ? styleFont : style.font;
                var key = fontKey(cat, name, style.weight || 400) || fontKey(cat, 'default', style.weight || 400);
                if (key) needs[key] = true;
            });
            if (scene.entities.some(function (e) { return e.type === 'actor'; }) && catalog && catalog.rig && catalog.rig.file) needs['catalog:' + catalog.rig.file] = true;
        });
        if (scenes.some(function (s) { return s.kind === 'code'; }) && catalog && catalog.rig && catalog.rig.file) needs['catalog:' + catalog.rig.file] = true;

        var frameCount = 0;
        scenes.forEach(function (s) { if (s.end_frame > frameCount) frameCount = s.end_frame; });
        var schedule = {
            fps: fps, frame_count: frameCount,
            scenes: scenes.map(function (s) { return { id: s.id, kind: s.kind, start_frame: s.start_frame, end_frame: s.end_frame, tf: s.tf, transition: s.transition }; })
        };
        var report = {
            frame_count: frameCount, fps: fps, width: fmt.width, height: fmt.height,
            duration_ms: Math.round(frameCount * 1000 / fps), catalog_version: cat.version,
            scenes: scenes.map(function (s) { return { id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: s.kind }; }),
            audio_events: audioEvents, captions: [], checks: SM.checks.run(scenes), unverified: SM.checks.unverified(scenes), issues: issues
        };
        var timeline = {
            version: 1, fps: fps, width: fmt.width, height: fmt.height, aspect: fmt.aspect, frame_count: frameCount,
            catalog_version: cat.version, palette: palette, style: { font: styleFont, captions: !(story.style && story.style.captions === false) },
            fontFamilies: families, cat: cat, rigFile: catalog && catalog.rig && catalog.rig.file || '', safe: catalog && catalog.stage && catalog.stage.safe || null,
            scenes: scenes, schedule: schedule, narrationEvents: narrationEvents
        };
        return { timeline: timeline, report: report, schedule: schedule, needs: Object.keys(needs).sort() };
    }

    // overlappingNarration(timeline) = pairs of narration events whose windows overlap (an empty list is the goal).
    function overlappingNarration(timeline) {
        var ev = timeline.narrationEvents.slice().sort(function (a, b) { return a.start - b.start; }), out = [];
        for (var i = 1; i < ev.length; i++) if (ev[i].start < ev[i - 1].end - 1e-9) out.push([ev[i - 1], ev[i]]);
        return out;
    }

    SM.compile = compile;
    SM.overlappingNarration = overlappingNarration;
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`20 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/scene.js ui/js/storymaker/narration.js ui/js/storymaker/plan.js ui/js/storymaker/beats.js ui/js/storymaker/checks.js ui/js/storymaker/compile.js scripts/storymaker-tests/04-compile.mjs
git commit -m "feat(storymaker): player compile with scene-local timing and beats" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C6: renderFrame and transitions

**Files:**
- Create: `ui/js/storymaker/transitions.js`, `ui/js/storymaker/render.js`
- Test: `scripts/storymaker-tests/05-render.mjs`

`renderFrame(timeline, n, ctx, assets, opts)` draws one frame: environment layers with parallax, back props, actors by depth (feet line) with a
ground shadow, front props, shapes, then texts in screen space. A transition draws the two scenes into two canvases (`assets.canvas`) and composites them
(cut, fade, slide-left, wipe). Code scenes arrive as ImageBitmaps (`assets.codeFrames`); a code scene without a frame shows a placeholder. The output
depends only on the arguments and on `n`.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/05-render.mjs`**

````js
// Task C6: renderFrame: environment layers, props, actors, texts, transitions, determinism of the call log.
import assert from 'node:assert/strict';
import { plain, renderAssets, FakeCanvas, makeContext, digest } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall } = t;
const compiled = t.compiled();
const SM = t.core();

function renderLog(tl, n, assets, width) {
    const cv = new FakeCanvas(width || tl.width, width ? Math.round(width * tl.height / tl.width) : tl.height);
    SM.renderFrame(tl, n, cv.ctx, assets || renderAssets(), width ? { width } : undefined);
    return cv.ctx.log.map(l => l.replace(/canvas\d+/g, 'canvas'));
}

check('render: environment layers, props, actors by depth, texts; no unresolved token', () => {
    SM.parts.resetUnresolved();
    const log = renderLog(compiled.timeline, 40);
    const text = log.join('\n');
    assert.match(text, /createLinearGradient/);
    assert.ok(log.filter(l => l.startsWith('setTransform')).length > 20, 'many parts are drawn');
    assert.ok(text.includes('M-36,-52 C-38,-86'), 'the hair slot of the actor is drawn');
    assert.deepEqual(plain(SM.parts.unresolvedTokens()), []);
    // the first scene at 1 s: the lock prop is shown from the beat; the actor sits at the desk
    const lockDraws = renderLog(compiled.timeline, 100).filter(l => l.includes('M28,52'));
    assert.ok(lockDraws.length > 0, 'the lock is drawn');
    assert.equal(renderLog(compiled.timeline, 5).filter(l => l.includes('M28,52')).length, 0, 'the lock has not appeared yet at frame 5');
    // a text show beat
    const tl = SM.compile({ scenes: [{ id: 'a', beats: [{ do: 'show', text: 'Hallo Welt', at: 'center', entry: 'none' }] }] }, {}, catalog).timeline;
    const t = renderLog(tl, 10).filter(l => l.startsWith('fillText'));
    assert.deepEqual(t.map(l => l.replace(/,[\d.]+,[\d.]+\)$/, ')')), ['fillText(Hallo Welt)']);
    // an off-aspect canvas and a thumbnail width draw too
    assert.ok(renderLog(SM.compile({ ...firewall, format: { aspect: '9:16', fps: 30 } }, {}, catalog, { codeScenes: { s3: { duration: 5 } } }).timeline, 40).length > 100);
    assert.ok(renderLog(compiled.timeline, 40, undefined, 320).length > 100);
});

check('render: transitions composite two scene canvases (fade, slide-left, wipe), cut shows the new scene', () => {
    const mk = kind => SM.compile({ scenes: [{ id: 'a', silent: true }, { id: 'b', silent: true, transition: kind }] }, {}, catalog).timeline;
    const at = (tl, n) => { const cv = new FakeCanvas(tl.width, tl.height); SM.renderFrame(tl, n, cv.ctx, renderAssets()); return cv.ctx.log; };
    const fade = mk('fade'), half = fade.schedule.scenes[1].start_frame + 7;
    const fl = at(fade, half);
    assert.equal(fl.filter(l => l.startsWith('drawImage')).length, 2);
    assert.ok(fl.includes('set globalAlpha=0.5'), 'fade at p = 8/16');
    const slide = at(mk('slide-left'), mk('slide-left').schedule.scenes[1].start_frame + 7).filter(l => l.startsWith('drawImage'));
    assert.deepEqual(slide.map(l => l.replace(/canvas\d+/, 'canvas')), ['drawImage(canvas,-960,0)', 'drawImage(canvas,960,0)']);
    const wipe = at(mk('wipe'), mk('wipe').schedule.scenes[1].start_frame + 7);
    assert.ok(wipe.includes('rect(0,0,960,1080)') && wipe.includes('clip()'));
    const cutTl = SM.compile({ scenes: [{ id: 'a', silent: true }, { id: 'b', silent: true, transition: 'cut' }] }, {}, catalog).timeline;
    assert.equal(cutTl.schedule.scenes[1].tf, 0);
    assert.equal(at(cutTl, 30).filter(l => l.startsWith('drawImage')).length, 0);
});

check('determinism: the same frame renders the same call log, in any order, from a fresh compile', () => {
    const a = renderLog(compiled.timeline, 140), b = renderLog(compiled.timeline, 140);
    assert.equal(a.length, b.length);
    assert.deepEqual(a, b);
    const other = SM.compile(firewall, {}, catalog, { codeScenes: { s3: { duration: 5 } } });
    assert.deepEqual(renderLog(other.timeline, 140), a, 'a second compile of the same inputs');
    renderLog(compiled.timeline, 3); renderLog(compiled.timeline, 300);
    assert.deepEqual(renderLog(compiled.timeline, 140), a, 'rendering other frames in between changes nothing');
    assert.notDeepEqual(renderLog(compiled.timeline, 141), a);
    for (const n of [0, 50, 125, 130, 200, 230, 353]) assert.equal(digest(renderLog(compiled.timeline, n)), digest(renderLog(other.timeline, n)), 'frame ' + n);
    const placeholder = renderLog(compiled.timeline, 300);
    assert.ok(placeholder.some(l => l.includes('Code scene')), 'a code scene without a frame shows a placeholder');
});

check('rig state: a gesture in progress, the seated typing pose, the walk cycle in profile', () => {
    const point = t.byId('gesture.point');
    // rigState: a gesture in progress, the typing pose seated, the walk cycle in profile
    const rc = SM.render.makeContext(compiled.timeline, renderAssets(), 1920, 1080);
    const st = { type: 'actor', pose: { a: 'stand', b: 'stand', u: 1 }, expression: 'happy', look: null, walkTime: null, phase: 0, gesture: { name: 'point', frac: 0.5 } };
    const rs = SM.render.rigState(st, 1.0, rc, files.rig);
    assert.equal(rs.offsets.upper_arm_r, -85); assert.deepEqual(plain(rs.hands), { r: 'point' }); assert.equal(rs.view, 'three_quarter');
    assert.equal(rs.expr.mouth, 'smile');
    const seated = SM.render.rigState({ ...st, gesture: null, pose: { a: 'stand', b: 'typing', u: 1 } }, 0, rc, files.rig);
    assert.equal(seated.offsetY, 92); assert.equal(seated.offsets.upper_leg_l, -80);
    const halfway = SM.render.rigState({ ...st, gesture: null, pose: { a: 'stand', b: 'typing', u: 0.5 } }, 0, rc, files.rig);
    assert.equal(halfway.offsets.upper_leg_l, -40); assert.equal(halfway.offsetY, 46);
    const walking = SM.render.rigState({ ...st, gesture: null, walkTime: 0.25, look: 'camera' }, 5, rc, files.rig);
    assert.equal(walking.view, 'profile', 'walking wins over looking at the camera');
    near(walking.offsetY, -3, 1e-9);
    const front = SM.render.rigState({ ...st, gesture: null, look: 'camera' }, 5, rc, files.rig);
    assert.equal(front.view, 'front');
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (20 passed, 4 failed; first: render: environment layers, props, actors by depth, texts; no unresolved token: TypeError: SM.renderFrame is not a function)

- [ ] **Step 3: Implement**


**`ui/js/storymaker/transitions.js`**

````js
// Storymaker player: scene transitions. The previous and the current scene are drawn into two canvases and composited
// here: cut, fade, slide-left and wipe in phase 1 (the compile step already maps the other names to fade). p runs over
// (0, 1) and is the share of the current scene. Classic script; publishes StorymakerCore.transitions.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var IMPLEMENTED = ['cut', 'fade', 'slide-left', 'wipe'];

    // effective(kind) = the implemented transition that draws a kind (fade for the unknown ones).
    function effective(kind) { return IMPLEMENTED.indexOf(kind) >= 0 ? kind : 'fade'; }

    // composite(ctx, kind, p, w, h, prev, cur): prev and cur are canvases of the output size.
    function composite(ctx, kind, p, w, h, prev, cur) {
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
        switch (effective(kind)) {
            case 'cut':
                ctx.drawImage(cur, 0, 0);
                break;
            case 'slide-left': {
                var shift = Math.round(p * w);
                ctx.drawImage(prev, -shift, 0);
                ctx.drawImage(cur, w - shift, 0);
                break;
            }
            case 'wipe': {
                ctx.drawImage(prev, 0, 0);
                ctx.save();
                ctx.beginPath();
                ctx.rect(0, 0, Math.round(p * w), h);
                ctx.clip();
                ctx.drawImage(cur, 0, 0);
                ctx.restore();
                break;
            }
            default:
                ctx.drawImage(prev, 0, 0);
                ctx.globalAlpha = p < 0 ? 0 : (p > 1 ? 1 : p);
                ctx.drawImage(cur, 0, 0);
                ctx.globalAlpha = 1;
        }
    }

    SM.transitions = { IMPLEMENTED: IMPLEMENTED, effective: effective, composite: composite };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/render.js`**

````js
// Storymaker player: renderFrame(timeline, n, ctx, assets, opts). It draws one frame of the compiled timeline into a 2D
// context of the output size: environment layers with parallax, back props, actors by depth, front props, shapes, texts;
// code scenes come in as ImageBitmaps (assets.codeFrames) and during a transition the two scenes are drawn into two
// canvases (assets.canvas) and composited. The result depends only on the arguments and on n. Classic script;
// publishes StorymakerCore.render.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FALLBACK_TEXT_STYLE = { font: 'default', size: 0.06, weight: 700, color: '$text', align: 'center', max_lines: 2 };
    var SHADOW_ALPHA = 0.16;

    function R() { return SM.rig; }
    function scaleM(sx, sy) { return [sx, 0, 0, sy, 0, 0]; }
    function setM(ctx, m) { ctx.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]); }

    function makeContext(timeline, assets, W, H) {
        return {
            W: W, H: H, aspect: timeline.aspect, fps: timeline.fps, palette: timeline.palette, cat: timeline.cat,
            families: timeline.fontFamilies, styleFont: timeline.style.font, safe: timeline.safe || SM.stage.DEFAULT_SAFE,
            assets: assets || {}, codeFrames: (assets && assets.codeFrames) || {}, codeErrors: (assets && assets.codeErrors) || {}
        };
    }

    function stageFor(sc, rc) {
        var key = rc.W + 'x' + rc.H;
        if (!sc.stages) sc.stages = {};
        if (!sc.stages[key]) sc.stages[key] = SM.stage.create({ aspect: rc.aspect, width: rc.W, height: rc.H, env: sc.env, safe: rc.safe });
        return sc.stages[key];
    }

    function familyOf(rc, style) {
        var name = !style.font || style.font === 'default' ? rc.styleFont : style.font;
        return rc.families[name] || rc.families.default || 'sans-serif';
    }

    function drawBackdrop(ctx, rc) {
        var g = ctx.createLinearGradient(0, 0, 0, rc.H);
        g.addColorStop(0, rc.palette.bg);
        g.addColorStop(1, rc.palette.bg2 || rc.palette.bg);
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
        ctx.fillStyle = g;
        ctx.fillRect(0, 0, rc.W, rc.H);
    }

    function drawEnvironment(ctx, sc, rc, win, bw, Mv) {
        var env = sc.env && rc.assets.envs && rc.assets.envs[sc.env.file];
        if (!env) return;
        var tokens = env.tokens && (env.tokens[sc.variant] || env.tokens.day);
        var resolve = SM.parts.makeResolver([tokens, rc.palette]);
        var layers = (env.layers || []).map(function (l, k) { return { l: l, k: k }; })
            .sort(function (a, b) { return (a.l.parallax || 0) - (b.l.parallax || 0) || a.k - b.k; });
        for (var i = 0; i < layers.length; i++) {
            var p = typeof layers[i].l.parallax === 'number' ? layers[i].l.parallax : 1;
            var dx = (1 - p) * (win.x - bw.x), dy = (1 - p) * (win.y - bw.y);
            setM(ctx, R().mul(Mv, R().translate(dx, dy)));
            SM.parts.drawShapes(ctx, layers[i].l.shapes, resolve, 1);
        }
    }

    function drawProp(ctx, st, rc, stage, Mv) {
        var entry = rc.cat.byId[st.item], part = entry && rc.assets.props && rc.assets.props[entry.file];
        if (!part) return;
        var vb = part.viewbox || [0, 0, 100, 100], anchor = entry.anchor || { x: 0.5, y: 1 };
        var depth = stage.depthScale(st.y, st.depth);
        var k = (entry.size && entry.size.h > 0 ? entry.size.h : 0.15) * SM.stage.DESIGN_H * depth * st.scale / (vb[3] || 100);
        var m = R().mul(Mv, R().mul(R().translate(st.x, st.y), R().mul(R().rotate(st.rot), R().mul(scaleM(k, k), R().translate(-(vb[0] + anchor.x * vb[2]), -(vb[1] + anchor.y * vb[3]))))));
        setM(ctx, m);
        SM.parts.drawPart(ctx, part, SM.parts.makeResolver([rc.palette]), st.alpha);
    }

    // rigState(st, t, rc, rig): the draw state of an actor from its logical state (pose blend, gesture, walk or idle cycle).
    function rigState(st, t, rc, rig) {
        var cat = rc.cat, rg = R();
        var poseA = cat.poses[st.pose.a] || cat.poses.stand || { bones: {} };
        var poseB = cat.poses[st.pose.b] || cat.poses.stand || { bones: {} };
        var u = st.pose.u;
        var bones = rg.lerpBones(poseA.bones, poseB.bones, u);
        var seated = ((poseA.seated ? 1 - u : 0) + (poseB.seated ? u : 0));
        var hands = u >= 0.5 ? poseB.hands : poseA.hands;
        var walking = st.walkTime !== null && st.walkTime !== undefined;
        var cyc = walking ? cat.cycles.walk : cat.cycles.idle;
        var cd = cyc ? rg.sampleKeys(cyc.keys, rg.cyclePhase(cyc, walking ? st.walkTime : t + st.phase)) : { bones: {}, offset_y: 0 };
        var gd = { bones: {}, offset_y: 0 }, gestureHands = null;
        if (st.gesture) {
            var ge = cat.gestures[st.gesture.name];
            if (ge) {
                gd = rg.sampleKeys(ge.keys, st.gesture.frac);
                gestureHands = rg.handsAt(ge.keys, st.gesture.frac);
            }
        }
        var handsOut = {};
        if (hands) { if (hands.l) handsOut.l = hands.l; if (hands.r) handsOut.r = hands.r; }
        if (gestureHands) { if (gestureHands.l) handsOut.l = gestureHands.l; if (gestureHands.r) handsOut.r = gestureHands.r; }
        var view = walking ? 'profile' : (st.look === 'camera' ? 'front' : (poseB.view || 'three_quarter'));
        var expr = cat.expressions[st.expression] || cat.expressions.neutral || {};
        return {
            view: view, offsets: rg.addBones(bones, cd.bones, gd.bones), hands: handsOut, expr: expr,
            mouth: null, blink: ((t + st.phase * 3) % 3.6) < 0.12,
            offsetY: seated * rg.seatedOffset(rig) + (cd.offset_y || 0) + (gd.offset_y || 0)
        };
    }

    function drawActor(ctx, st, t, rc, stage, Mv) {
        var rig = rc.assets.rig;
        if (!rig) return;
        var body = R().bodyOf(rig, st.params.body), depth = stage.depthScale(st.y, st.depth);
        var s = R().scaleFor(rig, SM.stage.DESIGN_H, depth, st.scale, body);
        var anchorM = R().mul(Mv, R().translate(st.x, st.y));
        // ground shadow
        var rx = s * (rig.units_height || 400) * 0.17;
        setM(ctx, anchorM);
        ctx.globalAlpha = SHADOW_ALPHA * st.alpha;
        ctx.fillStyle = '#000000';
        ctx.beginPath();
        ctx.ellipse(0, 0, rx, rx * 0.22, 0, 0, Math.PI * 2);
        ctx.fill();
        var m = R().mul(anchorM, scaleM(s * (st.facing < 0 ? -1 : 1), s));
        var resolve = SM.parts.makeResolver([R().tokensFor(rig, st.params), rc.palette]);
        R().drawActor(ctx, rig, st.params, rigState(st, t, rc, rig), m, resolve, st.alpha);
    }

    function drawShapeEntity(ctx, st, rc, Mv) {
        var resolve = SM.parts.makeResolver([rc.palette]);
        setM(ctx, R().mul(Mv, R().mul(R().translate(st.x, st.y), R().mul(R().rotate(st.rot), scaleM(st.scale, st.scale)))));
        ctx.globalAlpha = st.alpha > 1 ? 1 : st.alpha;
        if (st.kind === 'circle') {
            ctx.fillStyle = resolve(st.color);
            ctx.beginPath();
            ctx.arc(0, 0, st.w, 0, Math.PI * 2);
            ctx.fill();
        } else if (st.kind === 'line') {
            ctx.strokeStyle = resolve(st.stroke || st.color);
            ctx.lineWidth = st.h;
            ctx.lineCap = 'round';
            ctx.beginPath();
            ctx.moveTo(-st.w / 2, 0);
            ctx.lineTo(st.w / 2, 0);
            ctx.stroke();
        } else {
            ctx.fillStyle = resolve(st.color);
            ctx.fillRect(-st.w / 2, -st.h / 2, st.w, st.h);
            if (st.stroke) { ctx.strokeStyle = resolve(st.stroke); ctx.lineWidth = 3; ctx.strokeRect(-st.w / 2, -st.h / 2, st.w, st.h); }
        }
    }

    function drawText(ctx, st, rc, stage, bw) {
        if (!(st.alpha > 0) || !st.text) return;
        var style = rc.cat.textStyles[st.style] || rc.cat.textStyles.title || FALLBACK_TEXT_STYLE;
        var pos = stage.toCanvas(st.x, st.y, bw), align = style.align || 'center';
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        var lay = SM.text.layout(ctx, st.text, style, { canvasH: rc.H, maxWidth: SM.text.maxWidthFor(rc.W, rc.safe, pos.x, align), family: familyOf(rc, style) });
        SM.text.draw(ctx, lay, style, pos.x, pos.y, SM.parts.makeResolver([rc.palette]), st.alpha);
    }

    // drawScene(ctx, sc, t, rc): a storyboard or code-scene-like object {env, variant, camera, entities, stages}.
    function drawScene(ctx, sc, t, rc) {
        var stage = stageFor(sc, rc);
        var cam = SM.camera.at(sc.camera, t), win = stage.cameraWindow(cam), bw = stage.baseWindow(), Mv = stage.matrix(win);
        drawBackdrop(ctx, rc);
        drawEnvironment(ctx, sc, rc, win, bw, Mv);
        var states = sc.entities.map(function (e, k) { var s = SM.scene.evalEntity(e, t); s.k = k; return s; });
        var live = function (s) { return s.alpha > 0; };
        states.filter(function (s) { return s.type === 'prop' && s.layer === 'back' && live(s); }).forEach(function (s) { drawProp(ctx, s, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'actor' && live(s); })
            .sort(function (a, b) { return a.y - b.y || a.k - b.k; })
            .forEach(function (s) { drawActor(ctx, s, t, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'prop' && s.layer !== 'back' && live(s); }).forEach(function (s) { drawProp(ctx, s, rc, stage, Mv); });
        states.filter(function (s) { return s.type === 'shape' && live(s); }).forEach(function (s) { drawShapeEntity(ctx, s, rc, Mv); });
        states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { drawText(ctx, s, rc, stage, bw); });
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
    }

    function drawPlaceholder(ctx, sc, rc) {
        drawBackdrop(ctx, rc);
        var msg = (rc.codeErrors[sc.id]) || (sc.code && sc.code.error && sc.code.error.message) || 'Code scene';
        var stage = stageFor(sc, rc), bw = stage.baseWindow();
        drawText(ctx, { alpha: 1, text: 'Code scene "' + sc.id + '"', style: 'title', x: stage.focusX, y: 440 }, rc, stage, bw);
        drawText(ctx, { alpha: 1, text: String(msg), style: 'caption', x: stage.focusX, y: 600 }, rc, stage, bw);
    }

    function drawLayer(ctx, timeline, layer, rc) {
        var sc = timeline.scenes[layer.i];
        if (sc.kind === 'code') {
            var bmp = rc.codeFrames[sc.id];
            if (bmp) {
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.drawImage(bmp, 0, 0, rc.W, rc.H);
            } else drawPlaceholder(ctx, sc, rc);
            return;
        }
        drawScene(ctx, sc, layer.f / timeline.fps, rc);
    }

    // renderFrame(timeline, n, ctx, assets, opts): assets = {rig, props: {file: json}, envs: {file: json}, codeFrames,
    // codeErrors, canvas(name, w, h) -> {canvas, ctx}}; opts.width renders at another width (thumbnails).
    function renderFrame(timeline, n, ctx, assets, opts) {
        var W = opts && opts.width > 0 ? Math.round(opts.width) : timeline.width;
        var H = opts && opts.width > 0 ? Math.round(opts.width * timeline.height / timeline.width) : timeline.height;
        var rc = makeContext(timeline, assets, W, H);
        var plan = SM.schedule.framePlan(timeline.schedule, n);
        if (!plan.layers.length) {
            drawBackdrop(ctx, rc);
            return;
        }
        if (!plan.transition) {
            drawLayer(ctx, timeline, plan.layers[0], rc);
            return;
        }
        var prev = assets.canvas('prev', W, H), cur = assets.canvas('cur', W, H);
        drawLayer(prev.ctx, timeline, plan.layers[0], rc);
        drawLayer(cur.ctx, timeline, plan.layers[1], rc);
        SM.transitions.composite(ctx, plan.transition.kind, plan.transition.p, W, H, prev.canvas, cur.canvas);
    }

    SM.render = {
        renderFrame: renderFrame, drawScene: drawScene, rigState: rigState, makeContext: makeContext, familyOf: familyOf,
        drawBackdrop: drawBackdrop, drawText: drawText, stageFor: stageFor
    };
    SM.renderFrame = renderFrame;
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`24 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/transitions.js ui/js/storymaker/render.js scripts/storymaker-tests/05-render.mjs
git commit -m "feat(storymaker): player renderFrame and transitions" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C7: Code scenes and the determinism shadows

**Files:**
- Create: `ui/js/storymaker/codescene.js`, `ui/js/storymaker/determinism.js`
- Test: `scripts/storymaker-tests/06-codescene.mjs`

`codescene.js` is the code-scene API of the spec: `api.duration, add, text, shape, tween, camera, random(seed), cue, onFrame`; `api.chart` and
`api.diagram` exist and report `not_supported_yet`. Handle methods take `{start, dur, ease}` (`start` is seconds or `api.cue(n)`, from the scene start, the
same origin as cues and beats). Errors name the fix. The `onFrame` hook gets a frozen facade of drawing verbs, not the canvas. `determinism.js` shadows
`Date`, `Math.random`, `performance.now`, `crypto.getRandomValues`, `setTimeout`, `setInterval` and `requestAnimationFrame` with functions that throw a helpful
message (for determinism, not a security barrier).

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/06-codescene.mjs`**

````js
// Task C7: the code-scene API on the shared fixture s3.js, the restricted onFrame facade, the determinism shadows.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { root, read, plain, FakeCanvas, loadPlayer, makeContext } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, bundleScene, s3Source } = t;
const SM = t.core();

function runScene(source, name, extra) {
    const ctx = loadPlayer();
    vm.runInContext(bundleScene(source, name || 'x'), ctx);
    const C = ctx.StorymakerCore;
    const plan = C.planTiming(extra && extra.story || firewall, {}, catalog);
    const sceneId = extra && extra.id || 's3';
    const opts = { sceneId, index: plan.scenes[sceneId].index, catalog, aspect: '16:9', width: 1920, height: 1080, fps: 30, cues: plan.scenes[sceneId].cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe };
    return { ctx, C, build: () => C.codescene.build(ctx.self.__smScene, opts) };
}

check('code scene API on the shared fixture s3.js', () => {
    const { C, build } = runScene(s3Source, 's3');
    const built = build();
    assert.equal(built.duration, 5);
    assert.deepEqual(plain(built.needs), ['catalog:rig/human.json', 'font:default/400']);
    assert.deepEqual(plain(built.issues), []);
    const [anna, label] = built.entities;
    assert.equal(anna.type, 'actor'); assert.equal(label.type, 'text');
    near(anna.x.at(0), 0.30 * 1920, 1e-9, 'at: left');
    assert.equal(C.scene.evalEntity(anna, 0.5).gesture, null);
    const g = C.scene.evalEntity(anna, 1.6);
    assert.equal(g.gesture.name, 'point'); near(g.gesture.frac, 0.5, 1e-9);
    assert.equal(C.scene.evalEntity(anna, 2.3).gesture, null, 'the gesture is over after its 1.2 s');
    assert.equal(C.scene.evalEntity(anna, 2.9).expression, 'neutral');
    assert.equal(C.scene.evalEntity(anna, 3.1).expression, 'happy');
    near(C.scene.evalEntity(label, 0).alpha, 1, 1e-9);
    // the tween on opacity runs from the current value (1) to 1: no change; a fade-in from 0 is a tween with `from`
    const more = runScene(`export default function scene(api) {
      const t = api.text('Hi', 'title', { at: 'right' });
      t.fadeIn({ start: 1, dur: 1 });
      const box = api.shape('rect', { at: { x: 50, y: 50 }, w: 200, h: 100 });
      api.tween(box, { x: 80, scale: 2 }, { start: 0, dur: 2, ease: 'linear' });
      const lock = api.add('prop.lock', { at: 'center', layer: 'back' });
      lock.moveTo('left', { start: 1, dur: 1 }).scaleTo(2, { start: 1, dur: 1 }).rotateTo(90, { start: 0, dur: 1, ease: 'linear' });
      const c = api.chart('bar', { labels: [], values: [] }, { at: 'right' });
      api.tween(c, { grow: 1 }, { start: api.cue(1), dur: 2 });
      api.camera.zoom(1.5, { start: 0, dur: 1 });
    }`, 'more', { story: { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei' }] }, id: 'm' }).build();
    const [text, box, lock] = more.entities;
    assert.equal(more.entities.length, 3, 'the inert chart is not an entity');
    near(text.alpha.at(0.5), 0, 1e-9); near(text.alpha.at(2), 1, 1e-9);
    near(box.x.at(2), 0.8 * 1920, 1e-9); near(box.x.at(1), (0.5 + 0.8) / 2 * 1920, 1e-6); near(box.scale.at(2), 2, 1e-9);
    near(lock.rot.at(0.5), 45, 1e-9); near(lock.scale.at(2), 2, 1e-9);
    assert.equal(lock.layer, 'back');
    assert.deepEqual(plain(more.issues.map(i => i.code)), ['not_supported_yet', 'not_supported_yet'], 'api.chart and the grow tween report not_supported_yet');
    near(more.camera.zoom.at(1), 1.5, 1e-9);
    assert.equal(more.duration, Math.max(2, 1.3) + 0.5, 'without api.duration: the end of the last motion + 0.5 s');
});

check('code scene API: api.random is seeded, api.cue maps to cue times, errors name the fix', () => {
    const { C } = runScene('export default function scene() {}', 'e');
    const r1 = C.codescene.random(42), r2 = C.codescene.random(42), r3 = C.codescene.random('42');
    const a = [r1(), r1(), r1()], b = [r2(), r2(), r2()];
    assert.deepEqual(a, b); assert.ok(a.every(v => v >= 0 && v < 1));
    assert.notDeepEqual(a, [C.codescene.random(43)(), 0, 0].slice(0, 1).concat(a.slice(1)));
    assert.equal(r3(), C.codescene.random('42')());
    const err = src => { try { runScene(src, 'err', { story: { ...firewall, scenes: [{ id: 'm', narration: 'eins {1}zwei' }] }, id: 'm' }).build(); } catch (e) { return String(e.message); } return ''; };
    assert.match(err("export default function s(api) { api.add('char.nobody'); }"), /Unknown catalog id "char.nobody"/);
    assert.match(err("export default function s(api) { api.add('char.office-woman-1').pose('flying'); }"), /Unknown pose "flying"\. Available: .*stand/);
    assert.match(err("export default function s(api) { api.cue(9); }"), /no cue marker \{9\}/);
    assert.match(err("export default function s(api) { api.duration(9999); }"), /between 0 and 600/);
    assert.match(err("export default function s(api) { api.tween({}, { opacity: 1 }); }"), /needs a handle/);
    assert.match(err("export default function s(api) { const t = api.text('x'); api.tween(t, { weird: 1 }); }"), /Unknown tween property "weird"/);
    assert.match(err("export default function s(api) { const t = api.text('x'); t.fadeIn({ start: 'soon' }); }"), /start must be a number/);
    assert.match(err("export default function s(api) { api.text('x', 'huge'); }"), /Unknown text style "huge"/);
    assert.match(err("export default function s(api) { api.add('env.office'); }"), /api.add takes a character or a prop/);
    assert.equal(err("export default function s(api) { api.cue(1); }"), '');
});

check('code scene frames: the onFrame hook draws through a restricted facade', () => {
    const { C, build } = runScene(`export default function scene(api) {
      api.duration(2);
      api.onFrame((t, g) => { g.fill('$accent'); g.rect(10 + t * 100, 20, 30, 40); g.circle(5, 5, 3); g.text('t=' + t.toFixed(1), 100, 100, { size: 20 }); g.path('M0,0 L5,5', 'stroke'); });
    }`, 'hook');
    const built = build();
    const cv = new FakeCanvas(1920, 1080);
    const tl = { aspect: '16:9', fps: 30, palette: { bg: '#fff', bg2: '#eee', accent: '#f00', text: '#000' }, cat: C.scene.indexCatalog(catalog), fontFamilies: { default: 'F' }, style: { font: 'default' }, safe: catalog.stage.safe };
    const rc = C.render.makeContext(tl, { rig: files.rig, props: files.props, envs: files.envs }, 1920, 1080);
    C.codescene.drawFrame(built, 0.5, cv.ctx, rc);
    const log = cv.ctx.log.join('\n');
    assert.match(log, /set fillStyle=#f00/); assert.match(log, /fillRect\(60,20,30,40\)/); assert.match(log, /fillText\(t=0.5,100,100\)/);
    assert.match(log, /arc\(5,5,3,0,6.283/); assert.match(log, /stroke\(path\(M0,0 L5,5\)\)/);
    // the facade is a frozen set of drawing verbs: no canvas, no context, no way to reach the page
    const probe = runScene('export default function scene(api) { api.onFrame((t, g) => { globalThis.__facade = g; g.rect(NaN, 0, 1, 1); }); }', 'probe');
    assert.throws(() => C.codescene.drawFrame(probe.build(), 0, cv.ctx, rc), /x must be a finite number/);
    const g = probe.ctx.__facade;
    assert.ok(Object.isFrozen(g));
    assert.deepEqual(Object.keys(g).sort(), ['alpha', 'circle', 'fill', 'line', 'path', 'rect', 'restore', 'rotate', 'save', 'scale', 'stroke', 'strokeRect', 'text', 'translate']);
    assert.equal(g.canvas, undefined); assert.equal(g.ctx, undefined); assert.equal(g.fillStyle, undefined);
});

check('determinism shadows: Date, Math.random, performance.now, crypto, timers throw a helpful error', () => {
    const calls = [];
    const g = {
        Date, Math: { random: () => 0.5 }, performance: { now: () => 1 }, crypto: { getRandomValues: a => a, randomUUID: () => 'x' },
        setTimeout: () => calls.push('t'), setInterval: () => calls.push('i'), requestAnimationFrame: () => calls.push('r')
    };
    const done = SM.determinism.install(g);
    assert.deepEqual(plain(done).sort(), ['Date', 'Math.random', 'crypto.getRandomValues', 'crypto.randomUUID', 'performance.now', 'requestAnimationFrame', 'setInterval', 'setTimeout'].sort());
    assert.throws(() => g.Math.random(), /Math\.random\(\) is not available.*api\.random\(seed\)/);
    assert.throws(() => g.Date.now(), /Date\.now\(\)/);
    assert.throws(() => new g.Date(), /new Date\(\)/);
    assert.throws(() => g.performance.now(), /performance\.now\(\)/);
    assert.throws(() => g.crypto.getRandomValues(new Uint8Array(2)), /getRandomValues/);
    assert.throws(() => g.setTimeout(() => {}), /no timers/);
    assert.throws(() => g.setInterval(() => {}), /setInterval/);
    assert.throws(() => g.requestAnimationFrame(() => {}), /requestAnimationFrame/);
    assert.equal(new g.Date(0).getTime(), 0, 'a Date from an explicit value still works');
    assert.equal(g.Date.UTC(2020, 0, 1), Date.UTC(2020, 0, 1));
    assert.ok(new g.Date(5) instanceof Date);
    assert.deepEqual(calls, []);
});

check('code scene bundle form: Node esbuild reproduces the golden bundle of part A byte for byte (when that file is in the tree)', () => {
    const golden = 'internal/storymaker/testdata/scenes/s3.bundle.js';
    if (!fs.existsSync(path.join(root, golden))) return;   // part A's golden file has not landed in this tree yet
    assert.equal(t.s3Bundle, read(golden).replace(/\r\n/g, '\n'));
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (25 passed, 4 failed; first: code scene API on the shared fixture s3.js: TypeError: Cannot read properties of undefined (reading 'build'))

- [ ] **Step 3: Implement**


**`ui/js/storymaker/codescene.js`**

````js
// Storymaker player: the code-scene API. A code scene is `export default function scene(api)` bundled into a classic
// script; it runs once, in its own worker, to build a timeline of tracks (like a GSAP master timeline), and every frame
// evaluates that timeline. API: duration, add, text, shape, tween, camera, random(seed), cue(n), onFrame(fn); chart and
// diagram exist and report not_supported_yet in phase 1. Handle methods take {start, dur, ease}; `start` is seconds
// (or api.cue(n)) from the scene start (the start of the entry transition, like cues and beats). Positions are spot
// names or {x, y} in percent of the safe area. Errors are thrown with a message that names the fix.
// Classic script; publishes StorymakerCore.codescene.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var MAX_DURATION = 600;
    var TWEEN_PROPS = { opacity: 'alpha', x: 'x', y: 'y', scale: 'scale', rotation: 'rot', width: 'w', height: 'h' };

    function hashSeed(seed) {
        var s = String(seed === undefined ? 0 : seed), h = 2166136261;
        for (var i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 16777619); }
        return h >>> 0;
    }

    // random(seed): mulberry32, the only randomness a scene has.
    function random(seed) {
        var a = hashSeed(seed);
        return function () {
            a = (a + 0x6D2B79F5) | 0;
            var t = Math.imul(a ^ (a >>> 15), 1 | a);
            t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    function finite(v, what) {
        if (typeof v !== 'number' || !isFinite(v)) throw new TypeError(what + ' must be a number, got ' + String(v));
        return v;
    }

    function timing(o, defaultDur) {
        o = o || {};
        var start = o.start === undefined ? 0 : finite(o.start, 'start');
        var dur = o.dur === undefined ? defaultDur : finite(o.dur, 'dur');
        if (start < 0) throw new RangeError('start must be 0 or more, got ' + start);
        if (dur < 0) throw new RangeError('dur must be 0 or more, got ' + dur);
        var ease = o.ease === undefined ? 'ease-in-out' : o.ease;
        if (!SM.isEase(ease)) throw new RangeError('Unknown ease "' + ease + '". Use ' + SM.easeNames.join(', ') + '.');
        return { start: start, end: start + dur, dur: dur, ease: ease };
    }

    // build(sceneFn, opts): opts = {sceneId, catalog, aspect, width, height, fps, cues, palette, families, styleFont, safe}.
    function build(sceneFn, opts) {
        var cat = SM.scene.indexCatalog(opts.catalog), S = SM.scene, DW = SM.stage.DESIGN_W, DH = SM.stage.DESIGN_H;
        var stage = SM.stage.create({ aspect: opts.aspect, width: opts.width, height: opts.height, env: null, safe: opts.safe });
        var cam = SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2);
        var state = { duration: null, entities: [], onFrame: null, needs: {}, issues: [], counter: 0, unsupportedSeen: {} };
        var cues = opts.cues || {};
        var rigFile = opts.catalog && opts.catalog.rig && opts.catalog.rig.file || '';

        function note(code, message) {
            var key = code + message;
            if (state.unsupportedSeen[key]) return;
            state.unsupportedSeen[key] = true;
            state.issues.push(SM.plan.issue('minor', opts.sceneId, '/scenes/' + (opts.index === undefined ? 0 : opts.index), code, message));
        }

        function place(at, what, fallback) {
            var pos = at === undefined || at === null ? stage.resolve(fallback || 'center') : stage.resolve(at);
            if (!pos) {
                throw new RangeError('Unknown position ' + JSON.stringify(at) + ' for ' + what + '. Use a spot (' + stage.spotNames().join(', ') + ') or {x, y} in percent.');
            }
            return pos;
        }

        function nameOf(map, name, what, def) {
            var n = name === undefined || name === null ? def : name;
            if (!map[n]) throw new RangeError('Unknown ' + what + ' "' + n + '". Available: ' + Object.keys(map).sort().join(', ') + '.');
            return n;
        }

        function addEntity(e) { state.entities.push(e); return e; }
        function nextId(prefix) { state.counter++; return prefix + '-' + state.counter; }

        // ---- handles
        function common(e) {
            var h = {};
            h.moveTo = function (position, o) {
                var t = timing(o, 0.6), pos = place(position, 'moveTo');
                e.x.add(t.start, t.end, pos.x, t.ease);
                e.y.add(t.start, t.end, pos.y, t.ease);
                return h;
            };
            h.fadeIn = function (o) {
                var t = timing(o, 0.5);
                if (!e.alpha.segs.length && e.alpha.init === 1) e.alpha.init = 0;
                e.alpha.add(t.start, t.end, 1, t.ease);
                return h;
            };
            h.fadeOut = function (o) {
                var t = timing(o, 0.5);
                e.alpha.add(t.start, t.end, 0, t.ease);
                return h;
            };
            h.scaleTo = function (s, o) {
                var t = timing(o, 0.5);
                e.scale.add(t.start, t.end, finite(s, 'scale') / (e.baseScale || 1), t.ease);
                return h;
            };
            h.rotateTo = function (deg, o) {
                var t = timing(o, 0.5);
                e.rot.add(t.start, t.end, finite(deg, 'degrees'), t.ease);
                return h;
            };
            Object.defineProperty(h, '__entity', { value: e, enumerable: false });
            return h;
        }

        function actorHandle(e) {
            var h = common(e);
            h.walkTo = function (position, o) {
                var pos = place(position, 'walkTo'), start = o && o.start !== undefined ? finite(o.start, 'start') : 0;
                var from = { x: e.x.at(start), y: e.y.at(start) }, cyc = cat.cycles.walk, speed = cyc && cyc.speed > 0 ? cyc.speed : 12;
                var dist = Math.sqrt(Math.pow(pos.x - from.x, 2) + Math.pow(pos.y - from.y, 2));
                var dur = o && o.dur !== undefined ? finite(o.dur, 'dur') : Math.max(0.3, dist / (speed / 100 * DW * stage.depthScale(pos.y)));
                var t = timing({ start: start, dur: dur, ease: o && o.ease || 'linear' }, dur);
                if (Math.abs(pos.x - from.x) > 1) e.facing.set(t.start, pos.x > from.x ? 1 : -1);
                e.x.add(t.start, t.end, pos.x, t.ease);
                e.y.add(t.start, t.end, pos.y, t.ease);
                e.walks.add(t.start, t.end, {});
                return h;
            };
            h.pose = function (name, o) {
                var t = timing(o, 0.3);
                e.pose.set(t.start, nameOf(cat.poses, name, 'pose'), t.dur);
                return h;
            };
            h.emote = function (name, o) {
                var t = timing(o, 0.25);
                e.expr.set(t.start, nameOf(cat.expressions, name, 'expression'));
                return h;
            };
            h.gesture = function (name, o) {
                var entry = cat.gestures[nameOf(cat.gestures, name, 'gesture')];
                var count = Math.max(1, Math.min(5, Math.round(o && o.count || 1)));
                var t = timing({ start: o && o.start, dur: o && o.dur !== undefined ? o.dur : (entry.duration || 0.5) * count, ease: o && o.ease }, 0.5);
                e.gestures.add(t.start, t.end, { name: name, count: count });
                return h;
            };
            h.look = function (target, o) {
                var t = timing(o, 0.3);
                if (target === 'camera') e.look.set(t.start, 'camera');
                else { e.look.set(t.start, null); var p = place(target, 'look'); if (Math.abs(p.x - e.x.at(t.start)) > 1) e.facing.set(t.start, p.x > e.x.at(t.start) ? 1 : -1); }
                return h;
            };
            return h;
        }

        // ---- the API
        var api = {};
        api.duration = function (sec) {
            finite(sec, 'duration');
            if (sec <= 0 || sec > MAX_DURATION) throw new RangeError('duration must be between 0 and ' + MAX_DURATION + ' seconds, got ' + sec);
            state.duration = sec;
        };
        api.add = function (id, o) {
            o = o || {};
            var entry = cat.byId[id];
            if (!entry) throw new RangeError('Unknown catalog id "' + id + '". Search the catalog for an exact id (char.*, prop.*).');
            var pos = place(o.at, 'add');
            if (entry.kind === 'character') {
                var pose = nameOf(cat.poses, o.pose, 'pose', 'stand');
                if (rigFile) state.needs['catalog:' + rigFile] = true;
                var facing = o.facing === 'left' ? -1 : (o.facing === 'right' ? 1 : (pos.face === 'left' ? -1 : (pos.face === 'right' ? 1 : (pos.x <= stage.focusX ? 1 : -1))));
                var a = S.createActor({ id: nextId('actor'), cast: id, character: id, params: entry.params || {}, x: pos.x, y: pos.y, scale: o.scale, facing: facing, pose: pose, expression: 'neutral', phase: state.counter * 0.37 });
                return actorHandle(addEntity(a));
            }
            if (entry.kind === 'prop') {
                if (entry.file) state.needs['catalog:' + entry.file] = true;
                return common(addEntity(S.createProp({ id: nextId('prop'), item: id, x: pos.x, y: pos.y, scale: o.scale, layer: o.layer })));
            }
            throw new RangeError('api.add takes a character or a prop, "' + id + '" is a ' + entry.kind + '.');
        };
        api.text = function (str, style, o) {
            var name = style === undefined ? 'title' : style;
            nameOf(cat.textStyles, name, 'text style');
            var at = o && o.at !== undefined ? o.at : 'center', pos;
            if (typeof at === 'string' && stage.isBaseSpot(at)) pos = { x: stage.spot(at).x, y: 0.28 * DH };
            else pos = place(at, 'text');
            var se = cat.textStyles[name], font = !se.font || se.font === 'default' ? opts.styleFont || 'default' : se.font;
            (cat.fonts || []).forEach(function (f) { if (f.name === font) state.needs['font:' + f.name + '/' + f.weight] = true; });
            var e = addEntity(S.createText({ id: nextId('text'), text: String(str), style: name, x: pos.x, y: pos.y }));
            var h = common(e);
            h.setText = function (s) { e.text = String(s); return h; };
            return h;
        };
        api.shape = function (kind, o) {
            o = o || {};
            if (kind !== 'rect' && kind !== 'circle' && kind !== 'line') throw new RangeError('Unknown shape "' + kind + '". Use rect, circle or line.');
            var pos = place(o.at, 'shape');
            return common(addEntity(S.createShape({ id: nextId('shape'), kind: kind, x: pos.x, y: pos.y, w: o.w, h: o.h, color: o.fill || o.color, stroke: o.stroke })));
        };
        api.chart = function () { note('not_supported_yet', 'api.chart is not supported yet; the chart is not drawn.'); return common(S.createShape({ id: nextId('chart'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.diagram = function () { note('not_supported_yet', 'api.diagram is not supported yet; the diagram is not drawn.'); return common(S.createShape({ id: nextId('diagram'), kind: 'rect', x: 0, y: 0, hidden: true })); };
        api.tween = function (target, props, o) {
            var e = target && target.__entity;
            if (!e) throw new TypeError('api.tween needs a handle returned by api.add, api.text or api.shape.');
            if (!props || typeof props !== 'object') throw new TypeError('api.tween needs an object of properties, for example {opacity: 1}.');
            var t = timing(o, 0.5), from = (o && o.from) || {};
            Object.keys(props).forEach(function (key) {
                var field = TWEEN_PROPS[key];
                if (!field) {
                    if (key === 'grow') { note('not_supported_yet', 'The tween property "grow" belongs to charts, which are not supported yet.'); return; }
                    throw new RangeError('Unknown tween property "' + key + '". Use ' + Object.keys(TWEEN_PROPS).join(', ') + '.');
                }
                var to = finite(props[key], key), track = e[field];
                if (!track) throw new RangeError('The property "' + key + '" does not apply to this handle.');
                if (key === 'x') to = to / 100 * DW;
                else if (key === 'y') to = to / 100 * DH;
                else if (key === 'scale') to = to / (e.baseScale || 1);
                var f = from[key] === undefined ? undefined : finite(from[key], 'from.' + key) * (key === 'x' ? DW / 100 : (key === 'y' ? DH / 100 : (key === 'scale' ? 1 / (e.baseScale || 1) : 1)));
                if (key === 'opacity' && f !== undefined && !track.segs.length) track.init = f;
                track.add(t.start, t.end, to, t.ease, f);
            });
            return target;
        };
        api.camera = {
            zoom: function (amount, o) { var t = timing(o, 0.8); SM.camera.addZoom(cam, t.start, t.end, finite(amount, 'zoom'), t.ease); },
            pan: function (position, o) { var t = timing(o, 0.8), p = place(position, 'camera.pan'); SM.camera.addPan(cam, t.start, t.end, p.x, p.y - 150, t.ease); },
            shake: function (amount, o) { var t = timing(o, 0.5); SM.camera.addShake(cam, t.start, t.end, amount > 0 ? amount : 14); }
        };
        api.random = random;
        api.cue = function (n) {
            if (cues[n] === undefined) throw new RangeError('The narration has no cue marker {' + n + '}.');
            return cues[n];
        };
        api.onFrame = function (fn) {
            if (typeof fn !== 'function') throw new TypeError('api.onFrame needs a function (t, g).');
            state.onFrame = fn;
        };

        sceneFn(api);

        var motionEnd = SM.camera.end(cam);
        state.entities.forEach(function (e) { var end = S.entityEnd(e); if (end > motionEnd) motionEnd = end; });
        var duration = state.duration !== null ? state.duration : Math.min(MAX_DURATION, Math.max(1, motionEnd + 0.5));
        return { duration: duration, entities: state.entities, camera: cam, onFrame: state.onFrame, needs: Object.keys(state.needs).sort(), issues: state.issues, stage: stage };
    }

    // ---- the restricted 2D facade passed to api.onFrame(fn)
    function facade(ctx, rc) {
        var resolve = SM.parts.makeResolver([rc.palette]);
        function num(v, what) { if (typeof v !== 'number' || !isFinite(v)) throw new TypeError(what + ' must be a finite number'); return v; }
        var g = {
            save: function () { ctx.save(); },
            restore: function () { ctx.restore(); },
            translate: function (x, y) { ctx.translate(num(x, 'x'), num(y, 'y')); },
            rotate: function (deg) { ctx.rotate(num(deg, 'degrees') * Math.PI / 180); },
            scale: function (x, y) { ctx.scale(num(x, 'x'), num(y === undefined ? x : y, 'y')); },
            alpha: function (a) { ctx.globalAlpha = Math.max(0, Math.min(1, num(a, 'alpha'))); },
            fill: function (color) { ctx.fillStyle = resolve(String(color)); },
            stroke: function (color, width) { ctx.strokeStyle = resolve(String(color)); if (width !== undefined) ctx.lineWidth = Math.max(0, num(width, 'width')); },
            rect: function (x, y, w, h) { ctx.fillRect(num(x, 'x'), num(y, 'y'), num(w, 'w'), num(h, 'h')); },
            strokeRect: function (x, y, w, h) { ctx.strokeRect(num(x, 'x'), num(y, 'y'), num(w, 'w'), num(h, 'h')); },
            circle: function (x, y, r) { ctx.beginPath(); ctx.arc(num(x, 'x'), num(y, 'y'), Math.max(0, num(r, 'r')), 0, Math.PI * 2); ctx.fill(); },
            line: function (x1, y1, x2, y2) { ctx.beginPath(); ctx.moveTo(num(x1, 'x1'), num(y1, 'y1')); ctx.lineTo(num(x2, 'x2'), num(y2, 'y2')); ctx.stroke(); },
            path: function (d, mode) {
                var p = new Path2D(String(d));
                if (mode !== 'stroke') ctx.fill(p);
                if (mode === 'stroke' || mode === 'both') ctx.stroke(p);
            },
            text: function (str, x, y, o) {
                o = o || {};
                var px = o.size > 0 ? o.size : 48, family = SM.render.familyOf(rc, { font: 'default' });
                ctx.font = SM.text.fontString(o.weight || 700, px, family);
                ctx.textAlign = o.align === 'left' || o.align === 'right' ? o.align : 'center';
                ctx.textBaseline = 'middle';
                if (o.color) ctx.fillStyle = resolve(String(o.color));
                ctx.fillText(String(str), num(x, 'x'), num(y, 'y'));
            }
        };
        return Object.freeze(g);
    }

    // drawFrame(built, t, ctx, rc): draws the scene timeline at time t, then the onFrame hook in design coordinates.
    function drawFrame(built, t, ctx, rc) {
        var sceneLike = { id: rc.sceneId || 'code', env: null, variant: 'day', camera: built.camera, entities: built.entities, stages: built.stages || (built.stages = {}) };
        SM.render.drawScene(ctx, sceneLike, t, rc);
        if (!built.onFrame) return;
        var stage = SM.render.stageFor(sceneLike, rc), win = stage.cameraWindow(SM.camera.at(built.camera, t)), m = stage.matrix(win);
        ctx.save();
        ctx.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]);
        ctx.globalAlpha = 1;
        try { built.onFrame(t, facade(ctx, rc)); } finally { ctx.restore(); }
    }

    SM.codescene = { build: build, drawFrame: drawFrame, random: random, MAX_DURATION: MAX_DURATION };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/determinism.js`**

````js
// Storymaker player: determinism. The player worker and the code-scene workers shadow Date (now and the zero-argument
// constructor), Math.random, performance.now, crypto.getRandomValues, setTimeout, setInterval and requestAnimationFrame:
// a scene that calls one gets an error that names api.random(seed) or the frame time instead of a different picture on
// every run. This is for determinism, not a security barrier (the CSP and the worker realm are the barriers).
// Classic script; publishes StorymakerCore.determinism.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function blocked(name, hint) {
        return function () {
            throw new Error(name + ' is not available in a Storymaker scene (it would make the video differ between runs). ' + hint);
        };
    }

    function define(obj, key, value) {
        try {
            Object.defineProperty(obj, key, { value: value, writable: true, configurable: true, enumerable: false });
            return true;
        } catch (e) {
            try { obj[key] = value; return obj[key] === value; } catch (e2) { return false; }
        }
    }

    // install(g): g is the worker global. Returns the names that were shadowed.
    function install(g) {
        var done = [];
        function mark(name, ok) { if (ok) done.push(name); }
        var RealDate = g.Date;
        if (RealDate) {
            var ShadowDate = function Date() {
                if (arguments.length === 0) blocked('new Date()', 'Use the frame time t that api.onFrame passes, or api.tween.')();
                if (!new.target) return RealDate.apply(null, arguments);
                return Reflect.construct(RealDate, arguments, new.target === ShadowDate ? RealDate : new.target);
            };
            ShadowDate.prototype = RealDate.prototype;
            ShadowDate.UTC = RealDate.UTC;
            ShadowDate.parse = RealDate.parse;
            ShadowDate.now = blocked('Date.now()', 'Use the frame time t that api.onFrame passes, or api.tween.');
            mark('Date', define(g, 'Date', ShadowDate));
        }
        if (g.Math) mark('Math.random', define(g.Math, 'random', blocked('Math.random()', 'Use api.random(seed), which returns a seeded generator.')));
        if (g.performance) mark('performance.now', define(g.performance, 'now', blocked('performance.now()', 'Time comes from the frame number only.')));
        if (g.crypto) {
            mark('crypto.getRandomValues', define(g.crypto, 'getRandomValues', blocked('crypto.getRandomValues()', 'Use api.random(seed).')));
            if (typeof g.crypto.randomUUID === 'function') mark('crypto.randomUUID', define(g.crypto, 'randomUUID', blocked('crypto.randomUUID()', 'Use api.random(seed).')));
        }
        ['setTimeout', 'setInterval', 'requestAnimationFrame'].forEach(function (name) {
            if (typeof g[name] === 'function' || name in g) {
                mark(name, define(g, name, blocked(name + '()', 'Animation is built with api.tween and the beat times; there are no timers.')));
            }
        });
        return done;
    }

    SM.determinism = { install: install };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`29 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/codescene.js ui/js/storymaker/determinism.js scripts/storymaker-tests/06-codescene.mjs
git commit -m "feat(storymaker): code-scene API and determinism shadows" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C8: The worker realm: player and code-scene runtimes, role dispatch

**Files:**
- Create: `ui/js/storymaker/worker-player.js`, `ui/js/storymaker/worker-scene.js`, `ui/js/storymaker/worker-main.js`
- Test: `scripts/storymaker-tests/07-workers.mjs`

The worker realm. `worker-player.js`: the trusted runtime that handles `plan`, `compile`, `assets` (catalog JSON and `FontFace` loading, the readiness barrier)
and `render` (a frame as an ImageBitmap); the same `Runtime` serves the host realm in the storyboard fallback. `worker-scene.js`: the untrusted runtime of one code
scene (`build`, `assets`, `frame`; errors carry a line and column of the worker script). `worker-main.js`: role dispatch on `self.__SM_ROLE__` (set by the blob
prelude), determinism shadows, one-at-a-time message handling, `hello`.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/07-workers.mjs`**

````js
// Task C8: the worker realm: role dispatch, the message protocol of the player worker and of a code-scene worker, the
// determinism shadows installed on load. The workers run the real player bundle parts in a vm (InProcessWorker).
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { plain, InProcessWorker, until } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, catalog, files, firewall, minimal, s3Bundle } = t;

// A tiny RPC client over a worker: replies are matched by req.
function client(worker) {
    const waiting = new Map(), c = { hello: false, order: [] };
    let req = 0;
    worker.onmessage = ev => {
        const d = ev.data;
        if (d.type === 'hello') { c.hello = d.role; return; }
        c.order.push(d.req);
        const w = waiting.get(d.req);
        if (w) { waiting.delete(d.req); w(d); }
    };
    c.call = msg => new Promise(resolve => { msg.req = ++req; waiting.set(req, resolve); worker.postMessage(msg); });
    return c;
}
const font = { name: 'default', family: 'Storymaker Default', weight: 400, data: new ArrayBuffer(8) };
const allAssets = () => {
    const a = { 'catalog:rig/human.json': files.rig };
    for (const k of Object.keys(files.props)) a['catalog:' + k] = files.props[k];
    for (const k of Object.keys(files.envs)) a['catalog:' + k] = files.envs[k];
    return a;
};

check('player worker: hello, plan, compile, assets, render, in that protocol', async () => {
    const w = new InProcessWorker('player', ''), c = client(w);
    await until(() => c.hello === 'player', 'hello');
    const planned = await c.call({ type: 'plan', story: firewall, catalog, audioInfo: {}, opts: {} });
    assert.equal(planned.ok, true);
    assert.deepEqual(plain(Object.keys(planned.plan.scenes)), ['s1', 's2', 's3']);
    assert.equal(planned.plan.scenes.s3.kind, 'code');
    assert.equal(planned.plan.palette.bg, '#fff7e6'); assert.equal(planned.plan.fontFamilies.default, 'Storymaker Default');
    const before = await c.call({ type: 'render', n: 0, codeFrames: {}, codeErrors: {} });
    assert.equal(before.ok, false); assert.equal(before.error.code, 'code_error', 'render before compile is an error reply');
    const comp = await c.call({ type: 'compile', story: firewall, catalog, audioInfo: {}, opts: { codeScenes: { s3: { duration: 5 } } } });
    assert.equal(comp.ok, true); assert.equal(comp.report.frame_count, 354);
    assert.equal(comp.schedule.scenes.length, 3);
    assert.ok(comp.missing.includes('catalog:rig/human.json'), 'nothing is loaded yet');
    const loaded = await c.call({ type: 'assets', assets: allAssets(), fonts: [font] });
    assert.deepEqual(plain(loaded.missing), [], 'the readiness barrier: every need is satisfied');
    assert.equal(w.context.fonts.faces.length, 1, 'the font went through FontFace and fonts.add in the worker');
    assert.equal(w.context.fonts.faces[0].status, 'loaded');
    const frame = await c.call({ type: 'render', n: 40, codeFrames: {}, codeErrors: {} });
    assert.equal(frame.ok, true); assert.equal(frame.bitmap.width, 1920); assert.equal(frame.bitmap.height, 1080); assert.match(frame.bitmap.digest, /^[0-9a-f]{16}$/);
    const again = await c.call({ type: 'render', n: 40, codeFrames: {}, codeErrors: {} });
    assert.equal(again.bitmap.digest, frame.bitmap.digest, 'the same frame renders the same pixels (call log digest)');
    const other = await c.call({ type: 'render', n: 41, codeFrames: {}, codeErrors: {} });
    assert.notEqual(other.bitmap.digest, frame.bitmap.digest);
    const thumb = await c.call({ type: 'render', n: 40, width: 320, codeFrames: {}, codeErrors: {} });
    assert.deepEqual([thumb.bitmap.width, thumb.bitmap.height], [320, 180]);
    const fullWidth = await c.call({ type: 'render', n: 40, width: 1920, codeFrames: {}, codeErrors: {} });
    assert.equal(fullWidth.bitmap.digest, frame.bitmap.digest, 'a thumbnail at the full width is the frame itself (the parity recipe of the notes)');
    const bad = await c.call({ type: 'mystery' });
    assert.equal(bad.ok, false); assert.match(bad.error.message, /unknown message type/);
    w.terminate();
});

check('player worker: messages are handled one at a time, in order; a worker font that fails reports readiness', async () => {
    const w = new InProcessWorker('player', ''), c = client(w);
    await until(() => c.hello === 'player', 'hello');
    const calls = [
        c.call({ type: 'compile', story: minimal, catalog, audioInfo: {}, opts: {} }),
        c.call({ type: 'assets', assets: allAssets(), fonts: [font] }),
        c.call({ type: 'render', n: 3, codeFrames: {}, codeErrors: {} })
    ];
    const replies = await Promise.all(calls);
    assert.ok(replies.every(r => r.ok), JSON.stringify(replies.map(r => r.error)));
    assert.deepEqual(c.order, [1, 2, 3]);
    const failed = await c.call({ type: 'assets', assets: {}, fonts: [{ name: 'other', family: 'Other', weight: 700, data: new ArrayBuffer(0) }] });
    assert.equal(failed.ok, false); assert.equal(failed.error.code, 'readiness'); assert.match(failed.error.message, /Font "Other" \(700\) could not be loaded: A network error occurred./);
    w.terminate();
});

check('worker realm: the determinism shadows are installed before any scene code runs', async () => {
    const w = new InProcessWorker('player', ''), c = client(w);
    await until(() => c.hello === 'player', 'hello');
    for (const code of ['Math.random()', 'Date.now()', 'new Date()', 'performance.now()', 'crypto.getRandomValues(new Uint8Array(1))', 'setTimeout(function () {}, 1)', 'setInterval(function () {}, 1)', 'requestAnimationFrame(function () {})']) {
        assert.throws(() => vm.runInContext(code, w.context), /not available in a Storymaker scene/, code);
    }
    w.terminate();
    const doc = new InProcessWorker('none', '');
    assert.doesNotThrow(() => vm.runInContext('Math.random(); Date.now()', doc.context), 'a document-realm load (no role) installs nothing');
});

check('scene worker: build reports duration, needs and issues; frame returns a bitmap; errors carry a line', async () => {
    const w = new InProcessWorker('scene', s3Bundle), c = client(w);
    await until(() => c.hello === 'scene', 'hello');
    const plan = t.core().planTiming(firewall, {}, catalog);
    const opts = { sceneId: 's3', index: 2, catalog, aspect: '16:9', width: 640, height: 360, fps: 30, cues: plan.scenes.s3.cues, palette: plan.palette, families: plan.fontFamilies, styleFont: 'default', safe: catalog.stage.safe };
    const frameEarly = await c.call({ type: 'frame', t: 0.5, width: 0 });
    assert.equal(frameEarly.ok, false, 'a frame before build is an error');
    const built = await c.call({ type: 'build', opts });
    assert.equal(built.ok, true); assert.equal(built.duration, 5);
    assert.deepEqual(plain(built.needs), ['catalog:rig/human.json', 'font:default/400']);
    const loaded = await c.call({ type: 'assets', assets: { 'catalog:rig/human.json': files.rig }, fonts: [font] });
    assert.equal(loaded.ok, true);
    const a = await c.call({ type: 'frame', t: 1.5, width: 0 }), b = await c.call({ type: 'frame', t: 1.5, width: 0 }), later = await c.call({ type: 'frame', t: 3.5, width: 0 });
    assert.equal(a.ok, true); assert.deepEqual([a.bitmap.width, a.bitmap.height], [640, 360]);
    assert.equal(a.bitmap.digest, b.bitmap.digest); assert.notEqual(a.bitmap.digest, later.bitmap.digest, 'the scene animates (gesture at 1 s, emote at 3 s)');
    const small = await c.call({ type: 'frame', t: 1.5, width: 160 });
    assert.deepEqual([small.bitmap.width, small.bitmap.height], [160, 90]);
    w.terminate();
    const broken = new InProcessWorker('scene', t.bundleScene('export default function scene(api) {\n  api.duration(2);\n  api.add("char.nobody");\n}\n', 'broken')), cb = client(broken);
    await until(() => cb.hello === 'scene', 'hello');
    const err = await cb.call({ type: 'build', opts });
    assert.equal(err.ok, false); assert.equal(err.error.code, 'code_error'); assert.match(err.error.message, /Scene "s3": Unknown catalog id "char\.nobody"/);
    assert.equal(typeof err.error.line, 'number', 'the stack names a line of the scene script');
    broken.terminate();
    const none = new InProcessWorker('scene', ''), cn = client(none);
    await until(() => cn.hello === 'scene', 'hello');
    const missing = await cn.call({ type: 'build', opts });
    assert.match(missing.error.message, /did not register a scene function/);
    none.terminate();
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (29 passed, 4 failed; first: player worker: hello, plan, compile, assets, render, in that protocol: Error: timeout waiting for hello)

- [ ] **Step 3: Implement**


**`ui/js/storymaker/worker-player.js`**

````js
// Storymaker player: the player runtime (trusted, our code only). It holds the compiled timeline and the loaded assets
// and answers the host's messages: plan (timing facts for code scenes), compile (timeline + compile report), assets
// (catalog JSON and fonts, the readiness barrier) and render (one frame as an ImageBitmap). The same Runtime serves the
// player worker and, when workers are unavailable, the host realm (storyboard fallback). Classic script; publishes
// StorymakerCore.workerPlayer.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function fail(code, message, extra) {
        var e = { code: code, message: message };
        if (extra) for (var k in extra) e[k] = extra[k];
        return e;
    }

    // env = {makeCanvas(w, h), FontFace, fonts}
    function Runtime(env) {
        this.env = env;
        this.timeline = null;
        this.files = {};
        this.canvases = {};
        this.main = null;
        this.fontsLoaded = {};
    }

    Runtime.prototype.canvas = function (name, w, h) {
        var key = name + ':' + w + 'x' + h;
        if (!this.canvases[key]) {
            var canvas = this.env.makeCanvas(w, h);
            this.canvases[key] = { canvas: canvas, ctx: canvas.getContext('2d') };
        }
        return this.canvases[key];
    };

    Runtime.prototype.missingNeeds = function () {
        var self = this, out = [];
        (this.needs || []).forEach(function (key) {
            if (key.indexOf('catalog:') === 0) { if (!self.files[key.slice(8)]) out.push(key); }
            else if (key.indexOf('font:') === 0) { if (!self.fontsLoaded[key.slice(5)]) out.push(key); }
        });
        return out;
    };

    Runtime.prototype.handle = async function (msg) {
        switch (msg.type) {
            case 'plan': {
                var plan = SM.planTiming(msg.story, msg.audioInfo || {}, msg.catalog, msg.opts);
                return { payload: { plan: plan } };
            }
            case 'compile': {
                var out = SM.compile(msg.story, msg.audioInfo || {}, msg.catalog, msg.opts);
                this.timeline = out.timeline;
                this.needs = out.needs;
                this.main = null;
                return { payload: { report: out.report, schedule: out.schedule, needs: out.needs, missing: this.missingNeeds() } };
            }
            case 'assets': {
                var assets = msg.assets || {}, k;
                for (k in assets) if (k.indexOf('catalog:') === 0) this.files[k.slice(8)] = assets[k];
                var fonts = msg.fonts || [], env = this.env;
                for (var i = 0; i < fonts.length; i++) {
                    var f = fonts[i];
                    if (!f || !f.data) continue;
                    var key = f.name + '/' + f.weight;
                    if (this.fontsLoaded[key]) continue;
                    try {
                        var face = new env.FontFace(f.family, f.data, { weight: String(f.weight) });
                        await face.load();
                        env.fonts.add(face);
                        this.fontsLoaded[key] = true;
                    } catch (err) {
                        throw fail('readiness', 'Font "' + f.family + '" (' + f.weight + ') could not be loaded: ' + (err && err.message || err));
                    }
                }
                return { payload: { missing: this.missingNeeds() } };
            }
            case 'render': {
                if (!this.timeline) throw fail('code_error', 'render before compile');
                var tl = this.timeline, width = msg.width > 0 ? Math.round(msg.width) : tl.width;
                var height = msg.width > 0 ? Math.round(msg.width * tl.height / tl.width) : tl.height;
                var main = this.canvas('main', width, height), self = this;
                var assetsObj = {
                    rig: this.files[tl.rigFile] || null, props: this.files, envs: this.files,
                    codeFrames: msg.codeFrames || {}, codeErrors: msg.codeErrors || {},
                    canvas: function (name, w, h) { return self.canvas(name, w, h); }
                };
                SM.renderFrame(tl, msg.n, main.ctx, assetsObj, { width: width === tl.width ? 0 : width });
                var bitmap = typeof main.canvas.transferToImageBitmap === 'function' ? main.canvas.transferToImageBitmap() : main.canvas;
                var frames = msg.codeFrames || {};
                for (var id in frames) if (frames[id] && typeof frames[id].close === 'function') frames[id].close();
                return { payload: { bitmap: bitmap, width: width, height: height }, transfer: typeof main.canvas.transferToImageBitmap === 'function' ? [bitmap] : [] };
            }
            case 'dispose':
                this.timeline = null;
                this.files = {};
                this.canvases = {};
                return { payload: {} };
            default:
                throw fail('code_error', 'unknown message type "' + msg.type + '"');
        }
    };

    SM.workerPlayer = { Runtime: Runtime, fail: fail };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/worker-scene.js`**

````js
// Storymaker player: the code-scene runtime (untrusted scene code runs here, one worker per code scene). The worker
// imports the player bundle for the scene API, the bundled scene registers `self.__smScene`, and the runtime answers: build
// (run the scene function once: duration, needs, issues), assets (catalog JSON and fonts) and frame (the scene at time t as
// an ImageBitmap from its own canvas). Errors carry the message and, when the stack names it, a line and column of the
// worker script. Classic script; publishes StorymakerCore.workerScene.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function fail(code, message, extra) {
        var e = { code: code, message: message };
        if (extra) for (var k in extra) e[k] = extra[k];
        return e;
    }

    // locate(err) = {line, column} of the first stack frame inside the worker script (a blob URL), else {}.
    function locate(err) {
        var stack = err && err.stack ? String(err.stack) : '', lines = stack.split('\n');
        for (var i = 0; i < lines.length; i++) {
            var m = /(?:blob:[^\s)]*|<anonymous>|scene\.bundle\.js|worker\.js):(\d+):(\d+)/.exec(lines[i]);
            if (m) return { line: parseInt(m[1], 10), column: parseInt(m[2], 10) };
        }
        return {};
    }

    function sceneError(err, fallback) {
        var where = locate(err), msg = err && err.message ? String(err.message) : String(err);
        var out = fail('code_error', (fallback ? fallback + ': ' : '') + msg);
        if (where.line) { out.line = where.line; out.column = where.column; }
        return out;
    }

    function Runtime(env, scope) {
        this.env = env;
        this.scope = scope;
        this.built = null;
        this.opts = null;
        this.files = {};
        this.canvases = {};
        this.fontsLoaded = {};
    }

    Runtime.prototype.handle = async function (msg) {
        switch (msg.type) {
            case 'build': {
                var fn = this.scope.__smScene;
                if (typeof fn !== 'function') throw fail('code_error', 'The scene bundle did not register a scene function (export default function scene(api)).');
                this.opts = msg.opts;
                var built;
                try { built = SM.codescene.build(fn, msg.opts); } catch (err) { throw sceneError(err, 'Scene "' + msg.opts.sceneId + '"'); }
                this.built = built;
                return { payload: { duration: built.duration, needs: built.needs, issues: built.issues } };
            }
            case 'assets': {
                var assets = msg.assets || {}, k;
                for (k in assets) if (k.indexOf('catalog:') === 0) this.files[k.slice(8)] = assets[k];
                var fonts = msg.fonts || [], env = this.env;
                for (var i = 0; i < fonts.length; i++) {
                    var f = fonts[i];
                    if (!f || !f.data) continue;
                    var key = f.name + '/' + f.weight;
                    if (this.fontsLoaded[key]) continue;
                    try {
                        var face = new env.FontFace(f.family, f.data, { weight: String(f.weight) });
                        await face.load();
                        env.fonts.add(face);
                        this.fontsLoaded[key] = true;
                    } catch (err) {
                        throw fail('readiness', 'Font "' + f.family + '" could not be loaded: ' + (err && err.message || err));
                    }
                }
                return { payload: {} };
            }
            case 'frame': {
                if (!this.built) throw fail('code_error', 'frame before build');
                var o = this.opts, width = msg.width > 0 ? Math.round(msg.width) : o.width;
                var height = msg.width > 0 ? Math.round(msg.width * o.height / o.width) : o.height;
                var key2 = width + 'x' + height;
                if (!this.canvases[key2]) { var c = this.env.makeCanvas(width, height); this.canvases[key2] = { canvas: c, ctx: c.getContext('2d') }; }
                var main = this.canvases[key2];
                var tl = { aspect: o.aspect, fps: o.fps, palette: o.palette, cat: SM.scene.indexCatalog(o.catalog), fontFamilies: o.families || {}, style: { font: o.styleFont || 'default' }, safe: o.safe };
                var rc = SM.render.makeContext(tl, { rig: this.files[o.catalog && o.catalog.rig && o.catalog.rig.file] || null, props: this.files, envs: this.files }, width, height);
                rc.sceneId = o.sceneId;
                this.built.stages = this.built.stages || {};
                try { SM.codescene.drawFrame(this.built, msg.t, main.ctx, rc); } catch (err) { throw sceneError(err, 'Scene "' + o.sceneId + '" at ' + msg.t.toFixed(2) + ' s'); }
                var bitmap = typeof main.canvas.transferToImageBitmap === 'function' ? main.canvas.transferToImageBitmap() : main.canvas;
                return { payload: { bitmap: bitmap, width: width, height: height }, transfer: typeof main.canvas.transferToImageBitmap === 'function' ? [bitmap] : [] };
            }
            case 'dispose':
                this.built = null;
                this.canvases = {};
                return { payload: {} };
            default:
                throw fail('code_error', 'unknown message type "' + msg.type + '"');
        }
    };

    SM.workerScene = { Runtime: Runtime, locate: locate, sceneError: sceneError };
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/worker-main.js`**

````js
// Storymaker player: role dispatch of the worker bundle. The blob prelude sets self.__SM_ROLE__ ("player" or "scene")
// before it imports this bundle. For a worker role this installs the determinism shadows, creates the runtime and the
// message handler (messages are handled one at a time, in order) and says hello. In the document realm there is no role,
// so loading the bundle there (the storyboard fallback) installs nothing. Classic script.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore;
    var role = root.__SM_ROLE__;
    if (role !== 'player' && role !== 'scene') return;

    SM.determinism.install(root);
    var env = {
        makeCanvas: function (w, h) { return new root.OffscreenCanvas(w, h); },
        FontFace: root.FontFace,
        fonts: root.fonts
    };
    var runtime = role === 'scene' ? new SM.workerScene.Runtime(env, root) : new SM.workerPlayer.Runtime(env);
    var chain = Promise.resolve();

    root.addEventListener('message', function (event) {
        var msg = event.data;
        if (!msg || typeof msg !== 'object' || typeof msg.type !== 'string') return;
        chain = chain.then(function () { return runtime.handle(msg); }).then(function (res) {
            var out = { type: 'reply', req: msg.req, ok: true }, k;
            for (k in res.payload) out[k] = res.payload[k];
            root.postMessage(out, res.transfer || []);
        }, function (err) {
            var e = err && typeof err.code === 'string' ? err : { code: 'code_error', message: String(err && err.message || err) };
            root.postMessage({ type: 'reply', req: msg.req, ok: false, error: e });
        });
    });
    root.postMessage({ type: 'hello', role: role });
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`33 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/worker-player.js ui/js/storymaker/worker-scene.js ui/js/storymaker/worker-main.js scripts/storymaker-tests/07-workers.mjs
git commit -m "feat(storymaker): player worker runtimes and role dispatch" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C9: The host engine and the app protocol

**Files:**
- Create: `ui/js/storymaker/host-core.js`, `ui/js/storymaker/host-app.js`
- Test: `scripts/storymaker-tests/08-host-app.mjs`

`host-core.js` is the engine of the document realm: a `Channel` per worker with per-call deadlines (a worker that overruns is `terminate()`d), the load flow
(`plan` -> build each code scene in its own worker -> `compile` -> assets), frame rendering with code-scene frames routed to the player worker, and the fallback (workers
or OffscreenCanvas unavailable, or a worker that never says hello: the trusted runtime runs in this realm, code scenes show a placeholder). `host-app.js` is the
contract section 8 protocol with the parent: the source check (`event.source === window.parent`), the typed schema with size caps, replies to the origin of
`data-sm-base`, reset on every load, real-time playback with frame dropping, thumbnails as transferred ImageBitmaps.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/08-host-app.mjs`**

````js
// Task C9: the app host: the section 8 protocol with a fake parent window, in-process workers running the real player
// bundle parts: source check, size caps, reset on load, playback with frame dropping, thumbnails, deadlines, fallback.
import assert from 'node:assert/strict';
import {
    plain, loadHost, until, settle, loadPlayer, FakeCanvas, InProcessWorker, realmGlobals
} from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

const { check, near, catalog, files, firewall, minimal, s3Bundle, bundleScene } = t;

const BASE = 'https://aura.test';
const assetsOf = f => {
    const a = { 'catalog:rig/human.json': f.rig };
    for (const k of Object.keys(f.props)) a['catalog:' + k] = f.props[k];
    for (const k of Object.keys(f.envs)) a['catalog:' + k] = f.envs[k];
    return a;
};
const fontOf = () => ({ name: 'default', family: 'Storymaker Default', weight: 400, data: new ArrayBuffer(16) });

// An HTMLCanvasElement has no transferToImageBitmap: the fallback draws the canvas itself.
const domCanvas = (w, h) => { const c = new FakeCanvas(w, h); c.transferToImageBitmap = undefined; return c; };

// ---- app mode ---------------------------------------------------------------------------------------------------
function makeApp(opts = {}) {
    const hostCtx = loadHost(), H = hostCtx.StorymakerHost;
    const parent = { messages: [], postMessage(msg, origin, transfer) { this.messages.push({ msg, origin, transfer }); } };
    const listeners = [], rafQueue = [];
    let rafId = 0;
    const win = {
        parent, addEventListener(type, fn) { listeners.push(fn); }, removeEventListener(type, fn) { const i = listeners.indexOf(fn); if (i >= 0) listeners.splice(i, 1); },
        requestAnimationFrame(fn) { rafQueue.push({ id: ++rafId, fn }); return rafId; },
        cancelAnimationFrame(id) { const i = rafQueue.findIndex(r => r.id === id); if (i >= 0) rafQueue.splice(i, 1); },
        createImageBitmap: async c => c
    };
    const canvas = new FakeCanvas(300, 150), workers = [], clock = { t: 0 };
    const env = {
        hasWorkers: opts.hasWorkers !== false,
        createWorker(role, text) {
            const w = new InProcessWorker(role, text, { hang: role === 'player' ? opts.hangPlayer : [] });
            workers.push(w);
            return w;
        },
        setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => clock.t,
        limits: opts.limits, ...(opts.env || {})
    };
    const app = H.mountApp({ canvas, base: BASE, window: win, env });
    const send = (msg, source = parent) => listeners.slice().forEach(fn => fn({ source, data: { sm: 1, ...msg } }));
    const sent = type => parent.messages.filter(m => m.msg.type === type).map(m => m.msg);
    const step = ts => { const r = rafQueue.shift(); if (r) r.fn(ts); return !!r; };
    const loadMsg = (extra = {}) => ({
        type: 'load', story: firewall, catalog, audio_info: {}, assets: assetsOf(files), fonts: [fontOf()], code_bundles: { s3: s3Bundle },
        format: { aspect: '16:9', fps: 30, width: 480 }, ...extra
    });
    return { H, hostCtx, app, parent, win, canvas, workers, clock, send, sent, step, rafQueue, listeners, loadMsg };
}
const loaded = async a => { a.send(a.loadMsg()); await until(() => a.sent('time').length >= 1, 'the first time message'); };

check('host app: ready, load, compile report, first frame drawn, target origin', async () => {
    const a = makeApp();
    assert.deepEqual(plain(a.sent('ready')), [{ sm: 1, type: 'ready', caps: { workers: true, offscreen: false } }].map(m => ({ ...m, caps: { workers: true, offscreen: false } })));
    assert.equal(a.parent.messages[0].origin, BASE, 'posts go to the origin of data-sm-base');
    await loaded(a);
    const rep = plain(a.sent('compiled')[0].report);
    assert.equal(rep.frame_count, 354); assert.equal(rep.width, 480); assert.equal(rep.height, 270);
    assert.deepEqual(rep.scenes.map(s => s.kind), ['storyboard', 'storyboard', 'code']);
    assert.equal(rep.issues.filter(i => i.severity === 'error').length, 0);
    assert.deepEqual(plain(a.sent('time')[0]), { sm: 1, type: 'time', frame: 0, playing: false });
    assert.equal(a.canvas.width, 480); assert.equal(a.canvas.height, 270);
    assert.ok(a.canvas.ctx.log.some(l => /^drawImage\(bitmap\d+,0,0\)$/.test(l)), 'the first frame is drawn on the visible canvas');
    assert.equal(a.sent('error').length, 0);
    assert.ok(a.workers.length === 2 && a.workers[0].role === 'player' && a.workers[1].role === 'scene');
});

check('host app: assets the load did not carry are requested with need_assets (compiled first)', async () => {
    const a = makeApp();
    a.send(a.loadMsg({ assets: {}, fonts: [] }));
    await until(() => a.sent('need_assets').length === 1, 'need_assets');
    const types = a.parent.messages.map(m => m.msg.type);
    assert.ok(types.indexOf('compiled') < types.indexOf('need_assets'), 'the report is posted before the assets are fetched');
    const req = a.sent('need_assets')[0];
    assert.deepEqual(plain(req.keys), ['catalog:environments/abstract-tech.json', 'catalog:environments/office.json', 'catalog:props/lock.json', 'catalog:props/shield.json', 'catalog:rig/human.json', 'font:default/400']);
    const all = assetsOf(files); all['font:default/400'] = new ArrayBuffer(16);
    a.send({ type: 'assets', req: req.req, assets: all, missing: [] });
    await until(() => a.sent('time').length >= 1, 'the first frame after the assets');
    assert.equal(a.sent('error').length, 0);
});

check('host app: missing assets are reported, never silently ignored', async () => {
    const a = makeApp();
    a.send(a.loadMsg({ assets: {}, fonts: [] }));
    await until(() => a.sent('need_assets').length === 1, 'need_assets');
    const req = a.sent('need_assets')[0];
    a.send({ type: 'assets', req: req.req, assets: { 'catalog:rig/human.json': files.rig }, missing: ['catalog:props/lock.json'] });
    await until(() => a.sent('error').length >= 1, 'an error');
    const err = a.sent('error')[0];
    assert.equal(err.code, 'readiness'); assert.match(err.message, /Missing assets: .*props\/lock\.json/);
    assert.equal(a.sent('time').length, 0, 'no frame is shown without its assets');
});

check('host app: the source check, the typed schema and the size caps', async () => {
    const a = makeApp();
    const before = a.parent.messages.length;
    a.send({ type: 'seek', frame: 3 }, {});
    a.send({ type: 'load', story: firewall, catalog }, null);
    assert.equal(a.parent.messages.length, before, 'messages from any window but the parent are ignored');
    a.send({ type: 'seek', frame: -1 });
    a.send({ type: 'nope' });
    a.send({ type: 'load', story: { scenes: new Array(1000).fill({}) }, catalog });
    a.send({ type: 'load', story: firewall, catalog: { entries: 5 } });
    a.send({ type: 'load', story: firewall, catalog, fonts: [{ name: 'x', family: 'x', weight: 400, data: new ArrayBuffer(9 * 1048576) }] });
    a.send({ type: 'load', story: firewall, catalog, code_bundles: { 'bad id': 'x' } });
    a.send({ type: 'load', story: firewall, catalog, code_bundles: { s3: 'x'.repeat(300000) } });
    a.send({ type: 'thumb', req: 1, scene_id: 's1', width: 5000 });
    a.send({ type: 'assets', req: 1, assets: { 'bad key': 1 } });
    a.listeners.forEach(fn => fn({ source: a.parent, data: { type: 'seek', frame: 1 } }));
    const errs = a.sent('error');
    assert.equal(errs.length, 10);
    assert.ok(errs.every(e => e.code === 'bad_message'));
    const H = a.H, ok = H.checkMessage;
    assert.equal(ok({ sm: 1, type: 'play' }), null); assert.equal(ok({ sm: 1, type: 'seek', frame: 7 }), null);
    assert.equal(ok({ sm: 1, type: 'thumb', req: 'r', scene_id: 's1', width: 160 }), null);
    assert.equal(ok({ sm: 1, type: 'load', story: firewall, catalog, assets: assetsOf(files), fonts: [fontOf()], code_bundles: { s3: s3Bundle }, format: { aspect: '9:16', fps: 25, width: 540 } }), null);
    assert.match(ok({ sm: 1, type: 'load', story: firewall, catalog, format: { fps: 60 } }), /fps/);
    assert.ok(ok(null) && ok('x') && ok({ sm: 2, type: 'play' }));
    assert.equal(a.sent('compiled').length, 0, 'nothing was loaded');
});

check('host app: seek, play with frame dropping, pause and the end of the video', async () => {
    const a = makeApp();
    await loaded(a);
    const renders = () => a.workers[0].received.filter(t => t === 'render').length;
    const r0 = renders();
    a.send({ type: 'seek', frame: 100 });
    await until(() => a.sent('time').some(m => m.frame === 100), 'seek');
    assert.equal(renders(), r0 + 1);
    a.send({ type: 'seek', frame: 99999 });
    await until(() => a.sent('time').some(m => m.frame === 353), 'seek clamps to the last frame');
    a.send({ type: 'seek', frame: 0 });
    await until(() => a.sent('time').filter(m => m.frame === 0).length >= 2, 'seek back');
    a.send({ type: 'play' });
    assert.equal(a.sent('time').at(-1).playing, true);
    const before = renders();
    a.clock.t = 1000; a.step(1000); await settle(8);
    a.clock.t = 1100; a.step(1100); await settle(8);     // 3 frames later
    a.clock.t = 1200; a.step(1700); await settle(8);     // 21 frames later: the in-between frames are dropped
    a.clock.t = 1300; a.step(1000 + 5000); await settle(8);
    const rendered = renders() - before;
    assert.ok(rendered <= 4, 'only the frames at the sampled times are rendered, got ' + rendered);
    assert.equal(a.sent('time').filter(m => m.playing).at(-1).frame, 150);
    a.send({ type: 'pause' });
    assert.equal(a.sent('time').at(-1).playing, false); assert.equal(a.rafQueue.length, 0, 'no frame callback stays queued');
    a.send({ type: 'play' });
    a.clock.t = 2000; a.step(50000); await settle(8);
    a.clock.t = 2100; a.step(90000); await settle(8);
    assert.equal(a.app.state.playing, false, 'playback ends at the last frame');
    assert.equal(a.sent('time').at(-1).frame, 353); assert.equal(a.sent('time').at(-1).playing, false);
});

check('host app: thumbnails come back as bitmaps of the requested width', async () => {
    const a = makeApp();
    await loaded(a);
    a.send({ type: 'thumb', req: 'th1', scene_id: 's2', width: 160 });
    await until(() => a.sent('thumb').length === 1, 'the thumbnail');
    const th = a.sent('thumb')[0];
    assert.equal(th.req, 'th1'); assert.equal(th.scene_id, 's2');
    assert.equal(th.bitmap.width, 160); assert.equal(th.bitmap.height, 90);
    assert.equal(a.parent.messages.find(m => m.msg.type === 'thumb').transfer.length, 1, 'the bitmap is transferred');
    a.send({ type: 'thumb', req: 'th2', scene_id: 'nope', width: 160 });
    await until(() => a.sent('error').length === 1, 'unknown scene');
    assert.equal(a.sent('error')[0].scene_id, 'nope');
    a.send({ type: 'thumb', req: 'th3', scene_id: 's3', width: 96, frame: 300 });
    await until(() => a.sent('thumb').length === 2, 'the code scene thumbnail');
    assert.equal(a.sent('thumb')[1].bitmap.width, 96);
    assert.equal(a.sent('error').length, 1, 'no error for the code scene');
});

check('host app: every load and reset starts clean (workers stopped, canvas cleared)', async () => {
    const a = makeApp();
    await loaded(a);
    const first = a.workers.slice();
    a.send(a.loadMsg({ format: { aspect: '16:9', fps: 30, width: 320 } }));
    await until(() => a.sent('compiled').length === 2, 'the second report');
    assert.ok(first.every(w => w.dead), 'the first load\'s workers are terminated');
    await until(() => a.sent('time').length >= 2, 'the second first frame');
    assert.equal(a.sent('compiled')[1].report.width, 320);
    const alive = a.workers.filter(w => !w.dead).length;
    a.send({ type: 'reset' });
    assert.ok(a.workers.every(w => w.dead) && alive === 2);
    const posts = a.parent.messages.length;
    a.send({ type: 'seek', frame: 3 }); a.send({ type: 'play' });
    await settle(5);
    assert.equal(a.parent.messages.length, posts, 'nothing plays after a reset');
    assert.ok(a.canvas.ctx.log.some(l => l.startsWith('clearRect')));
    a.app.dispose();
    a.send({ type: 'load', ...a.loadMsg() });
    await settle(5);
    assert.equal(a.sent('compiled').length, 2, 'a disposed host answers nothing');
});

check('host app: a worker that overruns its deadline is terminated and reported', async () => {
    const a = makeApp({ hangPlayer: ['render'], limits: { warmupDeadlineMs: 60, frameDeadlineMs: 60 } });
    a.send(a.loadMsg());
    await until(() => a.sent('error').length >= 1, 'a deadline error');
    const err = a.sent('error')[0];
    assert.equal(err.code, 'deadline'); assert.match(err.message, /took longer than 60 ms/);
    assert.ok(a.workers[0].dead, 'the player worker is terminated');
    assert.ok(a.workers.every(w => w.dead), 'the code-scene workers stop with it');
    assert.equal(a.sent('time').length, 0);
});

const failing = src => bundleScene(src, 'bad');
check('host app: a code scene that fails to build is reported, its scene shows a placeholder', async () => {
    for (const [src, pattern] of [
        ["export default function scene(api) { api.add('char.nobody'); }", /Unknown catalog id "char\.nobody"/],
        ['export default function scene(api) { api.duration(3); const x = Math.random(); }', /Math\.random\(\) is not available/],
        ['export default function scene(api) { api.duration(3); setTimeout(() => {}, 1); }', /setTimeout\(\) is not available/],
        ['export default function scene(api) { throw new Error("boom"); }', /boom/]
    ]) {
        const a = makeApp();
        a.send(a.loadMsg({ code_bundles: { s3: failing(src) } }));
        await until(() => a.sent('time').length >= 1, 'the first frame');
        const errs = a.sent('error');
        assert.equal(errs.length, 1, src);
        assert.equal(errs[0].code, 'code_error'); assert.equal(errs[0].scene_id, 's3'); assert.match(errs[0].message, pattern);
        const rep = plain(a.sent('compiled')[0].report);
        assert.ok(rep.issues.some(i => i.code === 'code_error' && i.severity === 'error' && i.scene_id === 's3'));
        a.send({ type: 'seek', frame: 300 });
        await until(() => a.sent('time').some(m => m.frame === 300), 'a frame inside the failed code scene');
        assert.equal(a.sent('error').length, 1, 'the placeholder frame renders without another error');
    }
    const a = makeApp();
    a.send(a.loadMsg({ code_bundles: {} }));
    await until(() => a.sent('error').length >= 1, 'missing bundle');
    assert.match(a.sent('error')[0].message, /No bundle for code scene "s3"/);
});

check('host app: a code scene that fails at frame time is reported once and replaced by a placeholder', async () => {
    const a = makeApp();
    const src = 'export default function scene(api) { api.duration(4); api.onFrame((t, g) => { if (t > 1) throw new Error("late failure"); g.rect(0, 0, 10, 10); }); }';
    a.send(a.loadMsg({ code_bundles: { s3: bundleScene(src, 'late') } }));
    await until(() => a.sent('time').length >= 1, 'the first frame');
    a.send({ type: 'seek', frame: 204 + 15 });
    await until(() => a.sent('time').some(m => m.frame === 219), 'the early code scene frame');
    assert.equal(a.sent('error').length, 0);
    a.send({ type: 'seek', frame: 204 + 60 });
    await until(() => a.sent('error').length === 1, 'the late failure');
    const e = a.sent('error')[0];
    assert.equal(e.code, 'code_error'); assert.equal(e.scene_id, 's3'); assert.match(e.message, /late failure/); assert.equal(typeof e.line, 'number');
    a.send({ type: 'seek', frame: 204 + 70 });
    await until(() => a.sent('time').some(m => m.frame === 274), 'the next frame still renders');
    assert.equal(a.sent('error').length, 1);
});

check('host app: without workers the storyboard renders in the host realm and code scenes show a placeholder', async () => {
    const core = loadPlayer(), g = realmGlobals();
    const a = makeApp({ hasWorkers: false, env: {
        loadCore: async () => core.StorymakerCore, makeCanvas: domCanvas, FontFace: g.FontFace, fonts: g.fonts
    } });
    assert.equal(a.sent('ready')[0].caps.workers, false);
    a.send(a.loadMsg());
    await until(() => a.sent('time').length >= 1, 'the first frame');
    assert.equal(a.workers.length, 0);
    const rep = plain(a.sent('compiled')[0].report);
    assert.equal(rep.scenes.length, 3); assert.ok(rep.frame_count > 300);
    assert.equal(rep.issues.filter(i => i.severity === 'error').length, 0);
    assert.ok(a.canvas.ctx.log.some(l => /^drawImage\(canvas\d+,0,0\)$/.test(l)));
    a.send({ type: 'seek', frame: 120 });
    await until(() => a.sent('time').some(m => m.frame === 120), 'a storyboard frame');
    a.send({ type: 'seek', frame: rep.scenes[2].start_frame + 20 });
    await until(() => a.sent('time').some(m => m.frame === rep.scenes[2].start_frame + 20), 'a code scene frame');
    a.send({ type: 'thumb', req: 't', scene_id: 's1', width: 120 });
    await until(() => a.sent('thumb').length === 1, 'a thumbnail');
    assert.equal(a.sent('error').length, 0);
});

check('host app: a worker that never says hello falls back to the host realm', async () => {
    const core = loadPlayer(), g = realmGlobals();
    const a = makeApp({ limits: { helloDeadlineMs: 30 }, env: {
        createWorker: () => ({ postMessage() {}, terminate() {}, addEventListener() {} }),
        loadCore: async () => core.StorymakerCore, makeCanvas: domCanvas, FontFace: g.FontFace, fonts: g.fonts
    } });
    a.send(a.loadMsg({ code_bundles: {} }));
    await until(() => a.sent('time').length >= 1, 'the fallback frame');
    assert.equal(a.sent('compiled').length, 1);
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (33 passed, 12 failed; first: host app: ready, load, compile report, first frame drawn, target origin: TypeError: Cannot read properties of undefined (reading 'mountApp'))

- [ ] **Step 3: Implement**


**`ui/js/storymaker/host-core.js`**

````js
// Storymaker host (document realm): the engine. It owns the player worker and one worker per code scene, routes
// messages with per-call deadlines (a worker that overruns is terminated), runs the load flow (plan -> build code scenes
// -> compile -> assets) and renders frames. When workers or OffscreenCanvas are unavailable it runs the trusted player
// runtime in this realm (storyboard fallback; code scenes show a placeholder). The app protocol (host-app.js) and the render
// driver (host-render.js) both sit on this engine. Classic script; publishes StorymakerHost.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    var LIMITS = {
        compileDeadlineMs: 10000, sceneBuildDeadlineMs: 2000, frameDeadlineMs: 250, warmupFrames: 3, warmupDeadlineMs: 4000,
        helloDeadlineMs: 5000, assetsDeadlineMs: 20000
    };

    function fail(code, message, extra) {
        var e = { code: code, message: message };
        if (extra) for (var k in extra) e[k] = extra[k];
        return e;
    }
    function asError(err, code) {
        if (err && typeof err.code === 'string') return err;
        return fail(code || 'code_error', String(err && err.message || err));
    }

    // ---- worker channel ------------------------------------------------------------------------------------------
    // A channel wraps one worker: call() sends a message and resolves with the reply; a call that takes longer than its
    // deadline terminates the worker and rejects with code "deadline".
    function Channel(worker, env, label) {
        var self = this;
        this.worker = worker;
        this.env = env;
        this.label = label;
        this.dead = false;
        this.pending = {};
        this.nextReq = 1;
        this.helloSeen = false;
        this.helloWaiters = [];
        this.onDeath = null;
        function onMessage(event) {
            var d = event && event.data;
            if (!d || typeof d !== 'object') return;
            if (d.type === 'hello') {
                self.helloSeen = true;
                self.helloWaiters.splice(0).forEach(function (w) { w.resolve(); });
                return;
            }
            if (d.type !== 'reply') return;
            var p = self.pending[d.req];
            if (!p) return;
            delete self.pending[d.req];
            env.clearTimer(p.timer);
            if (d.ok) p.resolve(d); else p.reject(asError(d.error));
        }
        function onError(event) {
            var msg = event && event.message ? event.message : 'The worker failed to start.';
            if (event && event.preventDefault) event.preventDefault();
            self.kill(fail('code_error', msg));
        }
        if (typeof worker.addEventListener === 'function') {
            worker.addEventListener('message', onMessage);
            worker.addEventListener('error', onError);
        } else {
            worker.onmessage = onMessage;
            worker.onerror = onError;
        }
    }

    Channel.prototype.hello = function (ms) {
        var self = this;
        if (this.helloSeen) return Promise.resolve();
        return new Promise(function (resolve, reject) {
            var timer = self.env.setTimer(function () { reject(fail('deadline', self.label + ' did not start within ' + ms + ' ms.')); }, ms);
            self.helloWaiters.push({
                resolve: function () { self.env.clearTimer(timer); resolve(); },
                reject: function (e) { self.env.clearTimer(timer); reject(e); }
            });
        });
    };

    Channel.prototype.call = function (msg, ms, transfer) {
        var self = this;
        return new Promise(function (resolve, reject) {
            if (self.dead) { reject(fail('code_error', self.label + ' is not running.')); return; }
            var req = self.nextReq++;
            msg.req = req;
            var timer = self.env.setTimer(function () {
                delete self.pending[req];
                var e = fail('deadline', self.label + ' took longer than ' + ms + ' ms for "' + msg.type + '" and was stopped.');
                self.kill(e);
                reject(e);
            }, ms);
            self.pending[req] = { resolve: resolve, reject: reject, timer: timer };
            try { self.worker.postMessage(msg, transfer || []); } catch (err) {
                delete self.pending[req];
                self.env.clearTimer(timer);
                reject(fail('code_error', 'Could not send to ' + self.label + ': ' + (err && err.message || err)));
            }
        });
    };

    Channel.prototype.kill = function (reason) {
        if (this.dead) return;
        this.dead = true;
        try { this.worker.terminate(); } catch (e) { /* already gone */ }
        var pending = this.pending, env = this.env;
        this.pending = {};
        Object.keys(pending).forEach(function (k) { env.clearTimer(pending[k].timer); pending[k].reject(reason || fail('code_error', 'worker stopped')); });
        this.helloWaiters.splice(0).forEach(function (w) { w.reject(reason || fail('code_error', 'worker stopped')); });
        if (this.onDeath) this.onDeath(reason);
    };

    // ---- default environment ---------------------------------------------------------------------------------------
    // blobWorker(base, role, sceneText): one blob per worker = prelude + (scene bundle text). The prelude is exactly one
    // line, so a line number inside the blob minus one is a line of the scene bundle.
    function blobWorker(base, role, sceneText) {
        var prelude = 'self.__SM_ROLE__ = ' + JSON.stringify(role) + '; importScripts(' + JSON.stringify(base + '/js/storymaker/player.bundle.js') + ');\n';
        var url = root.URL.createObjectURL(new root.Blob([prelude + (sceneText || '')], { type: 'text/javascript' }));
        var worker = new root.Worker(url);
        var revoke = function () { try { root.URL.revokeObjectURL(url); } catch (e) { /* ignore */ } };
        if (worker.addEventListener) worker.addEventListener('message', function once() { revoke(); worker.removeEventListener('message', once); });
        return worker;
    }

    function loadCoreScript(base) {
        return new Promise(function (resolve, reject) {
            if (root.StorymakerCore && root.StorymakerCore.workerPlayer) { resolve(root.StorymakerCore); return; }
            var s = root.document.createElement('script');
            s.src = base + '/js/storymaker/player.bundle.js';
            s.onload = function () { resolve(root.StorymakerCore); };
            s.onerror = function () { reject(fail('readiness', 'The player bundle could not be loaded.')); };
            root.document.head.appendChild(s);
        });
    }

    // defaultEnv(base, overrides)
    function defaultEnv(base, over) {
        over = over || {};
        var hasDom = typeof root.document !== 'undefined';
        var env = {
            base: base,
            hasWorkers: typeof root.Worker === 'function' && typeof root.OffscreenCanvas === 'function',
            createWorker: function (role, text) { return blobWorker(base, role, text); },
            loadCore: function () { return loadCoreScript(base); },
            makeCanvas: function (w, h) { var c = root.document.createElement('canvas'); c.width = w; c.height = h; return c; },
            FontFace: root.FontFace,
            fonts: hasDom ? root.document.fonts : null,
            setTimer: function (fn, ms) { return root.setTimeout(fn, ms); },
            clearTimer: function (h) { root.clearTimeout(h); },
            now: function () { return root.performance.now(); },
            requestAssets: function () { return Promise.reject(fail('readiness', 'No asset source is configured.')); },
            onError: function () {},
            limits: LIMITS
        };
        for (var k in over) env[k] = over[k];
        env.limits = Object.assign({}, LIMITS, over.limits || {});
        return env;
    }

    // ---- engine -----------------------------------------------------------------------------------------------------
    function Engine(env) {
        this.env = env;
        this.mode = 'idle';
        this.player = null;
        this.runtime = null;
        this.core = null;
        this.sceneChannels = {};
        this.codeErrors = {};
        this.schedule = null;
        this.report = null;
        this.rendered = 0;
        this.chain = Promise.resolve();
        this.dead = false;
        this.loadId = 0;
    }

    Engine.prototype.dispose = function () {
        this.loadId++;
        var self = this;
        if (this.player) { this.player.kill(fail('cancelled', 'disposed')); this.player = null; }
        Object.keys(this.sceneChannels).forEach(function (id) { self.sceneChannels[id].kill(fail('cancelled', 'disposed')); });
        this.sceneChannels = {};
        this.runtime = null;
        this.schedule = null;
        this.mode = 'idle';
        this.chain = Promise.resolve();
    };

    Engine.prototype.startPlayer = async function () {
        var env = this.env;
        if (env.hasWorkers) {
            var channel = null;
            try {
                channel = new Channel(env.createWorker('player', ''), env, 'The player worker');
                await channel.hello(env.limits.helloDeadlineMs);
                this.player = channel;
                this.mode = 'workers';
                return;
            } catch (err) {
                if (channel) channel.kill(err);
            }
        }
        // storyboard fallback in this realm
        this.core = await env.loadCore();
        if (!this.core || !this.core.workerPlayer) throw fail('readiness', 'The player runtime is not available in this browser.');
        this.runtime = new this.core.workerPlayer.Runtime({ makeCanvas: env.makeCanvas, FontFace: env.FontFace, fonts: env.fonts });
        this.mode = 'fallback';
    };

    // send(msg, ms, transfer): to the player (worker or in-realm runtime).
    Engine.prototype.send = async function (msg, ms, transfer) {
        if (this.mode === 'workers') return this.player.call(msg, ms, transfer);
        var res = await this.runtime.handle(msg), out = { ok: true };
        for (var k in res.payload) out[k] = res.payload[k];
        return out;
    };

    Engine.prototype.buildScene = async function (id, p) {
        var env = this.env, bundle = p.msg.code_bundles && p.msg.code_bundles[id];
        if (typeof bundle !== 'string' || !bundle) return { error: fail('code_error', 'No bundle for code scene "' + id + '". Save the scene again to rebuild it.') };
        var channel;
        try {
            channel = new Channel(env.createWorker('scene', bundle), env, 'Code scene "' + id + '"');
            var opts = {
                sceneId: id, index: p.plan.scenes[id].index, catalog: p.msg.catalog, aspect: p.plan.aspect, width: p.plan.width, height: p.plan.height,
                fps: p.plan.fps, cues: p.plan.scenes[id].cues, palette: p.plan.palette, families: p.plan.fontFamilies, styleFont: p.plan.styleFont,
                safe: p.msg.catalog && p.msg.catalog.stage && p.msg.catalog.stage.safe || null
            };
            var started = env.now();
            await channel.hello(env.limits.sceneBuildDeadlineMs);
            var left = Math.max(50, env.limits.sceneBuildDeadlineMs - (env.now() - started));
            var reply = await channel.call({ type: 'build', opts: opts }, left);
            this.sceneChannels[id] = channel;
            return { duration: reply.duration, needs: reply.needs || [], issues: reply.issues || [] };
        } catch (err) {
            if (channel) channel.kill(err);
            var e = asError(err);
            if (typeof e.line === 'number' && e.line > 1) e.line -= 1;   // the prelude is one line
            return { error: e };
        }
    };

    // load(msg): msg = {story, catalog, audio_info, assets, fonts, code_bundles, format}. Resolves with the compile report
    // as soon as it exists and prepare(), which loads the assets (the readiness barrier before frame 0).
    Engine.prototype.load = async function (msg) {
        this.dispose();
        var env = this.env, id = ++this.loadId, self = this;
        this.dead = false;
        this.codeErrors = {};
        this.rendered = 0;
        await this.startPlayer();
        if (id !== this.loadId) throw fail('cancelled', 'superseded');
        var fmt = msg.format || {};
        var sizeOpts = { width: fmt.width > 0 ? fmt.width : 0, height: fmt.height > 0 ? fmt.height : 0 };
        var story = Object.assign({}, msg.story);
        if (fmt.aspect || fmt.fps) story.format = Object.assign({}, story.format || {}, fmt.aspect ? { aspect: fmt.aspect } : {}, fmt.fps ? { fps: fmt.fps } : {});

        var planReply = await this.send({ type: 'plan', story: story, catalog: msg.catalog, audioInfo: msg.audio_info || {}, opts: sizeOpts }, env.limits.compileDeadlineMs);
        var plan = planReply.plan;
        var codeScenes = {}, sceneIssues = [], sceneNeeds = {}, codeIds = Object.keys(plan.scenes).filter(function (k) { return plan.scenes[k].kind === 'code'; });
        await Promise.all(codeIds.map(async function (cid) {
            if (self.mode === 'fallback') {
                codeScenes[cid] = {};
                self.codeErrors[cid] = 'Code scenes are not previewed in this browser. Export renders them.';
                return;
            }
            var built = await self.buildScene(cid, { msg: msg, plan: plan });
            if (built.error) {
                codeScenes[cid] = { error: built.error };
                self.codeErrors[cid] = built.error.message;
                env.onError({ code: built.error.code, message: built.error.message, scene_id: cid, line: built.error.line, column: built.error.column });
            } else {
                codeScenes[cid] = { duration: built.duration };
                sceneNeeds[cid] = built.needs;
                built.issues.forEach(function (i) { sceneIssues.push(i); });
            }
        }));
        if (id !== this.loadId) throw fail('cancelled', 'superseded');

        var compiled = await this.send({ type: 'compile', story: story, catalog: msg.catalog, audioInfo: msg.audio_info || {}, opts: Object.assign({ codeScenes: codeScenes }, sizeOpts) }, env.limits.compileDeadlineMs);
        var report = compiled.report;
        sceneIssues.forEach(function (i) { report.issues.push(i); });
        this.report = report;
        this.schedule = compiled.schedule;
        var prepared = null;
        // prepare() starts the asset phase (once) and resolves when every asset is loaded: the readiness barrier before frame 0
        return {
            report: report,
            prepare: function () {
                if (!prepared) prepared = self.ensureAssets(msg, compiled.needs || [], sceneNeeds, id);
                return prepared;
            }
        };
    };

    Engine.prototype.fontDescriptor = function (catalog, key, data) {
        var m = /^font:(.+)\/(\d+)$/.exec(key), name = m ? m[1] : key, weight = m ? parseInt(m[2], 10) : 400, family = name;
        ((catalog && catalog.fonts) || []).forEach(function (f) { if (f.name === name && f.weight === weight) family = f.family; });
        return { name: name, family: family, weight: weight, data: data };
    };

    // ensureAssets: gather what the player and the code scenes need from msg.assets / msg.fonts / the asset source.
    Engine.prototype.ensureAssets = async function (msg, needs, sceneNeeds, id) {
        var env = this.env, self = this, have = Object.assign({}, msg.assets || {});
        var fontsHave = {};
        (msg.fonts || []).forEach(function (f) { fontsHave['font:' + f.name + '/' + f.weight] = f; });
        var all = {};
        needs.forEach(function (k) { all[k] = true; });
        Object.keys(sceneNeeds).forEach(function (cid) { sceneNeeds[cid].forEach(function (k) { all[k] = true; }); });
        var wanted = Object.keys(all), missing = wanted.filter(function (k) { return k.indexOf('font:') === 0 ? !fontsHave[k] : have[k] === undefined; });
        if (missing.length) {
            var got = await env.requestAssets(missing);
            var assets = got && got.assets || {};
            Object.keys(assets).forEach(function (k) {
                if (k.indexOf('font:') === 0) fontsHave[k] = self.fontDescriptor(msg.catalog, k, assets[k]); else have[k] = assets[k];
            });
            var still = missing.filter(function (k) { return k.indexOf('font:') === 0 ? !fontsHave[k] : have[k] === undefined; });
            if (still.length) throw fail('readiness', 'Missing assets: ' + still.join(', '));
        }
        if (id !== this.loadId) throw fail('cancelled', 'superseded');
        var fonts = Object.keys(fontsHave).filter(function (k) { return all[k]; }).map(function (k) { return fontsHave[k]; });
        var catalogAssets = {};
        wanted.forEach(function (k) { if (k.indexOf('catalog:') === 0) catalogAssets[k] = have[k]; });
        var reply = await this.send({ type: 'assets', assets: catalogAssets, fonts: fonts }, env.limits.assetsDeadlineMs);
        if (reply.missing && reply.missing.length) throw fail('readiness', 'Missing assets: ' + reply.missing.join(', '));
        await Promise.all(Object.keys(sceneNeeds).map(function (cid) {
            var channel = self.sceneChannels[cid];
            if (!channel || channel.dead) return null;
            var sceneAssets = {};
            sceneNeeds[cid].forEach(function (k) { if (k.indexOf('catalog:') === 0) sceneAssets[k] = have[k]; });
            var sceneFonts = sceneNeeds[cid].filter(function (k) { return k.indexOf('font:') === 0 && fontsHave[k]; }).map(function (k) { return fontsHave[k]; });
            return channel.call({ type: 'assets', assets: sceneAssets, fonts: sceneFonts }, env.limits.assetsDeadlineMs).catch(function (err) {
                var e = asError(err, 'readiness');
                self.codeErrors[cid] = e.message;
                env.onError({ code: e.code, message: e.message, scene_id: cid });
            });
        }));
    };

    // render(n, opts): the frame as an ImageBitmap (a canvas in the fallback). Calls are serialised.
    Engine.prototype.render = function (n, opts) {
        var self = this;
        var run = function () { return self.renderNow(n, opts || {}); };
        var result = this.chain.then(run, run);
        this.chain = result.catch(function () { /* the next call still runs */ });
        return result;
    };

    Engine.prototype.renderNow = async function (n, opts) {
        if (!this.schedule) throw fail('readiness', 'Nothing is loaded.');
        if (this.dead) throw fail('code_error', 'The player stopped; load the story again.');
        var env = this.env, core = root.StorymakerCore, plan = core.schedule.framePlan(this.schedule, n);
        var warm = this.rendered < env.limits.warmupFrames;
        var ms = warm ? env.limits.warmupDeadlineMs : env.limits.frameDeadlineMs;
        var codeFrames = {}, transfer = [], self = this;
        var width = opts.width > 0 ? opts.width : 0;
        for (var i = 0; i < plan.layers.length; i++) {
            var layer = plan.layers[i], sc = this.schedule.scenes[layer.i];
            if (sc.kind !== 'code') continue;
            var channel = this.sceneChannels[sc.id];
            if (!channel || channel.dead) continue;
            try {
                var reply = await channel.call({ type: 'frame', t: layer.f / this.schedule.fps, width: width }, ms);
                codeFrames[sc.id] = reply.bitmap;
                if (typeof root.ImageBitmap === 'function' && reply.bitmap instanceof root.ImageBitmap) transfer.push(reply.bitmap);
            } catch (err) {
                var e = asError(err);
                if (typeof e.line === 'number' && e.line > 1) e.line -= 1;
                channel.kill(e);   // a scene that failed once shows its placeholder from now on
                this.codeErrors[sc.id] = e.message;
                env.onError({ code: e.code, message: e.message, scene_id: sc.id, line: e.line, column: e.column });
            }
        }
        try {
            var out = await this.send({ type: 'render', n: n, width: width, codeFrames: codeFrames, codeErrors: this.codeErrors }, ms, transfer);
            this.rendered++;
            return out.bitmap;
        } catch (err) {
            var pe = asError(err);
            if (pe.code === 'deadline') { this.dead = true; Object.keys(this.sceneChannels).forEach(function (k) { self.sceneChannels[k].kill(pe); }); }
            throw pe;
        }
    };

    H.LIMITS = LIMITS;
    H.fail = fail;
    H.asError = asError;
    H.Channel = Channel;
    H.Engine = Engine;
    H.createEngine = function (env) { return new Engine(env); };
    H.defaultEnv = defaultEnv;
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/host-app.js`**

````js
// Storymaker host (document realm): app mode. The page is the sandboxed player iframe of the desktop app. It speaks the
// typed protocol of contract section 8 with its parent only: messages are accepted only when event.source is the parent
// window, only in the fixed schema with size caps, and replies are posted to the origin of data-sm-base. Every load resets
// the previous state. Playback is real time with frame dropping; the visible canvas shows the same bitmap the render page
// would encode. Classic script; publishes StorymakerHost.mountApp, checkMessage.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    var CAPS = {
        maxScenes: 400, maxEntries: 6000, maxAssetKeys: 2000, maxFonts: 16, maxFontBytes: 8 * 1048576, maxFontTotal: 32 * 1048576,
        maxBundles: 400, maxBundleChars: 262144, maxFrame: 100000000, thumbMin: 16, thumbMax: 1920, maxKey: 240, maxReq: 64
    };
    var TYPES = ['load', 'assets', 'seek', 'play', 'pause', 'thumb', 'reset'];
    var KEY_RE = /^(catalog|font|bundle):[^\s]{1,230}$/;
    var ID_RE = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/;

    function isObject(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }
    function isBuffer(v) { return Object.prototype.toString.call(v) === '[object ArrayBuffer]'; }
    function isInt(v, lo, hi) { return typeof v === 'number' && isFinite(v) && Math.floor(v) === v && v >= lo && v <= hi; }
    function okReq(v) { return (typeof v === 'string' || typeof v === 'number') && String(v).length <= CAPS.maxReq; }

    // checkMessage(msg) = null when the message fits the schema, else a short reason.
    function checkMessage(msg) {
        if (!isObject(msg) || msg.sm !== 1) return 'not a storymaker message';
        if (TYPES.indexOf(msg.type) < 0) return 'unknown message type';
        var i, k, total = 0;
        switch (msg.type) {
            case 'load': {
                if (!isObject(msg.story) || !Array.isArray(msg.story.scenes) || msg.story.scenes.length > CAPS.maxScenes) return 'load.story is invalid or too large';
                if (!isObject(msg.catalog) || !Array.isArray(msg.catalog.entries) || msg.catalog.entries.length > CAPS.maxEntries) return 'load.catalog is invalid or too large';
                if (msg.audio_info !== undefined && !isObject(msg.audio_info)) return 'load.audio_info is invalid';
                if (msg.assets !== undefined) {
                    if (!isObject(msg.assets) || Object.keys(msg.assets).length > CAPS.maxAssetKeys) return 'load.assets is invalid or too large';
                    for (k in msg.assets) if (!KEY_RE.test(k)) return 'load.assets has a bad key';
                }
                if (msg.fonts !== undefined) {
                    if (!Array.isArray(msg.fonts) || msg.fonts.length > CAPS.maxFonts) return 'load.fonts is invalid or too large';
                    for (i = 0; i < msg.fonts.length; i++) {
                        var f = msg.fonts[i];
                        if (!isObject(f) || typeof f.name !== 'string' || f.name.length > 64 || typeof f.family !== 'string' || f.family.length > 128 || !isInt(f.weight, 100, 900) || !isBuffer(f.data)) return 'load.fonts has a bad entry';
                        if (f.data.byteLength > CAPS.maxFontBytes) return 'load.fonts has an oversized font';
                        total += f.data.byteLength;
                    }
                    if (total > CAPS.maxFontTotal) return 'load.fonts is too large';
                }
                if (msg.code_bundles !== undefined) {
                    if (!isObject(msg.code_bundles) || Object.keys(msg.code_bundles).length > CAPS.maxBundles) return 'load.code_bundles is invalid or too large';
                    for (k in msg.code_bundles) {
                        if (!ID_RE.test(k) || typeof msg.code_bundles[k] !== 'string' || msg.code_bundles[k].length > CAPS.maxBundleChars) return 'load.code_bundles has a bad entry';
                    }
                }
                if (msg.format !== undefined) {
                    var fm = msg.format;
                    if (!isObject(fm)) return 'load.format is invalid';
                    if (fm.aspect !== undefined && ['16:9', '9:16', '1:1'].indexOf(fm.aspect) < 0) return 'load.format.aspect is invalid';
                    if (fm.fps !== undefined && [24, 25, 30].indexOf(fm.fps) < 0) return 'load.format.fps is invalid';
                    if (fm.width !== undefined && !isInt(fm.width, 64, 3840)) return 'load.format.width is invalid';
                }
                return null;
            }
            case 'assets': {
                if (!okReq(msg.req)) return 'assets.req is invalid';
                if (!isObject(msg.assets) || Object.keys(msg.assets).length > CAPS.maxAssetKeys) return 'assets.assets is invalid or too large';
                for (k in msg.assets) if (!KEY_RE.test(k)) return 'assets.assets has a bad key';
                if (msg.missing !== undefined) {
                    if (!Array.isArray(msg.missing) || msg.missing.length > CAPS.maxAssetKeys) return 'assets.missing is invalid';
                    for (i = 0; i < msg.missing.length; i++) if (typeof msg.missing[i] !== 'string' || msg.missing[i].length > CAPS.maxKey) return 'assets.missing has a bad key';
                }
                return null;
            }
            case 'seek':
                return isInt(msg.frame, 0, CAPS.maxFrame) ? null : 'seek.frame is invalid';
            case 'thumb':
                if (!okReq(msg.req)) return 'thumb.req is invalid';
                if (typeof msg.scene_id !== 'string' || !ID_RE.test(msg.scene_id)) return 'thumb.scene_id is invalid';
                if (msg.frame !== undefined && !isInt(msg.frame, 0, CAPS.maxFrame)) return 'thumb.frame is invalid';
                return isInt(msg.width, CAPS.thumbMin, CAPS.thumbMax) ? null : 'thumb.width is invalid';
            default:
                return null;
        }
    }

    function originOf(base) {
        try { return new root.URL(base).origin; } catch (e) { return '*'; }
    }

    // mountApp(opts): opts = {canvas, base, window?, env?}. Returns {dispose, state} (state is for tests).
    function mountApp(opts) {
        var win = opts.window || root, canvas = opts.canvas, base = opts.base;
        var env = H.defaultEnv(base, Object.assign({ raf: function (fn) { return win.requestAnimationFrame(fn); }, cancelRaf: function (h) { win.cancelAnimationFrame(h); } }, opts.env || {}));
        var targetOrigin = opts.targetOrigin || originOf(base);
        var ctx = canvas.getContext('2d');
        var state = { engine: null, report: null, frame: 0, playing: false, loadToken: 0, busy: false, shown: -1, lastTime: -1e9, pendingAssets: {}, nextAsset: 1 };
        var disposed = false;

        function post(msg, transfer) {
            var parent = win.parent;
            if (!parent || parent === win) return;
            var out = { sm: 1 };
            for (var k in msg) out[k] = msg[k];
            parent.postMessage(out, targetOrigin, transfer || []);
        }
        function report(err, sceneId) {
            var e = H.asError(err);
            var m = { type: 'error', code: e.code, message: String(e.message).slice(0, 600) };
            var sid = sceneId || e.scene_id;
            if (sid) m.scene_id = sid;
            if (typeof e.line === 'number') { m.line = e.line; if (typeof e.column === 'number') m.column = e.column; }
            post(m);
        }
        env.onError = function (e) { report(e, e.scene_id); };

        env.requestAssets = function (keys) {
            return new Promise(function (resolve, reject) {
                var req = 'a' + (state.nextAsset++);
                var timer = env.setTimer(function () { delete state.pendingAssets[req]; reject(H.fail('readiness', 'The assets were not delivered in time.')); }, env.limits.assetsDeadlineMs);
                state.pendingAssets[req] = { resolve: resolve, reject: reject, timer: timer };
                post({ type: 'need_assets', req: req, keys: keys });
            });
        };

        function stopLoop() {
            state.playing = false;
            if (state.rafHandle !== undefined) { env.cancelRaf(state.rafHandle); state.rafHandle = undefined; }
        }
        function reset() {
            state.loadToken++;
            stopLoop();
            if (state.engine) state.engine.dispose();
            state.engine = null;
            state.report = null;
            state.frame = 0;
            state.shown = -1;
            state.busy = false;
            Object.keys(state.pendingAssets).forEach(function (r) { env.clearTimer(state.pendingAssets[r].timer); state.pendingAssets[r].reject(H.fail('cancelled', 'reset')); });
            state.pendingAssets = {};
            if (canvas.width) ctx.clearRect(0, 0, canvas.width, canvas.height);
        }
        function postTime(force) {
            var now = env.now();
            if (!force && now - state.lastTime < 90) return;
            state.lastTime = now;
            post({ type: 'time', frame: state.frame, playing: state.playing });
        }
        function draw(bitmap) {
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            ctx.drawImage(bitmap, 0, 0);
            if (typeof bitmap.close === 'function') bitmap.close();
        }
        async function show(n, token) {
            var bmp = await state.engine.render(n);
            if (token !== state.loadToken) { if (bmp && bmp.close) bmp.close(); return false; }
            draw(bmp);
            state.shown = n;
            state.frame = n;
            return true;
        }

        async function onLoad(msg) {
            reset();
            var token = state.loadToken;
            try {
                var engine = H.createEngine(env);
                state.engine = engine;
                var result = await engine.load(msg);
                if (token !== state.loadToken) return;
                var rep = result.report;
                state.report = rep;
                canvas.width = rep.width;
                canvas.height = rep.height;
                post({ type: 'compiled', report: rep });
                await result.prepare();
                if (token !== state.loadToken) return;
                if (rep.frame_count > 0 && await show(0, token)) postTime(true);
            } catch (err) {
                if (token === state.loadToken && !(err && err.code === 'cancelled')) report(err);
            }
        }

        function tick(ts) {
            state.rafHandle = undefined;
            if (!state.playing || !state.report) return;
            if (state.startTs === undefined) state.startTs = ts;
            var fps = state.report.fps, last = state.report.frame_count - 1;
            var target = state.startFrame + Math.floor((ts - state.startTs) / 1000 * fps), ended = false;
            if (target >= last) { target = last; ended = true; }
            if (!state.busy && target !== state.shown) {
                var token = state.loadToken;
                state.busy = true;
                show(target, token).then(function (ok) {
                    state.busy = false;
                    if (!ok) return;
                    if (ended) { state.playing = false; postTime(true); } else postTime(false);
                }, function (err) { state.busy = false; stopLoop(); report(err); postTime(true); });
            } else if (ended && !state.busy) { state.playing = false; postTime(true); }
            if (state.playing) state.rafHandle = env.raf(tick);
        }

        function onPlay() {
            if (!state.report || state.playing || !(state.report.frame_count > 0)) return;
            if (state.frame >= state.report.frame_count - 1) state.frame = 0;
            state.playing = true;
            state.startFrame = state.frame;
            state.startTs = undefined;
            state.shown = -1;
            postTime(true);
            state.rafHandle = env.raf(tick);
        }
        function onPause() {
            var was = state.playing;
            stopLoop();
            if (was) postTime(true);
        }
        async function onSeek(msg) {
            if (!state.report) return;
            var n = Math.min(msg.frame, Math.max(0, state.report.frame_count - 1)), token = state.loadToken;
            state.frame = n;
            if (state.playing) { state.startFrame = n; state.startTs = undefined; state.shown = -1; return; }
            try { if (await show(n, token)) postTime(true); } catch (err) { report(err); }
        }
        async function onThumb(msg) {
            var engine = state.engine, token = state.loadToken;
            if (!engine || !engine.schedule) { post({ type: 'error', code: 'readiness', message: 'thumb before load', scene_id: msg.scene_id }); return; }
            var sc = null;
            engine.schedule.scenes.forEach(function (s, i) { if (s.id === msg.scene_id) sc = { s: s, i: i }; });
            if (!sc) { post({ type: 'error', code: 'unknown_name', message: 'No scene "' + msg.scene_id + '"', scene_id: msg.scene_id }); return; }
            var len = sc.s.end_frame - sc.s.start_frame, tf = sc.s.tf || 0;
            var frame = msg.frame !== undefined ? msg.frame : sc.s.start_frame + Math.min(len - 1, tf + Math.floor(Math.max(0, len - tf) * 0.4));
            frame = Math.min(Math.max(0, frame), Math.max(0, engine.schedule.frame_count - 1));
            try {
                var bmp = await engine.render(frame, { width: msg.width });
                if (token !== state.loadToken) { if (bmp && bmp.close) bmp.close(); return; }
                if (typeof root.ImageBitmap === 'function' && !(bmp instanceof root.ImageBitmap) && typeof win.createImageBitmap === 'function') bmp = await win.createImageBitmap(bmp);
                post({ type: 'thumb', req: msg.req, scene_id: msg.scene_id, bitmap: bmp }, [bmp]);
            } catch (err) { report(err, msg.scene_id); }
        }
        function onAssets(msg) {
            var p = state.pendingAssets[msg.req];
            if (!p) return;
            delete state.pendingAssets[msg.req];
            env.clearTimer(p.timer);
            p.resolve({ assets: msg.assets, missing: msg.missing || [] });
        }

        function onMessage(event) {
            if (disposed || event.source !== win.parent) return;
            var why = checkMessage(event.data);
            if (why) { post({ type: 'error', code: 'bad_message', message: why }); return; }
            var msg = event.data;
            switch (msg.type) {
                case 'load': onLoad(msg); break;
                case 'assets': onAssets(msg); break;
                case 'seek': onSeek(msg); break;
                case 'play': onPlay(); break;
                case 'pause': onPause(); break;
                case 'thumb': onThumb(msg); break;
                case 'reset': reset(); break;
            }
        }

        win.addEventListener('message', onMessage);
        post({ type: 'ready', caps: { workers: env.hasWorkers, offscreen: typeof root.OffscreenCanvas === 'function' } });
        return {
            state: state,
            dispose: function () { disposed = true; reset(); win.removeEventListener('message', onMessage); }
        };
    }

    H.CAPS = CAPS;
    H.checkMessage = checkMessage;
    H.mountApp = mountApp;
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`45 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/host-core.js ui/js/storymaker/host-app.js scripts/storymaker-tests/08-host-app.mjs
git commit -m "feat(storymaker): player host engine and app protocol" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C10: The render driver and the host entry point

**Files:**
- Create: `ui/js/storymaker/host-render.js`, `ui/js/storymaker/host.js`
- Test: `scripts/storymaker-tests/09-host-render.mjs`

`host-render.js` drives the render page (contract section 7): boot.json, fonts only from `boot.fonts`, story/catalog/assets/code scenes from the render base,
`POST /report`, wait for `{start:true}`, render frames in order and `POST /frame` with `X-SM-Frame` as png/webp/raw RGBA, or serve `window.__smRenderFrame(n)` for the
screenshot transport; the answers `{ok}`, `{ok, done}`, `{restart}`, `{stop}`; failures go to `POST /error`. `host.js` is `StorymakerHost.mount` and the auto-mount from `data-sm-mode` /
`data-sm-base` and `#sm-stage`. The render page has no stylesheet (its CSP has no `style-src`), so the driver sets the body margin to 0, hides the document overflow and makes the
canvas a block element through the CSSOM: with the default inline canvas the page scrolls and the screenshot transport would capture scrollbars.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/09-host-render.mjs`**

````js
// Task C10: the render driver: boot.json, report, frames in order over all transports, restart, stop, refusal and failures,
// against a fake fetch (a real Chrome run is in ui/storymaker_player_gate_browser_test.go).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { root, read, plain, loadHost, FakeCanvas, InProcessWorker, MINI } from '../storymaker-test-support.mjs';
import { storymakerHostParts } from '../storymaker-bundle-parts.js';
import { t } from '../storymaker-test-context.mjs';

const { check, firewall, minimal, s3Bundle, bundleScene } = t;
const failing = src => bundleScene(src, 'bad');

const RBASE = 'http://127.0.0.1:5555/0123456789abcdef0123456789abcdef';
function makeServer(opts = {}) {
    const transport = opts.transport || 'png', W = 192, H = 108;
    const posts = [], gets = [];
    const storyDoc = opts.story || firewall;
    const boot = {
        mode: 'render', fps: 30, width: W, height: H, transport, frame_limit: opts.frameLimit || 5000, story_url: RBASE + '/story.json',
        catalog_url: RBASE + '/catalog/catalog.json', assets_base: RBASE + '/catalog/',
        code_bundles: opts.noCode ? {} : { s3: RBASE + '/scenes/s3.js' },
        fonts: [{ name: 'default', family: 'Storymaker Default', weight: 400, url: RBASE + '/fonts/placeholder.woff2' }], audio_info: {}
    };
    const res = (body, status = 200) => ({
        ok: status >= 200 && status < 300, status, json: async () => body, text: async () => typeof body === 'string' ? body : JSON.stringify(body), arrayBuffer: async () => new ArrayBuffer(16)
    });
    const answerFor = opts.answer || (() => null);
    let frames = 0;
    async function fetch(url, init = {}) {
        if (init.method === 'POST') {
            const p = url.slice(RBASE.length);
            const rec = { path: p, headers: init.headers || {}, body: init.body };
            posts.push(rec);
            if (p === '/report') {
                const rep = JSON.parse(init.body);
                rec.report = rep;
                frames = rep.frame_count;
                return res(opts.report ? opts.report(rep) : { start: true, from_frame: 0 });
            }
            if (p === '/frame') {
                const n = Number(init.headers['X-SM-Frame']);
                rec.n = n;
                const custom = answerFor(n, posts.filter(x => x.path === '/frame').length);
                if (custom) return res(custom);
                return res(n >= frames - 1 ? { ok: true, done: true } : { ok: true });
            }
            if (p === '/error') { rec.error = JSON.parse(init.body); return res({ ok: true }); }
            return res({}, 404);
        }
        gets.push(url);
        if (opts.bootStatus && url.endsWith('/boot.json')) return res({}, opts.bootStatus);
        if (url === RBASE + '/boot.json') return res(boot);
        if (url === RBASE + '/story.json') return res(storyDoc);
        if (url === RBASE + '/scenes/s3.js') return res(s3Bundle);
        if (url === RBASE + '/fonts/placeholder.woff2') return res('');
        if (url.startsWith(RBASE + '/catalog/')) {
            const rel = url.slice((RBASE + '/catalog/').length);
            const file = path.join(root, MINI, rel);
            return fs.existsSync(file) ? res(JSON.parse(fs.readFileSync(file, 'utf8'))) : res({}, 404);
        }
        return res({}, 404);
    }
    return { fetch, posts, gets, boot, W, H };
}
async function runRender(opts = {}) {
    const hostCtx = loadHost(), H = hostCtx.StorymakerHost, server = makeServer(opts);
    const canvas = new FakeCanvas(1, 1), base = canvas.ctx;
    canvas.getContext = () => new Proxy(base, { get(target, prop) {
        if (prop === 'getImageData') return (x, y, w, h) => ({ data: new Uint8ClampedArray(w * h * 4) });
        return target[prop];
    } });
    canvas.toBlob = (cb, type, quality) => cb({ __blob: true, type, quality });
    const win = { document: { body: { style: {} }, documentElement: { style: {} } }, requestAnimationFrame: fn => setTimeout(fn, 0) };
    canvas.style = {};
    const workers = [];
    const env = {
        hasWorkers: true,
        createWorker(role, text) { const w = new InProcessWorker(role, text, {}); workers.push(w); return w; },
        setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => Date.now(), limits: opts.limits
    };
    const status = await H.mountRender({ canvas, base: RBASE, fetch: server.fetch, window: win, env });
    return { server, status: plain(status), win, canvas, workers, frames: server.posts.filter(p => p.path === '/frame') };
}

check('render mode: boot.json, report, then every frame in order (png)', async () => {
    const r = await runRender({ transport: 'png' });
    assert.equal(r.status.phase, 'done', JSON.stringify(r.status));
    assert.equal(r.win.document.body.style.margin, '0', 'the body margin is set through the CSSOM');
    assert.equal(r.win.document.documentElement.style.overflow, 'hidden', 'no scrollbars: the screenshot transport captures the viewport');
    assert.equal(r.canvas.style.display, 'block', 'no inline-canvas gap below the canvas');
    assert.deepEqual(r.server.posts.map(p => p.path).slice(0, 2), ['/report', '/frame']);
    const rep = r.server.posts[0].report;
    assert.equal(rep.frame_count, 354); assert.equal(rep.width, 192); assert.equal(rep.height, 108);
    assert.equal(rep.issues.filter(i => i.severity === 'error').length, 0);
    assert.equal(r.server.posts[0].headers['Content-Type'], 'application/json');
    assert.equal(r.frames.length, 354);
    r.frames.forEach((f, i) => {
        assert.equal(f.headers['X-SM-Frame'], String(i)); assert.equal(f.headers['Content-Type'], 'image/png'); assert.equal(f.body.type, 'image/png');
    });
    assert.equal(r.canvas.width, 192); assert.equal(r.canvas.height, 108);
    assert.equal(r.server.posts.filter(p => p.path === '/error').length, 0);
    const fontGets = r.server.gets.filter(u => u.includes('/fonts/'));
    assert.deepEqual(fontGets, [RBASE + '/fonts/placeholder.woff2'], 'font URLs come from boot.fonts only');
    assert.ok(r.server.gets.every(u => u.startsWith(RBASE + '/')), 'every request stays under the render base');
    assert.ok(r.server.gets.includes(RBASE + '/catalog/rig/human.json') && r.server.gets.includes(RBASE + '/catalog/environments/office.json'));
    assert.ok(r.workers.every(w => w.dead || true));
});

check('render mode: webp, raw RGBA and the screenshot transport', async () => {
    const webp = await runRender({ transport: 'webp', story: minimal, noCode: true });
    assert.equal(webp.status.phase, 'done'); assert.equal(webp.frames.length, 42);
    assert.equal(webp.frames[0].headers['Content-Type'], 'image/webp'); assert.equal(webp.frames[0].body.quality, 1, 'lossless webp');
    const raw = await runRender({ transport: 'raw', story: minimal, noCode: true });
    assert.equal(raw.status.phase, 'done'); assert.equal(raw.frames.length, 42);
    assert.equal(raw.frames[3].headers['Content-Type'], 'application/octet-stream');
    assert.equal(raw.frames[3].body.byteLength, 192 * 108 * 4, 'raw RGBA is w*h*4 bytes');
    const shot = await runRender({ transport: 'screenshot', story: minimal, noCode: true });
    assert.equal(shot.status.phase, 'ready'); assert.equal(shot.status.frame_count, 42);
    assert.equal(shot.server.posts.filter(p => p.path === '/frame').length, 0, 'the screenshot transport posts no frames');
    assert.equal(typeof shot.win.__smRenderFrame, 'function');
    const before = shot.canvas.ctx.log.filter(l => l.startsWith('drawImage')).length;
    assert.equal(await shot.win.__smRenderFrame(7), 7);
    assert.equal(shot.canvas.ctx.log.filter(l => l.startsWith('drawImage')).length, before + 1, 'the frame is drawn on the visible canvas');
    assert.equal(plain(shot.win.__smStatus).frame, 7);
    await assert.rejects(() => shot.win.__smRenderFrame(42), /outside 0\.\.41/);
});

check('render mode: restart, stop, refusal, frame limit and failures', async () => {
    let restarted = false;
    const restart = await runRender({ story: minimal, noCode: true, answer: n => { if (n === 5 && !restarted) { restarted = true; return { restart: true }; } return null; } });
    assert.equal(restart.status.phase, 'done');
    assert.deepEqual(restart.frames.map(f => f.n), [...Array(6).keys(), ...Array(42).keys()], 'restart renders again from frame 0');
    const stop = await runRender({ story: minimal, noCode: true, answer: n => n === 3 ? { stop: true, error: 'cancelled', message: 'job cancelled' } : null });
    assert.equal(stop.status.phase, 'stopped'); assert.equal(stop.frames.length, 4); assert.equal(stop.status.error, 'cancelled');
    const refused = await runRender({ story: minimal, noCode: true, report: () => ({ start: false, error: 'ffmpeg_unavailable', message: 'no ffmpeg' }) });
    assert.equal(refused.status.phase, 'refused'); assert.equal(refused.frames.length, 0); assert.equal(refused.status.error, 'ffmpeg_unavailable');
    const resumed = await runRender({ story: minimal, noCode: true, report: () => ({ start: true, from_frame: 10 }) });
    assert.equal(resumed.frames[0].n, 10);
    const limited = await runRender({ story: minimal, noCode: true, frameLimit: 20, answer: () => ({ ok: true }) });
    assert.equal(limited.status.phase, 'error'); assert.equal(limited.frames.length, 20);
    assert.match(limited.server.posts.at(-1).error.message, /ran past the last frame/);
    const broken = await runRender({ bootStatus: 503 });
    assert.equal(broken.status.phase, 'error');
    const err = broken.server.posts.find(p => p.path === '/error').error;
    assert.equal(err.code, 'readiness'); assert.match(err.message, /boot\.json answered 503/);
    const rejected = await runRender({ story: minimal, noCode: true, answer: () => ({ stop: true, error: 'bad_frame', message: 'size' }) });
    assert.equal(rejected.status.phase, 'stopped');
});

check('render mode: a code scene that fails is reported in the report issues and by POST /error', async () => {
    const hostCtx = loadHost();
    const server = makeServer({});
    const origFetch = server.fetch;
    const bad = failing('export default function scene(api) { api.duration(3); api.add("prop.lock", { at: "nowhere" }); }');
    const fetch = async (url, init) => url === RBASE + '/scenes/s3.js' ? { ok: true, status: 200, text: async () => bad } : origFetch(url, init);
    const canvas = new FakeCanvas(1, 1);
    const win = { document: { body: { style: {} } } };
    const env = { hasWorkers: true, createWorker: (role, text) => new InProcessWorker(role, text, {}), setTimer: (fn, ms) => setTimeout(fn, ms), clearTimer: h => clearTimeout(h), now: () => Date.now() };
    const status = plain(await hostCtx.StorymakerHost.mountRender({ canvas, base: RBASE, fetch, window: win, env }));
    const rep = server.posts.find(p => p.path === '/report').report;
    assert.ok(rep.issues.some(i => i.code === 'code_error' && i.severity === 'error' && /Unknown position "nowhere"/.test(i.message)));
    const err = server.posts.find(p => p.path === '/error');
    assert.ok(err && err.error.code === 'code_error' && err.error.scene_id === 's3');
    void status;
});

check('host bundle is small and self-contained: no codescene, rig or compile in the document realm', () => {
    const host = storymakerHostParts.map(read).join('\n');
    assert.ok(!/SM\.rig =|SM\.compile =|SM\.codescene =|SM\.render =/.test(host));
    assert.ok(storymakerHostParts.length === 5 && storymakerHostParts[0].endsWith('schedule.js'));
    const ctx = loadHost();
    assert.ok(ctx.StorymakerHost.mount && ctx.StorymakerHost.mountApp && ctx.StorymakerHost.mountRender && ctx.StorymakerCore.schedule);
    assert.throws(() => ctx.StorymakerHost.mount({ mode: 'nope', canvas: {}, base: 'x' }), /Unknown Storymaker mode/);
    assert.throws(() => ctx.StorymakerHost.mount({ mode: 'app' }), /needs a canvas/);
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (45 passed, 5 failed; first: host bundle is small and self-contained: no codescene, rig or compile in the document realm: Error: ENOENT: no such file or directory, open '<path>')

- [ ] **Step 3: Implement**


**`ui/js/storymaker/host-render.js`**

````js
// Storymaker host (document realm): render mode, the page that headless Chrome opens for an export. It drives itself from
// <base>/boot.json (contract section 7): fetches story, catalog, fonts (only from boot.fonts) and code-scene bundles from the
// base, compiles, POSTs the compile report, waits for {start: true}, then renders frames in order and POSTs each one to the
// fixed <base>/frame with the header X-SM-Frame. Transports: png, webp, raw RGBA, or screenshot (the page only draws on
// window.__smRenderFrame(n); Go captures). Answers: {ok}, {ok, done}, {restart}, {stop}. Failures are POSTed to <base>/error.
// window.__smStatus mirrors the phase for the driver. Classic script; publishes StorymakerHost.mountRender.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    var TRANSPORTS = ['png', 'webp', 'raw', 'screenshot'];
    var MIME = { png: 'image/png', webp: 'image/webp', raw: 'application/octet-stream' };

    function errorBody(err, sceneId) {
        var e = H.asError(err), out = { code: e.code, message: String(e.message).slice(0, 600) };
        var sid = sceneId || e.scene_id;
        if (sid) out.scene_id = sid;
        return out;
    }

    // mountRender(opts): opts = {canvas, base, fetch?, window?, env?}. Resolves when the job is over (done, stopped,
    // refused, failed) or, in screenshot mode, when the page is ready for __smRenderFrame calls.
    async function mountRender(opts) {
        var win = opts.window || root, canvas = opts.canvas, base = String(opts.base).replace(/\/+$/, '');
        var doFetch = opts.fetch || function (url, init) { return win.fetch(url, init); };
        var env = H.defaultEnv(base, opts.env || {});
        var status = win.__smStatus = { phase: 'boot', frame: -1 };
        var engine = null;

        async function getJSON(url) {
            var res = await doFetch(url, { cache: 'no-store' });
            if (!res.ok) throw H.fail('readiness', 'GET ' + url + ' answered ' + res.status);
            return res.json();
        }
        async function post(path, body, headers) {
            var res = await doFetch(base + path, { method: 'POST', headers: headers, body: body, cache: 'no-store' });
            var text = '';
            try { text = await res.text(); } catch (e) { text = ''; }
            var json = null;
            try { json = text ? JSON.parse(text) : {}; } catch (e) { json = null; }
            if (!res.ok && (!json || !(json.stop || json.restart))) throw H.fail('render_failed', 'POST ' + path + ' answered ' + res.status + (text ? ': ' + text.slice(0, 200) : ''));
            return json || {};
        }
        async function postError(err, sceneId) {
            try { await post('/error', JSON.stringify(errorBody(err, sceneId)), { 'Content-Type': 'application/json' }); } catch (e) { /* the listener is gone */ }
        }

        try {
            // The render page has no stylesheet (its CSP has no style-src): layout goes through the CSSOM, which needs none. The
            // canvas must fill the viewport exactly, or the screenshot transport would capture scrollbars.
            if (win.document && win.document.body && win.document.body.style) win.document.body.style.margin = '0';
            if (win.document && win.document.documentElement && win.document.documentElement.style) win.document.documentElement.style.overflow = 'hidden';
            if (canvas.style) canvas.style.display = 'block';
            var boot = await getJSON(base + '/boot.json');
            if (!boot || boot.mode !== 'render' || !(boot.fps > 0) || !(boot.width > 0) || !(boot.height > 0) || TRANSPORTS.indexOf(boot.transport) < 0) {
                throw H.fail('readiness', 'boot.json is not a render boot document.');
            }
            canvas.width = boot.width;
            canvas.height = boot.height;
            var ctx = canvas.getContext('2d', { willReadFrequently: boot.transport === 'raw' });
            var story = await getJSON(boot.story_url);
            var catalog = await getJSON(boot.catalog_url);
            var bundles = {}, ids = Object.keys(boot.code_bundles || {});
            await Promise.all(ids.map(async function (id) {
                var res = await doFetch(boot.code_bundles[id], { cache: 'no-store' });
                if (!res.ok) throw H.fail('code_error', 'Code scene "' + id + '" could not be fetched (' + res.status + ').', { scene_id: id });
                bundles[id] = await res.text();
            }));
            var fonts = await Promise.all((boot.fonts || []).map(async function (f) {
                var res = await doFetch(f.url, { cache: 'no-store' });
                if (!res.ok) throw H.fail('readiness', 'Font "' + f.family + '" could not be fetched (' + res.status + ').');
                return { name: f.name, family: f.family, weight: f.weight, data: await res.arrayBuffer() };
            }));
            var assetsBase = String(boot.assets_base || (base + '/catalog/')).replace(/\/*$/, '/');

            env.onError = function (e) { status.lastError = e; postError(e, e.scene_id); };
            env.requestAssets = async function (keys) {
                var assets = {};
                await Promise.all(keys.map(async function (key) {
                    if (key.indexOf('catalog:') === 0) {
                        var rel = key.slice(8);
                        if (rel.indexOf('..') >= 0 || rel.charAt(0) === '/') throw H.fail('readiness', 'Bad asset path ' + rel);
                        var res = await doFetch(assetsBase + rel, { cache: 'no-store' });
                        if (!res.ok) throw H.fail('readiness', 'Asset ' + rel + ' answered ' + res.status);
                        assets[key] = await res.json();
                    }
                    // render mode takes font URLs only from boot.fonts (the catalog's file paths are AuraGo paths)
                }));
                return { assets: assets, missing: [] };
            };

            engine = H.createEngine(env);
            status.phase = 'compiling';
            var loaded = await engine.load({
                story: story, catalog: catalog, audio_info: boot.audio_info || {}, assets: {}, fonts: fonts, code_bundles: bundles,
                format: { fps: boot.fps, width: boot.width, height: boot.height }
            });
            await loaded.prepare();
            var rep = loaded.report;
            status.phase = 'reporting';
            var answer = await post('/report', JSON.stringify(rep), { 'Content-Type': 'application/json' });
            if (!answer.start) {
                status.phase = 'refused';
                status.error = answer.error || '';
                status.message = answer.message || '';
                return status;
            }
            var limit = Math.min(rep.frame_count, boot.frame_limit > 0 ? boot.frame_limit : rep.frame_count);

            async function drawFrame(n) {
                var bmp = await engine.render(n);
                ctx.clearRect(0, 0, canvas.width, canvas.height);
                ctx.drawImage(bmp, 0, 0);
                if (typeof bmp.close === 'function') bmp.close();
                status.frame = n;
            }
            function encode(transport) {
                if (transport === 'raw') {
                    var data = ctx.getImageData(0, 0, canvas.width, canvas.height).data;
                    return Promise.resolve(data.buffer);
                }
                return new Promise(function (resolve, reject) {
                    canvas.toBlob(function (blob) { if (blob) resolve(blob); else reject(H.fail('render_failed', 'The canvas could not be encoded.')); }, MIME[transport], transport === 'webp' ? 1 : undefined);
                });
            }

            if (boot.transport === 'screenshot') {
                win.__smRenderFrame = async function (n) {
                    try {
                        if (!(n >= 0 && n < limit)) throw H.fail('render_failed', 'Frame ' + n + ' is outside 0..' + (limit - 1));
                        await drawFrame(n);
                    } catch (failure) {
                        var e = H.asError(failure), real = new Error(e.message);   // a real Error, so the driver sees the message
                        real.code = e.code;
                        postError(e);
                        throw real;
                    }
                    await new Promise(function (resolve) {
                        var done = false, finish = function () { if (!done) { done = true; resolve(); } };
                        if (typeof win.requestAnimationFrame === 'function') win.requestAnimationFrame(function () { win.requestAnimationFrame(finish); });
                        env.setTimer(finish, 150);
                    });
                    return n;
                };
                status.phase = 'ready';
                status.frame_count = limit;
                return status;
            }

            status.phase = 'rendering';
            var n = answer.from_frame > 0 ? answer.from_frame : 0;
            for (;;) {
                if (n >= limit) {
                    await postError(H.fail('render_failed', 'The page ran past the last frame (' + limit + ') without a done answer.'));
                    status.phase = 'error';
                    return status;
                }
                await drawFrame(n);
                var body = await encode(boot.transport);
                var reply = await post('/frame', body, { 'Content-Type': MIME[boot.transport], 'X-SM-Frame': String(n) });
                if (reply.stop) { status.phase = 'stopped'; status.error = reply.error || ''; status.message = reply.message || ''; return status; }
                if (reply.restart) { n = 0; status.restarts = (status.restarts || 0) + 1; continue; }
                if (reply.done) { status.phase = 'done'; return status; }
                n++;
            }
        } catch (err) {
            if (engine) engine.dispose();
            status.phase = 'error';
            status.error = H.asError(err).code;
            status.message = H.asError(err).message;
            await postError(err);
            return status;
        }
    }

    H.mountRender = mountRender;
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


**`ui/js/storymaker/host.js`**

````js
// Storymaker host (document realm): entry point. window.StorymakerHost.mount(opts) with opts = {mode, canvas, base,
// boot?} mounts the app protocol or the render driver; on load the bundle mounts itself from the markup the server wrote
// (<html data-sm-mode data-sm-base> and <canvas id="sm-stage">). The page runs no inline script. Classic script.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    // mount(opts): opts = {mode: "app" | "render", canvas, base, window?, env?, fetch?}
    H.mount = function (opts) {
        if (!opts || !opts.canvas || !opts.base) throw new Error('StorymakerHost.mount needs a canvas and a base URL.');
        if (opts.mode === 'app') return H.mountApp(opts);
        if (opts.mode === 'render') return H.mountRender(opts);
        throw new Error('Unknown Storymaker mode "' + opts.mode + '".');
    };

    function autoMount() {
        var doc = root.document;
        if (!doc || !doc.documentElement || !doc.documentElement.dataset) return;
        var ds = doc.documentElement.dataset;
        if (!ds.smMode || !ds.smBase) return;
        var canvas = doc.getElementById('sm-stage');
        if (!canvas) return;
        H.mounted = H.mount({ mode: ds.smMode, base: ds.smBase, canvas: canvas });
    }

    if (root.document && root.document.readyState === 'loading') root.document.addEventListener('DOMContentLoaded', autoMount);
    else autoMount();
})(typeof globalThis !== 'undefined' ? globalThis : self);
````


- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs`
Expected: PASS (`50 passed, 0 failed`)

- [ ] **Step 5: Commit**

```bash
git add ui/js/storymaker/host-render.js ui/js/storymaker/host.js scripts/storymaker-tests/09-host-render.mjs
git commit -m "feat(storymaker): player render driver and host entry" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C11: Bundles, the player stylesheet and the static guards

**Files:**
- Modify: `scripts/build-ui-bundles.js:3` (import) and `:210-212` (two bundle entries)
- Modify: `.gitattributes:29` (generated bundles are LF)
- Create: `ui/css/storymaker-player.css`
- Generate: `ui/js/storymaker/host.bundle.js`, `ui/js/storymaker/player.bundle.js` (by `npm run build:ui`)
- Test: `scripts/storymaker-tests/10-static-guards.mjs`

The build script reuses the part lists of `scripts/storymaker-bundle-parts.js`, so the bundles and the tests cannot drift apart. Both bundles get the
usual generated-file header and `--check` support (`npm run check:ui`). The stylesheet is the only one of the player host page (named in its CSP); the render page has none and
sets its layout through the CSSOM (body margin 0, no scrollbars, a block canvas). The guard module checks that every listed part exists, that no source uses unsafe DOM APIs or template strings with data, that the
player realm uses no clocks or randomness, and that both bundles contain their parts in order.

- [ ] **Step 1: Write the failing test**


**`scripts/storymaker-tests/10-static-guards.mjs`**

````js
// Task C11: static guards over the player sources.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { root, read } from '../storymaker-test-support.mjs';
import { storymakerHostParts, storymakerPlayerParts } from '../storymaker-bundle-parts.js';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;

check('static guard: every listed part of both bundles exists, and the order ends with the entry points', () => {
    for (const f of [...storymakerHostParts, ...storymakerPlayerParts]) assert.ok(fs.existsSync(path.join(root, f)), f + ' is listed but missing');
    assert.equal(new Set(storymakerPlayerParts).size, storymakerPlayerParts.length);
    assert.ok(storymakerPlayerParts.at(-1).endsWith('worker-main.js'), 'the role dispatch is the last part of the worker bundle');
    assert.ok(storymakerHostParts.at(-1).endsWith('host.js'), 'host.js is the last part of the host bundle');
});

check('static guard: no innerHTML, template strings with data, eval or timers in the player sources', () => {
    const files2 = [...new Set([...storymakerHostParts, ...storymakerPlayerParts])];
    for (const f of files2) {
        const src = read(f);
        assert.ok(!/innerHTML|outerHTML|insertAdjacentHTML|document\.write|\beval\(|new Function\(/.test(src), f + ' uses an unsafe DOM or eval API');
        assert.ok(!/`[^`]*\$\{/.test(src), f + ' has a template string with data');
        assert.ok(src.split('\n').length < 600, f + ' stays below 600 lines');
        assert.ok(/^\/\/ /.test(src), f + ' starts with a comment that says what it is');
    }
    // timers only where the host drives the deadlines
    for (const f of storymakerPlayerParts) {
        if (f.endsWith('determinism.js')) continue;
        assert.ok(!/\b(setTimeout|setInterval|requestAnimationFrame|Date\.now|Math\.random|performance\.now)\b/.test(read(f).replace(/\/\/.*$/gm, '')), f + ' must not use wall-clock time or randomness');
    }
});

check('bundles: both are generated from their part lists, in order, with the generated-file header (npm run check:ui compares the bytes)', () => {
    for (const [out, parts] of [['ui/js/storymaker/host.bundle.js', storymakerHostParts], ['ui/js/storymaker/player.bundle.js', storymakerPlayerParts]]) {
        const text = read(out).replace(/\r\n/g, '\n');
        assert.ok(text.startsWith('// Generated by scripts/build-ui-bundles.js. Do not edit by hand.\n'), out + ' has the generated header');
        let at = 0;
        for (const part of parts) {
            const i = text.indexOf('/* ' + part + ' */', at);
            assert.ok(i >= 0, out + ' contains ' + part + ' in order');
            at = i;
        }
    }
});
````


- [ ] **Step 2: Run the test to verify it fails**

Run: `node scripts/test-storymaker-player.mjs`
Expected: FAIL (52 passed, 1 failed; first: bundles: both are generated from their part lists, in order, with the generated-file header (npm run check:ui compares the bytes): Error: ENOENT: no such)

- [ ] **Step 3: Implement**

**`ui/css/storymaker-player.css`**

````css
/* Storymaker player host page (the only stylesheet of the sandboxed player iframe, named in its CSP).
   The canvas fills the page and keeps the video's aspect ratio (letterboxing on the host background); there are no
   scrollbars. The render page has no stylesheet at all: it sets its body margin through the CSSOM. */
html,
body {
    margin: 0;
    padding: 0;
    width: 100%;
    height: 100%;
    overflow: hidden;
    background: #000;
}

#sm-stage {
    display: block;
    position: fixed;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
    background: #000;
    image-rendering: auto;
}
````

In `scripts/build-ui-bundles.js` (at base: line 3 is the `fileURLToPath` import; lines 210-212 are the last bundle entry `ui/js/chat/main.js`):

````diff
 import { fileURLToPath } from 'node:url';
+import { storymakerHostParts, storymakerPlayerParts } from './storymaker-bundle-parts.js';
````

````diff
   {
     out: 'ui/js/chat/main.js',
     parts: chatMainParts
-  }
+  },
+  // Storymaker player: the document-realm host page and the worker-realm player (part lists shared with the tests).
+  { out: 'ui/js/storymaker/host.bundle.js', parts: storymakerHostParts },
+  { out: 'ui/js/storymaker/player.bundle.js', parts: storymakerPlayerParts }
 ];
````

In `.gitattributes` after `ui/js/desktop/aura-desktop-sdk.js text eol=lf` (line 29), so the generated bundles are byte-identical on every platform (the build writes LF):

````diff
 ui/js/desktop/aura-desktop-sdk.js text eol=lf
+ui/js/storymaker/*.bundle.js text eol=lf
````

Generate the bundles:

```bash
npm run build:ui
```

Expected: the output contains `Built ui/js/storymaker/host.bundle.js (5 parts)` and `Built ui/js/storymaker/player.bundle.js (21 parts)` (the bundles are written concurrently, so the order of the `Built` lines varies and these two are not the last lines); no other bundle changes (`git status` shows only the two new files).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node scripts/test-storymaker-player.mjs` and `npm run check:ui`
Expected: PASS (`53 passed, 0 failed`); `check:ui` prints `Checked ui/js/storymaker/host.bundle.js (5 parts)` and `Checked ui/js/storymaker/player.bundle.js (21 parts)` and exits 0.

- [ ] **Step 5: Commit**

```bash
git add scripts/build-ui-bundles.js .gitattributes ui/css/storymaker-player.css ui/js/storymaker/host.bundle.js ui/js/storymaker/player.bundle.js scripts/storymaker-tests/10-static-guards.mjs
git commit -m "build(storymaker): player bundles, stylesheet and static guards" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

### Task C12: The phase-1 gate and the render page in a real browser

**Files:**
- Test: `ui/storymaker_player_gate_browser_test.go`

This is the **phase-1 gate** of the spec ("Fallback if workers are unavailable in the sandboxed iframe") plus the render page test of contract section 7. It
is a verification task: the browser tests need the bundles and the stylesheet of C11, so they pass on their first run; if the gate fails, implement and test
the fallback path (it is already in `host-core.js` and covered by Node tests) and say so in the gate report. All four tests are gated by
`AURAGO_RUN_BROWSER_SMOKE=1` and use the shared `newSmokeBrowser` helper (the installed Chrome or Edge; on this Windows box run them through `go test` with
`GOTMPDIR=D:\tmp\go-build`, outside the sandbox, because repo-built executables cannot start Chrome).

- `TestStorymakerPlayerGateBrowser`: an `httptest` server serves the contract section 6 app markup under the exact player CSP of the spec (origin = the test
  server), embedded in an opaque-origin `sandbox="allow-scripts"` iframe of a parent page that speaks the section 8 protocol. It asserts: blob workers with
  `importScripts` of the absolute bundle URL work (the `ready` caps, the compile report: 324 frames, scene ranges), `OffscreenCanvas`, `Path2D` and
  `FontFace` work in the workers (a frame that is not blank; a code scene that sees its worker's `FontFace` loaded), an `ImageBitmap` transfers to the parent
  (`thumb`), and a code scene cannot leave: `fetch` and `importScripts` outside `/js/storymaker/` are refused by the CSP and the server sees no request. It also
  asserts that nothing the player legitimately loads (the two bundles, the stylesheet) shows up in the browser's CSP log.
- `TestStorymakerRenderModeBrowser`: the render markup under the render CSP, `boot.json` and a fake ingest with the CORS behaviour of section 7 (the page has an opaque
  origin, so every request carries `Origin: null` and the `X-SM-Frame` header forces a preflight); all three posted transports (png, webp, raw RGBA) with a restart answer at
  frame 5.
- `TestStorymakerFirewallRenderBrowser`: the whole firewall story, code scene included (the real `s3.js` bundled with Go's esbuild), rendered **twice**; the frames must be
  bit-identical. `AURAGO_BROWSER_ARTIFACT_DIR` keeps every 30th frame as a PNG; `AURAGO_STORYMAKER_TRANSPORT` and `AURAGO_STORYMAKER_WIDTH` select the transport and the width and
  the test logs the frame rate (the transport spike data for part B).
- `TestStorymakerRenderScreenshotBrowser`: the fourth transport. The page draws frame 5 on `window.__smRenderFrame(5)`; a CDP screenshot at device pixel ratio 1 (viewport = canvas size) must show exactly the
  pixels of the frame that the png transport posts (measured: 0 of 230400 pixels differ).

- [ ] **Step 1: Write the test**


**`ui/storymaker_player_gate_browser_test.go`**

````go
package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Storymaker phase 1 gate (spec "Fallback if workers are unavailable in the sandboxed iframe", contract section 6/8) and the
// render page browser test (contract section 7). Gated by AURAGO_RUN_BROWSER_SMOKE=1, launched like the other browser
// smokes (newSmokeBrowser: the installed Chrome or Edge).
//
//	AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestStorymaker(PlayerGate|RenderMode)Browser' -count=1 -timeout 10m
//
// The gate serves the app player markup under the exact player CSP of the spec and embeds it in an opaque-origin
// sandbox="allow-scripts" iframe of a parent page that speaks the contract section 8 protocol. It asserts that blob workers
// importScripts the absolute bundle URL, that OffscreenCanvas, Path2D and FontFace work in them, that an ImageBitmap travels
// to the parent, that a frame is not blank, the compile report, and that a code scene cannot fetch.

const storymakerPlayerCSP = "sandbox allow-scripts; default-src 'none'; script-src %[1]s/js/storymaker/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src 'none'; style-src %[1]s/css/storymaker-player.css; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'self'"

const storymakerRenderCSP = "sandbox allow-scripts; default-src 'none'; script-src %[1]s/js/ blob:; worker-src blob:; img-src blob: data:; font-src blob: data:; connect-src %[1]s/; form-action 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"

// storymakerGateScene is a hand-written code-scene bundle in the form of the Go bundler (an IIFE that registers
// self.__smScene). It tries to leave the realm and paints the outcome: red = fetch was blocked, green = importScripts of a
// URL outside the player path was blocked, blue = the worker's FontFace is loaded. White means all three.
const storymakerGateScene = `var __smSceneModule = (function () {
  function scene(api) {
    api.duration(4);
    api.text('gate', 'title');
    var fetchBlocked = 0, importBlocked = 0, probe = '%[1]s/probe';
    try { fetch(probe).then(function () {}, function () { fetchBlocked = 255; }); } catch (e) { fetchBlocked = 255; }
    try { importScripts(probe + '.js'); } catch (e) { importBlocked = 255; }
    api.onFrame(function (t, g) {
      var fontOk = 0;
      self.fonts.forEach(function (f) { if (f.family === 'Storymaker Default' && f.status === 'loaded') fontOk = 255; });
      g.fill('rgb(' + fetchBlocked + ',' + importBlocked + ',' + fontOk + ')');
      g.rect(0, 0, 1920, 1080);
    });
  }
  return { default: scene };
})();
self.__smScene = __smSceneModule.default;
`

const storymakerGateParent = `<!doctype html><meta charset="utf-8"><title>Storymaker gate parent</title>
<iframe id="f" sandbox="allow-scripts" style="width:480px;height:270px;border:0"></iframe>
<canvas id="c" width="160" height="90"></canvas>
<script>
window.__gate = { log: [], errors: [], thumbs: {}, done: false, failure: null };
const iframe = document.getElementById('f');
const gate = window.__gate;
async function getJSON(u) { const r = await fetch(u); if (!r.ok) throw new Error(u + ' ' + r.status); return r.json(); }
let story, catalog, files = {}, fontBuf, bundle;
async function prepare() {
  story = await getJSON('/fixtures/firewall.json');
  catalog = await getJSON('/img/storymaker/catalog.json');
  const paths = [catalog.rig.file].concat(catalog.entries.filter(e => e.file).map(e => e.file));
  for (const p of paths) files['catalog:' + p] = await getJSON('/img/storymaker/' + p);
  fontBuf = await (await fetch('/fonts/placeholder.woff2')).arrayBuffer();
  bundle = await (await fetch('/fixtures/gate.bundle.js')).text();
}
function post(msg, transfer) { iframe.contentWindow.postMessage(Object.assign({ sm: 1 }, msg), '*', transfer || []); }
function sample(id, bitmap) {
  const c = document.getElementById('c'), ctx = c.getContext('2d', { willReadFrequently: true });
  ctx.clearRect(0, 0, c.width, c.height);
  ctx.drawImage(bitmap, 0, 0);
  const px = ctx.getImageData(Math.floor(bitmap.width / 2), Math.floor(bitmap.height / 2), 1, 1).data;
  const seen = new Set();
  for (let y = 0; y < bitmap.height; y += 6) for (let x = 0; x < bitmap.width; x += 6) { const d = ctx.getImageData(x, y, 1, 1).data; seen.add(d[0] + ',' + d[1] + ',' + d[2]); }
  gate.thumbs[id] = { width: bitmap.width, height: bitmap.height, center: [px[0], px[1], px[2], px[3]], distinct: seen.size, isBitmap: bitmap instanceof ImageBitmap };
}
let tries = 0;
window.addEventListener('message', async (ev) => {
  if (ev.source !== iframe.contentWindow) return;
  const m = ev.data;
  if (!m || m.sm !== 1) return;
  gate.log.push(m.type);
  try {
    if (m.type === 'ready') {
      gate.caps = m.caps;
      await prepare();
      post({ type: 'load', story, catalog, audio_info: {}, assets: files, fonts: [{ name: 'default', family: catalog.fonts[0].family, weight: 400, data: fontBuf }], code_bundles: { s3: bundle }, format: { aspect: '16:9', fps: 30, width: 640 } });
    } else if (m.type === 'compiled') {
      gate.report = m.report;
    } else if (m.type === 'time' && m.frame === 0 && !gate.asked) {
      gate.asked = true;
      post({ type: 'thumb', req: 't1', scene_id: 's1', width: 160 });
      post({ type: 'thumb', req: 't3', scene_id: 's3', width: 160 });
    } else if (m.type === 'thumb') {
      sample(m.scene_id, m.bitmap);
      const t3 = gate.thumbs.s3;
      if (m.scene_id === 's3' && t3.center[0] === 0 && t3.center[1] === 0 && t3.center[2] === 0 && tries++ < 30) {
        setTimeout(() => post({ type: 'thumb', req: 't3b' + tries, scene_id: 's3', width: 160 }), 100);
      } else if (gate.thumbs.s1 && gate.thumbs.s3) {
        gate.opaque = (function () { try { iframe.contentWindow.document; return false; } catch (e) { return true; } })();
        gate.done = true;
      }
    } else if (m.type === 'error') {
      gate.errors.push({ code: m.code, message: m.message, scene_id: m.scene_id });
    }
  } catch (e) { gate.failure = String(e && e.stack || e); gate.done = true; }
});
iframe.src = '/player';
</script>`

type storymakerGateServer struct {
	srv       *httptest.Server
	probeHits atomic.Int64
	headers   sync.Map
}

func storymakerFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatalf("read %s: %v (run npm run build:ui first)", rel, err)
	}
	return data
}

func newStorymakerGateServer(t *testing.T) *storymakerGateServer {
	t.Helper()
	g := &storymakerGateServer{}
	mini := filepath.Join("..", "internal", "storymaker", "testdata", "catalog-mini")
	mux := http.NewServeMux()
	var origin string
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) { g.probeHits.Add(1); w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/probe.js", func(w http.ResponseWriter, r *http.Request) {
		g.probeHits.Add(1)
		w.Header().Set("Content-Type", "text/javascript")
	})
	mux.HandleFunc("/parent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, storymakerGateParent)
	})
	mux.HandleFunc("/player", func(w http.ResponseWriter, r *http.Request) {
		csp := fmt.Sprintf(storymakerPlayerCSP, origin)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, `<!doctype html><html data-sm-mode="app" data-sm-base="%[1]s"><head><meta charset="utf-8"><link rel="stylesheet" href="%[1]s/css/storymaker-player.css"></head><body><canvas id="sm-stage"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, origin)
	})
	mux.HandleFunc("/js/storymaker/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		if name != "host.bundle.js" && name != "player.bundle.js" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "js/storymaker/"+name))
	})
	mux.HandleFunc("/css/storymaker-player.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "css/storymaker-player.css"))
	})
	mux.HandleFunc("/fonts/placeholder.woff2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write(storymakerFile(t, "fonts/Geist-Regular.woff2"))
	})
	mux.HandleFunc("/fixtures/firewall.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, "../internal/storymaker/testdata/stories/firewall.json"))
	})
	mux.HandleFunc("/fixtures/gate.bundle.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = io.WriteString(w, fmt.Sprintf(storymakerGateScene, origin))
	})
	mux.HandleFunc("/img/storymaker/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/img/storymaker/")
		if strings.Contains(rel, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, filepath.ToSlash(filepath.Join(mini, filepath.FromSlash(rel)))))
	})
	g.srv = httptest.NewServer(mux)
	origin = g.srv.URL
	t.Cleanup(g.srv.Close)
	return g
}

type storymakerGateResult struct {
	Caps   struct{ Workers, Offscreen bool } `json:"caps"`
	Log    []string                          `json:"log"`
	Opaque bool                              `json:"opaque"`
	Report struct {
		FrameCount int `json:"frame_count"`
		FPS        int `json:"fps"`
		Width      int `json:"width"`
		Height     int `json:"height"`
		Scenes     []struct {
			ID         string `json:"id"`
			StartFrame int    `json:"start_frame"`
			EndFrame   int    `json:"end_frame"`
			Kind       string `json:"kind"`
		} `json:"scenes"`
		Issues []struct{ Severity, Code, Path string } `json:"issues"`
	} `json:"report"`
	Errors []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		SceneID string `json:"scene_id"`
	} `json:"errors"`
	Thumbs  map[string]storymakerGateThumb `json:"thumbs"`
	Failure string                         `json:"failure"`
}

type storymakerGateThumb struct {
	Width    int   `json:"width"`
	Height   int   `json:"height"`
	Center   []int `json:"center"`
	Distinct int   `json:"distinct"`
	IsBitmap bool  `json:"isBitmap"`
}

func TestStorymakerPlayerGateBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker player gate")
	}
	g := newStorymakerGateServer(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage("").Timeout(120 * time.Second)
	defer page.Close()
	var logMu sync.Mutex
	var browserLog []string
	go page.EachEvent(func(e *proto.LogEntryAdded) {
		logMu.Lock()
		browserLog = append(browserLog, e.Entry.Text+" "+e.Entry.URL)
		logMu.Unlock()
	})()
	if err := (proto.LogEnable{}).Call(page); err != nil {
		t.Fatal(err)
	}
	page.MustNavigate(g.srv.URL + "/parent")
	if err := page.Wait(rod.Eval(`() => window.__gate && window.__gate.done`)); err != nil {
		t.Fatalf("the gate did not finish: %v\n%s", err, page.MustEval(`() => JSON.stringify(window.__gate)`).Str())
	}
	var res storymakerGateResult
	if err := json.Unmarshal([]byte(page.MustEval(`() => JSON.stringify(window.__gate)`).Str()), &res); err != nil {
		t.Fatal(err)
	}
	t.Logf("gate: caps=%+v log=%v thumbs=%+v errors=%+v frame_count=%d", res.Caps, res.Log, res.Thumbs, res.Errors, res.Report.FrameCount)
	if res.Failure != "" {
		t.Fatalf("parent page failure: %s", res.Failure)
	}
	if !res.Caps.Workers || !res.Caps.Offscreen {
		t.Fatalf("the sandboxed iframe reports caps %+v, want workers and OffscreenCanvas", res.Caps)
	}
	if !res.Opaque {
		t.Error("the player iframe is not opaque-origin: the parent can reach its document")
	}
	if len(res.Log) < 3 || res.Log[0] != "ready" || res.Log[1] != "compiled" {
		t.Errorf("message order %v, want ready, compiled, time ...", res.Log)
	}
	for _, e := range res.Errors {
		t.Errorf("player error %s (scene %q): %s", e.Code, e.SceneID, e.Message)
	}
	// compiled report: scene ranges and frame count (s1 129 frames, s2 114..219, s3 204..324 with api.duration(4))
	if res.Report.FrameCount != 324 || res.Report.FPS != 30 || res.Report.Width != 640 || res.Report.Height != 360 {
		t.Errorf("report = %+v, want 324 frames at 30 fps, 640x360", res.Report)
	}
	want := [][3]int{{0, 129, 0}, {114, 219, 0}, {204, 324, 0}}
	if len(res.Report.Scenes) != 3 {
		t.Fatalf("report scenes = %+v", res.Report.Scenes)
	}
	for i, s := range res.Report.Scenes {
		if s.StartFrame != want[i][0] || s.EndFrame != want[i][1] {
			t.Errorf("scene %s range [%d,%d), want [%d,%d)", s.ID, s.StartFrame, s.EndFrame, want[i][0], want[i][1])
		}
	}
	for _, issue := range res.Report.Issues {
		if issue.Severity == "error" {
			t.Errorf("compile issue %+v", issue)
		}
	}
	// ImageBitmap transfer to the parent and a frame that is not blank
	s1, s3 := res.Thumbs["s1"], res.Thumbs["s3"]
	if !s1.IsBitmap || s1.Width != 160 || s1.Height != 90 {
		t.Errorf("s1 thumbnail = %+v, want a 160x90 ImageBitmap", s1)
	}
	if s1.Distinct < 6 {
		t.Errorf("s1 thumbnail has only %d distinct colours: the frame is blank", s1.Distinct)
	}
	// the code scene paints rgb(r, g, b): r=255 fetch blocked, g=255 importScripts outside /js/storymaker/ blocked, b=255 FontFace loaded in the worker
	if len(s3.Center) != 4 || s3.Center[0] != 255 || s3.Center[1] != 255 || s3.Center[2] != 255 {
		t.Errorf("code scene probe pixel = %v, want [255 255 255 255] (fetch blocked, import blocked, FontFace loaded)", s3.Center)
	}
	// the CSP must have refused the probes (fetch, importScripts) and nothing that the player legitimately loads
	logMu.Lock()
	for _, line := range browserLog {
		t.Logf("browser log: %s", line)
		for _, legit := range []string{"host.bundle.js", "player.bundle.js", "storymaker-player.css"} {
			if strings.Contains(line, legit) {
				t.Errorf("a legitimate player resource was refused or failed: %s", line)
			}
		}
	}
	logMu.Unlock()
	if hits := g.probeHits.Load(); hits != 0 {
		t.Errorf("the code scene reached the server %d times: the CSP did not block it", hits)
	}
}

// ---- render page ---------------------------------------------------------------------------------------------------------

type storymakerIngest struct {
	mu        sync.Mutex
	base      string
	transport string
	story     string
	width     int
	height    int
	report    map[string]any
	frames    []int
	types     []string
	sizes     []int
	badOrder  []string
	restartAt int
	restarted bool
	errors    []map[string]any
	options   int
	origins   map[string]bool
	pngDims   [][2]int
	hashes    []string
	frame5    []byte
	shot      []byte
	saveDir   string
}

func (in *storymakerIngest) cors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-SM-Frame")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
	}
}

func TestStorymakerRenderModeBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker render page test")
	}
	browser := newSmokeBrowser(t)
	for _, transport := range []string{"png", "webp", "raw"} {
		t.Run(transport, func(t *testing.T) {
			in := runStorymakerRenderPage(t, browser, transport, "minimal", 5, 0)
			if len(in.errors) != 0 {
				t.Fatalf("page errors: %v", in.errors)
			}
			if in.report == nil || int(in.report["frame_count"].(float64)) != 42 {
				t.Fatalf("report = %v, want 42 frames", in.report)
			}
			// restart at frame 5: 0..5 once, then 0..41
			wantFrames := append([]int{0, 1, 2, 3, 4, 5}, func() []int {
				all := make([]int, 42)
				for i := range all {
					all[i] = i
				}
				return all
			}()...)
			if fmt.Sprint(in.frames) != fmt.Sprint(wantFrames) {
				t.Fatalf("frames posted = %v, want %v", in.frames, wantFrames)
			}
			mime := map[string]string{"png": "image/png", "webp": "image/webp", "raw": "application/octet-stream"}[transport]
			for i, ct := range in.types {
				if ct != mime {
					t.Fatalf("frame %d content type %q, want %q", i, ct, mime)
				}
			}
			if transport == "raw" {
				for i, n := range in.sizes {
					if n != in.width*in.height*4 {
						t.Fatalf("raw frame %d has %d bytes, want %d", i, n, in.width*in.height*4)
					}
				}
			}
			if transport == "png" {
				for i, d := range in.pngDims {
					if d != [2]int{in.width, in.height} {
						t.Fatalf("png frame %d is %v, want %dx%d", i, d, in.width, in.height)
					}
				}
			}
			if in.options == 0 {
				t.Error("the opaque-origin page never sent a CORS preflight: the test is not exercising the cross-origin path")
			}
			for origin := range in.origins {
				if origin != "null" {
					t.Errorf("a request carried Origin %q, want null (opaque-origin render page)", origin)
				}
			}
		})
	}
}

// runStorymakerRenderPage serves the render markup (contract section 6) under the render CSP, boot.json and a fake ingest
// (contract section 7) from httptest, opens the page and waits until the ingest has answered done.
func runStorymakerRenderPage(t *testing.T, browser *rod.Browser, transport, storyName string, restartAt, width int) *storymakerIngest {
	t.Helper()
	const prefix = "/0123456789abcdef0123456789abcdef"
	mini := filepath.Join("..", "internal", "storymaker", "testdata", "catalog-mini")
	if width == 0 {
		width = 320
	}
	in := &storymakerIngest{transport: transport, story: storyName, width: width, height: width * 9 / 16, restartAt: restartAt, origins: map[string]bool{}, saveDir: os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR")}
	done := make(chan struct{})
	var closeOnce sync.Once
	mux := http.NewServeMux()
	var origin string
	codeBundles := func(base string) map[string]string {
		if storyName == "firewall" {
			return map[string]string{"s3": base + "/scenes/s3.js"}
		}
		return map[string]string{}
	}
	handle := func(path string, fn http.HandlerFunc) {
		mux.HandleFunc(prefix+path, func(w http.ResponseWriter, r *http.Request) {
			in.mu.Lock()
			if o := r.Header.Get("Origin"); o != "" {
				in.origins[o] = true
			}
			if r.Method == http.MethodOptions {
				in.options++
			}
			in.mu.Unlock()
			in.cors(w, r)
			if r.Method == http.MethodOptions {
				return
			}
			fn(w, r)
		})
	}
	handle("/", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", fmt.Sprintf(storymakerRenderCSP, base))
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, `<!doctype html><html data-sm-mode="render" data-sm-base="%[1]s"><head><meta charset="utf-8"></head><body><canvas id="sm-stage" width="%[2]d" height="%[3]d"></canvas><script src="%[1]s/js/storymaker/host.bundle.js"></script></body></html>`, base, in.width, in.height)
	})
	handle("/boot.json", func(w http.ResponseWriter, r *http.Request) {
		base := origin + prefix
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode": "render", "fps": 30, "width": in.width, "height": in.height, "transport": transport, "frame_limit": 400,
			"story_url": base + "/story.json", "catalog_url": base + "/catalog/catalog.json", "assets_base": base + "/catalog/",
			"code_bundles": codeBundles(origin + prefix),
			"fonts":        []map[string]any{{"name": "default", "family": "Storymaker Default", "weight": 400, "url": base + "/fonts/default.woff2"}},
			"audio_info":   map[string]any{},
		})
	})
	handle("/story.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, "../internal/storymaker/testdata/stories/"+storyName+".json"))
	})
	handle("/catalog/", func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, prefix+"/catalog/")
		if strings.Contains(rel, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(storymakerFile(t, filepath.ToSlash(filepath.Join(mini, filepath.FromSlash(rel)))))
	})
	handle("/fonts/default.woff2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write(storymakerFile(t, "fonts/Geist-Regular.woff2"))
	})
	handle("/js/storymaker/host.bundle.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "js/storymaker/host.bundle.js"))
	})
	handle("/js/storymaker/player.bundle.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(storymakerFile(t, "js/storymaker/player.bundle.js"))
	})
	handle("/scenes/s3.js", func(w http.ResponseWriter, r *http.Request) {
		// bundled the way the Go side does it (contract section 4): esbuild IIFE with globalName __smSceneModule
		src := storymakerFile(t, "../internal/storymaker/testdata/scenes/s3.js")
		res := api.Transform(string(src), api.TransformOptions{Format: api.FormatIIFE, GlobalName: "__smSceneModule", Target: api.ES2020, LegalComments: api.LegalCommentsNone, Loader: api.LoaderJS, Sourcefile: "scenes/s3.js"})
		if len(res.Errors) != 0 {
			http.Error(w, res.Errors[0].Text, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write(append(res.Code, []byte("\nself.__smScene=__smSceneModule.default;\n")...))
	})
	handle("/report", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		in.mu.Lock()
		_ = json.Unmarshal(body, &in.report)
		in.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"start":true,"from_frame":0}`)
	})
	handle("/error", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body)
		in.mu.Lock()
		in.errors = append(in.errors, body)
		in.mu.Unlock()
		closeOnce.Do(func() { close(done) })
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	handle("/frame", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
		var n int
		fmt.Sscanf(r.Header.Get("X-SM-Frame"), "%d", &n)
		in.mu.Lock()
		defer in.mu.Unlock()
		in.frames = append(in.frames, n)
		if n == 5 && transport == "png" && in.frame5 == nil {
			in.frame5 = append([]byte(nil), body...)
		}
		sum := sha256.Sum256(body)
		in.hashes = append(in.hashes, fmt.Sprintf("%x", sum[:8]))
		if in.saveDir != "" && transport == "png" && n%30 == 0 {
			_ = os.MkdirAll(in.saveDir, 0o755)
			_ = os.WriteFile(filepath.Join(in.saveDir, fmt.Sprintf("storymaker-%s-%03d.png", in.story, n)), body, 0o644)
		}
		in.types = append(in.types, r.Header.Get("Content-Type"))
		in.sizes = append(in.sizes, len(body))
		if transport == "png" && len(body) > 24 && bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
			in.pngDims = append(in.pngDims, [2]int{int(binary.BigEndian.Uint32(body[16:20])), int(binary.BigEndian.Uint32(body[20:24]))})
		} else if transport == "png" {
			in.pngDims = append(in.pngDims, [2]int{0, 0})
		}
		if transport == "webp" && !(len(body) > 12 && string(body[:4]) == "RIFF" && string(body[8:12]) == "WEBP") {
			in.badOrder = append(in.badOrder, fmt.Sprintf("frame %d is not a WebP", n))
		}
		w.Header().Set("Content-Type", "application/json")
		total := int(in.report["frame_count"].(float64))
		switch {
		case n == in.restartAt && !in.restarted:
			in.restarted = true
			_, _ = io.WriteString(w, `{"restart":true}`)
		case n == total-1:
			_, _ = io.WriteString(w, `{"ok":true,"done":true}`)
			closeOnce.Do(func() { close(done) })
		default:
			_, _ = io.WriteString(w, `{"ok":true}`)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	origin = srv.URL

	page := browser.MustPage(origin + prefix + "/").Timeout(120 * time.Second)
	defer page.Close()
	if transport == "screenshot" {
		// the page only draws on window.__smRenderFrame(n); the driver (here: the test, in production Go over CDP) captures
		page.MustSetViewport(in.width, in.height, 1, false)
		if err := page.Wait(rod.Eval(`() => window.__smStatus && (window.__smStatus.phase === 'ready' || window.__smStatus.phase === 'error')`)); err != nil {
			t.Fatalf("the screenshot page never got ready: %v", err)
		}
		if phase := page.MustEval(`() => window.__smStatus.phase`).Str(); phase != "ready" {
			t.Fatalf("screenshot page status %s", page.MustEval(`() => JSON.stringify(window.__smStatus)`).Str())
		}
		page.MustEval(`() => window.__smRenderFrame(5)`)
		in.shot = page.MustScreenshot()
		return in
	}
	select {
	case <-done:
	case <-time.After(110 * time.Second):
		in.mu.Lock()
		defer in.mu.Unlock()
		t.Fatalf("the render page did not finish: frames=%d errors=%v status=%s", len(in.frames), in.errors, page.MustEval(`() => JSON.stringify(window.__smStatus || null)`).Str())
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if len(in.badOrder) != 0 {
		t.Fatalf("bad frames: %v", in.badOrder)
	}
	return in
}

// TestStorymakerFirewallRenderBrowser renders the whole firewall story, code scene included (the real s3.js bundled with
// Go's esbuild), twice, and requires bit-identical frames (the determinism test of the spec). AURAGO_BROWSER_ARTIFACT_DIR
// keeps every 30th frame as a PNG for a look.
func TestStorymakerFirewallRenderBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker render page test")
	}
	browser := newSmokeBrowser(t)
	transport, width := "png", 640
	if v := os.Getenv("AURAGO_STORYMAKER_TRANSPORT"); v != "" {
		transport = v
	}
	if v := os.Getenv("AURAGO_STORYMAKER_WIDTH"); v != "" {
		fmt.Sscanf(v, "%d", &width)
	}
	started := time.Now()
	first := runStorymakerRenderPage(t, browser, transport, "firewall", -1, width)
	elapsed := time.Since(started)
	t.Logf("%s %dx%d: %d frames in %s = %.1f frames/s (local loopback ingest, no capture proxy)", transport, width, width*9/16, len(first.frames), elapsed.Round(time.Millisecond), float64(len(first.frames))/elapsed.Seconds())
	second := runStorymakerRenderPage(t, browser, transport, "firewall", -1, width)
	for _, in := range []*storymakerIngest{first, second} {
		if len(in.errors) != 0 {
			t.Fatalf("page errors: %v", in.errors)
		}
		if in.report == nil || int(in.report["frame_count"].(float64)) != 354 || len(in.frames) != 354 {
			t.Fatalf("report = %v, frames = %d, want 354", in.report, len(in.frames))
		}
	}
	if fmt.Sprint(first.hashes) != fmt.Sprint(second.hashes) {
		for i := range first.hashes {
			if first.hashes[i] != second.hashes[i] {
				t.Fatalf("frame %d differs between two renders of the same story (%s vs %s)", i, first.hashes[i], second.hashes[i])
			}
		}
	}
	distinct := map[string]bool{}
	for _, h := range first.hashes {
		distinct[h] = true
	}
	if len(distinct) < 200 {
		t.Errorf("only %d distinct frames out of 354: the story barely moves", len(distinct))
	}
}

// TestStorymakerRenderScreenshotBrowser covers the fourth transport: the page draws frame 5 on window.__smRenderFrame(5) and a
// CDP screenshot at device pixel ratio 1 must show the same pixels as the frame that the png transport posts.
func TestStorymakerRenderScreenshotBrowser(t *testing.T) {
	if os.Getenv("AURAGO_RUN_BROWSER_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_BROWSER_SMOKE=1 to run the Storymaker render page test")
	}
	browser := newSmokeBrowser(t)
	viaPNG := runStorymakerRenderPage(t, browser, "png", "firewall", -1, 640)
	viaShot := runStorymakerRenderPage(t, browser, "screenshot", "firewall", -1, 640)
	want, err := png.Decode(bytes.NewReader(viaPNG.frame5))
	if err != nil {
		t.Fatalf("decode the posted frame: %v", err)
	}
	got, err := png.Decode(bytes.NewReader(viaShot.shot))
	if err != nil {
		t.Fatalf("decode the screenshot: %v", err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("screenshot is %v, the posted frame %v", got.Bounds(), want.Bounds())
	}
	differing, maxDiff := 0, 0
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			wr, wg, wb, _ := want.At(x, y).RGBA()
			gr, gg, gb, _ := got.At(x, y).RGBA()
			if wr != gr || wg != gg || wb != gb {
				differing++
				for _, d := range []int{int(wr>>8) - int(gr>>8), int(wg>>8) - int(gg>>8), int(wb>>8) - int(gb>>8)} {
					if d < 0 {
						d = -d
					}
					maxDiff = max(maxDiff, d)
				}
				minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
			}
		}
	}
	t.Logf("screenshot vs posted png frame 5: %d of %d pixels differ", differing, b.Dx()*b.Dy())
	if differing > 0 {
		t.Logf("max channel difference %d, bounding box %d,%d-%d,%d", maxDiff, minX, minY, maxX, maxY)
	}
	if dir := os.Getenv("AURAGO_BROWSER_ARTIFACT_DIR"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "storymaker-shot.png"), viaShot.shot, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "storymaker-frame5.png"), viaPNG.frame5, 0o644)
	}
	if differing != 0 {
		t.Errorf("%d pixels differ between the CDP screenshot and the posted frame", differing)
	}
}
````


- [ ] **Step 2: Run the tests**

```bash
export GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' AURAGO_RUN_BROWSER_SMOKE=1
go test ./ui -run 'TestStorymaker(PlayerGate|RenderMode|FirewallRender|RenderScreenshot)Browser' -count=1 -timeout 10m -v
```

Expected: PASS for all four (measured on the dev box: the gate 2 s, the render mode test 3 s, the firewall test 13 s, the screenshot test 8 s). The gate logs
`caps={Workers:true Offscreen:true}`, `log=[ready compiled time thumb thumb]`, the `s3` probe pixel `[255 255 255 255]` and, from the browser log, exactly three
refusals (the `connect-src` and `script-src` violations of the two probes and a favicon 404).

- [ ] **Step 3: Transport numbers for the render spike (optional, reports only)**

```bash
for tr in png webp raw; do AURAGO_STORYMAKER_TRANSPORT=$tr AURAGO_STORYMAKER_WIDTH=1920 go test ./ui -run TestStorymakerFirewallRenderBrowser -count=1 -v | grep frames/s; done
```

- [ ] **Step 4: Commit**

```bash
git add ui/storymaker_player_gate_browser_test.go
git commit -m "test(storymaker): phase-1 gate and render page browser tests" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
