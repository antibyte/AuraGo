// AuraPinball audio engine: AudioContext lifecycle, buses, limiter, shared reverb, sample loader, noise pool, ambience beds.
// Nothing here touches AudioContext at import time (Node can import it).
import { ASSET_ROOT } from '../config.js';

export const SAMPLE_NAMES = Object.freeze([
    'flipper', 'bumper', 'sling', 'target', 'launch', 'drain', 'lock', 'jackpot', 'bonus', 'clack', 'rubber', 'scoop', 'knock'
]);
export const REVERB_SECONDS = Object.freeze({ neon: 1.2, leviathan: 2.2, temple: 1.2 });

// Perceptual volume curve; a true zero stays exactly zero.
export function volumeGain(v) {
    const x = Number.isFinite(+v) ? Math.min(1, Math.max(0, +v)) : 0;
    return x <= 0.001 ? 0 : x * x;
}

function mulberry(seed) {
    let s = seed >>> 0;
    return () => { s = (s + 0x6D2B79F5) >>> 0; let t = Math.imul(s ^ (s >>> 15), 1 | s); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
}

export function createEngine({ tableId = 'neon', assetURL = path => path, assetRoot = ASSET_ROOT, settings = {}, AudioContextCtor = null, fetchImpl = null } = {}) {
    let ctx = null, disposed = false, reduced = !!settings.reduced, vols = { effects: settings.effects ?? .65, music: settings.music ?? .5, ambience: settings.ambience ?? .3 };
    let buses = null, reverbSend = null, reverbWet = null, ambienceStop = null, suspendedByUs = false;
    const buffers = new Map(), noises = new Map(), pending = new Map(), life = new AbortController();

    function Ctor() { return AudioContextCtor || globalThis.AudioContext || globalThis.webkitAudioContext || null; }
    function apply(bus, key) {
        if (!bus) return;
        const g = volumeGain(vols[key]), t = ctx.currentTime;
        bus.gain.cancelScheduledValues(t);
        if (g === 0) bus.gain.setValueAtTime(0, t); else bus.gain.setTargetAtTime(g, t, .03);
    }
    function applyAll() { if (!ctx) return; apply(buses.effects, 'effects'); apply(buses.music, 'music'); apply(buses.ambience, 'ambience'); reverbWet.gain.value = reduced ? 0 : .55; }

    function impulse(seconds) {
        const rate = ctx.sampleRate, n = Math.max(1, Math.floor(rate * seconds)), buf = ctx.createBuffer(2, n, rate), rnd = mulberry(seconds > 2 ? 1337 : 7331);
        for (let c = 0; c < 2; c++) {
            const d = buf.getChannelData(c);
            let lp = 0;
            for (let i = 0; i < n; i++) {
                const t = i / n, env = Math.pow(1 - t, 3.2) * Math.min(1, i / (rate * .004));
                lp += ((rnd() * 2 - 1) - lp) * (.25 + .6 * (1 - t) * (1 - t)); // darkens as it decays
                d[i] = lp * env * 1.6;
            }
        }
        return buf;
    }

    function build() {
        const AC = Ctor();
        if (!AC) return false;
        try { ctx = new AC({ latencyHint: 'interactive' }); } catch { try { ctx = new AC(); } catch { return false; } }
        const limiter = ctx.createDynamicsCompressor();
        limiter.threshold.value = -4; limiter.knee.value = 4; limiter.ratio.value = 14; limiter.attack.value = .002; limiter.release.value = .12;
        const master = ctx.createGain(); master.gain.value = 1;
        master.connect(limiter); limiter.connect(ctx.destination);
        const effects = ctx.createGain(), music = ctx.createGain(), ambience = ctx.createGain();
        for (const b of [effects, music, ambience]) { b.gain.value = 0; b.connect(master); }
        const convolver = ctx.createConvolver(); convolver.buffer = impulse(REVERB_SECONDS[tableId] || 1.2);
        reverbSend = ctx.createGain(); reverbSend.gain.value = 1;
        reverbWet = ctx.createGain(); reverbWet.gain.value = reduced ? 0 : .55;
        reverbSend.connect(convolver); convolver.connect(reverbWet); reverbWet.connect(master);
        buses = { master, effects, music, ambience, limiter };
        applyAll();
        return true;
    }

    // Pooled loopable noise (white / pink / brown), 2 s, mono, generated once per context.
    function noise(kind = 'white') {
        if (!ctx) return null;
        if (noises.has(kind)) return noises.get(kind);
        const rate = ctx.sampleRate, n = rate * 2, buf = ctx.createBuffer(1, n, rate), d = buf.getChannelData(0), rnd = mulberry(kind === 'pink' ? 11 : kind === 'brown' ? 23 : 5);
        let b0 = 0, b1 = 0, b2 = 0, last = 0;
        for (let i = 0; i < n; i++) {
            const w = rnd() * 2 - 1;
            if (kind === 'pink') { b0 = .99765 * b0 + w * .0990460; b1 = .96300 * b1 + w * .2965164; b2 = .57000 * b2 + w * 1.0526913; d[i] = (b0 + b1 + b2 + w * .1848) * .2; }
            else if (kind === 'brown') { last = (last + .02 * w) / 1.02; d[i] = last * 3.5; }
            else d[i] = w;
        }
        // Crossfade the seam so loops do not click.
        const f = Math.min(256, n >> 3);
        for (let i = 0; i < f; i++) { const k = (i + 1) / f; d[n - f + i] = d[n - f + i] * (1 - k) + d[i] * k; }
        noises.set(kind, buf);
        return buf;
    }

    async function loadOne(name, signal) {
        const doFetch = fetchImpl || globalThis.fetch;
        const response = await doFetch(assetURL(assetRoot + 'audio/' + name + '.ogg'), { signal });
        if (!response.ok) throw Error('audio');
        const data = await response.arrayBuffer();
        const buf = await new Promise((resolve, reject) => { const p = ctx.decodeAudioData(data, resolve, reject); if (p && p.then) p.then(resolve, reject); });
        buffers.set(name, buf);
    }
    // Loads every missing sample; a failed sample is simply absent (recipes keep their synth layer). Resolves to the loaded count.
    async function loadSamples(names = SAMPLE_NAMES, signal = null) {
        if (!ctx || disposed) return 0;
        const sig = signal || life.signal;
        await Promise.all(names.filter(n => !buffers.has(n)).map(n => {
            if (!pending.has(n)) pending.set(n, loadOne(n, sig).catch(() => {}).finally(() => pending.delete(n)));
            return pending.get(n);
        }));
        return buffers.size;
    }

    async function unlock() {
        if (disposed) return false;
        if (!ctx && !build()) return false;
        try { if (ctx.state !== 'running') await ctx.resume(); } catch { /* needs a gesture */ }
        suspendedByUs = false;
        return ctx.state === 'running';
    }
    async function resume() { if (!ctx || disposed) return false; try { await ctx.resume(); } catch { return false; } suspendedByUs = false; return ctx.state === 'running'; }
    async function suspend() { if (!ctx || disposed) return; suspendedByUs = true; try { await ctx.suspend(); } catch { /* ignore */ } }

    function setSettings(s = {}) {
        reduced = !!s.reduced;
        for (const k of ['effects', 'music', 'ambience']) if (s[k] !== undefined) vols[k] = s[k];
        applyAll();
    }

    // ---- ambience beds (subtle, per table) ----
    function startAmbience(id = tableId) {
        stopAmbience();
        if (!ctx || disposed) return;
        const out = buses.ambience, nodes = [], timers = [], rnd = mulberry(99);
        const add = n => { nodes.push(n); return n; };
        const loopNoise = (kind, type, freq, q, level) => {
            const src = add(ctx.createBufferSource()); src.buffer = noise(kind); src.loop = true;
            const f = add(ctx.createBiquadFilter()); f.type = type; f.frequency.value = freq; f.Q.value = q;
            const g = add(ctx.createGain()); g.gain.value = level; src.connect(f); f.connect(g); g.connect(out); src.start(); return { f, g };
        };
        const tone = (freq, level, type = 'sine') => { const o = add(ctx.createOscillator()); o.type = type; o.frequency.value = freq; const g = add(ctx.createGain()); g.gain.value = level; o.connect(g); g.connect(out); o.start(); return { o, g }; };
        const lfo = (target, rate, depth) => { const o = add(ctx.createOscillator()); o.frequency.value = rate; const g = add(ctx.createGain()); g.gain.value = depth; o.connect(g); g.connect(target); o.start(); };
        const every = (min, max, fn) => { const tick = () => { fn(); timers.push(setTimeout(tick, (min + rnd() * (max - min)) * 1000)); }; timers.push(setTimeout(tick, (min + rnd() * (max - min)) * 600)); };
        if (id === 'leviathan') {
            tone(38, .22); tone(57.2, .08); const rum = loopNoise('brown', 'lowpass', 140, .5, .5); lfo(rum.g.gain, .07, .18);
            every(14, 30, () => { // whale-like swell: slow gliding sine pair with vibrato
                const t = ctx.currentTime, base = 90 + rnd() * 90, dur = 5 + rnd() * 3, o = ctx.createOscillator(), v = ctx.createOscillator(), vg = ctx.createGain(), g = ctx.createGain();
                o.type = 'sine'; o.frequency.setValueAtTime(base, t); o.frequency.exponentialRampToValueAtTime(base * (rnd() < .5 ? 1.6 : .62), t + dur);
                v.frequency.value = 5; vg.gain.value = base * .02; v.connect(vg); vg.connect(o.frequency);
                g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(.09, t + dur * .4); g.gain.linearRampToValueAtTime(0, t + dur);
                o.connect(g); g.connect(out); g.connect(reverbSend); o.start(t); v.start(t); o.stop(t + dur + .1); v.stop(t + dur + .1);
            });
        } else if (id === 'temple') {
            const bugs = loopNoise('pink', 'bandpass', 4600, 6, .05); lfo(bugs.g.gain, 9, .035); lfo(bugs.g.gain, .13, .02);
            loopNoise('brown', 'lowpass', 300, .4, .18);
            every(2.5, 8, () => { // bird chirps: a short run of rising sine blips
                const t0 = ctx.currentTime, n = 2 + Math.floor(rnd() * 4), base = 2200 + rnd() * 1800, pan = ctx.createStereoPanner ? ctx.createStereoPanner() : null;
                if (pan) { pan.pan.value = rnd() * 1.6 - .8; pan.connect(out); pan.connect(reverbSend); }
                for (let i = 0; i < n; i++) {
                    const t = t0 + i * .11, o = ctx.createOscillator(), g = ctx.createGain();
                    o.type = 'sine'; o.frequency.setValueAtTime(base, t); o.frequency.exponentialRampToValueAtTime(base * 1.35, t + .06);
                    g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(.05, t + .01); g.gain.exponentialRampToValueAtTime(.0005, t + .08);
                    o.connect(g); g.connect(pan || out); o.start(t); o.stop(t + .1);
                }
            });
        } else {
            tone(55, .1); tone(110.4, .04); tone(165.2, .012, 'triangle');
            const hum = loopNoise('pink', 'lowpass', 500, .6, .09);
            const traffic = loopNoise('brown', 'bandpass', 220, .8, .0); lfo(traffic.g.gain, .05, .06);
            every(6, 16, () => { // a distant vehicle passing: noise swell that sweeps across the band
                const t = ctx.currentTime, dur = 3 + rnd() * 2, src = ctx.createBufferSource(), f = ctx.createBiquadFilter(), g = ctx.createGain();
                src.buffer = noise('brown'); src.loop = true; f.type = 'bandpass'; f.Q.value = 1.2;
                f.frequency.setValueAtTime(180, t); f.frequency.linearRampToValueAtTime(420, t + dur);
                g.gain.setValueAtTime(0, t); g.gain.linearRampToValueAtTime(.16, t + dur * .5); g.gain.linearRampToValueAtTime(0, t + dur);
                src.connect(f); f.connect(g); g.connect(out); src.start(t); src.stop(t + dur + .1);
            });
            void hum;
        }
        ambienceStop = () => { timers.forEach(clearTimeout); for (const n of nodes) { try { n.stop && n.stop(); } catch { /* not started */ } try { n.disconnect(); } catch { /* ignore */ } } };
    }
    function stopAmbience() { if (ambienceStop) { ambienceStop(); ambienceStop = null; } }

    function dispose() {
        if (disposed) return;
        disposed = true; life.abort(); stopAmbience();
        if (ctx) { try { ctx.close(); } catch { /* ignore */ } }
        buffers.clear(); noises.clear(); pending.clear();
    }
    function inspect() { return { unlocked: !!ctx && ctx.state === 'running', state: ctx ? ctx.state : 'none', buffers: buffers.size, ambience: !!ambienceStop, suspendedByUs }; }

    return {
        get ctx() { return ctx; }, get buses() { return buses; }, get reverbSend() { return reverbSend; }, get time() { return ctx ? ctx.currentTime : 0; },
        unlock, resume, suspend, setSettings, loadSamples, buffer: name => buffers.get(name) || null, noise,
        startAmbience, stopAmbience, dispose, inspect, tableId, signal: life.signal
    };
}
