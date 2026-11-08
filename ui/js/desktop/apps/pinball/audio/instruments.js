// WebAudio voice builders for the pinball music. Every voice is `(ctx, dest, t, p) => void` with
// p = { freq, dur, vel, ...instrument params }. Nodes are short lived and disconnect when the last source ends.
// Nothing here touches an AudioContext at import time. Noise is a fixed LCG (no Math.random).
const noiseCache = new WeakMap();
const FLOOR = 0.0001;

function noiseBuffer(ctx) {
    let buf = noiseCache.get(ctx);
    if (!buf) {
        buf = ctx.createBuffer(1, ctx.sampleRate, ctx.sampleRate);
        const d = buf.getChannelData(0);
        let s = 22222;
        for (let i = 0; i < d.length; i++) { s = (Math.imul(s, 1664525) + 1013904223) >>> 0; d[i] = s / 2147483648 - 1; }
        noiseCache.set(ctx, buf);
    }
    return buf;
}
function osc(ctx, type, freq, t, stop, detune) {
    const o = ctx.createOscillator();
    o.type = type;
    o.frequency.setValueAtTime(freq, t);
    if (detune) o.detune.value = detune;
    o.start(t);
    o.stop(stop);
    return o;
}
function noise(ctx, t, stop) {
    const s = ctx.createBufferSource();
    s.buffer = noiseBuffer(ctx);
    s.loop = true;
    s.start(t, (t * 7.31) % 0.9);
    s.stop(stop);
    return s;
}
function filt(ctx, type, freq, q) {
    const f = ctx.createBiquadFilter();
    f.type = type;
    f.frequency.value = freq;
    if (q !== undefined) f.Q.value = q;
    return f;
}
// Attack to peak, exponential fall to peak*sus at `hold`, release to silence. Returns the end time.
function amp(g, t, peak, a, hold, rel, sus = 1) {
    const p = g.gain;
    const h = Math.max(hold, a + 0.02);
    p.setValueAtTime(FLOOR, t);
    p.linearRampToValueAtTime(Math.max(peak, FLOOR * 2), t + a);
    p.exponentialRampToValueAtTime(Math.max(peak * sus, FLOOR * 2), t + h);
    p.exponentialRampToValueAtTime(FLOOR, t + h + rel);
    return t + h + rel + 0.03;
}
function finish(sources, nodes) {
    let left = sources.length;
    for (const s of sources) {
        s.onended = () => {
            if (--left > 0) return;
            for (const n of nodes) { try { n.disconnect(); } catch (e) { /* already gone */ } }
        };
    }
}
const vel = (p) => Math.max(0.05, Math.min(1, p.vel === undefined ? 0.8 : p.vel));

export function sawBass(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), f = filt(ctx, 'lowpass', 260, 4);
    const end = amp(g, t, 0.55 * v, 0.006, Math.max(0.1, p.dur * 0.9), 0.07, 0.7);
    const cut = p.cutoff || 1500;
    f.frequency.setValueAtTime(cut * (0.5 + 0.5 * v), t);
    f.frequency.exponentialRampToValueAtTime(Math.max(160, cut * 0.18), t + Math.min(0.3, p.dur));
    const a = osc(ctx, 'sawtooth', p.freq, t, end), b = osc(ctx, 'sine', p.freq / 2, t, end);
    const sub = ctx.createGain();
    sub.gain.value = p.sub === undefined ? 0.7 : p.sub;
    a.connect(f); f.connect(g); b.connect(sub); sub.connect(g); g.connect(dest);
    finish([a, b], [a, b, f, sub, g]);
}
export function squareArp(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), f = filt(ctx, 'lowpass', p.cutoff || 3200, 1.2);
    const hold = p.sustain ? p.dur : Math.min(p.dur, 0.14);
    const end = amp(g, t, 0.22 * v, 0.004, hold, p.sustain ? 0.12 : 0.05, p.sustain ? 0.8 : 0.4);
    const a = osc(ctx, 'square', p.freq, t, end, -7), b = osc(ctx, 'square', p.freq, t, end, 7);
    const srcs = [a, b], nodes = [a, b, f, g];
    if (p.sustain) { // delayed vibrato for held lead notes
        const lfo = osc(ctx, 'sine', 5.5, t, end), d = ctx.createGain();
        d.gain.setValueAtTime(0, t);
        d.gain.linearRampToValueAtTime(14, t + Math.min(0.4, p.dur));
        lfo.connect(d); d.connect(a.detune); d.connect(b.detune);
        srcs.push(lfo); nodes.push(lfo, d);
    }
    a.connect(f); b.connect(f); f.connect(g); g.connect(dest);
    finish(srcs, nodes);
}
export function pad(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), f = filt(ctx, 'lowpass', p.cutoff || 1100, 0.7);
    const a = Math.min(p.attack || 0.35, p.dur * 0.5), end = amp(g, t, 0.2 * v, a, p.dur, p.release || 0.6, 0.85);
    f.frequency.setValueAtTime((p.cutoff || 1100) * 0.5, t);
    f.frequency.linearRampToValueAtTime(p.cutoff || 1100, t + p.dur * 0.6);
    const oscs = [-12, 0, 12].map((c) => osc(ctx, 'sawtooth', p.freq, t, end, c));
    for (const o of oscs) o.connect(f);
    f.connect(g); g.connect(dest);
    finish(oscs, [...oscs, f, g]);
}
export function pluck(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), f = filt(ctx, 'lowpass', 600, 2);
    const end = amp(g, t, 0.34 * v, 0.003, Math.min(0.5, 0.12 + p.dur * 0.3), 0.12, p.ring || 0.12);
    f.frequency.setValueAtTime((p.bright || 5000) * v, t);
    f.frequency.exponentialRampToValueAtTime(400, t + 0.25);
    const a = osc(ctx, 'sawtooth', p.freq, t, end), b = osc(ctx, 'triangle', p.freq * 2, t, end, 4);
    a.connect(f); b.connect(f); f.connect(g); g.connect(dest);
    finish([a, b], [a, b, f, g]);
}
export function marimba(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), life = p.decay || 0.45;
    const end = amp(g, t, 0.5 * v, 0.002, life, 0.08, 0.02);
    const car = osc(ctx, 'sine', p.freq, t, end), mod = osc(ctx, 'sine', p.freq * 3.99, t, end);
    const mg = ctx.createGain();
    mg.gain.setValueAtTime(p.freq * 1.7 * v, t);
    mg.gain.exponentialRampToValueAtTime(p.freq * 0.02, t + 0.12);
    mod.connect(mg); mg.connect(car.frequency);
    const click = osc(ctx, 'sine', p.freq * 9.2, t, t + 0.04), cg = ctx.createGain();
    cg.gain.setValueAtTime(0.06 * v, t);
    cg.gain.exponentialRampToValueAtTime(FLOOR, t + 0.03);
    click.connect(cg); cg.connect(g); car.connect(g); g.connect(dest);
    finish([car, mod, click], [car, mod, click, mg, cg, g]);
}
export function flute(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain();
    const end = amp(g, t, 0.34 * v, Math.min(0.07, p.dur * 0.4), p.dur, 0.18, 0.85);
    const a = osc(ctx, 'sine', p.freq, t, end), b = osc(ctx, 'triangle', p.freq * 2, t, end);
    const bg = ctx.createGain();
    bg.gain.value = 0.12;
    b.connect(bg); bg.connect(g);
    const lfo = osc(ctx, 'sine', 5.2, t, end), lg = ctx.createGain();
    lg.gain.setValueAtTime(0, t);
    lg.gain.linearRampToValueAtTime(p.vibrato === undefined ? 9 : p.vibrato, t + Math.min(0.35, p.dur));
    lfo.connect(lg); lg.connect(a.detune); lg.connect(b.detune);
    const n = noise(ctx, t, end), nf = filt(ctx, 'bandpass', p.freq * 2, 3), ng = ctx.createGain();
    ng.gain.setValueAtTime(0.5 * v, t);
    ng.gain.exponentialRampToValueAtTime(0.05 * v, t + 0.12);
    n.connect(nf); nf.connect(ng); ng.connect(g);
    a.connect(g); g.connect(dest);
    finish([a, b, lfo, n], [a, b, lfo, n, bg, lg, nf, ng, g]);
}
export function subDrone(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain();
    const end = amp(g, t, 0.5 * v, Math.min(1.2, p.dur * 0.5), p.dur, 1.2, 0.9);
    const a = osc(ctx, 'sine', p.freq, t, end), b = osc(ctx, 'sine', p.freq * 2.003, t, end);
    const bg = ctx.createGain();
    bg.gain.value = 0.18;
    const lfo = osc(ctx, 'sine', 0.13, t, end), lg = ctx.createGain();
    lg.gain.value = 0.1;
    lfo.connect(lg); lg.connect(bg.gain);
    a.connect(g); b.connect(bg); bg.connect(g); g.connect(dest);
    finish([a, b, lfo], [a, b, lfo, bg, lg, g]);
}
// Sonar ping: a sine blip into its own feedback delay (voice-local, torn down after the tail).
export function ping(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain();
    const end = amp(g, t, 0.36 * v, 0.004, 0.5, 0.1, 0.05);
    const a = osc(ctx, 'sine', p.freq, t, end), b = osc(ctx, 'sine', p.freq * 1.5, t, t + 0.12);
    const bg = ctx.createGain();
    bg.gain.value = 0.12;
    b.connect(bg); bg.connect(g); a.connect(g);
    const echo = p.echo || 0.36;
    const d = ctx.createDelay(1.5), fb = ctx.createGain(), lp = filt(ctx, 'lowpass', 2400);
    d.delayTime.value = echo;
    fb.gain.value = p.feedback || 0.5;
    g.connect(d); d.connect(lp); lp.connect(fb); fb.connect(d); lp.connect(dest); g.connect(dest);
    // The echo tail outlives the oscillators: a silent keep-alive source tears the loop down afterwards.
    const keep = osc(ctx, 'sine', 20, t, t + echo * 7), kg = ctx.createGain();
    kg.gain.value = 0;
    keep.connect(kg); kg.connect(g);
    finish([a, b, keep], [a, b, keep, bg, kg, g, d, fb, lp]);
}
export function kick(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain();
    const end = amp(g, t, 0.95 * v, 0.002, 0.2, 0.15, 0.2);
    const o = osc(ctx, 'sine', p.pitch || 150, t, end);
    o.frequency.exponentialRampToValueAtTime(p.low || 46, t + 0.11);
    const click = osc(ctx, 'triangle', 900, t, t + 0.02), cg = ctx.createGain();
    cg.gain.setValueAtTime(0.3 * v, t);
    cg.gain.exponentialRampToValueAtTime(FLOOR, t + 0.015);
    click.connect(cg); cg.connect(g); o.connect(g); g.connect(dest);
    finish([o, click], [o, click, cg, g]);
}
export function snare(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), end = amp(g, t, 0.7 * v, 0.001, 0.09, 0.12, 0.25);
    const n = noise(ctx, t, end), hp = filt(ctx, 'highpass', 1500), bp = filt(ctx, 'peaking', 3000);
    const body = osc(ctx, 'triangle', p.tone || 190, t, t + 0.12), bg = ctx.createGain();
    bg.gain.setValueAtTime(0.5 * v, t);
    bg.gain.exponentialRampToValueAtTime(FLOOR, t + 0.1);
    n.connect(hp); hp.connect(bp); bp.connect(g); body.connect(bg); bg.connect(g); g.connect(dest);
    finish([n, body], [n, body, hp, bp, bg, g]);
}
export function hat(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), open = p.dur > 0.2;
    const end = amp(g, t, 0.4 * v, 0.001, open ? 0.2 : 0.04, 0.03, 0.1);
    const n = noise(ctx, t, end), hp = filt(ctx, 'highpass', p.cutoff || 7200), bp = filt(ctx, 'peaking', 10000);
    n.connect(hp); hp.connect(bp); bp.connect(g); g.connect(dest);
    finish([n], [n, hp, bp, g]);
}
export function clap(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), bp = filt(ctx, 'bandpass', 1300, 1.4);
    const n = noise(ctx, t, t + 0.4);
    g.gain.setValueAtTime(FLOOR, t);
    for (const k of [0, 0.011, 0.023]) {
        g.gain.linearRampToValueAtTime(0.65 * v, t + k + 0.001);
        g.gain.exponentialRampToValueAtTime(0.1 * v, t + k + 0.01);
    }
    g.gain.linearRampToValueAtTime(0.5 * v, t + 0.026);
    g.gain.exponentialRampToValueAtTime(FLOOR, t + 0.24);
    n.connect(bp); bp.connect(g); g.connect(dest);
    finish([n], [n, bp, g]);
}
export function tom(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), end = amp(g, t, 0.75 * v, 0.002, p.decay || 0.28, 0.1, 0.1);
    const o = osc(ctx, 'sine', p.freq * 1.5, t, end);
    o.frequency.exponentialRampToValueAtTime(p.freq, t + 0.07);
    const sk = osc(ctx, 'triangle', p.freq * 2.4, t, t + 0.05), sg = ctx.createGain();
    sg.gain.setValueAtTime(0.14 * v, t);
    sg.gain.exponentialRampToValueAtTime(FLOOR, t + 0.04);
    sk.connect(sg); sg.connect(g); o.connect(g); g.connect(dest);
    finish([o, sk], [o, sk, sg, g]);
}
export function shaker(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), end = amp(g, t, 0.3 * v, 0.012, 0.03, 0.04, 0.3);
    const n = noise(ctx, t, end), bp = filt(ctx, 'bandpass', p.cutoff || 5200, 1.1);
    n.connect(bp); bp.connect(g); g.connect(dest);
    finish([n], [n, bp, g]);
}
export function logDrum(ctx, dest, t, p) {
    const v = vel(p), g = ctx.createGain(), end = amp(g, t, 0.85 * v, 0.003, p.decay || 0.3, 0.1, 0.05);
    const o = osc(ctx, 'sine', p.freq * 1.18, t, end);
    o.frequency.exponentialRampToValueAtTime(p.freq, t + 0.05);
    const h = osc(ctx, 'triangle', p.freq * 2, t, t + 0.12), hg = ctx.createGain();
    hg.gain.setValueAtTime(0.25 * v, t);
    hg.gain.exponentialRampToValueAtTime(FLOOR, t + 0.1);
    h.connect(hg); hg.connect(g); o.connect(g); g.connect(dest);
    finish([o, h], [o, h, hg, g]);
}

export const INSTRUMENTS = Object.freeze({
    sawBass, squareArp, pad, pluck, marimba, flute, subDrone, ping, kick, snare, hat, clap, tom, shaker, logDrum, log: logDrum
});
