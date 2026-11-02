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
// stories that only the player tests use (the cast sheet that the parity browser test renders)
export const playerStory = name => readJSON('internal/storymaker/testdata/player/' + name + '.json');

// The real starter catalog (ui/img/storymaker, part D): the manifest, the rig, every prop and every environment file.
export const REAL = 'ui/img/storymaker/';
export const realCatalog = () => readJSON(REAL + 'catalog.json');
let realCache = null;
export function realFiles() {
    if (!realCache) {
        const cat = realCatalog(), props = {}, envs = {};
        for (const e of cat.entries) {
            if (e.kind === 'prop') props[e.file] = readJSON(REAL + e.file);
            else if (e.kind === 'environment') envs[e.file] = readJSON(REAL + e.file);
        }
        realCache = { rig: readJSON(REAL + cat.rig.file), props, envs };
    }
    return realCache;
}

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
export function renderAssets(extra = {}, files = miniFiles()) {
    const f = files, canvases = {};
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

// Render assets of the real starter catalog.
export const realAssets = (extra = {}) => renderAssets(extra, realFiles());

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

// Deadlines for the tests that drive the host with real timers. The production limits (250 ms per frame, 2 s per scene build) make
// such a test fail when the machine is busy, so the tests run with generous ones; a test that is about a deadline passes its own.
export const TEST_LIMITS = { compileDeadlineMs: 60000, sceneBuildDeadlineMs: 20000, assetsDeadlineMs: 60000, helloDeadlineMs: 20000, frameDeadlineMs: 20000, warmupDeadlineMs: 20000 };

// loadHost(): the real host.bundle.js parts in a fresh vm context (no document: nothing auto-mounts).
export function loadHost() {
    const ctx = vm.createContext({ console, URL, Promise });
    vm.runInContext('globalThis.self = globalThis;', ctx);
    vm.runInContext(hostSource(), ctx, { filename: 'host.bundle.js' });
    return ctx;
}
