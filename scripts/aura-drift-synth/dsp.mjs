// Seeded float DSP toolkit for the Aura Drift audio pack.
// Pure functions over Float64Array, Math only (no Math.random, no Date): the same script gives the same samples.
export const SR = 44100;
export const TAU = Math.PI * 2;

export const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));
export const lerp = (a, b, t) => a + (b - a) * t;
export const dbToGain = db => Math.pow(10, db / 20);
export const mtof = midi => 440 * Math.pow(2, (midi - 69) / 12);
export const secs = s => Math.round(s * SR);
export const mk = n => new Float64Array(n);

// mulberry32; every recipe gets its own seed so recipes never disturb each other.
export function rng(seed) {
    let a = seed >>> 0;
    const next = () => {
        a = (a + 0x6D2B79F5) | 0;
        let t = Math.imul(a ^ (a >>> 15), 1 | a);
        t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
    next.range = (lo, hi) => lo + (hi - lo) * next();
    next.sign = () => (next() < 0.5 ? -1 : 1);
    next.gauss = () => { let s = 0; for (let i = 0; i < 6; i++) s += next(); return (s - 3) / 0.7071; };
    next.pick = list => list[Math.floor(next() * list.length) % list.length];
    return next;
}
export function seedOf(text) {
    let h = 2166136261;
    for (let i = 0; i < text.length; i++) { h ^= text.charCodeAt(i); h = Math.imul(h, 16777619); }
    return h >>> 0;
}

// ---- noise ----
export function white(n, r) { const o = mk(n); for (let i = 0; i < n; i++) o[i] = r() * 2 - 1; return o; }
export function pink(n, r) {
    const o = mk(n);
    let b0 = 0, b1 = 0, b2 = 0, b3 = 0, b4 = 0, b5 = 0, b6 = 0;
    for (let i = 0; i < n; i++) {
        const w = r() * 2 - 1;
        b0 = 0.99886 * b0 + w * 0.0555179; b1 = 0.99332 * b1 + w * 0.0750759; b2 = 0.969 * b2 + w * 0.153852;
        b3 = 0.8665 * b3 + w * 0.3104856; b4 = 0.55 * b4 + w * 0.5329522; b5 = -0.7616 * b5 - w * 0.016898;
        o[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11; b6 = w * 0.115926;
    }
    return o;
}
export function brown(n, r) {
    const o = mk(n); let s = 0;
    for (let i = 0; i < n; i++) { s = (s + (r() * 2 - 1) * 0.04) * 0.998; o[i] = s * 3.5; }
    return o;
}

// ---- biquad (RBJ) ----
export function coef(type, f, q = 0.7071, gainDb = 0) {
    f = clamp(f, 10, SR * 0.49);
    const w0 = TAU * f / SR, cos = Math.cos(w0), sin = Math.sin(w0), alpha = sin / (2 * q), A = Math.pow(10, gainDb / 40);
    let b0, b1, b2, a0, a1, a2;
    switch (type) {
        case 'lp': b0 = (1 - cos) / 2; b1 = 1 - cos; b2 = b0; a0 = 1 + alpha; a1 = -2 * cos; a2 = 1 - alpha; break;
        case 'hp': b0 = (1 + cos) / 2; b1 = -(1 + cos); b2 = b0; a0 = 1 + alpha; a1 = -2 * cos; a2 = 1 - alpha; break;
        case 'bp': b0 = alpha; b1 = 0; b2 = -alpha; a0 = 1 + alpha; a1 = -2 * cos; a2 = 1 - alpha; break;
        case 'notch': b0 = 1; b1 = -2 * cos; b2 = 1; a0 = 1 + alpha; a1 = -2 * cos; a2 = 1 - alpha; break;
        case 'peak': b0 = 1 + alpha * A; b1 = -2 * cos; b2 = 1 - alpha * A; a0 = 1 + alpha / A; a1 = -2 * cos; a2 = 1 - alpha / A; break;
        case 'lowshelf': { const s = 2 * Math.sqrt(A) * alpha; b0 = A * ((A + 1) - (A - 1) * cos + s); b1 = 2 * A * ((A - 1) - (A + 1) * cos); b2 = A * ((A + 1) - (A - 1) * cos - s); a0 = (A + 1) + (A - 1) * cos + s; a1 = -2 * ((A - 1) + (A + 1) * cos); a2 = (A + 1) + (A - 1) * cos - s; break; }
        case 'highshelf': { const s = 2 * Math.sqrt(A) * alpha; b0 = A * ((A + 1) + (A - 1) * cos + s); b1 = -2 * A * ((A - 1) + (A + 1) * cos); b2 = A * ((A + 1) + (A - 1) * cos - s); a0 = (A + 1) - (A - 1) * cos + s; a1 = 2 * ((A - 1) - (A + 1) * cos); a2 = (A + 1) - (A - 1) * cos - s; break; }
        default: throw new Error('unknown filter ' + type);
    }
    return [b0 / a0, b1 / a0, b2 / a0, a1 / a0, a2 / a0];
}
export function filter(x, type, f, q = 0.7071, gainDb = 0) {
    const [b0, b1, b2, a1, a2] = coef(type, f, q, gainDb);
    const o = mk(x.length); let z1 = 0, z2 = 0;
    for (let i = 0; i < x.length; i++) {
        const v = x[i], y = b0 * v + z1;
        z1 = b1 * v - a1 * y + z2; z2 = b2 * v - a2 * y; o[i] = y;
    }
    return o;
}
export const chain = (x, ...stages) => stages.reduce((acc, s) => filter(acc, ...s), x);

// Time-varying state-variable filter (TPT). cutoff and q may be numbers or functions of the sample index.
export function svf(x, mode, cutoff, q = 0.7071) {
    const n = x.length, o = mk(n);
    let ic1 = 0, ic2 = 0, g = 0, k = 1, a1 = 0, a2 = 0, a3 = 0;
    const fc = typeof cutoff === 'function' ? cutoff : () => cutoff;
    const qf = typeof q === 'function' ? q : () => q;
    for (let i = 0; i < n; i++) {
        if ((i & 15) === 0) {
            g = Math.tan(Math.PI * clamp(fc(i), 15, SR * 0.45) / SR); k = 1 / Math.max(0.05, qf(i));
            a1 = 1 / (1 + g * (g + k)); a2 = g * a1; a3 = g * a2;
        }
        const v3 = x[i] - ic2, v1 = a1 * ic1 + a2 * v3, v2 = ic2 + a2 * ic1 + a3 * v3;
        ic1 = 2 * v1 - ic1; ic2 = 2 * v2 - ic2;
        o[i] = mode === 'lp' ? v2 : mode === 'bp' ? v1 * k : mode === 'hp' ? x[i] - k * v1 - v2 : x[i] - k * v1;
    }
    return o;
}

// ---- oscillators ----
function polyBlep(t, dt) {
    if (t < dt) { t /= dt; return t + t - t * t - 1; }
    if (t > 1 - dt) { t = (t - 1) / dt; return t * t + t + t + 1; }
    return 0;
}
export class Osc {
    constructor(phase = 0) { this.p = phase; }
    step(dt) { this.p += dt; if (this.p >= 1) this.p -= 1; }
    saw(f) { const dt = Math.abs(f) / SR; const v = 2 * this.p - 1 - polyBlep(this.p, dt); this.step(dt); return v; }
    square(f, pw = 0.5) {
        const dt = Math.abs(f) / SR; let v = this.p < pw ? 1 : -1;
        v += polyBlep(this.p, dt); v -= polyBlep((this.p + 1 - pw) % 1, dt); this.step(dt); return v;
    }
    sine(f) { const v = Math.sin(TAU * this.p); this.step(Math.abs(f) / SR); return v; }
    tri(f) { const v = 4 * Math.abs(this.p - 0.5) - 1; this.step(Math.abs(f) / SR); return v; }
}

// Karplus-Strong string with allpass tuning. bright 0..1 shapes the excitation, pick 0..1 the pick position.
export function pluck(freq, n, r, { decay = 0.997, bright = 0.5, pick = 0.3 } = {}) {
    const D = SR / freq - 0.5, N = Math.max(2, Math.floor(D)), frac = D - N, c = (1 - frac) / (1 + frac);
    const line = new Float64Array(N);
    let lp = 0;
    const k = 0.15 + 0.85 * bright;
    for (let i = 0; i < N; i++) { lp += k * ((r() * 2 - 1) - lp); line[i] = lp; }
    const pp = Math.max(1, Math.floor(N * pick * 0.5)), tmp = Float64Array.from(line);
    for (let i = 0; i < N; i++) line[i] = tmp[i] - tmp[(i + pp) % N] * 0.7;
    let m = 0; for (let i = 0; i < N; i++) m = Math.max(m, Math.abs(line[i]));
    for (let i = 0; i < N; i++) line[i] /= (m || 1);
    const o = mk(n); let idx = 0, apIn = 0, apOut = 0;
    for (let i = 0; i < n; i++) {
        const a = line[idx], b = line[(idx + 1) % N], avg = 0.5 * (a + b);
        const y = c * (avg - apOut) + apIn; apIn = avg; apOut = y;
        line[idx] = y * decay; o[i] = a; idx = (idx + 1) % N;
    }
    return o;
}

// Sum of damped sinusoids: partials = [[ratio, amp, decaySeconds], ...]
export function modal(f0, partials, n, { attack = 0.0005, jitter = 0, r = null } = {}) {
    const o = mk(n);
    for (const [ratio, amp, decay] of partials) {
        const f = f0 * ratio * (1 + (r ? (r() - 0.5) * jitter : 0));
        if (f > SR * 0.48) continue;
        const w = TAU * f / SR, ph = r ? r() * TAU : 0, tau = Math.max(0.002, decay) * SR / 6.9;
        const len = Math.min(n, Math.floor(tau * 8));
        for (let i = 0; i < len; i++) o[i] += amp * Math.sin(w * i + ph) * Math.exp(-i / tau) * (i < attack * SR ? i / (attack * SR) : 1);
    }
    return o;
}

// ---- envelopes and buffer arithmetic ----
export function envelope(n, points) { // points: [[timeSec, value], ...] linear segments
    const o = mk(n);
    for (let s = 0; s < points.length - 1; s++) {
        const [t0, v0] = points[s], [t1, v1] = points[s + 1];
        const i0 = Math.round(t0 * SR), i1 = Math.min(n, Math.round(t1 * SR));
        for (let i = Math.max(0, i0); i < i1; i++) o[i] = v0 + (v1 - v0) * (i - i0) / Math.max(1, i1 - i0);
    }
    const last = points[points.length - 1];
    for (let i = Math.max(0, Math.round(last[0] * SR)); i < n; i++) o[i] = last[1];
    return o;
}
export const expDecay = (n, tauSec, delay = 0) => { const o = mk(n), d = Math.round(delay * SR); for (let i = d; i < n; i++) o[i] = Math.exp(-(i - d) / (tauSec * SR)); return o; };
export function mul(a, b) { const n = Math.min(a.length, b.length), o = mk(a.length); for (let i = 0; i < n; i++) o[i] = a[i] * b[i]; return o; }
export function scale(a, g) { const o = mk(a.length); for (let i = 0; i < a.length; i++) o[i] = a[i] * g; return o; }
export function add(out, src, at = 0, gain = 1) {
    const s0 = Math.round(at * SR);
    for (let i = 0; i < src.length; i++) { const j = s0 + i; if (j >= 0 && j < out.length) out[j] += src[i] * gain; }
    return out;
}
export function addWrap(out, src, atSample, gain = 1) { // circular add for loops
    const n = out.length;
    for (let i = 0; i < src.length; i++) out[((atSample + i) % n + n) % n] += src[i] * gain;
    return out;
}
export const sum = (n, list) => { const o = mk(n); for (const b of list) for (let i = 0; i < Math.min(n, b.length); i++) o[i] += b[i]; return o; };
export function peak(x) { let p = 0; for (let i = 0; i < x.length; i++) { const a = Math.abs(x[i]); if (a > p) p = a; } return p; }
export function rms(x) { let s = 0; for (let i = 0; i < x.length; i++) s += x[i] * x[i]; return Math.sqrt(s / Math.max(1, x.length)); }
export function fadeEdges(x, inSec = 0.002, outSec = 0.004) {
    const a = Math.round(inSec * SR), b = Math.round(outSec * SR), n = x.length;
    for (let i = 0; i < a && i < n; i++) x[i] *= i / a;
    for (let i = 0; i < b && i < n; i++) x[n - 1 - i] *= i / b;
    return x;
}
export function softClip(x, drive = 1, mix = 1) {
    const o = mk(x.length), norm = Math.tanh(drive) || 1;
    for (let i = 0; i < x.length; i++) o[i] = (1 - mix) * x[i] + mix * Math.tanh(x[i] * drive) / norm;
    return o;
}
export function hardClipAsym(x, drive, bias = 0.15) {
    const o = mk(x.length);
    for (let i = 0; i < x.length; i++) o[i] = Math.tanh(x[i] * drive + bias) - Math.tanh(bias);
    return o;
}
export function dcBlock(x) { const o = mk(x.length); let px = 0, py = 0; for (let i = 0; i < x.length; i++) { py = x[i] - px + 0.995 * py; px = x[i]; o[i] = py; } return o; }
export function reverseBuf(x) { const o = mk(x.length); for (let i = 0; i < x.length; i++) o[i] = x[x.length - 1 - i]; return o; }
export function resampleLinear(x, ratio, n = Math.floor(x.length / ratio)) {
    const o = mk(n);
    for (let i = 0; i < n; i++) { const p = i * ratio, j = Math.floor(p), f = p - j; o[i] = (x[j] ?? 0) * (1 - f) + (x[j + 1] ?? 0) * f; }
    return o;
}
export function fixedLength(x, n) { const o = mk(n); o.set(x.subarray(0, Math.min(n, x.length))); return o; }

// Equal-power crossfade loop: out[i] = x[i] blended with x[L+i] in the first xf samples; length n - xf.
export function loopify(x, xfSec) {
    const xf = Math.round(xfSec * SR), L = x.length - xf, o = mk(L);
    for (let i = 0; i < L; i++) o[i] = x[i];
    for (let i = 0; i < xf; i++) { const th = Math.PI / 2 * (i + 0.5) / xf; o[i] = x[i] * Math.sin(th) + x[L + i] * Math.cos(th); }
    return o;
}
// Snap a modulation rate so that it completes a whole number of cycles within a loop of n samples.
export function loopHz(n, hz) { const cycles = Math.max(1, Math.round(hz * n / SR)); return cycles * SR / n; }

// ---- time effects ----
export function pan(x, p = 0) { // p -1 .. 1, equal power
    const a = (p + 1) * Math.PI / 4, l = mk(x.length), rr = mk(x.length), gl = Math.cos(a), gr = Math.sin(a);
    for (let i = 0; i < x.length; i++) { l[i] = x[i] * gl; rr[i] = x[i] * gr; }
    return [l, rr];
}
export function stereoSum(n, parts) { const L = mk(n), R = mk(n); for (const [l, r] of parts) for (let i = 0; i < Math.min(n, l.length); i++) { L[i] += l[i]; R[i] += r[i]; } return [L, R]; }

// Freeverb-style stereo reverb. size 0..1, damp 0..1.
export function reverb([inL, inR], { size = 0.6, damp = 0.4, wet = 0.25, tail = 0, width = 1, predelay = 0 } = {}) {
    const n = inL.length + Math.round(tail * SR), spread = 23;
    const combT = [1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617], apT = [556, 441, 341, 225];
    const fb = 0.7 + size * 0.28, pd = Math.round(predelay * SR);
    const make = off => ({
        combs: combT.map(t => ({ b: new Float64Array(t + off), i: 0, s: 0 })),
        aps: apT.map(t => ({ b: new Float64Array(t + off), i: 0 }))
    });
    const chans = [make(0), make(spread)], outs = [mk(n), mk(n)], ins = [inL, inR];
    for (let c = 0; c < 2; c++) {
        const { combs, aps } = chans[c], out = outs[c], src = ins[c];
        for (let i = 0; i < n; i++) {
            const k = i - pd, x = (k >= 0 && k < src.length ? src[k] : 0) * 0.015;
            let acc = 0;
            for (const cb of combs) {
                const y = cb.b[cb.i]; cb.s = y * (1 - damp) + cb.s * damp; cb.b[cb.i] = x + cb.s * fb;
                if (++cb.i >= cb.b.length) cb.i = 0; acc += y;
            }
            for (const ap of aps) {
                const bufout = ap.b[ap.i], y = -acc + bufout; ap.b[ap.i] = acc + bufout * 0.5;
                if (++ap.i >= ap.b.length) ap.i = 0; acc = y;
            }
            out[i] = acc;
        }
    }
    const L = mk(n), R = mk(n);
    for (let i = 0; i < n; i++) {
        const dl = i < inL.length ? inL[i] : 0, dr = i < inR.length ? inR[i] : 0;
        const wl = outs[0][i] * (0.5 + 0.5 * width) + outs[1][i] * (0.5 - 0.5 * width), wr = outs[1][i] * (0.5 + 0.5 * width) + outs[0][i] * (0.5 - 0.5 * width);
        L[i] = dl + wl * wet * 3; R[i] = dr + wr * wet * 3;
    }
    return [L, R];
}
export function monoReverb(x, opts) { const [l, r] = reverb([x, x], opts); const o = mk(l.length); for (let i = 0; i < o.length; i++) o[i] = 0.5 * (l[i] + r[i]); return o; }

// Run a stereo processor on a three-fold tiling and keep the middle copy: effect tails wrap round a loop.
export function loopProcess([L, R], fn) {
    const n = L.length, t = a => { const o = mk(n * 3); o.set(a, 0); o.set(a, n); o.set(a, 2 * n); return o; };
    const [pl, pr] = fn([t(L), t(R)]);
    return [pl.slice(n, 2 * n), pr.slice(n, 2 * n)];
}
export function pingPong([L, R], { time = 0.375, feedback = 0.4, mix = 0.3, damp = 2800 } = {}) {
    const d = Math.round(time * SR), n = L.length, oL = Float64Array.from(L), oR = Float64Array.from(R);
    const bl = mk(n), br = mk(n); let lpl = 0, lpr = 0;
    const a = 1 - Math.exp(-TAU * damp / SR);
    for (let i = 0; i < n; i++) {
        const il = i >= d ? br[i - d] : 0, ir = i >= d ? bl[i - d] : 0;
        lpl += a * (il - lpl); lpr += a * (ir - lpr);
        bl[i] = (L[i] + R[i]) * 0.5 * (i % 2 === 0 ? 1 : 1) + lpl * feedback;
        br[i] = lpr * feedback;
        oL[i] += il * mix; oR[i] += ir * mix;
    }
    return [oL, oR];
}
export function echo(x, { time = 0.25, feedback = 0.4, mix = 0.4, damp = 3500, tail = 0.6 } = {}) {
    const d = Math.round(time * SR), n = x.length + Math.round(tail * SR), o = mk(n), line = mk(n); let lp = 0;
    const a = 1 - Math.exp(-TAU * damp / SR);
    for (let i = 0; i < n; i++) {
        const dry = i < x.length ? x[i] : 0; const del = i >= d ? line[i - d] : 0;
        lp += a * (del - lp); line[i] = dry + lp * feedback; o[i] = dry + del * mix;
    }
    return o;
}
// Mono to stereo ensemble: three modulated taps per side. rate should be loopHz-snapped for loops.
export function chorus(x, { rate = 0.4, depth = 0.003, base = 0.012, mix = 0.5 } = {}) {
    const n = x.length, L = mk(n), R = mk(n), taps = [[0, 1], [0.33, 1.13], [0.66, 0.87]];
    const read = (i, d) => { const p = i - d; if (p < 0) return 0; const j = Math.floor(p), f = p - j; return x[j] * (1 - f) + (x[j + 1] ?? x[j]) * f; };
    for (let i = 0; i < n; i++) {
        let l = 0, rr = 0;
        for (const [ph, rt] of taps) {
            const a = Math.sin(TAU * (rate * rt * i / SR + ph)), b = Math.sin(TAU * (rate * rt * i / SR + ph + 0.5));
            l += read(i, (base + depth * a) * SR); rr += read(i, (base + depth * b) * SR);
        }
        L[i] = x[i] * (1 - mix) + l / 3 * mix * 1.2; R[i] = x[i] * (1 - mix) + rr / 3 * mix * 1.2;
    }
    return [L, R];
}
// Feed-forward compressor on the stereo sum; warm == true runs the loop twice so the envelope state wraps.
export function compress([L, R], { threshold = -14, ratio = 3, attack = 0.005, release = 0.12, makeup = 0, warm = true } = {}) {
    const n = L.length, th = dbToGain(threshold), aa = Math.exp(-1 / (attack * SR)), ar = Math.exp(-1 / (release * SR));
    const oL = mk(n), oR = mk(n); let env = 0;
    for (let pass = warm ? 0 : 1; pass < 2; pass++) {
        for (let i = 0; i < n; i++) {
            const lvl = Math.max(Math.abs(L[i]), Math.abs(R[i]));
            env = lvl > env ? aa * env + (1 - aa) * lvl : ar * env + (1 - ar) * lvl;
            let g = 1; if (env > th) g = Math.pow(env / th, 1 / ratio - 1);
            if (pass === 1) { const mg = g * dbToGain(makeup); oL[i] = L[i] * mg; oR[i] = R[i] * mg; }
        }
    }
    return [oL, oR];
}
// Gain curve ducking after each hit time (seconds); wraps round n samples.
export function duckEnv(n, hits, { depth = 0.7, release = 0.18, attack = 0.004, shape = 1.6 } = {}) {
    const env = new Float64Array(n).fill(1), a = Math.round(attack * SR), rel = Math.round(release * 5 * SR);
    for (const h of hits) {
        const s0 = Math.round(h * SR);
        for (let k = 0; k < a + rel; k++) {
            const g = k < a ? 1 - depth * (k / a) : 1 - depth * Math.exp(-Math.pow((k - a) / (release * SR), shape * 0.65));
            const j = (s0 + k) % n; if (g < env[j]) env[j] = g;
        }
    }
    return env;
}
export function applyEnv(x, env) { const o = mk(x.length); for (let i = 0; i < x.length; i++) o[i] = x[i] * env[i % env.length]; return o; }

// ---- vocal formant source ----
// f0(i) in Hz per sample; formants(i) returns [[freq, q, gain], ...]. A glottal pulse train with jitter feeds parallel resonators.
export function formantVoice(n, f0, formants, r, { jitter = 0.01, shimmer = 0.05, breath = 0.04, amp = null, tilt = 2600, pulse = 'saw' } = {}) {
    const src = mk(n), osc = new Osc(r()); let jit = 0, shim = 1;
    for (let i = 0; i < n; i++) {
        if ((i & 255) === 0) { jit = (r() - 0.5) * 2 * jitter; shim = 1 + (r() - 0.5) * 2 * shimmer; }
        const f = f0(i) * (1 + jit);
        const v = pulse === 'square' ? osc.square(f, 0.3) : osc.saw(f);
        src[i] = v * shim + (r() * 2 - 1) * breath;
    }
    const tilted = filter(src, 'lp', tilt, 0.6);
    const bank = formants(0).map(() => ({ ic1: 0, ic2: 0 })), out = mk(n);
    let coefs = null;
    for (let i = 0; i < n; i++) {
        if ((i & 31) === 0) {
            const fm = formants(i);
            coefs = fm.map(([f, q, g]) => { const gg = Math.tan(Math.PI * clamp(f, 80, SR * 0.45) / SR), k = 1 / q, a1 = 1 / (1 + gg * (gg + k)); return { gg, k, a1, a2: gg * a1, a3: gg * gg * a1, g }; });
        }
        let acc = 0; const x = tilted[i];
        for (let b = 0; b < bank.length; b++) {
            const c = coefs[b], s = bank[b], v3 = x - s.ic2, v1 = c.a1 * s.ic1 + c.a2 * v3, v2 = s.ic2 + c.a2 * s.ic1 + c.a3 * v3;
            s.ic1 = 2 * v1 - s.ic1; s.ic2 = 2 * v2 - s.ic2; acc += v1 * c.k * c.g;
        }
        out[i] = acc * (amp ? amp(i) : 1);
    }
    return out;
}

// Convolve with a short impulse response (direct, for IRs up to a few thousand taps).
export function convolve(x, ir, tail = true) {
    const n = x.length + (tail ? ir.length : 0), o = mk(n);
    for (let i = 0; i < x.length; i++) { const v = x[i]; if (v === 0) continue; for (let k = 0; k < ir.length && i + k < n; k++) o[i + k] += v * ir[k]; }
    return o;
}

// Resample a layer (pitch shift by playback ratio), used for variation of the same recipe.
export function pitchShift(x, ratio) { return resampleLinear(x, ratio); }

// ---- output ----
export function toWavF32(channels, sampleRate = SR) {
    const n = channels[0].length, ch = channels.length, data = Buffer.alloc(44 + n * ch * 4);
    data.write('RIFF', 0); data.writeUInt32LE(36 + n * ch * 4, 4); data.write('WAVEfmt ', 8); data.writeUInt32LE(16, 16);
    data.writeUInt16LE(3, 20); data.writeUInt16LE(ch, 22); data.writeUInt32LE(sampleRate, 24); data.writeUInt32LE(sampleRate * ch * 4, 28);
    data.writeUInt16LE(ch * 4, 32); data.writeUInt16LE(32, 34); data.write('data', 36); data.writeUInt32LE(n * ch * 4, 40);
    let off = 44;
    for (let i = 0; i < n; i++) for (let c = 0; c < ch; c++) { data.writeFloatLE(channels[c][i], off); off += 4; }
    return data;
}
