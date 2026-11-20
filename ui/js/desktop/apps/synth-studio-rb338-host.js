// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

// Main-thread host of the OpenRB-338 WebAssembly engine: asset loading, the live AudioWorklet
// engine, the offline Worker renderer and the Arrange audio-clip cache.
(function (root, factory) {
    const api = factory(root);
    if (typeof module === 'object' && module.exports) module.exports = api;
    else root.SynthStudioRB338Host = api;
})(typeof globalThis !== 'undefined' ? globalThis : window, function (root) {
    'use strict';

    const WASM_URL = '/js/vendor/rb338/rb338-engine.wasm';
    const KIT_URL = '/js/vendor/rb338/Standard.rbm';
    const WORKLET_URL = '/js/desktop/apps/synth-studio-rb338-worklet.js';
    const WORKER_URL = '/js/desktop/apps/synth-studio-rb338-worker.js';
    const RATE = 44100;
    const TICKS_PER_BAR = 768;

    function versioned(url) {
        const lazy = root.AuraLazyAssets;
        if (lazy && typeof lazy.versionedURL === 'function') return lazy.versionedURL(url);
        return url + '?v=' + encodeURIComponent(root.BUILD_VERSION || 'dev');
    }

    // ---- assets -------------------------------------------------------------------------------

    let assetPromise = null;

    function assets() {
        if (assetPromise) return assetPromise;
        const get = async (url) => {
            const res = await root.fetch(versioned(url));
            if (!res.ok) throw new Error('rb338 asset ' + url + ' failed: ' + res.status);
            return res.arrayBuffer();
        };
        assetPromise = (async () => {
            const [bytes, kit] = await Promise.all([get(WASM_URL), get(KIT_URL)]);
            const module = await WebAssembly.compile(bytes);
            return { module, bytes, kit };
        })();
        assetPromise.catch(() => { assetPromise = null; });
        return assetPromise;
    }

    function postInit(target, a, extra) {
        const msg = Object.assign({ type: 'init', module: a.module, kit: a.kit }, extra || {});
        try {
            target.postMessage(msg);
        } catch (err) {
            if (err && err.name === 'DataCloneError') {
                delete msg.module;
                msg.bytes = a.bytes;
                target.postMessage(msg);
            } else {
                throw err;
            }
        }
    }

    // ---- live engine ----------------------------------------------------------------------------

    function live(opts) {
        const o = opts || {};
        const Ctor = root.AudioContext || root.webkitAudioContext;
        if (!Ctor) throw new Error('Web Audio is not available');
        // synchronous: must stay inside the user gesture
        let context;
        try {
            context = new Ctor({ sampleRate: RATE, latencyHint: 'interactive' });
        } catch (_err) {
            context = new Ctor({ latencyHint: 'interactive' });
        }
        const ratio = context.sampleRate === RATE ? 1 : RATE / context.sampleRate;

        let node = null;
        let closed = false;
        let nextId = 1;
        const pendingOut = [];
        const waiting = new Map();
        let readySettled = false;
        let readyResolve;
        let readyReject;
        const ready = new Promise((res, rej) => { readyResolve = res; readyReject = rej; });
        ready.catch(() => { /* surfaced through onError/onCrash as well */ });

        const post = (msg, transfer) => {
            if (closed) return;
            if (!node) pendingOut.push([msg, transfer]);
            else node.port.postMessage(msg, transfer || []);
        };

        const rejectAll = (err) => {
            for (const w of waiting.values()) w.reject(err);
            waiting.clear();
        };

        const settleReady = (ok, value) => {
            if (readySettled) return;
            readySettled = true;
            if (ok) readyResolve(value); else readyReject(value);
        };

        const onMessage = (e) => {
            const d = e.data;
            if (!d) return;
            switch (d.type) {
                case 'ready': {
                    const v = { defaults: d.defaults, patterns: d.patterns, info: d.info };
                    settleReady(true, v);
                    if (o.onReady) o.onReady(v);
                    break;
                }
                case 'snap':
                    if (o.onSnapshot) o.onSnapshot(d.buf);
                    break;
                case 'loaded': {
                    const v = { id: d.id, ok: d.ok, code: d.code, patterns: d.patterns, info: d.info };
                    const w = waiting.get(d.id);
                    if (w) { waiting.delete(d.id); w.resolve(v); }
                    if (o.onLoaded) o.onLoaded(v);
                    break;
                }
                case 'saved': {
                    const w = waiting.get(d.id);
                    if (w) { waiting.delete(d.id); w.resolve(d.bytes); }
                    if (o.onSaved) o.onSaved({ id: d.id, bytes: d.bytes });
                    break;
                }
                case 'error': {
                    const err = new Error('rb338 error ' + d.code);
                    err.code = d.code;
                    if (d.id && waiting.has(d.id)) {
                        const w = waiting.get(d.id);
                        waiting.delete(d.id);
                        w.reject(err);
                    } else if (d.code === 1 || d.code === 2 || d.code === 3 || d.code === 10) {
                        settleReady(false, err);
                    }
                    if (o.onError) o.onError(d.code);
                    break;
                }
                case 'crash': {
                    const err = new Error(d.message || 'rb338 crash');
                    settleReady(false, err);
                    rejectAll(err);
                    if (o.onCrash) o.onCrash(d.message || 'crash');
                    break;
                }
                default:
                    break;
            }
        };

        const request = (type, extra, transfer) => new Promise((resolve, reject) => {
            const id = nextId++;
            waiting.set(id, { resolve, reject });
            post(Object.assign({ type, id }, extra || {}), transfer);
        });

        const boot = (async () => {
            const [a] = await Promise.all([assets(), context.audioWorklet.addModule(versioned(WORKLET_URL))]);
            if (closed) return;
            node = new root.AudioWorkletNode(context, 'aurago-rb338', {
                numberOfInputs: 0,
                numberOfOutputs: 1,
                outputChannelCount: [2],
                processorOptions: { ratio }
            });
            node.port.onmessage = onMessage;
            node.onprocessorerror = () => {
                const err = new Error('rb338 worklet processor error');
                settleReady(false, err);
                rejectAll(err);
                if (o.onCrash) o.onCrash(err.message);
            };
            node.connect(context.destination);
            const extra = {};
            if (o.song && o.song.byteLength) extra.song = o.song;
            postInit(node.port, a, extra);
            for (const [msg, transfer] of pendingOut.splice(0)) node.port.postMessage(msg, transfer || []);
        })();
        boot.catch((err) => {
            settleReady(false, err);
            rejectAll(err);
            if (o.onError) o.onError(err && err.name === 'DataCloneError' ? 10 : 0);
        });

        return {
            context,
            ready,
            send(batch, seq) {
                const bytes = batch instanceof Uint8Array ? batch : new Uint8Array(batch);
                const copy = bytes.slice();
                post({ type: 'msgs', buf: copy, seq: seq >>> 0 }, [copy.buffer]);
            },
            load(bytes) {
                const copy = new Uint8Array(bytes).slice();
                return request('load', { bytes: copy }, [copy.buffer]);
            },
            newSong() {
                return request('new');
            },
            save() {
                return request('save');
            },
            startup() { post({ type: 'startup' }); },
            resume() { return context.state === 'closed' ? Promise.resolve() : context.resume(); },
            suspend() { return context.state === 'closed' ? Promise.resolve() : context.suspend(); },
            close() {
                if (closed) return;
                closed = true;
                rejectAll(new Error('rb338 closed'));
                try { if (node) node.disconnect(); } catch (_err) { /* already gone */ }
                if (node) node.port.onmessage = null;
                if (context.state !== 'closed') context.close().catch(() => {});
            }
        };
    }

    // ---- offline renderer -----------------------------------------------------------------------

    function renderer() {
        let worker = null;
        let nextId = 1;
        let closed = false;
        const waiting = new Map();
        let ready = null;

        const fail = (err) => {
            for (const w of waiting.values()) w.reject(err);
            waiting.clear();
        };

        const start = () => {
            if (ready) return ready;
            ready = (async () => {
                const a = await assets();
                if (closed) throw new Error('rb338 renderer closed');
                worker = new root.Worker(versioned(WORKER_URL), { type: 'module' });
                const up = new Promise((resolve, reject) => {
                    worker.onmessage = (e) => {
                        const d = e.data;
                        if (!d) return;
                        if (d.type === 'ready') { resolve(); return; }
                        if (d.type === 'rendered') {
                            const w = waiting.get(d.id);
                            if (w) { waiting.delete(d.id); w.resolve({ wav: d.wav, frames: d.frames }); }
                        } else if (d.type === 'error') {
                            const err = new Error('rb338 render error ' + d.code);
                            err.code = d.code;
                            const w = waiting.get(d.id);
                            if (w) { waiting.delete(d.id); w.reject(err); } else { reject(err); fail(err); }
                        }
                    };
                    worker.onerror = (ev) => {
                        const err = new Error((ev && ev.message) || 'rb338 worker error');
                        reject(err);
                        fail(err);
                    };
                });
                postInit(worker, a);
                await up;
            })();
            ready.catch(() => { ready = null; });
            return ready;
        };

        return {
            async render(req) {
                const r = req || {};
                await start();
                if (closed) throw new Error('rb338 renderer closed');
                const song = r.song instanceof ArrayBuffer ? r.song.slice(0) : new Uint8Array(r.song).slice().buffer;
                return new Promise((resolve, reject) => {
                    const id = nextId++;
                    waiting.set(id, { resolve, reject });
                    worker.postMessage({
                        type: 'render', id, song,
                        mode: r.mode === 'pattern' || r.mode === 1 ? 'pattern' : 'song',
                        bars: r.bars || 4,
                        preroll: !!r.preroll
                    }, [song]);
                });
            },
            close() {
                closed = true;
                fail(new Error('rb338 renderer closed'));
                if (worker) worker.terminate();
                worker = null;
                ready = null;
            }
        };
    }

    // ---- audio clips ----------------------------------------------------------------------------

    function wavToBuffer(wav) {
        const dv = new DataView(wav);
        const frames = (wav.byteLength - 44) >> 2;
        const buf = new root.AudioBuffer({ numberOfChannels: 2, length: Math.max(1, frames), sampleRate: RATE });
        const l = buf.getChannelData(0);
        const r = buf.getChannelData(1);
        for (let i = 0; i < frames; i++) {
            l[i] = dv.getInt16(44 + i * 4, true) / 32768;
            r[i] = dv.getInt16(46 + i * 4, true) / 32768;
        }
        return buf;
    }

    function b64ToBytes(b64) {
        const s = root.atob(String(b64 || ''));
        const out = new Uint8Array(s.length);
        for (let i = 0; i < s.length; i++) out[i] = s.charCodeAt(i);
        return out;
    }

    function clipKey(clip) {
        const a = clip && clip.audio;
        if (!a || a.kind !== 'rb338') return '';
        return [a.mode, a.start, a.end, a.song].join('|');
    }

    function createClips() {
        const cache = new Map(); // key -> AudioBuffer | 'pending' | 'failed'
        let rend = null;
        let queue = Promise.resolve();

        const renderClip = async (clip) => {
            const a = clip.audio;
            if (!rend) rend = renderer();
            const pattern = a.mode === 'pattern';
            const bars = Math.max(1, Math.round((a.end - a.start) / TICKS_PER_BAR));
            const song = b64ToBytes(a.song);
            // pattern clips use the same pre-roll cycle as Send to Arrange, so they loop seamlessly
            const out = await rend.render({ song: song.buffer, mode: a.mode, bars, preroll: pattern });
            return wavToBuffer(out.wav);
        };

        return {
            buffer(clip) {
                const v = cache.get(clipKey(clip));
                return v && typeof v === 'object' ? v : null;
            },
            // Seeds the cache with a buffer already rendered by Send to Arrange.
            put(clip, audioBuffer) {
                const k = clipKey(clip);
                if (k && audioBuffer) cache.set(k, audioBuffer);
            },
            prepare(project, onReady) {
                const todo = [];
                const seen = new Set();
                for (const track of (project && project.tracks) || []) {
                    for (const clip of track.clips || []) {
                        const k = clipKey(clip);
                        if (!k || seen.has(k) || cache.has(k)) continue;
                        seen.add(k);
                        todo.push([k, clip]);
                    }
                }
                if (!todo.length) return Promise.resolve();
                for (const [k] of todo) cache.set(k, 'pending');
                const run = queue.then(async () => {
                    for (const [k, clip] of todo) {
                        try {
                            cache.set(k, await renderClip(clip));
                            if (onReady) onReady(clip);
                        } catch (_err) {
                            cache.set(k, 'failed');
                        }
                    }
                });
                queue = run.catch(() => {});
                return run;
            },
            failed(clip) {
                return cache.get(clipKey(clip)) === 'failed';
            },
            close() {
                if (rend) rend.close();
                rend = null;
            }
        };
    }

    return { assets, live, renderer, clips: createClips(), wavToBuffer, clipKey };
});
