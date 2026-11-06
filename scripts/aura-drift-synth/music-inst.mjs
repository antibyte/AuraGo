// Instruments for the biome scores: drum synthesis, bass voices, pads, plucks, bells, reeds, flutes, voices.
// Every voice returns a mono buffer (tail included) for one note; the arranger places it on the loop.
import {
    SR, TAU, mk, secs, filter, svf, Osc, add, scale, sum, white, softClip, modal, pluck, thump, noiseBurst, mtof, formantVoice, mixVowel, clamp, tone, SAW_H
} from './kit.mjs';

const env = (n, a, d, s, r, len) => { // ADSR-ish gain curve for a note of `len` seconds inside a buffer of n samples
    const o = mk(n);
    for (let i = 0; i < n; i++) {
        const t = i / SR;
        o[i] = t < a ? t / a : t < a + d ? 1 - (1 - s) * (t - a) / d : t < len ? s : Math.max(0, s * (1 - (t - len) / r));
    }
    return o;
};

// ---- drums ----
export function kick(type, r) {
    switch (type) {
        case 'deep': return sum(secs(0.95), [thump(0.95, 118, 41, 0.05, 0.36, 1.6)]);
        case 'hard': return sum(secs(0.5), [softClip(thump(0.5, 210, 49, 0.022, 0.13, 1), 3.2), scale(noiseBurst(r, 0.03, 0.004, { hp: 1800 }), 0.35)]);
        case 'soft': return sum(secs(0.55), [thump(0.55, 124, 54, 0.04, 0.19, 1), scale(noiseBurst(r, 0.02, 0.003, { bp: 1500 }), 0.1)]);
        case 'heart': return sum(secs(0.7), [thump(0.7, 98, 46, 0.07, 0.24, 1.1)]);
        default: return sum(secs(0.5), [thump(0.5, 170, 47, 0.03, 0.17, 1.2), scale(noiseBurst(r, 0.025, 0.004, { hp: 2200 }), 0.3)]);
    }
}
export function snare(type, r) {
    const body = f => thump(0.25, f * 1.4, f, 0.012, 0.045);
    switch (type) {
        case 'gated': { const x = sum(secs(0.4), [scale(noiseBurst(r, 0.4, 0.14, { bp: 2400, q: 0.7 }), 1), body(210), scale(noiseBurst(r, 0.1, 0.02, { hp: 5000 }), 0.4)]); for (let i = Math.round(0.19 * SR); i < x.length; i++) x[i] *= Math.max(0, 1 - (i - 0.19 * SR) / (0.025 * SR)); return x; }
        case 'brush': return sum(secs(0.3), [scale(noiseBurst(r, 0.3, 0.09, { bp: 4200, q: 0.5 }), 0.7), scale(noiseBurst(r, 0.12, 0.03, { hp: 2000 }), 0.3)]);
        case 'rim': return sum(secs(0.15), [modal(870, [[1, 1, 0.05], [2.4, 0.5, 0.03], [4.1, 0.2, 0.02]], secs(0.15)), scale(noiseBurst(r, 0.02, 0.003, { bp: 3000, q: 1.5 }), 0.6)]);
        case 'clap': { const o = mk(secs(0.45)); [0, 0.011, 0.023, 0.04].forEach((at, k) => add(o, noiseBurst(r, k === 3 ? 0.35 : 0.04, k === 3 ? 0.09 : 0.012, { bp: 1500, q: 0.9 }), at, 0.6 + 0.1 * k)); return o; }
        case 'hard': return softClip(sum(secs(0.3), [scale(noiseBurst(r, 0.25, 0.07, { bp: 2000, q: 0.6 }), 1.2), body(185), scale(noiseBurst(r, 0.12, 0.015, { hp: 6000 }), 0.5)]), 2.2);
        default: return sum(secs(0.3), [scale(noiseBurst(r, 0.25, 0.065, { bp: 2300, q: 0.8 }), 1), body(200), scale(noiseBurst(r, 0.08, 0.012, { hp: 6500 }), 0.3)]);
    }
}
const HAT_RATIOS = [2, 3, 4.16, 5.43, 6.79, 8.21];
export function hat(open, r, bright = 1) {
    const len = open ? 0.38 : 0.1, n = secs(len), o = mk(n), tau = open ? 0.11 : 0.022, osc = HAT_RATIOS.map((m, k) => new Osc(k * 0.13));
    for (let i = 0; i < n; i++) { let v = 0; for (let k = 0; k < 6; k++) v += osc[k].square(40 * m(k) * bright); o[i] = v * Math.exp(-i / (tau * SR)) * Math.min(1, i / 20); }
    function m(k) { return HAT_RATIOS[k]; }
    const x = filter(filter(o, 'hp', 7200, 0.8), 'bp', 10500, 0.6);
    const nz = noiseBurst(r, len, tau, { hp: 6500 });
    return sum(n, [scale(x, 0.45), scale(nz, 0.6)]);
}
export const shaker = (r, open = false) => noiseBurst(r, open ? 0.14 : 0.07, open ? 0.05 : 0.022, { bp: 6500, q: 0.6, attack: open ? 0.012 : 0.004 });
export const tamb = r => sum(secs(0.25), [noiseBurst(r, 0.2, 0.05, { hp: 5500 }), scale(modal(7400, [[1, 1, 0.1], [1.43, 0.6, 0.08]], secs(0.2)), 0.2)]);
export const tom = (f, r) => sum(secs(0.5), [thump(0.5, f * 1.5, f, 0.05, 0.16, 1.1), scale(noiseBurst(r, 0.04, 0.006, { bp: 1800 }), 0.25)]);
export const woodblock = (f, r) => sum(secs(0.12), [modal(f, [[1, 1, 0.045], [2.2, 0.4, 0.025]], secs(0.12)), scale(noiseBurst(r, 0.01, 0.002, { hp: 3000 }), 0.25)]);
export const darbuka = (kind, r) => kind === 'doum'
    ? sum(secs(0.4), [thump(0.4, 220, 105, 0.03, 0.12, 1.1), scale(modal(300, [[1, 1, 0.1]], secs(0.2)), 0.2)])
    : sum(secs(0.2), [modal(1900, [[1, 1, 0.04], [1.5, 0.5, 0.03]], secs(0.15)), scale(noiseBurst(r, 0.05, 0.008, { bp: 3600, q: 1.5 }), 0.7)]);
export const frameDrum = r => sum(secs(0.5), [thump(0.5, 160, 90, 0.04, 0.2, 1.1), scale(noiseBurst(r, 0.06, 0.012, { lp: 1600 }), 0.4)]);
export const anvil = (f, r) => sum(secs(0.9), [modal(f, [[1, 1, 0.5], [2.76, 0.7, 0.4], [5.4, 0.5, 0.3], [8.9, 0.3, 0.2], [13.4, 0.15, 0.12]], secs(0.9)), scale(noiseBurst(r, 0.02, 0.003, { hp: 2000 }), 0.7)]);
export const crash = (r, len = 2.2) => noiseBurst(r, len, len / 3.2, { hp: 5000 });
export function riser(r, len, f0 = 300, f1 = 9000, level = 1) {
    const n = secs(len), x = svf(white(n, r), 'bp', i => f0 * Math.pow(f1 / f0, i / n), 1.0);
    for (let i = 0; i < n; i++) { const t = i / n; x[i] *= t * t * level * 2; }
    return x;
}
export function reverseCrash(r, len) { const n = secs(len), x = noiseBurst(r, len, len / 3, { hp: 4000 }), o = mk(n); for (let i = 0; i < n; i++) o[i] = x[n - 1 - i] * (i / n); return o; }

// ---- bass ----
export function bass(kind, f, len, r) {
    const n = secs(len + 0.12), o = new Osc(0);
    const out = mk(n);
    if (kind === 'pluck') return sum(n, [scale(pluck(f, n, r, { decay: 0.9975, bright: 0.35, pick: 0.15 }), 1.6)]);
    if (kind === 'sine') { const e = env(n, 0.01, 0.05, 0.85, 0.08, len); for (let i = 0; i < n; i++) out[i] = (Math.sin(TAU * f * i / SR) + 0.25 * Math.sin(TAU * 2 * f * i / SR)) * e[i]; return out; }
    if (kind === 'fm') {
        const e = env(n, 0.004, 0.18, 0.4, 0.06, len); let p1 = 0, p2 = 0;
        for (let i = 0; i < n; i++) { const t = i / SR; p2 += TAU * f * 2 / SR; const mod = Math.sin(p2) * (2.2 * Math.exp(-t / 0.12) + 0.5); p1 += TAU * f / SR; out[i] = Math.sin(p1 + mod) * e[i]; }
        return out;
    }
    const e = env(n, 0.006, 0.12, kind === 'reese' ? 0.9 : 0.62, 0.07, len), o2 = new Osc(0.3), o3 = new Osc(0.6);
    const detune = kind === 'reese' ? 1.006 : 1.0015;
    for (let i = 0; i < n; i++) out[i] = o.saw(f) + (kind === 'reese' ? o2.saw(f * detune) + o3.saw(f / detune) : o2.square(f * 0.5, 0.5) * 0.9);
    const cutoff = kind === 'reese' ? i => 260 + 420 * Math.exp(-i / (0.2 * SR)) : kind === 'hard' ? i => 280 + 1500 * Math.exp(-i / (0.1 * SR)) : i => 200 + 1100 * Math.exp(-i / (0.09 * SR));
    const y = svf(out, 'lp', cutoff, kind === 'hard' ? 2.4 : 1.2);
    const sub = mk(n); for (let i = 0; i < n; i++) sub[i] = Math.sin(TAU * f * i / SR) * 0.9;
    const mix = mk(n); for (let i = 0; i < n; i++) mix[i] = (y[i] * (kind === 'hard' ? 0.6 : 0.5) + sub[i] * 0.7) * e[i];
    return kind === 'hard' ? softClip(mix, 3.5) : kind === 'reese' ? softClip(mix, 1.8) : mix;
}

// ---- pads ----
// One chord (list of Hz) held for `len` seconds: detuned saws through a moving low-pass.
export function padChord(freqs, len, { voices = 5, spreadCents = 14, cutoff = 1400, movement = 0.5, lfoHz = 0.25, lfoPhase = 0, attack = 0.5, release = 0.8, q = 0.9, wave = 'saw', noise = 0 } = {}, r) {
    const n = secs(len + release), mixed = mk(n);
    for (const f of freqs) {
        for (let v = 0; v < voices; v++) {
            const cents = voices === 1 ? 0 : spreadCents * (v / (voices - 1) * 2 - 1), fr = f * Math.pow(2, cents / 1200), o = new Osc(r());
            for (let i = 0; i < n; i++) mixed[i] += wave === 'tri' ? o.tri(fr) : wave === 'square' ? o.square(fr, 0.5) * 0.7 : o.saw(fr);
        }
    }
    const lo = svf(mixed, 'lp', i => cutoff * (1 + movement * Math.sin(TAU * lfoHz * i / SR + lfoPhase)), q), e = env(n, attack, 0.2, 0.85, release, len), g = 1 / (voices * Math.sqrt(freqs.length));
    for (let i = 0; i < n; i++) lo[i] *= e[i] * g;
    if (noise) { const air = svf(white(n, r), 'bp', cutoff * 1.6, 0.6); for (let i = 0; i < n; i++) lo[i] += air[i] * e[i] * noise; }
    return lo;
}
// Formant "aah" choir pad
export function choir(freqs, len, r, { vowel = ['a', 'o'], attack = 0.7, release = 0.9, level = 0.5 } = {}) {
    const n = secs(len + release), out = mk(n);
    for (const f of freqs) {
        for (const d of [-0.004, 0.004]) {
            const x = formantVoice(n, i => f * (1 + d) * (1 + 0.004 * Math.sin(TAU * 4.8 * i / SR)), i => mixVowel(vowel[0], vowel[1], 0.5 + 0.5 * Math.sin(TAU * 0.2 * i / SR)), r, { jitter: 0.004, shimmer: 0.04, breath: 0.03, tilt: 2200 });
            for (let i = 0; i < n; i++) out[i] += x[i];
        }
    }
    const e = env(n, attack, 0.2, 0.9, release, len);
    for (let i = 0; i < n; i++) out[i] *= e[i] * level / Math.sqrt(freqs.length * 2);
    return out;
}

// ---- melodic voices ----
export function accordion(f, len, r, { vibHz = 5.4 } = {}) {
    const n = secs(len + 0.1), a = new Osc(0), b = new Osc(0.2), c = new Osc(0.5), x = mk(n);
    for (let i = 0; i < n; i++) { const t = i / SR, vib = 1 + 0.003 * Math.min(1, t / 0.2) * Math.sin(TAU * vibHz * t); x[i] = a.saw(f * vib) * 0.5 + b.saw(f * vib * 1.0065) * 0.5 + c.square(f * vib * 0.5 * 0.9975, 0.35) * 0.35; }
    const y = svf(svf(x, 'bp', 1400, 0.45), 'lp', 4200, 0.8), e = env(n, 0.03, 0.05, 0.9, 0.07, len);
    const bellows = svf(white(n, r), 'bp', 2400, 1); for (let i = 0; i < n; i++) y[i] = (y[i] * 1.8 + bellows[i] * 0.02) * e[i] * (0.9 + 0.1 * Math.sin(TAU * 4.2 * i / SR));
    return y;
}
export function guitarTwang(f, len, r, { slideFrom = 0, decay = 0.9985 } = {}) {
    const n = secs(len + 0.6);
    let x;
    if (slideFrom) { // slide: render a plucked note then re-pitch with a gliding resample
        const base = pluck(f, n + 2000, r, { decay, bright: 0.8, pick: 0.18 }), o = mk(n); let pos = 0;
        for (let i = 0; i < n; i++) { const ratio = (slideFrom / f) ** 0 * (1 + (slideFrom / f - 1) * Math.exp(-i / (0.07 * SR))); pos += ratio; const j = Math.floor(pos), fr = pos - j; o[i] = (base[j] ?? 0) * (1 - fr) + (base[j + 1] ?? 0) * fr; }
        x = o;
    } else x = pluck(f, n, r, { decay, bright: 0.8, pick: 0.18 });
    const tw = svf(x, 'bp', 2400, 1.2); for (let i = 0; i < n; i++) x[i] = (x[i] * 1.4 + tw[i] * 0.8) * (i < 200 ? i / 200 : 1);
    return x;
}
export const zither = (f, len, r) => scale(pluck(f, secs(len + 0.7), r, { decay: 0.9992, bright: 0.95, pick: 0.12 }), 1.2);
export const harp = (f, len, r) => scale(pluck(f, secs(len + 0.9), r, { decay: 0.9994, bright: 0.55, pick: 0.35 }), 1.3);
export function marimba(f, len, r) { const n = secs(Math.max(len, 0.35) + 0.5); return sum(n, [modal(f, [[1, 1, 0.35], [3.93, 0.28, 0.1], [9.1, 0.08, 0.04]], n), scale(noiseBurst(r, 0.01, 0.002, { bp: 2500 }), 0.12)]); }
export function kalimba(f, len, r) { const n = secs(Math.max(len, 0.4) + 0.8); return sum(n, [modal(f, [[1, 1, 0.9], [5.4, 0.22, 0.2], [8.9, 0.09, 0.1]], n), scale(noiseBurst(r, 0.012, 0.002, { bp: 4000 }), 0.1)]); }
export function fmBell(f, len, r, { ratio = 3.5, index = 3, tau = 0.8, glass = false } = {}) {
    const n = secs(len + tau * 1.4), o = mk(n); let p1 = 0, p2 = 0;
    for (let i = 0; i < n; i++) { const t = i / SR, e = Math.exp(-t / tau); p2 += TAU * f * ratio / SR; p1 += TAU * f / SR; o[i] = (Math.sin(p1 + Math.sin(p2) * index * e) + (glass ? 0.35 * Math.sin(p1 * 4.07) * Math.exp(-t / (tau * 0.4)) : 0)) * e * Math.min(1, t / 0.002); }
    return o;
}
export function flute(f, len, r, { breath = 0.07 } = {}) {
    const n = secs(len + 0.12), x = mk(n), a = new Osc(0);
    for (let i = 0; i < n; i++) { const t = i / SR, vib = 1 + 0.0045 * Math.min(1, t / 0.25) * Math.sin(TAU * 5.1 * t); const ph = a.sine(f * vib); x[i] = ph + 0.18 * Math.sin(2 * Math.asin(Math.max(-1, Math.min(1, ph))) * 1) ; }
    const nz = svf(white(n, r), 'bp', f * 2.4, 1.6), e = env(n, 0.07, 0.04, 0.88, 0.09, len);
    for (let i = 0; i < n; i++) { const t = i / SR; x[i] = (x[i] * 0.6 + nz[i] * breath * (1.6 - Math.min(1, t / 0.15) * 0.6)) * e[i]; }
    return x;
}
export function squareLead(f, len, r, { glideFrom = 0, pw = 0.35 } = {}) {
    const n = secs(len + 0.15), a = new Osc(0), b = new Osc(0.4), c = new Osc(0.7), x = mk(n);
    for (let i = 0; i < n; i++) { const t = i / SR, g = glideFrom ? 1 + (glideFrom / f - 1) * Math.exp(-t / 0.05) : 1, v = 1 + 0.004 * Math.min(1, t / 0.3) * Math.sin(TAU * 5.6 * t), fr = f * g * v; x[i] = a.square(fr, pw) + b.saw(fr * 1.007) * 0.6 + c.square(fr * 0.993, 0.5) * 0.5; }
    const y = svf(x, 'lp', i => 1800 + 3200 * Math.exp(-i / (0.35 * SR)), 1.1), e = env(n, 0.008, 0.08, 0.8, 0.12, len);
    for (let i = 0; i < n; i++) y[i] *= e[i] * 0.55;
    return y;
}
export function superLead(f, len, r, { voices = 7, cents = 16, cutoffHi = 7000 } = {}) {
    const n = secs(len + 0.2), x = mk(n);
    for (let v = 0; v < voices; v++) { const o = new Osc(r()), fr = f * Math.pow(2, cents * (v / (voices - 1) * 2 - 1) / 1200); for (let i = 0; i < n; i++) x[i] += o.saw(fr); }
    const y = svf(x, 'lp', i => 1200 + (cutoffHi - 1200) * Math.exp(-i / (0.25 * SR)), 1.0), e = env(n, 0.006, 0.1, 0.75, 0.15, len);
    for (let i = 0; i < n; i++) y[i] *= e[i] / voices * 1.6;
    return y;
}
export function sawStab(f, len, r, { hard = false } = {}) {
    const n = secs(len + 0.1), x = mk(n), os = [0, 0.3, 0.6].map(p => new Osc(p));
    for (let i = 0; i < n; i++) x[i] = os[0].saw(f) + os[1].saw(f * 1.01) + os[2].square(f * 0.5, 0.5) * 0.6;
    const y = svf(x, 'lp', i => 500 + 3500 * Math.exp(-i / (0.07 * SR)), hard ? 2.2 : 1.2), e = env(n, 0.003, 0.08, 0.3, 0.05, len);
    for (let i = 0; i < n; i++) y[i] *= e[i];
    return hard ? softClip(y, 2.5) : y;
}
export function theremin(f, len, r) {
    const n = secs(len + 0.3), x = mk(n), o = new Osc(0); let lag = f * 0.96;
    for (let i = 0; i < n; i++) { const t = i / SR; lag += (f - lag) * 0.0008; x[i] = o.sine(lag * (1 + 0.012 * Math.min(1, t / 0.4) * Math.sin(TAU * 5.8 * t))); }
    const e = env(n, 0.12, 0.1, 0.9, 0.3, len);
    for (let i = 0; i < n; i++) x[i] = (x[i] + 0.12 * Math.sin(2 * Math.PI * 2 * x[i])) * e[i] * 0.7;
    return x;
}
export function yodel(r, phrase, tempoSec) { // [[midi, beats]] chest/head alternation on "o-lay-ee"
    const total = phrase.reduce((a, [, b]) => a + b, 0) * tempoSec + 0.3, n = secs(total), out = mk(n);
    let t = 0, k = 0;
    for (const [midi, beats] of phrase) {
        const len = beats * tempoSec, head = k % 2 === 1, f = mtof(midi) * (head ? 1.0 : 0.5), m = secs(len + 0.05);
        const x = formantVoice(m, i => f * (1 + 0.012 * Math.sin(TAU * 6 * i / SR)) * (head && i < 0.03 * SR ? 0.9 + 3.3 * (i / SR) : 1), i => mixVowel(head ? 'i' : 'o', head ? 'e' : 'a', i / m, head ? 1.15 : 1), r, { jitter: 0.015, shimmer: 0.08, breath: 0.05, tilt: 3000, amp: i => Math.min(1, i / 300) * Math.min(1, (m - i) / 700) });
        add(out, x, t, 0.8); t += len; k++;
    }
    return out;
}
export const bellTree = (r, n = 10) => { const o = mk(secs(1.6)); for (let k = 0; k < n; k++) add(o, modal(1800 * Math.pow(1.12, k), [[1, 1, 0.6], [2.7, 0.3, 0.3]], secs(1.0)), k * 0.035, 0.3); return o; };
export const cowbell = (f, r) => sum(secs(0.7), [modal(f, [[1, 1, 0.35], [1.51, 0.6, 0.25], [2.4, 0.3, 0.15]], secs(0.7)), scale(noiseBurst(r, 0.01, 0.002, { hp: 2500 }), 0.2)]);
void clamp; void mtof; void tone; void SAW_H;
