import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const read = file => readFileSync(new URL('../' + file, import.meta.url), 'utf8');
class CustomEvent extends Event {
    constructor(type, options = {}) { super(type); this.detail = options.detail; }
}
class Target extends EventTarget {
    listeners = new Set();
    addEventListener(type, listener) { this.listeners.add(listener); super.addEventListener(type, listener); }
    removeEventListener(type, listener) { this.listeners.delete(listener); super.removeEventListener(type, listener); }
}
const flush = async () => { for (let i = 0; i < 20; i++) await Promise.resolve(); };
const emit = (target, type, detail) => target.dispatchEvent(new CustomEvent(type, { detail }));

function harness({ reduced = false, key, autoLoad = true, failCatalog = false, failScript = false } = {}) {
    const window = new Target(), document = new Target(), runtime = new Target(), motion = new Target();
    const frames = new Map(), timers = new Map(), requests = [], players = [], observers = [];
    let nextID = 0, now = 1000;
    motion.matches = reduced;
    document.hidden = false;
    document.head = { appendChild(script) { requests.push(script.src); queueMicrotask(() => failScript ? script.onerror() : script.onload()); } };
    document.createElement = () => ({ remove() {} });
    const image = { getAttribute(name) { return this[name]; } }, canvas = {};
    const host = { dataset: {}, isConnected: true, innerHTML: '', querySelector: name => name === 'img' ? image : canvas };
    class Observer {
        constructor(fn) { this.fn = fn; this.disconnected = false; observers.push(this); }
        observe() { this.fn([{ isIntersecting: true }]); }
        disconnect() { this.disconnected = true; }
    }
    class Rive {
        constructor(options) {
            this.options = options;
            this.inputs = Object.fromEntries(['mode', 'viseme', 'mouthOpen', 'headTilt'].map(name => [name, { name, value: 0 }]));
            players.push(this);
            if (autoLoad) queueMicrotask(() => options.onLoad());
        }
        stateMachineInputs() { return Object.values(this.inputs); }
        resizeDrawingSurfaceToCanvas(dpr) { this.dpr = dpr; }
        play() { this.playing = true; this.options.onAdvance(); }
        pause() { this.playing = false; }
        cleanup() { this.cleaned = true; this.playing = false; }
    }
    Object.assign(window, {
        BUILD_VERSION: 'test build', _activePersonaIconKey: key, devicePixelRatio: 3,
        IntersectionObserver: Observer, ResizeObserver: Observer,
        matchMedia: () => motion,
        requestAnimationFrame(fn) { const id = ++nextID; frames.set(id, fn); return id; },
        cancelAnimationFrame(id) { frames.delete(id); },
        setTimeout(fn) { const id = ++nextID; timers.set(id, fn); return id; },
        clearTimeout(id) { timers.delete(id); },
        rive: { Rive, Layout: class {}, Fit: { Contain: 'contain' }, Alignment: { Center: 'center' }, RuntimeLoader: {
            setWasmUrl(url) { requests.push(url); }, setWasmFallbackUrl(url) { assert.equal(url, null); }
        } }
    });
    Object.assign(runtime, { state: 'idle', sessionId: '', userSpeaking: false, providerSpeaking: false, actionActive: false, adapter: null });
    const context = vm.createContext({ window, document, IntersectionObserver: Observer, ResizeObserver: Observer,
        fetch: async url => {
            requests.push(url);
            return { ok: !failCatalog, json: async () => url === '/api/personalities'
                ? { active: 'neutral', personalities: [{ name: 'neutral', core: true }] }
                : JSON.parse(read('ui/img/personas/animated/catalog.json')) };
        }
    });
    vm.runInContext(read('ui/js/realtime-speech/avatar.js'), context);
    return { window, document, runtime, motion, frames, timers, requests, players, observers, host, image,
        mount: visible => window.AuraRealtimeSpeechAvatar.mount(host, { runtime, visible }),
        step(ms = 16) { now += ms; const work = [...frames.values()]; frames.clear(); work.forEach(fn => fn(now)); },
        persona(key) { window._activePersonaIconKey = key; emit(window, 'aurago:persona-icon-change', { key }); }
    };
}

// Real-time output, not provider intent or microphone energy, owns speech poses.
const h = harness();
const view = h.mount(false);
await flush();
assert.equal(h.requests.length, 0, 'a mounted closed overlay must not load anything');
view.setVisible(true);
await flush();
assert.equal(h.players.length, 1);
assert.ok(h.requests.includes('/api/personalities'), 'Desktop must resolve a persona before Agent Chat exists');
assert.equal(h.host.dataset.persona, 'neutral');
assert.equal(h.players[0].dpr, 2);
assert.equal(h.players[0].options.enableRiveAssetCDN, false);
assert.ok(h.requests.filter(url => url !== '/api/personalities').every(url => url.startsWith('/') && url.endsWith('?v=test%20build')));
const inputs = h.players[0].inputs;
h.step();
assert.equal(inputs.mode.value, 0);
h.runtime.sessionId = 'test'; h.runtime.state = 'speaking'; h.runtime.providerSpeaking = true;
h.runtime.adapter = { getOutputLevel: () => 0 };
emit(h.runtime, 'level', { rms: 1 }); h.step();
assert.equal(inputs.mouthOpen.value, 0, 'pending output and loud microphone must leave the mouth closed');
h.runtime.adapter.getOutputLevel = () => 0.3;
h.runtime.actionActive = true; h.runtime.state = 'executing';
h.step(50); h.step(50);
assert.equal(inputs.mode.value, 3, 'audible output takes priority over an ongoing action');
assert.ok(inputs.mouthOpen.value > 0.3);
assert.equal(inputs.viseme.value, 1);
const speechPoses = new Set();
for (const level of [0.3, 0.08, 0.14, 0.2, 0.04, 0]) {
    h.runtime.adapter.getOutputLevel = () => level;
    for (let frame = 0; frame < 6; frame++) { h.step(); speechPoses.add(inputs.viseme.value); }
}
assert.ok(speechPoses.size >= 4, 'changing speech energy must change actual Rive poses, not just a numeric mouthOpen value');
assert.equal(inputs.viseme.value, 0, 'a short speech pause must visibly close the mouth');
h.runtime.adapter.getOutputLevel = () => 0.3; h.step(50); h.step(50);
h.runtime.providerSpeaking = false; h.runtime.state = 'listening'; h.step();
assert.equal(inputs.mode.value, 3, 'a response.done event must not truncate an audible tail');
h.runtime.adapter.getOutputLevel = () => 0; h.step(50);
assert.equal(inputs.mode.value, 3, 'a short pause must not chatter between expressions');
h.step(180);
assert.equal(inputs.mouthOpen.value, 0);
assert.equal(inputs.mode.value, 2);
h.runtime.actionActive = false;
h.runtime.adapter = { player: { getOutputLevel: () => 0.4 } }; h.step(80); h.step(50);
assert.equal(inputs.mode.value, 3, 'a replacement PCM adapter must supply the current output');
h.runtime.userSpeaking = true; emit(h.runtime, 'state', {});
assert.equal(inputs.mouthOpen.value, 0, 'barge-in closes immediately');
h.step(); assert.equal(inputs.mode.value, 1);
h.runtime.userSpeaking = false;
for (const value of [NaN, Infinity, -1, undefined]) {
    h.runtime.adapter = { getOutputLevel: () => value }; h.step(200);
    assert.equal(inputs.mouthOpen.value, 0);
}
h.runtime.adapter = { getOutputLevel() { throw Error('unavailable'); } }; h.step();
assert.equal(inputs.mouthOpen.value, 0);
for (const state of ['error', 'parked', 'connecting', 'reconnecting', 'idle', 'closed']) {
    h.runtime.state = state; emit(h.runtime, 'state', {}); h.step();
    assert.equal(inputs.mode.value, 0); assert.equal(inputs.mouthOpen.value, 0);
}
view.setVisible(false);
assert.equal(h.frames.size, 0); assert.equal(h.players[0].playing, false);
view.setVisible(true); await flush();
assert.equal(h.players.length, 1, 'reopening must reuse the loaded player');
h.document.hidden = true; emit(h.document, 'visibilitychange');
assert.equal(h.frames.size, 0);
h.document.hidden = false; emit(h.document, 'visibilitychange');
h.motion.matches = true; emit(h.motion, 'change');
assert.equal(h.host.dataset.animated, 'false'); assert.equal(h.frames.size, 0);
h.motion.matches = false; emit(h.motion, 'change');
assert.equal(h.frames.size, 1);
for (const key of ['mcp', 'terminator']) {
    const previous = h.players.at(-1); h.persona(key); await flush();
    assert.equal(previous.cleaned, true);
    h.runtime.state = 'speaking'; h.runtime.adapter = { getOutputLevel: () => 0.3 };
    h.step(50); h.step(50);
    assert.ok(h.players.at(-1).inputs.viseme.value > 1, 'robots use energy-controlled display/jaw poses');
}
h.persona('../../outside'); await flush();
assert.equal(h.host.dataset.persona, 'custom');
assert.ok(h.image.src.includes('/custom.png?'));
assert.equal(h.players.at(-1).cleaned, true);
view.dispose(); view.dispose();
assert.equal(h.frames.size, 0); assert.equal(h.timers.size, 0);
for (const target of [h.window, h.document, h.runtime, h.motion]) assert.equal(target.listeners.size, 0);
assert.ok(h.observers.every(observer => observer.disconnected));

// Late Rive callbacks, reduced motion on first open, and failure fallback.
const late = harness({ key: 'neutral', autoLoad: false });
const lateView = late.mount(true); await flush();
const stale = late.players[0]; late.persona('punk'); await flush();
stale.options.onLoad(); stale.options.onAdvance();
assert.equal(stale.cleaned, true); assert.equal(late.host.dataset.animated, 'false');
lateView.dispose(); late.players.at(-1).options.onLoad();
assert.equal(late.frames.size, 0); assert.ok(late.players.every(player => player.cleaned));
const reduced = harness({ reduced: true, key: 'friend' });
const reducedView = reduced.mount(true); await flush();
assert.equal(reduced.players.length, 0); assert.ok(reduced.image.src.includes('/friend.png?'));
assert.equal(reduced.requests.some(url => url.includes('/rive/')), false);
reduced.motion.matches = false; emit(reduced.motion, 'change'); await flush();
assert.equal(reduced.players.length, 1); reducedView.dispose();
for (const options of [{ failCatalog: true }, { failScript: true }, { autoLoad: false }]) {
    const failed = harness(options), controller = failed.mount(true); await flush();
    for (const callback of [...failed.timers.values()]) callback();
    assert.equal(failed.host.dataset.animated, 'false'); assert.equal(failed.frames.size, 0);
    controller.dispose();
}

// Exercise the real output getters with playback state changes.
const window = { AuraRealtimeProviders: {} };
const context = vm.createContext({ window, EventTarget, CustomEvent });
for (const file of ['provider-common', 'provider-openai', 'provider-speech-lab']) vm.runInContext(read('ui/js/realtime-speech/' + file + '.js'), context);
const analyser = { getFloatTimeDomainData(buffer) { buffer.fill(0.2); } };
for (const provider of ['openai', 'speech_lab']) {
    const adapter = new window.AuraRealtimeProviders[provider]({});
    const audio = { paused: false, ended: false, muted: false, volume: 1 };
    Object.assign(adapter, { remoteAudio: audio, outputAudio: audio, outputContext: { state: 'running' },
        outputTap: { analyser, buffer: new Float32Array(16) }, outputSource: {}, outputAnalyser: analyser, outputBuffer: new Float32Array(16) });
    assert.ok(adapter.getOutputLevel() > 0);
    for (const property of ['paused', 'ended', 'muted']) {
        audio[property] = true; assert.equal(adapter.getOutputLevel(), 0); audio[property] = false;
    }
    audio.volume = 0; assert.equal(adapter.getOutputLevel(), 0); audio.volume = 1;
    adapter.outputContext.state = 'suspended'; assert.equal(adapter.getOutputLevel(), 0);
}
const pcm = new window.AuraRealtimeProviderCommon.PCMPlayer();
Object.assign(pcm, { active: true, analyser, analyserTime: new Float32Array(16), context: { state: 'running' } });
assert.ok(pcm.getOutputLevel() > 0);
pcm.context.state = 'suspended'; assert.equal(pcm.getOutputLevel(), 0);

console.log('Live Speech avatar state, playback, loading and disposal checks passed.');
