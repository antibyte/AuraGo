// Procedural pinball music: pure bar expansion + conductor + lookahead scheduler (all Node-testable),
// and createMusic() which turns them into WebAudio. No AudioContext is touched at import time.
import { INSTRUMENTS } from './instruments.js';

export const STEPS_PER_BAR = 16;
export const DUCK_DB = -8;
const DUCK_GAIN = Math.pow(10, DUCK_DB / 20);

function hash01(a, b, c) {
    let h = (Math.imul(a + 1, 374761393) + Math.imul(b + 1, 668265263) + Math.imul(c + 1, 2147483629)) | 0;
    h = Math.imul(h ^ (h >>> 13), 1274126177);
    return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
}

function locate(song, section, bar) {
    const names = song.sections && song.sections[section];
    if (!names || !names.length) return null;
    let total = 0;
    for (const n of names) total += song.patterns[n].bars;
    let b = ((bar % total) + total) % total;
    const index = b;
    for (const name of names) {
        const pattern = song.patterns[name];
        if (b < pattern.bars) return { name, pattern, local: b, index, total };
        b -= pattern.bars;
    }
    return null;
}

function collect(song, tracks, from, to, out, trackOrder, tag) {
    for (const track of Object.keys(tracks)) {
        const cfg = song.instruments[track];
        if (!cfg) continue;
        for (const ev of tracks[track]) {
            if (ev[0] >= from && ev[0] < to) out.push({ step: ev[0] - from, track, voice: cfg.type, midi: ev[1], len: ev[2], vel: ev[3], order: trackOrder[track], tag });
        }
    }
}

// Pure and deterministic: the events of one bar of `section`, bar counted from the section start (it loops).
// opts.transition adds the song's transition overlay (a short fill on the first bar after a section change).
export function expandBar(song, section, bar, opts = {}) {
    const at = locate(song, section, bar);
    if (!at) return { section, pattern: null, bar, index: 0, events: [] };
    const order = {};
    Object.keys(song.instruments).forEach((k, i) => { order[k] = i; });
    const from = at.local * STEPS_PER_BAR, to = from + STEPS_PER_BAR;
    let tracks = at.pattern.tracks;
    const fill = song.fills && song.fills[section];
    if (fill && at.index === at.total - 1 && at.total > 1) {
        tracks = Object.assign({}, tracks);
        const f = song.patterns[fill].tracks;
        for (const k of Object.keys(f)) tracks[k] = f[k].map((e) => [e[0] + from, e[1], e[2], e[3]]);
    }
    const events = [];
    collect(song, tracks, from, to, events, order, 'groove');
    if (opts.transition && song.transitions) {
        const tp = song.transitions[section] || song.transitions.default;
        if (tp) collect(song, song.patterns[tp].tracks, 0, STEPS_PER_BAR, events, order, 'transition');
    }
    for (const e of events) {
        e.vel = Math.round(Math.min(1, e.vel * (0.93 + 0.1 * hash01(bar, e.step, e.order))) * 1000) / 1000;
    }
    events.sort((a, b) => a.step - b.step || a.order - b.order || a.midi - b.midi);
    return { section, pattern: at.name, bar, index: at.index, events };
}

// Length of a stinger in 16th steps (last note end, at least 1).
export function stingerLength(song, cue) {
    const p = song.patterns[song.stingers[cue]];
    let end = 1;
    for (const k of Object.keys(p.tracks)) for (const e of p.tracks[k]) end = Math.max(end, e[0] + e[2]);
    return end;
}

// Walks the global 16th step counter. Section requests apply on the next bar line; stingers start on the next step.
export function createConductor(song) {
    let current = 'off', pending = null, barCounter = 0, justSwitched = false;
    let byStep = [];
    const queued = [], active = [];
    return {
        get section() { return current; },
        get pending() { return pending; },
        get idle() { return current === 'off' && pending === null && !queued.length && !active.length; },
        request(name) {
            if (name !== 'off' && !(song.sections && song.sections[name])) return false;
            pending = name === current ? null : name;
            return true;
        },
        stinger(cue) {
            if (!song.stingers || !song.stingers[cue]) return false;
            queued.push(cue);
            return true;
        },
        step(g) {
            const barStep = ((g % STEPS_PER_BAR) + STEPS_PER_BAR) % STEPS_PER_BAR;
            const res = { step: g, barStep, boundary: barStep === 0, switched: false, section: current, events: [], stingerStarted: null };
            if (barStep === 0) {
                if (pending !== null) {
                    justSwitched = current !== 'off' && pending !== 'off';
                    current = pending; pending = null; barCounter = 0; res.switched = true;
                } else { barCounter++; justSwitched = false; }
                const bar = expandBar(song, current, barCounter, { transition: justSwitched });
                byStep = [];
                for (let i = 0; i < STEPS_PER_BAR; i++) byStep.push([]);
                for (const e of bar.events) byStep[e.step].push(e);
                res.bar = bar;
            }
            res.section = current;
            for (const e of byStep[barStep] || []) res.events.push(e);
            while (queued.length) {
                const cue = queued.shift();
                const p = song.patterns[song.stingers[cue]];
                const entry = { cue, start: g, len: stingerLength(song, cue), pattern: p };
                active.push(entry);
                res.stingerStarted = { cue, steps: entry.len };
            }
            for (let i = active.length - 1; i >= 0; i--) {
                const a = active[i], rel = g - a.start;
                for (const track of Object.keys(a.pattern.tracks)) {
                    const cfg = song.instruments[track];
                    if (!cfg) continue;
                    for (const ev of a.pattern.tracks[track]) {
                        if (ev[0] === rel) res.events.push({ step: barStep, track, voice: cfg.type, midi: ev[1], len: ev[2], vel: ev[3], tag: 'stinger' });
                    }
                }
                if (rel >= a.len) active.splice(i, 1);
            }
            return res;
        }
    };
}

// Lookahead scheduler. clock = { now() }, timer = { start(fn, ms) -> handle, stop(handle) }.
// Calls onStep(stepIndex, time) exactly once per step whose time falls inside now+horizon. Steps more than
// maxLag behind the clock (a throttled tab) are skipped, never replayed.
export function createScheduler(clock, timer, opts = {}) {
    const o = Object.assign({ stepSec: 0.125, horizon: 0.12, interval: 25, lead: 0.06, maxLag: 0.5, onStep() {} }, opts);
    let handle = null, running = false, step = 0, next = 0;
    function tick() {
        if (!running) return;
        const now = clock.now();
        while (running && next < now + o.horizon) {
            if (next < now - o.maxLag) { step++; next += o.stepSec; continue; }
            const s = step, t = next;
            step++; next += o.stepSec;
            o.onStep(s, t);
        }
    }
    return {
        start() {
            if (running) return;
            running = true; step = 0; next = clock.now() + o.lead;
            handle = timer.start(tick, o.interval);
            tick();
        },
        stop() {
            running = false;
            if (handle !== null) timer.stop(handle);
            handle = null;
        },
        tick,
        get running() { return running; },
        get step() { return step; },
        get nextTime() { return next; },
        set stepSec(v) { o.stepSec = v; }
    };
}

const defaultTimer = { start: (fn, ms) => setInterval(fn, ms), stop: (h) => clearInterval(h) };

function impulse(ctx, seconds) {
    const rate = ctx.sampleRate, len = Math.max(1, Math.floor(rate * seconds));
    const buf = ctx.createBuffer(2, len, rate);
    for (let c = 0; c < 2; c++) {
        const d = buf.getChannelData(c);
        let s = 9001 + c * 77;
        for (let i = 0; i < len; i++) {
            s = (Math.imul(s, 1664525) + 1013904223) >>> 0;
            d[i] = (s / 2147483648 - 1) * Math.pow(1 - i / len, 2.6);
        }
    }
    return buf;
}

// opts: { reduced, gain, timer, reverbSeconds }
export function createMusic(ctx, out, song, opts = {}) {
    const stepSec = 60 / song.bpm / 4;
    const swing = song.swing || 0;
    const bus = ctx.createGain();
    const base = opts.gain === undefined ? 1 : opts.gain;
    bus.gain.value = base;
    bus.connect(out);
    let convolver = null, wet = null;
    if (!opts.reduced) {
        convolver = ctx.createConvolver();
        convolver.buffer = impulse(ctx, opts.reverbSeconds || song.reverb || 1.2);
        wet = ctx.createGain();
        wet.gain.value = 0.6;
        convolver.connect(wet); wet.connect(bus);
    }
    const nodes = {};
    function trackNode(name) {
        if (nodes[name]) return nodes[name];
        const cfg = song.instruments[name];
        const input = ctx.createGain();
        input.gain.value = cfg.gain === undefined ? 0.5 : cfg.gain;
        input.connect(bus);
        let send = null;
        if (convolver && cfg.send) { send = ctx.createGain(); send.gain.value = cfg.send; input.connect(send); send.connect(convolver); }
        nodes[name] = { input, send };
        return nodes[name];
    }
    let conductor = createConductor(song), disposed = false;
    const scheduler = createScheduler({ now: () => ctx.currentTime }, opts.timer || defaultTimer, {
        stepSec,
        onStep(g, t) {
            const r = conductor.step(g);
            const t0 = Math.max(t, ctx.currentTime);
            if (r.stingerStarted) {
                const len = r.stingerStarted.steps * stepSec;
                bus.gain.cancelScheduledValues(t0);
                bus.gain.setTargetAtTime(base * DUCK_GAIN, t0, 0.03);
                bus.gain.setTargetAtTime(base, t0 + len, 0.3);
            }
            for (const ev of r.events) {
                const cfg = song.instruments[ev.track], fn = INSTRUMENTS[cfg.type];
                if (!fn) continue;
                const when = Math.max(t0, t0 + (ev.step % 2 ? swing * stepSec : 0));
                const p = Object.assign({}, cfg, { freq: 440 * Math.pow(2, (ev.midi - 69) / 12), dur: ev.len * stepSec, vel: ev.vel });
                fn(ctx, trackNode(ev.track).input, when, p);
            }
            if (conductor.idle) scheduler.stop();
        }
    });
    function wake() { if (!disposed && !scheduler.running) scheduler.start(); }
    return {
        section(name) {
            if (disposed || !conductor.request(name)) return false;
            wake();
            return true;
        },
        stinger(cue) {
            if (disposed || !conductor.stinger(cue)) return false;
            wake();
            return true;
        },
        stop() {
            scheduler.stop();
            conductor = createConductor(song);
            bus.gain.cancelScheduledValues(ctx.currentTime);
            bus.gain.setTargetAtTime(base, ctx.currentTime, 0.05);
        },
        dispose() {
            if (disposed) return;
            scheduler.stop();
            disposed = true;
            for (const k of Object.keys(nodes)) { nodes[k].input.disconnect(); if (nodes[k].send) nodes[k].send.disconnect(); }
            if (convolver) { convolver.disconnect(); wet.disconnect(); }
            bus.disconnect();
        },
        get state() { return { section: conductor.section, pending: conductor.pending, running: scheduler.running }; }
    };
}
